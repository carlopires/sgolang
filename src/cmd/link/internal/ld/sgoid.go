// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import (
	"sort"
	"strings"

	"cmd/internal/objabi"
)

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
