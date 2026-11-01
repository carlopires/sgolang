package model

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// labScenarios are the reference scenarios of the lab model; their streams for fixed seeds are
// pinned in testdata/lab-streams.json (labgolden_test.go) for the runtime.
var labScenarios = map[string]LScenario{
	// Two children rendezvous on a capacity-0 channel and write.
	"lab-pingpong": {Caps: []int{0}, K: 1024, Quota: 100000,
		Tasks: [][]LOp{
			{{Kind: LSend, Ch: 0, N: 1}, {Kind: LWrite, Text: "sent\n"}},
			{{Kind: LRecv, Ch: 0}, {Kind: LWrite, Text: "received\n"}},
		}},
	// D06: timers with equal deadlines fire in timer-ID order; time jumps.
	"lab-timer-ties": {K: 1024, Quota: 100000,
		Tasks: [][]LOp{
			{{Kind: LSleep, N: 20}, {Kind: LWrite, Text: "a20\n"}},
			{{Kind: LSleep, N: 10}, {Kind: LWrite, Text: "b10\n"}},
			{{Kind: LSleep, N: 20}, {Kind: LWrite, Text: "c20\n"}},
		}},
	// D07: a runnable task keeps virtual time from advancing; the sleeper wakes after it.
	"lab-runnable-holds-time": {K: 100, Quota: 100000,
		Tasks: [][]LOp{
			{{Kind: LWork, N: 450}, {Kind: LWrite, Text: "work done\n"}},
			{{Kind: LSleep, N: 1}, {Kind: LWrite, Text: "slept\n"}},
		}},
	// D07, D11: a busy loop exhausts the quota.
	"lab-quota": {K: 100, Quota: 1000,
		Tasks: [][]LOp{
			{{Kind: LWork, N: 5000}},
			{{Kind: LWrite, Text: "other\n"}},
		}},
	// D08: a receive nobody serves, while main joins: managed deadlock.
	"lab-deadlock": {Caps: []int{0}, K: 1024, Quota: 100000,
		Tasks: [][]LOp{{{Kind: LRecv, Ch: 0}}}},
	// D03: a select with two ready arms; the choice is recorded.
	"lab-select": {Caps: []int{1, 1}, K: 1024, Quota: 100000,
		Root: []LOp{{Kind: LSend, Ch: 0, N: 1}, {Kind: LSend, Ch: 1, N: 2},
			{Kind: LSelect, Arms: []LArm{{Ch: 0}, {Ch: 1}}}, {Kind: LWrite, Text: "chose\n"}}},
	// Cancellation wakes a sleeper; a child that fails cancels its siblings.
	"lab-cancel": {Caps: []int{0}, K: 1024, Quota: 100000,
		Tasks: [][]LOp{
			{{Kind: LSleep, N: 50}, {Kind: LWrite, Text: "not reached\n"}},
			{{Kind: LClose, Ch: 0}, {Kind: LSend, Ch: 0, N: 1}},
		}},
	// D13: an ordering bug: the writer may run before or after the reader's check.
	"lab-order-bug": {Caps: []int{1}, K: 1024, Quota: 100000,
		Tasks: [][]LOp{
			{{Kind: LWrite, Text: "x"}, {Kind: LSend, Ch: 0, N: 1}},
			{{Kind: LWrite, Text: "y"}, {Kind: LRecv, Ch: 0}},
		},
		Root: []LOp{{Kind: LWrite, Text: "z"}}},
	// main cancels the nursery while a child waits; fifo policy.
	"lab-fifo-cancel": {Caps: []int{0}, K: 1024, Quota: 100000, Policy: "fifo",
		Tasks: [][]LOp{{{Kind: LRecv, Ch: 0}}, {{Kind: LCheckpoint}, {Kind: LWrite, Text: "c\n"}}},
		Root:  []LOp{{Kind: LWrite, Text: "m\n"}, {Kind: LCancel}}},
}

// TestLabPCGVectors checks the generator against math/rand/v2 and SplitMix64 against its
// reference output (D01, D44).
func TestLabPCGVectors(t *testing.T) {
	for _, seed := range [][2]uint64{{0, 0}, {42, 0}, {1, 2}, {^uint64(0), 12345}} {
		ref := rand.NewPCG(seed[0], seed[1])
		p := PCG{seed[0], seed[1]}
		for i := range 1000 {
			if a, b := p.Uint64(), ref.Uint64(); a != b {
				t.Fatalf("seed %v draw %d: %#x, math/rand/v2 %#x", seed, i, a, b)
			}
		}
		rr := rand.New(rand.NewPCG(seed[0], seed[1]))
		q := PCG{seed[0], seed[1]}
		for i := range 1000 {
			n := uint64(i%37 + 1)
			if a, b := q.Uint64n(n), rr.Uint64N(n); a != b {
				t.Fatalf("seed %v bounded draw %d (n=%d): %d, math/rand/v2 %d", seed, i, n, a, b)
			}
		}
	}
	x := uint64(0)
	if got := SplitMix64(&x); got != 0xe220a8397b1dcdaf {
		t.Fatalf("SplitMix64(0) = %#x, want 0xe220a8397b1dcdaf", got)
	}
}

// TestLabStreamIsolation: draws from the data or fault stream do not change the sched stream.
func TestLabStreamIsolation(t *testing.T) {
	a, b := NewStreams(7), NewStreams(7)
	for range 100 {
		b.Data.Uint64()
		b.Fault.Uint64()
	}
	for i := range 100 {
		if a.Sched.Uint64() != b.Sched.Uint64() {
			t.Fatalf("sched draw %d changed by data/fault draws", i)
		}
	}
	if NewStreams(7).Sched == NewStreams(8).Sched || NewStreams(7).Sched == NewStreams(7).Data {
		t.Fatal("streams are not distinct")
	}
}

// TestLabDeterministicAndReplay: the same seed gives the same stream; replaying a stream
// reproduces it and takes every decision from it (D02, D03).
func TestLabDeterministicAndReplay(t *testing.T) {
	for name, sc := range labScenarios {
		for seed := range uint64(50) {
			a, err := RunLab(sc, seed)
			if err != nil {
				t.Fatalf("%s seed %d: %v", name, seed, err)
			}
			b, _ := RunLab(sc, seed)
			if !slices.Equal(a.Lines(), b.Lines()) {
				t.Fatalf("%s seed %d: two runs differ", name, seed)
			}
			r, err := ReplayLab(sc, ParseLabTrace(a.Lines()))
			if err != nil {
				t.Fatalf("%s seed %d: replay: %v", name, seed, err)
			}
			if !slices.Equal(a.Lines(), r.Lines()) || r.Outcome() != a.Outcome() {
				t.Fatalf("%s seed %d: replay differs", name, seed)
			}
		}
	}
}

// TestLabDivergence: an altered choice or a truncated stream stops replay at the first mismatch
// with the expected and actual events and the last matching one (D05, D12).
func TestLabDivergence(t *testing.T) {
	sc := labScenarios["lab-order-bug"]
	a, _ := RunLab(sc, 3)
	tr := ParseLabTrace(a.Lines())
	i := slices.IndexFunc(tr, func(e LEvent) bool { return e.Kind == "choose" && len(e.Ready) > 1 })
	if i < 0 {
		t.Fatal("no decision with several ready tasks")
	}
	bad := slices.Clone(tr)
	bad[i].Pick = 99 // not ready
	_, err := ReplayLab(sc, bad)
	d, ok := err.(*Divergence)
	if !ok || d.Seq != i || d.LastMatching != i-1 || !strings.Contains(d.Expected, `"pick":99`) {
		t.Fatalf("altered pick: got %v", err)
	}
	if _, err := ReplayLab(sc, tr[:len(tr)-1]); err == nil {
		t.Fatal("a truncated stream replayed as complete")
	}
}

// TestLabTimeRules: time jumps only when nothing is ready; ties fire by timer ID; a runnable task
// holds time; quota exhaustion and deadlock are distinct outcomes (D06, D07, D08).
func TestLabTimeRules(t *testing.T) {
	// Under fifo the sleeps start in spawn order (timer IDs 0, 1, 2) and the tied timers' tasks
	// become ready in timer-ID order; under random, either may run first after the tie.
	fifo := labScenarios["lab-timer-ties"]
	fifo.Policy = "fifo"
	r, _ := RunLab(fifo, 1)
	if r.Stdout != "b10\na20\nc20\n" {
		t.Fatalf("timer ties (fifo): stdout %q", r.Stdout)
	}
	for seed := range uint64(30) {
		if rr, _ := RunLab(labScenarios["lab-timer-ties"], seed); !strings.HasPrefix(rr.Stdout, "b10\n") {
			t.Fatalf("seed %d: the earlier deadline did not fire first: %q", seed, rr.Stdout)
		}
	}
	var fires []string
	for _, e := range r.Events {
		if e.Kind == "advance" || e.Kind == "fire" {
			fires = append(fires, e.Line())
		}
	}
	want := []string{`{"e":"advance","to":10}`, `{"e":"fire","task":2,"timer":1}`,
		`{"e":"advance","to":20}`, `{"e":"fire","task":1,"timer":0}`, `{"e":"fire","task":3,"timer":2}`}
	if !slices.Equal(fires, want) {
		t.Fatalf("time events %v, want %v", fires, want)
	}
	for seed := range uint64(30) {
		r, _ := RunLab(labScenarios["lab-runnable-holds-time"], seed)
		if r.Stdout != "work done\nslept\n" {
			t.Fatalf("seed %d: time advanced while a task was runnable: %q", seed, r.Stdout)
		}
		if q, _ := RunLab(labScenarios["lab-quota"], seed); q.Kind != "LabQuotaExhausted" || q.Status != 2 {
			t.Fatalf("seed %d: quota outcome %s", seed, q.Outcome())
		}
	}
	if d, _ := RunLab(labScenarios["lab-deadlock"], 0); d.Kind != "LabDeadlock" {
		t.Fatalf("deadlock outcome %s", d.Outcome())
	}
}

// TestLabExplore: enumeration gives each scenario's outcome set, and every seeded run is in it.
func TestLabExplore(t *testing.T) {
	for name, sc := range labScenarios {
		set, runs, err := ExploreLab(sc, 200000)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for seed := range uint64(100) {
			r, _ := RunLab(sc, seed)
			if !set[r.Outcome()] {
				t.Fatalf("%s seed %d: outcome %s not in the explored set", name, seed, r.Outcome())
			}
		}
		t.Logf("%s: %d runs, %d outcomes", name, runs, len(set))
	}
}
