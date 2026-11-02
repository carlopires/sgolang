#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""policy-report.py - the S-Go target policy report (W05 deliverable; SGO-AUTH-001..003, SGO-TCB-002).

Writes sgo/docs/policy.md from the sources that enforce the policy, and checks that they agree:
  - the effect kinds are the same, in the same order, in the runtime (ceiling bits), the driver
    (grant lines), the linker (-sgogrants), package sgo (handle kinds), and SGO-AUTH-003;
  - the capability handle types are the same in the compiler's admission (sgoadmit) and the type
    checker (types2 isSgoHandle);
  - every denied escape route names a conformance case that exists and expects a rejection or a
    failure;
  - the TCB allowlist is for the current spec version.

    policy-report.py           check that sgo/docs/policy.md is current (CI)
    policy-report.py --write   rewrite it
"""
import sgoargs
import json
import os
import re
import sys

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
SGO = os.path.join(ROOT, "sgo")
OUT = os.path.join(SGO, "docs", "policy.md")
CASES = os.path.join(SGO, "testdata", "conformance")


def read(rel):
    return open(os.path.join(ROOT, rel)).read()


def go_strings(src, var):
    """The string literals of a Go composite literal assigned to var."""
    m = re.search(var + r"\s*=\s*(?:\[\.\.\.\]|\[\])string\{([^}]*)\}", src)
    if not m:
        sys.exit(f"policy-report: {var} not found")
    return re.findall(r'"([^"]*)"', m.group(1))


def kinds():
    found = {
        "runtime (src/runtime/sgo_auth.go sgoKindNames)": go_strings(read("src/runtime/sgo_auth.go"), "sgoKindNames"),
        "driver (src/cmd/sgo/module.go grantKinds)": go_strings(read("src/cmd/sgo/module.go"), "grantKinds"),
        "linker (src/cmd/link/internal/ld/sgoid.go sgoGrantKinds)": go_strings(read("src/cmd/link/internal/ld/sgoid.go"), "sgoGrantKinds"),
    }
    effects = read("src/sgo/effects.go")
    consts = re.search(r"const \(\n\tkindClock uint8 = 1 << iota\n((?:\tkind\w+\n)+)\)", effects)
    if not consts:
        sys.exit("policy-report: kind constants not found in src/sgo/effects.go")
    names = ["Clock"] + re.findall(r"\tkind(\w+)", consts.group(1))
    found["package sgo (src/sgo/effects.go kind constants)"] = [n.lower() for n in names]
    spec = read("sgo/spec/SPEC-AUTH.md")
    spec = spec[spec.index("### SGO-AUTH-003"):]  # the clause that lists the kinds' accessors
    found["spec (SGO-AUTH-003 root accessors)"] = [m.lower() for m in re.findall(r"`root\.(\w+)\(", spec)]
    ref = found["runtime (src/runtime/sgo_auth.go sgoKindNames)"]
    errs = [f"effect kinds differ: {where} has {k}, the runtime has {ref}" for where, k in found.items() if k != ref]
    return ref, errs


def handles():
    adm = read("src/cmd/compile/internal/sgoadmit/admit_expr.go")
    m = re.search(r"var sgoHandleTypes = map\[string\]bool\{([^}]*)\}", adm)
    a = sorted(re.findall(r'"(\w+)": true', m.group(1)))
    t2 = read("src/cmd/compile/internal/types2/sgo.go")
    m = re.search(r"func isSgoHandle\(T Type\) bool \{.*?switch n\.obj\.name \{\n\tcase ([^:]*):", t2, re.S)
    t = sorted(re.findall(r'"(\w+)"', m.group(1)))
    generic = sorted(set(t) - set(a))  # Channel, SendPermit: admitted only instantiated (sgoInstance)
    errs = []
    if set(a) - set(t):
        errs.append(f"handle types admitted by sgoadmit but not handles in types2: {sorted(set(a) - set(t))}")
    if generic != ["Channel", "SendPermit"]:
        errs.append(f"types2 handles not in sgoHandleTypes should be exactly the generic Channel and SendPermit, not {generic}")
    return sorted(t), errs


# Denied escape routes and the conformance case that shows each denial (A05, A06, A07, A09, S05, S13).
ROUTES = [
    ("unsafe", "SGO-NATIVE-001", "native-unsafe"),
    ("cgo (import \"C\")", "SGO-NATIVE-001", "native-cgo"),
    ("assembly files", "SGO-NATIVE-001", "native-asm"),
    ("//go:linkname", "SGO-NATIVE-001", "native-linkname"),
    ("raw system calls (syscall)", "SGO-NATIVE-001", "native-syscall-import"),
    ("plugin loading (plugin)", "SGO-NATIVE-001", "native-plugin-import"),
    ("runtime finalizers and cleanups (runtime)", "SGO-DEF-001", "native-runtime-finalizer"),
    ("scope finalizers", "SGO-DEF-001", "def-scope-finalizer"),
    ("reflection (reflect)", "SGO-REFLECT-001", "reflect-import"),
    ("ambient output (fmt)", "SGO-BIND-002", "bind-import-fmt"),
    ("ambient I/O in a dependency (os)", "SGO-BIND-002", "auth-dependency-ambient"),
    ("ambient time (time)", "SGO-TIMER-001", "timer-time-package"),
    ("ambient randomness (math/rand)", "SGO-RANDOM-001", "random-import"),
    ("network (net)", "SGO-NET-001", "net-dial"),
    ("shared-memory synchronization (sync)", "SGO-SYNC-001", "sync-mutex"),
    ("bare go statement, also in a dependency, generated file, or dead code", "SGO-SPAWN-005", "spawn-go-in-dependency"),
    ("init functions (initialization paths)", "SGO-BIND-003", "bind-init-func"),
    ("non-S-Go source inputs (.go files)", "SGO-TOOL-001", "tool-go-file-input"),
    ("stock Go objects", "SGO-IMP-002", "imp-stock-object"),
    ("user generics", "SGO-GEN-001", "type-generic"),
    ("effect through a captured handle without the kind in the ceiling", "SGO-AUTH-001", "auth-spawn-without"),
    ("effect through a function value", "SGO-AUTH-001", "auth-function-value"),
    ("effect through a dependency", "SGO-AUTH-001", "auth-dependency-effect"),
    ("effect of a kind that is not granted", "SGO-AUTH-002", "auth-missing-grant"),
    ("root handle from a non-root context", "SGO-AUTH-002", "auth-root-only"),
    ("lent handle after its nursery closed", "SGO-AUTH-003", "auth-lend-revoked"),
    ("a user package named like a TCB package", "SGO-TCB-002", "tcb-user-package-named-runtime"),
    ("a module named like the trusted package sgo", "SGO-TCB-002", "tcb-module-named-sgo"),
]


def routes():
    errs, rows = [], []
    for what, clause, case in ROUTES:
        p = os.path.join(CASES, case, "expect.json")
        if not os.path.exists(p):
            errs.append(f"escape route {what!r}: case {case} does not exist")
            continue
        e = json.load(open(p))
        if not (e["kind"] == "reject" or e.get("exit", 0) != 0):
            errs.append(f"escape route {what!r}: case {case} does not expect a rejection or a failure")
        how = ", ".join(sorted({d["code"] for d in e.get("diagnostics", [])})) or json.loads(e["stderr_exact"])["kind"]
        rows.append(f"| {what} | {clause} | `{case}` | {how} |")
    return rows, errs


def render():
    k, e1 = kinds()
    h, e2 = handles()
    r, e3 = routes()
    tcb = json.load(open(os.path.join(SGO, "TARGET_TCB.json")))
    version = open(os.path.join(SGO, "spec", "VERSION")).read().strip()
    e4 = [] if tcb.get("spec_version") == version else [f"TARGET_TCB.json is for spec {tcb.get('spec_version')}, not {version}"]
    prims = {p["id"]: p for p in tcb["primitives"]}
    effect_rows = []
    accessor = {"clock": "root.Clock()", "wallclock": "root.WallClock()", "random": "root.Random(seed)",
                "entropy": "root.Entropy()", "simstore": "root.SimStore()"}
    tcbof = {"clock": "TCB-P13", "wallclock": "TCB-P13", "random": "none (package sgo, deterministic)",
             "entropy": "TCB-P14", "simstore": "TCB-P15"}
    for kind in k:
        t = tcbof[kind]
        if t.startswith("TCB-") and t not in prims:
            e4.append(f"kind {kind}: {t} is not in TARGET_TCB.json")
        effect_rows.append(f"| `{kind}` | `grant {kind}` | `{accessor[kind]}` | `*sgo.{accessor[kind].split('.')[1].split('(')[0]}` | {t} |")
    lines = [
        "# S-Go target policy (spec " + version + ")",
        "",
        "Generated by `sgo/scripts/policy-report.py` from the sources that enforce the policy; CI fails if it is stale or if those sources disagree. Default deny: anything not listed as admitted is rejected (`SGO_SPEC_ADMISSION.json`).",
        "",
        "## Authority",
        "",
        "There is no ambient authority. A program's root grants are the `grant` lines of its main module's `sgo.mod` (SGO-AUTH-002), recorded in the binary's build ID (`grants=`); dependencies cannot add grants. Every task has a ceiling: the root's is the grants, a child's is its spawner's minus what `n.SpawnWithout` removes, and nothing widens it (SGO-AUTH-001). An effect operation needs both a handle that permits it and its kind in the calling task's ceiling; otherwise it fails with `MissingCapability` before any effect.",
        "",
        "| Kind | sgo.mod | Root accessor | Handle | TCB primitive |",
        "|---|---|---|---|---|",
        *effect_rows,
        "",
        "Attenuation: `SimStore.ReadOnly`, `SimStore.Prefix`; every handle has `LendTo(n)`, revoked when `n` closes (SGO-AUTH-003).",
        "",
        "Capability handle types (opaque, used only behind `*`, never nil, never compared; SGO-TYPE-005): " + ", ".join(f"`*sgo.{x}`" for x in h) + " (`Channel` and `SendPermit` only instantiated, SGO-PERMIT-002).",
        "",
        "## Trusted computing base",
        "",
        f"`sgo/TARGET_TCB.json` (SGO-TCB-002): spec {tcb['spec_version']}, allowlist digest `{tcb['allowlist_digest'][:16]}`, {tcb['summary']['packages']} packages, {tcb['summary']['files']} files ({tcb['summary']['files_changed_from_upstream']} changed from upstream), {len(prims)} primitives. A file is trusted only at its own import path and file name with its exact SHA-256: neither a package name nor copied content confers trust (W12 R1).",
        "",
        "## Denied escape routes",
        "",
        "| Route | Clause | Case | Result |",
        "|---|---|---|---|",
        *r,
        "",
    ]
    return "\n".join(lines), e1 + e2 + e3 + e4


def main():
    sgoargs.check(flags={"--write": "bool"})  # strict command line (W12 R14)
    text, errs = render()
    for e in errs:
        print(f"policy-report: {e}", file=sys.stderr)
    if errs:
        sys.exit(1)
    if "--write" in sys.argv:
        open(OUT, "w").write(text)
        print(f"policy-report: wrote {os.path.relpath(OUT, ROOT)}")
        return
    if not os.path.exists(OUT) or open(OUT).read() != text:
        sys.exit("policy-report: sgo/docs/policy.md is stale; run sgo/scripts/policy-report.py --write")
    print(f"policy-report: ok: {len(ROUTES)} denied routes with cases; effect kinds and handle types agree")


if __name__ == "__main__":
    main()
