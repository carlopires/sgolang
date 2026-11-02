// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import (
	"strings"
	"testing"
)

// TestSGoIdentConfigErr: the linker's identity-configuration rules (D41, D69, SGO-TCB-002 / W12 R1).
func TestSGoIdentConfigErr(t *testing.T) {
	for _, tc := range []struct {
		pkg, config, want string
		test              bool
		code              string // "" = accepted
	}{
		{"runtime", "tcb", "prod", false, ""},
		{"internal/stringslite", "tcb", "lab", false, ""},
		{"example.com/p2/sl", "tcb", "prod", false, "SGO_E_FOREIGN_OBJECT"},
		{"main", "tcb", "prod", false, "SGO_E_FOREIGN_OBJECT"},
		{"main", "tcb", "prod", true, "SGO_E_FOREIGN_OBJECT"},
		{"example.com/m/lib", "prod", "prod", false, ""},
		{"example.com/m/lib", "lab", "prod", false, "SGO_E_SPEC_MISMATCH"},
		{"example.com/m/lib", "prod", "lab", false, "SGO_E_SPEC_MISMATCH"},
		{"main", "testmain", "prod", true, ""},
		{"main", "prod", "prod", true, "SGO_E_SPEC_MISMATCH"},
		{"example.com/m/lib", "testmain", "prod", true, "SGO_E_SPEC_MISMATCH"},
	} {
		got := sgoIdentConfigErr(tc.pkg, tc.config, tc.want, tc.test)
		if tc.code == "" && got != "" || tc.code != "" && !strings.HasPrefix(got, tc.code) {
			t.Errorf("sgoIdentConfigErr(%q, %q, %q, %v) = %q, want %s", tc.pkg, tc.config, tc.want, tc.test, got, tc.code)
		}
	}
}

// TestSGoLinkFlagsAndBuildID: refuse-by-default link flags and the required, consistent -buildid
// (D77, SGO-BUILD-001, W12 R3).
func TestSGoLinkFlagsAndBuildID(t *testing.T) {
	for _, f := range []string{"o", "importcfg", "buildmode", "buildid", "sgodrainbudget", "sgogrants", "sgoiogrants", "sgolab", "sgotest"} {
		if msg := sgoLinkFlagErr(f); msg != "" {
			t.Errorf("driver flag -%s refused: %s", f, msg)
		}
	}
	for _, f := range []string{"f", "s", "w", "strictdups", "randlayout", "R", "k", "installsuffix", "debugnosplit", "race", "X", "B"} {
		if msg := sgoLinkFlagErr(f); !strings.HasPrefix(msg, "SGO_E_USAGE") {
			t.Errorf("flag -%s admitted", f)
		}
	}
	id := func(mid string) string {
		return "sgo1/spec=0.6.0/admission=abc" + mid + "/target=linux-amd64/toolchain=t/upstream=u"
	}
	for _, tc := range []struct {
		name, id, drain, grants string
		lab, test               bool
		ok                      bool
	}{
		{"production", id("/drain=none/grants=none"), "", "", false, false, true},
		{"grants and io", id("/drain=500/grants=clock,random/io=deadbeef"), "500", "clock,random", false, false, true},
		{"lab", id("/config=lab/drain=none/grants=none"), "", "", true, false, true},
		{"lab test", id("/config=lab-test/drain=none/grants=none"), "", "", true, true, true},
		{"missing", "", "", "", false, false, false},
		{"not S-Go", "abc", "", "", false, false, false},
		{"lab binary claiming production", id("/drain=none/grants=none"), "", "", true, false, false},
		{"production claiming lab", id("/config=lab/drain=none/grants=none"), "", "", false, false, false},
		{"other spec", "sgo1/spec=9.9.9/admission=abc/drain=none/grants=none/target=a/toolchain=t/upstream=u", "", "", false, false, false},
		{"drain mismatch", id("/drain=none/grants=none"), "500", "", false, false, false},
		{"grants mismatch", id("/drain=none/grants=clock"), "", "", false, false, false},
		{"repeated key", id("/drain=none/grants=none/drain=none"), "", "", false, false, false},
		{"no target", "sgo1/spec=0.6.0/admission=abc/drain=none/grants=none/toolchain=t/upstream=u", "", "", false, false, false},
	} {
		msg := sgoBuildIDErr(tc.id, "0.6.0", "abc", tc.drain, tc.grants, tc.lab, tc.test)
		if (msg == "") != tc.ok {
			t.Errorf("%s: sgoBuildIDErr = %q, want ok=%v", tc.name, msg, tc.ok)
		}
	}
}
