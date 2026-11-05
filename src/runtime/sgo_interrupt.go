// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Interrupts cancel the root (D179). In an S-Go program the first SIGINT or SIGTERM cancels the
// virtual root scope, sgoInterruptScope, which is the outermost ancestor of every scope: every task
// observes the cancellation at its next checkpoint, wait, or I/O operation, sgo.IsInterrupted
// recognizes it, and the program unwinds through its nurseries, releasing every resource. A second
// signal ends the process at once from the signal handler (status 128+signal: 130 for SIGINT, 143
// for SIGTERM), so two signals that arrive close together are never merged into one.
//
// Delivery, checked by the model sgo/model/interruptmodel.go: the handler counts signals and wakes
// sgoInterruptThread through a note (darwin: a pipe, sgo_interrupt_darwin.go). That thread is a system M without a P, counted like sysmon in
// sched.nmsys, so the deadlock check is unaffected while it sleeps; it hands the parked watcher
// goroutine to the scheduler (injectglist works without a P). The watcher publishes the
// cancellation and runs the ordinary wake scan on a P. A parked goroutine does not hide a
// deadlock; a goroutine or M waiting in a system call would.
//
// Before the program's root is entered, and in lab runs, signals keep their default actions. A
// SIGINT ignored when the process started (a background job) stays ignored (initsig).

package runtime

import (
	"internal/runtime/atomic"
	_ "unsafe" // go:linkname
)

// The signal numbers of SIGINT and SIGTERM on every S-Go target (darwin and linux).
const (
	sgoSigInt  = 2
	sgoSigTerm = 15
)

var (
	sgoInterruptArmed atomic.Bool   // SIGINT and SIGTERM go to sgoInterruptSignal
	sgoInterrupts     atomic.Uint32 // signals delivered
	sgoInterruptSig   atomic.Uint32 // the first signal
	sgoInterruptNote  note          // wakes sgoInterruptThread (off darwin: sgo_interrupt_note.go)

	// sgoInterruptScope is the virtual root scope: never opened, joined, or closed; only its own
	// cancellation is used (sgoCancelledFrom treats it as the last ancestor of every scope).
	sgoInterruptScope sgoScope

	sgoInterruptLock    mutex
	sgoInterruptWatcher guintptr // the watcher, while parked (no write barrier: the thread has no P)
	sgoInterruptPending bool     // the thread woke before the watcher parked
	sgoInterruptIdle    note     // never woken: the interrupt thread's final wait
)

// sgoInterruptArm starts the interrupt thread and watcher and routes the two signals to them. It
// runs once, from sgoEnterMain, outside lab runs.
func sgoInterruptArm() {
	sgoInterruptNoteSetup()
	go sgoInterruptWatch()
	systemstack(func() { newm(sgoInterruptThread, nil, -1) })
	sgoInterruptArmed.Store(true)
}

// sgoInterruptSignal is called by the signal handler (signal_unix.go). It neither allocates nor
// locks: the first signal wakes the interrupt thread, the second ends the process.
//
//go:nosplit
//go:nowritebarrierrec
func sgoInterruptSignal(sig uint32) {
	if sgoInterrupts.Add(1) == 1 {
		sgoInterruptSig.Store(sig)
		sgoInterruptNoteWake()
		return
	}
	exit(int32(128 + sig))
}

// sgoInterruptThread runs on its own M without a P and only waits for the first signal.
//
//go:nowritebarrierrec
func sgoInterruptThread() {
	lock(&sched.lock)
	sched.nmsys++
	checkdead()
	unlock(&sched.lock)

	sgoInterruptNoteSleep()
	lock(&sgoInterruptLock)
	gp := sgoInterruptWatcher.ptr()
	sgoInterruptWatcher = 0
	if gp == nil {
		sgoInterruptPending = true
	}
	unlock(&sgoInterruptLock)
	if gp != nil {
		var list gList
		list.push(gp)
		injectglist(&list)
	}
	for { // nothing more to do: park this M for good
		notesleep(&sgoInterruptIdle)
	}
}

// sgoInterruptWatch parks until the interrupt thread hands it to the scheduler, then cancels the
// virtual root scope.
func sgoInterruptWatch() {
	lock(&sgoInterruptLock)
	if !sgoInterruptPending {
		sgoInterruptWatcher.set(getg())
		goparkunlock(&sgoInterruptLock, waitReasonSGoInterrupt, traceBlockForever, 1)
	} else {
		unlock(&sgoInterruptLock)
	}
	sgoInterruptPublish(sgoInterruptSig.Load())
}

// sgoInterruptPublish cancels the virtual root scope and wakes every task now cancelled. It does
// not start a drain budget: the virtual root never finalizes, and a second signal is the bound.
func sgoInterruptPublish(sig uint32) {
	s := &sgoInterruptScope
	lock(&s.lock)
	if s.cancelSeq.Load() != 0 {
		unlock(&s.lock)
		return
	}
	s.state = sgoCancelling
	s.cause = sgoCause{kind: sgoCauseInterrupt, id: uint64(sig)}
	s.cancelSeq.Store(sgoCancelSeq.Add(1))
	unlock(&s.lock)
	sgoWakeCancelled()
}

// sgoInterruptMessage is the text of the cancellation error of an interrupt.
func sgoInterruptMessage(sig uint64) string {
	if sig == sgoSigTerm {
		return "cancelled: interrupted (SIGTERM)"
	}
	return "cancelled: interrupted (SIGINT)"
}

// sgoIsInterrupted reports whether err is a cancellation by an interrupt (D179).
func sgoIsInterrupted(err error) bool {
	if e, ok := any(err).(*sgoCancelErr); ok {
		return e.interrupted
	}
	e, ok := any(err).(*sgoIOError)
	return ok && e.kind == "Cancelled" && e.interrupted
}

//go:linkname sgo_isInterrupted sgo.runtime_isInterrupted
func sgo_isInterrupted(err error) bool { return err != nil && sgoIsInterrupted(err) }
