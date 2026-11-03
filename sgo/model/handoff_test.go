// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import (
	"fmt"
	"testing"
)

func handoffPrograms() []HandoffProgram {
	var ps []HandoffProgram
	for n := 1; n <= 3; n++ {
		for _, lc := range []bool{false, true} {
			for _, et := range []bool{false, true} {
				for _, nt := range []bool{false, true} {
					ps = append(ps, HandoffProgram{N: n, LoserCloses: lc, EarlyTake: et, NoTake: nt})
				}
			}
		}
	}
	return ps
}

// Every variant keeps the invariants on every interleaving, and every child can win the race.
func TestHandoff(t *testing.T) {
	states := 0
	for _, p := range handoffPrograms() {
		r := ExploreHandoff(p)
		if r.Err != nil {
			t.Fatalf("%+v: %v", p, r.Err)
		}
		states += r.States
		if !p.NoTake && len(r.Winners) != p.N {
			t.Errorf("%+v: winners %v, want every one of %d children", p, r.Winners, p.N)
		}
	}
	t.Logf("%d programs, %d states explored, no violation", len(handoffPrograms()), states)
}

// Each mutation of the protocol is found as a concrete violation with its schedule.
func TestHandoffMutations(t *testing.T) {
	for _, m := range []struct {
		name string
		flag *bool
	}{
		{"put-overwrites", &handoffPutOverwrites},
		{"put-keeps-link", &handoffPutKeepsLink},
		{"slot-leaks", &handoffSlotLeaks},
	} {
		*m.flag = true
		var err error
		for _, p := range handoffPrograms() {
			if r := ExploreHandoff(p); r.Err != nil {
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
