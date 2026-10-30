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
//	sgo version
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
		return usage("missing command (sgo build | run | version)")
	}
	switch cmd, rest := args[0], args[1:]; cmd {
	case "build":
		return cmdBuild(rest)
	case "run":
		return cmdRun(rest)
	case "version":
		if len(rest) != 0 {
			return usage("sgo version takes no arguments")
		}
		return cmdVersion()
	case "fmt":
		return usage("sgo fmt is not implemented yet (S-Go W03)")
	default:
		return usage("unknown command %q", cmd)
	}
}

// buildFlags are the flags of sgo build and sgo run.
type buildFlags struct {
	out    string
	target string
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
			return nil, usage("-drain-budget is not implemented yet (S-Go W04)")
		default:
			return nil, usage("unknown flag %s for sgo %s", a, cmd)
		}
	}
	return f, nil
}

func cmdVersion() error {
	t, err := findToolchain()
	if err != nil {
		return err
	}
	fmt.Printf("sgo %s spec %s admission %s upstream %s (host-built)\n", t.version, t.specVersion, t.admission, t.upstream)
	return nil
}
