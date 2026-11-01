// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgo

import "unsafe"

// Implemented in the runtime (sgo_sync.go).
func runtime_eventNew() unsafe.Pointer
func runtime_eventSet(e unsafe.Pointer)
func runtime_eventWait(e, task unsafe.Pointer) error
func runtime_eventIsSet(e unsafe.Pointer) bool
func runtime_limiterNew(n int) unsafe.Pointer
func runtime_limiterAcquire(l, task unsafe.Pointer) (unsafe.Pointer, error)
func runtime_tokenRelease(k unsafe.Pointer)

// Event is a one-shot signal (D39): Set wakes every waiter, and stays set.
type Event struct{ e unsafe.Pointer }

// NewEvent returns an event that is not set.
func NewEvent() *Event { return &Event{runtime_eventNew()} }

// Set sets the event; setting a set event has no effect. Set happens before every Wait that
// returns nil.
func (ev *Event) Set() { runtime_eventSet(ev.e) }

// Wait returns nil once the event is set, or a cancellation error if the task is cancelled while
// it waits (a cancellation point).
func (ev *Event) Wait(c *Context) error {
	c.own()
	return runtime_eventWait(ev.e, c.t)
}

// IsSet reports whether the event is set.
func (ev *Event) IsSet() bool { return runtime_eventIsSet(ev.e) }

// CapacityLimiter bounds how many tasks hold one of its n slots at once (D39).
type CapacityLimiter struct{ l unsafe.Pointer }

// Token is one held slot of a limiter. It belongs to the task that acquired it.
type Token struct{ k unsafe.Pointer }

// NewCapacityLimiter returns a limiter of n >= 1 slots.
func NewCapacityLimiter(n int) *CapacityLimiter {
	if n < 1 {
		runtime_fail("InvalidArgument", "NewCapacityLimiter with fewer than one slot")
	}
	return &CapacityLimiter{runtime_limiterNew(n)}
}

// Acquire waits, in FIFO order, for a free slot and returns its token, or a cancellation error if
// the task is cancelled while it waits (a cancellation point). A token that is not released is
// released at the end of the enclosing nursery body or task, with a TokenAbandoned diagnostic.
func (lim *CapacityLimiter) Acquire(c *Context) (*Token, error) {
	c.own()
	k, err := runtime_limiterAcquire(lim.l, c.t)
	if err != nil {
		return nil, err
	}
	return &Token{k}, nil
}

// Release returns the slot. Releasing twice, or releasing another task's token, is a
// SpawnContractViolation.
func (tok *Token) Release() {
	if tok == nil {
		runtime_violation("release of a token that was never acquired")
	}
	runtime_tokenRelease(tok.k)
}
