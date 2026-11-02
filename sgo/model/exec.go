// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import (
	"fmt"
	"sort"
)

func (s *State) stepTask(t *Task) {
	if s.cfg.Mut == MutLateRegister && t.ID != 0 && !s.observedRun[t.ID] {
		s.Scopes[t.Scope].Live[t.ID] = true // wrong: registration after publication (breaks OWN-2)
	}
	s.observedRun[t.ID] = true
	if t.status == joined {
		s.finalize(t)
		return
	}
	f := &t.frames[len(t.frames)-1]
	if t.unwinding != nil || t.returning != nil {
		s.pop(t)
		return
	}
	if f.pc >= len(f.code) {
		if f.nursery >= 0 {
			s.beginJoin(t, f) // body ended normally (SGO-NUR-003)
		} else {
			t.returning = &Value{} // falling off the end returns nil
		}
		return
	}
	in := f.code[f.pc]
	f.pc++
	s.exec(t, f, in)
}

func (s *State) exec(t *Task, f *frame, in Instr) {
	switch in.Op {
	case OpNursery: // SGO-NUR-001
		id := len(s.Scopes)
		s.Scopes = append(s.Scopes, &Scope{ID: id, Owner: t.ID, Live: map[int]bool{},
			Outcomes: map[int]string{}, Errors: map[int]string{}, Fails: map[int]*Failure{}})
		wasCancelled := s.cancelled(t)
		env := copyEnv(f.env)
		env[in.Name] = id
		t.frames = append(t.frames, frame{code: in.Body, env: env, nursery: id})
		s.logf("T%d opens S%d", t.ID, id)
		if wasCancelled {
			s.cancelScope(id, "ancestor already cancelled")
		}
	case OpSpawn: // SGO-SPAWN-002/003
		id, ok := f.env[in.Name]
		sc := s.Scopes[id]
		cur := s.currentScope(t)
		member := ok && (cur == id || s.cfg.Mut == MutNoCurrentScopeCheck && (sc.Owner == t.ID || t.Scope == id))
		switch {
		case member && sc.State == Cancelling:
			s.logf("T%d spawn into CANCELLING S%d: no child (R5)", t.ID, id)
		case member && (sc.State == Open || sc.State == Joining):
			c := &Task{ID: len(s.Tasks), Scope: id, frames: []frame{{code: in.Child, env: copyEnv(f.env), nursery: -1}}}
			s.Tasks = append(s.Tasks, c)
			sc.Children = append(sc.Children, c.ID)
			if s.cfg.Mut != MutLateRegister {
				sc.Live[c.ID] = true // registered before it can run (OWN-2)
			}
			s.logf("T%d spawns T%d into S%d", t.ID, c.ID, id)
		default:
			t.unwinding = &Failure{Kind: "SpawnContractViolation", Msg: fmt.Sprintf("T%d spawn into S%d (%s)", t.ID, id, sc.State)}
			s.logf("T%d SpawnContractViolation", t.ID)
		}
	case OpCancel: // SGO-CANCEL-001
		id := f.env[in.Name]
		if s.currentScope(t) != id {
			t.unwinding = &Failure{Kind: "SpawnContractViolation", Msg: fmt.Sprintf("T%d cancel of S%d", t.ID, id)}
			return
		}
		s.cancelScope(id, fmt.Sprintf("Cancel by task %d", t.ID))
	case OpCheckpoint: // SGO-CANCEL-002
		if s.cancelled(t) {
			s.observeCancel(t, in)
		} else {
			s.logf("T%d checkpoint ok", t.ID)
		}
	case OpSleep: // SGO-WAIT-001
		if s.cancelled(t) {
			s.observeCancel(t, in)
			return
		}
		s.nextWait++
		t.waitID, t.status = s.nextWait, sleeping
	case OpReturnNil:
		t.returning = &Value{}
	case OpReturnErr:
		t.returning = &Value{Err: true, Msg: in.Msg}
	case OpPanic:
		t.unwinding = &Failure{Kind: "Panic", Msg: in.Msg}
	case OpSpin:
		t.status = spinning
		s.logf("T%d spins (non-cooperative)", t.ID)
	case OpMark:
		s.Trace = append(s.Trace, fmt.Sprintf("  mark T%d %s", t.ID, in.Msg))
	}
}

func (s *State) beginJoin(t *Task, f *frame) {
	sc := s.Scopes[f.nursery]
	if sc.State == Open {
		sc.State = Joining
	}
	f.joining = true
	t.status = joined
	s.logf("T%d joins S%d (%s)", t.ID, sc.ID, sc.State)
}

// pop leaves the top frame while returning or unwinding (SGO-CTL-004, SGO-DEF-005).
func (s *State) pop(t *Task) {
	f := &t.frames[len(t.frames)-1]
	if f.nursery >= 0 {
		if t.unwinding != nil {
			s.cancelScope(f.nursery, "unwind")
		}
		s.beginJoin(t, f)
		return
	}
	t.frames = t.frames[:len(t.frames)-1]
	s.end(t)
}

// finalize completes a joined nursery and applies its outcome (SGO-NUR-003, SGO-NUR-004).
func (s *State) finalize(t *Task) {
	f := t.frames[len(t.frames)-1]
	sc := s.Scopes[f.nursery]
	sc.State = Finalizing
	sc.State = Closed
	if s.closedOnce[sc.ID] {
		panic(fmt.Sprintf("model defect: S%d closed twice", sc.ID))
	}
	s.closedOnce[sc.ID] = true
	t.frames = t.frames[:len(t.frames)-1]
	t.status = running
	ids := append([]int(nil), sc.Children...)
	sort.Ints(ids)
	if s.cfg.Mut == MutLatestFailure {
		sort.Sort(sort.Reverse(sort.IntSlice(ids)))
	}
	var failed, errored []int
	for _, id := range ids {
		switch sc.Outcomes[id] {
		case Failed:
			failed = append(failed, id)
		case Errored:
			errored = append(errored, id)
		}
	}
	chain := func(skip int) []string {
		var c []string
		for _, id := range append(failed, errored...) {
			if id != skip {
				c = append(c, fmt.Sprintf("T%d %s", id, sc.Outcomes[id]))
			}
		}
		return c
	}
	switch {
	case t.unwinding != nil: // rule 1: the owner's failure stays primary
		t.unwinding.Chain = append(t.unwinding.Chain, chain(-1)...)
	case len(failed) > 0: // rule 2: lowest creation ID
		p := *sc.Fails[failed[0]]
		p.Chain = append(append([]string(nil), p.Chain...), chain(failed[0])...)
		t.unwinding, t.returning = &p, nil
	case len(errored) > 0: // rule 3
		t.unwinding = &Failure{Kind: "ChildError", Msg: sc.Errors[errored[0]], Chain: chain(errored[0])}
		t.returning = nil
	}
	s.logf("T%d closes S%d", t.ID, sc.ID)
}

// end handles a task whose last frame was popped.
func (s *State) end(t *Task) {
	if len(t.frames) > 0 {
		return
	}
	t.status = done
	if t.ID == 0 { // SGO-PROC-002
		switch {
		case t.unwinding != nil && t.unwinding.Kind == "ChildError":
			s.Exit, s.ExitKind, s.ExitMsg = 1, "ChildError", t.unwinding.Msg
		case t.unwinding != nil:
			s.Exit, s.ExitKind, s.ExitMsg = 2, t.unwinding.Kind, t.unwinding.Msg
		case t.returning != nil && t.returning.Err:
			s.Exit, s.ExitKind, s.ExitMsg = 1, "MainError", t.returning.Msg
		default:
			s.Exit, s.ExitKind = 0, ""
		}
		if s.ExitKind == "" {
			s.logf("process exit %d", s.Exit)
		} else {
			s.logf("process exit %d %s: %s", s.Exit, s.ExitKind, s.ExitMsg)
		}
		return
	}
	sc := s.Scopes[t.Scope] // SGO-SPAWN-004
	switch {
	case t.unwinding != nil:
		t.Outcome, t.Fail = Failed, t.unwinding
		sc.Fails[t.ID] = t.unwinding
	case t.returning != nil && t.returning.Cancelled:
		t.Outcome = Cancelled
	case t.returning != nil && t.returning.Err:
		t.Outcome = Errored
		sc.Errors[t.ID] = t.returning.Msg
	default:
		t.Outcome = Succeeded
	}
	sc.Outcomes[t.ID] = t.Outcome
	delete(sc.Live, t.ID)
	s.published[t.ID]++
	if s.cfg.Mut == MutDuplicateCompletion {
		s.published[t.ID]++ // wrong: the completion is published a second time
	}
	s.logf("T%d ends %s", t.ID, t.Outcome)
	if (t.Outcome == Failed || t.Outcome == Errored) && s.cfg.Mut != MutNoCancelOnFail {
		s.cancelScope(sc.ID, fmt.Sprintf("child %d failed", t.ID))
	}
}
