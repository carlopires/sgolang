// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import "fmt"

// ScopeProgram is one variant of the structured-resources scenario (scoperes.go).
type ScopeProgram struct {
	K      int  // children spawned into the nursery (1 or 2)
	RootA  bool // the root acquires A into its own body
	UserU  bool // the root owns a user resource U (Release uses B) after B
	ChildC bool // every child acquires C into its own body
	Move   int  // 0 none, 1 child 1 moves its C to the nursery, 2 child 1 moves it to child 2's body
	CloseB bool // child 1 closes B early
	Cancel bool // the root cancels the nursery after spawning
	Fail   byte // the resource whose release fails: 0 none, 'A', 'B', 'C' (child 1's), 'U'
	Escape bool // the root uses B after the nursery ended (refused by D151 unless srEscapeUnchecked)
}

// Admitted reports whether the static rules admit the program (D151: no use beyond the scope's name).
func (p ScopeProgram) Admitted() bool { return !p.Escape || srEscapeUnchecked }

const (
	srChildEnd = 4 // child steps: 0 acquire, 1 use A and B, 2 close/move, 3 use C, 4 end
	srRootEnd  = 7 // root steps: 0 A, 1 open nursery (B, U), 2 spawn, 3 cancel, 4 use, 5 join+release, 6 after, 7 end
)

func (s *srState) child(p ScopeProgram, t int) (blocked bool, err error) {
	i := t - 1
	pc := s.PC[t]
	if s.Cancel && pc < srChildEnd && s.Out[i] == 0 && pc != 2 { // cancellation points: acquire and uses
		s.Out[i], s.PC[t] = 3, srChildEnd
		return false, nil
	}
	fail := func(f bool) {
		if f && s.Out[i] == 0 {
			s.Out[i], s.PC[t] = 2, srChildEnd // HandleClosed: the child returns an error
		}
	}
	switch pc {
	case 0:
		if p.ChildC {
			s.C[i] = s.acquire(t+1, p.Fail == 'C' && t == 1, false, -1)
		}
	case 1:
		for _, r := range []int{s.A, s.B} {
			if r >= 0 {
				f, e := s.use(t, r)
				if e != nil {
					return false, e
				}
				fail(f)
				if s.PC[t] == srChildEnd {
					return false, nil
				}
			}
		}
	case 2:
		if t == 1 && p.CloseB {
			if e := s.close(t, s.B); e != nil {
				return false, e
			}
		}
		if t == 1 && p.Move != 0 && s.C[0] >= 0 {
			to := 1
			if p.Move == 2 {
				to = 3 // child 2's body: does not enclose child 1's
			}
			if e := s.moveTo(t, s.C[0], to); e != nil {
				return false, e
			}
		}
	case 3:
		if c := s.C[i]; c >= 0 {
			f, e := s.use(t, c)
			if e != nil {
				return false, e
			}
			fail(f)
			if s.PC[t] == srChildEnd {
				return false, nil
			}
		}
	case srChildEnd:
		if !srTaskEndLeaks {
			failed, e := s.release(t+1, s.Out[i] == 0)
			if e != nil {
				return false, e
			}
			if s.route(failed, s.Out[i] != 0) {
				s.Out[i] = 2 // a release error is the child's error
			}
		}
		if s.Out[i] == 0 {
			s.Out[i] = 1
		}
	}
	s.PC[t]++
	return false, nil
}

func (s *srState) root(p ScopeProgram) (blocked bool, err error) {
	switch s.PC[0] {
	case 0:
		if p.RootA {
			s.A = s.acquire(0, p.Fail == 'A', false, -1)
		}
	case 1:
		s.B = s.acquire(1, p.Fail == 'B', false, -1)
		if p.UserU {
			s.U = s.acquire(1, p.Fail == 'U', true, s.B)
		}
	case 2: // spawn: the children become runnable
	case 3:
		s.Cancel = p.Cancel
	case 4:
		if _, e := s.use(0, s.B); e != nil {
			return false, e
		}
		if p.Move == 1 && p.ChildC {
			c := s.C[0]
			if c < 0 || s.Res[c].Scope != 1 {
				if s.PC[1] <= srChildEnd {
					return true, nil // wait for the winner (a channel receive in the program)
				}
			} else if _, e := s.use(0, c); e != nil {
				return false, e
			}
		}
	case 5: // the nursery's end: join, release, outcome
		joined := true
		for k := 1; k <= p.K; k++ {
			joined = joined && s.PC[k] > srChildEnd
		}
		if !joined && !srReleaseBeforeJoin {
			return true, nil
		}
		if !s.Scopes[1].Ended {
			failedChild := false
			for k := 0; k < p.K; k++ {
				failedChild = failedChild || s.Out[k] == 2
			}
			failed, e := s.release(1, !failedChild)
			if e != nil {
				return false, e
			}
			s.RootErr = s.route(failed, failedChild) || failedChild
		}
		if !joined {
			return true, nil
		}
	case 6:
		if !s.RootErr {
			if p.Escape {
				if _, e := s.use(0, s.B); e != nil {
					return false, e
				}
			}
			if s.A >= 0 {
				if _, e := s.use(0, s.A); e != nil {
					return false, e
				}
			}
		}
	case srRootEnd:
		if !srTaskEndLeaks {
			failed, e := s.release(0, !s.RootErr)
			if e != nil {
				return false, e
			}
			s.route(failed, s.RootErr)
		}
	}
	s.PC[0]++
	return false, nil
}

// finalCheck: every resource released exactly once and every failed release recorded once.
func (s *srState) finalCheck() error {
	for r, x := range s.Res {
		if x.Closes != 1 {
			return fmt.Errorf("resource %d closed or released %d times", r, x.Closes)
		}
		if x.Failed && x.Sinks != 1 {
			return fmt.Errorf("the failed release of resource %d reached %d places, want 1", r, x.Sinks)
		}
	}
	return nil
}

// ScopeResult is the outcome of exploring one program.
type ScopeResult struct {
	States, Ends int
	Err          error
}

// ExploreScopes runs p over every interleaving of its K+1 tasks.
func ExploreScopes(p ScopeProgram) ScopeResult {
	s0 := &srState{PC: make([]int, p.K+1), Out: make([]int, p.K), C: make([]int, p.K), A: -1, B: -1, U: -1}
	s0.Scopes = append(s0.Scopes, srScope{Parent: -1}, srScope{Parent: 0})
	for k := 0; k < p.K; k++ {
		s0.C[k] = -1
		s0.Scopes = append(s0.Scopes, srScope{Parent: 1})
	}
	var res ScopeResult
	seen := map[string]bool{}
	var dfs func(s *srState, sched []int) bool
	dfs = func(s *srState, sched []int) bool {
		k := s.key()
		if seen[k] {
			return true
		}
		seen[k] = true
		res.States++
		live, moved := false, false
		for t := 0; t <= p.K; t++ {
			last := srChildEnd
			if t == 0 {
				last = srRootEnd
			} else if s.PC[0] <= 2 {
				live = live || s.PC[t] <= last
				continue // not spawned yet
			}
			if s.PC[t] > last {
				continue
			}
			live = true
			n := s.clone()
			var blocked bool
			var err error
			if t == 0 {
				blocked, err = n.root(p)
			} else {
				blocked, err = n.child(p, t)
			}
			if blocked && err == nil && n.key() == k {
				continue
			}
			moved = true
			sc := append(append([]int(nil), sched...), t)
			if err != nil {
				res.Err = fmt.Errorf("%v (schedule %v)", err, sc)
				return false
			}
			if !dfs(n, sc) {
				return false
			}
		}
		if live && !moved {
			res.Err = fmt.Errorf("deadlock: every live task is blocked (schedule %v)", sched)
			return false
		}
		if !live {
			res.Ends++
			if err := s.finalCheck(); err != nil {
				res.Err = fmt.Errorf("%v (schedule %v)", err, sched)
				return false
			}
		}
		return true
	}
	dfs(s0, nil)
	return res
}
