#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""test-std.py — sgo test of every standard-library package (SGO-IMP-004, D110).

Each package under src/sgostd is copied, with its _test.sgo files, into a scratch module and run
with `sgo test -v ./...`: the same source, compiled and tested under the same rules as user code.
Its own std/... imports still resolve to the installed standard library. Every package must have
at least one test, and every test must pass.
"""

import os
import platform
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
STD = os.path.join(ROOT, "src", "sgostd")
GOOS = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
GOARCH = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[platform.machine()]
SGO = os.path.join(ROOT, "pkg", "tool", f"{GOOS}_{GOARCH}", "sgo")
VERSION = open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip()


def packages():
    for d, _, files in sorted(os.walk(STD)):
        if any(f.endswith(".sgo") and not f.endswith("_test.sgo") for f in files):
            yield os.path.relpath(d, STD).replace(os.sep, "/")


def main():
    total = 0
    pkgs = list(packages())
    if not pkgs:
        sys.exit("test-std: no std packages")
    with tempfile.TemporaryDirectory() as tmp:
        env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, "cache")}
        for p in pkgs:
            mod = os.path.join(tmp, p.replace("/", "_"))
            dst = os.path.join(mod, p)
            os.makedirs(dst)
            src = os.path.join(STD, p)
            for f in os.listdir(src):
                if f.endswith(".sgo"):
                    shutil.copy2(os.path.join(src, f), dst)
            open(os.path.join(mod, "sgo.mod"), "w").write(f"module example.com/stdtest\n\nsgo {VERSION}\n")
            r = subprocess.run([SGO, "test", "-v", "./..."], cwd=mod, env=env, capture_output=True, text=True, timeout=300)
            passed = re.findall(r"^--- PASS: (\S+)", r.stdout, re.M)
            failed = re.findall(r"^--- FAIL: (\S+)", r.stdout, re.M)
            if r.returncode != 0 or failed or not passed:
                sys.exit(f"test-std: std/{p}: exit {r.returncode}, passed {passed}, failed {failed}\n{r.stdout[-1500:]}{r.stderr[-1500:]}")
            print(f"test-std: std/{p}: {len(passed)} tests passed")
            total += len(passed)
    print(f"test-std: ok: {len(pkgs)} package(s), {total} tests")


if __name__ == "__main__":
    main()
