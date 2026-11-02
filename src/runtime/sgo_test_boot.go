// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test binaries (W10, D62, D64). The linker's -sgotest sets sgoTestBuild; only then does the
// root task read a test selection from fd 3 (written by sgo test, never the environment):
//
//	test N     the index of the test to run in the generated test main
//	tmp PATH   the test's temporary directory, served as an extra rw directory grant
//
// A production binary has sgoTestBuild empty: no input of its environment or descriptors makes it
// a test binary, and sgo.NewTest fails in it.
//
// At the end of the test (the root task returning) every obligation of the root is resolved, and
// any it left (abandoned handles, permits, or tokens) fails the test with LeakedWork (D64).

package runtime

import "unsafe"

// sgoTestBuild is "test" in a test binary (set by the linker, never by -X).
var sgoTestBuild string

const sgoTestFD = 3

// sgoTestGrantName names the test directory's grant; no sgo.mod grant name contains NUL, so
// root.Dir cannot reach it, only T.TempDir.
const sgoTestGrantName = "\x00test"

var (
	sgoTestIndex = -1 // the selected test
	sgoTestGrant = -1 // the index of the test's directory grant
)

// sgoTestBoot reads the test selection; it runs after the module's grants are initialized.
func sgoTestBoot() {
	if sgoTestBuild != "test" {
		return
	}
	data, ok := sgoLabReadAll(sgoTestFD)
	if !ok {
		sgoDefect("a test binary runs only under sgo test (no test selection on fd 3)")
	}
	for len(data) > 0 {
		line := data
		if i := sgoIndexByte(data, '\n'); i >= 0 {
			line, data = data[:i], data[i+1:]
		} else {
			data = ""
		}
		key, val := line, ""
		if i := sgoIndexByte(line, ' '); i >= 0 {
			key, val = line[:i], line[i+1:]
		}
		switch key {
		case "test":
			sgoTestIndex = int(sgoLabAtoi(val))
		case "tmp":
			g := sgoDirGrant{name: sgoTestGrantName, ro: false, path: val, fd: -1}
			if sgoLab != nil {
				g.fake = &sgoFakeNode{dir: true}
			} else {
				fd := sgoOpenat(sgoAT_FDCWD, sgoCString(val), sgoO_RDONLY|sgoO_DIRECTORY|sgoO_CLOEXEC, 0)
				if fd < 0 {
					sgoDefect("test selection: cannot open the test directory " + val)
				}
				g.fd = fd
			}
			sgoDirGrants = append(sgoDirGrants, g)
			sgoTestGrant = len(sgoDirGrants) - 1
		case "":
		default:
			sgoDefect("test selection: unknown key " + key)
		}
	}
	if sgoTestIndex < 0 || sgoTestGrant < 0 {
		sgoDefect("test selection: missing test index or directory")
	}
}

// sgoTestEnd resolves the root's obligations at the end of a test (D64, SGO-TEST-003). If the test
// returned nil and left any, it fails the test with LeakedWork; if it returned an error, that stays
// the primary outcome and the abandoned obligations are returned for its record's chain.
func sgoTestEnd(t *sgoTask, returnedErr bool) []sgoFailureInfo {
	if sgoTestBuild != "test" {
		return nil
	}
	before, from := t.abandoned, len(t.entries)
	sgoPermitsResolve(t, nil, true) // permits, tokens, and handles the test left open
	leaked := t.abandoned - before
	if leaked == 0 {
		return nil
	}
	chain := make([]sgoFailureInfo, 0, len(t.entries)-from)
	for _, e := range t.entries[from:] {
		chain = append(chain, e.info)
	}
	if returnedErr {
		return chain
	}
	sgoWriteRecord(sgoFailureInfo{"LeakedWork", "the test left " + sgoItoa(uint64(leaked)) + " obligation(s) unresolved (D64)", ""}, chain, sgoOrigin{})
	sgoProgramExit(1, "LeakedWork")
	return nil
}

// Package sgo entry points (src/sgo/test.go).

//go:linkname sgo_testStart sgo.runtime_testStart
func sgo_testStart(task unsafe.Pointer) (index, grant int, ok bool) {
	if sgoTestBuild != "test" || (*sgoTask)(task) != sgoCurrent() || sgoCurrent().id != 0 {
		return 0, 0, false
	}
	return sgoTestIndex, sgoTestGrant, true
}

//go:linkname sgo_testLog sgo.runtime_testLog
func sgo_testLog(s string) {
	b := make([]byte, 0, len(s)+8)
	b = append(b, "    "...)
	b = append(b, s...)
	b = append(b, '\n')
	write(2, unsafe.Pointer(&b[0]), int32(len(b)))
}
