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

// chanGoldenScenarios are the reference scenarios for the W06 runtime: the runtime lane must show
// that every outcome it produces is in the model's outcome set for the same scenario.
var chanGoldenScenarios = map[string]ChanScenario{
	"buffered-two-senders":    {Cap: 1, Senders: []int{SendCommit, SendCommit}, Receivers: 2},
	"buffered-abort":          {Cap: 2, Senders: []int{SendCommit, SendAbort, SendCommit}, Receivers: 2},
	"buffered-close-race":     {Cap: 1, Senders: []int{SendCommit}, Receivers: 1, Close: true},
	"rendezvous-two-senders":  {Cap: 0, Senders: []int{SendCommit, SendCommit}, Receivers: 2},
	"rendezvous-abort-close":  {Cap: 0, Senders: []int{SendCommit, SendAbort}, Receivers: 1, Close: true},
	"buffered-cancel-race":    {Cap: 1, Senders: []int{SendCancelRace}, Receivers: 1},
	"rendezvous-cancel-close": {Cap: 0, Senders: []int{SendCancelRace, SendCommit}, Receivers: 2, Close: true},
	"buffered-task-end":       {Cap: 1, Senders: []int{SendTaskEnd, SendCommit}, Receivers: 1},
	"buffered-alias-double":   {Cap: 2, Senders: []int{SendAliasDouble, SendCommit}, Receivers: 2, Close: true},
	"mixed-cancel-end-alias":  {Cap: 1, Senders: []int{SendCancelRace, SendTaskEnd, SendAliasDouble}, Receivers: 2, Close: true},
}

type chanGolden struct {
	Scenario ChanScenario `json:"scenario"`
	Outcomes []string     `json:"outcomes"`
	Blocked  bool         `json:"blocked_histories"`
}

// TestChanGoldenOutcomes pins the outcome set of each reference scenario in
// testdata/chan-outcomes.json (rewrite with -update).
func TestChanGoldenOutcomes(t *testing.T) {
	got := map[string]chanGolden{}
	for name, sc := range chanGoldenScenarios {
		r := ExploreChan(sc)
		if r.Violation != "" {
			t.Fatalf("%s: %s\n  trace: %s", name, r.Violation, r.Trace)
		}
		var outs []string
		for o := range r.Outcomes {
			outs = append(outs, o)
		}
		sort.Strings(outs)
		got[name] = chanGolden{Scenario: sc, Outcomes: outs, Blocked: r.Deadlocked > 0}
	}
	data, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := filepath.Join("testdata", "chan-outcomes.json")
	// The runtime lane (src/runtime/sgo_chan_test.go) reads a copy that is overlaid with the
	// runtime package; it must be identical.
	runtimeCopy := filepath.Join("..", "..", "src", "runtime", "testdata", "sgo-chan-outcomes.json")
	if *update {
		for _, p := range []string{path, runtimeCopy} {
			if err := os.WriteFile(p, data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	if rc, err := os.ReadFile(runtimeCopy); err != nil || string(rc) != string(data) {
		t.Fatalf("runtime copy %s is missing or stale (rerun with -update): %v", runtimeCopy, err)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create)", err)
	}
	if string(want) != string(data) {
		t.Fatalf("channel model outcomes changed; review the model change and rerun with -update")
	}
}
