#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
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

from sgotool import buildid

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
SPEC_VERSION = open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip()
GOOS = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
GOARCH = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[platform.machine()]
TOOLS = os.path.join(ROOT, "pkg", "tool", f"{GOOS}_{GOARCH}")

BAD = ('package main\n\nimport (\n\t"unsafe"\n\n\t"sgo"\n)\n\nfunc f() {}\n\n'
       'func main(root *sgo.Context) error {\n\tgo f()\n\t_ = unsafe.Sizeof(1)\n\treturn nil\n}\n')
GOOD = 'package main\n\nimport "sgo"\n\nfunc main(root *sgo.Context) error {\n\treturn nil\n}\n'
WANT_BAD = ["main.sgo:4:2: SGO_E_UNADMITTED", "main.sgo:12:2: SGO_E_UNSTRUCTURED_SPAWN"]


def run(args, cwd, env):
    return subprocess.run(args, cwd=cwd, env=env, capture_output=True, text=True)


def refused(r, out_file, want="SGO_E_USAGE"):
    """'' if the tool refused with want (None: any refusal) and wrote nothing, else the problem."""
    out = r.stdout + r.stderr
    if r.returncode == 0 or want is not None and want not in out:
        return f"exit {r.returncode}, want {want}:\n{out}"
    if out_file and os.path.exists(out_file):
        return f"refused but wrote {out_file}"
    return ""


def main():
    problems = []
    with tempfile.TemporaryDirectory() as tmp:
        tmp = os.path.realpath(tmp)
        env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, "cache"),
               "GOROOT": ROOT, "GOOS": GOOS, "GOARCH": GOARCH, "CGO_ENABLED": "0",
               "GOEXPERIMENT": "", "GODEBUG": ""}
        mod = os.path.join(tmp, "m")
        os.makedirs(mod)
        open(os.path.join(mod, "sgo.mod"), "w").write("module example.com/m\n\nsgo " + SPEC_VERSION + "\n")
        open(os.path.join(mod, "main.sgo"), "w").write(GOOD)
        r = run([os.path.join(TOOLS, "sgo"), "build", "-o", os.path.join(tmp, "good")], mod, env)
        if r.returncode != 0:
            sys.exit(f"test-flags: reference build failed:\n{r.stdout}{r.stderr}")
        tcb = glob.glob(os.path.join(tmp, "cache", "sgo-driver", "tcb", "*", "manifest.sha256"))
        cfgp = os.path.join(tmp, "importcfg")
        with open(cfgp, "w") as f:
            for a in sorted(glob.glob(os.path.join(os.path.dirname(tcb[0]), "*.a"))):
                f.write(f"packagefile {os.path.basename(a)[:-2].replace('%', '/')}={a}\n")

        # Compiler: permissive or legacy flags and environment do not change admission.
        open(os.path.join(mod, "bad.sgo.txt"), "w").write(BAD)
        bad = os.path.join(tmp, "bad")
        os.makedirs(bad)
        open(os.path.join(bad, "main.sgo"), "w").write(BAD)
        # Admitted command lines (the driver's flags) still report the S-Go denials.
        variants = [
            ("no flags", [], {}),
            ("-complete omitted", None, {}),
            ("GODEBUG=panicnil=1", [], {"GODEBUG": "panicnil=1"}),
            ("-sgotestmain", ["-sgotestmain"], {}),  # admits only sgo.StartTest (D69)
            ("-m=2 (diagnostic output only)", ["-m=2"], {}),
        ]
        for label, flags, extra in variants:
            base = ["-o", os.path.join(tmp, "x.a"), "-p", "main", "-pack", "-importcfg", cfgp]
            args = base + (["-complete"] + flags if flags is not None else []) + ["main.sgo"]
            r = run([os.path.join(TOOLS, "compile")] + args, bad, {**env, **extra})
            out = r.stdout + r.stderr
            if r.returncode == 0 or not all(w in out for w in WANT_BAD):
                problems.append(f"compile {label}: exit {r.returncode}, want both S-Go denials:\n{out}")
        # Every other flag or compiler setting is refused before anything is compiled (D77, W12 R3).
        compile_refused = [[f] for f in ("-std", "-+", "-lang=go1.21", "-d=checkptr=0", "-wb=false", "-B", "-N",
                                         "-l", "-race", "-spectre=all", "-dynlink", "-shared", "-d=ssa/check/off",
                                         "-smallframes", "-asan", "-msan", "-live", "-S")]
        compile_env = [{"GOEXPERIMENT": "noregabiargs"}, {"GOCOMPILEDEBUG": "loopvarhash=1"}]
        compile_env.append({"GOAMD64": "v3"} if GOARCH == "amd64" else {"GOARM64": "v8.1"})
        # Upstream's flag parsing already rejects these on some platforms, before S-Go's check runs.
        upstream_rejects = {"-spectre=all", "-asan", "-msan"}
        for flags, extra in [(f, {}) for f in compile_refused] + [([], e) for e in compile_env]:
            out_a = os.path.join(tmp, "refused.a")
            if os.path.exists(out_a):
                os.remove(out_a)
            r = run([os.path.join(TOOLS, "compile"), "-o", out_a, "-p", "main", "-pack", "-complete",
                     "-importcfg", cfgp] + flags + ["main.sgo"], mod, {**env, **extra})
            want = None if set(flags) & upstream_rejects else "SGO_E_USAGE"
            if msg := refused(r, out_a, want):
                problems.append(f"compile {' '.join(flags) or extra}: {msg}")
        r = run([os.path.join(TOOLS, "compile"), "-o", os.path.join(tmp, "x.a"), "-p", "runtime", "-pack",
                 "-importcfg", cfgp, "-std", "main.sgo"], bad, env)  # -+ is refused first (legacy-audit.py)
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
                 "-buildmode=exe", buildid(), main_a], tmp, env)
        if r.returncode != 0:
            sys.exit(f"test-flags: plain link failed:\n{r.stdout}{r.stderr}")
        # D69: -sgotestmain compiles only package main, and such a package links only as the main
        # package of a test binary; a test binary's main must be one.
        r = run([os.path.join(TOOLS, "compile"), "-sgotestmain", "-o", os.path.join(tmp, "lib.a"), "-p", "example.com/m/lib",
                 "-pack", "-complete", "-importcfg", cfgp, "main.sgo"], mod, env)
        if r.returncode == 0 or "SGO_E_USAGE: -sgotestmain" not in r.stdout + r.stderr:
            problems.append(f"compile -sgotestmain -p example.com/m/lib: want SGO_E_USAGE:\n{r.stdout}{r.stderr}")
        tm_a = os.path.join(tmp, "testmain.a")
        r = run([os.path.join(TOOLS, "compile"), "-sgotestmain", "-o", tm_a, "-p", "main", "-pack", "-complete",
                 "-importcfg", cfgp, "main.sgo"], mod, env)
        if r.returncode != 0:
            problems.append(f"compile -sgotestmain -p main failed:\n{r.stdout}{r.stderr}")
        for label, flags, a in (("testmain without -sgotest", [], tm_a), ("-sgotest with a production main", ["-sgotest"], main_a)):
            r = run([os.path.join(TOOLS, "link"), "-o", os.path.join(tmp, "t"), "-importcfg", cfgp, "-buildmode=exe",
                     buildid("test" if flags else None)] + flags + [a], tmp, env)
            if r.returncode == 0 or "SGO_E_SPEC_MISMATCH" not in r.stdout + r.stderr:
                problems.append(f"link {label}: want SGO_E_SPEC_MISMATCH:\n{r.stdout}{r.stderr}")
        refused_link = [("-buildmode=c-shared", ["-buildmode=c-shared"]),
                   ("-buildmode=c-archive", ["-buildmode=c-archive"]),
                   ("-buildmode=plugin", ["-buildmode=plugin"]),
                   ("-X", ["-X", "runtime.buildVersion=stock"]),
                   ("-checklinkname=0", ["-checklinkname=0"]),
                   ("-linkmode=external", ["-linkmode=external"]),
                   ("no -buildid", "nobuildid"),
                   ("-buildid not S-Go metadata", ["-buildid=abc"]),
                   ("-sgolab with production -buildid", ["-sgolab"]),
                   ("-buildid claiming a drain budget", ["-buildid=" + buildid()[len("-buildid="):].replace("drain=none", "drain=500")]),
                   ("-buildid claiming grants", ["-buildid=" + buildid()[len("-buildid="):].replace("grants=none", "grants=clock")])]
        refused_link += [(f, [f]) for f in ("-f", "-s", "-w", "-strictdups=0", "-randlayout=1", "-k=x",
                                             "-installsuffix=x", "-debugnosplit", "-race", "-msan", "-asan")]
        refused_link.append(("-R 0x2000", ["-R", "0x2000"]))
        # On darwin every executable is PIE, so -buildmode=pie is the plain executable there.
        if GOOS != "darwin":
            refused_link.append(("-buildmode=pie", ["-buildmode=pie"]))
        for label, flags in refused_link:
            out = os.path.join(tmp, "z")
            if os.path.exists(out):
                os.remove(out)
            if flags == "nobuildid":
                args = []
            elif any(f.startswith("-buildid") for f in flags):
                args = flags
            else:
                args = [buildid()] + flags  # a production -buildid, so -sgolab contradicts it
            r = run([os.path.join(TOOLS, "link"), "-o", out, "-importcfg", cfgp] + args + [main_a], tmp, env)
            if msg := refused(r, out, "SGO_E_"):
                problems.append(f"link {label}: {msg}")
        # Harness self-test (D79): an admitted flag treated as "must be refused" must be reported, so
        # the refusal checks cannot pass vacuously.
        out = os.path.join(tmp, "selftest")
        r = run([os.path.join(TOOLS, "link"), "-o", out, "-importcfg", cfgp, "-buildmode=exe", buildid(), main_a], tmp, env)
        if not refused(r, out):
            problems.append("self-test: an admitted link command line was reported as refused; the harness is vacuous")
    for p in problems:
        print("test-flags:", p, file=sys.stderr)
    if problems:
        sys.exit(f"test-flags: {len(problems)} problem(s)")
    print("test-flags: ok: compiler flags and environment do not change admission; TCB paths need "
          "pinned files; driver and linker refuse permissive flags and non-executable modes")


if __name__ == "__main__":
    main()
