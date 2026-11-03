#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""W13 review V3: compiled S-Go code reaches maps only through the ordered map (SGO-COMP-006).

Builds the W13 acceptance programs and every comp-map-* conformance case that builds, disassembles
every function of the module's packages, and requires that they call no entry point of Go's
randomized map (runtime.makemap*, mapaccess*, mapassign*, mapdelete*, mapIterStart, mapIterNext,
mapclear) and that, over all programs, the S-Go entry points (runtime.sgomap*, sgomakemap) are
called. A Go map handed an *sgoMap (or the reverse) would corrupt memory, not fail to compile, so
the check is on the object code.
"""

import json
import os
import platform
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
GOOS = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
GOARCH = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}[platform.machine()]
SGO = os.path.join(ROOT, "pkg", "tool", f"{GOOS}_{GOARCH}", "sgo")
CONF = os.path.join(ROOT, "sgo", "testdata", "conformance")

GO_MAP = re.compile(r"\b(?:CALL|BL|JMP|B)\s+runtime\.(makemap\w*|mapaccess\w*|mapassign\w*|mapdelete\w*|"
                    r"mapIterStart|mapIterNext|mapclear)\b")
SGO_MAP = re.compile(r"\b(?:CALL|BL|JMP|B)\s+runtime\.(sgomap\w+|sgomakemap)\b")


def run(args, cwd, env, check=True):
    r = subprocess.run(args, cwd=cwd, env=env, capture_output=True, text=True)
    if check and r.returncode != 0:
        sys.exit(f"test-map-lowering: {' '.join(args[:3])} failed:\n{r.stdout}{r.stderr}")
    return r


def cases():
    names = sorted(n for n in os.listdir(CONF) if n.startswith(("w13-p", "comp-map-")))
    for n in names:
        e = json.load(open(os.path.join(CONF, n, "expect.json")))
        if e.get("kind") == "run":
            yield n


def main():
    boot = subprocess.run([os.path.join(ROOT, "sgo", "scripts", "fetch-bootstrap.sh")], capture_output=True,
                          text=True, check=True).stdout.strip()
    used = set()
    checked = 0
    with tempfile.TemporaryDirectory() as tmp:
        tmp = os.path.realpath(tmp)
        env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, "cache"), "GOROOT": ROOT,
               "GOOS": GOOS, "GOARCH": GOARCH, "CGO_ENABLED": "0", "GOEXPERIMENT": "", "GODEBUG": ""}
        benv = {"HOME": tmp, "PATH": "/usr/bin:/bin", "GOROOT": boot, "GOCACHE": os.path.join(tmp, "gocache")}
        for n in cases():
            mod = os.path.join(tmp, n)
            shutil.copytree(os.path.join(CONF, n), mod)
            path = re.search(r"^module\s+(\S+)", open(os.path.join(mod, "sgo.mod")).read(), re.M).group(1)
            prog = os.path.join(tmp, n + ".bin")
            run([SGO, "build", "-o", prog, "."], mod, env)
            dis = run([os.path.join(boot, "bin", "go"), "tool", "objdump", "-s",
                       r"^(main|" + re.escape(path) + r"(/[^.]*)?)\.", prog], tmp, benv).stdout
            bad = sorted(set(GO_MAP.findall(dis)))
            if bad:
                sys.exit(f"test-map-lowering: {n}: S-Go code calls Go map entry points {bad}")
            used |= set(SGO_MAP.findall(dis))
            checked += 1
    if checked < 10 or not {"sgomakemap", "sgomapassign"} <= used or not any(u.startswith("sgomapaccess") for u in used):
        sys.exit(f"test-map-lowering: too little covered: {checked} programs, S-Go map entry points {sorted(used)}")
    print(f"test-map-lowering: ok: {checked} programs call only the ordered map ({', '.join(sorted(used))})")


if __name__ == "__main__":
    main()
