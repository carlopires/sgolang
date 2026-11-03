// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

// The connection hand-up of D144 (W18): sgo.ConnSlot, explored over every interleaving before the
// runtime implements it.
//
// The root task (task 0) owns a slot and a nursery of N child tasks. Each child dials (it then owns
// a fresh connection), puts it into the slot, and, if it lost the race (AlreadySet), closes the
// connection or leaves it to be closed when the child ends. After the nursery joins, the root
// takes the slot's connection, uses it, and closes it; it may also try to take while the children
// run. At the root's end the slot is closed, closing a connection still in it.
//
// The runtime rules being modeled:
//   - a connection is in exactly one place: its owner's handle list, or the slot;
//   - only a task touches its own handle list (Put unlinks from the putter's, Take links into the
//     taker's);
//   - Put succeeds only into an empty slot, and only for the connection's owner;
//   - a task's end closes the connections in its list; the slot's close closes its content.
//
// Invariants checked on every reachable state: no task touches another task's handle list, no
// connection is used after it is closed or by a task that does not own it, and at the end of the
// program every connection is closed exactly once.

import (
	"fmt"
	"slices"
	"strings"
)

// Mutations of the runtime protocol, for the seeded-bug tests.
var (
	handoffPutOverwrites bool // Put into a full slot replaces the content (no AlreadySet)
	handoffPutKeepsLink  bool // Put leaves the connection in the putter's handle list
	handoffSlotLeaks     bool // closing the slot does not close its content
)

const (
	hoInList = iota // in its owner's handle list
	hoInSlot
	hoClosed
)

type hoConn struct {
	Owner, Where, Closes int
	InList               []int // tasks whose handle list holds it
}

type hoState struct {
	Conns []hoConn
	PC    []int // per task: next step
	Mine  []int // per child: its connection (-1 none)
	Lost  []bool
	Slot  int // connection in the slot, -1 empty
	Took  int // connection the root took, -1 none
}

// HandoffProgram is one variant of the scenario: N children; whether losers close their connection
// explicitly; whether the root tries an early Take while the children run.
type HandoffProgram struct {
	N           int
	LoserCloses bool
	EarlyTake   bool
	NoTake      bool // the root never takes (an error path): the slot's close must close the winner
}

func (p HandoffProgram) childSteps() int { return 3 } // Dial, Put, Close-if-lost; then end
func (p HandoffProgram) rootSteps() int  { return 4 } // [early Take], join, Take+use+close, end

func (s *hoState) clone() *hoState {
	c := &hoState{PC: slices.Clone(s.PC), Mine: slices.Clone(s.Mine), Lost: slices.Clone(s.Lost), Slot: s.Slot, Took: s.Took}
	for _, x := range s.Conns {
		x.InList = slices.Clone(x.InList)
		c.Conns = append(c.Conns, x)
	}
	return c
}

func (s *hoState) key() string {
	var b strings.Builder
	fmt.Fprint(&b, s.Conns, s.PC, s.Mine, s.Lost, s.Slot, s.Took)
	return b.String()
}

// touchList checks that task t touches only its own handle list.
func (s *hoState) touchList(t, owner int) error {
	if t != owner {
		return fmt.Errorf("task %d touched the handle list of task %d", t, owner)
	}
	return nil
}

func (s *hoState) unlink(c, t int) {
	s.Conns[c].InList = slices.DeleteFunc(s.Conns[c].InList, func(x int) bool { return x == t })
}

func (s *hoState) close(c, t int) error {
	x := &s.Conns[c]
	if x.Where == hoClosed {
		return fmt.Errorf("task %d closed connection %d twice", t, c)
	}
	x.Where = hoClosed
	x.Closes++
	return nil
}

// endTask closes the connections in task t's handle list (HandleAbandoned).
func (s *hoState) endTask(t int) error {
	for c := range s.Conns {
		if slices.Contains(s.Conns[c].InList, t) {
			s.unlink(c, t)
			if err := s.close(c, t); err != nil {
				return err
			}
		}
	}
	return nil
}

// step runs task t's next step; blocked reports a join that must wait.
func (s *hoState) step(p HandoffProgram, t int) (blocked bool, err error) {
	if t > 0 {
		i := t - 1
		switch s.PC[t] {
		case 0: // Dial: a fresh connection in the child's list
			s.Conns = append(s.Conns, hoConn{Owner: t, Where: hoInList, InList: []int{t}})
			s.Mine[i] = len(s.Conns) - 1
		case 1: // Put
			c := s.Mine[i]
			if s.Conns[c].Owner != t || s.Conns[c].Where != hoInList {
				return false, fmt.Errorf("task %d put a connection it does not hold", t)
			}
			if s.Slot >= 0 && !handoffPutOverwrites {
				s.Lost[i] = true // AlreadySet: the connection stays the child's
				break
			}
			if err := s.touchList(t, s.Conns[c].Owner); err != nil {
				return false, err
			}
			if !handoffPutKeepsLink {
				s.unlink(c, t)
			}
			s.Conns[c].Where = hoInSlot
			s.Conns[c].Owner = 0 // the slot's owner
			s.Slot = c
		case 2: // a loser closes its connection, or leaves it to its end
			if s.Lost[i] && p.LoserCloses {
				c := s.Mine[i]
				s.unlink(c, t)
				if err := s.close(c, t); err != nil {
					return false, err
				}
			}
		case 3: // end
			if err := s.endTask(t); err != nil {
				return false, err
			}
		}
		s.PC[t]++
		return false, nil
	}
	switch s.PC[0] {
	case 0: // early Take (optional): takes what is there, if anything
		if p.EarlyTake && !p.NoTake && s.Slot >= 0 {
			if err := s.take(); err != nil {
				return false, err
			}
		}
	case 1: // join: wait for every child to end
		for k := 1; k <= p.N; k++ {
			if s.PC[k] <= p.childSteps() {
				return true, nil
			}
		}
	case 2: // Take (if not taken yet), use, close
		if s.Took < 0 && s.Slot >= 0 && !p.NoTake {
			if err := s.take(); err != nil {
				return false, err
			}
		}
		if s.Took >= 0 {
			c := s.Took
			if x := s.Conns[c]; x.Where != hoInList || x.Owner != 0 {
				return false, fmt.Errorf("the root used connection %d after taking it, but it is no longer its own (where %d, owner %d)", c, x.Where, x.Owner)
			}
			s.unlink(c, 0)
			if err := s.close(c, 0); err != nil {
				return false, err
			}
		}
	case 3: // end: the root's handles, then the slot and its content
		if err := s.endTask(0); err != nil {
			return false, err
		}
		if s.Slot >= 0 && !handoffSlotLeaks {
			if err := s.close(s.Slot, 0); err != nil {
				return false, err
			}
			s.Slot = -1
		}
	}
	s.PC[0]++
	return false, nil
}

func (s *hoState) take() error {
	c := s.Slot
	s.Slot = -1
	s.Conns[c].Where = hoInList
	s.Conns[c].InList = append(s.Conns[c].InList, 0) // linked by the root into its own list
	s.Took = c
	return nil
}

func (s *hoState) done(p HandoffProgram) bool {
	if s.PC[0] <= p.rootSteps()-1 {
		return false
	}
	for k := 1; k <= p.N; k++ {
		if s.PC[k] <= p.childSteps() {
			return false
		}
	}
	return true
}

// finalCheck: every connection closed exactly once, nothing left in a list or the slot.
func (s *hoState) finalCheck() error {
	for c, x := range s.Conns {
		if x.Closes != 1 {
			return fmt.Errorf("connection %d closed %d times at the end of the program", c, x.Closes)
		}
		if len(x.InList) != 0 {
			return fmt.Errorf("connection %d is closed but still in the handle list of tasks %v", c, x.InList)
		}
	}
	return nil
}

// HandoffResult is the outcome of exploring one program.
type HandoffResult struct {
	States, Ends int
	Winners      map[int]bool // child indices that won in some interleaving
	Err          error
}

// ExploreHandoff runs p over every interleaving of its N+1 tasks.
func ExploreHandoff(p HandoffProgram) HandoffResult {
	s0 := &hoState{PC: make([]int, p.N+1), Mine: make([]int, p.N), Lost: make([]bool, p.N), Slot: -1, Took: -1}
	for i := range s0.Mine {
		s0.Mine[i] = -1
	}
	res := HandoffResult{Winners: map[int]bool{}}
	seen := map[string]bool{}
	var dfs func(s *hoState, sched []int) bool
	dfs = func(s *hoState, sched []int) bool {
		if k := s.key(); seen[k] {
			return true
		} else {
			seen[k] = true
		}
		res.States++
		if s.done(p) {
			res.Ends++
			if err := s.finalCheck(); err != nil {
				res.Err = fmt.Errorf("%v (schedule %v)", err, sched)
				return false
			}
			for i, c := range s.Mine {
				if c == s.Took && c >= 0 {
					res.Winners[i] = true
				}
			}
			return true
		}
		for t := 0; t <= p.N; t++ {
			last := p.childSteps()
			if t == 0 {
				last = p.rootSteps() - 1
			}
			if s.PC[t] > last {
				continue
			}
			n := s.clone()
			blocked, err := n.step(p, t)
			if blocked {
				continue
			}
			sc := append(slices.Clone(sched), t)
			if err != nil {
				res.Err = fmt.Errorf("%v (schedule %v)", err, sc)
				return false
			}
			if !dfs(n, sc) {
				return false
			}
		}
		return true
	}
	dfs(s0, nil)
	return res
}
