#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

"""CLI process interface checks the conformance runner cannot express (D178): a program whose
standard output is piped into a reader that exits sees IsBrokenPipe and may end quietly, with no
SIGPIPE; under a pseudo-terminal, IsTerminal holds and TerminalWidth is the terminal's width; and
the arguments of an installed binary reach root.Args; a read of idle stdin ends at its deadline;
ReadSecret does not echo under a pseudo-terminal and restores echo after; and a lab run reads the
-stdin text and replays it.

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
	in := root.Stdin()
	if len(args) > 0 && args[0] == "intr" {
		check out.WriteString("ready\\n")
		nursery (n) {
			n.Spawn(func(t *sgo.Context) error {
				_, e := in.ReadLine(64)
				_ = out.WriteString("reader interrupted=" + strconv.FormatBool(sgo.IsInterrupted(e)) + "\\n")
				return e
			})
			n.Spawn(func(t *sgo.Context) error {
				e := t.Sleep(60000)
				_ = out.WriteString("sleeper interrupted=" + strconv.FormatBool(sgo.IsInterrupted(e)) + "\\n")
				return e
			})
		}
		_ = out.WriteString("nursery closed\\n")
		return root.Checkpoint()
	}
	if len(args) > 0 && args[0] == "stuck" {
		check out.WriteString("ready\\n")
		_, e := in.ReadLine(64)
		check out.WriteString("first interrupted=" + strconv.FormatBool(sgo.IsInterrupted(e)) + "\\n")
		k := 0
		for {
			k++
		}
	}
	if len(args) > 0 && args[0] == "idle" {
		var err error = nil
		nursery (n) {
			n.CancelAfter(150)
			_, e := in.ReadLine(256)
			err = e
		}
		return out.WriteString("deadline=" + strconv.FormatBool(sgo.IsDeadline(err)) + "\\n")
	}
	if len(args) > 0 && args[0] == "secret" {
		check out.WriteString("password: ")
		pw := check in.ReadSecret(256)
		check out.WriteString("got " + strconv.Itoa(len(pw)) + " chars\\n")
		line := check in.ReadLine(256)
		return out.WriteString("then read: " + line + "\\n")
	}
	if len(args) > 0 && args[0] == "lines" {
		n := 0
		for {
			_, err := in.ReadLine(256)
			if sgo.IsEOF(err) {
				return out.WriteString("lines=" + strconv.Itoa(n) + "\\n")
			}
			if err != nil {
				return err
			}
			n++
		}
	}
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

        # A read of idle stdin (a pipe whose writer waits) ends at its 150 ms deadline.
        r = subprocess.run(["bash", "-c", 'sleep 5 | "$0" idle', prog], capture_output=True, text=True, timeout=30)
        if r.stdout != "deadline=true\n":
            problems.append("idle stdin: stdout %r stderr %r" % (r.stdout, r.stderr))

        # A lab run reads the -stdin text, records it, and the replay reproduces it.
        with open(os.path.join(tmp, "in.txt"), "w") as f:
            f.write("one\ntwo\nthree\n")
        trace = os.path.join(tmp, "t.jsonl")
        r = subprocess.run([sgo, "lab", "run", "-stdin", "in.txt", "-trace", trace, ".", "--", "lines"], cwd=tmp, capture_output=True, text=True, timeout=120)
        r2 = subprocess.run([sgo, "lab", "replay", trace, "."], cwd=tmp, capture_output=True, text=True, timeout=120)
        if r.stdout != "lines=3\n" or r2.returncode != 0 or r2.stdout != "lines=3\n":
            problems.append("lab stdin: run %r %r, replay %r %r" % (r.stdout, r.stderr[-200:], r2.stdout, r2.stderr[-200:]))

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
            # ReadSecret: the secret is read but not echoed; echo is back for the next line.
            feed = "sleep 1; printf 'hunter2\\n'; sleep 0.5; printf 'visible\\n'; sleep 0.5"
            if platform.system() == "Darwin":
                cmd = ["bash", "-c", "(%s) | script -q /dev/null %s secret" % (feed, prog)]
            else:
                cmd = ["bash", "-c", "(%s) | script -qec '%s secret' /dev/null" % (feed, prog)]
            r = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
            out = r.stdout.replace("\r", "").replace("^D", "")
            if "got 7 chars" not in out or "hunter2" in out or "then read: visible" not in out:
                problems.append("ReadSecret: output %r" % out)
        else:
            problems.append("terminal: no script command to allocate a pseudo-terminal")

        # Interrupts (D179): the first SIGINT or SIGTERM cancels the root, so the program unwinds with
        # status 1; a second one ends it at once with 128+signal. The child starts with default signal
        # dispositions (a background job of a non-interactive shell would ignore SIGINT).
        import signal, time
        def interrupt(mode, sigs):
            p = subprocess.Popen([prog, mode], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
                                 preexec_fn=lambda: (signal.signal(signal.SIGINT, signal.SIG_DFL), signal.signal(signal.SIGTERM, signal.SIG_DFL)))
            deadline = time.time() + 10
            while p.poll() is None and time.time() < deadline:
                line = p.stdout.readline()
                if line == "ready\n":
                    break
            for s in sigs:
                time.sleep(0.3)
                p.send_signal(s)
            try:
                out, err = p.communicate(timeout=10)
            except subprocess.TimeoutExpired:
                p.kill()
                out, err = p.communicate()
                return -1, out, err
            return p.returncode, out, err
        for sig, name, status in [(signal.SIGINT, "SIGINT", 130), (signal.SIGTERM, "SIGTERM", 143)]:
            code, out, err = interrupt("intr", [sig])
            want = ["reader interrupted=true", "sleeper interrupted=true", "nursery closed"]
            if code != 1 or sorted(out.split()) != sorted(" ".join(want).split()) or "interrupted (%s)" % name not in err:
                problems.append("interrupt %s: exit %d, stdout %r, stderr %r" % (name, code, out, err[-300:]))
            code, out, err = interrupt("stuck", [sig, sig])
            if code != status or out != "first interrupted=true\n":
                problems.append("second %s: exit %d (want %d), stdout %r, stderr %r" % (name, code, status, out, err[-300:]))

    if problems:
        for p in problems:
            print("test-cli: FAIL", p)
        sys.exit(1)
    print("test-cli: ok: installed-binary arguments, broken pipe ends quietly, terminal detection and width, idle stdin ends at its deadline, ReadSecret hides the secret, lab stdin replays, an interrupt unwinds, a second one ends the process")


if __name__ == "__main__":
    main()
