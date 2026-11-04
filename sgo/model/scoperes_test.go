// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import (
	"fmt"
	"testing"
)

func scopePrograms() []ScopeProgram {
	var ps []ScopeProgram
	for k := 1; k <= 2; k++ {
		for _, a := range []bool{false, true} {
			for _, u := range []bool{false, true} {
				for _, c := range []bool{false, true} {
					for mv := 0; mv <= 2; mv++ {
						for _, cb := range []bool{false, true} {
							for _, cn := range []bool{false, true} {
								for _, f := range []byte{0, 'A', 'B', 'C', 'U'} {
									for _, esc := range []bool{false, true} {
										p := ScopeProgram{K: k, RootA: a, UserU: u, ChildC: c, Move: mv, CloseB: cb, Cancel: cn, Fail: f, Escape: esc}
										if (mv != 0 && !c) || (mv == 2 && k < 2) || (f == 'A' && !a) || (f == 'C' && !c) || (f == 'U' && !u) {
											continue
										}
										ps = append(ps, p)
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return ps
}

// Every admitted variant keeps properties 1–5 on every interleaving.
func TestScopes(t *testing.T) {
	states, admitted := 0, 0
	for _, p := range scopePrograms() {
		if !p.Admitted() {
			continue
		}
		admitted++
		r := ExploreScopes(p)
		if r.Err != nil {
			t.Fatalf("%+v: %v", p, r.Err)
		}
		if r.Ends == 0 {
			t.Fatalf("%+v: no complete run", p)
		}
		states += r.States
	}
	t.Logf("%d programs admitted (of %d), %d states explored, no violation", admitted, len(scopePrograms()), states)
}

// Each mutation of the protocol is found as a concrete violation with its schedule.
func TestScopesMutations(t *testing.T) {
	for _, m := range []struct {
		name string
		flag *bool
	}{
		{"release-before-join", &srReleaseBeforeJoin},
		{"release-fifo", &srReleaseFIFO},
		{"close-keeps-in-list", &srCloseKeepsInList},
		{"move-no-unlink", &srMoveNoUnlink},
		{"move-any-scope", &srMoveAnyScope},
		{"drop-chain", &srDropChain},
		{"task-end-leaks", &srTaskEndLeaks},
		{"escape-unchecked", &srEscapeUnchecked},
		{"release-ok-true", &srReleaseOKTrue},
	} {
		*m.flag = true
		var err error
		for _, p := range scopePrograms() {
			if !p.Admitted() {
				continue
			}
			if r := ExploreScopes(p); r.Err != nil {
				err = fmt.Errorf("%+v: %v", p, r.Err)
				break
			}
		}
		*m.flag = false
		if err == nil {
			t.Errorf("mutation %s not detected", m.name)
			continue
		}
		t.Logf("mutation %s detected: %v", m.name, err)
	}
}
