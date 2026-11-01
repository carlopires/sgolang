// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Native channels, select, and the cancellation unwind (spec 0.3.0, D32-D35, W07). Native channels
// are managed channels (sgo_chan.go): the same sgoChan, waiter records, lock order (channel locks
// in address order, then sgoSleepLock), and cancellation registry. A native send carries its value
// boxed in its waiter, so reserve and commit are one step that the sender cannot interleave. The
// executable reference model is sgo/model/nativemodel.go; the runtime lane checks this file against
// its outcome sets (sgo/model/testdata/native-outcomes.json).
//
// Compiled S-Go code reaches channels only through the sgo* entry points below (lowered by walk);
// Go's hchan entry points are refused by the compiler for S-Go packages.

package runtime

import (
	"internal/runtime/atomic"
	"unsafe"
)

// sgoCancelUnwind is the internal signal of the cancellation unwind (D33): a native operation that
// waits while its task is cancelled ends with no effect and unwinds the task with this value. It is
// not a failure: deferred calls run, owned nurseries are joined, and the task ends Cancelled, as if
// it had returned err. recover is not part of S-Go, so a program cannot catch it (N08).
type sgoCancelUnwind struct {
	err  error
	fail any // a failure that replaced the unwind (a failed child joined during it)
}

func (u *sgoCancelUnwind) Error() string { return u.err.Error() }

// sgoUnwindingCancel returns the cancellation signal if the calling task's current unwinding started
// as a cancellation unwind.
func sgoUnwindingCancel() *sgoCancelUnwind {
	var first *_panic
	for p := getg()._panic; p != nil; p = p.link {
		if p.goexit {
			break
		}
		first = p
	}
	if first == nil {
		return nil
	}
	u, _ := first.arg.(*sgoCancelUnwind)
	return u
}

// sgoUnwindRetarget gives the oldest record of the current unwinding the origin that sgoPropagate
// left pending (the failed child's), when a joined child's failure replaces a cancellation unwind.
func sgoUnwindRetarget(t *sgoTask) {
	if t.pend.seq == 0 {
		return
	}
	var first *_panic
	for p := getg()._panic; p != nil; p = p.link {
		if p.goexit {
			break
		}
		first = p
	}
	if first != nil {
		first.sgoSeq, first.sgoTask, first.sgoScope = t.pend.seq, t.pend.origin.task, t.pend.origin.scope
	}
	t.pend.seq = 0
}

// sgoLaterFailure is the failure that ends a cancellation unwind as Failed: a failed child joined
// during the unwind, or a deferred call that failed (latest is the newest unwinding value).
func sgoLaterFailure(latest any, u *sgoCancelUnwind) any {
	if u.fail != nil {
		return u.fail
	}
	if l, ok := latest.(*sgoCancelUnwind); !ok || l != u {
		return latest
	}
	return nil
}

// sgoNativeCancel starts the cancellation unwind of the calling task.
func sgoNativeCancel(t *sgoTask) {
	err := sgoTaskCancellation(t)
	if err == nil {
		throw("sgo: cancellation unwind of a task that is not cancelled")
	}
	panic(&sgoCancelUnwind{err: err})
}

// sgoClosedChannel fails the calling task: a native send on a closed channel (D35).
func sgoClosedChannel() {
	panic(error(&sgoFailure{"ClosedChannel", "send on a closed channel", ""}))
}

// sgoSel is the single terminal decision of one blocking native operation or select: the first
// waker to move it from Waiting decides the operation (one CAS across all its arms).
type sgoSel struct {
	state  atomic.Uint32
	winner *sgoChanWait
}

// claim moves w's operation from Waiting to st; the waker that succeeds owns the decision.
func (w *sgoChanWait) claim(st uint32) bool {
	if w.sel == nil {
		return w.state.CompareAndSwap(sgoWaitWaiting, st)
	}
	if w.sel.state.CompareAndSwap(sgoWaitWaiting, st) {
		w.sel.winner = w
		return true
	}
	return false
}

func (w *sgoChanWait) waiting() bool {
	if w.sel == nil {
		return w.state.Load() == sgoWaitWaiting
	}
	return w.sel.state.Load() == sgoWaitWaiting
}

func sgoNativeBox(c *sgoChan, elem unsafe.Pointer) unsafe.Pointer {
	box := mallocgc(c.elem.Size_, c.elem, true)
	typedmemmove(c.elem, box, elem)
	return box
}

// sgoNativeTake copies a received box to dst (nil: discarded).
func sgoNativeTake(c *sgoChan, dst, box unsafe.Pointer) {
	if raceenabled {
		raceacquire(box)
	}
	if dst != nil {
		typedmemmove(c.elem, dst, box)
	}
}

func sgoNativeZero(c *sgoChan, dst unsafe.Pointer) {
	if raceenabled {
		raceacquire(unsafe.Pointer(c))
	}
	if dst != nil {
		typedmemclr(c.elem, dst)
	}
}

// sgoRaceRendezvous gives a capacity-0 operation the two-way order of a rendezvous for the race
// detector (SGO-CHAN-002: the receive also happens before the send completes): every operation
// releases into the channel when it starts and acquires from it when it completes.
func sgoRaceRendezvousStart(c *sgoChan) {
	if raceenabled && c.cap == 0 {
		racereleasemerge(unsafe.Pointer(c))
	}
}

func sgoRaceRendezvousEnd(c *sgoChan) {
	if raceenabled && c.cap == 0 {
		raceacquire(unsafe.Pointer(c))
	}
}

func sgomakechan(t *chantype, size int) *sgoChan {
	if size < 0 {
		panic(error(&sgoFailure{"InvalidArgument", "make of a channel with a negative capacity", ""}))
	}
	return &sgoChan{cap: int64(size), elem: t.Elem, native: true}
}
