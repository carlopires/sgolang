// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Results of value-returning tasks (D161, D168; sgo.Go, sgo.Task[T]).
//
// Every Result() and Wait call gets its own native channel that delivers the task's result once
// (edge-triggered, W22 review G2 and H1): a select loop with an arm on one such channel takes it
// once and then waits on its other arms. A call registers nothing: a result channel is listed on
// its task only while a receiver is parked on it (a plain receive or a select arm), so a channel
// that was abandoned is garbage, and the list is bounded by the receivers parked now. Publication
// sets the result, detaches the list, and delivers to one parked receiver of each listed channel.
// A receiver that finds the result published when it registers takes it without parking.
// Nothing is allocated under r.lock or c.lock (W12 R9).

package runtime

import (
	"internal/runtime/atomic"
	"unsafe"
)

type sgoTaskRes struct {
	lock   mutex
	elem   *_type
	done   atomic.Bool
	val    unsafe.Pointer // the boxed result, set before done
	parked *sgoChan       // result channels with a parked receiver (sgoTaskResLink.prev/next)
}

// sgoTaskResLink is a result channel's link to its task (sgoChan.res).
type sgoTaskResLink struct {
	r          *sgoTaskRes
	taken      bool  // the channel delivered the result (under c.lock)
	waiters    int32 // receivers parked on the channel (under r.lock)
	listed     bool  // on r.parked (under r.lock)
	prev, next *sgoChan
}

//go:linkname sgo_taskResNew sgo.runtime_taskResNew
func sgo_taskResNew(elem unsafe.Pointer) unsafe.Pointer {
	return unsafe.Pointer(&sgoTaskRes{elem: (*_type)(elem)})
}

// sgo_taskResSet publishes the result *v once; later calls have no effect.
//
//go:linkname sgo_taskResSet sgo.runtime_taskResSet
func sgo_taskResSet(rp, v unsafe.Pointer) {
	r := (*sgoTaskRes)(rp)
	box := mallocgc(r.elem.Size_, r.elem, true)
	typedmemmove(r.elem, box, v)
	if raceenabled {
		racereleasemerge(box)
	}
	lock(&r.lock)
	if r.done.Load() {
		unlock(&r.lock)
		return
	}
	r.val = box
	r.done.Store(true)
	list := r.parked
	r.parked = nil
	for c := list; c != nil; c = c.res.next {
		c.res.listed = false // a receiver leaving now does not unlink it: the list is the publisher's
	}
	unlock(&r.lock)
	for c := list; c != nil; c = c.res.next {
		lock(&c.lock)
		w := sgoTaskResDeliver(c)
		unlock(&c.lock)
		sgoReadyOne(w)
	}
}

// sgoTaskResDeliver gives the published result to one parked receiver of result channel c, under
// c.lock, unless c already delivered it.
func sgoTaskResDeliver(c *sgoChan) *sgoChanWait {
	if c.res.taken {
		return nil
	}
	for w := c.recvs.first; w != nil; w = w.next {
		if w.claim(sgoWaitDone) {
			c.recvs.remove(w)
			w.val, w.delivered = c.res.r.val, true
			c.res.taken = true
			sgoLabSelectWoken(w)
			return w
		}
	}
	return nil // every parked receiver is a select won elsewhere: a later receive takes it
}

// sgoTaskResRegister lists result channel c for a receiver about to park, under c.lock; it
// reports false, listing nothing, when the result is published and c has not delivered it (the
// receiver takes it instead of parking). A channel that delivered it is never ready again: its
// receiver parks unlisted, woken only by its other arms or by cancellation.
func sgoTaskResRegister(c *sgoChan) bool {
	l := c.res
	r := l.r
	lock(&r.lock)
	if r.done.Load() {
		parks := l.taken
		if parks {
			l.waiters++ // unlisted: sgoTaskResUnregister only counts it down
		}
		unlock(&r.lock)
		return parks
	}
	l.waiters++
	if !l.listed {
		l.listed, l.prev, l.next = true, nil, r.parked
		if r.parked != nil {
			r.parked.res.prev = c
		}
		r.parked = c
	}
	unlock(&r.lock)
	return true
}

// sgoTaskResUnregister undoes one sgoTaskResRegister when the receiver's wait ends.
func sgoTaskResUnregister(c *sgoChan) {
	l := c.res
	r := l.r
	lock(&r.lock)
	l.waiters--
	if l.waiters == 0 && l.listed {
		if l.prev != nil {
			l.prev.res.next = l.next
		} else {
			r.parked = l.next
		}
		if l.next != nil {
			l.next.res.prev = l.prev
		}
		l.listed, l.prev, l.next = false, nil, nil
	}
	unlock(&r.lock)
}

// sgo_taskResChan returns a new result channel of the task: it delivers the result once.
//
//go:linkname sgo_taskResChan sgo.runtime_taskResChan
func sgo_taskResChan(rp unsafe.Pointer) unsafe.Pointer {
	r := (*sgoTaskRes)(rp)
	return unsafe.Pointer(&sgoChan{elem: r.elem, native: true, res: &sgoTaskResLink{r: r}})
}

// sgoTaskResParked counts the result channels listed on r (tests).
func sgoTaskResParked(r *sgoTaskRes) int {
	lock(&r.lock)
	n := 0
	for c := r.parked; c != nil; c = c.res.next {
		n++
	}
	unlock(&r.lock)
	return n
}

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
