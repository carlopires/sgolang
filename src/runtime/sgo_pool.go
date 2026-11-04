// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Pools with leases (D175; sgo.NewPool, src/sgo/pool.go; model sgo/model/poolmodel.go).
//
// A pool's idle items are the queue of a group without a limit (sgo_group.go): Put publishes,
// which never waits and hands the item to the oldest waiting receiver, if any; Acquire is a native
// receive, so waiters are served oldest first and a cancelled waiter unwinds without taking an
// item. A lease is a resource registered by package sgo (runtime_resOwn) whose release puts the
// item back. Both entry points check that the caller runs inside the pool's nursery, and Acquire
// that the lease's scope lies within it, so no lease outlives the items' owner.

package runtime

import (
	"internal/runtime/atomic"
	"unsafe"
)

// sgoPoolInside fails the current task unless it runs inside the pool's nursery.
func sgoPoolInside(scope unsafe.Pointer, op string) {
	if !sgoInside(sgoCurrent(), (*sgoScope)(scope), nil) {
		panic(error(&sgoFailure{"MissingCapability", op + ": the task does not run inside the pool's nursery", ""}))
	}
}

//go:linkname sgo_poolPut sgo.runtime_poolPut
func sgo_poolPut(scope, g, v unsafe.Pointer) {
	sgoPoolInside(scope, "Put")
	sgo_groupPublish(g, v)
}

// sgo_poolAcquire waits for an idle item and stores it in *dst; the lease will be a resource of
// scope (task, nurs), which must be the pool's nursery or lie inside it.
//
//go:linkname sgo_poolAcquire sgo.runtime_poolAcquire
func sgo_poolAcquire(scope, g, task, nurs, dst unsafe.Pointer) {
	sgoPoolInside(scope, "Acquire")
	ln, lb := sgoScopeOf(task, nurs)
	if !sgoEncloses((*sgoScope)(scope), nil, ln, lb) {
		panic(error(&sgoFailure{"MissingCapability", "Acquire: the lease's scope does not lie inside the pool's nursery", ""}))
	}
	sgochanrecv1((*sgoGroup)(g).res, dst)
}

// sgo_poolSpend marks a lease spent (*flag from 0 to 1) and reports whether this call did: Return,
// Drop, and the lease's release race only through it, so an item goes back at most once.
//
//go:linkname sgo_poolSpend sgo.runtime_poolSpend
func sgo_poolSpend(flag *uint32) bool {
	return atomic.Cas(flag, 0, 1)
}

//go:linkname sgo_poolSpent sgo.runtime_poolSpent
func sgo_poolSpent(flag *uint32) bool {
	return atomic.Load(flag) != 0
}
