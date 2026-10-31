// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package objabi

import (
	"fmt"
	"strings"

	"internal/buildcfg"
)

const (
	ElfRelocOffset   = 256
	MachoRelocOffset = 2048    // reserve enough space for ELF relocations
	GlobalDictPrefix = ".dict" // prefix for names of global dictionaries
)

// HeaderString returns the toolchain configuration string written in
// Go object headers. This string ensures we don't attempt to import
// or link object files that are incompatible with each other. This
// string always starts with "go object ".
func HeaderString() string {
	archExtra := ""
	if k, v := buildcfg.GOGOARCH(); k != "" && v != "" {
		archExtra = " " + k + "=" + v
	}
	return fmt.Sprintf("go object %s %s %s%s X:%s sgo:%s\n",
		buildcfg.GOOS, buildcfg.GOARCH,
		buildcfg.Version, archExtra,
		strings.Join(buildcfg.Experiment.Enabled(), ","), SGoABI)
}

// SGoABI is the S-Go ABI identity (SGO-IMP-002): the lowering contract of nurseries, failures, and
// checked arithmetic between compiled S-Go packages and the runtime. It is part of every object
// and export-data header the S-Go tools write, so an object written by a stock Go toolchain
// (whose header lacks " sgo:") is recognizable as foreign.
const SGoABI = "sgo-abi-1"

// IsSGoHeader reports whether an object header line was written by the S-Go tools.
func IsSGoHeader(hdr string) bool { return strings.Contains(hdr, " sgo:") }
