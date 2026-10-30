#!/usr/bin/env python3
"""conformance-compile.py — W03 progress meter: run the conformance *reject* cases directly against the
stage-0 compiler (no sgo driver yet). Eligible cases: kind "reject", all sources in the case root,
no build_args/procedure/build_env. Each case's files are compiled as package main with an importcfg
for the compiled sgo package; diagnostics are compared in order (code and exact position).

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


def eligible(d, e):
    if e.get("kind") != "reject" or e.get("build_args") or e.get("procedure") or e.get("build_env"):
        return False
    if any(os.path.isdir(os.path.join(d, x)) for x in os.listdir(d)):
        return False
    return any(f.endswith(".sgo") for f in os.listdir(d))


def main():
    verbose = "-v" in sys.argv
    only = [a for a in sys.argv[1:] if not a.startswith("-")]
    if not os.path.exists(COMPILE):
        sys.exit(f"no stage-0 compiler at {COMPILE}; run sgo/scripts/build-stage0.sh")
    cases = conf.load_cases()
    passed, failed, skipped = [], [], []
    with tempfile.TemporaryDirectory() as tmp:
        sgo_o = os.path.join(tmp, "sgo.o")
        subprocess.check_call([COMPILE, "-p", "sgo", "-std", "-o", sgo_o, os.path.join(ROOT, "src", "sgo", "sgo.go")])
        cfg = os.path.join(tmp, "importcfg")
        open(cfg, "w").write(f"packagefile sgo={sgo_o}\n")
        for name, d in cases.items():
            if only and name not in only:
                continue
            e = json.load(open(os.path.join(d, "expect.json")))
            if not eligible(d, e):
                skipped.append(name)
                continue
            files = sorted(f for f in os.listdir(d) if f.endswith(".sgo"))
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
    print(f"conformance-compile: {len(passed)} passed, {len(failed)} failed, {len(skipped)} not eligible "
          f"(of {len(cases)} cases; eligible = flat reject cases without driver flags)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
