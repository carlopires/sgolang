// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Internal, non-distributed conformance hooks for the S-Go scope core (W02). This file is compiled only
// into the runtime's own tests; it is not part of any S-Go target and admits no source language.

package runtime

import "unsafe"

type SGoScope = sgoScope
type SGoTask = sgoTask

type SGoResult struct {
	Outcome   string
	Primary   uint64
	Err       error
	Panic     any
	Children  int64
	Failures  int64
	Cancelled int64
}

func sgoOutcomeName(o sgoOutcome) string {
	return [...]string{"Running", "Succeeded", "Cancelled", "Errored", "Failed"}[o]
}

func SGoEnterRoot()                        { sgoEnterRoot() }
func SGoLeaveRoot()                        { sgoLeaveRoot() }
func SGoOpen() (*SGoScope, error)          { return sgoNurseryOpen() }
func SGoIsTask() bool                      { return sgoCurrent() != nil }
func SGoTaskID(t *SGoTask) uint64          { return t.id }
func SGoTaskScope(t *SGoTask) *SGoScope    { return t.scope }
func SGoCurrent(t *SGoTask) *SGoScope      { return t.currentScope() }
func SGoScopeID(s *SGoScope) uint64        { return s.id }
func SGoScopeParent(s *SGoScope) *SGoScope { return s.parent }

func SGoScopeState(s *SGoScope) string {
	lock(&s.lock)
	st := s.state
	unlock(&s.lock)
	return st.String()
}

func SGoSpawn(s *SGoScope, fn func(*SGoTask) error) error { return sgoSpawn(s, fn) }

// Compiler entry points of the nursery statement (sgo_nursery.go), as function values so that the
// caller's frame (the test function) is the frame that executes the nursery.
var (
	SGoNurseryEnter = sgoNurseryEnter // func() unsafe.Pointer
	SGoNurseryExit  = sgoNurseryExit  // func(unsafe.Pointer)
)

func SGoScopeOf(p unsafe.Pointer) *SGoScope { return (*sgoScope)(p) }

// SGoNest reports how many nurseries the calling task has open.
func SGoNest() int { return len(sgoCurrent().nest) }

func SGoJoin(s *SGoScope) (SGoResult, error) {
	r, _, err := sgoNurseryJoin(s)
	return SGoResult{sgoOutcomeName(r.Outcome), r.Primary, r.Err, r.Panic, r.Children, r.Failures, r.Cancelled}, err
}

// W04 cancellation hooks (sgo_cancel.go).

func SGoCancel(s *SGoScope) error   { return sgoNurseryCancel(s) }
func SGoCheckpoint() error          { return sgoTaskCancellation(sgoCurrent()) }
func SGoSleep(ms int64) error       { return sgoSleepFor(sgoCurrent(), ms) }
func SGoIsCancelled(err error) bool { return sgoIsCancelled(err) }

// SGoSleepers reports how many Sleep calls are registered.
func SGoSleepers() int {
	n := 0
	lock(&sgoSleepLock)
	for r := sgoSleepers; r != nil; r = r.next {
		n++
	}
	unlock(&sgoSleepLock)
	return n
}

// SGoCurrentTaskID reports the calling goroutine's task ID, if it is an S-Go task.
func SGoCurrentTaskID() (uint64, bool) {
	t := sgoCurrent()
	if t == nil {
		return 0, false
	}
	return t.id, true
}

// SGoSetProgram marks the process as an S-Go program, as sgoEnterMain does (child-process tests).
func SGoSetProgram() { sgoProgram = true }

// SGoSetEnforce switches go-statement enforcement (sgo_service.go) for child-process tests.
func SGoSetEnforce(on bool) { sgoEnforce.Store(on) }

// SGoServiceStarted reports how many goroutines of the named service kind were started.
func SGoServiceStarted(name string) int64 {
	for k, n := range sgoServiceNames {
		if n == name {
			return sgoServiceStarted[k].Load()
		}
	}
	return -1
}

// SGoServiceCensus lists live goroutines started by a runtime function that are neither S-Go tasks,
// the main goroutine, nor classified runtime services.
func SGoServiceCensus() (unclassified []string, services map[string]int) {
	services = map[string]int{}
	forEachG(func(gp *g) {
		if readgstatus(gp) == _Gdead {
			return
		}
		f := findfunc(gp.startpc)
		if !f.valid() {
			return
		}
		name := funcname(f)
		switch {
		case gp.sgo != nil || name == "runtime.main" || name == "runtime.sgoTaskStart":
		case gp.sgoService != 0:
			services[sgoServiceNames[gp.sgoService]]++
		case len(name) > 8 && name[:8] == "runtime.":
			unclassified = append(unclassified, name)
		}
	})
	return
}

// W06 channel hooks (sgo_chan.go). Values are ints boxed by the test, as package sgo boxes T.
type SGoChan = sgoChan
type SGoPermit = sgoPermit

func SGoChanNew(capacity int64) *SGoChan { return sgoChanNew(capacity) }
func SGoReserve(c *SGoChan) (*SGoPermit, error) {
	return sgoChanReserve(c, sgoCurrent())
}
func SGoCommit(p *SGoPermit, v int) error {
	b := new(int)
	*b = v
	return sgoChanCommit(p, unsafe.Pointer(b))
}
func SGoAbort(p *SGoPermit) { sgoChanAbort(p) }
func SGoReceive(c *SGoChan) (int, bool, error) {
	v, ok, err := sgoChanReceive(c, sgoCurrent())
	if !ok {
		return 0, false, err
	}
	return *(*int)(v), true, nil
}
func SGoClose(c *SGoChan) { sgoChanClose(c) }

// SGoChanRing reports the address of the value ring, its length, and how many of its slots hold a
// value (a received value's slot is cleared) (W12 R9).
func SGoChanRing(c *SGoChan) (addr uintptr, n int, live int) {
	lock(&c.lock)
	defer unlock(&c.lock)
	if len(c.buf) == 0 {
		return 0, 0, 0
	}
	for _, v := range c.buf {
		if v != nil {
			live++
		}
	}
	return uintptr(unsafe.Pointer(&c.buf[0])), len(c.buf), live
}
func SGoPermitState(p *SGoPermit) string { return sgoPermitName(p.state) }

// SGoChanQueued reports the values committed and not yet received.
func SGoChanQueued(c *SGoChan) []int {
	lock(&c.lock)
	defer unlock(&c.lock)
	out := []int{}
	for i := int64(0); i < c.count; i++ {
		out = append(out, *(*int)(c.qat(i)))
	}
	return out
}

// SGoChanState reports the reservation count and the parked waiters of c.
func SGoChanState(c *SGoChan) (reserved int64, senders, recvs int) {
	lock(&c.lock)
	defer unlock(&c.lock)
	for w := c.senders.first; w != nil; w = w.next {
		senders++
	}
	for w := c.recvs.first; w != nil; w = w.next {
		recvs++
	}
	return c.reserved, senders, recvs
}

// SGoAbandoned reports how many permits the runtime aborted for the calling task.
func SGoAbandoned() int64 { return sgoCurrent().abandoned }

// SGoSetChanFault installs f at the channel fault-injection points (nil removes it).
func SGoSetChanFault(f func(point int)) { sgoChanFault = f }

const SGoChanFaultPoints = sgoFaultPoints

// SGoCommitPtr and SGoReceivePtr send and receive an arbitrary pointer (H11 lifetime tests).
func SGoCommitPtr(p *SGoPermit, v unsafe.Pointer) error { return sgoChanCommit(p, v) }
func SGoReceivePtr(c *SGoChan) (unsafe.Pointer, bool, error) {
	return sgoChanReceive(c, sgoCurrent())
}

// W05 readiness hooks (sgo_ready.go).
func SGoStart(s *SGoScope, fn func(*SGoTask) error) error { return sgoStart(s, fn) }

// SGoReadySignal is ready.Signal() for the calling task's readiness record.
func SGoReadySignal() { sgoReadySignal(sgoCurrent().ready) }

// SGoReadyWaiters reports how many starters are parked waiting for readiness.
func SGoReadyWaiters() int {
	n := 0
	lock(&sgoSleepLock)
	for r := sgoReadyWaiters; r != nil; r = r.next {
		n++
	}
	unlock(&sgoSleepLock)
	return n
}

// SGoIsProgram exposes sgoIsProgram: false in binaries linked by the upstream linker (W12 R2).
func SGoIsProgram() bool { return sgoIsProgram() }

// SGoGetenv exposes gogetenv, which an S-Go program answers with "" for every key (W12 R2).
func SGoGetenv(key string) string { return gogetenv(key) }

// SGoCopy exposes sgo_copy, sgo.Copy for the dynamic type of v (SGO-COMP-004). Values passed here
// must not hold Go maps: S-Go maps are the runtime's ordered map (sgo_map.go).
func SGoCopy(v any) any { return sgo_copy(v) }
