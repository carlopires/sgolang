package model

import (
	"fmt"
	"slices"
	"strings"
)

// NResult summarizes an exploration of a native scenario.
type NResult struct {
	States, Terminal, Blocked int
	Violation, Trace          string
	Outcomes                  map[string]bool
}

// nCheck checks the invariants of s; final reports that no step is enabled.
func nCheck(sc NScenario, s *nState, final bool) string {
	for c, q := range s.queues {
		if sc.Caps[c] > 0 && len(q) > sc.Caps[c] {
			return fmt.Sprintf("c%d holds %d values, capacity %d", c, len(q), sc.Caps[c])
		}
		if sc.Caps[c] == 0 && len(q) != 0 {
			return fmt.Sprintf("capacity-0 c%d buffered a value", c)
		}
		// Receive order is a prefix of commit order (FIFO); nothing is received twice or uncommitted.
		got, want := s.received[c], s.commits[c]
		if len(got) > len(want) || !slices.Equal(got, want[:len(got)]) {
			return fmt.Sprintf("c%d received %v, not a prefix of the commit order %v", c, got, want)
		}
		if final && len(got)+len(q) != len(want) {
			return fmt.Sprintf("c%d lost a committed value", c)
		}
	}
	for i, n := range s.selects {
		if n > 1 {
			return fmt.Sprintf("t%d: %d select arms took effect", i, n)
		}
	}
	for e, ok := range s.waitsOK {
		if ok && !s.set[e] {
			return fmt.Sprintf("Wait on e%d returned before Set", e)
		}
	}
	for l, h := range s.held {
		owned := 0
		for _, t := range s.tasks {
			for _, x := range t.tokens {
				if x == l {
					owned++
				}
			}
		}
		if h < 0 || h > sc.Limits[l] || h != owned {
			return fmt.Sprintf("l%d: %d held, %d owned, limit %d", l, h, owned, sc.Limits[l])
		}
		for _, w := range s.lwait[l] {
			if s.tasks[w].outcome != "" || !s.tasks[w].waiting {
				return fmt.Sprintf("l%d: t%d is queued but not waiting", l, w)
			}
		}
	}
	if final {
		for l, h := range s.held {
			done := true
			for _, t := range s.tasks {
				done = done && t.outcome != ""
			}
			if done && h != 0 {
				return fmt.Sprintf("l%d: %d tokens leaked after every task ended", l, h)
			}
		}
	}
	return ""
}

func nOutcome(s *nState) string {
	var b strings.Builder
	for i, t := range s.tasks {
		o := t.outcome
		if o == "" {
			o = "blocked"
		}
		fmt.Fprintf(&b, "t%d=%s ", i, o)
	}
	for c := range s.received {
		fmt.Fprintf(&b, "c%d=%v/%v ", c, s.received[c], s.queues[c])
	}
	fmt.Fprintf(&b, "diags=%d", s.diags)
	return b.String()
}

// nKey identifies a state for memoization: everything but the trace.
func nKey(s *nState) string {
	return fmt.Sprint(s.queues, s.closed, s.set, s.held, s.lwait, s.commits, s.received, s.selects,
		s.diags, s.waitsOK, s.tasks)
}

// ExploreNative enumerates every interleaving of sc (memoized on the state) and checks the
// invariants after each step.
func ExploreNative(sc NScenario) *NResult {
	r := &NResult{Outcomes: map[string]bool{}}
	seen := map[string]bool{}
	var walk func(s *nState)
	walk = func(s *nState) {
		if r.Violation != "" {
			return
		}
		k := nKey(s)
		if seen[k] {
			return
		}
		seen[k] = true
		r.States++
		next := nSteps(sc, s)
		if v := nCheck(sc, s, len(next) == 0); v != "" {
			r.Violation, r.Trace = v, strings.Join(s.trace, "; ")
			return
		}
		if len(next) == 0 {
			done := true
			for _, t := range s.tasks {
				done = done && t.outcome != ""
			}
			if done {
				r.Terminal++
			} else {
				r.Blocked++
			}
			r.Outcomes[nOutcome(s)] = true
			return
		}
		for _, n := range next {
			walk(n)
		}
	}
	walk(newNState(sc))
	return r
}
