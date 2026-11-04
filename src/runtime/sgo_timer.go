// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Scope timers: n.NewTimer(ms), t.NewTimer(ms) (D171, SGO-WAIT-004 draft).
//
// A scope timer is the timer channel of t.After (sgo_after.go) kept as a resource of its scope:
// its handle (kind sgoHTimer) is in the scope's list, so the scope disarms it when it ends
// (sgoHandleFree). The channel is marked owned, so a select using it does not drop it when the
// select ends, and any number of selects may use it. Reset re-arms it from now with a new value
// box; Stop disarms it (fired = true: no arm proceeds on it until a Reset).
//
// Re-arming in production: the deadline and the fired flag change under c.lock, and the runtime
// timer is reset after c.lock is released; a fire left from the previous arming finds the new
// deadline in the future and does nothing (sgoAfterFire). In the lab the timer's entry is
// replaced under l.lock: only the baton holder runs S-Go operations.

package runtime

import "unsafe"

const sgoHTimer = 8 // a scope timer (D171)

// sgo_timerNew is s.NewTimer(ms) after the argument checks: a timer of scope (task, nurs) armed
// for ms milliseconds from now. It returns the handle and the channel.
//
//go:linkname sgo_timerNew sgo.runtime_timerNew
func sgo_timerNew(task, nurs unsafe.Pointer, ms int) (unsafe.Pointer, unsafe.Pointer) {
	n, b := sgoScopeOf(task, nurs)
	l := sgoResTarget(n, b, "NewTimer")
	var zero any = 0
	c := &sgoChan{cap: 1, buf: sgoChanBuf(1), elem: efaceOf(&zero)._type, native: true}
	c.tmr = &sgoAfter{owned: true}
	if sgoLab == nil {
		c.tmr.timer = new(timer)
		c.tmr.timer.init(sgoAfterFire, c)
	}
	h := &sgoHandle{kind: sgoHTimer, fd: -1, name: "timer", tmr: c, nurs: n, body: b}
	l.push(h)
	sgoTimerArm(c, ms)
	return unsafe.Pointer(h), unsafe.Pointer(c)
}

// sgo_timerReset is tm.Reset(ms): the timer is re-armed for ms milliseconds from now, whether it
// had fired, been stopped, or was still armed.
//
//go:linkname sgo_timerReset sgo.runtime_timerReset
func sgo_timerReset(hp unsafe.Pointer, ms int) {
	h := sgoTimerUse(hp, "Timer.Reset")
	sgoTimerArm(h.tmr, ms)
}

// sgo_timerStop is tm.Stop(): no arm on the timer proceeds until a Reset.
//
//go:linkname sgo_timerStop sgo.runtime_timerStop
func sgo_timerStop(hp unsafe.Pointer) {
	h := sgoTimerUse(hp, "Timer.Stop")
	sgoTimerDisarm(h.tmr)
}

// sgo_timerCheck checks a use of tm.C().
//
//go:linkname sgo_timerCheck sgo.runtime_timerCheck
func sgo_timerCheck(hp unsafe.Pointer) {
	sgoTimerUse(hp, "Timer.C")
}

// sgoTimerUse checks that the current task runs inside the timer's scope and that the scope has
// not ended (the flow rule makes the latter unreachable from admitted code).
func sgoTimerUse(hp unsafe.Pointer, op string) *sgoHandle {
	h := (*sgoHandle)(hp)
	if !sgoInside(sgoCurrent(), h.nurs, h.body) {
		panic(error(&sgoFailure{"MissingCapability", op + ": the task does not run inside the timer's scope", ""}))
	}
	if h.closed {
		panic(error(&sgoFailure{"HandleClosed", op + ": the timer's scope has ended", ""}))
	}
	return h
}

// sgoTimerArm arms scope timer c for ms milliseconds from now, delivering ms.
func sgoTimerArm(c *sgoChan, ms int) {
	a := c.tmr
	v := ms
	box := sgoNativeBox(c, unsafe.Pointer(&v)) // allocated before any lock is taken
	if l := sgoLab; l != nil {
		l.reserve()
		lock(&l.lock)
		sgoLabTimerDrop(l, c)
		lock(&c.lock)
		a.box, a.fired, a.when = box, false, l.now+int64(ms)
		unlock(&c.lock)
		if cap(l.timers) == len(l.timers) {
			throw("sgo: lab timer capacity not reserved")
		}
		l.timers = append(l.timers, sgoLabTimer{deadline: a.when, id: l.nextTmr, t: sgoCurrent(), ch: c}) // lock-alloc: ok (capacity reserved)
		l.nextTmr++
		unlock(&l.lock)
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
	lock(&c.lock)
	a.box, a.fired, a.when = box, false, when
	unlock(&c.lock)
	a.timer.reset(when, 0)
}

// sgoTimerDisarm stops scope timer c: by Stop, or when its scope ends.
func sgoTimerDisarm(c *sgoChan) {
	a := c.tmr
	if l := sgoLab; l != nil {
		lock(&l.lock)
		sgoLabTimerDrop(l, c)
		lock(&c.lock)
		a.fired = true
		unlock(&c.lock)
		unlock(&l.lock)
		return
	}
	lock(&c.lock)
	a.fired = true
	unlock(&c.lock)
	a.timer.stop()
}

// sgoLabTimerDrop removes timer channel c's pending entry, with l.lock held.
func sgoLabTimerDrop(l *sgoLabState, c *sgoChan) {
	n := 0
	for _, tm := range l.timers {
		if tm.ch != c {
			l.timers[n] = tm
			n++
		}
	}
	clear(l.timers[n:])
	l.timers = l.timers[:n]
}
