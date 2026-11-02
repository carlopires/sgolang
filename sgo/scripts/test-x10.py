#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""test-x10.py - nurseries in exported functions across packages, with and without inlining (X10).

A library exports small (inlinable) functions whose bodies contain nurseries: a nursery whose
child writes a captured result, one whose child fails, and a return that crosses two nested
nurseries. main calls them from frames with deferred calls. The program is built twice from the
same sources with the fork's compile and link - default optimization and with inlining disabled
(-l) - and both binaries, and the driver's build, must produce the same exit status, stdout, and
diagnostic record, equal to the expected ones.

The inlining decisions are pinned too (-m=2): the lowered nursery statement costs more than the
inlining budget, so a function containing a nursery is never inlined into its caller, while the
spawned closures inside it are. The budget can only grow through PGO or the new-inliner
experiment, and the driver admits neither, so this is the only configuration S-Go builds.
Generics are denied in subset 0 (SGO-GEN-001); that half of X10 waits for their admission.
"""

import glob
import os
import platform
import subprocess
import sys
import tempfile

from sgotool import buildid

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
SPEC_VERSION = open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip()
GOOS = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
GOARCH = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[platform.machine()]
TOOLS = os.path.join(ROOT, "pkg", "tool", f"{GOOS}_{GOARCH}")

LIB = '''package lib

import "sgo"

// Check verifies a+b in a child task; a mismatch is the child's error after the join.
func Check(a, b, want int) {
	nursery (n) {
		n.Spawn(func(task *sgo.Context) error {
			if a+b != want {
				return sgo.NewError("bad sum")
			}
			return nil
		})
	}
}

// Nested returns from inside two nurseries; both close, inner first, before the result is seen.
func Nested(w *sgo.Writer) int {
	nursery (outer) {
		outer.Spawn(func(task *sgo.Context) error { return nil })
		nursery (inner) {
			inner.Spawn(func(task *sgo.Context) error { return w.WriteString("inner child\\n") })
			return 7
		}
	}
}

// Fail unwinds with its child's panic once the nursery joined.
func Fail(msg string) {
	nursery (n) {
		n.Spawn(func(task *sgo.Context) error { panic(msg) })
	}
}
'''

MAIN = '''package main

import (
	"sgo"

	"example.com/x10/lib"
)

func check(w *sgo.Writer) error {
	defer func() { _ = w.WriteString("deferred after the failed nursery\\n") }()
	lib.Check(2, 3, 5)
	if lib.Nested(w) != 7 {
		return sgo.NewError("bad nested")
	}
	if err := w.WriteString("before failure\\n"); err != nil {
		return err
	}
	lib.Fail("boom")
	return w.WriteString("not reached\\n")
}

func main(root *sgo.Context) error {
	return check(root.Stdout())
}
'''

WANT_STDOUT = "inner child\nbefore failure\ndeferred after the failed nursery\n"
WANT_STDERR = ('{"sgo_diagnostic":1,"kind":"Panic","message":"boom","position":"lib/lib.sgo:31:43",'
               '"task_id":4,"scope_id":4,"chain":[]}\n')


def run(args, cwd, env, check=True):
    r = subprocess.run(args, cwd=cwd, env=env, capture_output=True, text=True)
    if check and r.returncode != 0:
        sys.exit(f"test-x10: {os.path.basename(args[0])} failed:\n{r.stdout}{r.stderr}")
    return r


def main():
    with tempfile.TemporaryDirectory() as tmp:
        tmp = os.path.realpath(tmp)
        env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, "cache"),
               "GOROOT": ROOT, "GOOS": GOOS, "GOARCH": GOARCH, "CGO_ENABLED": "0",
               "GOEXPERIMENT": "", "GODEBUG": ""}
        mod = os.path.join(tmp, "x10")
        os.makedirs(os.path.join(mod, "lib"))
        open(os.path.join(mod, "sgo.mod"), "w").write("module example.com/x10\n\nsgo " + SPEC_VERSION + "\n")
        open(os.path.join(mod, "lib", "lib.sgo"), "w").write(LIB)
        open(os.path.join(mod, "main.sgo"), "w").write(MAIN)

        # The driver's build is the reference configuration (and fills the TCB cache).
        run([os.path.join(TOOLS, "sgo"), "build", "-o", os.path.join(tmp, "prog-driver")], mod, env)
        tcb = glob.glob(os.path.join(tmp, "cache", "sgo-driver", "tcb", "*", "manifest.sha256"))
        if len(tcb) != 1:
            sys.exit("test-x10: TCB cache not found")
        cfg = {os.path.basename(a)[:-2].replace("%", "/"): a
               for a in glob.glob(os.path.join(os.path.dirname(tcb[0]), "*.a"))}

        def importcfg(name, extra):
            p = os.path.join(tmp, name + ".importcfg")
            with open(p, "w") as f:
                for k, v in sorted({**cfg, **extra}.items()):
                    f.write(f"packagefile {k}={v}\n")
            return p

        results = {}
        # -l (no inlining) is refused by the shipped compiler (W12 R3), so the only optimization
        # setting an S-Go program has is the default one; check that the refusal holds.
        rl = subprocess.run([os.path.join(TOOLS, "compile"), "-o", os.path.join(tmp, "l.a"), "-p", "example.com/x10/lib",
                             "-pack", "-complete", "-importcfg", importcfg("lib", {}), "-l", "lib/lib.sgo"],
                            cwd=mod, env=env, capture_output=True, text=True)
        if rl.returncode == 0 or "SGO_E_USAGE: compile flag -l is not admitted" not in rl.stderr:
            sys.exit(f"test-x10: compile -l was not refused:\n{rl.stdout}{rl.stderr}")
        for label, flags in (("inline", ["-m=2"]),):
            lib_a, main_a, exe = (os.path.join(tmp, f"{x}-{label}") for x in ("lib.a", "main.a", "prog"))
            rl = run([os.path.join(TOOLS, "compile"), "-o", lib_a, "-p", "example.com/x10/lib", "-pack",
                 "-complete", "-trimpath", mod + "=>", "-importcfg", importcfg("lib", {})] + flags +
                ["lib/lib.sgo"], mod, env)
            r = run([os.path.join(TOOLS, "compile"), "-o", main_a, "-p", "main", "-pack", "-complete",
                     "-trimpath", mod + "=>", "-importcfg", importcfg("main", {"example.com/x10/lib": lib_a})] +
                    flags + ["main.sgo"], mod, env)
            if label == "inline":
                report = rl.stdout + rl.stderr + r.stdout + r.stderr
                for fn in ("Check", "Nested", "Fail"):
                    if f"cannot inline {fn}: function too complex" not in report or f"inlining call to lib.{fn}" in report:
                        sys.exit(f"test-x10: unexpected inlining decision for lib.{fn}:\n{report}")
                for fn in ("Check.func1", "Nested.func2", "Fail.func1"):
                    if f"can inline {fn} " not in report:
                        sys.exit(f"test-x10: spawned closure {fn} is no longer inlinable:\n{report}")
            run([os.path.join(TOOLS, "link"), "-o", exe, "-importcfg",
                 importcfg("link", {"example.com/x10/lib": lib_a}), "-buildmode=exe", buildid(), main_a], tmp, env)
            results[label] = run([exe], tmp, {"PATH": "/usr/bin:/bin"}, check=False)
        results["driver"] = run([os.path.join(tmp, "prog-driver")], tmp, {"PATH": "/usr/bin:/bin"}, check=False)

        for label, r in results.items():
            if (r.returncode, r.stdout, r.stderr) != (2, WANT_STDOUT, WANT_STDERR):
                sys.exit(f"test-x10: {label}: exit {r.returncode}\nstdout:\n{r.stdout}stderr:\n{r.stderr}"
                         f"want exit 2\nstdout:\n{WANT_STDOUT}stderr:\n{WANT_STDERR}")
    print("test-x10: ok: cross-package nurseries behave identically with default optimization "
          "(nursery functions not inlined, spawned closures inlinable) and as built by the driver; -l is refused")


if __name__ == "__main__":
    main()
