// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"flag"
	"fmt"
	"internal/buildcfg"
	"internal/goversion"
	"os"
	"sort"
)

// SGo compile flag allowlists (D77, W12 R3). Every compilation emits an S-Go identity, so the
// shipped compiler accepts exactly the command lines the sgo driver writes: anything else could
// change code generation or disable a check (-wb=false, -B, -d=..., -N, -race, ...) without the
// object recording it. The lists are refuse-by-default: a flag added upstream is refused until it
// is added here deliberately.
var (
	// SGoFlagsSGo are the flags of an S-Go package (sgo build, check, test, lab).
	// -m only prints optimization decisions (test-x10 reads them); it changes no code.
	SGoFlagsSGo = []string{"o", "p", "importcfg", "pack", "complete", "trimpath", "sgolab", "sgocheck", "sgotestmain", "m"}
	// SGoFlagsTCB are the flags of a trusted target package (the driver's TCB build, tcb.go).
	SGoFlagsTCB = []string{"o", "p", "importcfg", "pack", "complete", "trimpath", "std", "lang", "symabis", "asmhdr"}
)

// SGoCheckToolInput refuses, with SGO_E_USAGE and exit status 2, a compile flag or a compiler
// environment setting that the driver never uses (D77, W12 R3). sgo reports whether the package is
// S-Go code (else it is trusted target code, SGO-TCB-002).
func SGoCheckToolInput(sgo bool) {
	allowed := SGoFlagsTCB
	if sgo {
		allowed = SGoFlagsSGo
	}
	var bad []string
	flag.CommandLine.Visit(func(f *flag.Flag) {
		for _, a := range allowed {
			if f.Name == a {
				return
			}
		}
		bad = append(bad, "-"+f.Name)
	})
	sort.Strings(bad)
	for _, b := range bad {
		sgoUsage("compile flag %s is not admitted (SGO-TOOL-001)", b)
	}
	if !sgo && Flag.Lang != "" && Flag.Lang != fmt.Sprintf("go1.%d", goversion.Version) {
		sgoUsage("trusted target code is compiled with -lang=go1.%d, not %s (SGO-TCB-002)", goversion.Version, Flag.Lang)
	}
	if v := os.Getenv("GOCOMPILEDEBUG"); v != "" {
		sgoUsage("GOCOMPILEDEBUG is not admitted (SGO-TOOL-001)")
	}
	// The target configuration is the one the driver pins (toolchain.go).
	for _, e := range buildcfg.SGoTargetConfigErrs() {
		sgoUsage("%s", e)
	}
	if sgoUsageErrors > 0 {
		os.Exit(2)
	}
}

var sgoUsageErrors int

func sgoUsage(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "sgo:1:1: SGO_E_USAGE: "+format+"\n", args...)
	sgoUsageErrors++
}
