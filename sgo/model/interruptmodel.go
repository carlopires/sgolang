// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

// D179 interrupt protocol, explored over every interleaving: SIGINT or SIGTERM cancels the root
// (src/runtime/sgo_interrupt.go).
//
// The signal handler counts deliveries. The first wakes the interrupt watcher through a note; the
// second ends the process at once from the handler (status 130 or 143), so two signals that arrive
// before the watcher runs are never merged into one. The watcher publishes the cancellation of the
// virtual root scope, which is the outermost ancestor of every scope, then scans the sleeping tasks
// under the sleep lock and wakes those whose cancellation is now in effect. A task checks for
// cancellation and parks in one step under the sleep lock.
//
// The program: the root acquires a resource, checks for cancellation, opens a nursery and spawns
// two children, each of which acquires a resource and then either finishes its work or parks in a
// wait that only cancellation ends (an idle read of stdin); the nursery joins, and the root
// releases its resource and returns. Up to two signals arrive at any point while the process
// lives.
//
// Properties: (I1) after an interrupt no run ends with a task still parked: every wait is woken;
// (I2) a run that ends by its own unwind releases every resource it acquired; (I3) a second signal
// ends the process with status 130, whatever the watcher and the tasks are doing; (I4) a run that
// ends by unwinding after an interrupt reports the cancellation (status 1), and one with no
// interrupt in effect before the root's checks returns 0; (I5) the root starts no new work (the
// nursery) once the interrupt is in effect for it. The exploration runs twice: with at most one
// signal for I1, I2, I4 and I5 (a second signal would end every hang and hide it), and with two for
// I3.

import "fmt"

// Mutations, for the seeded-bug tests.
var (
	inSplitPark   bool // a task checks for cancellation and parks in two steps, outside the lock
	inCoalesce    bool // the second signal is queued to the watcher instead of exiting in the handler
	inNewNursery  bool // a nursery opened after the interrupt does not inherit it
	inNoRootCheck bool // the root's checks do not see the virtual root scope
)

const (
	inChNone    = iota // not spawned
	inChHeld           // resource acquired
	inChChecked        // (inSplitPark) saw no cancellation, about to park
	inChParked         // parked until cancellation
	inChDone           // finished, resource released
)

type inState struct {
	signals   int8 // signals delivered
	noted     bool // the watcher's note is set
	queued    int8 // (inCoalesce) signals queued to the watcher
	published bool // the virtual root scope is cancelled
	scanned   bool // the watcher's wake scan ran after publishing
	rootPC    int8 // 0 start, 1 nursery open, 2 nursery closed, 3 returned
	rootHeld  bool
	rootSaw   bool // the root observed cancellation
	nurseryIn bool // the nursery inherits the interrupt (opened before or after it)
	child     [2]int8
	childSaw  [2]bool
	lateWork  bool // the root opened the nursery after the interrupt was published
	exited    bool
	status    int16
}

func (s inState) key() string { return fmt.Sprint(s) }

// cancelled reports whether the cancellation is in effect for a task of the nursery (root: false
// means the root's own checks).
func (s *inState) cancelled(inNursery bool) bool {
	if !s.published {
		return false
	}
	if inNursery {
		return s.nurseryIn
	}
	return !inNoRootCheck
}

func inSteps(s inState, maxSignals int8) []inState {
	if s.exited {
		return nil
	}
	var out []inState
	add := func(n inState) { out = append(out, n) }

	// A signal arrives.
	if s.signals < maxSignals {
		n := s
		n.signals++
		if n.signals == 1 {
			n.noted = true
		} else if inCoalesce {
			n.queued++
		} else {
			n.exited, n.status = true, 130
		}
		add(n)
	}

	// The watcher: publish on the note, then scan the sleepers; with inCoalesce, a queued second
	// signal it receives ends the process.
	if s.noted && !s.published {
		n := s
		n.published = true
		if !inNewNursery {
			n.nurseryIn = true
		}
		add(n)
	}
	if s.published && !s.scanned {
		n := s
		n.scanned = true
		for i := range n.child {
			if n.child[i] == inChParked && n.cancelled(true) {
				n.child[i], n.childSaw[i] = inChDone, true
			}
		}
		add(n)
	}
	// (inCoalesce) A second signal queued while the note is still set merges into the first
	// wake-up (sigsend keeps one pending bit per signal), so the watcher never sees it.

	// The root.
	switch s.rootPC {
	case 0:
		n := s
		n.rootHeld = true
		if n.cancelled(false) {
			n.rootSaw = true
			n.rootPC = 2 // skips the nursery
		} else {
			n.rootPC = 1
			n.lateWork = n.published
			if n.published && !inNewNursery {
				n.nurseryIn = true
			}
			n.child = [2]int8{inChHeld, inChHeld}
		}
		add(n)
	case 1:
		if s.child[0] == inChDone && s.child[1] == inChDone {
			n := s
			n.rootPC = 2
			if n.cancelled(true) {
				n.rootSaw = true // the nursery returns the cancellation to its owner
			}
			add(n)
		}
	case 2:
		n := s
		n.rootHeld = false
		n.rootPC = 3
		n.exited = true
		if n.rootSaw {
			n.status = 1
		}
		add(n)
	}

	// The children.
	for i := range s.child {
		switch s.child[i] {
		case inChHeld:
			n := s // finishes its work
			n.child[i] = inChDone
			add(n)
			n = s // waits
			if inSplitPark {
				if !n.cancelled(true) {
					n.child[i] = inChChecked
				} else {
					n.child[i], n.childSaw[i] = inChDone, true
				}
			} else if n.cancelled(true) {
				n.child[i], n.childSaw[i] = inChDone, true
			} else {
				n.child[i] = inChParked
			}
			add(n)
		case inChChecked:
			n := s
			n.child[i] = inChParked
			add(n)
		}
	}
	return out
}

func inCheck(s inState, final bool) error {
	if !final {
		return nil
	}
	if s.signals >= 2 && !(s.exited && s.status == 130) {
		return fmt.Errorf("I3 second signal did not end the process: %+v", s)
	}
	if s.exited && s.status == 130 {
		return nil
	}
	if !s.exited {
		if s.signals >= 1 {
			return fmt.Errorf("I1 a task stays parked after the interrupt: %+v", s)
		}
		return nil // a program that waits forever with no interrupt is the program's own hang
	}
	if s.rootHeld {
		return fmt.Errorf("I2 the root's resource is not released: %+v", s)
	}
	for i, c := range s.child {
		if c != inChNone && c != inChDone {
			return fmt.Errorf("I2 child %d's resource is not released: %+v", i, s)
		}
	}
	if s.lateWork {
		return fmt.Errorf("I5 the root opened the nursery after the interrupt: %+v", s)
	}
	if s.rootSaw != (s.status == 1) {
		return fmt.Errorf("I4 status %d but the root saw cancellation %v: %+v", s.status, s.rootSaw, s)
	}
	return nil
}

// InterruptExplore explores every interleaving and returns the number of states.
func InterruptExplore() (int, error) {
	total := 0
	for _, m := range []int8{1, 2} {
		n, err := inExplore(m)
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func inExplore(maxSignals int8) (int, error) {
	s0 := inState{}
	seen := map[string]bool{s0.key(): true}
	work := []inState{s0}
	for len(work) > 0 {
		s := work[len(work)-1]
		work = work[:len(work)-1]
		next := inSteps(s, maxSignals)
		if err := inCheck(s, len(next) == 0); err != nil {
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
