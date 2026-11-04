// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

// Structured resources (W21, D148–D152): every resource is acquired into a scope (a nursery or a
// task body) and released when that scope ends, explored over every interleaving before the
// runtime implements it.
//
// Scopes of the scenario: 0 is the root task's body, 1 a nursery opened by the root, and 2..K+1
// the bodies of the K children spawned into it. The root may hold a resource A in its body and
// holds B (and optionally a user resource U whose Release uses B) in the nursery. Each child may
// acquire C into its own body, use A, B, and C, close B early, and move C outward (MoveTo), the
// Happy Eyeballs hand-up. The root may cancel the nursery. One release may be set to fail.
//
// Rules modeled: a task may use a resource only while it runs inside the resource's scope; a
// nursery joins its children, then releases its resources in reverse order of acquisition, then
// computes its outcome; a task body releases its resources before its outcome exists; MoveTo
// moves only to an enclosing scope; an explicit Close removes the resource from its scope.
//
// Properties checked: (1) no use after a scope released the resource; (2) every resource is
// released exactly once; (3) MoveTo never lets a resource outlive a task that can use it; (4)
// every failed release error reaches exactly one place (a caller, a task or nursery outcome, or
// a chain); (5) a user resource's Release runs before the resources it uses are released, and sees
// ok == the scope is ending normally.

import "fmt"

// Mutations of the protocol, for the seeded-bug tests.
var (
	srReleaseBeforeJoin bool // a nursery releases its resources before joining its children
	srReleaseFIFO       bool // a scope releases in acquisition order
	srCloseKeepsInList  bool // an explicit Close leaves the resource in its scope's list
	srMoveNoUnlink      bool // MoveTo leaves the resource in its old scope's list too
	srMoveAnyScope      bool // MoveTo accepts a scope that does not enclose the resource's
	srDropChain         bool // release errors after the primary outcome are dropped
	srTaskEndLeaks      bool // a task body's end does not release its resources
	srEscapeUnchecked   bool // the static escape rule (D151) is not enforced
	srReleaseOKTrue     bool // Release always sees ok == true
)

const (
	srOpen     = iota
	srClosed   // closed explicitly by a task
	srReleased // released by its scope
)

type srRes struct {
	Scope, State, Closes, Sinks int
	Fail, User, Failed          bool
	Holds                       int // a user resource: the resource its Release uses, -1 none
	OK                          int // a user resource: the ok its Release saw (-1 not run)
}

type srScope struct {
	Parent int
	List   []int // open resources, in acquisition (or move) order
	Ended  bool
}

type srState struct {
	Res     []srRes
	Scopes  []srScope
	PC      []int // per task: next step (task 0 is the root)
	Out     []int // per child: 0 running, 1 succeeded, 2 errored, 3 cancelled
	C       []int // per child: its resource, -1 none
	A, B, U int   // the root's resources, -1 none
	Cancel  bool  // the nursery was cancelled
	RootErr bool  // the root is unwinding from the nursery
}

func (s *srState) clone() *srState {
	c := *s
	c.Res = append([]srRes(nil), s.Res...)
	c.Scopes = nil
	for _, x := range s.Scopes {
		x.List = append([]int(nil), x.List...)
		c.Scopes = append(c.Scopes, x)
	}
	c.PC = append([]int(nil), s.PC...)
	c.Out = append([]int(nil), s.Out...)
	c.C = append([]int(nil), s.C...)
	return &c
}

func (s *srState) key() string { return fmt.Sprint(*s) }

// taskScope is the innermost scope task t runs in.
func (s *srState) taskScope(t int) int {
	if t > 0 {
		return t + 1
	}
	if s.PC[0] >= 1 && !s.Scopes[1].Ended {
		return 1
	}
	return 0
}

// encloses reports whether scope a is s or an ancestor of it.
func (s *srState) encloses(a, x int) bool {
	for ; x >= 0; x = s.Scopes[x].Parent {
		if x == a {
			return true
		}
	}
	return false
}

func (s *srState) acquire(scope int, fail, user bool, holds int) int {
	s.Res = append(s.Res, srRes{Scope: scope, Fail: fail, User: user, Holds: holds, OK: -1})
	r := len(s.Res) - 1
	s.Scopes[scope].List = append(s.Scopes[scope].List, r)
	return r
}

// use: a violation if r was released by its scope or t is outside it; failed if r was closed.
func (s *srState) use(t, r int) (failed bool, err error) {
	x := &s.Res[r]
	if !s.encloses(x.Scope, s.taskScope(t)) {
		return false, fmt.Errorf("task %d used resource %d outside its scope %d", t, r, x.Scope)
	}
	if x.State == srReleased {
		return false, fmt.Errorf("task %d used resource %d after its scope released it", t, r)
	}
	return x.State == srClosed, nil
}

func (s *srState) unlist(r int) {
	sc := &s.Scopes[s.Res[r].Scope]
	for i, y := range sc.List {
		if y == r {
			sc.List = append(sc.List[:i:i], sc.List[i+1:]...)
			return
		}
	}
}

// close is an explicit Close by task t; its error goes to the caller.
func (s *srState) close(t, r int) error {
	if _, err := s.use(t, r); err != nil {
		return err
	}
	x := &s.Res[r]
	if x.State != srOpen {
		return nil // HandleClosed, returned to the caller
	}
	x.State, x.Closes = srClosed, x.Closes+1
	if x.Fail {
		x.Failed = true
		x.Sinks++
	}
	if !srCloseKeepsInList {
		s.unlist(r)
	}
	return nil
}

// moveTo moves r to scope to, for task t; refused (no effect) unless to encloses r's scope.
func (s *srState) moveTo(t, r, to int) error {
	if _, err := s.use(t, r); err != nil {
		return err
	}
	if !s.encloses(to, s.Res[r].Scope) && !srMoveAnyScope {
		return nil
	}
	if !srMoveNoUnlink {
		s.unlist(r)
	}
	s.Res[r].Scope = to
	s.Scopes[to].List = append(s.Scopes[to].List, r)
	return nil
}

// release ends scope sc: its resources in reverse order, user resources seeing ok. It returns the
// resources whose release failed, in release order.
func (s *srState) release(sc int, ok bool) (failed []int, err error) {
	l := s.Scopes[sc].List
	s.Scopes[sc].List, s.Scopes[sc].Ended = nil, true
	for k := range l {
		r := l[len(l)-1-k]
		if srReleaseFIFO {
			r = l[k]
		}
		x := &s.Res[r]
		fails := x.Fail
		if x.User {
			if h := x.Holds; h >= 0 {
				if s.Res[h].State == srReleased {
					return nil, fmt.Errorf("Release of user resource %d ran after resource %d it uses was released", r, h)
				}
				fails = fails || s.Res[h].State == srClosed // HandleClosed inside Release
			}
			x.OK = 0
			if ok || srReleaseOKTrue {
				x.OK = 1
			}
			if (x.OK == 1) != ok {
				return nil, fmt.Errorf("Release of user resource %d saw ok=%v, scope ending normally=%v", r, x.OK == 1, ok)
			}
		}
		x.State, x.Closes = srReleased, x.Closes+1
		if fails {
			x.Failed = true
			failed = append(failed, r)
		}
	}
	return failed, nil
}

// route records failed releases: the first becomes the outcome unless one already exists, and the
// rest join the chain. It reports whether an outcome was created.
func (s *srState) route(failed []int, hasPrimary bool) bool {
	for i, r := range failed {
		if i == 0 && !hasPrimary {
			s.Res[r].Sinks++
			continue
		}
		if !srDropChain {
			s.Res[r].Sinks++
		}
	}
	return !hasPrimary && len(failed) > 0
}
