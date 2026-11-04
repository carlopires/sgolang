// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Timers as select arms: t.After(ms) (D160, SGO-WAIT-003 draft).
//
// A timer is a native receive-only channel of capacity 1 with an attached deadline. It is never
// sent on: an arm on it can proceed once the deadline has passed (sgoAfterDue), and taking the
// value marks it fired. A select (or receive) parked on it is woken by the deadline: the
// production timer callback, or in the lab the virtual timer fired at an advance (D45), claims
// the waiting receiver exactly as a sender would. The compiler admits After only as the operand
// of a select receive arm, so the channel is used by one select of the task that made it; that
// select drops the timer when it ends, by any arm or by cancellation, and the timer then costs
// nothing (D160).
//
// Locks: the deadline state is under the channel's lock in production. The lab fires with l.lock
// held and without c.lock (fireAfter): only the task holding the baton runs S-Go operations.

package runtime

import "unsafe"

type sgoAfter struct {
	when  int64          // deadline: monotonic ns (production) or virtual ms (lab)
	box   unsafe.Pointer // the delivered value (the duration), boxed at creation
	fired bool           // under c.lock: the value was taken or delivered
	timer *timer         // production only
}

// sgo_after is task.After(ms) after the argument and ownership checks.
//
//go:linkname sgo_after sgo.runtime_after
func sgo_after(task unsafe.Pointer, ms int) unsafe.Pointer {
	t := (*sgoTask)(task)
	var zero any = 0
	c := &sgoChan{cap: 1, buf: sgoChanBuf(1), elem: efaceOf(&zero)._type, native: true}
	v := ms
	a := &sgoAfter{box: sgoNativeBox(c, unsafe.Pointer(&v))}
	c.tmr = a
	if l := sgoLab; l != nil {
		l.reserve() // no runtime lock is held here
		lock(&l.lock)
		a.when = l.now + int64(ms)
		if cap(l.timers) == len(l.timers) {
			throw("sgo: lab timer capacity not reserved")
		}
		l.timers = append(l.timers, sgoLabTimer{deadline: a.when, id: l.nextTmr, t: t, ch: c}) // lock-alloc: ok (capacity reserved)
		l.nextTmr++
		unlock(&l.lock)
		return unsafe.Pointer(c)
	}
	ns := int64(ms) * 1_000_000
	if ns/1_000_000 != int64(ms) {
		ns = maxWhen
	}
	a.when = nanotime() + ns
	if a.when < 0 {
		a.when = maxWhen
	}
	a.timer = new(timer)
	a.timer.init(sgoAfterFire, c)
	a.timer.reset(a.when, 0)
	return unsafe.Pointer(c)
}

// sgoAfterDue reports, under c.lock, whether the timer's value can be taken now.
func sgoAfterDue(a *sgoAfter) bool {
	if a.fired {
		return false
	}
	if l := sgoLab; l != nil {
		return l.now >= a.when
	}
	return nanotime() >= a.when
}

// sgoAfterDeliver hands the value to a waiting receiver, under c.lock. It returns the receiver.
func sgoAfterDeliver(c *sgoChan) *sgoChanWait {
	a := c.tmr
	if a.fired {
		return nil
	}
	for r := c.recvs.first; r != nil; r = r.next {
		if r.claim(sgoWaitDone) {
			c.recvs.remove(r)
			r.val, r.delivered = a.box, true
			a.fired = true
			return r
		}
	}
	return nil // no receiver parked yet: the arm sees the deadline as due when it checks
}

// sgoAfterFire is the production timer function of one After call.
func sgoAfterFire(arg any, _ uintptr, _ int64) {
	c := arg.(*sgoChan)
	lock(&c.lock)
	w := sgoAfterDeliver(c)
	unlock(&c.lock)
	sgoReadyOne(w)
}

// fireAfter is the lab's virtual fire of timer channel c at an advance, with l.lock held. It does
// not take c.lock: the fire runs on the baton holder, and when that is a select parking on c it
// already holds c.lock (sgoLabBlock is called before the park releases the arm locks); otherwise
// the lock is free and no other goroutine runs S-Go operations.
func (l *sgoLabState) fireAfter(c *sgoChan) {
	w := sgoAfterDeliver(c)
	if w != nil {
		if w.sel != nil {
			one := [1]int32{w.arm}
			l.emit(sgoLabEvent{kind: sgoLabSelect, task: w.task.labID, set: l.addSet(one[:]), pick: w.arm})
		}
		l.wakeLocked(w.task)
	}
}

// sgoAfterStop drops timer channel c when the select (or receive) using it ends.
func sgoAfterStop(c *sgoChan) {
	if l := sgoLab; l != nil {
		lock(&l.lock)
		n := 0
		for _, tm := range l.timers {
			if tm.ch != c {
				l.timers[n] = tm
				n++
			}
		}
		clear(l.timers[n:])
		l.timers = l.timers[:n]
		unlock(&l.lock)
		return
	}
	if t := c.tmr.timer; t != nil {
		t.stop()
	}
}

// sgoSelHasTimer reports whether a select has a timer receive arm.
func sgoSelHasTimer(cas0 *sgoScase, nsends, nrecvs int) bool {
	if nrecvs == 0 {
		return false
	}
	cas := unsafe.Slice(cas0, nsends+nrecvs)
	for i := nsends; i < len(cas); i++ {
		if cas[i].c != nil && cas[i].c.tmr != nil {
			return true
		}
	}
	return false
}

// sgoSelStopTimers drops the timers of a select's receive arms (deferred by sgoselectgoTimers).
func sgoSelStopTimers(cas0 *sgoScase, nsends, nrecvs int) {
	cas := unsafe.Slice(cas0, nsends+nrecvs)
	for i := nsends; i < len(cas); i++ {
		if cas[i].c != nil && cas[i].c.tmr != nil {
			sgoAfterStop(cas[i].c)
		}
	}
}
