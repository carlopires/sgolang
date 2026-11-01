// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import (
	"fmt"
	"sort"
	"strings"
)

// hBugCloseWhileLent lets the owner close a lent handle (the model's seeded-bug test only).
var hBugCloseWhileLent bool

// hSteps returns the states reachable by one step of task i (none if it is done or waits).
func hSteps(sc HScenario, s *hState, i int) []*hState {
	script := sc.Tasks[i]
	t := &s.tasks[i]
	if t.pc >= len(script) {
		return nil
	}
	op := script[t.pc]
	next := func(f func(n *hState)) []*hState {
		n := s.clone()
		f(n)
		return []*hState{n}
	}
	done := func(n *hState, r string) {
		nt := &n.tasks[i]
		nt.results = append(nt.results, r)
		nt.pc++
		nt.active, nt.progress = false, 0
	}
	h := s.h[op.H]
	switch op.Kind {
	case HOpen:
		return next(func(n *hState) {
			n.h[op.H] = hHandle{gen: h.gen + 1, open: true, space: sc.Cap}
			done(n, "open")
		})
	case HClose:
		return next(func(n *hState) {
			switch {
			case i != 0:
				done(n, "err NotOwner")
			case !h.open:
				done(n, "err HandleClosed")
			case h.lent && !hBugCloseWhileLent:
				done(n, "err HandleLent")
			default:
				n.h[op.H].open = false
				done(n, "closed")
			}
		})
	case HLend:
		return next(func(n *hState) { n.h[op.H].lent = true; done(n, "lent") })
	case HUnlend:
		// The nursery closes only after its children end: wait for every borrower.
		for j := 1; j < len(sc.Tasks); j++ {
			if (!sc.Peer || j != len(sc.Tasks)-1) && s.tasks[j].pc < len(sc.Tasks[j]) {
				return nil
			}
		}
		return next(func(n *hState) { n.h[op.H].lent = false; done(n, "unlent") })
	case HCancel:
		return next(func(n *hState) { n.tasks[op.N].cancelled = true; done(n, "cancel") })
	case HDeliver:
		if !h.open {
			return nil // the peer sends only on an open connection
		}
		return next(func(n *hState) {
			n.h[op.H].readable += op.N
			n.h[op.H].delivered += op.N
			done(n, "")
		})
	case HDrain:
		return next(func(n *hState) { n.h[op.H].space += op.N; done(n, "") })
	}
	// HRead, HWrite. A borrower runs only while the handle is lent (children are spawned after
	// the lend, and the lend ends only after they end).
	if i != 0 && !h.lent {
		return nil
	}
	if !h.open || (t.active && t.gen != h.gen) {
		p := 0
		if t.active {
			p = t.progress
		}
		return next(func(n *hState) { done(n, fmt.Sprintf("err HandleClosed %d", p)) })
	}
	gen, progress := h.gen, 0
	if t.active {
		progress = t.progress
	}
	sizes := func(avail int) []int {
		if avail == 1 {
			return []int{1}
		}
		return []int{1, avail}
	}
	if op.Kind == HRead {
		if h.readable == 0 {
			if t.cancelled {
				return next(func(n *hState) { done(n, "err Cancelled 0") })
			}
			return nil // waits
		}
		var out []*hState
		for _, k := range sizes(min(h.readable, op.N)) {
			out = append(out, next(func(n *hState) {
				n.h[op.H].readable -= k
				n.h[op.H].consumed += k
				done(n, fmt.Sprintf("read %d", k))
			})...)
		}
		return out
	}
	if h.space == 0 {
		if t.cancelled {
			return next(func(n *hState) { done(n, fmt.Sprintf("err Cancelled %d", progress)) })
		}
		return nil // waits
	}
	var out []*hState
	for _, k := range sizes(min(h.space, op.N-progress)) {
		out = append(out, next(func(n *hState) {
			n.h[op.H].space -= k
			n.h[op.H].written += k
			nt := &n.tasks[i]
			nt.active, nt.gen, nt.progress = true, gen, progress+k
			if nt.progress == op.N {
				done(n, fmt.Sprintf("wrote %d", op.N))
			}
		})...)
	}
	return out
}

// ExploreHandles returns the outcome set of a scenario: per task (the peer excluded) its results
// in order, with "stuck" if it never finished; tasks joined by " | ". Every reachable state is
// checked against the contract's invariants.
func ExploreHandles(sc HScenario) ([]string, error) {
	n := len(sc.Tasks)
	start := &hState{tasks: make([]hTask, n)}
	maxH := 0
	for _, script := range sc.Tasks {
		for _, op := range script {
			maxH = max(maxH, op.H+1)
		}
	}
	start.h = make([]hHandle, maxH)
	seen := map[string]bool{}
	outcomes := map[string]bool{}
	var visit func(s *hState) error
	visit = func(s *hState) error {
		k := s.key()
		if seen[k] {
			return nil
		}
		seen[k] = true
		if err := s.check(); err != nil {
			return err
		}
		moved := false
		for i := range sc.Tasks {
			for _, ns := range hSteps(sc, s, i) {
				moved = true
				if err := visit(ns); err != nil {
					return err
				}
			}
		}
		if !moved {
			var parts []string
			for i, t := range s.tasks {
				if sc.Peer && i == n-1 {
					continue
				}
				r := strings.Join(nonEmpty(t.results), ",")
				if t.pc < len(sc.Tasks[i]) {
					r += ",stuck"
				}
				parts = append(parts, r)
			}
			outcomes[strings.Join(parts, " | ")] = true
		}
		return nil
	}
	if err := visit(start); err != nil {
		return nil, err
	}
	var out []string
	for o := range outcomes {
		out = append(out, o)
	}
	sort.Strings(out)
	return out, nil
}

func nonEmpty(a []string) []string {
	var out []string
	for _, s := range a {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
