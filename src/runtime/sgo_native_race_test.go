// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// W07 race gate (N07): the race detector sees the synchronization of native channels, close,
// Event, and CapacityLimiter, and still reports a deliberate race. Run with -race.

package runtime_test

import (
	"internal/race"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

var sgoRaceShared int

// sgoRaceScenarios each run two tasks that write and read sgoRaceShared, ordered only by the named
// primitive ("racy": not ordered at all).
var sgoRaceScenarios = map[string]func(t *testing.T){
	"buffered": func(t *testing.T) {
		c := runtime.SGoNativeNew(1)
		twoTasks(t, func() { sgoRaceShared = 1; runtime.SGoNSend(c, 1) },
			func() { runtime.SGoNRecv(c); _ = sgoRaceShared })
	},
	"rendezvous": func(t *testing.T) {
		// For capacity 0 the receive also happens before the send completes.
		c := runtime.SGoNativeNew(0)
		twoTasks(t, func() { runtime.SGoNSend(c, 1); sgoRaceShared = 1 },
			func() { _ = sgoRaceShared; runtime.SGoNRecv(c) })
	},
	"close": func(t *testing.T) {
		c := runtime.SGoNativeNew(0)
		twoTasks(t, func() { sgoRaceShared = 1; runtime.SGoNClose(c) },
			func() { runtime.SGoNRecv(c); _ = sgoRaceShared })
	},
	"select": func(t *testing.T) {
		a, b := runtime.SGoNativeNew(0), runtime.SGoNativeNew(0)
		twoTasks(t, func() {
			sgoRaceShared = 1
			runtime.SGoNSelect([]runtime.SGoNArm{{Send: true, Ch: a, Val: 1}, {Send: true, Ch: b, Val: 2}}, true)
		},
			func() {
				runtime.SGoNSelect([]runtime.SGoNArm{{Ch: a}, {Ch: b}}, true)
				_ = sgoRaceShared
			})
	},
	"event": func(t *testing.T) {
		ev := runtime.SGoEventNew()
		twoTasks(t, func() { sgoRaceShared = 1; runtime.SGoEventSet(ev) },
			func() {
				if runtime.SGoEventWait(ev) == nil {
					_ = sgoRaceShared
				}
			})
	},
	"limiter": func(t *testing.T) {
		lim := runtime.SGoLimiterNew(1)
		use := func() {
			k, err := runtime.SGoAcquire(lim)
			if err != nil {
				return
			}
			sgoRaceShared++
			runtime.SGoRelease(k)
		}
		twoTasks(t, use, use)
	},
	"racy": func(t *testing.T) {
		ready := runtime.SGoNativeNew(2) // only to make the two writes overlap in time
		twoTasks(t, func() { runtime.SGoNSend(ready, 1); sgoRaceShared = 1 },
			func() { runtime.SGoNSend(ready, 1); sgoRaceShared = 2 })
	},
}

func twoTasks(t *testing.T, a, b func()) {
	root(t, func() {
		s := mustOpen(t)
		mustSpawn(t, s, func(*runtime.SGoTask) error { a(); return nil })
		mustSpawn(t, s, func(*runtime.SGoTask) error { b(); return nil })
		mustJoin(t, s)
	})
}

// TestSGoNativeRaceHelper runs one scenario in a child process (see TestSGoNativeRace).
func TestSGoNativeRaceHelper(t *testing.T) {
	name := os.Getenv("SGO_RACE_SCENARIO")
	if name == "" {
		t.Skip("helper of TestSGoNativeRace")
	}
	sgoRaceScenarios[name](t)
}

func TestSGoNativeRace(t *testing.T) {
	if !race.Enabled {
		t.Skip("the race gate runs with -race")
	}
	for name := range sgoRaceScenarios {
		t.Run(name, func(t *testing.T) {
			raced := false
			for range 10 { // ten histories per scenario
				cmd := exec.Command(os.Args[0], "-test.run=^TestSGoNativeRaceHelper$", "-test.count=1")
				cmd.Env = append(os.Environ(), "SGO_RACE_SCENARIO="+name, "GORACE=halt_on_error=1")
				out, _ := cmd.CombinedOutput()
				if strings.Contains(string(out), "WARNING: DATA RACE") {
					raced = true
					if name != "racy" {
						t.Fatalf("race reported across a synchronizing %s:\n%s", name, out)
					}
					break
				}
			}
			if name == "racy" && !raced {
				t.Fatal("the deliberate race was not reported")
			}
		})
	}
}
