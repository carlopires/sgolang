#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""test-check.py — sgo check agrees with sgo build (W10, D63, T03).

For every conformance case, sgo check and sgo build run on a copy of the case:
  - both succeed, or both fail with byte-identical diagnostics (codes, positions, messages);
  - sgo check writes no binary and no file into the case;
  - sgo check -json reports the same diagnostics as JSON lines (file, line, column, code).
Also: sgo check ./... covers every package of a multi-package module, and a dependency that does
not check stops its dependents.

    test-check.py [--sgo PATH] [case ...]
"""
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
CASES = os.path.join(ROOT, "sgo", "testdata", "conformance")
DIAG = re.compile(r"^(.+?):(\d+):(\d+): ([A-Z][A-Z0-9_]+): (.*)$")


def snapshot(d):
    out = {}
    for root, _, files in os.walk(d):
        for f in files:
            p = os.path.join(root, f)
            out[os.path.relpath(p, d)] = os.lstat(p).st_mtime_ns
    return out


def main():
    args = sys.argv[1:]
    sgo = os.path.join(ROOT, "pkg", "tool", "darwin_arm64", "sgo")
    if args[:1] == ["--sgo"]:
        sgo, args = args[1], args[2:]
    names = args or sorted(os.listdir(CASES))
    env = {"PATH": "/usr/bin:/bin", "HOME": os.environ.get("HOME", "/tmp")}
    problems, n = [], 0
    for name in names:
        src = os.path.join(CASES, name)
        e = json.load(open(os.path.join(src, "expect.json")))
        if e.get("procedure") or e.get("build_args") or e.get("build_env"):
            continue  # build-procedure cases (rebuild, flags) are covered by their own tests
        if e.get("kind") == "test":
            continue  # library packages under sgo test: there is no program to build
        n += 1
        with tempfile.TemporaryDirectory() as tmp:
            d = os.path.join(tmp, name)
            shutil.copytree(src, d, symlinks=True)
            before = snapshot(d)
            c = subprocess.run([sgo, "check"], cwd=d, env=env, capture_output=True, text=True, timeout=120)
            if snapshot(d) != before:
                problems.append(f"{name}: sgo check changed the case directory")
            b = subprocess.run([sgo, "build", "-o", os.path.join(tmp, "prog")], cwd=d, env=env,
                               capture_output=True, text=True, timeout=120)
            if (c.returncode == 0) != (b.returncode == 0):
                problems.append(f"{name}: check exit {c.returncode}, build exit {b.returncode}: {c.stderr.strip()[:160]!r} / {b.stderr.strip()[:160]!r}")
                continue
            if c.stderr != b.stderr:
                problems.append(f"{name}: diagnostics differ:\n  check {c.stderr.strip()[:300]!r}\n  build {b.stderr.strip()[:300]!r}")
                continue
            if c.stdout:
                problems.append(f"{name}: sgo check wrote to stdout: {c.stdout[:120]!r}")
            j = subprocess.run([sgo, "check", "-json"], cwd=d, env=env, capture_output=True, text=True, timeout=120)
            want = [m.groups()[:4] for m in map(DIAG.match, b.stderr.splitlines()) if m]
            got = []
            for line in j.stdout.splitlines():
                rec = json.loads(line)
                if "code" in rec and "line" in rec:
                    got.append((rec["file"], str(rec["line"]), str(rec["column"]), rec["code"]))
            if got != want or (j.returncode == 0) != (c.returncode == 0):
                problems.append(f"{name}: -json {got} (exit {j.returncode}), want {want} (exit {c.returncode})")
    # A module with two packages: ./... checks both; a broken dependency stops its dependent.
    with tempfile.TemporaryDirectory() as tmp:
        os.makedirs(os.path.join(tmp, "lib"))
        open(os.path.join(tmp, "sgo.mod"), "w").write("module example.com/m\n\nsgo %s\n" % open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip())
        open(os.path.join(tmp, "lib", "lib.sgo"), "w").write("package lib\n\nfunc Twice(n int) int {\n\treturn n + n\n}\n")
        open(os.path.join(tmp, "main.sgo"), "w").write('package main\n\nimport (\n\t"example.com/m/lib"\n\t"sgo"\n)\n\nfunc main(root *sgo.Context) error {\n\treturn root.Stdout().WriteString("x" + lib.Name())\n}\n')
        r = subprocess.run([sgo, "check", "./..."], cwd=tmp, env=env, capture_output=True, text=True, timeout=120)
        if r.returncode == 0 or "lib.Name" not in r.stderr and "Name" not in r.stderr:
            problems.append(f"multi-package: undefined lib.Name not reported: exit {r.returncode} {r.stderr!r}")
        open(os.path.join(tmp, "lib", "lib.sgo"), "w").write("package lib\n\nfunc Name() string {\n\treturn \"y\"\n}\n")
        r = subprocess.run([sgo, "check", "./..."], cwd=tmp, env=env, capture_output=True, text=True, timeout=120)
        if r.returncode != 0 or r.stderr:
            problems.append(f"multi-package: clean module: exit {r.returncode} {r.stderr!r}")
        open(os.path.join(tmp, "lib", "lib.sgo"), "w").write("package lib\n\nfunc Name() string {\n\treturn 1\n}\n")
        r = subprocess.run([sgo, "check", "./..."], cwd=tmp, env=env, capture_output=True, text=True, timeout=120)
        if r.returncode == 0 or not r.stderr.startswith("lib/lib.sgo:"):
            problems.append(f"multi-package: broken dependency: exit {r.returncode} {r.stderr!r}")
        if os.path.exists(os.path.join(tmp, "m")) or os.path.exists(os.path.join(tmp, "main")):
            problems.append("multi-package: sgo check wrote a binary")
    for p in problems:
        print("test-check:", p)
    print(f"test-check: {n} cases, check and build agree; {len(problems)} problem(s)")
    sys.exit(1 if problems else 0)


main()
