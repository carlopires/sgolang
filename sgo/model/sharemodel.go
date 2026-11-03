// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

// The sharing rule for mutable aggregates (W13 step 13.0, D86, D89, D90).
//
// A program is a set of task bodies over variables of three static kinds:
//
//	Val     a reference-free aggregate (a struct of integers): copied implicitly wherever it goes
//	Ref     an aggregate with internal references (slices, maps, pointers): task-local
//	Frozen  a deeply immutable value (sgo.Frozen[T]): shareable by any number of tasks
//
// CheckShare is the proposed compile-time rule. ExploreShare runs an accepted program over every
// interleaving and checks the ownership invariant: every access to a mutable object is made by the
// task that currently owns it, and ownership changes only by a receive. Exclusive ownership with
// channel hand-off implies there is no data race (SGO-MEM-003).

import "fmt"

// SKind is a variable's static kind.
type SKind byte

const (
	SVal    SKind = 'v'
	SRef    SKind = 'r'
	SFrozen SKind = 'f'
)

// SOp is a statement.
type SOp byte

const (
	SAlloc      SOp = iota // V := new(Kind)          (Kind is Val or Ref)
	SChild                 // V.child = new(Ref)      (nests a fresh object under V)
	SAlias                 // V := W                  (V shares W's object)
	SStore                 // V.child = W             (links W's object into V's graph)
	SWrite                 // V.x = 1
	SWriteChild            // V.child.x = 1
	SRead                  // _ = V.x
	SReadChild             // _ = V.child.x
	SSend                  // Ch <- Mode(V)
	SRecv                  // V := <-Ch
	SFreeze                // V := sgo.Freeze(W)     (deep copy into frozen objects)
	SSpawn                 // n.Spawn(Body), capturing Caps
)

// SMode is how a send passes its value.
type SMode byte

const (
	SPlain SMode = iota // ch <- v
	SCopy               // ch <- sgo.Copy(v)
	SMove               // ch <- move(v)
)

// SStmt is one statement of a task body.
type SStmt struct {
	Op   SOp
	V, W string
	Kind SKind // SAlloc
	Ch   int   // SSend, SRecv
	Mode SMode // SSend
	Body int   // SSpawn: index of the child body
	Caps []string
}

// SProgram is a program: Bodies[0] is the root task; Chans gives each channel's element kind.
type SProgram struct {
	Bodies [][]SStmt
	Chans  []SKind
}

// Mutations of the checker, for the model's seeded-bug tests.
var (
	shareAllowPlainRef    bool // accept ch <- v for a Ref v (no copy)
	shareAllowRefCapture  bool // accept a Spawn capture of a Ref variable
	shareAllowAliasedMove bool // accept move(v) when v has been aliased or stored
	shareAllowFrozenWrite bool // accept writes through a Frozen variable
	shareShallowCopy      bool // sgo.Copy copies only the top object (runtime mutation)
)

// CheckShare applies the proposed compile-time rule and returns the diagnostic code of the first
// violation, or "" if the program is accepted.
// Child bodies are checked at each Spawn that starts them, with that Spawn's captures.
func CheckShare(p SProgram) string {
	return checkBody(p, 0, nil)
}

// checkBody checks body b. caps gives the kinds of the variables captured by the Spawn that
// starts it (nil for the root).
func checkBody(p SProgram, b int, caps map[string]SKind) string {
	kinds := map[string]SKind{}
	for v, k := range caps {
		kinds[v] = k
	}
	tainted := map[string]bool{} // aliased or stored: not movable
	dead := map[string]bool{}    // moved
	use := func(v string) (SKind, string) {
		k, ok := kinds[v]
		if !ok {
			return 0, "SGO_E_UNDEFINED"
		}
		if dead[v] {
			return 0, "SGO_E_USE_AFTER_MOVE"
		}
		return k, ""
	}
	for _, s := range p.Bodies[b] {
		switch s.Op {
		case SAlloc:
			kinds[s.V] = s.Kind
			delete(dead, s.V)
			delete(tainted, s.V)
		case SChild:
			k, e := use(s.V)
			if e != "" {
				return e
			}
			if k != SRef {
				return checkKindErr(k)
			}
		case SAlias:
			k, e := use(s.W)
			if e != "" {
				return e
			}
			kinds[s.V] = k
			delete(dead, s.V)
			if k == SRef {
				tainted[s.V], tainted[s.W] = true, true
			}
		case SStore:
			kv, e := use(s.V)
			if e != "" {
				return e
			}
			kw, e := use(s.W)
			if e != "" {
				return e
			}
			if kv != SRef || kw != SRef {
				return checkKindErr(kv)
			}
			tainted[s.V], tainted[s.W] = true, true
		case SWrite, SWriteChild:
			k, e := use(s.V)
			if e != "" {
				return e
			}
			if k == SFrozen && !shareAllowFrozenWrite {
				return "SGO_E_FROZEN_WRITE"
			}
			if s.Op == SWriteChild && k == SVal {
				return "SGO_E_TYPE"
			}
		case SRead, SReadChild:
			k, e := use(s.V)
			if e != "" {
				return e
			}
			if s.Op == SReadChild && k == SVal {
				return "SGO_E_TYPE"
			}
		case SSend:
			k, e := use(s.V)
			if e != "" {
				return e
			}
			if k != p.Chans[s.Ch] {
				return "SGO_E_TYPE"
			}
			switch s.Mode {
			case SPlain:
				if k == SRef && !shareAllowPlainRef {
					return "SGO_E_UNCOPIED_SEND"
				}
			case SMove:
				if k != SRef {
					return "SGO_E_TYPE"
				}
				if tainted[s.V] && !shareAllowAliasedMove {
					return "SGO_E_MOVE_ALIASED"
				}
				dead[s.V] = true
			}
		case SRecv:
			kinds[s.V] = p.Chans[s.Ch]
			delete(dead, s.V)
			delete(tainted, s.V)
		case SFreeze:
			k, e := use(s.W)
			if e != "" {
				return e
			}
			if k == SVal {
				return "SGO_E_TYPE"
			}
			kinds[s.V] = SFrozen
			delete(dead, s.V)
		case SSpawn:
			cm := map[string]SKind{}
			for _, c := range s.Caps {
				k, e := use(c)
				if e != "" {
					return e
				}
				if k == SRef && !shareAllowRefCapture {
					return "SGO_E_TASK_LOCAL_CAPTURE"
				}
				cm[c] = k
			}
			if code := checkBody(p, s.Body, cm); code != "" {
				return code
			}
		default:
			panic(fmt.Sprint("unknown op ", s.Op))
		}
	}
	return ""
}

func checkKindErr(SKind) string { return "SGO_E_TYPE" }
