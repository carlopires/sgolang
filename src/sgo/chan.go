// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgo

import "unsafe"

// Managed channels with send permits (SGO-PERMIT-002). The protocol lives in the runtime
// (src/runtime/sgo_chan.go); this file boxes values of T and checks context ownership. The
// compiler admits T only as int, bool, string, or error (D29).

func runtime_chanNew(capacity int) unsafe.Pointer
func runtime_chanReserve(ch, task unsafe.Pointer) (unsafe.Pointer, error)
func runtime_chanCommit(p, v unsafe.Pointer) error
func runtime_chanAbort(p unsafe.Pointer)
func runtime_chanReceive(ch, task unsafe.Pointer) (unsafe.Pointer, bool, error)
func runtime_chanClose(ch unsafe.Pointer)

// Channel is a managed channel of values of type T. It belongs to no nursery.
type Channel[T any] struct{ c unsafe.Pointer }

// SendPermit is a reservation of one send on a Channel; it is resolved by Commit or Abort, only
// by the task that reserved it. Copies share one terminal state.
type SendPermit[T any] struct{ p unsafe.Pointer }

// NewChannel returns a channel of the given capacity; capacity 0 is a rendezvous channel.
func NewChannel[T any](capacity int) *Channel[T] {
	if capacity < 0 {
		runtime_fail("InvalidArgument", "NewChannel with a negative capacity")
	}
	return &Channel[T]{runtime_chanNew(capacity)}
}

// Reserve waits for room to send (a cancellation point) and returns a permit, a cancellation
// error with no reservation, or a closed error if the channel is closed.
func (ch *Channel[T]) Reserve(c *Context) (*SendPermit[T], error) {
	c.own()
	p, err := runtime_chanReserve(ch.c, c.t)
	if err != nil {
		return nil, err
	}
	return &SendPermit[T]{p}, nil
}

// Commit publishes v exactly once and returns nil, or returns a closed error with no effect if
// the channel was closed first.
func (p *SendPermit[T]) Commit(v T) error {
	if p == nil {
		runtime_violation("commit of a permit that was never reserved")
	}
	box := new(T)
	*box = v
	return runtime_chanCommit(p.p, unsafe.Pointer(box))
}

// Abort releases the reservation; aborting a resolved permit has no effect.
func (p *SendPermit[T]) Abort() {
	if p == nil {
		runtime_violation("abort of a permit that was never reserved")
	}
	runtime_chanAbort(p.p)
}

// Receive waits (a cancellation point) and returns the next value with ok = true, or ok = false
// once the channel is closed and empty.
func (ch *Channel[T]) Receive(c *Context) (T, bool, error) {
	c.own()
	var zero T
	v, ok, err := runtime_chanReceive(ch.c, c.t)
	if !ok {
		return zero, false, err
	}
	return *(*T)(v), true, err
}

// Close closes the channel; closing a closed channel has no effect.
func (ch *Channel[T]) Close() {
	runtime_chanClose(ch.c)
}
