// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import (
	"slices"
	"testing"
)

// The I01 confinement scenarios with their outcome sets over every interleaving of the
// resolution with another process's renames (W09, D53). The runtime's resolver is checked
// against the same scenarios on a real file system.
var ResolveScenarios = map[string]struct {
	Sc   RScenario
	Want []string
}{
	"plain":            {RScenario{Entries: []string{"d grant", "d grant/a", "f grant/a/x"}, Path: "a/x"}, []string{"ok grant/a/x"}},
	"missing":          {RScenario{Entries: []string{"d grant"}, Path: "a/x"}, []string{"NotFound"}},
	"file-as-dir":      {RScenario{Entries: []string{"d grant", "f grant/a"}, Path: "a/x"}, []string{"NotFound"}},
	"dotdot":           {RScenario{Entries: []string{"d grant", "d out", "f out/x"}, Path: "../out/x"}, []string{"PathRejected"}},
	"inner-dotdot":     {RScenario{Entries: []string{"d grant", "d grant/a", "f grant/a/x"}, Path: "a/../a/x"}, []string{"PathRejected"}},
	"absolute":         {RScenario{Entries: []string{"d grant"}, Path: "/etc/passwd"}, []string{"PathRejected"}},
	"dot":              {RScenario{Entries: []string{"d grant", "f grant/x"}, Path: "./x"}, []string{"PathRejected"}},
	"empty-name":       {RScenario{Entries: []string{"d grant", "d grant/a", "f grant/a/x"}, Path: "a//x"}, []string{"PathRejected"}},
	"prefix-confusion": {RScenario{Entries: []string{"d grant", "d grantx", "f grantx/x"}, Path: "x"}, []string{"NotFound"}},
	"symlink-dir":      {RScenario{Entries: []string{"d grant", "l grant/s", "d out", "f out/x"}, Path: "s/x"}, []string{"PathRejected"}},
	"symlink-last":     {RScenario{Entries: []string{"d grant", "l grant/s"}, Path: "s"}, []string{"PathRejected"}},
	// Another process moves a directory out of the grant during the resolution: the result is
	// either the file while it was inside, or NotFound; never the moved-out file.
	"moved-out": {RScenario{Entries: []string{"d grant", "d grant/a", "f grant/a/x", "d out"}, Path: "a/x",
		Renames: []string{"grant/a out/a"}}, []string{"NotFound", "ok grant/a/x"}},
	// It moves a directory out and another in under the same name: whichever is inside at
	// verification, or a retry that finds the new one.
	"swap": {RScenario{Entries: []string{"d grant", "d grant/a", "f grant/a/x", "d out", "d out/b", "f out/b/x"}, Path: "a/x",
		Renames: []string{"grant/a out/a", "out/b grant/a"}}, []string{"NotFound", "ok grant/a/x", "ok out/b/x"}},
	// It swaps a symlink in for a directory: refused once seen, never followed.
	"symlink-swapped-in": {RScenario{Entries: []string{"d grant", "d grant/a", "f grant/a/x", "d out", "l out/l"}, Path: "a/x",
		Renames: []string{"grant/a out/a", "out/l grant/a"}}, []string{"NotFound", "PathRejected", "ok grant/a/x"}},
}

func TestResolveScenarios(t *testing.T) {
	for name, c := range ResolveScenarios {
		got, err := ExploreResolve(c.Sc)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !slices.Equal(got, c.Want) {
			t.Errorf("%s: outcomes %q, want %q", name, got, c.Want)
		}
	}
}

// Without the verification step the moved-out scenario resolves a file outside the grant: the
// check that guards the property fires.
func TestResolveWithoutVerifyEscapes(t *testing.T) {
	defer func() { ioSkipVerify = false }()
	ioSkipVerify = true
	if _, err := ExploreResolve(ResolveScenarios["moved-out"].Sc); err == nil {
		t.Fatal("an unverified resolution did not escape the grant")
	}
}
