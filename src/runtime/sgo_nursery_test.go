// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"
)

// Tests of the nursery statement's runtime entry points (sgo_nursery.go): the outcome applied at a
// normal exit (SGO-NUR-004) and the unwinding barrier (SGO-DEF-005).

func spawnIn(t *testing.T, p unsafe.Pointer, fn func(*runtime.SGoTask) error) {
	t.Helper()
	mustSpawn(t, runtime.SGoScopeOf(p), fn)
}

// catch runs f and returns the value it panicked with, or nil.
func catch(f func()) (r any) {
	defer func() { r = recover() }()
	f()
	return nil
}

// SGO-NUR-004 rules 2-4 when the body ends normally.
func TestSGoNurseryExitOutcomes(t *testing.T) {
	root(t, func() {
		// Rule 4: all children succeeded; the statement completes normally.
		if r := catch(func() {
			p := runtime.SGoNurseryEnter()
			spawnIn(t, p, func(*runtime.SGoTask) error { return nil })
			runtime.SGoNurseryExit(p)
		}); r != nil {
			t.Fatalf("succeeding children: exit panicked with %v", r)
		}

		// Rule 3: ChildError with the message of the errored child with the smallest creation ID.
		r := catch(func() {
			p := runtime.SGoNurseryEnter()
			spawnIn(t, p, func(*runtime.SGoTask) error { time.Sleep(5 * time.Millisecond); return errors.New("first") })
			spawnIn(t, p, func(*runtime.SGoTask) error { return errors.New("second") })
			runtime.SGoNurseryExit(p)
		})
		if err, ok := r.(error); !ok || err.Error() != "ChildError: first" {
			t.Fatalf("errored children: got %v, want ChildError: first", r)
		}

		// Rule 2: a failed child is primary over an errored one, and its payload is unwound as is.
		r = catch(func() {
			p := runtime.SGoNurseryEnter()
			spawnIn(t, p, func(*runtime.SGoTask) error { return errors.New("errored") })
			spawnIn(t, p, func(*runtime.SGoTask) error { panic("boom") })
			runtime.SGoNurseryExit(p)
		})
		if r != "boom" {
			t.Fatalf("failed child: got %v, want boom", r)
		}
		if n := runtime.SGoNest(); n != 0 {
			t.Fatalf("%d nurseries still open", n)
		}
	})
}

// SGO-DEF-005: unwinding leaves frames innermost first; in each frame the active nurseries complete
// before the frame's deferred calls run, and nurseries of outer frames stay open until unwinding
// reaches their frame.
func TestSGoUnwindCompletesNurseriesBeforeDefers(t *testing.T) {
	root(t, func() {
		var mu sync.Mutex
		var log []string
		add := func(s string) {
			mu.Lock()
			log = append(log, s)
			mu.Unlock()
		}
		if r := catch(func() { sgoOuterFrame(t, add) }); r != "unwind" {
			t.Fatalf("panic value %v, want unwind", r)
		}
		idx := map[string]int{}
		for i, s := range log {
			idx[s] = i + 1
		}
		for _, s := range []string{"inner child", "inner defer, outer open", "outer child", "outer defer, none open"} {
			if idx[s] == 0 {
				t.Fatalf("missing %q in %q", s, log)
			}
		}
		if idx["inner child"] > idx["inner defer, outer open"] || idx["outer child"] > idx["outer defer, none open"] ||
			idx["inner defer, outer open"] > idx["outer defer, none open"] {
			t.Fatalf("order %q", log)
		}
		if n := runtime.SGoNest(); n != 0 {
			t.Fatalf("%d nurseries still open after unwinding", n)
		}
	})
}

//go:noinline
func sgoOuterFrame(t *testing.T, add func(string)) {
	p := runtime.SGoNurseryEnter()
	defer func() { // open-coded defer
		if runtime.SGoNest() == 0 {
			add("outer defer, none open")
		} else {
			add("outer defer, nursery still open")
		}
	}()
	spawnIn(t, p, func(*runtime.SGoTask) error { time.Sleep(20 * time.Millisecond); add("outer child"); return nil })
	sgoInnerFrame(t, add)
	runtime.SGoNurseryExit(p) // not reached
}

//go:noinline
func sgoInnerFrame(t *testing.T, add func(string)) {
	p := runtime.SGoNurseryEnter()
	for i := 0; i < 1; i++ {
		defer func() { // linked (non-open-coded) defer
			if runtime.SGoNest() == 1 {
				add("inner defer, outer open")
			} else {
				add("inner defer, wrong nest")
			}
		}()
	}
	spawnIn(t, p, func(*runtime.SGoTask) error { time.Sleep(20 * time.Millisecond); add("inner child"); return nil })
	sgoGrowStack(200) // the nursery's frame offset must survive stack copying
	panic("unwind")
}

//go:noinline
func sgoGrowStack(n int) int {
	var buf [256]byte
	buf[n%256] = byte(n)
	if n == 0 {
		return int(buf[0])
	}
	return sgoGrowStack(n-1) + int(buf[n%256])
}

// A nursery opened in a loop is a distinct scope each iteration and is completed each time
// (SGO-CTL-004); nothing accumulates across iterations.
func TestSGoNurseryInLoop(t *testing.T) {
	root(t, func() {
		var done [3]bool
		for i := 0; i < 3; i++ {
			p := runtime.SGoNurseryEnter()
			spawnIn(t, p, func(*runtime.SGoTask) error { done[i] = true; return nil })
			runtime.SGoNurseryExit(p)
			if !done[i] || runtime.SGoNest() != 0 {
				t.Fatalf("iteration %d: done=%v nest=%d", i, done[i], runtime.SGoNest())
			}
		}
	})
}
