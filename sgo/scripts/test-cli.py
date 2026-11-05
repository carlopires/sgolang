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
	if len(args) > 0 && args[0] == "wait" {
		e := root.Sleep(60000)
		check out.WriteString("wait interrupted=" + strconv.FormatBool(sgo.IsInterrupted(e)) + "\\n")
		return e
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


# APP_PROBE uses the per-user directories and sockets of D182.
APP_PROBE = """package main

import (
	"sgo"
	"std/strconv"
)

func kind(err error) string {
	return err.Error()
}

func main(root *sgo.Context) error {
	out := root.Stdout()
	cfg := root.Dir("config")
	n := 0
	old, err := cfg.ReadFile("settings")
	if err != nil {
		check out.WriteString("config: " + kind(err) + "\\n")
	} else {
		check out.WriteString("config: " + old + "\\n")
		n = 1
	}
	check cfg.WriteFile("settings", "run=" + strconv.Itoa(n+1))
	_, e := root.Dir("state").ReadFile("log")
	check out.WriteString("state: " + kind(e) + "\\n")
	l := check root.Listen("unix:@runtime/ctl.sock")
	nursery (s) {
		c := check s.Dial("unix:@runtime/ctl.sock")
		_ = check c.Write("ping\\n")
		a := check l.Accept(s)
		line := check a.ReadLine(64)
		check out.WriteString("socket: " + line + "\\n")
	}
	return l.Close()
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

        # Per-user application directories and sockets (D182): resolved by the runtime under the
        # platform convention, created with mode 0700 on first use, never visible to the program.
        app = os.path.join(tmp, "appdirs")
        os.makedirs(app)
        with open(os.path.join(app, "sgo.mod"), "w") as f:
            f.write("module example.com/tools/clitool\n\nsgo %s\n\ngrant dir config rw\ngrant dir state ro\n"
                    "grant listen unix:@runtime/ctl.sock\ngrant connect unix:@runtime/ctl.sock\n" % open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip())
        with open(os.path.join(app, "main.sgo"), "w") as f:
            f.write(APP_PROBE)
        r = subprocess.run([sgo, "build", "-o", "prog", "."], cwd=app, capture_output=True, text=True, timeout=300)
        if r.returncode != 0:
            problems.append("appdirs build: %s" % r.stderr[-400:])
        else:
            home, rt = os.path.join(tmp, "home"), os.path.join(tmp, "rt")
            os.makedirs(home)
            os.makedirs(rt)
            env = {"PATH": "/usr/bin:/bin", "HOME": home}
            if platform.system() == "Darwin":
                env["TMPDIR"] = rt
                cfg = os.path.join(home, "Library", "Application Support", "clitool")
            else:
                env["XDG_RUNTIME_DIR"] = rt
                cfg = os.path.join(home, ".config", "clitool")
            outs = [subprocess.run([os.path.join(app, "prog")], cwd=app, env=env, capture_output=True, text=True, timeout=30) for _ in range(2)]
            want = ["config: NotFound: Dir.ReadFile\nstate: NotFound: Dir.ReadFile\nsocket: ping\n", "config: run=1\nstate: NotFound: Dir.ReadFile\nsocket: ping\n"]
            if [o.stdout for o in outs] != want or any(o.returncode for o in outs):
                problems.append("appdirs: %r" % [(o.returncode, o.stdout, o.stderr[-200:]) for o in outs])
            elif not os.path.isdir(cfg) or os.stat(cfg).st_mode & 0o777 != 0o700:
                problems.append("appdirs: %s missing or not mode 0700" % cfg)
            elif not os.path.isdir(os.path.join(rt, "clitool")) or os.listdir(os.path.join(rt, "clitool")):
                problems.append("appdirs: socket directory %s/clitool missing or not emptied" % rt)
            if platform.system() != "Darwin": # an XDG variable overrides the default; a relative one is ignored
                xdg = os.path.join(tmp, "xdg")
                r = subprocess.run([os.path.join(app, "prog")], cwd=app, env=dict(env, XDG_CONFIG_HOME=xdg), capture_output=True, text=True, timeout=30)
                if not os.path.isfile(os.path.join(xdg, "clitool", "settings")):
                    problems.append("appdirs: XDG_CONFIG_HOME not used: %r" % r.stdout)
            r = subprocess.run([os.path.join(app, "prog")], cwd=app, env={"PATH": "/usr/bin:/bin"}, capture_output=True, text=True, timeout=30)
            if r.returncode != 1 or "NotFound" not in r.stderr:
                problems.append("appdirs without HOME: exit %d %r" % (r.returncode, r.stderr[-200:]))

        # Lab interrupts (D179): -interrupt injects them at virtual times; the trace replays them.
        for times, status in [("2000", 1), ("2000,2000", 130)]:
            trace = os.path.join(tmp, "i.jsonl")
            r = subprocess.run([sgo, "lab", "run", "-interrupt", times, "-trace", trace, ".", "--", "wait"], cwd=tmp, capture_output=True, text=True, timeout=120)
            r2 = subprocess.run([sgo, "lab", "replay", trace, "."], cwd=tmp, capture_output=True, text=True, timeout=120)
            want = "wait interrupted=true\n" if status == 1 else ""
            if r.returncode != status or r.stdout != want or r2.returncode != status or r2.stdout != want:
                problems.append("lab -interrupt %s: run %d %r, replay %d %r %r" % (times, r.returncode, r.stdout, r2.returncode, r2.stdout, r2.stderr[-200:]))

        # Lab input refusals (SGO-LAB-007): bad -stdin, -env, and -interrupt values are refused before
        # the run, and a replay takes its input from the trace alone (no -stdin, -env, -interrupt).
        refusals = [(["run", "-stdin", os.path.join(tmp, "absent.txt")], "cannot read -stdin file"),
                    (["run", "-env", "NOEQUALS"], "-env must be NAME=VALUE"),
                    (["run", "-interrupt", "5,4"], "non-decreasing"),
                    (["run", "-interrupt", "1,2,3"], "at most two times"),
                    (["replay", "-stdin", os.path.join(tmp, "in.txt"), os.path.join(tmp, "i.jsonl")], "unknown flag -stdin"),
                    (["replay", "-env", "A=1", os.path.join(tmp, "i.jsonl")], "unknown flag -env")]
        for args, want in refusals:
            r = subprocess.run([sgo, "lab"] + args + ["."], cwd=tmp, capture_output=True, text=True, timeout=120)
            if r.returncode != 2 or "SGO_E_USAGE" not in r.stderr or want not in r.stderr:
                problems.append("lab refusal %s: exit %d %r" % (" ".join(args[:2]), r.returncode, r.stderr[-200:]))

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
    print("test-cli: ok: installed-binary arguments, broken pipe ends quietly, terminal detection and width, idle stdin ends at its deadline, ReadSecret hides the secret, lab stdin replays, lab input refusals, an interrupt unwinds, a second one ends the process, lab interrupts replay, application directories and sockets")


if __name__ == "__main__":
    main()
