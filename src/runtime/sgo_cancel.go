// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// S-Go cancellation (W04): requests and propagation (SGO-CANCEL-001), observation at checkpoints
// (SGO-CANCEL-002), cancellation errors (SGO-CANCEL-003), linkage (SGO-CANCEL-004), Sleep
// (SGO-WAIT-001), and the drain budget (SGO-DRAIN-002). Trusted primitive TCB-P04 of SGO-TCB-001.
//
// Propagation is computed, not pushed. A scope's parent is its owner's current scope when it was
// opened, so the scopes linked to a task (its spawn scope and the nurseries it is executing inside,
// SGO-CANCEL-004) and every scope that transitively cancels them lie on one parent chain, starting
// at the task's current scope. A scope is CANCELLING in effect if it, or a scope above it on that
// chain, moved to CANCELLING on its own (Cancel, ChildFailed, Unwind); the inner scopes of a
// cancelled scope are CANCELLING with cause Ancestor (SGO-CANCEL-001). Inner cancellation never
// reaches outer scopes, and it stops affecting a task once the inner nursery closes and leaves the
// task's chain (consumed at its boundary, SGO-CANCEL-002). Every own transition takes the next
// number of a process-wide sequence, so "the first cause" is well defined along a chain.
//
// Only sleeping tasks have to be woken; they wait in sgoSleepers under sgoSleepLock. Sleep checks
// cancellation and registers under that lock and parks releasing it; a canceller makes its
// transition visible before scanning under the same lock, so a wake-up cannot be lost. Exactly one
// of the timer and a canceller wins a sleep (sgoSleep.state), and every Sleep call has its own
// record and timer, so a stale timer can only touch a finished record (C08).
//
// Lock order: scope lock (leaf, never held across the calls below), sgoSleepLock, then runtime
// timer locks.

package runtime

import (
	"internal/runtime/atomic"
	"unsafe"
)

// sgoCauseKind is the cause of an own cancellation transition (SGO-CANCEL-001).
type sgoCauseKind uint8

const (
	sgoCauseNone        sgoCauseKind = iota
	sgoCauseCancel                   // n.Cancel() by task id
	sgoCauseChildFailed              // child id ended Errored or Failed
	sgoCauseUnwind                   // the owner's body unwound through the nursery
)

type sgoCause struct {
	kind sgoCauseKind
	id   uint64
}

// sgoCancelSeq numbers own cancellation transitions process-wide; 0 means not cancelled.
var sgoCancelSeq atomic.Uint64

// sgoCancel moves s to CANCELLING with cause c if it is OPEN or JOINING (SGO-CANCEL-001). Later
// requests have no effect (sticky and idempotent). It wakes the sleeping tasks the transition
// cancels and starts the drain budget timer. The caller holds no scope lock.
func sgoCancel(s *sgoScope, c sgoCause) {
	lock(&s.lock)
	if s.state != sgoOpen && s.state != sgoJoining {
		unlock(&s.lock)
		return
	}
	s.state = sgoCancelling
	s.cause = c
	s.cancelSeq.Store(sgoCancelSeq.Add(1)) // visible before the scan below
	unlock(&s.lock)
	if raceenabled {
		racereleasemerge(unsafe.Pointer(&s.cancelSeq))
	}
	sgoDrainStart(s)
	sgoWakeCancelled()
}

// sgoCancelledFrom reports the own cancellation that is in effect for scope s: the earliest own
// transition among s and the scopes above it on its parent chain, or nil.
func sgoCancelledFrom(s *sgoScope) *sgoScope {
	var first *sgoScope
	var firstSeq uint64
	for p := s; p != nil; p = p.parent {
		if q := p.cancelSeq.Load(); q != 0 && (first == nil || q < firstSeq) {
			first, firstSeq = p, q
		}
	}
	return first
}

// sgoOuterCancelled returns the outermost scope linked to t that is CANCELLING in effect, or nil.
// Linked scopes are t's current scope up to and including its spawn scope (root: up to its
// outermost nursery). t.nest belongs to t, so only t itself, or a canceller while t is parked in
// Sleep, calls this. It does not allocate (it runs under sgoSleepLock).
func sgoOuterCancelled(t *sgoTask) *sgoScope {
	var outer *sgoScope
	for p := t.currentScope(); p != nil; p = p.parent {
		if sgoCancelledFrom(p) != nil {
			outer = p
		}
		if p == t.scope {
			break
		}
	}
	return outer
}

// sgoTaskCancellation returns the cancellation error the task t would observe now, or nil
// (SGO-CANCEL-002): the cause of the outermost linked scope that is CANCELLING in effect, which is
// that scope's own cause if its own transition came first, else the ancestor whose did.
func sgoTaskCancellation(t *sgoTask) error {
	outer := sgoOuterCancelled(t)
	if outer == nil {
		return nil
	}
	from := sgoCancelledFrom(outer)
	if from == outer {
		return sgoCancelError(outer.cause, 0)
	}
	if raceenabled {
		raceacquire(unsafe.Pointer(&from.cancelSeq))
	}
	return sgoCancelError(sgoCause{}, from.id)
}

// sgoCancelErr is a cancellation error (SGO-CANCEL-003); only Checkpoint and Sleep create one.
type sgoCancelErr struct{ msg string }

func (e *sgoCancelErr) Error() string { return e.msg }

// sgoCancelError formats the cancellation error of an own cause, or of ancestor scope anc (anc != 0).
func sgoCancelError(c sgoCause, anc uint64) error {
	var msg string
	switch {
	case anc != 0:
		msg = "cancelled: ancestor scope " + sgoItoa(anc) + " cancelled"
	case c.kind == sgoCauseCancel:
		msg = "cancelled: Cancel by task " + sgoItoa(c.id)
	case c.kind == sgoCauseChildFailed:
		msg = "cancelled: child " + sgoItoa(c.id) + " failed"
	default:
		msg = "cancelled: unwind"
	}
	return &sgoCancelErr{msg}
}

func sgoItoa(v uint64) string {
	var b [20]byte
	return string(sgoAppendUint(b[:0], v))
}

func sgoAppendUint(b []byte, v uint64) []byte {
	var d [20]byte
	i := len(d)
	for {
		i--
		d[i] = byte('0' + v%10)
		v /= 10
		if v == 0 {
			break
		}
	}
	return append(b, d[i:]...)
}

// sgoIsCancelled reports whether err is a cancellation error (SGO-CANCEL-003).
//
// The assertion goes through any: it then compares type descriptors, which are unique, instead of
// itab pointers. A test binary of the runtime built by the upstream toolchain was observed to hold
// two itabs for (*sgoCancelErr, error), which made a direct error-to-concrete assertion fail.
func sgoIsCancelled(err error) bool {
	_, ok := any(err).(*sgoCancelErr)
	return ok
}

// sgoNurseryCancel is n.Cancel() (SGO-CANCEL-001): the caller must be able to spawn into s
// (SGO-SPAWN-002 (b), (c)); in CANCELLING, FINALIZING, or CLOSED the call has no effect.
func sgoNurseryCancel(s *sgoScope) error {
	t := sgoCurrent()
	switch {
	case s == nil:
		return &sgoViolation{"cancel with a nil nursery handle"}
	case t == nil:
		return &sgoViolation{"cancel from a goroutine that is not an S-Go task"}
	case t.currentScope() != s:
		return &sgoViolation{"cancel of a nursery that is not the caller's current scope"}
	}
	sgoCancel(s, sgoCause{sgoCauseCancel, t.id})
	return nil
}

// sgoSleep is the wait record of one Sleep call.
type sgoSleep struct {
	g          *g
	task       *sgoTask
	timer      *timer
	when       int64
	state      atomic.Uint32 // 0 waiting, 1 timer won, 2 cancellation won
	prev, next *sgoSleep     // in sgoSleepers, under sgoSleepLock
	wake       *sgoSleep     // wake list of sgoWakeCancelled, under sgoSleepLock
}

var (
	sgoSleepLock mutex
	sgoSleepers  *sgoSleep
)

// sgoSleepFor is task.Sleep(ms) (SGO-WAIT-001): nil after at least ms milliseconds of monotonic
// time, or a cancellation error as soon as a linked scope is CANCELLING in effect.
func sgoSleepFor(t *sgoTask, ms int64) error {
	if err := sgoTaskCancellation(t); err != nil {
		return err
	}
	if ms == 0 {
		Gosched()
		return sgoTaskCancellation(t)
	}
	r := &sgoSleep{g: getg(), task: t, timer: new(timer)}
	r.timer.init(sgoSleepFire, r)
	ns := ms * 1_000_000
	if ns/1_000_000 != ms {
		ns = maxWhen
	}
	r.when = nanotime() + ns
	if r.when < 0 {
		r.when = maxWhen
	}
	lock(&sgoSleepLock)
	if sgoOuterCancelled(t) != nil {
		unlock(&sgoSleepLock)
		return sgoTaskCancellation(t)
	}
	r.next = sgoSleepers
	if sgoSleepers != nil {
		sgoSleepers.prev = r
	}
	sgoSleepers = r
	gopark(sgoSleepPark, unsafe.Pointer(r), waitReasonSleep, traceBlockSleep, 1)
	// Woken by the timer or a canceller; leave the registry and drop the timer.
	lock(&sgoSleepLock)
	sgoSleepRemove(r)
	unlock(&sgoSleepLock)
	r.timer.stop()
	if r.state.Load() == 2 {
		return sgoTaskCancellation(t)
	}
	return nil
}

// sgoSleepPark runs after the sleeper is parked: it releases sgoSleepLock, then arms the timer.
// Arming after the park means the timer can never ready a goroutine that is not parked yet.
func sgoSleepPark(gp *g, p unsafe.Pointer) bool {
	r := (*sgoSleep)(p)
	unlock(&sgoSleepLock)
	r.timer.reset(r.when, 0)
	return true
}

// sgoSleepFire is the timer function of one Sleep call.
func sgoSleepFire(arg any, _ uintptr, _ int64) {
	r := arg.(*sgoSleep)
	if r.state.CompareAndSwap(0, 1) {
		goready(r.g, 0)
	}
}

func sgoSleepRemove(r *sgoSleep) {
	if r.prev != nil {
		r.prev.next = r.next
	} else if sgoSleepers == r {
		sgoSleepers = r.next
	}
	if r.next != nil {
		r.next.prev = r.prev
	}
	r.prev, r.next = nil, nil
}

// sgoWakeCancelled wakes every sleeping task that is now cancelled. Sleepers are parked while
// registered, so their nesting does not change during the scan.
func sgoWakeCancelled() {
	var wake *sgoSleep
	lock(&sgoSleepLock)
	for r := sgoSleepers; r != nil; r = r.next {
		if sgoOuterCancelled(r.task) != nil && r.state.CompareAndSwap(0, 2) {
			r.wake, wake = wake, r
		}
	}
	chanWake := sgoChanWakeCancelled() // channel waiters (sgo_chan.go)
	unlock(&sgoSleepLock)
	sgoChanReady(chanWake)
	// A stale timer fire for these records fails its state CAS and does nothing.
	for wake != nil {
		r := wake
		wake, r.wake = r.wake, nil
		r.timer.stop()
		goready(r.g, 0)
	}
}

// sgoDrainBudget is the drain budget in milliseconds as a decimal string, set by the linker from
// sgo build -drain-budget (SGO-DRAIN-002); empty means no budget. No environment variable or
// run-time flag changes it. sgoDrainMS is its value, parsed when the program starts.
var (
	sgoDrainBudget string
	sgoDrainMS     int64
)

// sgoDrainInit parses sgoDrainBudget. The driver writes only positive decimal integers; anything
// else is a defect of the build.
func sgoDrainInit() {
	if sgoDrainBudget == "" {
		return
	}
	var v int64
	for i := 0; i < len(sgoDrainBudget); i++ {
		c := sgoDrainBudget[i]
		if c < '0' || c > '9' || v > (1<<62)/10 {
			sgoDefect("malformed drain budget in the binary")
		}
		v = v*10 + int64(c-'0')
	}
	if v <= 0 {
		sgoDefect("malformed drain budget in the binary")
	}
	sgoDrainMS = v
}

// sgoDrainStart starts the drain budget timer of a scope that just moved to CANCELLING on its own.
// Scopes cancelled in effect through an ancestor need none: the ancestor cannot reach FINALIZING
// before they close, so its deadline is the earlier one.
func sgoDrainStart(s *sgoScope) {
	if sgoDrainMS <= 0 {
		return
	}
	t := new(timer)
	t.init(sgoDrainFire, s)
	lock(&s.lock)
	if s.state != sgoCancelling {
		unlock(&s.lock)
		return // already finalizing: nothing to watch
	}
	s.drain = t
	unlock(&s.lock)
	t.reset(nanotime()+sgoDrainMS*1_000_000, 0)
}

// sgoDrainFire ends the process if the scope has not reached FINALIZING within the budget
// (SGO-DRAIN-002): one DrainBudgetExceeded record naming the scope, its live children, and the
// cause, then exit status 3. Stdout stays locked, so a write in progress completes and no later
// write starts; deferred calls of live tasks do not run. It does not allocate.
func sgoDrainFire(arg any, _ uintptr, _ int64) {
	s := arg.(*sgoScope)
	lock(&s.lock)
	if s.state != sgoCancelling {
		unlock(&s.lock)
		return
	}
	lock(&sgoStdoutLock) // never released: the process ends here
	var buf [512]byte
	b := append(buf[:0], "scope "...)
	b = sgoAppendUint(b, s.id)
	b = append(b, " did not drain within "...)
	b = sgoAppendUint(b, uint64(sgoDrainMS))
	b = append(b, " ms; live children:"...)
	for c := s.first; c != nil; c = c.sibling {
		if c.outcome == sgoRunning && len(b) < len(buf)-64 {
			b = append(b, ' ')
			b = sgoAppendUint(b, c.id)
		}
	}
	b = append(b, "; cause: "...)
	switch s.cause.kind {
	case sgoCauseCancel:
		b = append(b, "Cancel by task "...)
		b = sgoAppendUint(b, s.cause.id)
	case sgoCauseChildFailed:
		b = append(b, "child "...)
		b = sgoAppendUint(b, s.cause.id)
		b = append(b, " failed"...)
	default:
		b = append(b, "unwind"...)
	}
	msg := unsafe.String(&b[0], len(b))
	sgoWriteRecord(sgoFailureInfo{"DrainBudgetExceeded", msg, ""}, nil, sgoOrigin{task: s.owner.id, scope: s.id})
	exit(3)
}
