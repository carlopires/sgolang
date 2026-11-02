// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden histories")

// detects reports how a scenario run under a mutation differs from the correct semantics.
func detects(sc scenario, mut Mutation) string {
	cfg := sc.cfg
	cfg.Mut = mut
	r := Explore(sc.prog, cfg, 200000)
	if r.Violation != "" {
		return "invariant: " + strings.SplitN(r.Violation, "\n", 2)[0]
	}
	if got := strings.Join(r.OutcomeSet(), "|"); got != strings.Join(sc.expect, "|") {
		return "outcome: " + got
	}
	if sc.property != nil {
		for _, s := range r.Terminals {
			if msg := sc.property(s); msg != "" {
				return "property: " + msg
			}
		}
	}
	return ""
}

// TestMutationsDetected: the oracle must reject every deliberately wrong semantics (plan W01 gate).
func TestMutationsDetected(t *testing.T) {
	muts := map[Mutation]string{
		MutEarlyJoin:           "join ignores live children",
		MutLateRegister:        "child registered after publication",
		MutNoCancelOnFail:      "failing child does not cancel its scope",
		MutTaskStickyCancel:    "cancellation sticky per task",
		MutLatestFailure:       "nursery outcome picks latest failure",
		MutNoCurrentScopeCheck: "spawn into a non-current scope allowed",
		MutDoubleWake:          "sleep resolved twice",
		MutDuplicateCompletion: "child completion published twice",
	}
	for mut, desc := range muts {
		var hits []string
		for _, sc := range scenarios {
			if how := detects(sc, mut); how != "" {
				hits = append(hits, sc.name+" ("+how+")")
			}
		}
		if len(hits) == 0 {
			t.Errorf("mutation %q was not detected by any scenario", desc)
			continue
		}
		t.Logf("mutation %q detected by %d scenario(s), e.g. %s", desc, len(hits), hits[0])
	}
}

// TestGoldenHistories pins the first-enabled-event schedule of key scenarios (plan W01 golden list).
func TestGoldenHistories(t *testing.T) {
	for _, name := range []string{"normal-join", "descendant-during-join", "cancel-propagates",
		"inner-cancel-not-outer", "two-failures-lowest-id", "spawn-into-outer-from-inner", "cancel-consumed-at-boundary"} {
		var sc *scenario
		for i := range scenarios {
			if scenarios[i].name == name {
				sc = &scenarios[i]
			}
		}
		final, v := Run(sc.prog, sc.cfg)
		if v != "" {
			t.Fatalf("%s: violation %s", name, v)
		}
		got := join(final.Trace) + "outcome: " + outcomeKey(final) + "\n"
		path := filepath.Join("testdata", "golden", name+".txt")
		if *update {
			os.MkdirAll(filepath.Dir(path), 0o755)
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v (run with -update to create)", name, err)
		}
		if string(want) != got {
			t.Errorf("%s: history differs from %s\n--- got\n%s--- want\n%s", name, path, got, want)
		}
	}
}

// TestCheckRejectsInvalidHistories: hand-made invalid states must be rejected by Check.
func TestCheckRejectsInvalidHistories(t *testing.T) {
	base, _ := Run(Program{N("n", Sp("n", SL(), RN()))}, Config{})
	cases := map[string]func(s *State){
		"closed scope with live child": func(s *State) { s.Scopes[0].Live[1] = true; s.Tasks[1].status = running },
		"scope state went backwards":   func(s *State) { s.Scopes[0].State = Open },
		"wait resolved twice":          func(s *State) { s.waitsDone[99] = 2 },
		"task missing from its scope":  func(s *State) { s.Scopes[0].Children = nil },
		"completion published twice":   func(s *State) { s.published[1] = 2 },
	}
	for name, corrupt := range cases {
		bad := base.Clone()
		corrupt(bad)
		if v := Check(base, bad); v == "" {
			t.Errorf("%s: Check accepted an invalid state", name)
		} else {
			t.Logf("%s: rejected (%s)", name, v)
		}
	}
}
