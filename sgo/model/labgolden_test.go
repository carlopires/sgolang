package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// labGoldenSeeds are the seeds whose canonical streams are pinned for every lab scenario. The
// runtime lane must emit exactly these streams (the same choices from the same seed) and replay
// each of them.
var labGoldenSeeds = []uint64{0, 1, 2, 7, 42}

type labGoldenRun struct {
	Seed    uint64   `json:"seed"`
	Stream  []string `json:"stream"`
	Outcome string   `json:"outcome"`
}

type labGolden struct {
	Scenario LScenario      `json:"scenario"`
	Runs     []labGoldenRun `json:"runs"`
	Outcomes []string       `json:"outcomes"` // every outcome of every schedule (ExploreLab)
}

// TestLabGoldenStreams pins testdata/lab-streams.json and its runtime copy (rewrite both with
// -update).
func TestLabGoldenStreams(t *testing.T) {
	got := map[string]labGolden{}
	for name, sc := range labScenarios {
		g := labGolden{Scenario: sc}
		for _, seed := range labGoldenSeeds {
			r, err := RunLab(sc, seed)
			if err != nil {
				t.Fatalf("%s seed %d: %v", name, seed, err)
			}
			g.Runs = append(g.Runs, labGoldenRun{Seed: seed, Stream: r.Lines(), Outcome: r.Outcome()})
		}
		set, _, err := ExploreLab(sc, 200000)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for o := range set {
			g.Outcomes = append(g.Outcomes, o)
		}
		sort.Strings(g.Outcomes)
		got[name] = g
	}
	data, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := filepath.Join("testdata", "lab-streams.json")
	runtimeCopy := filepath.Join("..", "..", "src", "runtime", "testdata", "sgo-lab-streams.json")
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
