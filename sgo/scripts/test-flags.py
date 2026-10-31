#!/usr/bin/env python3
"""test-flags.py - no flag, mode, or environment of the driver, compiler, or linker reopens stock
Go semantics (S01, S15; SGO-TOOL-001, SGO-TCB-002, SGO-BUILD-001).

Compiler: a file with a bare go statement and an unsafe import is compiled with each permissive
or legacy flag (-std, -+, -lang, -d=..., -complete omitted, GOEXPERIMENT and GODEBUG settings);
every variant reports the same S-Go diagnostics, and claiming a TCB package path (-p runtime)
is SGO_E_TCB_DIGEST. Driver: unknown and permissive flags are SGO_E_USAGE. Linker: an S-Go
program links only as an executable; other build modes, -X overrides, and linkname-check
switches are refused.
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
TOOLS = os.path.join(ROOT, "pkg", "tool", f"{GOOS}_{GOARCH}")

BAD = ('package main\n\nimport (\n\t"unsafe"\n\n\t"sgo"\n)\n\nfunc f() {}\n\n'
       'func main(root *sgo.Context) error {\n\tgo f()\n\t_ = unsafe.Sizeof(1)\n\treturn nil\n}\n')
GOOD = 'package main\n\nimport "sgo"\n\nfunc main(root *sgo.Context) error {\n\treturn nil\n}\n'
WANT_BAD = ["main.sgo:4:2: SGO_E_UNADMITTED", "main.sgo:12:2: SGO_E_UNSTRUCTURED_SPAWN"]


def run(args, cwd, env):
    return subprocess.run(args, cwd=cwd, env=env, capture_output=True, text=True)


def main():
    problems = []
    with tempfile.TemporaryDirectory() as tmp:
        tmp = os.path.realpath(tmp)
        env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, "cache"),
               "GOROOT": ROOT, "GOOS": GOOS, "GOARCH": GOARCH, "CGO_ENABLED": "0",
               "GOEXPERIMENT": "", "GODEBUG": ""}
        mod = os.path.join(tmp, "m")
        os.makedirs(mod)
        open(os.path.join(mod, "sgo.mod"), "w").write("module example.com/m\n\nsgo 0.1.0\n")
        open(os.path.join(mod, "main.sgo"), "w").write(GOOD)
        r = run([os.path.join(TOOLS, "sgo"), "build", "-o", os.path.join(tmp, "good")], mod, env)
        if r.returncode != 0:
            sys.exit(f"test-flags: reference build failed:\n{r.stdout}{r.stderr}")
        tcb = glob.glob(os.path.join(tmp, "cache", "sgo-driver", "tcb", "*", "complete"))
        cfgp = os.path.join(tmp, "importcfg")
        with open(cfgp, "w") as f:
            for a in sorted(glob.glob(os.path.join(os.path.dirname(tcb[0]), "*.a"))):
                f.write(f"packagefile {os.path.basename(a)[:-2].replace('%', '/')}={a}\n")

        # Compiler: permissive or legacy flags and environment do not change admission.
        open(os.path.join(mod, "bad.sgo.txt"), "w").write(BAD)
        bad = os.path.join(tmp, "bad")
        os.makedirs(bad)
        open(os.path.join(bad, "main.sgo"), "w").write(BAD)
        variants = [
            ("no flags", [], {}),
            ("-std", ["-std"], {}),
            ("-+ (compiling runtime)", ["-+"], {}),
            ("-lang=go1.21", ["-lang=go1.21"], {}),
            ("-d=checkptr=0", ["-d=checkptr=0"], {}),
            ("-complete omitted", None, {}),
            ("GOEXPERIMENT=noregabiargs", [], {"GOEXPERIMENT": "noregabiargs"}),
            ("GODEBUG=panicnil=1", [], {"GODEBUG": "panicnil=1"}),
        ]
        for label, flags, extra in variants:
            base = ["-o", os.path.join(tmp, "x.a"), "-p", "main", "-pack", "-importcfg", cfgp]
            args = base + (["-complete"] + flags if flags is not None else []) + ["main.sgo"]
            r = run([os.path.join(TOOLS, "compile")] + args, bad, {**env, **extra})
            out = r.stdout + r.stderr
            if r.returncode == 0 or not all(w in out for w in WANT_BAD):
                problems.append(f"compile {label}: exit {r.returncode}, want both S-Go denials:\n{out}")
        r = run([os.path.join(TOOLS, "compile"), "-o", os.path.join(tmp, "x.a"), "-p", "runtime", "-pack",
                 "-importcfg", cfgp, "-+", "-std", "main.sgo"], bad, env)
        if r.returncode == 0 or "SGO_E_TCB_DIGEST" not in r.stdout + r.stderr:
            problems.append(f"compile -p runtime: want SGO_E_TCB_DIGEST:\n{r.stdout}{r.stderr}")

        # Driver: no permissive flags.
        for flags in (["-gcflags=-l"], ["-ldflags=-X main.x=1"], ["-tags=go"], ["-compat"], ["-buildmode=c-shared"]):
            r = run([os.path.join(TOOLS, "sgo"), "build"] + flags + ["-o", os.path.join(tmp, "y")], mod, env)
            if r.returncode == 0 or "SGO_E_USAGE" not in r.stdout + r.stderr:
                problems.append(f"sgo build {' '.join(flags)}: want SGO_E_USAGE:\n{r.stdout}{r.stderr}")

        # Linker: an S-Go program links only as an executable, without overrides.
        main_a = os.path.join(tmp, "good.a")
        r = run([os.path.join(TOOLS, "compile"), "-o", main_a, "-p", "main", "-pack", "-complete",
                 "-importcfg", cfgp, "main.sgo"], mod, env)
        if r.returncode != 0:
            sys.exit(f"test-flags: compile of the valid program failed:\n{r.stdout}{r.stderr}")
        r = run([os.path.join(TOOLS, "link"), "-o", os.path.join(tmp, "ok"), "-importcfg", cfgp,
                 "-buildmode=exe", main_a], tmp, env)
        if r.returncode != 0:
            sys.exit(f"test-flags: plain link failed:\n{r.stdout}{r.stderr}")
        refused = [("-buildmode=c-shared", ["-buildmode=c-shared"]),
                   ("-buildmode=c-archive", ["-buildmode=c-archive"]),
                   ("-buildmode=plugin", ["-buildmode=plugin"]),
                   ("-X", ["-X", "runtime.buildVersion=stock"]),
                   ("-checklinkname=0", ["-checklinkname=0"]),
                   ("-linkmode=external", ["-linkmode=external"])]
        # On darwin every executable is PIE, so -buildmode=pie is the plain executable there.
        if GOOS != "darwin":
            refused.append(("-buildmode=pie", ["-buildmode=pie"]))
        for label, flags in refused:
            r = run([os.path.join(TOOLS, "link"), "-o", os.path.join(tmp, "z"), "-importcfg", cfgp] + flags +
                    [main_a], tmp, env)
            if r.returncode == 0 or "SGO_E_" not in r.stdout + r.stderr:
                problems.append(f"link {label}: exit {r.returncode}, want an S-Go refusal:\n{r.stdout}{r.stderr}")
    for p in problems:
        print("test-flags:", p, file=sys.stderr)
    if problems:
        sys.exit(f"test-flags: {len(problems)} problem(s)")
    print("test-flags: ok: compiler flags and environment do not change admission; TCB paths need "
          "pinned files; driver and linker refuse permissive flags and non-executable modes")


if __name__ == "__main__":
    main()
