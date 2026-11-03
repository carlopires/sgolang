// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

// Exploration of the sharing rule: every interleaving of an accepted program, and a bounded
// generator of programs for the soundness check (accepted ⇒ no ownership violation).

import (
	"fmt"
	"math/rand"
	"slices"
)

// ShareResult summarizes one exploration.
type ShareResult struct {
	States  int
	Bounded bool  // the state bound was reached: the exploration is incomplete
	Err     error // the first ownership violation found, with its schedule
}

const shareMaxStates = 200000

// ExploreShare runs p over every interleaving of its tasks.
func ExploreShare(p SProgram) ShareResult {
	init := &sState{Chans: make([][]int, len(p.Chans)), Tasks: []sTask{{Vars: map[string]int{}, Kinds: map[string]SKind{}}}}
	seen := map[string]bool{}
	var res ShareResult
	var dfs func(s *sState, sched []int) bool
	dfs = func(s *sState, sched []int) bool {
		k := s.key()
		if seen[k] {
			return true
		}
		seen[k] = true
		res.States++
		if res.States > shareMaxStates {
			res.Bounded = true
			return false
		}
		for t := range s.Tasks {
			if s.Tasks[t].PC >= len(p.Bodies[s.Tasks[t].Body]) {
				continue
			}
			n := s.clone()
			blocked, err := n.step(p, t)
			if blocked {
				continue
			}
			sc := append(append([]int(nil), sched...), t)
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
	dfs(init, nil)
	return res
}

// GenShare generates a random program: a root body and up to two child bodies over three
// channels (Ref, Frozen, Val). Generation is type-directed (statements use defined variables of a
// fitting kind) so that most rejections come from the sharing rule, not from type errors; the
// sharing-relevant choices (plain, copy or move sends, captures of any kind, aliasing, use after
// move, writes through frozen values) are left unconstrained.
func GenShare(r *rand.Rand) SProgram { return genShare(r, false, false) }

// GenShareErr is GenShare with error values (W13 review V2): it also generates V := error(W) for W
// of any kind, a fourth channel of error values, and child reads and writes through errors (the
// Error method of a value-receiver type writing what its value refers to). GenShare's stream is
// unchanged, so its recorded acceptance figures stay reproducible.
func GenShareErr(r *rand.Rand) SProgram { return genShare(r, true, false) }

// GenShareIface is GenShareErr with interface values (W17, D132): V := I(W) for Val and Ref W,
// assertions back to a Ref, a fifth channel of interface values, method calls (reads and writes,
// direct and through the dynamic value's referents), and error(i). Sends of interfaces use any mode,
// captures any kind; GenShare's and GenShareErr's streams are unchanged.
func GenShareIface(r *rand.Rand) SProgram { return genShare(r, true, true) }

func genShare(r *rand.Rand, withErr, withIface bool) SProgram {
	p := SProgram{Chans: []SKind{SRef, SFrozen, SVal}}
	lastOp := SSpawn
	childKinds := []SKind{SRef, SFrozen}
	if withErr {
		p.Chans = append(p.Chans, SErr)
		lastOp = SToErr
		childKinds = append(childKinds, SErr)
	}
	if withIface {
		p.Chans = append(p.Chans, SIface)
		lastOp = SAssert
		childKinds = append(childKinds, SIface)
	}
	nb := 1 + r.Intn(3)
	names := []string{"a", "b", "c", "d"}
	caps := make([]map[string]SKind, nb) // kinds captured by the body's first spawn
	caps[0] = map[string]SKind{}
	chanOf := map[SKind]int{SRef: 0, SFrozen: 1, SVal: 2, SErr: 3, SIface: 4}
	for b := 0; b < nb; b++ {
		kinds := map[string]SKind{}
		for v, k := range caps[b] {
			kinds[v] = k
		}
		defined := func(want ...SKind) (string, bool) {
			var vs []string
			for _, v := range names {
				if k, ok := kinds[v]; ok && (len(want) == 0 || slices.Contains(want, k)) {
					vs = append(vs, v)
				}
			}
			if len(vs) == 0 {
				return "", false
			}
			return vs[r.Intn(len(vs))], true
		}
		fresh := names[r.Intn(len(names))]
		var body []SStmt
		n := 3 + r.Intn(6)
		for i := 0; i < n; i++ {
			fresh = names[r.Intn(len(names))]
			var st SStmt
			ok := true
			switch op := SOp(r.Intn(int(lastOp) + 1)); op {
			case SAlloc:
				k := SRef
				if r.Intn(3) == 0 {
					k = SVal
				}
				st = SStmt{Op: op, V: fresh, Kind: k}
				kinds[fresh] = k
			case SChild:
				var v string
				v, ok = defined(SRef)
				st = SStmt{Op: op, V: v}
			case SAlias:
				var w string
				w, ok = defined()
				st = SStmt{Op: op, V: fresh, W: w}
				if ok {
					kinds[fresh] = kinds[w]
				}
			case SStore:
				var v, w string
				v, ok = defined(SRef)
				if ok {
					w, ok = defined(SRef)
				}
				st = SStmt{Op: op, V: v, W: w}
			case SWrite, SRead:
				var v string
				v, ok = defined()
				st = SStmt{Op: op, V: v}
			case SWriteChild, SReadChild:
				var v string
				v, ok = defined(childKinds...)
				st = SStmt{Op: op, V: v}
			case SSend:
				var v string
				v, ok = defined()
				m := SMode(r.Intn(3))
				if ok && kinds[v] != SRef && m == SMove {
					m = SPlain
				}
				st = SStmt{Op: op, V: v, Ch: chanOf[kinds[v]], Mode: m}
			case SToErr:
				var w string
				w, ok = defined(SVal, SRef, SErr, SIface)
				st = SStmt{Op: op, V: fresh, W: w}
				if ok {
					kinds[fresh] = SErr
				}
			case SToIface:
				var w string
				w, ok = defined(SVal, SRef)
				st = SStmt{Op: op, V: fresh, W: w}
				if ok {
					kinds[fresh] = SIface
				}
			case SAssert:
				var w string
				w, ok = defined(SIface)
				st = SStmt{Op: op, V: fresh, W: w}
				if ok {
					kinds[fresh] = SRef
				}
			case SRecv:
				ch := r.Intn(len(p.Chans))
				st = SStmt{Op: op, V: fresh, Ch: ch}
				kinds[fresh] = p.Chans[ch]
			case SFreeze:
				var w string
				w, ok = defined(SRef, SFrozen)
				st = SStmt{Op: op, V: fresh, W: w}
				if ok {
					kinds[fresh] = SFrozen
				}
			case SSpawn:
				var v string
				v, ok = defined()
				if !ok || b+1 >= nb {
					ok = false
					break
				}
				c := b + 1 + r.Intn(nb-b-1)
				st = SStmt{Op: op, Body: c, Caps: []string{v}}
				if caps[c] == nil {
					caps[c] = map[string]SKind{v: kinds[v]}
				}
			}
			if ok {
				body = append(body, st)
			}
		}
		p.Bodies = append(p.Bodies, body)
	}
	return p
}

// ShareSoundness generates n programs from seed and explores every accepted one. It returns the
// number accepted, the number whose exploration reached the state bound (incomplete), and the
// first accepted program with a violation.
func ShareSoundness(seed int64, n int) (accepted, bounded int, bad *SProgram, err error) {
	return shareSoundness(GenShare, seed, n)
}

// ShareIfaceSoundness is ShareSoundness over GenShareIface's programs.
func ShareIfaceSoundness(seed int64, n int) (accepted, bounded int, bad *SProgram, err error) {
	return shareSoundness(GenShareIface, seed, n)
}

// ShareErrSoundness is ShareSoundness over GenShareErr's programs.
func ShareErrSoundness(seed int64, n int) (accepted, bounded int, bad *SProgram, err error) {
	return shareSoundness(GenShareErr, seed, n)
}

func shareSoundness(gen func(*rand.Rand) SProgram, seed int64, n int) (accepted, bounded int, bad *SProgram, err error) {
	r := rand.New(rand.NewSource(seed))
	for i := 0; i < n; i++ {
		p := gen(r)
		if CheckShare(p) != "" {
			continue
		}
		accepted++
		res := ExploreShare(p)
		if res.Err != nil {
			return accepted, bounded, &p, res.Err
		}
		if res.Bounded {
			bounded++
		}
	}
	return accepted, bounded, nil, nil
}
