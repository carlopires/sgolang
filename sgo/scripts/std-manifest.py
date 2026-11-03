#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""std-manifest.py — the standard library's MANIFEST (SGO-IMP-004).

    std-manifest.py          check src/sgostd/MANIFEST against the files (exit 1 if stale)
    std-manifest.py --write  rewrite it

MANIFEST lists every non-test .sgo file under src/sgostd with its SHA-256 ("<hex>  <slash path>"),
sorted by path. The driver refuses to compile a std/ package whose files differ from it, so a
modified installation is detected rather than trusted.
"""

import hashlib
import os
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
STD = os.path.join(ROOT, "src", "sgostd")


def render():
    lines = []
    for d, dirs, files in os.walk(STD):
        dirs.sort()
        for f in sorted(files):
            if f.endswith(".sgo") and not f.endswith("_test.sgo"):
                p = os.path.join(d, f)
                rel = os.path.relpath(p, STD).replace(os.sep, "/")
                lines.append((rel, hashlib.sha256(open(p, "rb").read()).hexdigest()))
    lines.sort()
    return "".join(f"{h}  {rel}\n" for rel, h in lines)


def main():
    want = render()
    path = os.path.join(STD, "MANIFEST")
    if sys.argv[1:] == ["--write"]:
        open(path, "w").write(want)
        print(f"std-manifest: wrote {want.count(chr(10))} entries")
        return
    have = open(path).read() if os.path.exists(path) else ""
    if have != want:
        sys.exit("std-manifest: src/sgostd/MANIFEST is stale; run sgo/scripts/std-manifest.py --write")
    print(f"std-manifest: ok: {want.count(chr(10))} entries")


if __name__ == "__main__":
    main()
