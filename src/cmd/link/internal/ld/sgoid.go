// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import (
	"internal/buildcfg"
	"internal/platform"
	"slices"
	"sort"
	"strings"

	"cmd/internal/objabi"
)

// sgoCheckConfig refuses link configurations that the S-Go toolchain does not build (S01, S15;
// SGO-TOOL-001, SGO-BUILD-001). This linker links only S-Go programs (sgoCheckIdentity refuses
// any package without an S-Go identity), and an S-Go program is an executable, linked
// internally, with the linkname check on and without -X string overrides: those would change
// trusted runtime data or bypass a check that the driver never exposes. It runs after loadlib,
// when the link mode is known.
func (ctxt *Link) sgoCheckConfig() {
	// The linker's own string data (runtime.buildVersion and the like) is not an override; only
	// -X flags are counted.

	// An executable is PIE where that is the platform's default (darwin), and only there.
	pie := ctxt.BuildMode == BuildModePIE && platform.DefaultPIE(buildcfg.GOOS, buildcfg.GOARCH, false)
	if ctxt.BuildMode != BuildModeExe && !pie {
		Errorf("sgo:1:1: SGO_E_USAGE: build mode %s is not admitted; S-Go programs are executables (SGO-BUILD-001)", ctxt.BuildMode)
	}
	if ctxt.LinkMode != LinkInternal {
		Errorf("sgo:1:1: SGO_E_USAGE: link mode %s is not admitted; S-Go programs link internally (SGO-BUILD-001)", ctxt.LinkMode)
	}
	if sgoXFlags != 0 {
		Errorf("sgo:1:1: SGO_E_USAGE: -X string overrides are not admitted (SGO-TOOL-001)")
	}
	if !*flagCheckLinkname {
		Errorf("sgo:1:1: SGO_E_USAGE: -checklinkname=0 is not admitted (SGO-TOOL-001)")
	}
}

// sgoXFlags counts -X flags on the command line (main.go).
var sgoXFlags int

// sgoSetDrainBudget handles -sgodrainbudget=ms: it sets runtime.sgoDrainBudget, which the runtime
// reads when the program starts (SGO-DRAIN-002). Only positive decimal integers are accepted, as
// the driver writes them.
func sgoSetDrainBudget(ctxt *Link, s string) {
	ok := s != "" && len(s) <= 12 && s[0] != '0'
	for _, c := range s {
		ok = ok && c >= '0' && c <= '9'
	}
	if !ok {
		Exitf("sgo:1:1: SGO_E_USAGE: -sgodrainbudget must be a positive number of milliseconds, not %q (SGO-DRAIN-002)", s)
	}
	addstrdata1(ctxt, "runtime.sgoDrainBudget="+s)
}

// sgoGrantKinds are the effect kinds a grant may name (SGO-AUTH-003), in the runtime's order.
var sgoGrantKinds = []string{"clock", "wallclock", "random", "entropy", "simstore"}

// sgoSetGrants handles -sgogrants=kinds: it sets runtime.sgoGrants, the root task's ceiling
// (SGO-AUTH-002). Only known kinds, each once, in the driver's canonical order are accepted.
func sgoSetGrants(ctxt *Link, s string) {
	next := 0
	for _, k := range strings.Split(s, ",") {
		i := slices.Index(sgoGrantKinds, k)
		if i < next {
			Exitf("sgo:1:1: SGO_E_USAGE: -sgogrants must list known effect kinds once each in canonical order, not %q (SGO-AUTH-002)", s)
		}
		next = i + 1
	}
	addstrdata1(ctxt, "runtime.sgoGrants="+s)
}

// sgoCheckIdentity verifies the S-Go identity of every linked package (SGO-IMP-002). Each package
// compiled by the S-Go compiler defines go:sgoid.<path>: the line
//
//	sgo1 <spec version> <admission digest> <ABI> <contract digest>
//
// followed by "import <path> <contract digest>" for each direct import as compiled
// (cmd/compile/internal/noder/sgo_identity.go). The linker requires every package to have one,
// all of them to agree on spec, admission digest, and ABI, and every recorded import contract to
// equal the contract of the package actually linked. It runs before dead-code elimination, which
// then drops the unreferenced identity symbols from the program.
func (ctxt *Link) sgoCheckIdentity() {
	ldr := ctxt.loader
	type ident struct {
		head, contract string
		imports        map[string]string
	}
	ids := map[string]*ident{}
	var pkgs []string
	for _, lib := range ctxt.Library {
		pkgs = append(pkgs, lib.Pkg)
		s := ldr.Lookup("go:sgoid."+lib.Pkg, 0)
		if s == 0 {
			Errorf("sgo:1:1: SGO_E_FOREIGN_OBJECT: package %s (%s) has no S-Go identity", lib.Pkg, lib.File)
			continue
		}
		lines := strings.Split(strings.TrimRight(string(ldr.Data(s)), "\n"), "\n")
		f := strings.Fields(lines[0])
		if len(f) != 5 || f[0] != "sgo1" {
			Errorf("sgo:1:1: SGO_E_FOREIGN_OBJECT: package %s has a malformed S-Go identity %q", lib.Pkg, lines[0])
			continue
		}
		id := &ident{head: strings.Join(f[:4], " "), contract: f[4], imports: map[string]string{}}
		for _, l := range lines[1:] {
			if g := strings.Fields(l); len(g) == 3 && g[0] == "import" {
				id.imports[g[1]] = g[2]
			}
		}
		ids[lib.Pkg] = id
	}
	sort.Strings(pkgs)
	// Every package must agree with the main package (or, without one, the first by path).
	ref := ids["main"]
	if ref == nil {
		for _, p := range pkgs {
			if ref = ids[p]; ref != nil {
				break
			}
		}
	}
	if ref == nil {
		return
	}
	if f := strings.Fields(ref.head); f[3] != objabi.SGoABI {
		Errorf("sgo:1:1: SGO_E_SPEC_MISMATCH: the program was compiled for ABI %s, this linker is %s", f[3], objabi.SGoABI)
	}
	for _, p := range pkgs {
		id := ids[p]
		if id == nil {
			continue
		}
		if id.head != ref.head {
			Errorf("sgo:1:1: SGO_E_SPEC_MISMATCH: package %s was compiled as %q, the program as %q", p, id.head, ref.head)
		}
		var deps []string
		for d := range id.imports {
			deps = append(deps, d)
		}
		sort.Strings(deps)
		for _, d := range deps {
			dep := ids[d]
			if dep != nil && dep.contract != id.imports[d] {
				Errorf("sgo:1:1: SGO_E_STALE_CONTRACT: package %s was compiled against %s contract %s, but the linked %s has contract %s; rebuild %s",
					p, d, id.imports[d], d, dep.contract, p)
			}
		}
	}
}
