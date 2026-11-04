// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Results of value-returning tasks (D161, D168; sgo.Go, sgo.Task[T]).
//
// A result record is one native channel that the runtime latches when the child ends (W22 review
// G2): the result is boxed once and stored as c.latch, and from then on every receive, from any
// reader, any number of times, gets it without consuming it, and every select arm on the channel
// can proceed (level-triggered). Task.Result and Task.Wait all use this one channel, so a call
// registers nothing; receivers parked before publication are all woken with the value, as close
// wakes every waiter. The box is made before c.lock is taken (W12 R9).

package runtime

import "unsafe"

//go:linkname sgo_taskResNew sgo.runtime_taskResNew
func sgo_taskResNew(elem unsafe.Pointer) unsafe.Pointer {
	return unsafe.Pointer(&sgoChan{elem: (*_type)(elem), native: true})
}

// sgo_taskResSet publishes the result *v once; later calls have no effect.
//
//go:linkname sgo_taskResSet sgo.runtime_taskResSet
func sgo_taskResSet(rp, v unsafe.Pointer) {
	c := (*sgoChan)(rp)
	box := mallocgc(c.elem.Size_, c.elem, true)
	typedmemmove(c.elem, box, v)
	if raceenabled {
		racereleasemerge(box)
	}
	lock(&c.lock)
	if c.latch != nil {
		unlock(&c.lock)
		return
	}
	c.latch = box
	var wake *sgoChanWait
	for r := c.recvs.first; r != nil; {
		next := r.next
		if r.claim(sgoWaitDone) {
			c.recvs.remove(r)
			r.val, r.delivered = box, true
			sgoLabSelectWoken(r)
			r.wake, wake = wake, r
		}
		r = next
	}
	unlock(&c.lock)
	sgoChanReady(wake)
}

// sgo_taskResChan returns the record's channel: the same one for every call.
//
//go:linkname sgo_taskResChan sgo.runtime_taskResChan
func sgo_taskResChan(rp unsafe.Pointer) unsafe.Pointer { return rp }

//go:linkname sgo_nativeRecv sgo.runtime_nativeRecv
func sgo_nativeRecv(c, dst unsafe.Pointer) {
	sgochanrecv1((*sgoChan)(c), dst)
}

// sgo_taskEndErr is the error of a child that ends without returning a result: its cancellation,
// or else a failure.
//
//go:linkname sgo_taskEndErr sgo.runtime_taskEndErr
func sgo_taskEndErr(task unsafe.Pointer) error {
	if err := sgoTaskCancellation((*sgoTask)(task)); err != nil {
		return err
	}
	return &sgoFailure{"TaskFailed", "the task ended without a result", ""}
}
