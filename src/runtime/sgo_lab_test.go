// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	. "runtime"
	"strings"
	"testing"
)

// The lab scenarios of sgo/model (labmodel.go), pinned with their canonical streams by
// TestLabGoldenStreams there and copied to testdata/sgo-lab-streams.json. The runtime interprets
// each script with its own operations; for every seed its stream must be the model's (D30).

type labOp struct {
	Kind int
	N    int
	Ch   int
	Text string
	Arms []struct {
		Send    bool
		Ch, Val int
	}
	Default bool
}

type labScenario struct {
	Caps   []int
	Root   []labOp
	Tasks  [][]labOp
	K      int
	Quota  int
	Policy string
}

type labGolden struct {
	Scenario labScenario
	Runs     []struct {
		Seed    uint64
		Stream  []string
		Outcome string
	}
}

const (
	lWork = iota
	lSleep
	lSend
	lRecv
	lClose
	lSelect
	lWrite
	lCheckpoint
	lCancel
	lError
)

// labRunScript runs ops in the calling task. It returns the error the task returns (nil: it
// returned normally), or panics with the failure of an operation.
func labRunScript(ops []labOp, chans []*SGoChan, s *SGoScope) error {
	for _, op := range ops {
		switch op.Kind {
		case lWork:
			for range op.N {
				SGoLabStep()
			}
		case lSleep:
			if err := SGoSleep(int64(op.N)); err != nil {
				return err
			}
		case lSend:
			SGoNSend(chans[op.Ch], op.N)
		case lRecv:
			SGoNRecv(chans[op.Ch])
		case lClose:
			SGoNClose(chans[op.Ch])
		case lSelect:
			arms := make([]SGoNArm, len(op.Arms))
			for i, a := range op.Arms {
				arms[i] = SGoNArm{Send: a.Send, Ch: chans[a.Ch], Val: a.Val}
			}
			SGoNSelect(arms, !op.Default)
		case lWrite:
			SGoLabWrite(op.Text)
		case lCheckpoint:
			if err := SGoLabCheckpoint(); err != nil {
				return err
			}
		case lCancel:
			SGoLabCancel(s)
		case lError:
			if s != nil { // main: the error is at the operation's point
				SGoLabPoint()
			}
			return errors.New(op.Text)
		}
	}
	return nil
}

// labRoot is main of a scenario: one nursery, one child per task script, Root, the join, and the
// program's outcome (as sgo/model finishMain computes it).
func labRoot(sc labScenario) {
	chans := make([]*SGoChan, len(sc.Caps))
	for i, c := range sc.Caps {
		chans[i] = SGoNativeNew(c)
	}
	s, err := SGoOpen()
	if err != nil {
		panic(err)
	}
	rootErr := ""
	func() {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if SGoIsCancelUnwind(v) {
				rootErr = "cancelled"
			} else {
				rootErr = "failed:" + SGoFailureKind(v)
			}
			SGoCancel(s) // an unwinding owner cancels its nursery before joining (SGO-DEF-005)
		}()
		for _, ops := range sc.Tasks {
			ops := ops
			if err := SGoSpawn(s, func(*SGoTask) error { return labRunScript(ops, chans, nil) }); err != nil {
				panic(err)
			}
		}
		if err := labRunScript(sc.Root, chans, s); err != nil {
			if SGoIsCancelled(err) {
				rootErr = "cancelled"
				SGoCancel(s)
			} else {
				rootErr = "errored"
			}
		}
	}()
	SGoJoin(s)
	if k, ok := strings.CutPrefix(rootErr, "failed:"); ok {
		SGoLabEnd(2, k)
		return
	}
	status, kind := int32(0), ""
	for _, o := range SGoLabOutcomes() {
		if k, ok := strings.CutPrefix(o, "Failed:"); ok {
			status, kind = 2, k
			break
		}
		if o == "Errored" {
			status, kind = 1, "ChildError"
			break
		}
	}
	if status == 0 && rootErr != "" {
		status, kind = 1, "MainError"
	}
	SGoLabEnd(status, kind)
}

func TestSGoLabStreams(t *testing.T) {
	data, err := os.ReadFile("testdata/sgo-lab-streams.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]labGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	for name, g := range golden {
		for _, run := range g.Runs {
			t.Run(fmt.Sprintf("%s/seed=%d", name, run.Seed), func(t *testing.T) {
				sc := g.Scenario
				res := SGoLabRun(run.Seed, sc.Policy == "fifo", int64(sc.K), int64(sc.Quota), func() { labRoot(sc) })
				if got, want := strings.Join(res.Lines, "\n"), strings.Join(run.Stream, "\n"); got != want {
					t.Fatalf("stream differs from the model's\ngot:\n%s\nwant:\n%s", got, want)
				}
				if !strings.HasPrefix(run.Outcome, fmt.Sprintf("exit=%d kind=%q", res.Status, res.Kind)) {
					t.Errorf("outcome exit=%d kind=%q, model %s", res.Status, res.Kind, run.Outcome)
				}
			})
		}
	}
}

// labPicks are the picks a replay consumes: those of the choose and select events whose set has
// two or more elements (replayPick in sgo_lab_sched.go; the driver extracts them the same way).
func labPicks(t *testing.T, stream []string) []int32 {
	var picks []int32
	for _, line := range stream {
		var e struct {
			E     string
			Ready []int32
			Arms  []int32
			Pick  int32
			Arm   int32
		}
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		switch {
		case e.E == "choose" && len(e.Ready) > 1:
			picks = append(picks, e.Pick)
		case e.E == "select" && len(e.Arms) > 1:
			picks = append(picks, e.Arm)
		}
	}
	return picks
}

// A replay takes every choice from the trace, never from the generator (D46): replaying each
// pinned stream's picks reproduces it, and a pick outside the ready set is a divergence.
func TestSGoLabReplay(t *testing.T) {
	data, err := os.ReadFile("testdata/sgo-lab-streams.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]labGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	for name, g := range golden {
		for _, run := range g.Runs {
			sc := g.Scenario
			picks := labPicks(t, run.Stream)
			res := SGoLabReplay(picks, sc.Policy == "fifo", int64(sc.K), int64(sc.Quota), func() { labRoot(sc) })
			if got, want := strings.Join(res.Lines, "\n"), strings.Join(run.Stream, "\n"); got != want {
				t.Fatalf("%s seed %d: replay differs from the recorded stream\ngot:\n%s\nwant:\n%s", name, run.Seed, got, want)
			}
		}
	}
	sc := golden["lab-pingpong"].Scenario
	res := SGoLabReplay([]int32{7}, false, int64(sc.K), int64(sc.Quota), func() { labRoot(sc) })
	if res.Kind != "LabReplayDivergence" || res.Status != 2 {
		t.Fatalf("replay with an impossible pick: exit=%d kind=%q, want LabReplayDivergence", res.Status, res.Kind)
	}
}
