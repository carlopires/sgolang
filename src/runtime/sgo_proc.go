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

// Environment variables by grant (D180). "grant env NAME" or "grant env PREFIX*" in sgo.mod lets
// the root read exactly those variables; every other name is refused (MissingCapability). In the
// lab the environment is the manifest's (sgo lab -env NAME=VALUE), never the host's.
var (
	sgoEnvGrants []string // granted names; a trailing '*' grants a prefix
	sgoLabEnv    []string // lab runs: NAME=VALUE entries from the manifest
)

// sgoEnvGranted reports whether name is granted.
func sgoEnvGranted(name string) bool {
	for _, g := range sgoEnvGrants {
		if g == name || len(g) > 0 && g[len(g)-1] == '*' && len(name) >= len(g)-1 && name[:len(g)-1] == g[:len(g)-1] {
			return true
		}
	}
	return false
}

// sgoLookupEnv finds key in a NAME=VALUE list.
func sgoLookupEnv(env []string, key string) (string, bool) {
	for _, s := range env {
		if len(s) > len(key) && s[len(key)] == '=' && s[:len(key)] == key {
			return s[len(key)+1:], true
		}
	}
	return "", false
}

// sgo_getenv is root.Getenv(name) after the root check: the value, whether it is set, and whether
// name is granted at all.
//
//go:linkname sgo_getenv sgo.runtime_getenv
func sgo_getenv(name string) (string, bool, bool) {
	if !sgoEnvGranted(name) {
		return "", false, false
	}
	if sgoLab != nil {
		v, ok := sgoLookupEnv(sgoLabEnv, name)
		return v, ok, true
	}
	v, ok := sgoLookupEnv(environ(), name)
	return v, ok, true
}
