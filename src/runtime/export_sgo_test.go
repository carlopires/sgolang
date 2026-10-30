// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Internal, non-distributed conformance hooks for the S-Go scope core (W02). This file is compiled only
// into the runtime's own tests; it is not part of any S-Go target and admits no source language.

package runtime

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

func SGoJoin(s *SGoScope) (SGoResult, error) {
	r, err := sgoNurseryJoin(s)
	return SGoResult{sgoOutcomeName(r.Outcome), r.Primary, r.Err, r.Panic, r.Children, r.Failures, r.Cancelled}, err
}

// SGoCurrentTaskID reports the calling goroutine's task ID, if it is an S-Go task.
func SGoCurrentTaskID() (uint64, bool) {
	t := sgoCurrent()
	if t == nil {
		return 0, false
	}
	return t.id, true
}
