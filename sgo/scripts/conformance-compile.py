#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""conformance-compile.py — W03 progress meter: run the conformance *reject* cases directly against the
stage-0 compiler (no sgo driver yet). Eligible cases: kind "reject", all sources in the case root,
no build_args/procedure/build_env. Each case's files are compiled as package main with an importcfg
for the compiled sgo package; diagnostics are compared in order (code and exact position).

It also guards against false positives: every flat *run* case must compile with no diagnostic other
than the gates of later W03 layers (LATER_LAYERS), and must exit 0 (an internal compiler error
without a position also counts). Any other outcome on a run case is a failure and makes the exit
status nonzero.

    conformance-compile.py [-v] [case ...]
"""

import json
import os
import re
import subprocess
import sys
import tempfile

ROOT = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
sys.path.insert(0, os.path.join(ROOT, "sgo", "scripts"))
import importlib.util

spec = importlib.util.spec_from_file_location("conformance", os.path.join(ROOT, "sgo", "scripts", "conformance.py"))
conf = importlib.util.module_from_spec(spec)
spec.loader.exec_module(conf)

PLAT = {"Darwin": "darwin", "Linux": "linux"}[os.uname().sysname] + "_" + {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[os.uname().machine]
COMPILE = os.path.join(ROOT, "pkg", "tool", PLAT, "compile")
DIAG = re.compile(r"^(\S+:\d+:\d+): (SGO_E_[A-Z_]+|[^:]*?):", re.M)
LINE = re.compile(r"^\S+:\d+:\d+: .*$", re.M)
# Diagnostics of features a later W03 layer implements (multi-package builds, layer 5).
LATER_LAYERS = ("could not import",)


def eligible(d, e, kind="reject"):
    if e.get("kind") != kind or e.get("build_args") or e.get("procedure") or e.get("build_env"):
        return False
    if any(os.path.isdir(os.path.join(d, x)) for x in os.listdir(d)):
        return False
    # Diagnostics in non-source files (sgo.mod, .s files) come from the sgo driver (W03 layer 5).
    if any(not conf.position(d, dg).split(":")[0].endswith(".sgo") for dg in e.get("diagnostics", [])):
        return False
    # Import cycles are found in the driver's module graph (SGO-IMP-001), which knows the module path.
    if any(dg.get("code") == "SGO_E_IMPORT_CYCLE" for dg in e.get("diagnostics", [])):
        return False
    return any(f.endswith(".sgo") for f in os.listdir(d))


def main():
    verbose = "-v" in sys.argv
    only = [a for a in sys.argv[1:] if not a.startswith("-")]
    if not os.path.exists(COMPILE):
        sys.exit(f"no stage-0 compiler at {COMPILE}; run sgo/scripts/build-stage0.sh")
    cases = conf.load_cases()
    passed, failed, skipped = [], [], []
    clean, spurious = [], []
    with tempfile.TemporaryDirectory() as tmp:
        sgo_o = os.path.join(tmp, "sgo.o")
        sgo_dir = os.path.join(ROOT, "src", "sgo")
        sgo_files = sorted(os.path.join(sgo_dir, f) for f in os.listdir(sgo_dir) if f.endswith(".go") and not f.endswith("_test.go"))
        subprocess.check_call([COMPILE, "-p", "sgo", "-std", "-o", sgo_o] + sgo_files)
        cfg = os.path.join(tmp, "importcfg")
        open(cfg, "w").write(f"packagefile sgo={sgo_o}\n")
        for name, d in cases.items():
            if only and name not in only:
                continue
            e = json.load(open(os.path.join(d, "expect.json")))
            # _test.sgo files are part of a package only under sgo test (SGO-TEST-002).
            files = sorted(f for f in os.listdir(d) if f.endswith(".sgo") and not f.endswith("_test.sgo")) if os.path.isdir(d) else []
            if eligible(d, e, "run"):
                r = subprocess.run([COMPILE, "-p", "main", "-importcfg", cfg, "-o", os.path.join(tmp, "out.o")] + files,
                                   cwd=d, capture_output=True, text=True)
                text = r.stdout + r.stderr
                bad = [l for l in LINE.findall(text) if not any(g in l for g in LATER_LAYERS)]
                if r.returncode != 0 and not bad and not any(g in text for g in LATER_LAYERS):
                    bad = [f"exit status {r.returncode}: " + " | ".join(text.strip().splitlines()[:3])]
                (spurious.append((name, bad[:2])) if bad else clean.append(name))
                continue
            if not eligible(d, e):
                skipped.append(name)
                continue
            r = subprocess.run([COMPILE, "-p", "main", "-importcfg", cfg, "-o", os.path.join(tmp, "out.o")] + files,
                               cwd=d, capture_output=True, text=True)
            got = [(m.group(1), m.group(2)) for m in DIAG.finditer(r.stdout + r.stderr)]  # the compiler reports on stdout
            want = [(conf.position(d, dg), dg["code"]) for dg in e["diagnostics"]]
            if r.returncode != 0 and got == want:
                passed.append(name)
            else:
                failed.append((name, got[:3], want))
    for name, got, want in failed:
        if verbose:
            print(f"FAIL {name}: got {got} want {want}")
    for name, bad in spurious:
        print(f"SPURIOUS {name}: {bad}")
    print(f"conformance-compile: {len(passed)} passed, {len(failed)} failed, {len(skipped)} not eligible "
          f"(of {len(cases)} cases; eligible = flat reject cases without driver flags or driver-level diagnostics)")
    print(f"conformance-compile: run cases: {len(clean)} clean, {len(spurious)} with spurious diagnostics")
    return 1 if spurious else 0


if __name__ == "__main__":
    sys.exit(main())
