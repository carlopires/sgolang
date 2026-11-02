// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package buildcfg

import (
	"fmt"
	"os"
)

// SGoTargetConfigErrs returns a diagnostic for each target setting of the environment that the S-Go
// toolchain does not build with (SGO-BUILD-001, D77, W12 R3): the sgo driver pins GOEXPERIMENT to
// the default set, GOAMD64=v1 or GOARM64=v8.0, and GOFIPS140=off, and the shipped compiler,
// assembler, and linker refuse any other value, so a direct invocation cannot change code generation
// without the object recording it.
func SGoTargetConfigErrs() []string {
	var errs []string
	if base, err := ParseGOEXPERIMENT(GOOS, GOARCH, ""); os.Getenv("GOEXPERIMENT") != "" || err == nil && Experiment.Flags != base.Flags {
		errs = append(errs, fmt.Sprintf("GOEXPERIMENT=%s is not admitted; S-Go targets use the default experiment set (SGO-BUILD-001)", Experiment.String()))
	}
	switch GOARCH {
	case "amd64":
		if GOAMD64 != 1 {
			errs = append(errs, fmt.Sprintf("GOAMD64=v%d is not admitted; the S-Go target is v1 (SGO-BUILD-001)", GOAMD64))
		}
	case "arm64":
		if g := GOARM64; g.Version != "v8.0" || g.LSE || g.Crypto {
			errs = append(errs, fmt.Sprintf("GOARM64=%s is not admitted; the S-Go target is v8.0 (SGO-BUILD-001)", g))
		}
	}
	if GOFIPS140 != "off" || os.Getenv("GOFIPS140") != "" && os.Getenv("GOFIPS140") != "off" {
		errs = append(errs, fmt.Sprintf("GOFIPS140=%s is not admitted; S-Go programs use no FIPS module (SGO-BUILD-001)", os.Getenv("GOFIPS140")))
	}
	return errs
}
