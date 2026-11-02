#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""fill-progress.py - one-off W12 (SR3): replace 'none yet' progress fields of frozen clauses.

implementation_coverage and executed_evidence are progress fields, excluded from clause digests
(spec-tool.py NON_NORMATIVE), so this changes no identity. For each frozen clause still saying
'none yet', implementation_coverage gets the work packages already named in the field and the
family's main implementation files; executed_evidence gets the clause's own fixtures (run in the
conformance lane on all three targets) and the evidence README of each named work package.
"""
import os
import re
import sys

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
SPEC = os.path.join(ROOT, "sgo", "spec")
RT, T2, AD, CS = "src/runtime/", "src/cmd/compile/internal/types2/", "src/cmd/compile/internal/sgoadmit/", "src/cmd/sgo/"
SYN = "src/cmd/compile/internal/syntax/"
FILES = {
    "SPEC-LEX": [SYN + "scanner.go", SYN + "parser.go", CS + "build.go"],
    "SPEC-BIND": [T2 + "sgo.go", AD + "admit.go"],
    "SPEC-TYPE": [T2 + "sgo.go", AD + "admit.go", AD + "admit_expr.go"],
    "SPEC-VALUE": ["src/cmd/compile/internal/ssagen/sgo.go", RT + "sgo_api.go", T2 + "sgo.go"],
    "SPEC-MEM": [T2 + "sgo_flow.go", T2 + "sgo.go"],
    "SPEC-CONTROL": [T2 + "sgo.go", T2 + "sgo_flow.go", RT + "sgo_nursery.go"],
    "SPEC-DEFER": [RT + "panic.go", RT + "sgo_nursery.go", RT + "sgo_record.go"],
    "SPEC-NURSERY": [SYN + "parser.go", T2 + "sgo.go", RT + "sgo_scope.go", RT + "sgo_nursery.go"],
    "SPEC-SPAWN": [T2 + "sgo.go", RT + "sgo_api.go", RT + "sgo_nursery.go"],
    "SPEC-CANCEL": [RT + "sgo_cancel.go", RT + "sgo_scope.go", RT + "sgo_api.go"],
    "SPEC-DRAIN": [RT + "sgo_nursery.go", RT + "sgo_main.go", CS + "test.go", CS + "lab.go"],
    "SPEC-WAIT": [RT + "sgo_api.go", RT + "sgo_nursery.go"],
    "SPEC-CAP": [RT + "sgo_auth.go", T2 + "sgo.go", "src/sgo/sgo.go"],
    "SPEC-IMPORT": ["src/cmd/compile/internal/noder/sgo_identity.go", AD + "identity.go", CS + "module.go", CS + "build.go"],
    "SPEC-TCB": [CS + "tcb.go", CS + "tcbasm.go", AD + "tcbtable.go", "sgo/TARGET_TCB.json"],
    "SPEC-TIMER": [RT + "sgo_api.go", RT + "sgo_effects.go", "src/sgo/sgo.go"],
    "SPEC-STACK": [RT + "stack.go", RT + "proc.go"],
    "SPEC-ERRORS": [AD + "admit.go", RT + "sgo_record.go", RT + "sgo_main.go"],
    "SPEC-PROC": ["src/cmd/compile/internal/noder/sgo_main.go", RT + "sgo_main.go", "src/sgo/sgo.go"],
    "SPEC-TEST": ["sgo/scripts/conformance.py"],
}
WP = re.compile(r"\bW(\d\d)\b")
FIX = re.compile(r"fixture:([\w-]+)")


def main():
    changed = 0
    for fn in sorted(os.listdir(SPEC)):
        if not fn.startswith("SPEC-") or not fn.endswith(".md"):
            continue
        fam = fn[:-3]
        path = os.path.join(SPEC, fn)
        blocks = re.split(r"(?m)^(?=### SGO-)", open(path).read())
        out = []
        for b in blocks:
            if b.startswith("### SGO-") and "- **status:** frozen" in b and "none yet" in b:
                if fam not in FILES:
                    sys.exit(f"fill-progress: no file map for {fam}")
                ic = re.search(r"(?m)^- \*\*implementation_coverage:\*\* (.*)$", b)
                wps = sorted(set(WP.findall(ic.group(1)))) or ["03"]
                fixtures = sorted(set(FIX.findall(b)))
                for f in fixtures:
                    if not os.path.isdir(os.path.join(ROOT, "sgo/testdata/conformance", f)):
                        sys.exit(f"fill-progress: {fam}: fixture {f} missing")
                impl = (", ".join("W" + w for w in wps) + ": " +
                        ", ".join(f"`{p}`" for p in FILES[fam]) + ".")
                ev = []
                if fixtures:
                    ev.append("conformance fixtures " + ", ".join(fixtures) +
                              " (darwin/arm64, linux/amd64, linux/arm64)")
                ev += [f"`sgo/evidence/w{w}/README.md`" for w in wps
                       if os.path.exists(os.path.join(ROOT, f"sgo/evidence/w{w}/README.md"))]
                b = re.sub(r"(?m)^- \*\*implementation_coverage:\*\* .*$",
                           lambda _: "- **implementation_coverage:** " + impl, b)
                b = re.sub(r"(?m)^- \*\*executed_evidence:\*\* .*$",
                           lambda _: "- **executed_evidence:** " + "; ".join(ev) + ".", b)
                changed += 1
            out.append(b)
        open(path, "w").write("".join(out))
    print(f"fill-progress: {changed} clauses updated")


if __name__ == "__main__":
    main()
