// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Structured synchronization primitives (D39): Event and CapacityLimiter. Both use the managed
// channel's lock, waiter records, and cancellation registry (sgo_chan.go); a limiter token is an
// obligation like a send permit, released with a TokenAbandoned entry at the owner's nursery-body
// exit and at task end. The executable reference model is sgo/model/nativemodel.go.

package runtime

import "unsafe"

// sgoEvent is sgo.Event: set is c.closed; waiters wait on c.recvs.
type sgoEvent struct{ c sgoChan }

func sgoEventSet(e *sgoEvent) {
	if raceenabled {
		racereleasemerge(unsafe.Pointer(e))
	}
	lock(&e.c.lock)
	if e.c.closed {
		unlock(&e.c.lock)
		return
	}
	e.c.closed = true
	wake := sgoChanClaimAll(&e.c.recvs, nil)
	unlock(&e.c.lock)
	sgoChanReady(wake)
}

// sgoEventWait returns nil once e is set (Set happens before it returns), or the cancellation error
// if the task is cancelled while it waits. A Wait that need not wait does not observe cancellation.
func sgoEventWait(e *sgoEvent, t *sgoTask) error {
	lock(&e.c.lock)
	for !e.c.closed {
		w := &sgoChanWait{task: t}
		if !sgoChanPark(&e.c, &e.c.recvs, w, waitReasonChanReceive, false) || w.state.Load() == sgoWaitCancelled {
			unlock(&e.c.lock)
			return sgoTaskCancellation(t)
		}
	}
	unlock(&e.c.lock)
	if raceenabled {
		raceacquire(unsafe.Pointer(e))
	}
	return nil
}

func sgoEventIsSet(e *sgoEvent) bool {
	lock(&e.c.lock)
	set := e.c.closed
	unlock(&e.c.lock)
	if set && raceenabled {
		raceacquire(unsafe.Pointer(e))
	}
	return set
}

// sgoLimiter is sgo.CapacityLimiter: n slots, used of them held; waiters queue FIFO on c.recvs.
// A released slot passes directly to the oldest waiting acquirer.
type sgoLimiter struct {
	c       sgoChan
	n, used int64
}

// sgoToken is a held slot: task-affine, released once.
type sgoToken struct {
	lim        *sgoLimiter
	owner      *sgoTask
	scope      *sgoScope // the owner's innermost nursery at Acquire (nil: the task body)
	released   bool
	prev, next *sgoToken // the owner's held tokens, touched only by the owner
}

func sgoLimiterNew(n int64) *sgoLimiter { return &sgoLimiter{n: n} }

func sgoLimiterCheck(l *sgoLimiter) {
	if l.used < 0 || l.used > l.n {
		throw("sgo: limiter holds more tokens than its capacity")
	}
}

func sgoLimiterAcquire(l *sgoLimiter, t *sgoTask) (*sgoToken, error) {
	lock(&l.c.lock)
	free := l.used < l.n
	for w := l.c.recvs.first; free && w != nil; w = w.next {
		free = !w.waiting() // FIFO: no slot is taken ahead of a waiting acquirer
	}
	if free {
		l.used++
		sgoLimiterCheck(l)
		unlock(&l.c.lock)
	} else {
		w := &sgoChanWait{task: t}
		if !sgoChanPark(&l.c, &l.c.recvs, w, waitReasonSemacquire, false) || w.state.Load() == sgoWaitCancelled {
			unlock(&l.c.lock)
			return nil, sgoTaskCancellation(t)
		}
		unlock(&l.c.lock) // Done: the releaser passed its slot
	}
	if raceenabled {
		raceacquire(unsafe.Pointer(l))
	}
	k := &sgoToken{lim: l, owner: t, scope: t.currentScope()}
	k.next = t.tokens
	if t.tokens != nil {
		t.tokens.prev = k
	}
	t.tokens = k
	return k, nil
}

func sgoTokenRelease(k *sgoToken) {
	if k == nil {
		panic(&sgoViolation{"release of a nil token"})
	}
	if sgoCurrent() != k.owner {
		panic(&sgoViolation{"release of a token by a task that did not acquire it"})
	}
	if k.released {
		panic(&sgoViolation{"release of a token that is already released"})
	}
	sgoTokenFree(k)
}

func sgoTokenFree(k *sgoToken) {
	k.released = true
	t := k.owner
	if k.prev != nil {
		k.prev.next = k.next
	} else if t.tokens == k {
		t.tokens = k.next
	}
	if k.next != nil {
		k.next.prev = k.prev
	}
	k.prev, k.next = nil, nil
	l := k.lim
	if raceenabled {
		racereleasemerge(unsafe.Pointer(l))
	}
	var wake *sgoChanWait
	lock(&l.c.lock)
	for w := l.c.recvs.first; w != nil; w = w.next {
		if w.claim(sgoWaitDone) {
			l.c.recvs.remove(w)
			wake = w
			break
		}
	}
	if wake == nil {
		l.used--
	}
	sgoLimiterCheck(l)
	unlock(&l.c.lock)
	sgoReadyOne(wake)
}

// sgoTokensResolve releases the task's unreleased tokens acquired inside scope s (all of them at task
// end), each with a TokenAbandoned entry in the task's failure sequence (D39).
func sgoTokensResolve(t *sgoTask, s *sgoScope, all bool) {
	for k := t.tokens; k != nil; {
		next := k.next
		if all || k.scope == s {
			t.abandoned++
			t.seq++
			t.entries = append(t.entries, sgoEntry{t.seq, sgoFailureInfo{"TokenAbandoned",
				"capacity token acquired by task " + sgoItoa(t.id) + " was not released; the runtime released it", ""}})
			sgoTokenFree(k)
		}
		k = next
	}
}

// Package sgo entry points (src/sgo/sync.go).

//go:linkname sgo_eventNew sgo.runtime_eventNew
func sgo_eventNew() unsafe.Pointer { return unsafe.Pointer(&sgoEvent{}) }

//go:linkname sgo_eventSet sgo.runtime_eventSet
func sgo_eventSet(e unsafe.Pointer) {
	sgoLabPoint() // a choice point in a lab run (D43); nothing otherwise
	sgoEventSet((*sgoEvent)(e))
}

//go:linkname sgo_eventWait sgo.runtime_eventWait
func sgo_eventWait(e, task unsafe.Pointer) error {
	sgoLabPoint()
	return sgoEventWait((*sgoEvent)(e), (*sgoTask)(task))
}

//go:linkname sgo_eventIsSet sgo.runtime_eventIsSet
func sgo_eventIsSet(e unsafe.Pointer) bool { return sgoEventIsSet((*sgoEvent)(e)) }

//go:linkname sgo_limiterNew sgo.runtime_limiterNew
func sgo_limiterNew(n int) unsafe.Pointer { return unsafe.Pointer(sgoLimiterNew(int64(n))) }

//go:linkname sgo_limiterAcquire sgo.runtime_limiterAcquire
func sgo_limiterAcquire(l, task unsafe.Pointer) (unsafe.Pointer, error) {
	sgoLabPoint()
	k, err := sgoLimiterAcquire((*sgoLimiter)(l), (*sgoTask)(task))
	return unsafe.Pointer(k), err
}

//go:linkname sgo_tokenRelease sgo.runtime_tokenRelease
func sgo_tokenRelease(k unsafe.Pointer) {
	sgoLabPoint()
	sgoTokenRelease((*sgoToken)(k))
}
