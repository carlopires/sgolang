// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Scope timers (D171, SGO-WAIT-004 draft).

package sgo

import "unsafe"

func runtime_timerNew(task, nurs unsafe.Pointer, ms int) (unsafe.Pointer, <-chan int) // sgo_timer.go
func runtime_timerReset(h unsafe.Pointer, ms int)
func runtime_timerStop(h unsafe.Pointer)
func runtime_timerCheck(h unsafe.Pointer)

// Timer is a timer owned by a scope (D171): a resource of the scope that made it, disarmed when
// that scope ends. Unlike t.After, it is kept across selects, so a delay can be measured from a
// fixed start (an RFC 8305 attempt delay) while other arms come and go. It needs no clock grant,
// as t.After: it reveals elapsed time only by the order in which arms proceed. Every task inside
// the timer's scope may use it.
type Timer struct {
	h unsafe.Pointer
	c <-chan int
}

// NewTimer returns a timer of the task's body, armed for ms milliseconds from now.
func (c *Context) NewTimer(ms int) *Timer {
	c.own()
	return newTimer(c, ms)
}

// NewTimer returns a timer of nursery n, armed for ms milliseconds from now.
func (n *Nursery) NewTimer(ms int) *Timer { return newTimer(n, ms) }

func newTimer(s Scope, ms int) *Timer {
	if ms < 0 {
		runtime_fail("InvalidArgument", "NewTimer with a negative duration")
	}
	task, nurs := s.sgoScope()
	h, ch := runtime_timerNew(task, nurs, ms)
	return &Timer{h, ch}
}

// C returns the timer's channel, for case <-tm.C(): in any number of selects. An arm on it can
// proceed once the timer's deadline has passed (virtual time in the lab), once per arming; it
// receives the duration the timer was last armed with.
func (tm *Timer) C() <-chan int {
	runtime_timerCheck(tm.h)
	return tm.c
}

// Reset re-arms the timer for ms milliseconds from now, whether it had fired, been stopped, or
// was still armed.
func (tm *Timer) Reset(ms int) {
	if ms < 0 {
		runtime_fail("InvalidArgument", "Timer.Reset with a negative duration")
	}
	runtime_timerReset(tm.h, ms)
}

// Stop disarms the timer: no arm on it proceeds until a Reset.
func (tm *Timer) Stop() { runtime_timerStop(tm.h) }

// MoveTo makes tm a resource of s, a scope enclosing tm's scope (SGO-RES-003).
func (tm *Timer) MoveTo(s Scope) error { return moveTo(tm.h, s) }
