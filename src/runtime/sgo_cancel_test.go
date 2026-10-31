// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"errors"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// waitFor polls cond for up to 10 s.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !cond(); {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(100 * time.Microsecond)
	}
}

// C01: an operation in an already-cancelled scope reports cancellation without registering a wait.
func TestSGoCancelBeforeRegistration(t *testing.T) {
	root(t, func() {
		s := mustOpen(t)
		if err := runtime.SGoCancel(s); err != nil {
			t.Fatal(err)
		}
		before := runtime.SGoSleepers()
		err := runtime.SGoSleep(60_000)
		if !runtime.SGoIsCancelled(err) || err.Error() != "cancelled: Cancel by task 0" {
			t.Fatalf("Sleep in a cancelled scope = %v", err)
		}
		if n := runtime.SGoSleepers(); n != before {
			t.Fatalf("Sleep registered a wait (%d -> %d)", before, n)
		}
		if err := runtime.SGoCheckpoint(); !runtime.SGoIsCancelled(err) {
			t.Fatalf("Checkpoint = %v", err)
		}
		mustJoin(t, s)
		if err := runtime.SGoCheckpoint(); err != nil {
			t.Fatalf("cancellation not consumed at the closed nursery: %v", err)
		}
	})
}

// C02: cancellation racing a child that is about to park is never lost: every long sleep returns
// a cancellation error promptly.
func TestSGoCancelBeforePark(t *testing.T) {
	root(t, func() {
		for i := 0; i < 2000; i++ {
			s := mustOpen(t)
			var cancelled atomic.Bool
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				err := runtime.SGoSleep(60_000)
				cancelled.Store(runtime.SGoIsCancelled(err))
				return err
			})
			if i%2 == 0 {
				runtime.Gosched()
			}
			if err := runtime.SGoCancel(s); err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			r := mustJoin(t, s)
			if d := time.Since(start); d > 5*time.Second {
				t.Fatalf("iteration %d: join took %v (lost wake-up)", i, d)
			}
			if !cancelled.Load() || r.Cancelled != 1 {
				t.Fatalf("iteration %d: sleep not cancelled, result %+v", i, r)
			}
		}
	})
}

// C03 (subset 0: readiness is denied, SGO-READY-001; the only readiness is timer expiry): timer
// expiry racing cancellation has exactly one winner, and the returned result matches it. A
// duplicate wake-up would crash the scheduler (goready of a running goroutine).
func TestSGoCancelVersusTimer(t *testing.T) {
	root(t, func() {
		var timer, cancelled int
		for i := 0; i < 3000; i++ {
			s := mustOpen(t)
			var res atomic.Int32
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				err := runtime.SGoSleep(1)
				switch {
				case err == nil:
					res.Store(1)
				case runtime.SGoIsCancelled(err):
					res.Store(2)
				default:
					res.Store(3)
				}
				return nil
			})
			if i%3 != 0 {
				time.Sleep(time.Duration(i%1000) * time.Microsecond)
			}
			if err := runtime.SGoCancel(s); err != nil {
				t.Fatal(err)
			}
			mustJoin(t, s)
			switch res.Load() {
			case 1:
				timer++
			case 2:
				cancelled++
			default:
				t.Fatalf("iteration %d: sleep result %d", i, res.Load())
			}
		}
		t.Logf("timer won %d, cancellation won %d", timer, cancelled)
		if timer == 0 || cancelled == 0 {
			t.Logf("only one side won; the race was not exercised in both directions this run")
		}
	})
}

// C04: cancellation is idempotent and the first cause sticks: a later Cancel and a later child
// failure do not change it.
func TestSGoCancelFirstCauseSticks(t *testing.T) {
	root(t, func() {
		s := mustOpen(t)
		release := make(chan struct{})
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			<-release
			return errors.New("late failure")
		})
		for i := 0; i < 3; i++ {
			if err := runtime.SGoCancel(s); err != nil {
				t.Fatal(err)
			}
		}
		close(release)
		waitFor(t, "child failure", func() bool { return runtime.SGoScopeState(s) == "CANCELLING" })
		time.Sleep(10 * time.Millisecond)
		if err := runtime.SGoCheckpoint(); err == nil || err.Error() != "cancelled: Cancel by task 0" {
			t.Fatalf("cause = %v, want the first one (Cancel by task 0)", err)
		}
		r := mustJoin(t, s)
		if r.Outcome != "Errored" {
			t.Fatalf("result %+v", r)
		}
	})
}

// C05 / SGO-CANCEL-004: an inner nursery's cancellation is consumed when it closes; an ancestor's
// is not, and it reaches inner scopes (cause Ancestor) and the tasks in them.
func TestSGoCancelNestedInherited(t *testing.T) {
	root(t, func() {
		outer := mustOpen(t)
		inner := mustOpen(t)
		if err := runtime.SGoCancel(inner); err != nil {
			t.Fatal(err)
		}
		mustJoin(t, inner)
		if err := runtime.SGoCheckpoint(); err != nil {
			t.Fatalf("inner cancellation not consumed: %v", err)
		}
		if err := runtime.SGoCancel(outer); err != nil {
			t.Fatal(err)
		}
		inner2 := mustOpen(t)
		if err := runtime.SGoCheckpoint(); err == nil || err.Error() != "cancelled: Cancel by task 0" {
			t.Fatalf("inside a new inner nursery: %v (the outermost linked cancelled scope is outer)", err)
		}
		ran := false
		mustSpawn(t, inner2, func(*runtime.SGoTask) error { ran = true; return nil })
		mustJoin(t, inner2)
		if ran {
			t.Fatal("a nursery opened inside a cancelled scope admitted a child (R5)")
		}
		if err := runtime.SGoCheckpoint(); !runtime.SGoIsCancelled(err) {
			t.Fatalf("outer cancellation consumed by the inner boundary: %v", err)
		}
		mustJoin(t, outer)
	})
}

// SGO-CANCEL-001 transitive propagation with exact ordering: the grandchild exists before the
// ancestor is cancelled and observes "ancestor scope <id> cancelled".
func TestSGoCancelPropagatesNested(t *testing.T) {
	root(t, func() {
		a := mustOpen(t)
		ready := make(chan struct{})
		var msg atomic.Value
		mustSpawn(t, a, func(*runtime.SGoTask) error {
			b, err := runtime.SGoOpen()
			if err != nil {
				return err
			}
			if err := runtime.SGoSpawn(b, func(*runtime.SGoTask) error {
				close(ready)
				err := runtime.SGoSleep(60_000)
				msg.Store(err.Error())
				return err
			}); err != nil {
				return err
			}
			_, err = runtime.SGoJoin(b)
			return err
		})
		<-ready
		waitFor(t, "grandchild asleep", func() bool { return runtime.SGoSleepers() > 0 })
		if err := runtime.SGoCancel(a); err != nil {
			t.Fatal(err)
		}
		r := mustJoin(t, a)
		want := "cancelled: ancestor scope " + itoa(runtime.SGoScopeID(a)) + " cancelled"
		if m, _ := msg.Load().(string); m != want || r.Outcome != "Succeeded" {
			t.Fatalf("grandchild saw %q (want %q), result %+v", m, want, r)
		}
	})
}

// C06: admission into a cancelled scope creates no child; the callback never runs.
func TestSGoCancelledAdmission(t *testing.T) {
	root(t, func() {
		s := mustOpen(t)
		if err := runtime.SGoCancel(s); err != nil {
			t.Fatal(err)
		}
		var ran atomic.Bool
		for i := 0; i < 100; i++ {
			mustSpawn(t, s, func(*runtime.SGoTask) error { ran.Store(true); return nil })
		}
		r := mustJoin(t, s)
		if ran.Load() || r.Children != 0 {
			t.Fatalf("cancelled scope admitted children: %+v", r)
		}
	})
}

// C08: no stale timer wakes a finished or later sleep; cancelled and expired records leave the
// registry.
func TestSGoTimerLifecycle(t *testing.T) {
	root(t, func() {
		base := runtime.SGoSleepers()
		for i := 0; i < 200; i++ {
			s := mustOpen(t)
			mustSpawn(t, s, func(*runtime.SGoTask) error { return runtime.SGoSleep(3) })
			if i%2 == 0 {
				if err := runtime.SGoCancel(s); err != nil {
					t.Fatal(err)
				}
			}
			mustJoin(t, s)
		}
		// Stale timers of cancelled sleeps fire now; a later sleep must last its full duration.
		start := time.Now()
		if err := runtime.SGoSleep(50); err != nil {
			t.Fatal(err)
		}
		if d := time.Since(start); d < 50*time.Millisecond {
			t.Fatalf("a 50 ms sleep returned after %v (woken by a stale timer)", d)
		}
		if n := runtime.SGoSleepers(); n != base {
			t.Fatalf("%d sleep records left registered", n-base)
		}
	})
}

// SGO-WAIT-001: a sleep lasts at least its duration on the monotonic clock; Sleep(0) returns nil
// unless cancelled.
func TestSGoSleepDuration(t *testing.T) {
	root(t, func() {
		for _, ms := range []int64{0, 1, 5, 20} {
			start := time.Now()
			if err := runtime.SGoSleep(ms); err != nil {
				t.Fatalf("Sleep(%d) = %v", ms, err)
			}
			if d := time.Since(start); d < time.Duration(ms)*time.Millisecond {
				t.Fatalf("Sleep(%d) returned after %v", ms, d)
			}
		}
	})
}

// P07 / SGO-CANCEL-004: an owner asleep inside the nursery is woken by a child failure; the
// failure is the cause.
func TestSGoOwnerWokenByChildFailure(t *testing.T) {
	root(t, func() {
		s := mustOpen(t)
		mustSpawn(t, s, func(task *runtime.SGoTask) error { return errors.New("boom") })
		start := time.Now()
		err := runtime.SGoSleep(60_000)
		if !runtime.SGoIsCancelled(err) || !strings.HasPrefix(err.Error(), "cancelled: child ") || time.Since(start) > 5*time.Second {
			t.Fatalf("owner sleep = %v after %v", err, time.Since(start))
		}
		if r := mustJoin(t, s); r.Outcome != "Errored" {
			t.Fatalf("result %+v", r)
		}
	})
}

// SGO-CANCEL-001 precondition: only a task that could spawn into the nursery may cancel it.
func TestSGoCancelMembership(t *testing.T) {
	root(t, func() {
		outer := mustOpen(t)
		inner := mustOpen(t)
		if err := runtime.SGoCancel(outer); err == nil {
			t.Fatal("cancel of an outer nursery from inside an inner one was accepted")
		}
		mustJoin(t, inner)
		if err := runtime.SGoCheckpoint(); err != nil {
			t.Fatalf("refused cancel had an effect: %v", err)
		}
		mustJoin(t, outer)
	})
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for ; v > 0; v /= 10 {
		b = append([]byte{byte('0' + v%10)}, b...)
	}
	return string(b)
}
