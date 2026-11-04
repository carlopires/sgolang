// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

// W22 protocols (D170–D172), explored over every interleaving: the group result protocol of
// sgo.NewGroup (src/runtime/sgo_group.go), the re-arming of scope timers (sgo_timer.go), and the
// lock discipline of the lab's deadline driver (sgo_deadline.go).
//
// Group: an owner starts K children through g.Go with limit L (0: none), then receives R results.
// A child publishes its result, then releases its slot. Publishing never waits: to a parked
// receiver if nothing is queued, else into the one-slot buffer if it is free and nothing is
// queued, else onto the overflow list; a receive that takes the buffered result refills the buffer
// from the overflow list. Properties: (G1) at most L children run at once; (G2) the results
// received are a prefix of the publication order, each received once; (G3) every run ends: the
// children all finish and the owner reads its R results (no deadlock), however few R is; (G4) the
// buffer is never empty while results are queued, which is why a publish may hand its result to a
// parked receiver whenever nothing is queued.
//
// Timer: a timer armed at time 0 for D, possibly re-armed (Reset) or stopped at chosen times, with
// the production timer's fires arriving at the deadlines of every arming, including stale ones.
// Properties: (T1) a value is delivered only at or after the current deadline; (T2) at most one
// value per arming; (T3) a stopped timer delivers nothing until a Reset.
//
// Driver: at every site where a lab task parks it holds a set of locks; cancelling a nursery takes
// the scope lock and the sleep lock. Property (D1): whoever cancels at an advance holds none of
// the locks cancelling takes.

import (
	"fmt"
	"slices"
)

// Mutations, for the seeded-bug tests.
var (
	grNoRefill      bool // a receive does not refill the buffer from the overflow list
	grOverwrite     bool // a publish into a full buffer overwrites instead of queueing
	grNoSlotRelease bool // a child never releases its slot
	grRefillLast    bool // a receive refills the buffer from the newest queued result
	tmNoStaleCheck  bool // a timer fire delivers without checking the current deadline
	dlInlineCancel  bool // the advance cancels on the baton holder's stack
)

// --- group protocol ---

type grState struct {
	k, lim, r int
	next      int  // owner: next child to start (k: all started)
	read      int  // owner: results received
	waitSem   bool // owner parked in g.Go at the limit
	waitRecv  bool // owner parked receiving
	running   int  // children started and not yet released
	child     [4]int8
	buf       int8 // buffered result: child id, -1 none
	over      [4]int8
	nover     int8
	pub       [4]int8 // publication order
	npub      int8
	got       [4]int8 // received order
	ngot      int8
}

func (s grState) key() string { return fmt.Sprint(s) }

// grSteps returns every state one step from s, and an error if a property fails in s.
func grSteps(s grState) ([]grState, error) {
	if s.lim > 0 && s.running > s.lim {
		return nil, fmt.Errorf("G1: %d children running, limit %d", s.running, s.lim)
	}
	if s.buf < 0 && s.nover > 0 {
		return nil, fmt.Errorf("G4: the buffer is empty with %d results queued", s.nover)
	}
	var out []grState
	// Owner.
	if !s.waitSem && !s.waitRecv {
		switch {
		case s.next < s.k:
			n := s
			if s.lim > 0 && s.running == s.lim {
				n.waitSem = true
			} else {
				n.running++
				n.child[s.next] = 1
				n.next++
			}
			out = append(out, n)
		case s.read < s.r:
			n := s
			if s.buf >= 0 {
				n.got[n.ngot], n.ngot = s.buf, n.ngot+1
				n.read++
				n.buf = -1
				if n.nover > 0 && !grNoRefill {
					if grRefillLast {
						n.buf = n.over[n.nover-1]
					} else {
						n.buf = n.over[0]
						copy(n.over[:], n.over[1:])
					}
					n.nover--
				}
			} else {
				n.waitRecv = true
			}
			out = append(out, n)
		}
	}
	// Children.
	for i := 0; i < s.k; i++ {
		switch s.child[i] {
		case 1: // publish
			n := s
			id := int8(i)
			n.pub[n.npub], n.npub = id, n.npub+1
			switch {
			case n.nover == 0 && n.waitRecv:
				n.got[n.ngot], n.ngot = id, n.ngot+1
				n.read++
				n.waitRecv = false
			case n.buf < 0 && n.nover == 0:
				n.buf = id
			case grOverwrite && n.buf >= 0:
				n.buf = id
			default:
				n.over[n.nover], n.nover = id, n.nover+1
			}
			n.child[i] = 2
			out = append(out, n)
		case 2: // release the slot
			if grNoSlotRelease {
				n := s
				n.child[i] = 3
				out = append(out, n)
				continue
			}
			n := s
			n.running--
			n.waitSem = false // a parked g.Go retries
			n.child[i] = 3
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		done := s.next == s.k && s.read == s.r
		for i := 0; i < s.k; i++ {
			done = done && s.child[i] == 3
		}
		if !done {
			return nil, fmt.Errorf("G3: no step possible before the end: %+v", s)
		}
		for i := int8(0); i < s.ngot; i++ {
			if s.got[i] != s.pub[i] {
				return nil, fmt.Errorf("G2: received %v, published %v", s.got[:s.ngot], s.pub[:s.npub])
			}
		}
	}
	return out, nil
}

// GroupExplore explores every interleaving of k children, limit lim, and r results read; it
// returns the number of states.
func GroupExplore(k, lim, r int) (int, error) {
	s0 := grState{k: k, lim: lim, r: r, buf: -1}
	seen := map[string]bool{s0.key(): true}
	work := []grState{s0}
	for len(work) > 0 {
		s := work[len(work)-1]
		work = work[:len(work)-1]
		next, err := grSteps(s)
		if err != nil {
			return len(seen), err
		}
		for _, n := range next {
			if k := n.key(); !seen[k] {
				seen[k] = true
				work = append(work, n)
			}
		}
	}
	return len(seen), nil
}

// --- timer re-arming ---

type tmOp struct {
	at   int
	kind byte // 'r': Reset(ms), 's': Stop
	ms   int
}

// TimerRun runs a timer armed at 0 for d with the operations ops at their times; the production
// timer fires at every arming's deadline (stale fires included), a parked receiver takes every
// delivery. It reports a property violation.
func TimerRun(d int, ops []tmOp) error {
	when, fired, armings := d, false, 1
	fires := []int{d}
	delivered := map[int]int{} // arming -> deliveries
	arming := 1
	stopped := false
	for now := 0; now <= 20; now++ {
		for _, op := range ops {
			if op.at != now {
				continue
			}
			switch op.kind {
			case 'r':
				arming, armings = armings+1, armings+1
				when, fired, stopped = now+op.ms, false, false
				fires = append(fires, when)
			case 's':
				fired, stopped = true, true
			}
		}
		for _, f := range fires {
			if f != now {
				continue
			}
			due := now >= when || tmNoStaleCheck
			if due && !fired {
				if now < when {
					return fmt.Errorf("T1: delivered at %d before the deadline %d", now, when)
				}
				if stopped {
					return fmt.Errorf("T3: a stopped timer delivered at %d", now)
				}
				fired = true
				delivered[arming]++
				if delivered[arming] > 1 {
					return fmt.Errorf("T2: arming %d delivered twice", arming)
				}
			}
		}
	}
	return nil
}

// TimerExplore runs every schedule of up to two operations (Reset of 1..4 ms, Stop) at times
// 0..6 against a timer armed for 1..4 ms; it returns the number of runs.
func TimerExplore() (int, error) {
	var opsAll []tmOp
	for at := 0; at <= 6; at++ {
		for ms := 1; ms <= 4; ms++ {
			opsAll = append(opsAll, tmOp{at, 'r', ms})
		}
		opsAll = append(opsAll, tmOp{at, 's', 0})
	}
	runs := 0
	for d := 1; d <= 4; d++ {
		for i := -1; i < len(opsAll); i++ {
			for j := i; j < len(opsAll); j++ {
				var ops []tmOp
				if i >= 0 {
					ops = append(ops, opsAll[i])
				}
				if j >= 0 && j != i {
					ops = append(ops, opsAll[j])
				}
				runs++
				if err := TimerRun(d, ops); err != nil {
					return runs, fmt.Errorf("d=%d ops=%v: %w", d, ops, err)
				}
			}
		}
	}
	return runs, nil
}

// --- deadline driver lock discipline ---

// dlParkSites are the lab's park sites and the locks the parking task holds when it calls
// sgoLabBlock (src/runtime: sgo_chan.go, sgo_lab_run.go, sgo_native_ops.go, sgo_ready.go,
// sgo_scope.go).
var dlParkSites = map[string][]string{
	"channel wait":    {"chan", "sleep"},
	"Sleep":           {"sleep"},
	"select":          {"chan", "sleep"},
	"Start, ready":    {"sleep"},
	"nursery join":    {"scope"},
	"task end":        {},
	"operation point": {},
}

// dlCancelTakes are the locks sgoCancel takes (sgo_cancel.go: the scope lock, then the sleep lock).
var dlCancelTakes = []string{"scope", "sleep"}

// DriverCheck reports, for every park site, whether the task that cancels at an advance holds a
// lock that cancelling takes (D1).
func DriverCheck() error {
	sites := make([]string, 0, len(dlParkSites))
	for s := range dlParkSites {
		sites = append(sites, s)
	}
	slices.Sort(sites)
	for _, site := range sites {
		var held []string
		if dlInlineCancel {
			held = dlParkSites[site] // the baton holder cancels on its own stack
		} // else: the driver cancels, holding nothing
		for _, l := range dlCancelTakes {
			if slices.Contains(held, l) {
				return fmt.Errorf("D1: at %q the canceller holds the %s lock that cancelling takes", site, l)
			}
		}
	}
	return nil
}

// --- deadline inheritance and cancelling error exits ---
//
// The root body opens nested nurseries 1..N (at most cnMax); nursery i has one child that sleeps
// until a scope it is linked to (nursery i or an enclosing one) is cancelled. Any open nursery may
// have a deadline. The body ends by returning normally from the innermost nursery (which must wait
// for its child) or by an error exit that crosses every open nursery (D159): each crossed nursery
// is cancelled, innermost first, then joined. Properties: (C1) a woken child observes the cause of
// the enclosing scope cancelled first (SGO-CANCEL-002), so a deadline inherited from an ancestor
// reports as a deadline (D165, sgo.IsDeadline); (C2) a nursery is cancelled only by its own
// deadline or an error exit crossing it, and a child wakes only if a scope it is linked to is
// cancelled; (C3) every run ends with every nursery closed (an error exit never waits on a
// sleeping child); (C4) a deadline never cancels a closed nursery.

const cnMax = 3

var (
	cnOwnCauseOnly    bool // a child reports only its own nursery's cause (no inheritance)
	cnCancelOnlySelf  bool // a cancellation is not seen by children of nested nurseries
	cnExitNoCancel    bool // an error exit joins the crossed nurseries without cancelling them
	cnFireAfterClose  bool // a nursery's deadline survives its close
	cnCancelParentToo bool // cancelling a nursery also cancels its parent
)

type cnState struct {
	n        int8 // nurseries opened so far
	depth    int8 // innermost open nursery (0: the root body only)
	open     [cnMax + 1]bool
	closed   [cnMax + 1]bool
	deadline [cnMax + 1]bool // a deadline is set and not yet fired
	seq      [cnMax + 1]int8 // cancellation sequence (0: not cancelled)
	cause    [cnMax + 1]int8 // 1 explicit, 2 deadline
	child    [cnMax + 1]int8 // 0 none, 1 sleeping, 2 woke (done)
	seen     [cnMax + 1]int8 // the cause the child observed
	req      [cnMax + 1]bool // a cancellation was requested: its deadline fired or an exit crossed it
	exiting  bool            // an error exit is crossing the open nurseries
	next     int8            // next cancellation sequence number
}

func (s cnState) key() string { return fmt.Sprint(s) }

func (s *cnState) cancel(i int8, cause int8) error {
	if s.closed[i] {
		return fmt.Errorf("C4: nursery %d cancelled after it closed", i)
	}
	s.req[i] = true
	if s.seq[i] == 0 {
		s.next++
		s.seq[i], s.cause[i] = s.next, cause
	}
	if cnCancelParentToo && i > 1 && s.seq[i-1] == 0 {
		s.next++
		s.seq[i-1], s.cause[i-1] = s.next, cause
	}
	return nil
}

// observed is the cause child i sees: the enclosing nursery (1..i) cancelled first, or 0.
func (s *cnState) observed(i int8) int8 {
	if cnOwnCauseOnly {
		if s.seq[i] != 0 {
			return s.cause[i]
		}
		if cnCancelOnlySelf {
			return 0
		}
		for j := i - 1; j >= 1; j-- {
			if s.seq[j] != 0 {
				return 1 // seen, but reported as a plain cancellation
			}
		}
		return 0
	}
	if cnCancelOnlySelf {
		if s.seq[i] != 0 {
			return s.cause[i]
		}
		return 0
	}
	best, cause := int8(0), int8(0)
	for j := int8(1); j <= i; j++ {
		if s.seq[j] != 0 && (best == 0 || s.seq[j] < best) {
			best, cause = s.seq[j], s.cause[j]
		}
	}
	return cause
}

func cnSteps(s cnState) ([]cnState, error) {
	var out []cnState
	try := func(n cnState, err error) error {
		if err != nil {
			return err
		}
		out = append(out, n)
		return nil
	}
	// The body.
	switch {
	case s.exiting && s.depth > 0:
		d := s.depth
		n := s
		if s.child[d] == 1 && !cnExitNoCancel {
			if err := try(n, n.cancel(d, 1)); err != nil { // cancel, then wait for the child
				return nil, err
			}
			out = out[:len(out)-1]
		}
		if n.child[d] == 2 || n.child[d] == 0 { // joined: close
			n.open[d], n.closed[d] = false, true
			if !cnFireAfterClose {
				n.deadline[d] = false
			}
			n.depth--
			out = append(out, n)
		} else if s.seq[d] == 0 && !cnExitNoCancel {
			out = append(out, n) // the cancellation step itself
		}
	case !s.exiting:
		if s.n < cnMax {
			for _, dl := range []bool{false, true} {
				n := s
				n.n++
				d := n.n
				n.depth, n.open[d], n.child[d], n.deadline[d] = d, true, 1, dl
				out = append(out, n)
			}
		}
		if s.depth > 0 {
			n := s
			n.exiting = true // an error exit (D159)
			out = append(out, n)
			if s.child[s.depth] == 2 { // a normal end of the innermost nursery
				m := s
				d := m.depth
				m.open[d], m.closed[d] = false, true
				if !cnFireAfterClose {
					m.deadline[d] = false
				}
				m.depth--
				out = append(out, m)
			}
		}
	}
	// Deadlines.
	for i := int8(1); i <= s.n; i++ {
		if s.deadline[i] {
			n := s
			n.deadline[i] = false
			if err := try(n, n.cancel(i, 2)); err != nil {
				return nil, err
			}
		}
	}
	// Children.
	for i := int8(1); i <= s.n; i++ {
		if s.child[i] != 1 {
			continue
		}
		linked := false
		for j := int8(1); j <= i; j++ {
			linked = linked || s.seq[j] != 0
		}
		c := s.observed(i)
		if c == 0 {
			continue
		}
		if !linked {
			return nil, fmt.Errorf("C2: child %d woke with no linked nursery cancelled", i)
		}
		n := s
		n.child[i], n.seen[i] = 2, c
		out = append(out, n)
	}
	for j := int8(1); j <= s.n; j++ {
		if s.seq[j] != 0 && !s.req[j] {
			return nil, fmt.Errorf("C2: nursery %d cancelled without its deadline or an exit crossing it", j)
		}
	}
	// C1: compare every woken child with the rule.
	for i := int8(1); i <= s.n; i++ {
		if s.child[i] != 2 {
			continue
		}
		best, want := int8(0), int8(0)
		for j := int8(1); j <= i; j++ {
			if s.seq[j] != 0 && (best == 0 || s.seq[j] < best) {
				best, want = s.seq[j], s.cause[j]
			}
		}
		if s.seen[i] != want && want != 0 {
			// the child observed at wake time: a later, earlier-sequenced cancellation is impossible,
			// so a mismatch is a wrong cause
			return nil, fmt.Errorf("C1: child %d observed cause %d, the first cancelled enclosing nursery has %d", i, s.seen[i], want)
		}
	}
	if len(out) == 0 {
		for i := int8(1); i <= s.n; i++ {
			if s.open[i] {
				return nil, fmt.Errorf("C3: no step possible with nursery %d open: %+v", i, s)
			}
		}
	}
	return out, nil
}

// CancelTreeExplore explores every run of the nested-nursery scenario; it returns the number of
// states.
func CancelTreeExplore() (int, error) {
	s0 := cnState{}
	seen := map[string]bool{s0.key(): true}
	work := []cnState{s0}
	for len(work) > 0 {
		s := work[len(work)-1]
		work = work[:len(work)-1]
		next, err := cnSteps(s)
		if err != nil {
			return len(seen), err
		}
		for _, n := range next {
			if k := n.key(); !seen[k] {
				seen[k] = true
				work = append(work, n)
			}
		}
	}
	return len(seen), nil
}
