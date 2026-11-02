#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""test-isolation.py — W10 parts of T01, T02, S01, S15, S19 (release tooling).

Runs against a toolchain (stage-0 by default, or a release's bin/sgo with --sgo):

  T01  a package compiled for the lab cannot be imported by a production package, nor the reverse
       (SGO_E_SPEC_MISMATCH before linking). Other mixtures (spec, admission, ABI, stale and
       foreign objects, test mains, edited TCB files with a warm cache) are test-identity,
       test-flags, and test-tcb.
  T02  a production binary run with lab-style environment (GOLABSEED, SGO_LAB_SEED, SGOLAB, ...)
       behaves exactly as without it and writes no trace; the lab is reachable only by sgo lab.
  S01, S15, S19 (new W10 tools)  sgo check and sgo test refuse permissive flags (SGO_E_USAGE),
       refuse .go inputs (SGO_E_UNADMITTED), and reject a denied construct the same way with
       GOFLAGS, GOEXPERIMENT, and GODEBUG set; sgo fmt refuses a .go file.

    test-isolation.py [--sgo PATH]
"""
import glob
import os
import platform
import subprocess
import sys
import tempfile

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
GOOS = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
GOARCH = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[platform.machine()]
SPEC = open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip()

LIB = "package lib\n\nfunc Two() int {\n\treturn 2\n}\n"
MAIN = ('package main\n\nimport (\n\t"sgo"\n\n\t"example.com/iso/lib"\n)\n\n'
        'func main(root *sgo.Context) error {\n\tif lib.Two() != 2 {\n\t\treturn sgo.NewError("bad")\n\t}\n'
        '\treturn root.Stdout().WriteString("ok\\n")\n}\n')
BAD = "package lib\n\nfunc f() {}\n\nfunc G() {\n\tgo f()\n}\n"


def main():
    args = sys.argv[1:]
    sgo = os.path.join(ROOT, "pkg", "tool", f"{GOOS}_{GOARCH}", "sgo")
    if args[:1] == ["--sgo"]:
        sgo = args[1]
    tools = os.path.dirname(os.path.realpath(sgo))
    problems = []

    def check(cond, msg):
        if not cond:
            problems.append(msg)

    with tempfile.TemporaryDirectory() as tmp:
        tmp = os.path.realpath(tmp)
        cache = os.path.join(tmp, "cache")
        env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": cache}
        mod = os.path.join(tmp, "iso")
        os.makedirs(os.path.join(mod, "lib"))
        open(os.path.join(mod, "sgo.mod"), "w").write(f"module example.com/iso\n\nsgo {SPEC}\n")
        open(os.path.join(mod, "lib", "lib.sgo"), "w").write(LIB)
        open(os.path.join(mod, "main.sgo"), "w").write(MAIN)

        def run(argv, cwd=mod, extra=None):
            return subprocess.run(argv, cwd=cwd, env={**env, **(extra or {})}, capture_output=True, text=True, timeout=600)

        r = run([sgo, "build", "-o", os.path.join(tmp, "prog")])
        if r.returncode != 0:
            sys.exit(f"test-isolation: reference build failed: {r.stderr}")

        # T01: configurations never mix (D41, SGO-IMP-002).
        tcb = glob.glob(os.path.join(cache, "sgo-driver", "tcb", "*", "manifest.sha256"))
        cfg = os.path.join(tmp, "importcfg")
        with open(cfg, "w") as f:
            for a in sorted(glob.glob(os.path.join(os.path.dirname(tcb[0]), "*.a"))):
                f.write(f"packagefile {os.path.basename(a)[:-2].replace('%', '/')}={a}\n")
        compile_ = os.path.join(tools, "compile")
        for lib_flags, main_flags, label in (([ "-sgolab"], [], "lab library in a production main"),
                                             ([], ["-sgolab"], "production library in a lab main")):
            liba = os.path.join(tmp, "lib.a")
            r = run([compile_] + lib_flags + ["-o", liba, "-p", "example.com/iso/lib", "-pack", "-complete",
                     "-importcfg", cfg, "-trimpath", mod + "=>", "lib/lib.sgo"])
            check(r.returncode == 0, f"T01 {label}: library compile failed: {r.stdout}{r.stderr}")
            cfg2 = os.path.join(tmp, "importcfg2")
            open(cfg2, "w").write(open(cfg).read() + f"packagefile example.com/iso/lib={liba}\n")
            r = run([compile_] + main_flags + ["-o", os.path.join(tmp, "main.a"), "-p", "main", "-pack", "-complete",
                     "-importcfg", cfg2, "-trimpath", mod + "=>", "main.sgo"])
            out = r.stdout + r.stderr
            check(r.returncode != 0 and "SGO_E_SPEC_MISMATCH" in out and "configuration" in out,
                  f"T01 {label}: want SGO_E_SPEC_MISMATCH at the import, got exit {r.returncode}: {out[:300]}")

        # T02: a production binary ignores lab-style environment and writes no trace (D41).
        base = run([os.path.join(tmp, "prog")])
        labenv = {"GOLABSEED": "7", "SGO_LAB_SEED": "7", "SGOLAB": "1", "SGO_LAB": "1", "GODEBUG": "sgolab=1"}
        r = run([os.path.join(tmp, "prog")], extra=labenv)
        check((r.returncode, r.stdout, r.stderr) == (base.returncode, base.stdout, base.stderr) == (0, "ok\n", ""),
              f"T02: production binary with lab environment: {r.returncode} {r.stdout!r} {r.stderr!r}")
        check(not any(f.endswith(".jsonl") for f in os.listdir(mod)), "T02: a production run wrote a trace")
        r = run([sgo, "lab", "run", "-seed", "7", os.path.join(tmp, "prog")])
        check(r.returncode != 0, "T02: sgo lab run accepted a production binary path")

        # S01, S15, S19 for the W10 tools.
        for cmd, flag in (("check", "-gcflags=-l"), ("check", "-compat"), ("test", "-tags=go"), ("test", "-ldflags=-X x=1"),
                          ("test", "-exec=/bin/sh"), ("check", "-toolexec=/bin/true")):
            r = run([sgo, cmd, flag, "./..."])
            check(r.returncode == 2 and "SGO_E_USAGE" in r.stderr, f"S01: sgo {cmd} {flag}: want SGO_E_USAGE, got {r.returncode} {r.stderr[:200]!r}")
        gomod = os.path.join(tmp, "gomod")
        os.makedirs(gomod)
        open(os.path.join(gomod, "sgo.mod"), "w").write(f"module example.com/gomod\n\nsgo {SPEC}\n")
        open(os.path.join(gomod, "x.sgo"), "w").write("package x\n\nfunc One() int {\n\treturn 1\n}\n")
        open(os.path.join(gomod, "legacy.go"), "w").write("package x\n\nfunc Legacy() {}\n")
        for cmd in ("check", "test", "build"):
            r = run([sgo, cmd, "."], cwd=gomod)
            check(r.returncode != 0 and "SGO_E_UNADMITTED" in r.stderr and "legacy.go" in r.stderr,
                  f"S15/S19: sgo {cmd} with a .go file: want SGO_E_UNADMITTED, got {r.returncode} {r.stderr[:200]!r}")
        r = run([sgo, "fmt", os.path.join(gomod, "legacy.go")], cwd=gomod)
        check(r.returncode != 0, "S19: sgo fmt accepted a .go file")
        os.remove(os.path.join(gomod, "legacy.go"))
        open(os.path.join(gomod, "x.sgo"), "w").write(BAD.replace("package lib", "package x"))
        open(os.path.join(gomod, "x_test.sgo"), "w").write("package x\n\nimport \"sgo\"\n\nfunc TestG(t *sgo.T) error {\n\tG()\n\treturn nil\n}\n")
        want = None
        for extra in ({}, {"GOFLAGS": "-gcflags=-l", "GOEXPERIMENT": "noregabiargs", "GODEBUG": "panicnil=1", "GO111MODULE": "off"}):
            for cmd in ("check", "test"):
                r = run([sgo, cmd, "."], cwd=gomod, extra=extra)
                diag = [l for l in r.stderr.splitlines() if "SGO_E_" in l]
                check(r.returncode != 0 and any("SGO_E_UNSTRUCTURED_SPAWN" in l for l in diag),
                      f"S01/S19: sgo {cmd} {extra or ''}: want SGO_E_UNSTRUCTURED_SPAWN, got {r.returncode} {r.stderr[:200]!r}")
                if want is None:
                    want = diag
                check(diag == want, f"S01: sgo {cmd} {extra or ''}: diagnostics {diag} differ from {want}")
    for p in problems:
        print("test-isolation: " + p)
    if problems:
        sys.exit(f"test-isolation: {len(problems)} problem(s)")
    print("test-isolation: ok: T01 lab/production packages never mix; T02 production binaries ignore lab environment; "
          "S01/S15/S19 sgo check and sgo test refuse permissive flags and .go inputs and reject the same way under GOFLAGS/GOEXPERIMENT/GODEBUG")


if __name__ == "__main__":
    main()
