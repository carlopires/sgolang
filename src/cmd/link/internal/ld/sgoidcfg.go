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
