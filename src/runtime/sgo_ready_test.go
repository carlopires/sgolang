// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"errors"
	"math/rand"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

// R01: Start waits for exactly one ready signal; the child keeps running in its nursery afterwards.
func TestSGoReadySuccess(t *testing.T) {
	root(t, func() {
		var signalled, after atomic.Bool
		s := mustOpen(t)
		err := runtime.SGoStart(s, func(*runtime.SGoTask) error {
			for range 50 {
				runtime.Gosched() // delay the ready notification
			}
			signalled.Store(true)
			runtime.SGoReadySignal()
			for range 20 {
				runtime.Gosched()
			}
			after.Store(true)
			return nil
		})
		if err != nil || !signalled.Load() {
			t.Fatalf("Start = %v before the signal (signalled %v)", err, signalled.Load())
		}
		r := mustJoin(t, s)
		if r.Outcome != "Succeeded" || r.Children != 1 || !after.Load() {
			t.Fatalf("join %+v, child ran after ready: %v", r, after.Load())
		}
		if n := runtime.SGoReadyWaiters(); n != 0 {
			t.Fatalf("%d starters still registered", n)
		}
	})
}

// R02: the child errors, fails, or returns before signalling; the starter wakes with that outcome
// and the child is joined with its nursery.
func TestSGoReadyChildEndsFirst(t *testing.T) {
	boom := errors.New("not ready: boom")
	for _, tc := range []struct {
		name    string
		child   func() error
		start   string // the Start error, or a prefix of it
		outcome string
	}{
		{"error", func() error { return boom }, boom.Error(), "Errored"},
		{"panic", func() error { panic("crash") }, "child ", "Failed"},
		{"return nil", func() error { return nil }, "child ", "Succeeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root(t, func() {
				s := mustOpen(t)
				err := runtime.SGoStart(s, func(*runtime.SGoTask) error { return tc.child() })
				if err == nil || !strings.HasPrefix(err.Error(), tc.start) {
					t.Fatalf("Start = %v, want %q...", err, tc.start)
				}
				if tc.start == "child " && !strings.HasSuffix(err.Error(), " ended before ready") {
					t.Fatalf("Start = %v", err)
				}
				func() {
					defer func() { recover() }() // a failed child unwinds the join (SGO-NUR-004)
					r, jerr := runtime.SGoJoin(s)
					if jerr != nil || r.Outcome != tc.outcome || r.Children != 1 {
						t.Errorf("join %+v %v, want %s with one child", r, jerr, tc.outcome)
					}
				}()
			})
		})
	}
}

// R02: cancellation of the starter's scope before readiness wakes the starter with a cancellation
// error; the started child stays registered, is cancelled with the nursery, and is joined.
func TestSGoReadyStarterCancelled(t *testing.T) {
	root(t, func() {
		outer := mustOpen(t)
		var startErr atomic.Value
		var childDone atomic.Bool
		mustSpawn(t, outer, func(*runtime.SGoTask) error {
			n := mustOpen(t)
			err := runtime.SGoStart(n, func(*runtime.SGoTask) error {
				defer childDone.Store(true)
				return runtime.SGoSleep(60_000) // never signals; ends by cancellation
			})
			startErr.Store(errBox{err})
			r := mustJoin(t, n)
			if r.Children != 1 || r.Cancelled != 1 {
				t.Errorf("inner join %+v", r)
			}
			return err
		})
		waitFor(t, "the starter to park", func() bool { return runtime.SGoReadyWaiters() == 1 })
		if err := runtime.SGoCancel(outer); err != nil {
			t.Fatal(err)
		}
		mustJoin(t, outer)
		err := startErr.Load().(errBox).err
		if !runtime.SGoIsCancelled(err) || !childDone.Load() {
			t.Fatalf("Start = %v, child done %v", err, childDone.Load())
		}
	})
}

type errBox struct{ err error }

// R03: a second Signal, through an alias, is a contract violation in the child; the accepted
// readiness is unchanged. A Signal by another task is rejected the same way.
func TestSGoReadyDuplicateSignal(t *testing.T) {
	root(t, func() {
		var second atomic.Value
		s := mustOpen(t)
		err := runtime.SGoStart(s, func(*runtime.SGoTask) error {
			runtime.SGoReadySignal()
			func() {
				defer func() { second.Store(recover()) }()
				runtime.SGoReadySignal()
			}()
			return nil
		})
		if err != nil {
			t.Fatalf("Start = %v", err)
		}
		mustJoin(t, s)
		if v, ok := second.Load().(error); !ok || !isViolation(v) || !strings.Contains(v.Error(), "twice") {
			t.Fatalf("second signal: %v", second.Load())
		}

		// The root task is not the started child: its Signal is rejected and changes nothing.
		s = mustOpen(t)
		release := make(chan struct{})
		if err := runtime.SGoStart(s, func(*runtime.SGoTask) error {
			runtime.SGoReadySignal()
			<-release
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		func() {
			defer func() {
				if r := recover(); r == nil || !isViolation(r.(error)) {
					t.Errorf("signal by another task: %v", r)
				}
			}()
			runtime.SGoReadySignal() // the root task has no readiness record
		}()
		close(release)
		mustJoin(t, s)
	})
}

// R04: parent cancellation races child admission and readiness. Start reports success only for a
// child that signalled, every admitted child is accounted for by the join, and nothing stays
// registered.
func TestSGoReadyCancelRace(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	var ok, cancelled, none int
	for range 400 {
		a, b := rng.Intn(40), rng.Intn(8) // child delay before signalling; canceller delay
		root(t, func() {
			outer := mustOpen(t)
			var signalled atomic.Bool
			var startErr atomic.Value
			var started atomic.Bool
			mustSpawn(t, outer, func(*runtime.SGoTask) error {
				n := mustOpen(t)
				started.Store(true)
				err := runtime.SGoStart(n, func(*runtime.SGoTask) error {
					for range a {
						runtime.Gosched()
					}
					if err := runtime.SGoCheckpoint(); err != nil {
						return err
					}
					signalled.Store(true)
					runtime.SGoReadySignal()
					return runtime.SGoSleep(60_000)
				})
				startErr.Store(errBox{err})
				r := mustJoin(t, n)
				if err == nil && (!signalled.Load() || r.Children != 1) {
					t.Errorf("Start succeeded for an unowned child: signalled %v, join %+v", signalled.Load(), r)
				}
				return err
			})
			// Spin rather than sleep: the canceller must run within a few scheduling steps of Start.
			for !started.Load() {
				runtime.Gosched()
			}
			for range b {
				runtime.Gosched()
			}
			if err := runtime.SGoCancel(outer); err != nil {
				t.Fatal(err)
			}
			mustJoin(t, outer)
			switch err := startErr.Load().(errBox).err; {
			case err == nil:
				ok++
			case runtime.SGoIsCancelled(err):
				cancelled++
			default:
				none++
			}
		})
		if n := runtime.SGoReadyWaiters(); n != 0 {
			t.Fatalf("%d starters still registered", n)
		}
	}
	t.Logf("ready won %d, cancellation won %d, other %d", ok, cancelled, none)
	if none != 0 {
		t.Fatalf("%d starts ended with neither success nor cancellation", none)
	}
	if ok == 0 || cancelled == 0 {
		t.Fatalf("the race was not exercised: ready won %d, cancellation won %d", ok, cancelled)
	}
}
