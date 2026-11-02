// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/importer"
	"cmd/compile/internal/objw"
	"cmd/compile/internal/sgoadmit"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/types2"
	"cmd/internal/obj"
	"cmd/internal/objabi"
)

// S-Go identity of compiled packages (SGO-IMP-002).
//
// Every package compiled by the S-Go compiler records, in its export data, the identity line
//
//	sgo1 <spec version> <admission digest> <ABI> <configuration> <contract digest>
//
// and, in its object file, the symbol go:sgoid.<package path> containing that line followed by one
// line "import <path> <contract digest>" per direct import, as seen when it was compiled. The
// importing compiler verifies the identity of every package it imports; the linker verifies every
// linked package and that each recorded import contract equals the contract of the package
// actually linked (cmd/link/internal/ld/sgoid.go). The contract digest is the SHA-256 of the
// package's exported declarations and their types.

// sgoContract computes the contract digest of a type-checked package: every exported
// package-level object and every exported method of an exported named type, as types2 prints it
// with full package paths, in name order.
func sgoContract(pkg *types2.Package) string {
	qf := func(p *types2.Package) string { return p.Path() }
	var lines []string
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		lines = append(lines, types2.ObjectString(obj, qf))
		if tn, ok := obj.(*types2.TypeName); ok {
			if named, ok := types2.Unalias(tn.Type()).(*types2.Named); ok {
				for i := 0; i < named.NumMethods(); i++ {
					if m := named.Method(i); m.Exported() {
						lines = append(lines, types2.ObjectString(m, qf))
					}
				}
			}
		}
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:16])
}

func sgoIdentityLine(contract string) string {
	return fmt.Sprintf("sgo1 %s %s %s %s %s", sgoadmit.SpecVersion, sgoadmit.AdmissionDigest, objabi.SGoABI, sgoConfig(), contract)
}

// sgoConfig is the execution configuration of the package being compiled: lab objects (step
// instrumentation, D43) and production objects never mix (D41, SGO-IMP-002). Trusted target code
// is not instrumented and is the same in both configurations: "tcb".
// sgoImportConfig is the configuration the S-Go packages this package imports were compiled for:
// a generated test main imports packages compiled as for any production program (D69).
func sgoImportConfig() string {
	if c := sgoConfig(); c != "testmain" {
		return c
	}
	return "prod"
}

func sgoConfig() string {
	switch {
	case !base.SGo:
		return "tcb"
	case base.Flag.SGoLab:
		return "lab"
	case base.Flag.SGoTestMain:
		return "testmain" // linked only as the main package of a test binary (D69)
	}
	return "prod"
}

// sgoCheckImports verifies the identity of every direct import and returns their contracts.
// Mismatches are reported at the import declaration.
func sgoCheckImports(m posMap, noders []*noder, pkg *types2.Package) map[string]string {
	importPos := map[string]syntax.Pos{}
	for _, p := range noders {
		for _, d := range p.file.DeclList {
			if id, ok := d.(*syntax.ImportDecl); ok && id.Path != nil {
				path := strings.Trim(id.Path.Value, "\"`")
				if _, seen := importPos[path]; !seen {
					importPos[path] = id.Path.Pos()
				}
			}
		}
	}
	want := fmt.Sprintf("sgo1 %s %s %s ", sgoadmit.SpecVersion, sgoadmit.AdmissionDigest, objabi.SGoABI)
	contracts := map[string]string{}
	for _, imp := range pkg.Imports() {
		path := imp.Path()
		if path == "unsafe" {
			continue
		}
		pos := m.makeXPos(importPos[path])
		line, ok := importer.SGoIdentities[path]
		switch {
		case !ok:
			base.ErrorfAt(pos, 0, "SGO_E_FOREIGN_OBJECT: package %s has no S-Go identity (SGO-IMP-002)", path)
		case !strings.HasPrefix(line, want):
			base.ErrorfAt(pos, 0, "SGO_E_SPEC_MISMATCH: package %s was compiled for %q, this compiler is %q (SGO-IMP-002)",
				path, strings.TrimSpace(line[:strings.LastIndex(line, " ")]), strings.TrimSpace(want))
		default:
			config, contract, _ := strings.Cut(line[len(want):], " ")
			if config == "tcb" && !sgoadmit.IsTCBPackage(path) {
				base.ErrorfAt(pos, 0, "SGO_E_FOREIGN_OBJECT: package %s claims trusted identity but is not a TCB package (SGO-TCB-002)", path)
				continue
			} else if config != "tcb" && config != sgoImportConfig() {
				base.ErrorfAt(pos, 0, "SGO_E_SPEC_MISMATCH: package %s was compiled for the %s configuration, this package for %s (D41, SGO-IMP-002)",
					path, config, sgoImportConfig())
				continue
			}
			contracts[path] = contract
		}
	}
	return contracts
}

// sgoEmitIdentity writes the go:sgoid.<path> symbol into the object file. It is unreferenced, so
// the linker's dead-code pass drops it from the program after verifying it.
func sgoEmitIdentity(path, contract string, imports map[string]string) {
	var b strings.Builder
	b.WriteString(sgoIdentityLine(contract))
	b.WriteByte('\n')
	var paths []string
	for p := range imports {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		fmt.Fprintf(&b, "import %s %s\n", p, imports[p])
	}
	data := b.String()
	s := base.Ctxt.Lookup("go:sgoid." + path)
	s.WriteString(base.Ctxt, 0, len(data), data)
	objw.Global(s, int32(len(data)), obj.RODATA|obj.NOPTR)
}
