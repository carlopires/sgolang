package model

import "fmt"

// Event is one atomic transition.
type Event struct {
	Kind  string // "step", "expire", "wake-cancel", "budget"
	Task  int
	Scope int
}

func (e Event) String() string {
	if e.Kind == "budget" {
		return fmt.Sprintf("budget(S%d)", e.Scope)
	}
	return fmt.Sprintf("%s(T%d)", e.Kind, e.Task)
}

// Enabled lists the events possible in s, in a fixed order.
func (s *State) Enabled() []Event {
	if s.Exit != -1 {
		return nil
	}
	var evs []Event
	for _, t := range s.Tasks {
		switch t.status {
		case running:
			evs = append(evs, Event{"step", t.ID, -1})
		case joined:
			sc := s.Scopes[t.frames[len(t.frames)-1].nursery]
			if len(sc.Live) == 0 || s.cfg.Mut == MutEarlyJoin {
				evs = append(evs, Event{"step", t.ID, -1})
			}
		case sleeping:
			// SGO-WAIT-001: time may pass; cancellation, if requested, may also win (allowed race).
			evs = append(evs, Event{"expire", t.ID, -1})
			if s.cancelled(t) {
				evs = append(evs, Event{"wake-cancel", t.ID, -1})
			}
		}
		if s.cfg.Mut == MutDoubleWake && t.status == running && t.waitID != 0 {
			evs = append(evs, Event{"expire", t.ID, -1})
		}
	}
	if s.cfg.DrainBudget {
		// SGO-DRAIN-002: a scope still CANCELLING with live children may exceed its budget.
		for _, sc := range s.Scopes {
			if sc.State == Cancelling && len(sc.Live) > 0 {
				evs = append(evs, Event{"budget", -1, sc.ID})
			}
		}
	}
	return evs
}

// Apply returns the state after event e; s is not modified.
func (s *State) Apply(e Event) *State {
	n := s.Clone()
	n.Trace = append(n.Trace, e.String())
	switch e.Kind {
	case "step":
		n.stepTask(n.Tasks[e.Task])
	case "expire":
		t := n.Tasks[e.Task]
		n.waitsDone[t.waitID]++ // CAN-2
		if n.cfg.Mut != MutDoubleWake || t.status == sleeping {
			t.waitID = 0
		}
		t.status = running
		n.logf("T%d sleep expired", t.ID)
	case "wake-cancel":
		t := n.Tasks[e.Task]
		n.waitsDone[t.waitID]++ // CAN-2
		if n.cfg.Mut != MutDoubleWake {
			t.waitID = 0
		}
		t.status = running
		n.observeCancel(t, t.frames[len(t.frames)-1].code[t.frames[len(t.frames)-1].pc-1])
	case "budget":
		n.Exit, n.ExitKind, n.ExitMsg = 3, "DrainBudgetExceeded", fmt.Sprintf("S%d", e.Scope)
		n.logf("drain budget exceeded for S%d", e.Scope)
	}
	return n
}

func (s *State) logf(format string, args ...any) {
	s.Trace = append(s.Trace, "  "+fmt.Sprintf(format, args...))
}

// linked returns the scopes linked to t: its spawn scope and every nursery it executes inside
// (SGO-CANCEL-004).
func (s *State) linked(t *Task) []int {
	var ids []int
	if t.Scope >= 0 {
		ids = append(ids, t.Scope)
	}
	for _, f := range t.frames {
		if f.nursery >= 0 {
			ids = append(ids, f.nursery)
		}
	}
	return ids
}

// cancelled reports whether t's context reports cancellation (SGO-CANCEL-002): some linked scope is
// CANCELLING. Stickiness is per scope; MutTaskStickyCancel makes it per task (wrong).
func (s *State) cancelled(t *Task) bool {
	if s.cfg.Mut == MutTaskStickyCancel && s.taskSticky[t.ID] {
		return true
	}
	for _, id := range s.linked(t) {
		if s.Scopes[id].State == Cancelling {
			return true
		}
	}
	return false
}

// currentScope is t's innermost open nursery, else its spawn scope (SGO-NUR-005).
func (s *State) currentScope(t *Task) int {
	for i := len(t.frames) - 1; i >= 0; i-- {
		if t.frames[i].nursery >= 0 {
			return t.frames[i].nursery
		}
	}
	return t.Scope
}

// cancelScope moves a scope to CANCELLING and propagates to nested scopes of linked tasks
// (SGO-CANCEL-001). Only the first cause is recorded (CAN-1).
func (s *State) cancelScope(id int, cause string) {
	sc := s.Scopes[id]
	if sc.State != Open && sc.State != Joining {
		return
	}
	sc.State, sc.Cause = Cancelling, cause
	s.logf("S%d CANCELLING (%s)", id, cause)
	inner := func(t *Task, from int) {
		for i := from; i < len(t.frames); i++ {
			if t.frames[i].nursery >= 0 {
				s.cancelScope(t.frames[i].nursery, fmt.Sprintf("ancestor scope %d cancelled", id))
			}
		}
	}
	owner := s.Tasks[sc.Owner]
	for i, f := range owner.frames {
		if f.nursery == id {
			inner(owner, i+1)
		}
	}
	for c := range sc.Live {
		inner(s.Tasks[c], 0)
	}
}

// observeCancel applies the result of a cancellation error at instruction in (SGO-CANCEL-002).
func (s *State) observeCancel(t *Task, in Instr) {
	if s.cfg.Mut == MutTaskStickyCancel {
		s.taskSticky[t.ID] = true
	}
	s.Trace = append(s.Trace, fmt.Sprintf("  T%d observed cancellation", t.ID))
	if !in.Continue {
		t.returning = &Value{Err: true, Cancelled: true, Msg: "cancelled"}
	}
}
