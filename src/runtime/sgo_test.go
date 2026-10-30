// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"errors"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// root runs f as the S-Go root task on the test goroutine (W02 conformance hooks).
func root(t *testing.T, f func()) {
	t.Helper()
	runtime.SGoEnterRoot()
	defer runtime.SGoLeaveRoot()
	f()
}

func mustOpen(t *testing.T) *runtime.SGoScope {
	t.Helper()
	s, err := runtime.SGoOpen()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func mustSpawn(t *testing.T, s *runtime.SGoScope, fn func(*runtime.SGoTask) error) {
	t.Helper()
	if err := runtime.SGoSpawn(s, fn); err != nil {
		t.Fatal(err)
	}
}

func mustJoin(t *testing.T, s *runtime.SGoScope) runtime.SGoResult {
	t.Helper()
	r, err := runtime.SGoJoin(s)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func isViolation(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "SpawnContractViolation")
}

// O01: join implies every child has finished; no child effect happens afterwards.
func TestSGoJoin(t *testing.T) {
	root(t, func() {
		var done atomic.Int64
		s := mustOpen(t)
		for i := 0; i < 50; i++ {
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				time.Sleep(time.Millisecond)
				done.Add(1)
				return nil
			})
		}
		r := mustJoin(t, s)
		if got := done.Load(); got != 50 {
			t.Fatalf("after join %d of 50 children finished", got)
		}
		time.Sleep(10 * time.Millisecond)
		if got := done.Load(); got != 50 || r.Children != 50 || r.Outcome != "Succeeded" {
			t.Fatalf("after join: done=%d result=%+v", got, r)
		}
		if st := runtime.SGoScopeState(s); st != "CLOSED" {
			t.Fatalf("state %s, want CLOSED", st)
		}
	})
}

// O02: a child's owner scope and current scope differ inside its own nursery; nested work joins first.
func TestSGoNestedOwnership(t *testing.T) {
	root(t, func() {
		var innerDone, outerAfterInner atomic.Bool
		outer := mustOpen(t)
		mustSpawn(t, outer, func(task *runtime.SGoTask) error {
			inner, err := runtime.SGoOpen()
			if err != nil {
				return err
			}
			if runtime.SGoTaskScope(task) != outer || runtime.SGoCurrent(task) != inner || runtime.SGoScopeParent(inner) != outer {
				return errors.New("owner/current scope wrong inside a nested nursery")
			}
			for i := 0; i < 5; i++ {
				if err := runtime.SGoSpawn(inner, func(*runtime.SGoTask) error {
					time.Sleep(time.Millisecond)
					innerDone.Store(true)
					return nil
				}); err != nil {
					return err
				}
			}
			if _, err := runtime.SGoJoin(inner); err != nil {
				return err
			}
			outerAfterInner.Store(innerDone.Load())
			if runtime.SGoCurrent(task) != outer {
				return errors.New("current scope not restored after the inner nursery closed")
			}
			return nil
		})
		r := mustJoin(t, outer)
		if r.Outcome != "Succeeded" || !outerAfterInner.Load() {
			t.Fatalf("result %+v err=%v innerJoinedFirst=%v", r, r.Err, outerAfterInner.Load())
		}
	})
}

func spawnVia(s *runtime.SGoScope, out *atomic.Int64) error {
	return runtime.SGoSpawn(s, func(*runtime.SGoTask) error { out.Add(1); return nil })
}

// O03: a helper given the handle explicitly spawns through the checked path into that nursery.
func TestSGoDelegation(t *testing.T) {
	root(t, func() {
		var n atomic.Int64
		s := mustOpen(t)
		for i := 0; i < 3; i++ {
			if err := spawnVia(s, &n); err != nil {
				t.Fatal(err)
			}
		}
		if r := mustJoin(t, s); r.Children != 3 || n.Load() != 3 {
			t.Fatalf("delegated children: result %+v ran %d", r, n.Load())
		}
	})
}

// O04 and S07: invalid handles are refused before any child is created.
func TestSGoInvalidHandles(t *testing.T) {
	root(t, func() {
		nop := func(*runtime.SGoTask) error { return nil }
		if err := runtime.SGoSpawn(nil, nop); !isViolation(err) {
			t.Errorf("nil handle: %v", err)
		}
		s := mustOpen(t)
		if err := runtime.SGoSpawn(s, nil); !isViolation(err) {
			t.Errorf("nil callback: %v", err)
		}
		// Ancestor bypass: inside inner, spawning into outer is refused (SGO-SPAWN-002 (c)).
		inner := mustOpen(t)
		if err := runtime.SGoSpawn(s, nop); !isViolation(err) {
			t.Errorf("spawn into outer from inner: %v", err)
		}
		if r := mustJoin(t, inner); r.Children != 0 {
			t.Errorf("inner got children: %+v", r)
		}
		// Unrelated task: a child of s may not spawn into b, which is not its current scope.
		b := s
		var childErr atomic.Value
		other := mustOpen(t)
		mustSpawn(t, other, func(*runtime.SGoTask) error {
			childErr.Store(runtime.SGoSpawn(b, nop))
			return nil
		})
		mustJoin(t, other)
		if err, _ := childErr.Load().(error); !isViolation(err) {
			t.Errorf("unrelated task spawn: %v", err)
		}
		r := mustJoin(t, s)
		if r.Children != 0 {
			t.Errorf("refused spawns created children: %+v", r)
		}
		// Closed: spawning into a closed scope is refused.
		if err := runtime.SGoSpawn(s, nop); !isViolation(err) {
			t.Errorf("spawn after close: %v", err)
		}
	})
	// A goroutine that is not an S-Go task has no authority at all.
	if _, err := runtime.SGoOpen(); !isViolation(err) {
		t.Errorf("non-task open: %v", err)
	}
}

// S06: no ambient authority. A plain goroutine cannot open or spawn; there is no way to obtain a
// nursery other than being handed one.
func TestSGoNoAmbientAuthority(t *testing.T) {
	root(t, func() {
		s := mustOpen(t)
		var openErr, spawnErr error
		var wg sync.WaitGroup
		wg.Add(1)
		go func() { // plain goroutine (test-lane Go code), not an S-Go task
			defer wg.Done()
			_, openErr = runtime.SGoOpen()
			spawnErr = runtime.SGoSpawn(s, func(*runtime.SGoTask) error { return nil })
		}()
		wg.Wait()
		if !isViolation(openErr) || !isViolation(spawnErr) {
			t.Errorf("plain goroutine: open=%v spawn=%v", openErr, spawnErr)
		}
		if r := mustJoin(t, s); r.Children != 0 {
			t.Errorf("children created without authority: %+v", r)
		}
	})
}

// O05: a live child admits a descendant after the owner entered join; it is joined, never lost.
func TestSGoDescendantDuringJoin(t *testing.T) {
	root(t, func() {
		for iter := 0; iter < 300; iter++ {
			var grand atomic.Int64
			s := mustOpen(t)
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				if iter%2 == 0 {
					time.Sleep(50 * time.Microsecond)
				}
				return runtime.SGoSpawn(s, func(*runtime.SGoTask) error {
					runtime.Gosched()
					grand.Add(1)
					return nil
				})
			})
			r := mustJoin(t, s)
			if r.Children != 2 || grand.Load() != 1 || r.Outcome != "Succeeded" {
				t.Fatalf("iteration %d: result %+v grandchild ran %d", iter, r, grand.Load())
			}
		}
	})
}

// O06: reused goroutine records never carry a stale task.
func TestSGoGoroutineReuse(t *testing.T) {
	root(t, func() {
		seen := map[uint64]bool{}
		for i := 0; i < 2000; i++ {
			var id uint64
			var ok bool
			s := mustOpen(t)
			mustSpawn(t, s, func(task *runtime.SGoTask) error {
				id, ok = runtime.SGoCurrentTaskID()
				if !ok || id != runtime.SGoTaskID(task) || runtime.SGoTaskScope(task) != s {
					return errors.New("child sees a wrong task record")
				}
				return nil
			})
			if r := mustJoin(t, s); r.Outcome != "Succeeded" || seen[id] {
				t.Fatalf("iteration %d: result %+v duplicate id %v", i, r, seen[id])
			}
			seen[id] = true
		}
	})
	// Plain goroutines started after many tasks finished (reusing their g records) are not tasks.
	var stale atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 2000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if runtime.SGoIsTask() {
				stale.Add(1)
			}
		}()
	}
	wg.Wait()
	if stale.Load() != 0 {
		t.Fatalf("%d reused goroutines inherited an S-Go task", stale.Load())
	}
}

// O07: objects reachable only through scope/task records survive GC while live and are reclaimed
// after quiescence.
func TestSGoGCRoots(t *testing.T) {
	var scopeFreed, payloadFreed atomic.Bool
	root(t, func() {
		release := make(chan struct{})
		s := mustOpen(t)
		for i := 0; i < 8; i++ {
			payload := make([]byte, 1<<20)
			payload[0], payload[len(payload)-1] = 7, 9
			if i == 0 {
				runtime.SetFinalizer(&payload[0], func(*byte) { payloadFreed.Store(true) })
			}
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				<-release
				if payload[0] != 7 || payload[len(payload)-1] != 9 {
					return errors.New("payload corrupted")
				}
				return nil
			})
		}
		for i := 0; i < 5; i++ {
			runtime.GC()
		}
		close(release)
		if r := mustJoin(t, s); r.Outcome != "Succeeded" {
			t.Fatalf("result %+v err=%v", r, r.Err)
		}
		runtime.SetFinalizer(s, func(*runtime.SGoScope) { scopeFreed.Store(true) })
	})
	for i := 0; i < 50 && !(scopeFreed.Load() && payloadFreed.Load()); i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	if !scopeFreed.Load() || !payloadFreed.Load() {
		t.Fatalf("not reclaimed after quiescence: scope=%v payload=%v", scopeFreed.Load(), payloadFreed.Load())
	}
}

//go:noinline
func deep(n int, at func()) int {
	var pad [256]byte
	pad[n%256] = byte(n)
	if n == 0 {
		at()
		return int(pad[0])
	}
	return deep(n-1, at) + int(pad[n%256])
}

// O08: the owner's stack grows and shrinks while its nursery and children are live.
func TestSGoMovableStack(t *testing.T) {
	root(t, func() {
		release := make(chan struct{})
		s := mustOpen(t)
		var ran atomic.Int64
		for i := 0; i < 4; i++ {
			mustSpawn(t, s, func(*runtime.SGoTask) error { <-release; ran.Add(1); return nil })
		}
		deep(4000, func() {
			runtime.GC()
			// Spawn from a deep frame: the scope pointer must not depend on the owner's stack.
			mustSpawn(t, s, func(*runtime.SGoTask) error { <-release; ran.Add(1); return nil })
		})
		runtime.GC() // may shrink the owner's stack
		close(release)
		if r := mustJoin(t, s); r.Children != 5 || ran.Load() != 5 || r.Outcome != "Succeeded" {
			t.Fatalf("result %+v ran %d", r, ran.Load())
		}
	})
}

// O09: writes by children are visible to the owner after join (run under -race too).
func TestSGoJoinVisibility(t *testing.T) {
	root(t, func() {
		vals := make([]int, 64)
		s := mustOpen(t)
		for i := range vals {
			mustSpawn(t, s, func(*runtime.SGoTask) error { vals[i] = i * i; return nil })
		}
		mustJoin(t, s)
		for i, v := range vals {
			if v != i*i {
				t.Fatalf("vals[%d] = %d after join", i, v)
			}
		}
	})
}

// SGO-NUR-004: failures beat errors; the lowest creation ID is primary regardless of timing.
func TestSGoOutcomes(t *testing.T) {
	root(t, func() {
		for iter := 0; iter < 50; iter++ {
			s := mustOpen(t)
			mustSpawn(t, s, func(*runtime.SGoTask) error { return errors.New("e1") })
			mustSpawn(t, s, func(*runtime.SGoTask) error { time.Sleep(time.Microsecond); panic("p2") })
			mustSpawn(t, s, func(*runtime.SGoTask) error { panic("p3") })
			r := mustJoin(t, s)
			if r.Outcome != "Failed" || r.Panic != "p2" || r.Failures != 3 {
				t.Fatalf("iteration %d: %+v", iter, r)
			}
		}
		s := mustOpen(t)
		mustSpawn(t, s, func(*runtime.SGoTask) error { return nil })
		mustSpawn(t, s, func(*runtime.SGoTask) error { return errors.New("only") })
		if r := mustJoin(t, s); r.Outcome != "Errored" || r.Err.Error() != "only" {
			t.Fatalf("errored: %+v", r)
		}
	})
}
