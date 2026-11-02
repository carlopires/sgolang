// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

// Identity configuration rules of the S-Go link check. Standard-library imports only, so the host
// test lane can test this file in isolation (test-host-pkg.sh --only, W12 R1).

import (
	"fmt"
	"strings"
)

// sgoIdentConfigErr checks a linked package's identity configuration against the program being
// linked (want: "prod" or "lab"; test: a test binary). It returns "" or the diagnostic.
//   - S-Go code is compiled for one execution configuration, the one being linked (D41);
//   - trusted target code ("tcb") is the same in every configuration, and only a TCB package path
//     may carry it (SGO-TCB-002, W12 R1);
//   - the main package of a test binary is the one sgo test generates (-sgotestmain, D69), and a
//     testmain package is never linked into any other program.
func sgoIdentConfigErr(pkg, config, want string, test bool) string {
	switch {
	case config == "tcb" && !sgoIsTCBPackage(pkg):
		return fmt.Sprintf("SGO_E_FOREIGN_OBJECT: package %s claims trusted identity but is not a TCB package (SGO-TCB-002)", pkg)
	case pkg == "main" && test:
		if config != "testmain" {
			return fmt.Sprintf("SGO_E_SPEC_MISMATCH: the main package of a test binary must be the one sgo test generates, not one compiled for %s (D69)", config)
		}
	case config == "testmain":
		return fmt.Sprintf("SGO_E_SPEC_MISMATCH: package %s is a generated test main; it is linked only into a test binary (D69)", pkg)
	case config != "tcb" && config != want:
		return fmt.Sprintf("SGO_E_SPEC_MISMATCH: package %s was compiled for the %s configuration, the program is linked for %s (D41, SGO-IMP-002)", pkg, config, want)
	}
	return ""
}

func sgoIsTCBPackage(pkg string) bool {
	for _, p := range strings.Fields(sgoTCBPackageList) {
		if p == pkg {
			return true
		}
	}
	return false
}

// sgoLinkFlags are the link flags the sgo driver writes (build.go link). Every other flag is
// refused (D77, W12 R3): refuse-by-default, so a flag added upstream stays refused until it is
// added here deliberately.
var sgoLinkFlags = []string{"o", "importcfg", "buildmode", "buildid", "sgodrainbudget", "sgogrants", "sgoiogrants", "sgolab", "sgotest"}

// sgoLinkFlagErr returns the diagnostic for a set flag that is not admitted, or "".
func sgoLinkFlagErr(name string) string {
	for _, a := range sgoLinkFlags {
		if name == a {
			return ""
		}
	}
	return fmt.Sprintf("SGO_E_USAGE: link flag -%s is not admitted (SGO-TOOL-001)", name)
}

// sgoBuildIDErr checks the -buildid the driver writes (SGO-BUILD-001): it is required, and it must
// agree with the link itself: the configuration (lab, test), the drain budget and root grants
// passed to this link, and the spec version and admission digest of the linked packages. It
// returns "" or the diagnostic.
func sgoBuildIDErr(id, spec, admission, drain, grants string, lab, test bool) string {
	if id == "" {
		return "SGO_E_USAGE: -buildid is required: every S-Go binary records its build metadata (SGO-BUILD-001)"
	}
	rest, ok := strings.CutPrefix(id, "sgo1/")
	if !ok {
		return fmt.Sprintf("SGO_E_USAGE: -buildid %q is not S-Go build metadata (SGO-BUILD-001)", id)
	}
	kv := map[string]string{}
	for _, seg := range strings.Split(rest, "/") {
		if k, v, ok := strings.Cut(seg, "="); ok {
			if _, dup := kv[k]; dup {
				return fmt.Sprintf("SGO_E_USAGE: -buildid repeats %s (SGO-BUILD-001)", k)
			}
			kv[k] = v
		}
	}
	config := ""
	switch {
	case lab && test:
		config = "lab-test"
	case lab:
		config = "lab"
	case test:
		config = "test"
	}
	if drain == "" {
		drain = "none"
	}
	if grants == "" {
		grants = "none"
	}
	for _, c := range []struct{ key, want string }{
		{"spec", spec}, {"admission", admission}, {"config", config}, {"drain", drain}, {"grants", grants},
	} {
		if kv[c.key] != c.want {
			return fmt.Sprintf("SGO_E_USAGE: -buildid records %s=%q, the link is %q (SGO-BUILD-001)", c.key, kv[c.key], c.want)
		}
	}
	for _, k := range []string{"target", "toolchain", "upstream"} {
		if kv[k] == "" {
			return fmt.Sprintf("SGO_E_USAGE: -buildid has no %s (SGO-BUILD-001)", k)
		}
	}
	return ""
}
