// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The process interface of S-Go programs (D178): command-line arguments.

package runtime

import _ "unsafe" // for go:linkname

// sgo_args returns a fresh copy of the command-line arguments after the program name (D178,
// SGO-PROC-004). The caller is the root task (package sgo checks it). In the lab the arguments
// are those of the manifest's run, recorded in its trace by the driver, so a replay sees the same.
//
//go:linkname sgo_args sgo.runtime_args
func sgo_args() []string {
	if len(argslice) <= 1 {
		return []string{}
	}
	out := make([]string, len(argslice)-1)
	copy(out, argslice[1:])
	return out
}
