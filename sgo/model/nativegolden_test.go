// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// nativeGoldenScenarios are the W07 reference scenarios: the runtime lane must show that every
// outcome it produces for one of them is in the model's outcome set.
var nativeGoldenScenarios = map[string]NScenario{
	"native-buffered":       {Caps: []int{1}, Tasks: [][]NOp{{snd(0, 1), snd(0, 2)}, {rcv(0), rcv(0)}}},
	"native-rendezvous":     {Caps: []int{0}, Tasks: [][]NOp{{snd(0, 1)}, {snd(0, 2)}, {rcv(0), rcv(0)}}},
	"native-close-race":     {Caps: []int{1}, Tasks: [][]NOp{{snd(0, 1), snd(0, 2)}, {cls(0)}, {rcv(0), rcv(0)}}},
	"native-select-recv":    {Caps: []int{1, 1}, Tasks: [][]NOp{{snd(0, 1)}, {snd(1, 2)}, {{Kind: NSelect, Arms: []NArm{{Ch: 0}, {Ch: 1}}}, {Kind: NSelect, Arms: []NArm{{Ch: 0}, {Ch: 1}}}}}},
	"native-select-send":    {Caps: []int{0, 0}, Tasks: [][]NOp{{{Kind: NSelect, Arms: []NArm{{Send: true, Ch: 0, Val: 1}, {Send: true, Ch: 1, Val: 2}}}, cls(0), cls(1)}, {rcv(0)}, {rcv(1)}}},
	"native-select-default": {Caps: []int{1}, Tasks: [][]NOp{{{Kind: NSelect, Arms: []NArm{{Ch: 0}}, Default: true}}, {snd(0, 5)}}},
	"native-cancel-recv":    {Caps: []int{0}, Tasks: [][]NOp{{rcv(0)}, {snd(0, 7)}}, Cancellable: []bool{true, true}},
	"native-send-closed":    {Caps: []int{1}, Tasks: [][]NOp{{cls(0), snd(0, 9)}, {rcv(0)}}},
	"native-event":          {Caps: []int{}, Events: 1, Tasks: [][]NOp{{{Kind: NWait}}, {{Kind: NWait}}, {{Kind: NSet}, {Kind: NSet}}}, Cancellable: []bool{true, false, false}},
	"native-limiter":        {Caps: []int{}, Limits: []int{1}, Tasks: [][]NOp{{{Kind: NAcquire}, {Kind: NRelease}}, {{Kind: NAcquire}}, {{Kind: NAcquire}, {Kind: NRelease}}}, Cancellable: []bool{false, false, true}},
}

type nativeGolden struct {
	Scenario NScenario `json:"scenario"`
	Outcomes []string  `json:"outcomes"`
	Blocked  bool      `json:"blocked_histories"`
}

// TestNativeGoldenOutcomes pins the outcome sets in testdata/native-outcomes.json and its runtime
// copy (rewrite both with -update).
func TestNativeGoldenOutcomes(t *testing.T) {
	got := map[string]nativeGolden{}
	for name, sc := range nativeGoldenScenarios {
		r := ExploreNative(sc)
		if r.Violation != "" {
			t.Fatalf("%s: %s\n  trace: %s", name, r.Violation, r.Trace)
		}
		var outs []string
		for o := range r.Outcomes {
			outs = append(outs, o)
		}
		sort.Strings(outs)
		got[name] = nativeGolden{Scenario: sc, Outcomes: outs, Blocked: r.Blocked > 0}
	}
	data, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := filepath.Join("testdata", "native-outcomes.json")
	runtimeCopy := filepath.Join("..", "..", "src", "runtime", "testdata", "sgo-native-outcomes.json")
	if *update {
		for _, p := range []string{path, runtimeCopy} {
			if err := os.WriteFile(p, data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	for _, p := range []string{path, runtimeCopy} {
		if want, err := os.ReadFile(p); err != nil || string(want) != string(data) {
			t.Fatalf("%s is missing or stale; review the model change and rerun with -update (%v)", p, err)
		}
	}
}
