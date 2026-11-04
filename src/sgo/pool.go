// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgo

import "unsafe"

// Pools with leases (D175; draft SGO-SPAWN-009). Implemented in the runtime (sgo_pool.go) on the
// unbounded native queue of groups (sgo_group.go).
func runtime_poolPut(scope, g, v unsafe.Pointer)
func runtime_poolAcquire(scope, g, task, nurs, dst unsafe.Pointer)
func runtime_poolSpend(flag *uint32) bool
func runtime_poolSpent(flag *uint32) bool

// Pool is a pool of items of type T that lives in a nursery (D175). Items are used exclusively,
// through leases: Acquire waits for an idle item, and the lease puts it back when its scope ends.
// The handle is shareable; it is used only by tasks inside its nursery (else MissingCapability).
// When T holds resources the pool is tied to its nursery like them, and every item must be tied at
// least as long (Put is checked as a send).
type Pool[T any] struct {
	n *Nursery
	q unsafe.Pointer
}

// Lease is exclusive use of one item of a pool, a resource of the scope that acquired it
// (SGO-RES-001). When the scope ends, after its tasks have ended, the item goes back to the pool
// (to the oldest waiting Acquire, or idle) unless the lease was returned or dropped first. It goes
// back also when the scope unwinds (a failure, a cancellation, an error exit): the state of a
// transaction over the item is the transaction's own resource, which sees Release(false). Drop an
// item that may be broken before the scope ends.
type Lease[T any] struct {
	p    *Pool[T]
	item poolItem[T]
	done uint32 // 1 once returned, dropped, or released (runtime_poolSpend)
}

// poolItem is the element of a pool's queue: a struct, so its type descriptor is concrete even
// when T is an interface type.
type poolItem[T any] struct{ v T }

func itemType[T any]() unsafe.Pointer {
	var e any = poolItem[T]{}
	return (*[2]unsafe.Pointer)(unsafe.Pointer(&e))[0]
}

// NewPool returns an empty pool that lives in nursery n.
func NewPool[T any](n *Nursery) *Pool[T] {
	if n == nil {
		runtime_violation("NewPool with a nil nursery handle")
	}
	return &Pool[T]{n: n, q: runtime_groupNew(itemType[T](), 0)}
}

// Put adds v to the pool as an idle item, or hands it to the oldest waiting Acquire. It never
// waits.
func (p *Pool[T]) Put(v T) {
	it := poolItem[T]{v: v}
	runtime_poolPut(p.n.s, p.q, unsafe.Pointer(&it))
}

// Acquire waits for an idle item and returns a lease on it, a resource of scope s, which must be
// the pool's nursery or lie inside it. Waiters are served oldest first. The wait is a cancellation
// point: a cancelled waiter unwinds without taking an item.
func (p *Pool[T]) Acquire(s Scope) *Lease[T] {
	task, nurs := s.sgoScope()
	l := &Lease[T]{p: p}
	runtime_poolAcquire(p.n.s, p.q, task, nurs, unsafe.Pointer(&l.item))
	runtime_resOwn(task, nurs, unsafe.Pointer(l), func(bool) error {
		if runtime_poolSpend(&l.done) {
			runtime_poolPut(p.n.s, p.q, unsafe.Pointer(&l.item))
		}
		return nil
	})
	return l
}

// Value returns the leased item.
func (l *Lease[T]) Value() T {
	if runtime_poolSpent(&l.done) {
		runtime_violation("Lease.Value after Return or Drop")
	}
	return l.item.v
}

// Return puts the item back now; the lease is spent.
func (l *Lease[T]) Return() {
	l.spend("Return")
	runtime_poolPut(l.p.n.s, l.p.q, unsafe.Pointer(&l.item))
}

// Drop keeps the item out of the pool for good (an item found broken); the lease is spent. The
// item is still released by the scope that owns it.
func (l *Lease[T]) Drop() {
	l.spend("Drop")
}

func (l *Lease[T]) spend(op string) {
	if !runtime_poolSpend(&l.done) {
		runtime_violation("Lease." + op + " after Return or Drop")
	}
}
