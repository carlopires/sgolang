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
