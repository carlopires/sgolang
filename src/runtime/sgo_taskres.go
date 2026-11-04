// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Results of value-returning tasks (D161, D168; sgo.Go, sgo.Task[T]).
//
// A result record holds the child's Result[T] once it ends. Every reader gets its own native
// channel of capacity 1 (Task.Result, Task.Wait), filled when the result is set, or at once if it
// already is; the value is delivered with the channel's send protocol, so a select parked on it is
// woken as by any sender. Nothing is allocated under r.lock (W12 R9): channels, list nodes, and
// boxes are made before it is taken.

package runtime

import "unsafe"

type sgoTaskRes struct {
	lock    mutex
	elem    *_type
	done    bool
	val     unsafe.Pointer // the result, boxed, once done
	waiters *sgoTaskResWaiter
}

type sgoTaskResWaiter struct {
	c    *sgoChan
	next *sgoTaskResWaiter
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
	lock(&r.lock)
	if r.done {
		unlock(&r.lock)
		return
	}
	r.done, r.val = true, box
	ws := r.waiters
	r.waiters = nil
	unlock(&r.lock)
	for w := ws; w != nil; w = w.next {
		sgoTaskResPut(w.c, box)
	}
}

// sgo_taskResChan returns a new channel that receives the result.
//
//go:linkname sgo_taskResChan sgo.runtime_taskResChan
func sgo_taskResChan(rp unsafe.Pointer) unsafe.Pointer {
	r := (*sgoTaskRes)(rp)
	c := &sgoChan{cap: 1, buf: sgoChanBuf(1), elem: r.elem, native: true}
	w := &sgoTaskResWaiter{c: c}
	lock(&r.lock)
	if !r.done {
		w.next, r.waiters = r.waiters, w
		unlock(&r.lock)
		return unsafe.Pointer(c)
	}
	val := r.val
	unlock(&r.lock)
	sgoTaskResPut(c, val)
	return unsafe.Pointer(c)
}

// sgoTaskResPut delivers a copy of the boxed result into reader channel c (capacity 1, never sent
// on otherwise, so it never blocks).
func sgoTaskResPut(c *sgoChan, val unsafe.Pointer) {
	box := sgoNativeBox(c, val)
	lock(&c.lock)
	w, _ := sgoNativeTrySend(c, box)
	unlock(&c.lock)
	sgoReadyOne(w)
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
