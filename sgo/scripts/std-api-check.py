#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""std-api-check.py - the standard library exports exactly its specified API (D126, SGO-IMP-004).

Every exported declaration of a non-test file under src/sgostd (functions, methods, types, and
constants) is compared with the declarations listed in its package's clause in
sgo/spec/SPEC-STD.md: the backquoted `func ...`, `type ...`, and `const ...` items of the clause's
grammar_and_types field. A name
exported but not specified, or specified but missing, or with a different signature, fails.

    std-api-check.py          check
    std-api-check.py --list   print the exported API of every package, in clause form
"""

import os
import re
import sys

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
STD = os.path.join(ROOT, "src", "sgostd")
SPEC = os.path.join(ROOT, "sgo", "spec", "SPEC-STD.md")


def norm(sig):
    return re.sub(r"\s+", " ", sig.strip())


def exported(pkg):
    """The exported declarations of package directory pkg, as normalized strings."""
    out = set()
    d = os.path.join(STD, pkg)
    for name in sorted(os.listdir(d)):
        if not name.endswith(".sgo") or name.endswith("_test.sgo"):
            continue
        text = open(os.path.join(d, name)).read()
        for m in re.finditer(r"^func (\([^)]*\) )?([A-Z]\w*)(\(.*?)\s*\{\s*$", text, re.M):
            recv = m.group(1) or ""
            recv = re.sub(r"\(\w+ ", "(", recv)
            out.add(norm(f"func {recv}{m.group(2)}{m.group(3)}"))
        for m in re.finditer(r"^type ([A-Z]\w*) ", text, re.M):
            out.add(f"type {m.group(1)}")
        for block in re.finditer(r"^const \((.*?)^\)", text, re.M | re.S):
            for line in block.group(1).splitlines():
                m = re.match(r"\s*([A-Z]\w*)\b", line)
                if m:
                    out.add(f"const {m.group(1)}")
        for m in re.finditer(r"^const ([A-Z]\w*)\b", text, re.M):
            out.add(f"const {m.group(1)}")
    return out


def packages():
    return sorted(p for p in os.listdir(STD) if os.path.isdir(os.path.join(STD, p)))


def specified():
    """Map package -> set of declarations from the api: lines of SPEC-STD.md clauses."""
    spec = {}
    text = open(SPEC).read()
    for clause in re.split(r"^### ", text, flags=re.M)[1:]:
        m = re.search(r"`std/(\w+)`", clause.splitlines()[0])
        if not m:
            continue
        decls = set()
        for line in clause.splitlines():
            if line.startswith("- **grammar_and_types:**"):
                decls |= {norm(x) for x in re.findall(r"`([^`]+)`", line) if re.match(r"(func|type|const) ", x)}
        spec[m.group(1)] = decls
    return spec


def main():
    if sys.argv[1:] == ["--list"]:
        for p in packages():
            print(f"std/{p}:")
            print("; ".join(f"`{d}`" for d in sorted(exported(p))) + ".")
        return
    if sys.argv[1:]:
        sys.exit(__doc__)
    spec = specified()
    problems = []
    for p in packages():
        have = exported(p)
        want = spec.get(p)
        if want is None:
            problems.append(f"std/{p}: no clause in SPEC-STD.md")
            continue
        for d in sorted(have - want):
            problems.append(f"std/{p}: exported but not specified: {d}")
        for d in sorted(want - have):
            problems.append(f"std/{p}: specified but not exported: {d}")
    for p in sorted(set(spec) - set(packages())):
        problems.append(f"std/{p}: specified but no package in src/sgostd")
    for x in problems:
        print("std-api-check: " + x)
    n = sum(len(exported(p)) for p in packages())
    if problems:
        sys.exit(f"std-api-check: FAIL: {len(problems)} problem(s)")
    print(f"std-api-check: ok: {len(packages())} packages, {n} exported declarations, each specified")


if __name__ == "__main__":
    main()
