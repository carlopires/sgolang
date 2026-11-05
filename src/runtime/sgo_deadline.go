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
// In the lab a deadline is a virtual timer (sgoLabCancelAfter). It fires at an advance, when no
// task is ready, but cancelling takes the scope lock and the sleep lock, which the baton holder
// may hold while it parks; so the advance hands the baton to the driver, a goroutine that is not
// a task, which cancels the due nurseries with no lock held and then decides again (sgoLabDriver).
// In the lab, s.deadline is the virtual deadline in milliseconds plus one (0: none).

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
	if l := sgoLab; l != nil {
		sgoLabCancelAfter(l, s, t, int64(ms))
		return
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
	if l := sgoLab; l != nil {
		lock(&l.lock)
		now := l.now
		unlock(&l.lock)
		return int(max(best-1-now, 0)), true
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

// sgoLabCancelAfter is n.CancelAfter(ms) in the lab: the deadline is a virtual timer of scope s,
// replacing an earlier one only if it is earlier (D165).
func sgoLabCancelAfter(l *sgoLabState, s *sgoScope, t *sgoTask, ms int64) {
	l.ensureDriver()
	l.reserve()
	if cap(l.pending) < l.deadlines+1 { // no runtime lock held: only the baton holder runs here
		p := make([]*sgoScope, len(l.pending), 2*l.deadlines+4)
		copy(p, l.pending)
		l.pending = p
	}
	lock(&l.lock)
	when := l.now + ms + 1
	lock(&s.lock)
	if s.state == sgoClosed || (s.deadline != 0 && s.deadline <= when) {
		unlock(&s.lock)
		unlock(&l.lock)
		return // only shortened, never extended (D165)
	}
	s.deadline = when
	unlock(&s.lock)
	if l.dropDeadline(s) {
		l.deadlines--
	}
	if cap(l.timers) == len(l.timers) {
		throw("sgo: lab timer capacity not reserved")
	}
	l.timers = append(l.timers, sgoLabTimer{deadline: when - 1, id: l.nextTmr, t: t, scope: s}) // lock-alloc: ok (capacity reserved)
	l.nextTmr++
	l.deadlines++
	unlock(&l.lock)
}

// ensureDriver starts the deadline driver of the run on first use. Only the baton holder calls it.
func (l *sgoLabState) ensureDriver() {
	if l.driver == nil {
		d := &sgoTask{labID: -1}
		l.driver = d
		go sgoLabDriver(l, d)
	}
}

// sgoLabInterruptAt holds the virtual times of the interrupts the lab manifest injects (D179).
var (
	sgoLabInterruptAt [2]int64
	sgoLabInterruptN  int
)

// sgoLabArmInterrupts adds a virtual timer for each injected interrupt. The driver delivers them
// as production does: the first cancels the virtual root scope, the second ends the run with
// status 130. The root calls it once, holding the baton, before main.
func sgoLabArmInterrupts(l *sgoLabState, root *sgoTask) {
	if sgoLabInterruptN == 0 {
		return
	}
	l.ensureDriver()
	for i := 0; i < sgoLabInterruptN; i++ {
		l.reserve()
		lock(&l.lock)
		if cap(l.timers) == len(l.timers) {
			throw("sgo: lab timer capacity not reserved")
		}
		l.timers = append(l.timers, sgoLabTimer{deadline: sgoLabInterruptAt[i], id: l.nextTmr, t: root, scope: &sgoInterruptScope}) // lock-alloc: ok (capacity reserved)
		l.nextTmr++
		l.deadlines++
		unlock(&l.lock)
	}
}

// dropDeadline removes scope s's deadline timer and any pending cancellation of s, with l.lock
// held; it reports whether a timer was removed.
func (l *sgoLabState) dropDeadline(s *sgoScope) bool {
	found := false
	n := 0
	for _, tm := range l.timers {
		if tm.scope == s {
			found = true
			continue
		}
		l.timers[n] = tm
		n++
	}
	clear(l.timers[n:])
	l.timers = l.timers[:n]
	n = 0
	for _, p := range l.pending {
		if p != s {
			l.pending[n] = p
			n++
		}
	}
	clear(l.pending[n:])
	l.pending = l.pending[:n]
	return found
}

// sgoLabScopeClosed drops the deadline of a nursery that closed (D162).
func sgoLabScopeClosed(s *sgoScope) {
	l := sgoLab
	lock(&l.lock)
	if l.dropDeadline(s) {
		l.deadlines--
	}
	unlock(&l.lock)
}

// sgoLabDriver runs the lab's deadline cancellations. It waits for the baton like a task (it is
// not one: no ID, never in a ready set, no trace events of its own); when an advance fires
// deadlines it is chosen, cancels each due nursery in timer-ID order with no lock held (the
// cancellation readies the tasks it wakes), and passes the baton on as a task's decision does.
func sgoLabDriver(l *sgoLabState, d *sgoTask) {
	lock(&l.lock)
	d.labG = getg()
	l.waitBaton(d)
	for {
		lock(&l.lock)
		for len(l.pending) > 0 {
			s := l.pending[0]
			copy(l.pending, l.pending[1:])
			l.pending[len(l.pending)-1] = nil
			l.pending = l.pending[:len(l.pending)-1]
			unlock(&l.lock)
			if s == &sgoInterruptScope { // an injected interrupt (D179)
				l.interrupts++
				if l.interrupts == 1 {
					sgoInterruptPublish(sgoSigInt)
				} else {
					lock(&l.lock)
					l.finish(128+sgoSigInt, "Interrupted")
					unlock(&l.lock)
					l.stop()
					return
				}
			} else {
				sgoCancel(s, sgoCause{kind: sgoCauseDeadline})
			}
			lock(&l.lock)
		}
		if l.finished {
			unlock(&l.lock)
			return
		}
		p := l.decide()
		if p == nil {
			unlock(&l.lock)
			l.stop()
			return
		}
		if gp := l.transfer(p); gp != nil {
			unlock(&l.lock)
			goready(gp, 0)
			lock(&l.lock)
		}
		l.waitBaton(d)
	}
}
