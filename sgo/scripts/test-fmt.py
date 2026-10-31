#!/usr/bin/env python3
"""test-fmt.py — executable test of sgo fmt (SGO-TOOL-001).

Formats a copy of every conformance case and checks that sgo fmt
  - either formats a file or refuses it with SGO_E_SYNTAX (never anything else),
  - is idempotent (a second run changes nothing),
  - keeps every nursery statement,
  - preserves meaning: each run case still builds and runs with the expected exit status and
    stdout after formatting.

    test-fmt.py
"""
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
PLAT = {"Darwin": "darwin", "Linux": "linux"}[os.uname().sysname] + "_" + \
    {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[os.uname().machine]
SGO = os.path.join(ROOT, "pkg", "tool", PLAT, "sgo")
CASES = os.path.join(ROOT, "sgo", "testdata", "conformance")
NURSERY = re.compile(rb"\bnursery\s*(\(\s*\w+\s*\))?\s*\{")


def main():
    pending = json.load(open(os.path.join(ROOT, "sgo", "testdata", "conformance-pending.json")))
    formatted = refused = ran = 0
    problems = []
    with tempfile.TemporaryDirectory() as tmp:
        env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, "cache")}
        for name in sorted(os.listdir(CASES)):
            src = os.path.join(CASES, name)
            if not os.path.isdir(src):
                continue
            d = os.path.join(tmp, name)
            shutil.copytree(src, d)
            files = sorted(os.path.relpath(os.path.join(r, f), d) for r, _, fs in os.walk(d) for f in fs if f.endswith(".sgo"))
            for f in files:
                p = os.path.join(d, f)
                before = open(p, "rb").read()
                r = subprocess.run([SGO, "fmt", f], cwd=d, env=env, capture_output=True, text=True)
                if r.returncode != 0:
                    if "SGO_E_SYNTAX" not in r.stderr or open(p, "rb").read() != before:
                        problems.append(f"{name}/{f}: refused without SGO_E_SYNTAX or changed: {r.stderr.strip()}")
                    refused += 1
                    continue
                once = open(p, "rb").read()
                subprocess.run([SGO, "fmt", f], cwd=d, env=env, capture_output=True)
                if open(p, "rb").read() != once:
                    problems.append(f"{name}/{f}: not idempotent")
                if len(NURSERY.findall(before)) != len(NURSERY.findall(once)):
                    problems.append(f"{name}/{f}: nursery statements changed")
                formatted += 1
            e = json.load(open(os.path.join(d, "expect.json")))
            if e["kind"] != "run" or name in pending or e.get("build_args") or e.get("procedure") or e.get("expect_watchdog"):
                continue
            exe = os.path.join(tmp, name + ".prog")
            b = subprocess.run([SGO, "build", "-o", exe], cwd=d, env=env, capture_output=True, text=True)
            if b.returncode != 0:
                problems.append(f"{name}: formatted case no longer builds: {b.stderr.strip()[:200]}")
                continue
            try:
                r = subprocess.run([exe], cwd=d, env=env, capture_output=True, text=True, timeout=e.get("timeout_ms", 10000) / 1000)
            except subprocess.TimeoutExpired:
                problems.append(f"{name}: formatted case timed out")
                continue
            outs = e.get("stdout_any_of", [e.get("stdout")])
            if r.returncode != e["exit"] or r.stdout not in outs:
                problems.append(f"{name}: formatted case behaves differently: exit {r.returncode}, stdout {r.stdout!r}")
            ran += 1
    for p in problems:
        print(f"test-fmt: {p}")
    print(f"test-fmt: {formatted} files formatted, {refused} refused (syntax), {ran} formatted run cases rebuilt and rerun, {len(problems)} problem(s)")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
