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
