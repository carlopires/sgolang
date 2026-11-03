// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// labTraceName is the default trace file of a main package: <dir name>.lab.jsonl in the current
// directory.
func labTraceName(dir, suffix string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return filepath.Base(abs) + suffix
}

func labSetup(f *labFlags, nargs int, what string) (string, *labManifest, func(), error) {
	if len(f.args) != nargs {
		return "", nil, nil, usage("sgo lab %s", what)
	}
	tmp, err := os.MkdirTemp("", "sgo-lab-build-")
	if err != nil {
		return "", nil, nil, err
	}
	cleanup := func() { os.RemoveAll(tmp) }
	exe, id, err := labBuild(f.args[nargs-1], tmp)
	if err != nil {
		cleanup()
		return "", nil, nil, err
	}
	net, err := readLabNet(f.args[nargs-1])
	if err != nil {
		cleanup()
		return "", nil, nil, err
	}
	m := &labManifest{Format: labFormat, Build: id, Seed: f.seed, Policy: f.policy, K: f.k, Quota: f.quota, Epoch: f.epoch, Net: net, program: exe}
	return f.args[nargs-1], m, cleanup, nil
}

func hangErr(f *labFlags) error {
	return labErr("SGO_E_LAB_HANG", "the run did not reach its end within %s and was killed: a host hang, not a lab outcome (D47)", f.timeout)
}

func cmdLabRun(args []string) error {
	f, err := parseLabFlags("run", args)
	if err != nil {
		return err
	}
	dir, m, cleanup, err := labSetup(f, 1, "run takes exactly one main package")
	if err != nil {
		return err
	}
	defer cleanup()
	trace := f.trace
	if trace == "" {
		trace = labTraceName(dir, ".lab.jsonl")
	}
	r, err := labExec(m, f.timeout, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	if err := writeLabTrace(trace, m, r); err != nil {
		return err
	}
	if r.hang {
		return hangErr(f)
	}
	fmt.Fprintf(os.Stderr, "sgo lab: seed %d, %d events, exit %d; trace %s\n", m.Seed, len(r.lines), r.status, trace)
	cleanup()
	os.Exit(r.status) // like sgo run, the program's status (SGO-TOOL-001)
	return nil
}

// labPicks are the choices a replay takes from a trace: the pick of every choose and select event
// whose set has two or more elements (replayPick, src/runtime/sgo_lab_sched.go).
func labPicks(lines []string) ([]int, error) {
	var picks []int
	for i, l := range lines {
		var e struct {
			E     string
			Ready []int
			Arms  []int
			Pick  int
			Arm   int
		}
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			return nil, labErr("SGO_E_LAB_TRACE", "event %d is not a canonical event: %s", i+1, l)
		}
		switch {
		case e.E == "choose" && len(e.Ready) > 1:
			picks = append(picks, e.Pick)
		case e.E == "select" && len(e.Arms) > 1:
			picks = append(picks, e.Arm)
		}
	}
	return picks, nil
}

func cmdLabReplay(args []string) error {
	f, err := parseLabFlags("replay", args)
	if err != nil {
		return err
	}
	if len(f.args) != 2 {
		return usage("sgo lab replay takes a trace and its main package")
	}
	rec, want, err := readLabTrace(f.args[0])
	if err != nil {
		return err
	}
	_, m, cleanup, err := labSetup(f, 2, "replay takes a trace and its main package")
	if err != nil {
		return err
	}
	defer cleanup()
	// D46: an incompatible manifest is rejected before running.
	if m.Build != rec.Build {
		return labErr("SGO_E_LAB_MANIFEST", "the trace was recorded by build %q; this build is %q", rec.Build, m.Build)
	}
	if !slices.Equal(m.Net, rec.Net) {
		return labErr("SGO_E_LAB_MANIFEST", "the trace was recorded with a different lab.net network table (D146)")
	}
	picks, err := labPicks(want)
	if err != nil {
		return err
	}
	m.Seed, m.Policy, m.K, m.Quota, m.Epoch, m.Replay, m.Picks = rec.Seed, rec.Policy, rec.K, rec.Quota, rec.Epoch, true, picks
	r, err := labExec(m, f.timeout, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	if r.hang {
		return hangErr(f)
	}
	for i := 0; i < len(want) || i < len(r.lines); i++ {
		var w, g string
		if i < len(want) {
			w = want[i]
		}
		if i < len(r.lines) {
			g = r.lines[i]
		}
		if w != g {
			last := "none (the first event)"
			if i > 0 {
				last = want[i-1]
			}
			return labErr("SGO_E_LAB_DIVERGENCE", "replay diverges at event %d: the trace has %s, the run has %s; last matching event: %s (D46)",
				i+1, orNone(w), orNone(g), last)
		}
	}
	fmt.Fprintf(os.Stderr, "sgo lab: replay reproduced %d events, exit %d\n", len(r.lines), r.status)
	cleanup()
	os.Exit(r.status)
	return nil
}

func orNone(s string) string {
	if s == "" {
		return "no event"
	}
	return s
}

func cmdLabSweep(args []string) error {
	f, err := parseLabFlags("sweep", args)
	if err != nil {
		return err
	}
	if !f.haveSeeds {
		return usage("sgo lab sweep needs -seeds A..B")
	}
	var expect []byte
	if f.expect != "" {
		if expect, err = os.ReadFile(f.expect); err != nil {
			return usage("cannot read -expect file %s", f.expect)
		}
	}
	dir, m, cleanup, err := labSetup(f, 1, "sweep takes exactly one main package")
	if err != nil {
		return err
	}
	defer cleanup()
	save := f.save
	if save == "" {
		save = labTraceName(dir, ".lab-fail.jsonl")
	}
	type outcome struct {
		status         int
		stdout, record string
		first, count   uint64
		failed         bool
	}
	outcomes := map[string]*outcome{}
	var failures, runs uint64
	saved := false
	for seed := f.seed; ; seed++ {
		m.Seed = seed
		r, err := labExec(m, f.timeout, io.Discard, io.Discard)
		if err != nil {
			return err
		}
		runs++
		failed := r.hang || r.status != 0 || (f.expect != "" && string(r.stdout) != string(expect))
		key := fmt.Sprintf("%d\x00%s\x00%s\x00%v", r.status, r.stdout, r.stderr, r.hang)
		o := outcomes[key]
		if o == nil {
			o = &outcome{status: r.status, stdout: string(r.stdout), record: strings.TrimSpace(string(r.stderr)), first: seed, failed: failed}
			outcomes[key] = o
		}
		o.count++
		if failed {
			failures++
			if !saved {
				if err := writeLabTrace(save, m, r); err != nil {
					return err
				}
				saved = true
			}
		}
		if seed == f.seedTo {
			break
		}
	}
	var list []*outcome
	for _, o := range outcomes {
		list = append(list, o)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].first < list[j].first })
	for _, o := range list {
		state := "pass"
		if o.failed {
			state = "FAIL"
		}
		fmt.Printf("%s exit=%d runs=%d first-seed=%d stdout=%q", state, o.status, o.count, o.first, o.stdout)
		if o.record != "" {
			fmt.Printf(" record=%s", o.record)
		}
		fmt.Println()
	}
	fmt.Printf("sgo lab sweep: seeds %d..%d: %d runs, %d failed, %d distinct outcomes; passing runs show those schedules pass, not every schedule (D48)\n",
		f.seed, f.seedTo, runs, failures, len(list))
	if failures > 0 {
		fmt.Printf("sgo lab sweep: first failing trace saved to %s (sgo lab replay %s %s)\n", save, save, dir)
		cleanup()
		os.Exit(1)
	}
	return nil
}
