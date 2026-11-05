#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

"""CLI process interface checks the conformance runner cannot express (D178): a program whose
standard output is piped into a reader that exits sees IsBrokenPipe and may end quietly, with no
SIGPIPE; under a pseudo-terminal, IsTerminal holds and TerminalWidth is the terminal's width; and
the arguments of an installed binary reach root.Args.

usage: test-cli.py [--sgo PATH]
"""

import os
import platform
import shutil
import subprocess
import sys
import tempfile

import sgoargs

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))

PROBE = """package main

import (
	"sgo"
	"std/strconv"
	"std/strings"
)

func main(root *sgo.Context) error {
	out, errw := root.Stdout(), root.Stderr()
	args := root.Args()
	if len(args) > 0 && args[0] == "flood" {
		for i := 0; i < 1000000; i++ {
			if err := out.WriteString("line of output that fills the pipe\\n"); err != nil {
				if sgo.IsBrokenPipe(err) {
					return errw.WriteString("broken pipe after " + strconv.Itoa(i) + " lines\\n")
				}
				return err
			}
		}
		return errw.WriteString("never broken\\n")
	}
	return out.WriteString("args=" + strings.Join(args, "|") + " terminal=" + strconv.FormatBool(out.IsTerminal()) + " width=" + strconv.Itoa(out.TerminalWidth()) + "\\n")
}
"""


def main():
    sgoargs.check(flags={"--sgo": "value"})
    args = sys.argv[1:]
    sgo = os.path.join(ROOT, "pkg", "tool", "%s_%s" % (platform.system().lower(), {"x86_64": "amd64", "aarch64": "arm64"}.get(platform.machine(), platform.machine())), "sgo")
    while args:
        if args[0] == "--sgo":
            sgo, args = args[1], args[2:]
        else:
            sys.exit("usage: test-cli.py [--sgo PATH]")
    problems = []
    with tempfile.TemporaryDirectory() as tmp:
        with open(os.path.join(tmp, "sgo.mod"), "w") as f:
            f.write("module example.com/cliprobe\n\nsgo %s\n" % open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip())
        with open(os.path.join(tmp, "main.sgo"), "w") as f:
            f.write(PROBE)
        prog = os.path.join(tmp, "prog")
        r = subprocess.run([sgo, "build", "-o", prog, "."], cwd=tmp, capture_output=True, text=True)
        if r.returncode != 0:
            sys.exit("test-cli: probe build failed:\n" + r.stdout + r.stderr)

        # Arguments of an installed binary, not a terminal.
        r = subprocess.run([prog, "get", "/hello", "two words"], capture_output=True, text=True, timeout=30)
        want = "args=get|/hello|two words terminal=false width=0\n"
        if r.returncode != 0 or r.stdout != want:
            problems.append("args: exit %d, stdout %r, want %r" % (r.returncode, r.stdout, want))

        # Broken pipe: the reader exits after one line; the program ends quietly with status 0.
        r = subprocess.run(["bash", "-c", '"$0" flood | head -1 >/dev/null; echo "status=${PIPESTATUS[0]}"', prog],
                           capture_output=True, text=True, timeout=60)
        if "status=0" not in r.stdout or "broken pipe after" not in r.stderr:
            problems.append("broken pipe: stdout %r, stderr %r" % (r.stdout, r.stderr))

        # A pseudo-terminal of 97 columns.
        if shutil.which("script"):
            inner = "stty cols 97; %s" % prog
            if platform.system() == "Darwin":
                cmd = ["script", "-q", "/dev/null", "sh", "-c", inner]
            else:
                cmd = ["script", "-qec", inner, "/dev/null"]
            r = subprocess.run(cmd, capture_output=True, text=True, timeout=30, stdin=subprocess.DEVNULL)
            out = r.stdout.replace("\r", "").replace("^D", "")
            if "args= terminal=true width=97" not in out:
                problems.append("terminal: exit %d, output %r" % (r.returncode, out))
        else:
            problems.append("terminal: no script command to allocate a pseudo-terminal")

    if problems:
        for p in problems:
            print("test-cli: FAIL", p)
        sys.exit(1)
    print("test-cli: ok: installed-binary arguments, broken pipe ends quietly, terminal detection and width")


if __name__ == "__main__":
    main()
