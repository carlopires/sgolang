// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// S-Go structured-concurrency core (W02): scope and task records, explicit-handle admission,
// registration before publication, join, and outcomes. Trusted primitives TCB-P01 (task creation),
// TCB-P02 (park/unpark), TCB-P03 (scope state and join) of SGO-TCB-001. Design: sgo/docs/runtime-design.md.
//
// Lock order: a scope lock is a leaf. It is never held while calling user code, allocating, parking
// (except through goparkunlock, which releases it), or acquiring another scope lock.

package runtime

import (
	"internal/runtime/atomic"
	"internal/runtime/sys"
	"unsafe"
)

// sgoScopeState is the scope state of SGO-NUR-002.
type sgoScopeState uint32

const (
	sgoOpen sgoScopeState = iota
	sgoJoining
	sgoCancelling
	sgoFinalizing
	sgoClosed
)

func (s sgoScopeState) String() string {
	switch s {
	case sgoOpen:
		return "OPEN"
	case sgoJoining:
		return "JOINING"
	case sgoCancelling:
		return "CANCELLING"
	case sgoFinalizing:
		return "FINALIZING"
	case sgoClosed:
		return "CLOSED"
	}
	return "INVALID"
}

// sgoOutcome is a child outcome of SGO-SPAWN-004.
type sgoOutcome uint8

const (
	sgoRunning sgoOutcome = iota
	sgoSucceeded
	sgoCancelled
	sgoErrored
	sgoFailed
)

// sgoScope is one execution of a nursery statement. It is a heap object reachable from its owner's
// task record (while open) and from its children's records; it holds no pointer into any stack.
type sgoScope struct {
	lock     mutex
	id       uint64
	state    sgoScopeState
	owner    *sgoTask
	parent   *sgoScope // the owner's current scope when this scope was opened (nil at top level)
	live     int64     // registered children that have not published an outcome
	first    *sgoTask  // registered children in creation order (intrusive list via sibling)
	last     *sgoTask
	joiner   *g // owner parked in join, or nil
	nchild   int64
	snapshot sgoScopeResult // filled at CLOSED (immutable afterwards)
	res      sgoResList     // resources of the nursery, released after its join (sgo_res.go, D148)
	// frame is stack.hi minus the SP of the frame executing the nursery statement (set by
	// sgoNurseryEnter). It is an offset, not an address, so it survives stack copying (§2.4);
	// unwinding uses it to complete the nurseries of the frames it leaves (SGO-DEF-005).
	frame uintptr
	// Own cancellation (sgo_cancel.go): cause and sequence number of the scope's own transition to
	// CANCELLING (cancelSeq 0: none; read without the lock along parent chains), and the drain
	// budget timer it started.
	cause     sgoCause
	cancelSeq atomic.Uint64
	drain     *timer
}

// sgoTask is the record of an S-Go task: the root task or a child admitted by sgoSpawn.
type sgoTask struct {
	id      uint64
	scope   *sgoScope   // scope the task was spawned into (owner scope); nil for the root task
	nest    []*sgoScope // nurseries this task is executing inside, innermost last
	sibling *sgoTask
	fn      func(*sgoTask) error // callback; cleared once it has run
	outcome sgoOutcome
	err     error // Errored: the returned error
	panic   any   // Failed: the panic value

	// Diagnostic record state (sgo_record.go). seq numbers the task's failures; pend is the
	// origin for the panic a nursery is about to propagate; entries are chain entries recorded by
	// joins during the current unwinding. Once Failed or Errored, chain is the task's own chain
	// and origin is where its primary failure was raised.
	seq  uint64
	pend struct {
		seq    uint64
		origin sgoOrigin
	}
	entries []sgoEntry
	chain   []sgoFailureInfo
	origin  sgoOrigin

	// Channel permits (sgo_chan.go): unresolved permits the task reserved (touched only by the
	// task), and how many the runtime aborted because the task left them unresolved.
	permits *sgoPermit
	tokens  *sgoToken  // limiter tokens the task holds (sgo_sync.go)
	res     sgoResList // resources of the task body, released when it ends (sgo_res.go, D148)
	// Lab state (sgo_lab.go): the lab task ID, ready and waiting flags, and the step count since
	// the task's last decision. Touched under sgoLab.lock or by the baton holder.
	labID                                 int32
	labReady, labRun, labParked, labWoken bool
	labInWait                             bool
	labDone                               bool
	labSeq, labSince                      int64
	labG                                  *g
	labWait                               string // what the task waits in (LabDeadlock wait graph)
	abandoned                             int64

	// ready is the readiness record of a child started by n.Start (sgo_ready.go), nil otherwise.
	ready *sgoReady

	// ceiling is the set of effect kinds the task may exercise (sgo_auth.go), fixed at creation.
	ceiling uint8
}

// sgoScopeResult is the immutable outcome of a closed scope (SGO-NUR-004 rules 2-4; rule 1, the
// owner's own unwinding, is applied by compiler-generated code in W03).
type sgoScopeResult struct {
	Outcome   sgoOutcome // sgoSucceeded, sgoErrored, or sgoFailed
	Primary   uint64     // creation ID of the primary failed/errored child (0 if none)
	Err       error
	Panic     any
	Children  int64
	Failures  int64 // children that ended Failed or Errored
	Cancelled int64
}

var (
	sgoNextTaskID  atomic.Uint64 // creation IDs; root task is 0, children start at 1
	sgoNextScopeID atomic.Uint64
)

// sgoViolation is the SpawnContractViolation of SGO-SPAWN-002 / SGO-CANCEL-001.
type sgoViolation struct{ msg string }

func (e *sgoViolation) Error() string { return "SpawnContractViolation: " + e.msg }

// sgoCurrent returns the calling goroutine's task record, or nil if it is not an S-Go task.
func sgoCurrent() *sgoTask { return getg().sgo }

// currentScope is the task's innermost open nursery, else its spawn scope (SGO-NUR-005).
func (t *sgoTask) currentScope() *sgoScope {
	if n := len(t.nest); n > 0 {
		return t.nest[n-1]
	}
	return t.scope
}

// sgoEnterRoot makes the calling goroutine the root task (creation ID 0). In production this is done
// once by runtime start-up before main (W03); W02 conformance hooks call it explicitly.
func sgoEnterRoot() *sgoTask {
	gp := getg()
	if gp.sgo != nil {
		throw("sgo: goroutine is already an S-Go task")
	}
	// The root of a program gets the granted kinds (sgoEnterMain); a root entered by the runtime's
	// own tests gets every kind and narrows it through SpawnWithout.
	gp.sgo = &sgoTask{id: 0, ceiling: sgoKindAll}
	return gp.sgo
}

// sgoLeaveRoot detaches the root record (test hooks only); every nursery must be closed.
func sgoLeaveRoot() {
	gp := getg()
	if gp.sgo == nil || len(gp.sgo.nest) != 0 {
		throw("sgo: leaving root with open nurseries")
	}
	sgoPermitsResolve(gp.sgo, nil, true)
	sgoResRelease(&gp.sgo.res, true)
	gp.sgo = nil
}

// sgoNurseryOpen starts a nursery statement owned by the calling task (SGO-NUR-001).
func sgoNurseryOpen() (*sgoScope, error) {
	t := sgoCurrent()
	if t == nil {
		return nil, &sgoViolation{"nursery opened by a goroutine that is not an S-Go task"}
	}
	s := &sgoScope{id: sgoNextScopeID.Add(1), owner: t, parent: t.currentScope()}
	t.nest = append(t.nest, s)
	return s, nil
}

// sgoSpawn admits fn as a child of s (SGO-SPAWN-002/003). It returns a *sgoViolation if admission
// is refused, and nil if the child was admitted or silently not created because s is CANCELLING (R5).
func sgoSpawn(s *sgoScope, fn func(*sgoTask) error) error {
	_, err := sgoAdmit(s, fn, nil, 0)
	return err
}

// sgoAdmit is sgoSpawn that also reports the created child (nil if none was created because s is
// CANCELLING), attaches the readiness record r, if any, before the child can run (n.Start,
// sgo_ready.go), and removes the kinds in without from the child's ceiling (n.SpawnWithout,
// sgo_auth.go).
func sgoAdmit(s *sgoScope, fn func(*sgoTask) error, r *sgoReady, without uint8) (*sgoTask, error) {
	if sgoLab != nil {
		sgoLabPoint()
	}
	t := sgoCurrent()
	switch {
	case s == nil:
		return nil, &sgoViolation{"spawn with a nil nursery handle"}
	case fn == nil:
		return nil, &sgoViolation{"spawn with a nil callback"}
	case t == nil:
		return nil, &sgoViolation{"spawn from a goroutine that is not an S-Go task"}
	case t.currentScope() != s:
		return nil, &sgoViolation{"spawn into a nursery that is not the caller's current scope"}
	}
	// Allocate before taking the scope lock (no allocation under runtime locks).
	c := &sgoTask{scope: s, fn: fn, ready: r, ceiling: t.ceiling &^ without}
	lock(&s.lock)
	switch s.state {
	case sgoOpen, sgoJoining:
		// CANCELLING in effect through an ancestor (cause Ancestor, SGO-CANCEL-001) is CANCELLING.
		if sgoCancelledFrom(s.parent) != nil {
			unlock(&s.lock)
			return nil, nil
		}
	case sgoCancelling:
		unlock(&s.lock)
		return nil, nil // R5: no child; cancellation is observed at the caller's next cancellation point
	default:
		st := s.state
		unlock(&s.lock)
		return nil, &sgoViolation{"spawn into a nursery in state " + st.String()}
	}
	if r != nil {
		r.child = c
	}
	// Register before the child can run (OWN-2).
	c.id = sgoNextTaskID.Add(1)
	s.live++
	s.nchild++
	if s.last == nil {
		s.first = c
	} else {
		s.last.sibling = c
	}
	s.last = c
	unlock(&s.lock)
	if sgoLab != nil {
		sgoLabSpawned(c)
	}

	gp := getg()
	pc := sys.GetCallerPC()
	start := sgoTaskStart // static funcval: no closure is allocated in the runtime
	fv := *(**funcval)(unsafe.Pointer(&start))
	systemstack(func() {
		newg := newproc1(fv, gp, pc, false, waitReasonZero)
		newg.sgo = c // before the goroutine is runnable
		pp := getg().m.p.ptr()
		runqput(pp, newg, true)
		if mainStarted {
			wakep()
		}
	})
	return c, nil
}

// sgoTaskStart is the entry point of every child goroutine. Its task record was attached to the g
// before the g became runnable.
func sgoTaskStart() {
	c := getg().sgo
	if sgoLab != nil {
		sgoLabStart(c)
	}
	fn := c.fn
	c.fn = nil
	sgoTaskMain(c, fn)
}

// sgoTaskMain runs a child callback and publishes its outcome exactly once.
func sgoTaskMain(c *sgoTask, fn func(*sgoTask) error) {
	normal := false
	defer func() {
		if !normal {
			// The child's failure is the first one of its unwinding (SGO-DEF-004), not a later
			// failure raised by a deferred call, which is what recover would return.
			// Its chain and origin are taken before recover ends the unwinding (sgo_record.go).
			var primary any
			c.origin = sgoOriginOf(c)
			// Unresolved permits are aborted first, so their PermitAbandoned entries are in the
			// chain (SGO-PERMIT-002, H09); the body's resources are released, unwinding (D148).
			sgoPermitsResolve(c, nil, true)
			sgoReleaseOutcome(c, sgoResRelease(&c.res, false), true)
			if p := getg()._panic; p != nil {
				primary, _ = sgoUnwound(p)
				c.chain, c.origin = sgoChain(c, p), sgoPrimaryOrigin(p)
			}
			v := recover()
			if primary == nil {
				primary = v
			}
			if u, ok := primary.(*sgoCancelUnwind); ok {
				// D33: a cancellation unwind ends the task Cancelled, as a returned cancellation
				// error would, unless a child or deferred call failed meanwhile.
				if later := sgoLaterFailure(v, u); later != nil {
					primary = later
				} else {
					c.outcome, c.err, c.chain = sgoCancelled, u.err, nil
					c.entries = nil
					sgoTaskExit(c)
					return
				}
			}
			c.outcome, c.panic = sgoFailed, primary
			c.entries = nil
		}
		sgoTaskExit(c)
	}()
	err := fn(c)
	// The body's resources are released before the outcome exists (SGO-RES-001): a failed release
	// of a body that returned nil makes the task Errored (D149); a panicking Release fails it.
	if f := sgoReleaseOutcome(c, sgoResRelease(&c.res, err == nil), err != nil); f != nil {
		if e, ok := f.(*sgoFailure); ok && e.kind == "ReleaseError" {
			err = e
		} else {
			panic(f)
		}
	}
	normal = true
	sgoPermitsResolve(c, nil, true)
	switch {
	case err == nil:
		c.outcome = sgoSucceeded
	case sgoIsCancelled(err):
		c.outcome, c.err = sgoCancelled, err // kept for n.Start (sgo_ready.go); not a failure
	default:
		c.outcome, c.err, c.origin = sgoErrored, err, sgoOriginOf(c)
		// An errored child's chain holds the permits it abandoned (only failures run deferred
		// panics, so these are its only entries).
		for _, e := range c.entries {
			c.chain = append(c.chain, e.info)
		}
	}
	c.entries = nil
}

// sgoTaskExit publishes the child's outcome and wakes the joining owner if it was the last child.
func sgoTaskExit(c *sgoTask) {
	if sgoLab != nil {
		sgoLabPoint() // the end point (D43), then the end event
		sgoLabTaskEnded(c)
	}
	if len(c.nest) != 0 {
		throw("sgo: task finished with an open nursery") // RuntimeDefect (SGO-ERR-003)
	}
	// Unresolved permits are aborted before the outcome is published (SGO-PERMIT-002, H09).
	sgoPermitsResolve(c, nil, true)
	// A started child that ends before signalling readiness wakes its starter (SGO-READY-002).
	if c.ready != nil {
		sgoReadyChildEnded(c)
	}
	s := c.scope
	// A child that ended Errored or Failed cancels its scope when the outcome is published, before
	// the scope's join can complete (SGO-SPAWN-004, SGO-CANCEL-001).
	if c.outcome == sgoErrored || c.outcome == sgoFailed {
		sgoCancel(s, sgoCause{sgoCauseChildFailed, c.id})
	}
	if raceenabled {
		racereleasemerge(unsafe.Pointer(s))
	}
	var wake *g
	lock(&s.lock)
	if c.outcome == sgoRunning {
		unlock(&s.lock)
		throw("sgo: outcome not set at task exit")
	}
	s.live--
	if s.live < 0 {
		unlock(&s.lock)
		throw("sgo: negative live-child count") // M04: completion published twice
	}
	if s.live == 0 && s.joiner != nil {
		wake, s.joiner = s.joiner, nil
	}
	unlock(&s.lock)
	if wake != nil {
		sgoWake(wake)
	}
	if sgoLab != nil {
		sgoLabEndTask(c)
	}
	getg().sgo = nil
}

// sgoNurseryJoin completes the calling task's innermost nursery s: it waits until no registered child
// is live, closes s, and returns its outcome (SGO-NUR-003, SGO-NUR-004) and its children that
// ended Failed or Errored, in creation order (for the diagnostic chain, sgo_record.go).
func sgoNurseryJoin(s *sgoScope) (sgoScopeResult, []*sgoTask, error) {
	if sgoLab != nil {
		sgoLabPoint()
	}
	t := sgoCurrent()
	if t == nil || s == nil || s.owner != t || t.currentScope() != s {
		return sgoScopeResult{}, nil, &sgoViolation{"join of a nursery that is not the caller's innermost nursery"}
	}
	// The body's unresolved permits are aborted before the join (SGO-PERMIT-002, H10).
	sgoPermitsResolve(t, s, false)
	lock(&s.lock)
	if s.state == sgoOpen {
		s.state = sgoJoining
	}
	for s.live > 0 {
		s.joiner = getg()
		if sgoLab != nil {
			sgoLabBlock(t, "nursery join")
			gopark(sgoJoinPark, unsafe.Pointer(s), waitReasonSGoJoin, traceBlockSync, 1)
			sgoLabAfterPark()
		} else {
			goparkunlock(&s.lock, waitReasonSGoJoin, traceBlockSync, 1)
		}
		lock(&s.lock)
	}
	if raceenabled {
		raceacquire(unsafe.Pointer(s))
	}
	s.state = sgoFinalizing
	drain := s.drain // stopped below, outside the lock
	s.drain = nil
	r := sgoScopeResult{Outcome: sgoSucceeded, Children: s.nchild}
	for c := s.first; c != nil; c = c.sibling {
		switch c.outcome {
		case sgoFailed:
			r.Failures++
			if r.Outcome != sgoFailed {
				r.Outcome, r.Primary, r.Panic, r.Err = sgoFailed, c.id, c.panic, nil
			}
		case sgoErrored:
			r.Failures++
			if r.Outcome == sgoSucceeded {
				r.Outcome, r.Primary, r.Err = sgoErrored, c.id, c.err
			}
		case sgoCancelled:
			r.Cancelled++
		case sgoRunning:
			unlock(&s.lock)
			throw("sgo: closed scope has a running child") // OWN-3 defect
		}
	}
	s.snapshot = r
	s.state = sgoClosed
	// Drop child records so a retained handle keeps only the snapshot alive (plan §5.2).
	first := s.first
	s.first, s.last = nil, nil
	unlock(&s.lock)
	if drain != nil {
		drain.stop()
	}
	t.nest[len(t.nest)-1] = nil
	t.nest = t.nest[:len(t.nest)-1]
	// The children are finished and no longer change; collect the failed ones outside the lock.
	var bad []*sgoTask
	for c := first; c != nil; c = c.sibling {
		if c.outcome == sgoFailed || c.outcome == sgoErrored {
			bad = append(bad, c)
		}
	}
	return r, bad, nil
}

// sgoJoinPark is the lab's join park commit: it releases the scope lock (sgo_lab_sched.go).
func sgoJoinPark(gp *g, p unsafe.Pointer) bool {
	unlock(&(*sgoScope)(p).lock)
	return sgoLabCommit(gp)
}
