// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import (
	"fmt"
	"maps"
	"slices"
	"testing"
)

// Channel kinds used by the scenarios: 0 carries Ref values, 1 Frozen, 2 Val.
var shareChans = []SKind{SRef, SFrozen, SVal}

func alloc(v string, k SKind) SStmt        { return SStmt{Op: SAlloc, V: v, Kind: k} }
func send(v string, ch int, m SMode) SStmt { return SStmt{Op: SSend, V: v, Ch: ch, Mode: m} }
func recv(v string, ch int) SStmt          { return SStmt{Op: SRecv, V: v, Ch: ch} }
func op(o SOp, v, w string) SStmt          { return SStmt{Op: o, V: v, W: w} }
func spawn(body int, caps ...string) SStmt { return SStmt{Op: SSpawn, Body: body, Caps: caps} }

// The crossing patterns of the W13 plan's five programs (§3), and the programs the rule must
// reject, with their diagnostics.
var ShareScenarios = map[string]struct {
	Bodies [][]SStmt
	Want   string // "" accepted
}{
	// P1: a worker builds a nested partial result and hands it over by copy, then by move.
	"p1-copy": {[][]SStmt{
		{spawn(1), recv("t", 0), op(SWriteChild, "t", "")},
		{alloc("part", SRef), op(SChild, "part", ""), op(SWriteChild, "part", ""), send("part", 0, SCopy), op(SWrite, "part", "")},
	}, ""},
	"p1-move": {[][]SStmt{
		{spawn(1), recv("t", 0), op(SWriteChild, "t", "")},
		{alloc("part", SRef), op(SChild, "part", ""), send("part", 0, SMove)},
	}, ""},
	// P2: a store task keeps its map and replies with copies; the session mutates its copy.
	"p2-reply-copy": {[][]SStmt{
		{spawn(1), alloc("req", SVal), send("req", 2, SPlain), recv("r", 0), op(SWrite, "r", "")},
		{alloc("store", SRef), recv("q", 2), op(SWrite, "store", ""), send("store", 0, SCopy), op(SWrite, "store", "")},
	}, ""},
	// P3: a dispatcher owns its queue and hands out reference-free jobs.
	"p3-value-jobs": {[][]SStmt{
		{alloc("q", SRef), alloc("job", SVal), spawn(1, "job"), op(SWrite, "job", ""), op(SWrite, "q", "")},
		{op(SWrite, "job", "")},
	}, ""},
	// P4: single-task aliasing is ordinary Go.
	"p4-local-alias": {[][]SStmt{
		{alloc("env", SRef), op(SAlias, "p", "env"), op(SStore, "env", "p"), op(SWriteChild, "p", "")},
	}, ""},
	// P5: one frozen table read by two workers.
	"p5-frozen-table": {[][]SStmt{
		{alloc("t", SRef), op(SChild, "t", ""), op(SFreeze, "f", "t"), spawn(1, "f"), spawn(1, "f"), op(SWrite, "t", "")},
		{op(SReadChild, "f", ""), op(SRead, "f", "")},
	}, ""},
	"frozen-over-channel": {[][]SStmt{
		{alloc("t", SRef), op(SFreeze, "f", "t"), spawn(1), send("f", 1, SPlain), op(SRead, "f", "")},
		{recv("g", 1), op(SRead, "g", "")},
	}, ""},

	"reject-plain-ref-send": {[][]SStmt{{alloc("a", SRef), send("a", 0, SPlain)}}, "SGO_E_UNCOPIED_SEND"},
	"reject-ref-capture": {[][]SStmt{
		{alloc("a", SRef), spawn(1, "a"), op(SWrite, "a", "")},
		{op(SWrite, "a", "")},
	}, "SGO_E_TASK_LOCAL_CAPTURE"},
	"reject-use-after-move":  {[][]SStmt{{alloc("a", SRef), send("a", 0, SMove), op(SWrite, "a", "")}}, "SGO_E_USE_AFTER_MOVE"},
	"reject-move-aliased":    {[][]SStmt{{alloc("a", SRef), op(SAlias, "b", "a"), send("a", 0, SMove), op(SWrite, "b", "")}}, "SGO_E_MOVE_ALIASED"},
	"reject-move-stored-in":  {[][]SStmt{{alloc("a", SRef), alloc("b", SRef), op(SStore, "a", "b"), send("a", 0, SMove), op(SWrite, "b", "")}}, "SGO_E_MOVE_ALIASED"},
	"reject-frozen-write":    {[][]SStmt{{alloc("a", SRef), op(SFreeze, "f", "a"), op(SWrite, "f", "")}}, "SGO_E_FROZEN_WRITE"},
	"reject-frozen-child-wr": {[][]SStmt{{alloc("a", SRef), op(SChild, "a", ""), op(SFreeze, "f", "a"), op(SWriteChild, "f", "")}}, "SGO_E_FROZEN_WRITE"},

	// Error values (W13 review V2): a value type boxed as an error is shared freely; a task-local
	// one is refused, because its Error method could write what the shared box refers to.
	"error-value-shared": {[][]SStmt{
		{alloc("v", SVal), op(SToErr, "err", "v"), spawn(1, "err"), op(SWrite, "err", ""), op(SWrite, "v", "")},
		{op(SWriteChild, "err", ""), op(SRead, "err", "")},
	}, ""},
	"reject-ref-error": {[][]SStmt{
		{alloc("s", SRef), op(SToErr, "err", "s"), spawn(1, "err"), op(SWrite, "s", "")},
		{op(SWriteChild, "err", "")},
	}, "SGO_E_TASK_LOCAL_CAPTURE"},
}

func TestShareScenarios(t *testing.T) {
	for name, c := range ShareScenarios {
		p := SProgram{Bodies: c.Bodies, Chans: shareChans}
		if got := CheckShare(p); got != c.Want {
			t.Errorf("%s: CheckShare = %q, want %q", name, got, c.Want)
			continue
		}
		if c.Want != "" {
			continue
		}
		if res := ExploreShare(p); res.Err != nil || res.Bounded {
			t.Errorf("%s: accepted program violates ownership or exploration incomplete: %+v", name, res)
		}
	}
}

const (
	shareSeed     = 13
	sharePrograms = 20000
)

// Soundness: every generated program the checker accepts keeps the ownership invariant on every
// interleaving.
func TestShareSoundness(t *testing.T) {
	acc, bounded, bad, err := ShareSoundness(shareSeed, sharePrograms)
	if err != nil {
		t.Fatalf("accepted program violates ownership: %v\n%+v", err, *bad)
	}
	if bounded != 0 {
		t.Fatalf("%d explorations reached the state bound: soundness is not established for them", bounded)
	}
	if acc < sharePrograms/20 {
		t.Fatalf("only %d of %d generated programs accepted: the generator is too weak", acc, sharePrograms)
	}
	t.Logf("%d of %d generated programs accepted; all explored without a violation", acc, sharePrograms)
}

// Soundness with error values: GenShareErr adds error(W) for W of every kind, a channel of errors,
// and reads and writes through them (W13 review V2).
func TestShareErrSoundness(t *testing.T) {
	acc, bounded, bad, err := ShareErrSoundness(shareSeed, sharePrograms)
	if err != nil {
		t.Fatalf("accepted program violates ownership: %v\n%+v", err, *bad)
	}
	if bounded != 0 {
		t.Fatalf("%d explorations reached the state bound: soundness is not established for them", bounded)
	}
	if acc < sharePrograms/20 {
		t.Fatalf("only %d of %d generated programs accepted: the generator is too weak", acc, sharePrograms)
	}
	t.Logf("%d of %d generated programs with error values accepted; all explored without a violation", acc, sharePrograms)
}

// The generated programs, not only the hand-written scenario, reach the error route: with error(v)
// of a Ref accepted, GenShareErr's programs alone show a violation.
func TestShareErrGeneratorFindsRefError(t *testing.T) {
	shareAllowRefError = true
	defer func() { shareAllowRefError = false }()
	_, _, bad, err := ShareErrSoundness(shareSeed, 5*sharePrograms)
	if err == nil {
		t.Fatal("with error(Ref) accepted, no generated program violates ownership: the generator does not reach the route")
	}
	t.Logf("found: %v\n%+v", err, *bad)
}

// Each relaxation of the rule (and a shallow sgo.Copy) admits a program that the explorer shows
// violating ownership: every part of the rule is needed.
func TestShareMutations(t *testing.T) {
	for _, m := range []struct {
		name string
		flag *bool
	}{
		{"plain-ref-send", &shareAllowPlainRef},
		{"ref-capture", &shareAllowRefCapture},
		{"aliased-move", &shareAllowAliasedMove},
		{"frozen-write", &shareAllowFrozenWrite},
		{"shallow-copy", &shareShallowCopy},
		{"ref-error", &shareAllowRefError},
	} {
		*m.flag = true
		by, err := "TestShareScenarios", shareScenarioViolation()
		if err == nil {
			by = "TestShareSoundness"
			_, _, _, err = ShareSoundness(shareSeed, 5*sharePrograms)
		}
		if err == nil {
			by = "TestShareErrSoundness"
			_, _, _, err = ShareErrSoundness(shareSeed, 5*sharePrograms)
		}
		*m.flag = false
		if err == nil {
			t.Errorf("mutation %s not detected", m.name)
			continue
		}
		t.Logf("mutation %s detected by %s: %v", m.name, by, err)
	}
}

// shareScenarioViolation explores every scenario the (possibly mutated) checker accepts and returns
// the first ownership violation.
func shareScenarioViolation() error {
	for _, name := range slices.Sorted(maps.Keys(ShareScenarios)) {
		p := SProgram{Bodies: ShareScenarios[name].Bodies, Chans: shareChans}
		if CheckShare(p) != "" {
			continue
		}
		if res := ExploreShare(p); res.Err != nil {
			return fmt.Errorf("%s: %v", name, res.Err)
		}
	}
	return nil
}
