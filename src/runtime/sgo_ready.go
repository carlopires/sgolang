// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// n.Start with a readiness handshake (W05, SGO-READY-002 draft; plan W05).
//
// The starter admits the child exactly as Spawn does, then waits for the first of: the child's
// ready.Signal(), the child's end, or cancellation of a scope linked to the starter. One CAS on
// the record's state is the terminal decision; whoever wins it reports the outcome, and the
// others have no effect on it. The child is a registered child of its nursery in every case.
//
// Locks: the decision and the starter's registration are made under sgoSleepLock (the
// cancellation registry lock shared with Sleep and channels). The starter registers and parks
// releasing it, so a signaller, the exiting child, or the canceller never readies a starter that is
// not parked yet; a waker readies the starter only if it is registered.

package runtime

import (
	"internal/runtime/atomic"
	"unsafe"
)

const (
	sgoReadyWaiting   uint32 = iota // no decision yet
	sgoReadySignalled               // ready.Signal() won: Start returns nil
	sgoReadyEnded                   // the child ended first: Start returns r.err
	sgoReadyCancelled               // cancellation of the starter won: Start returns a cancellation error
)

type sgoReady struct {
	starter    *sgoTask
	child      *sgoTask // set by sgoAdmit before the child can run
	g          *g       // the parked starter, while registered
	state      atomic.Uint32
	signals    int   // Signal calls by the child (touched only by the child)
	err        error // sgoReadyEnded: the error Start returns
	registered bool  // the starter is parked in sgoReadyWaiters (under sgoSleepLock)
	prev, next *sgoReady
	wake       *sgoReady
}

// sgoReadyWaiters are parked starters (under sgoSleepLock), scanned by sgoWakeCancelled.
var sgoReadyWaiters *sgoReady

// sgoStart is n.Start(f): it admits fn as a child of s and waits for its readiness decision.
func sgoStart(s *sgoScope, fn func(*sgoTask) error) error {
	t := sgoCurrent()
	r := &sgoReady{starter: t}
	c, err := sgoAdmit(s, fn, r)
	if err != nil {
		// A refused admission is a SpawnContractViolation that unwinds the caller, as for Spawn.
		panic(err)
	}
	if c == nil {
		// The scope is CANCELLING: no child was created (SGO-CANCEL-003). s is the caller's
		// current scope, so the caller's cancellation query reports it.
		if err := sgoTaskCancellation(t); err != nil {
			return err
		}
		return &sgoReadyErr{"start into a cancelled nursery: no child was created"}
	}
	lock(&sgoSleepLock)
	if r.state.Load() == sgoReadyWaiting {
		if sgoOuterCancelled(t) != nil {
			r.state.CompareAndSwap(sgoReadyWaiting, sgoReadyCancelled)
		} else {
			r.g, r.registered = getg(), true
			r.next = sgoReadyWaiters
			if sgoReadyWaiters != nil {
				sgoReadyWaiters.prev = r
			}
			sgoReadyWaiters = r
			gopark(sgoReadyPark, unsafe.Pointer(r), waitReasonSGoJoin, traceBlockSync, 1)
			lock(&sgoSleepLock)
			sgoReadyUnregister(r)
		}
	}
	unlock(&sgoSleepLock)
	switch r.state.Load() {
	case sgoReadySignalled:
		return nil
	case sgoReadyEnded:
		return r.err
	case sgoReadyCancelled:
		return sgoTaskCancellation(t)
	}
	throw("sgo: readiness wait ended without a decision")
	return nil
}

func sgoReadyPark(gp *g, p unsafe.Pointer) bool {
	unlock(&sgoSleepLock)
	return true
}

func sgoReadyUnregister(r *sgoReady) {
	if !r.registered {
		return
	}
	if r.prev != nil {
		r.prev.next = r.next
	} else if sgoReadyWaiters == r {
		sgoReadyWaiters = r.next
	}
	if r.next != nil {
		r.next.prev = r.prev
	}
	r.prev, r.next, r.registered = nil, nil, false
}

// sgoReadyDecide makes decision d if no decision was made, and readies the starter if it is parked.
func sgoReadyDecide(r *sgoReady, d uint32) bool {
	lock(&sgoSleepLock)
	won := r.state.CompareAndSwap(sgoReadyWaiting, d)
	var gp *g
	if won && r.registered {
		gp = r.g
		sgoReadyUnregister(r)
	}
	unlock(&sgoSleepLock)
	if gp != nil {
		goready(gp, 0)
	}
	return won
}

// sgoReadySignal is ready.Signal() (SGO-READY-002): only the started child may signal, once.
func sgoReadySignal(r *sgoReady) {
	t := sgoCurrent()
	switch {
	case r == nil:
		panic(&sgoViolation{"ready signal without a readiness handle"})
	case t != r.child:
		panic(&sgoViolation{"ready signal by a task that was not started with this readiness handle"})
	}
	r.signals++
	if r.signals > 1 {
		panic(&sgoViolation{"ready signalled twice"})
	}
	// A Signal after the starter was released by cancellation is accepted and has no effect.
	sgoReadyDecide(r, sgoReadySignalled)
}

// sgoReadyChildEnded runs when a started child publishes its outcome (sgoTaskExit): if it never
// signalled, Start returns the child's error, or an error naming the child for any other outcome.
func sgoReadyChildEnded(c *sgoTask) {
	r := c.ready
	if r.state.Load() != sgoReadyWaiting {
		return
	}
	var err error
	if c.outcome == sgoErrored || c.outcome == sgoCancelled {
		err = c.err
	}
	if err == nil {
		err = &sgoReadyErr{"child " + sgoItoa(c.id) + " ended before ready"}
	}
	r.err = err
	sgoReadyDecide(r, sgoReadyEnded)
}

// sgoReadyErr is the error of Start whose child ended before signalling without a returned error.
type sgoReadyErr struct{ msg string }

func (e *sgoReadyErr) Error() string { return e.msg }

// sgoReadyWakeCancelled claims every parked starter that is now cancelled (called by
// sgoWakeCancelled under sgoSleepLock) and returns them as a wake list.
func sgoReadyWakeCancelled() *sgoReady {
	var wake *sgoReady
	for r := sgoReadyWaiters; r != nil; {
		next := r.next
		if sgoOuterCancelled(r.starter) != nil && r.state.CompareAndSwap(sgoReadyWaiting, sgoReadyCancelled) {
			sgoReadyUnregister(r)
			r.wake, wake = wake, r
		}
		r = next
	}
	return wake
}

func sgoReadyReadyAll(wake *sgoReady) {
	for wake != nil {
		r := wake
		wake, r.wake = r.wake, nil
		goready(r.g, 0)
	}
}

// Package sgo entry points (src/sgo/ready.go).

//go:linkname sgo_start sgo.runtime_start
func sgo_start(scope unsafe.Pointer, fn func(task unsafe.Pointer) error) error {
	// Same ABI as func(*sgoTask) error (see sgo_spawn); the child reaches its readiness record
	// through its task (runtime_taskReady).
	return sgoStart((*sgoScope)(scope), *(*func(*sgoTask) error)(unsafe.Pointer(&fn)))
}

//go:linkname sgo_readySignal sgo.runtime_readySignal
func sgo_readySignal(ready unsafe.Pointer) {
	sgoReadySignal((*sgoReady)(ready))
}

//go:linkname sgo_taskReady sgo.runtime_taskReady
func sgo_taskReady(task unsafe.Pointer) unsafe.Pointer {
	return unsafe.Pointer((*sgoTask)(task).ready)
}
