package model

import (
	"fmt"
	"sort"
)

// Result summarizes an exhaustive exploration.
type Result struct {
	States    int                 // distinct states visited
	Outcomes  map[string]int      // terminal outcome -> number of terminal states reaching it
	Examples  map[string][]string // terminal outcome -> one trace reaching it
	Violation string              // first invariant violation, with its trace; empty if none
	Terminals []*State            // every distinct terminal state
}

// OutcomeSet returns the sorted terminal outcomes.
func (r *Result) OutcomeSet() []string {
	var o []string
	for k := range r.Outcomes {
		o = append(o, k)
	}
	sort.Strings(o)
	return o
}

func outcomeKey(s *State) string {
	if s.Exit == -1 {
		return "stuck"
	}
	if s.ExitKind == "" {
		return fmt.Sprintf("exit=%d", s.Exit)
	}
	return fmt.Sprintf("exit=%d %s: %s", s.Exit, s.ExitKind, s.ExitMsg)
}

// Explore enumerates every interleaving of p up to maxStates distinct states.
func Explore(p Program, cfg Config, maxStates int) *Result {
	r := &Result{Outcomes: map[string]int{}, Examples: map[string][]string{}}
	seen := map[string]bool{}
	var dfs func(s *State)
	dfs = func(s *State) {
		if r.Violation != "" || r.States >= maxStates {
			return
		}
		k := s.Key()
		if seen[k] {
			return
		}
		seen[k] = true
		r.States++
		evs := s.Enabled()
		if len(evs) == 0 {
			o := outcomeKey(s)
			r.Outcomes[o]++
			r.Terminals = append(r.Terminals, s)
			if _, ok := r.Examples[o]; !ok {
				r.Examples[o] = s.Trace
			}
			return
		}
		for _, e := range evs {
			n := s.Apply(e)
			if v := Check(s, n); v != "" {
				r.Violation = v + "\ntrace:\n" + join(n.Trace)
				return
			}
			dfs(n)
		}
	}
	init := NewState(p, &cfg)
	if v := Check(init, init); v != "" {
		r.Violation = v
		return r
	}
	dfs(init)
	if r.States >= maxStates && r.Violation == "" {
		r.Violation = fmt.Sprintf("state limit %d reached; exploration incomplete", maxStates)
	}
	return r
}

// Run follows the first enabled event at every step: one deterministic schedule (golden histories).
func Run(p Program, cfg Config) (*State, string) {
	s := NewState(p, &cfg)
	for steps := 0; steps < 10000; steps++ {
		evs := s.Enabled()
		if len(evs) == 0 {
			return s, ""
		}
		n := s.Apply(evs[0])
		if v := Check(s, n); v != "" {
			return n, v
		}
		s = n
	}
	return s, "step limit reached"
}

func join(lines []string) string {
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}

var rank = map[ScopeState]int{Open: 0, Joining: 1, Cancelling: 2, Finalizing: 3, Closed: 4}

// Check verifies the invariants of plan §3 on the transition prev -> next.
func Check(prev, next *State) string {
	for _, sc := range next.Scopes {
		// CAN-1: cancellation is sticky; states only move forward; the first cause is kept.
		if sc.ID < len(prev.Scopes) {
			p := prev.Scopes[sc.ID]
			if rank[sc.State] < rank[p.State] {
				return fmt.Sprintf("CAN-1: S%d went from %s back to %s", sc.ID, p.State, sc.State)
			}
			if p.Cause != "" && sc.Cause != p.Cause {
				return fmt.Sprintf("CAN-1: S%d cause changed from %q to %q", sc.ID, p.Cause, sc.Cause)
			}
		}
		if sc.Cause != "" && rank[sc.State] < rank[Cancelling] {
			return fmt.Sprintf("CAN-1: S%d has cause %q in state %s", sc.ID, sc.Cause, sc.State)
		}
		for id := range sc.Live {
			if next.Tasks[id].status == done {
				return fmt.Sprintf("OWN-3: S%d lists finished T%d as live", sc.ID, id)
			}
		}
		// OWN-3 / CAN-5: a closed scope has no live children.
		if sc.State == Closed {
			if len(sc.Live) > 0 {
				return fmt.Sprintf("OWN-3: S%d CLOSED with live children %v", sc.ID, sc.Live)
			}
			for _, id := range sc.Children {
				if next.Tasks[id].status != done {
					return fmt.Sprintf("OWN-4: S%d CLOSED while child T%d has not finished", sc.ID, id)
				}
			}
		}
	}
	for _, t := range next.Tasks {
		if t.ID == 0 {
			continue
		}
		// OWN-1: every task belongs to exactly one scope and is recorded there.
		if t.Scope < 0 || t.Scope >= len(next.Scopes) {
			return fmt.Sprintf("OWN-1: T%d has no owning scope", t.ID)
		}
		found := false
		for _, c := range next.Scopes[t.Scope].Children {
			found = found || c == t.ID
		}
		if !found {
			return fmt.Sprintf("OWN-1: T%d missing from S%d", t.ID, t.Scope)
		}
		// OWN-2: an unfinished task is registered live in its scope from the moment it exists.
		if t.status != done && !next.Scopes[t.Scope].Live[t.ID] {
			return fmt.Sprintf("OWN-2: T%d exists but is not registered live in S%d", t.ID, t.Scope)
		}
	}
	// M04: a child's completion is published exactly once.
	for id, n := range next.published {
		if n > 1 {
			return fmt.Sprintf("M04: completion of T%d published %d times", id, n)
		}
	}
	// CAN-2: every wait has at most one terminal disposition.
	for w, n := range next.waitsDone {
		if n > 1 {
			return fmt.Sprintf("CAN-2: wait %d resolved %d times", w, n)
		}
	}
	// SGO-PROC-002: statuses 0-2 only after every task finished.
	if next.Exit >= 0 && next.Exit <= 2 {
		for _, t := range next.Tasks {
			if t.status != done {
				return fmt.Sprintf("OWN-3: process exit %d while T%d has not finished", next.Exit, t.ID)
			}
		}
	}
	return ""
}
