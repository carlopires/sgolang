// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Scope deadlines (D162, D165, D170; SGO-CANCEL-005 draft).
//
// n.CancelAfter(ms) sets a deadline on nursery n: when it passes, n is cancelled with the cause
// DeadlineExceeded, which every task linked to n (and every nested scope and I/O wait) observes as
// it observes any cancellation; sgo.IsDeadline recognizes it, also when inherited. A later call
// may only shorten the deadline. The timer is dropped when n closes. t.Deadline() reports the
// time left before the earliest deadline of the scopes t is linked to.
//
// The lab does not run deadlines yet: a virtual fire happens while the blocked task may hold
// channel and scope locks, so CancelAfter in the lab ends the run with UnsupportedLabEffect.

package runtime

import "unsafe"

// sgo_cancelAfter is n.CancelAfter(ms) after the argument check.
//
//go:linkname sgo_cancelAfter sgo.runtime_cancelAfter
func sgo_cancelAfter(scope unsafe.Pointer, ms int) {
	s := (*sgoScope)(scope)
	t := sgoCurrent()
	switch {
	case t == nil:
		panic(error(&sgoViolation{"CancelAfter from a goroutine that is not an S-Go task"}))
	case t.currentScope() != s:
		panic(error(&sgoViolation{"CancelAfter of a nursery that is not the caller's current scope"}))
	}
	if sgoLab != nil {
		sgoLabUnsupported()
	}
	ns := int64(ms) * 1_000_000
	if ns/1_000_000 != int64(ms) {
		ns = maxWhen
	}
	when := nanotime() + ns
	if when < 0 {
		when = maxWhen
	}
	tm := new(timer) // allocated before the lock
	tm.init(sgoDeadlineFire, s)
	lock(&s.lock)
	if s.state == sgoClosed || (s.deadline != 0 && s.deadline <= when) {
		unlock(&s.lock)
		return // only shortened, never extended (D165)
	}
	old := s.dtimer
	s.deadline, s.dtimer = when, tm
	unlock(&s.lock)
	if old != nil {
		old.stop()
	}
	tm.reset(when, 0)
}

func sgoDeadlineFire(arg any, _ uintptr, _ int64) {
	sgoCancel(arg.(*sgoScope), sgoCause{kind: sgoCauseDeadline})
}

// sgo_deadline is t.Deadline(): the milliseconds left before the earliest deadline of the scopes
// linked to t and their ancestors, rounded down and never negative; false if there is none.
//
//go:linkname sgo_deadline sgo.runtime_deadline
func sgo_deadline(task unsafe.Pointer) (int, bool) {
	t := (*sgoTask)(task)
	var best int64
	for p := t.currentScope(); p != nil; p = p.parent {
		lock(&p.lock)
		d := p.deadline
		unlock(&p.lock)
		if d != 0 && (best == 0 || d < best) {
			best = d
		}
	}
	if best == 0 {
		return 0, false
	}
	left := (best - nanotime()) / 1_000_000
	if left < 0 {
		left = 0
	}
	return int(left), true
}

// sgoIsDeadline reports whether err is a cancellation by a deadline (D162).
func sgoIsDeadline(err error) bool {
	if e, ok := any(err).(*sgoCancelErr); ok {
		return e.deadline
	}
	e, ok := any(err).(*sgoIOError)
	return ok && e.kind == "Cancelled" && e.deadline
}

//go:linkname sgo_isDeadline sgo.runtime_isDeadline
func sgo_isDeadline(err error) bool { return err != nil && sgoIsDeadline(err) }
