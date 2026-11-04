// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import "fmt"

// Pools with leases (W23 step 4, D175): plMax tasks share plItems items of one pool. A task
// acquires a lease (waiting while no item is idle; it may be cancelled while it waits), holds the
// item, and either returns it early (Return), drops it (Drop: it never comes back), or keeps it;
// its lease scope then ends and releases the lease, which puts the item back unless it was dropped
// or already returned. A returned item goes to the oldest waiting task, if any, in the same step
// (a native channel hand-over: a waiter either gets the item or is cancelled, never both).
// Properties: (P1) an item is held by at most one lease; (P2) every item is idle, held, or dropped:
// none is lost or duplicated; (P3) a lease puts its item back at most once; (P4) no task waits
// while an item is idle.

const (
	plMax   = 3
	plItems = 2
)

var (
	plReturnTwice bool // the scope end puts back an item that Return already put back
	plDropReturns bool // a dropped item is put back at the scope end
	plCancelLoses bool // a waiter cancelled as an item is handed to it takes the item with it
	plNoWake      bool // an item put back while tasks wait goes idle instead of to a waiter
)

// Task phases.
const (
	plStart   = iota // not yet acquiring
	plWaiting        // waiting for an item
	plHolding        // holds a lease on item
	plKept           // returned or dropped early; the lease scope has not ended
	plDone           // the lease scope ended (or the task was cancelled while waiting)
)

type plState struct {
	phase   [plMax]int8
	item    [plMax]int8 // the item a lease holds (+1)
	dropped [plMax]bool // the lease dropped its item
	back    [plMax]int8 // how many times the lease put its item back
	order   [plMax]int8 // waiting order (+1; 0: not waiting)
	seq     int8
	idle    [plItems]bool
	gone    [plItems]bool // dropped from the pool
}

func (s plState) key() string { return fmt.Sprint(s) }

// putBack returns item it to the pool: to the oldest waiter, or idle.
func (s *plState) putBack(it int) {
	w := -1
	for i := 0; i < plMax; i++ {
		if s.phase[i] == plWaiting && (w < 0 || s.order[i] < s.order[w]) {
			w = i
		}
	}
	if w >= 0 && !plNoWake {
		s.phase[w], s.item[w], s.order[w] = plHolding, int8(it)+1, 0
		return
	}
	s.idle[it] = true
}

func plSteps(s plState) []plState {
	var out []plState
	for i := 0; i < plMax; i++ {
		switch s.phase[i] {
		case plStart: // Acquire: an idle item, or wait
			n := s
			taken := false
			for it := 0; it < plItems; it++ {
				if n.idle[it] {
					n.idle[it] = false
					n.phase[i], n.item[i] = plHolding, int8(it)+1
					taken = true
					break
				}
			}
			if !taken {
				n.seq++
				n.phase[i], n.order[i] = plWaiting, n.seq
			}
			out = append(out, n)
		case plWaiting: // cancelled while waiting
			n := s
			n.phase[i], n.order[i] = plDone, 0
			out = append(out, n)
		case plHolding:
			it := int(s.item[i]) - 1
			if plCancelLoses && s.back[i] == 0 { // the hand-over and a cancellation race
				n := s
				n.phase[i] = plDone
				out = append(out, n)
			}
			n := s // Return: the item goes back now
			n.phase[i] = plKept
			n.back[i]++
			n.putBack(it)
			out = append(out, n)
			n = s // Drop: the item leaves the pool
			n.phase[i], n.dropped[i] = plKept, true
			n.gone[it] = true
			out = append(out, n)
			n = s // the lease scope ends: the lease is released
			n.phase[i] = plDone
			n.back[i]++
			n.putBack(it)
			out = append(out, n)
		case plKept: // the lease scope ends after Return or Drop
			it := int(s.item[i]) - 1
			n := s
			n.phase[i] = plDone
			if plReturnTwice && !s.dropped[i] || plDropReturns && s.dropped[i] {
				n.back[i]++
				n.putBack(it)
			}
			out = append(out, n)
		}
	}
	return out
}

func plCheck(s plState) error {
	var held [plItems]int
	for i := 0; i < plMax; i++ {
		if s.phase[i] == plHolding {
			held[s.item[i]-1]++
		}
		if s.back[i] > 1 {
			return fmt.Errorf("P3: lease %d put its item back %d times", i, s.back[i])
		}
	}
	idle := false
	for it := 0; it < plItems; it++ {
		if held[it] > 1 {
			return fmt.Errorf("P1: item %d held by %d leases", it, held[it])
		}
		n := held[it]
		if s.idle[it] {
			n++
			idle = true
		}
		if s.gone[it] {
			n++
		}
		if n != 1 {
			return fmt.Errorf("P2: item %d is in %d places (held %d, idle %v, dropped %v): %+v", it, n, held[it], s.idle[it], s.gone[it], s)
		}
	}
	for i := 0; i < plMax; i++ {
		if idle && s.phase[i] == plWaiting {
			return fmt.Errorf("P4: task %d waits while an item is idle: %+v", i, s)
		}
	}
	return nil
}

// PoolExplore explores every run of the pool scenario; it returns the number of states.
func PoolExplore() (int, error) {
	s0 := plState{}
	for it := range s0.idle {
		s0.idle[it] = true
	}
	seen := map[string]bool{s0.key(): true}
	work := []plState{s0}
	for len(work) > 0 {
		s := work[len(work)-1]
		work = work[:len(work)-1]
		if err := plCheck(s); err != nil {
			return len(seen), err
		}
		for _, n := range plSteps(s) {
			if k := n.key(); !seen[k] {
				seen[k] = true
				work = append(work, n)
			}
		}
	}
	return len(seen), nil
}
