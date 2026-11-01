// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

// The I/O handle contract (W09, D54–D56): the reference for the runtime's adapters and the lab
// fakes (I03, I04, I05).
//
// Task 0 owns every handle (it opens them). A handle may be lent to the other tasks (the children
// of one nursery); the lend ends at Unlend (the nursery closes). Only the owner closes; Close
// while lent is HandleLent with no effect. Any operation on a closed handle is HandleClosed with
// no effect, and a reopened handle is a new generation: an operation issued against the old one
// never reaches it. Read(max) transfers at least 1 and at most max bytes, or fails with progress
// 0; Write(n) transfers all n bytes, or fails with the progress so far. A socket operation that
// must wait is a cancellation point: Cancel ends it with the progress so far. Cancellation never
// closes a handle. Transfers may be short: each step moves 1 byte or everything available.

import "fmt"

// HKind is an operation of a handle scenario.
type HKind int

const (
	HOpen    HKind = iota // the owner opens (or reopens) handle H: a new generation
	HRead                 // read at most N bytes from H
	HWrite                // write N bytes to H
	HClose                // close H
	HLend                 // the owner lends H to the other tasks
	HUnlend               // the lend ends (the nursery closed)
	HCancel               // cancel task N (its current or next waiting operation ends)
	HDeliver              // peer: N bytes arrive at H's read side
	HDrain                // peer: N bytes of buffer space free up at H's write side
)

// HOp is one operation; H is the handle index.
type HOp struct {
	Kind HKind
	H, N int
}

// HScenario is a handle scenario: Tasks[0] is the owner, the last script may be the peer's
// (Peer set). Cap is the initial write space of each handle.
type HScenario struct {
	Tasks [][]HOp
	Peer  bool
	Cap   int
}

type hHandle struct {
	gen       int // current generation (0: never opened)
	open      bool
	lent      bool
	readable  int // bytes available to read
	space     int // bytes of write space
	delivered int // bytes the peer delivered in total
	consumed  int // bytes read in total
	written   int // bytes written in total
}

type hTask struct {
	pc        int
	cancelled bool
	// The operation in progress (a read or write that has transferred part of its request).
	active   bool
	gen      int // the generation the operation was issued against
	progress int
	results  []string
}

type hState struct {
	h     []hHandle
	tasks []hTask
}

func (s *hState) clone() *hState {
	c := &hState{h: append([]hHandle(nil), s.h...), tasks: make([]hTask, len(s.tasks))}
	for i, t := range s.tasks {
		t.results = append([]string(nil), t.results...)
		c.tasks[i] = t
	}
	return c
}

func (s *hState) key() string { return fmt.Sprint(s.h, s.tasks) }

// check verifies the contract's invariants on a state.
func (s *hState) check() error {
	for i, h := range s.h {
		if h.consumed > h.delivered {
			return fmt.Errorf("handle %d: read %d bytes but only %d were delivered", i, h.consumed, h.delivered)
		}
		if h.readable != h.delivered-h.consumed {
			return fmt.Errorf("handle %d: %d readable, %d delivered, %d read", i, h.readable, h.delivered, h.consumed)
		}
		if h.lent && !h.open {
			return fmt.Errorf("handle %d: lent while closed", i)
		}
	}
	return nil
}
