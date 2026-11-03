// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import (
	"flag"
	"fmt"
	"internal/buildcfg"
	"internal/platform"
	"net/netip"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
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
		Errorf("sgo:1:1: SGO_E_USAGE: link mode %s is not admitted; S-Go programs link internally (SGO-BUILD-001)", &ctxt.LinkMode)
	}
}

// sgoCheckFlags refuses, before anything is loaded or written, every flag the sgo driver does not
// write and every target setting other than the one it pins (D77, W12 R3): SGO_E_USAGE, exit
// status 2, no output. sgoCheckConfig checks the admitted flags' values later, once the link mode is
// known.
func sgoCheckFlags() {
	var bad []string
	flag.CommandLine.Visit(func(f *flag.Flag) {
		switch {
		case f.Name == "X":
			bad = append(bad, "SGO_E_USAGE: -X string overrides are not admitted (SGO-TOOL-001)")
		case f.Name == "checklinkname" && !*flagCheckLinkname:
			bad = append(bad, "SGO_E_USAGE: -checklinkname=0 is not admitted (SGO-TOOL-001)")
		default:
			if msg := sgoLinkFlagErr(f.Name); msg != "" {
				bad = append(bad, msg)
			}
		}
	})
	for _, e := range buildcfg.SGoTargetConfigErrs() {
		bad = append(bad, "SGO_E_USAGE: "+e)
	}
	for _, b := range bad {
		fmt.Fprintf(os.Stderr, "sgo:1:1: %s\n", b)
	}
	if len(bad) > 0 {
		os.Exit(2)
	}
}

// sgoXFlags counts -X flags on the command line (main.go).
var sgoXFlags int

// sgoDrainArg and sgoGrantsArg are the -sgodrainbudget and -sgogrants values, checked against the
// -buildid (sgoBuildIDErr).
var sgoDrainArg, sgoGrantsArg string

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
	sgoDrainArg = s
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
	sgoGrantsArg = s
	addstrdata1(ctxt, "runtime.sgoGrants="+s)
}

// sgoSetIOGrants handles -sgoiogrants=lines: it sets runtime.sgoIOGrants, the file and network
// grants (D52, D57). Only the driver's canonical form is accepted: sorted, unique lines
// "dir NAME ro|rw PATH", "connect IP:PORT|unix:PATH|HOST:PORT", "listen IP:PORT|unix:PATH".
func sgoSetIOGrants(ctxt *Link, s string) {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		f := strings.SplitN(l, " ", 4)
		ok := i == 0 || lines[i-1] < l
		switch {
		case !ok:
		case f[0] == "dir":
			ok = len(f) == 4 && sgoGrantName.MatchString(f[1]) && (f[2] == "ro" || f[2] == "rw") && f[3] != "" && !strings.ContainsAny(f[3], "\n\x00")
		case (f[0] == "connect" || f[0] == "listen") && len(f) == 2 && strings.HasPrefix(f[1], "unix:"):
			p := f[1][len("unix:"):]
			ok = p != "" && len(p) <= 103 && !strings.ContainsRune(p, 0)
		case f[0] == "connect" && len(f) == 2 && sgoHostPort.MatchString(f[1]) && strings.ContainsAny(f[1][:strings.LastIndexByte(f[1], ':')], "abcdefghijklmnopqrstuvwxyz-"):
			// A host name endpoint (D145), resolved through a Dialer at run time. A name has a
			// letter or '-'; all digits and dots is an IP literal, checked below.
			i := strings.LastIndexByte(f[1], ':')
			port, _ := strconv.Atoi(f[1][i+1:])
			ok = i <= 253 && port >= 1 && port <= 65535
		case f[0] == "connect" || f[0] == "listen":
			ap, err := netip.ParseAddrPort(f[len(f)-1])
			ok = len(f) == 2 && err == nil && ap.String() == f[1] && ap.Port() != 0 && !ap.Addr().Is4In6() && ap.Addr().Zone() == ""
		default:
			ok = false
		}
		if !ok {
			Exitf("sgo:1:1: SGO_E_USAGE: -sgoiogrants must be the driver's canonical grant lines, not %q (D52)", l)
		}
	}
	addstrdata1(ctxt, "runtime.sgoIOGrants="+s)
}

var sgoGrantName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// sgoHostPort matches NAME:PORT with NAME dot-separated labels of [a-z0-9-] (1 to 63 bytes, not
// starting or ending with '-') and PORT without leading zeros; the caller checks the rest.
var sgoHostPort = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*:[1-9][0-9]{0,4}$`)

// sgoCheckIdentity verifies the S-Go identity of every linked package (SGO-IMP-002). Each package
// compiled by the S-Go compiler defines go:sgoid.<path>: the line
//
//	sgo1 <spec version> <admission digest> <ABI> <configuration> <contract digest>
//
// followed by "import <path> <contract digest>" for each direct import as compiled
// (cmd/compile/internal/noder/sgo_identity.go). The linker requires every package to have one,
// all of them to agree on spec, admission digest, and ABI, and every recorded import contract to
// equal the contract of the package actually linked. It runs before dead-code elimination, which
// then drops the unreferenced identity symbols from the program.
func (ctxt *Link) sgoCheckIdentity() {
	ldr := ctxt.loader
	type ident struct {
		head, config, contract string
		imports                map[string]string
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
		if len(f) != 6 || f[0] != "sgo1" {
			Errorf("sgo:1:1: SGO_E_FOREIGN_OBJECT: package %s has a malformed S-Go identity %q", lib.Pkg, lines[0])
			continue
		}
		id := &ident{head: strings.Join(f[:4], " "), config: f[4], contract: f[5], imports: map[string]string{}}
		// S-Go code is compiled for one execution configuration, the one being linked (D41);
		// trusted target code ("tcb") is the same in both.
		// The main package of a test binary is the one sgo test generates (-sgotestmain, D69), and
		// only it: a testmain package is never linked into any other program.
		if msg := sgoIdentConfigErr(lib.Pkg, id.config, sgoConfigName(), sgoTest); msg != "" {
			Errorf("sgo:1:1: %s", msg)
		}
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
	} else if msg := sgoBuildIDErr(*flagBuildid, f[1], f[2], sgoDrainArg, sgoGrantsArg, sgoLab, sgoTest); msg != "" {
		Errorf("sgo:1:1: %s", msg)
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

// sgoLab is set by -sgolab: the program is linked for the lab execution configuration (D41).
var sgoLab bool

// sgoTest is set by -sgotest: the program is an S-Go test binary (D62). It marks only the
// runtime; the packages are compiled as for any program of the configuration, so it does not
// change the configuration the identity check expects (the driver names it in the build ID).
var sgoTest bool

// sgoConfigName is the execution configuration the linked packages must have been compiled for.
func sgoConfigName() string {
	if sgoLab {
		return "lab"
	}
	return "prod"
}
