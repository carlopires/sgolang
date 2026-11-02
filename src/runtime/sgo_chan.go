// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Managed channels with send permits (W06, SGO-PERMIT-002 draft, plan §7). The executable
// reference model is sgo/model/chanmodel.go; the runtime lane checks this implementation against
// its outcome sets (sgo/model/testdata/chan-outcomes.json).
//
// Values are opaque pointers to boxes allocated by package sgo, so the runtime is not generic.
//
// Locks: a channel's lock, then sgoSleepLock (the cancellation registry shared with Sleep). The
// canceller (sgoWakeCancelled) takes only sgoSleepLock; channel operations take only the channel
// lock when they wake a waiter. A waiter is registered under both locks and parks releasing both,
// so neither kind of waker can ready it before it is parked.

package runtime

import (
	"internal/runtime/atomic"
	"unsafe"
)

// Permit states (plan §7.2); names match the model's PermitName.
const (
	sgoPermitReserved uint8 = iota + 1
	sgoPermitCommitted
	sgoPermitFailedClosed
	sgoPermitAborted
)

// Waiter states.
const (
	sgoWaitWaiting   uint32 = iota // parked; any waker may claim it
	sgoWaitDone                    // claimed by a channel operation: retry, or delivery is in val/ok
	sgoWaitCancelled               // claimed by the canceller
	sgoWaitMatched                 // C == 0 receiver bound to a reservation; only that permit resolves it
)

type sgoChan struct {
	lock     mutex
	cap      int64
	queue    []unsafe.Pointer // committed values not yet received (C > 0), FIFO
	reserved int64            // reservations not yet terminal
	closed   bool
	senders  sgoWaitList
	recvs    sgoWaitList
	elem     *_type // native channels (sgo_native.go): the element type of the boxed values
	native   bool
}

type sgoPermit struct {
	ch         *sgoChan
	owner      *sgoTask
	scope      *sgoScope // the owner's innermost nursery at Reserve (nil: the task body itself)
	state      uint8
	peer       *sgoChanWait // C == 0: the matched receiver
	prev, next *sgoPermit   // owner's unresolved permits, touched only by the owner
}

type sgoChanWait struct {
	g          *g
	task       *sgoTask
	ch         *sgoChan
	state      atomic.Uint32
	val        unsafe.Pointer // receiver: delivered value
	delivered  bool           // receiver: val is a delivery (ok = true)
	prev, next *sgoChanWait   // in senders or recvs, under ch.lock
	cprev      *sgoChanWait   // in sgoChanWaiters, under sgoSleepLock
	cnext      *sgoChanWait
	wake       *sgoChanWait   // wake list, under the lock of whoever builds it
	pre        *sgoChanWait   // senders this C == 0 receiver claimed; readied once it is parked
	listed     bool           // in the channel's list
	sel        *sgoSel        // native: the operation's single decision (nil: state decides)
	box        unsafe.Pointer // native sender: the boxed value
	closed     bool           // native sender: claimed by close (ClosedChannel)
	arm        int32          // native select: the arm's cas index (lab select events)
}

type sgoWaitList struct{ first, last *sgoChanWait }

func (l *sgoWaitList) push(w *sgoChanWait) {
	w.prev, w.next, w.listed = l.last, nil, true
	if l.last != nil {
		l.last.next = w
	} else {
		l.first = w
	}
	l.last = w
}

func (l *sgoWaitList) remove(w *sgoChanWait) {
	if !w.listed {
		return
	}
	if w.prev != nil {
		w.prev.next = w.next
	} else {
		l.first = w.next
	}
	if w.next != nil {
		w.next.prev = w.prev
	} else {
		l.last = w.prev
	}
	w.prev, w.next, w.listed = nil, nil, false
}

// sgoChanWaiters is the cancellation registry of parked channel waiters (under sgoSleepLock).
var sgoChanWaiters *sgoChanWait

// sgoClosedErr is the error of Reserve on a closed channel and of Commit after close.
type sgoClosedErr struct{ msg string }

func (e *sgoClosedErr) Error() string { return e.msg }

var (
	sgoErrReserveClosed = error(&sgoClosedErr{"ChannelClosed: reserve on a closed channel"})
	sgoErrCommitClosed  = error(&sgoClosedErr{"ChannelClosed: commit after close, no effect"})
)

func sgoChanNew(capacity int64) *sgoChan {
	return &sgoChan{cap: capacity}
}

// sgoChanClaimAll claims every waiting waiter of l (state Waiting -> Done), unlinks it, and adds it
// to the wake list. Called under c.lock.
func sgoChanClaimAll(l *sgoWaitList, wake *sgoChanWait) *sgoChanWait {
	for w := l.first; w != nil; {
		next := w.next
		if w.state.CompareAndSwap(sgoWaitWaiting, sgoWaitDone) {
			l.remove(w)
			w.wake, wake = wake, w
		}
		w = next
	}
	return wake
}

func sgoChanReady(wake *sgoChanWait) {
	for wake != nil {
		w := wake
		wake, w.wake = w.wake, nil
		sgoWake(w.g)
	}
}

// sgoChanPark parks the caller as waiter w on list l of c. Called with c.lock held; returns with
// c.lock held and w unregistered. It reports false, without parking, if the task is cancelled.
// With wakeSenders (a C == 0 receiver), the waiting senders are claimed after w is listed, so a
// sender that retries finds w, and they are readied only after w is parked and the locks released
// (in the lab, before the decision that precedes the park).
func sgoChanPark(c *sgoChan, l *sgoWaitList, w *sgoChanWait, reason waitReason, wakeSenders bool) bool {
	w.g, w.ch = getg(), c
	w.state.Store(sgoWaitWaiting)
	l.push(w)
	if wakeSenders {
		w.pre = sgoChanClaimAll(&c.senders, nil)
	}
	lock(&sgoSleepLock)
	if sgoOuterCancelled(w.task) != nil {
		unlock(&sgoSleepLock)
		l.remove(w)
		// Claimed senders are parked (they are listed only while parked); readying them under c.lock
		// only makes them wait for it.
		sgoChanReady(w.pre)
		w.pre = nil
		return false
	}
	w.cnext = sgoChanWaiters
	if sgoChanWaiters != nil {
		sgoChanWaiters.cprev = w
	}
	sgoChanWaiters = w
	if sgoLab != nil {
		// The claimed senders must be ready before the decision: readied only by the park commit,
		// after it, they would be missing from it, and with no other task ready the run would end
		// LabDeadlock while a sender was about to be woken. In the lab, readying only marks a task
		// ready; one chosen before its own park commit aborts that park (sgoLabCommit).
		sgoChanReady(w.pre)
		w.pre = nil
		sgoLabBlock(w.task, reason.String()) // the decision before parking (sgo_lab_sched.go)
	}
	gopark(sgoChanParkUnlock, unsafe.Pointer(w), reason, traceBlockChanRecv, 1)
	sgoLabAfterPark()
	lock(&c.lock)
	lock(&sgoSleepLock)
	sgoChanUnregister(w)
	unlock(&sgoSleepLock)
	l.remove(w)
	return true
}

func sgoChanParkUnlock(gp *g, p unsafe.Pointer) bool {
	w := (*sgoChanWait)(p)
	pre := w.pre
	w.pre = nil
	unlock(&sgoSleepLock)
	unlock(&w.ch.lock)
	sgoChanReady(pre)
	if sgoLab != nil {
		return sgoLabCommit(gp)
	}
	return true
}

func sgoChanUnregister(w *sgoChanWait) {
	if w.cprev != nil {
		w.cprev.cnext = w.cnext
	} else if sgoChanWaiters == w {
		sgoChanWaiters = w.cnext
	}
	if w.cnext != nil {
		w.cnext.cprev = w.cprev
	}
	w.cprev, w.cnext = nil, nil
}

// sgoChanWakeCancelled claims every registered channel waiter of a cancelled task. Called by
// sgoWakeCancelled under sgoSleepLock; returns the wake list. Matched receivers are not claimed:
// a rendezvous receive takes effect when it is matched, and completes when its permit resolves.
func sgoChanWakeCancelled() *sgoChanWait {
	var wake *sgoChanWait
	for w := sgoChanWaiters; w != nil; w = w.cnext {
		if sgoOuterCancelled(w.task) != nil && w.claim(sgoWaitCancelled) {
			w.wake, wake = wake, w
		}
	}
	return wake
}

// sgoChanReserve is ch.Reserve(task) (SGO-PERMIT-002).
func sgoChanReserve(c *sgoChan, t *sgoTask) (*sgoPermit, error) {
	if err := sgoTaskCancellation(t); err != nil {
		return nil, err
	}
	w := &sgoChanWait{task: t}
	// Allocated before taking c.lock: no allocation under a runtime lock (runtime-design §3,
	// W12 R9). The task's current scope does not change while it is blocked here, so the permit is
	// reused across retries; on the closed and cancelled exits it is dropped.
	p := &sgoPermit{ch: c, owner: t, scope: t.currentScope(), state: sgoPermitReserved}
	lock(&c.lock)
	for {
		if c.closed {
			unlock(&c.lock)
			return nil, sgoErrReserveClosed
		}
		if c.cap > 0 && int64(len(c.queue))+c.reserved < c.cap {
			c.reserved++
			sgoChanCheck(c)
			unlock(&c.lock)
			sgoPermitLink(p)
			sgoChanInject(sgoFaultAfterReserve)
			return p, nil
		}
		if c.cap == 0 {
			matched := false
			for r := c.recvs.first; r != nil; r = r.next {
				if r.state.CompareAndSwap(sgoWaitWaiting, sgoWaitMatched) {
					p.peer, matched = r, true
					break
				}
			}
			if matched {
				c.reserved++
				sgoChanCheck(c)
				unlock(&c.lock)
				sgoPermitLink(p)
				sgoChanInject(sgoFaultAfterReserve)
				return p, nil
			}
		}
		if !sgoChanPark(c, &c.senders, w, waitReasonChanSend, false) || w.state.Load() == sgoWaitCancelled {
			unlock(&c.lock)
			return nil, sgoTaskCancellation(t)
		}
	}
}

// sgoChanCommit is p.Commit(v): publishes v exactly once, or fails with no effect after close.
func sgoChanCommit(p *sgoPermit, v unsafe.Pointer) error {
	sgoPermitOwned(p, "commit")
	if p.state != sgoPermitReserved {
		panic(&sgoViolation{"commit of a permit that is already " + sgoPermitName(p.state)})
	}
	sgoChanInject(sgoFaultBeforeCommit)
	c := p.ch
	var wake *sgoChanWait
	var err error
	lock(&c.lock)
	c.reserved--
	switch {
	case c.closed:
		p.state, err = sgoPermitFailedClosed, sgoErrCommitClosed
		wake = sgoPermitReleasePeer(p, wake)
	case c.cap > 0:
		p.state = sgoPermitCommitted
		c.queue = append(c.queue, v)
		wake = sgoChanClaimAll(&c.recvs, wake)
	default:
		p.state = sgoPermitCommitted
		r := p.peer
		p.peer = nil
		r.val, r.delivered = v, true
		c.recvs.remove(r)
		r.state.Store(sgoWaitDone)
		r.wake, wake = wake, r
	}
	if p.state == sgoPermitFailedClosed {
		wake = sgoChanClaimAll(&c.senders, wake)
	}
	sgoChanCheck(c)
	unlock(&c.lock)
	sgoPermitUnlink(p)
	sgoChanReady(wake)
	sgoChanInject(sgoFaultAfterCommit)
	return err
}

// sgoChanAbort is p.Abort(): restores the reservation; no effect on a terminal permit.
func sgoChanAbort(p *sgoPermit) {
	sgoPermitOwned(p, "abort")
	if p.state != sgoPermitReserved {
		return
	}
	sgoPermitAbort(p)
}

func sgoPermitAbort(p *sgoPermit) {
	sgoChanInject(sgoFaultBeforeAbort)
	c := p.ch
	lock(&c.lock)
	c.reserved--
	p.state = sgoPermitAborted
	wake := sgoPermitReleasePeer(p, nil)
	wake = sgoChanClaimAll(&c.senders, wake)
	sgoChanCheck(c)
	unlock(&c.lock)
	sgoPermitUnlink(p)
	sgoChanReady(wake)
	sgoChanInject(sgoFaultAfterAbort)
}

// sgoPermitReleasePeer returns a matched receiver to waiting (or to cancelled, if its task was
// cancelled meanwhile). Called under c.lock.
func sgoPermitReleasePeer(p *sgoPermit, wake *sgoChanWait) *sgoChanWait {
	r := p.peer
	if r == nil {
		return wake
	}
	p.peer = nil
	if p.ch.closed {
		r.state.Store(sgoWaitDone)
		p.ch.recvs.remove(r)
		r.wake, wake = wake, r
		return wake
	}
	r.state.Store(sgoWaitWaiting)
	if sgoOuterCancelled(r.task) != nil && r.state.CompareAndSwap(sgoWaitWaiting, sgoWaitCancelled) {
		p.ch.recvs.remove(r)
		r.wake, wake = wake, r
	}
	return wake
}

// sgoChanReceive is ch.Receive(task): the next value with ok, or ok == false once closed and empty.
func sgoChanReceive(c *sgoChan, t *sgoTask) (unsafe.Pointer, bool, error) {
	if err := sgoTaskCancellation(t); err != nil {
		return nil, false, err
	}
	w := &sgoChanWait{task: t}
	lock(&c.lock)
	for {
		if c.cap > 0 && len(c.queue) > 0 {
			v := c.queue[0]
			c.queue[0] = nil
			c.queue = c.queue[1:]
			wake := sgoChanClaimAll(&c.senders, nil)
			sgoChanCheck(c)
			unlock(&c.lock)
			sgoChanReady(wake)
			sgoChanInject(sgoFaultAfterReceive)
			return v, true, nil
		}
		if c.closed {
			unlock(&c.lock)
			return nil, false, nil
		}
		// C == 0: a waiting receiver is what lets a sender reserve, so it wakes the senders.
		parked := sgoChanPark(c, &c.recvs, w, waitReasonChanReceive, c.cap == 0)
		switch {
		case !parked || w.state.Load() == sgoWaitCancelled:
			unlock(&c.lock)
			return nil, false, sgoTaskCancellation(t)
		case w.delivered:
			v := w.val
			unlock(&c.lock)
			return v, true, nil
		}
	}
}

// sgoChanClose is ch.Close(); closing a closed channel has no effect.
func sgoChanClose(c *sgoChan) {
	sgoChanInject(sgoFaultBeforeClose)
	lock(&c.lock)
	if c.closed {
		unlock(&c.lock)
		return
	}
	c.closed = true
	wake := sgoChanClaimAll(&c.senders, nil)
	wake = sgoChanClaimAll(&c.recvs, wake) // matched receivers stay until their permit resolves
	sgoChanCheck(c)
	unlock(&c.lock)
	sgoChanReady(wake)
	sgoChanInject(sgoFaultAfterClose)
}

// sgoChanCheck asserts the channel invariants under c.lock (W06 debug assertions): reservations
// are not negative and, for C > 0, queued plus reserved never exceeds the capacity. A violation
// is a runtime defect (SGO-ERR-003), not a program failure.
func sgoChanCheck(c *sgoChan) {
	if c.reserved < 0 {
		throw("sgo: negative channel reservation count")
	}
	if c.cap > 0 && int64(len(c.queue))+c.reserved > c.cap {
		throw("sgo: channel capacity exceeded")
	}
	if c.cap == 0 && len(c.queue) != 0 {
		throw("sgo: rendezvous channel holds a queued value")
	}
}

// Fault-injection points before and after each terminal transition (W06). sgoChanFault is set
// only by the runtime's own tests (export_sgo_test.go); it runs with no lock held.
const (
	sgoFaultAfterReserve = iota
	sgoFaultBeforeCommit
	sgoFaultAfterCommit
	sgoFaultBeforeAbort
	sgoFaultAfterAbort
	sgoFaultAfterReceive
	sgoFaultBeforeClose
	sgoFaultAfterClose
	sgoFaultPoints
)

var sgoChanFault func(point int)

func sgoChanInject(point int) {
	if f := sgoChanFault; f != nil {
		f(point)
	}
}

func sgoPermitOwned(p *sgoPermit, op string) {
	if p == nil {
		panic(&sgoViolation{op + " of a nil permit"})
	}
	if sgoCurrent() != p.owner {
		panic(&sgoViolation{op + " of a permit by a task that did not reserve it"})
	}
}

func sgoPermitName(s uint8) string {
	return [...]string{"none", "reserved", "committed", "failed-closed", "aborted"}[s]
}

func sgoPermitLink(p *sgoPermit) {
	t := p.owner
	p.next = t.permits
	if t.permits != nil {
		t.permits.prev = p
	}
	t.permits = p
}

func sgoPermitUnlink(p *sgoPermit) {
	t := p.owner
	if p.prev != nil {
		p.prev.next = p.next
	} else if t.permits == p {
		t.permits = p.next
	}
	if p.next != nil {
		p.next.prev = p.prev
	}
	p.prev, p.next = nil, nil
}

// sgoPermitsResolve aborts the calling task's unresolved permits reserved inside scope s (all of
// them when all is set: task end), and records each as abandoned (plan §7.2, H09, H10): a
// PermitAbandoned entry in the task's failure sequence, which appears in the diagnostic record's
// chain if the task fails or returns an error. It runs at owner-body exit before the join and
// before a task's outcome is published.
func sgoPermitsResolve(t *sgoTask, s *sgoScope, all bool) {
	sgoTokensResolve(t, s, all)  // limiter tokens are the same kind of obligation (D39)
	sgoHandlesResolve(t, s, all) // so are open I/O handles (D54)
	for p := t.permits; p != nil; {
		next := p.next
		if all || p.scope == s {
			t.abandoned++
			t.seq++
			t.entries = append(t.entries, sgoEntry{t.seq, sgoFailureInfo{"PermitAbandoned",
				"send permit reserved by task " + sgoItoa(t.id) + " was neither committed nor aborted; the runtime aborted it", ""}})
			sgoPermitAbort(p)
		}
		p = next
	}
}

// Package sgo entry points (src/sgo/chan.go). Values are boxes allocated by package sgo.

//go:linkname sgo_chanNew sgo.runtime_chanNew
func sgo_chanNew(capacity int) unsafe.Pointer {
	return unsafe.Pointer(sgoChanNew(int64(capacity)))
}

//go:linkname sgo_chanReserve sgo.runtime_chanReserve
func sgo_chanReserve(ch, task unsafe.Pointer) (unsafe.Pointer, error) {
	p, err := sgoChanReserve((*sgoChan)(ch), (*sgoTask)(task))
	return unsafe.Pointer(p), err
}

//go:linkname sgo_chanCommit sgo.runtime_chanCommit
func sgo_chanCommit(p, v unsafe.Pointer) error {
	return sgoChanCommit((*sgoPermit)(p), v)
}

//go:linkname sgo_chanAbort sgo.runtime_chanAbort
func sgo_chanAbort(p unsafe.Pointer) {
	sgoChanAbort((*sgoPermit)(p))
}

//go:linkname sgo_chanReceive sgo.runtime_chanReceive
func sgo_chanReceive(ch, task unsafe.Pointer) (unsafe.Pointer, bool, error) {
	return sgoChanReceive((*sgoChan)(ch), (*sgoTask)(task))
}

//go:linkname sgo_chanClose sgo.runtime_chanClose
func sgo_chanClose(ch unsafe.Pointer) {
	sgoChanClose((*sgoChan)(ch))
}
