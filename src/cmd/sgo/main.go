// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Sgo is the S-Go driver (SGO-TOOL-001). It is host code built only by the pinned bootstrap
// (stage-0, SGO-BUILD-001) and installed next to the fork's compile, asm, and link. It never runs a
// go binary and never accesses the network.
//
// Usage:
//
//	sgo build [-o out] [-target=<os>/<arch>] [package]
//	sgo run <main package>
//	sgo check [-json] [packages]
//	sgo test [-v] [-run regexp] [packages] (test.go: one process per test, D62)
//	sgo version
//	sgo fmt [files]
//	sgo lab run|replay|sweep ... (lab.go: the lab execution configuration, D41)
//
// Any other command or flag is rejected with SGO_E_USAGE. There is no flag, environment variable,
// file name, build tag, or pragma that changes the language or disables a check.
package main

import (
	"fmt"
	"os"
	"strings"
)

// toolErr is a diagnostic that ends the command: "pos: CODE: message" (SGO-ERR-001). Command-line
// errors have no source position and are reported at sgo:1:1.
type toolErr struct {
	pos, code, msg string
	status         int
}

func (e *toolErr) Error() string { return e.pos + ": " + e.code + ": " + e.msg }

func usage(format string, args ...any) *toolErr {
	return &toolErr{"sgo:1:1", "SGO_E_USAGE", fmt.Sprintf(format, args...), 2}
}

// buildFailed reports that diagnostics were already printed (compiler output).
var buildFailed = &toolErr{status: 1}

func main() {
	err := run(os.Args[1:])
	if err == nil {
		return
	}
	e, ok := err.(*toolErr)
	if !ok {
		e = &toolErr{"sgo:1:1", "SGO_E_INTERNAL", err.Error(), 1}
	}
	if e.code != "" {
		fmt.Fprintln(os.Stderr, e.Error())
	}
	os.Exit(e.status)
}

func run(args []string) error {
	if len(args) == 0 {
		return usage("missing command (sgo build | run | check | test | lab | mod | fmt | version)")
	}
	switch cmd, rest := args[0], args[1:]; cmd {
	case "build":
		return cmdBuild(rest)
	case "run":
		return cmdRun(rest)
	case "version":
		if len(rest) > 0 && rest[0] == "-m" {
			return cmdVersionM(rest[1:])
		}
		if len(rest) != 0 {
			return usage("sgo version takes no arguments, or -m and binaries")
		}
		return cmdVersion()
	case "fmt":
		return cmdFmt(rest)
	case "lab":
		return cmdLab(rest)
	case "test":
		return cmdTest(rest)
	case "check":
		return cmdCheck(rest)
	case "mod":
		return cmdMod(rest)
	default:
		return usage("unknown command %q", cmd)
	}
}

// buildFlags are the flags of sgo build and sgo run.
type buildFlags struct {
	out    string
	target string
	drain  int64    // drain budget in ms (SGO-DRAIN-002); 0: none
	args   []string // positional arguments
}

// parseBuildFlags accepts exactly the flags of SGO-TOOL-001 for the command; anything else is
// SGO_E_USAGE. Flags must precede positional arguments.
func parseBuildFlags(cmd string, args []string) (*buildFlags, error) {
	f := &buildFlags{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			f.args = args[i:]
			break
		}
		name, val, hasVal := strings.Cut(strings.TrimLeft(a, "-"), "=")
		switch {
		case name == "o" && cmd == "build":
			if !hasVal {
				if i+1 >= len(args) {
					return nil, usage("flag -o needs a value")
				}
				i++
				val = args[i]
			}
			f.out = val
		case name == "target" && cmd == "build":
			if !hasVal {
				if i+1 >= len(args) {
					return nil, usage("flag -target needs a value")
				}
				i++
				val = args[i]
			}
			f.target = val
		case name == "drain-budget":
			if !hasVal {
				if i+1 >= len(args) {
					return nil, usage("flag -drain-budget needs a value")
				}
				i++
				val = args[i]
			}
			d, ok := parseBudget(val)
			if !ok {
				return nil, usage("-drain-budget must be a positive number of milliseconds, not %q (SGO-DRAIN-002)", val)
			}
			f.drain = d
		default:
			return nil, usage("unknown flag %s for sgo %s", a, cmd)
		}
	}
	return f, nil
}

// parseBudget accepts a positive decimal integer of at most 12 digits (about 31 years).
func parseBudget(s string) (int64, bool) {
	if s == "" || len(s) > 12 {
		return 0, false
	}
	var v int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*10 + int64(c-'0')
	}
	return v, v > 0
}

func cmdVersion() error {
	t, err := findToolchain()
	if err != nil {
		return err
	}
	fmt.Printf("sgo %s spec %s admission %s upstream %s (host-built)\n", t.version, t.specVersion, t.admission, t.upstream)
	return nil
}
