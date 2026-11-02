#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""test-negative.py - negative-fixture honesty (D79, Q02; plan §12.3).

One representative per category of plan §12.3 must fail with its *specified* code and clause,
not merely a non-zero exit: the case's expectation must pin the code (a diagnostic code, a
failure record kind, a lab refusal, or exact output), cite the clause, and the shipped driver
must reproduce it. A harness self-test rewrites each pinned code to a wrong one in a copy of the
case and requires the conformance runner to report a failure, so no category passes vacuously.
"""
import importlib.util
import json
import os
import shutil
import sys
import tempfile

from sgotool import GOARCH, GOOS, ROOT

SGO = os.path.join(ROOT, "pkg", "tool", f"{GOOS}_{GOARCH}", "sgo")
CASES = os.path.join(ROOT, "sgo", "testdata", "conformance")

# category, case, clause, the pinned code as it appears in expect.json, and its wrong replacement.
CATEGORIES = [
    ("orphan on closure", "spawn-after-close", "SGO-SPAWN-002", '"SpawnContractViolation"', '"WrongKind"'),
    ("duplicate permit commit", "permit-double-commit", "SGO-PERMIT-002",
     "commit of a permit that is already committed", "commit of a permit that is fine"),
    ("capacity leak (permit)", "permit-panic-with-permit", "SGO-PERMIT-002", "PermitAbandoned", "PermitKept"),
    ("capacity leak (limiter token)", "sync-token-abandoned", "SGO-SYNC-002", "TokenAbandoned", "TokenKept"),
    ("lost wakeup (production)", "native-deadlock", "SGO-CHAN-003", "Deadlock", "Livelock"),
    ("lost wakeup (lab)", "lab-deadlock", "SGO-LAB-006", '"LabDeadlock"', '"WrongKind"'),
    ("policy bypass: undeclared effect", "auth-missing-grant", "SGO-AUTH-002", "MissingCapability", "WrongCapability"),
    ("policy bypass: capability forgery", "cap-context-as-nursery", "SGO-CAP-001", "SGO_E_TYPE_MISMATCH", "SGO_E_WRONG"),
    ("policy bypass: capability comparison", "auth-handle-compare-denied", "SGO-AUTH-001",
     "SGO_E_CAPABILITY_MISUSE", "SGO_E_WRONG"),
    # Lab refusals: the case pins which tampering is replayed; the code each must be refused with is
    # the runner's LAB_REFUSALS, which the self-test swaps.
    ("replay divergence: edited trace", "lab-trace-integrity", "SGO-LAB-005", '"reject-altered"', None),
    ("replay divergence: changed choice", "lab-impossible-choice", "SGO-LAB-005", '"reject-changed-choice"', None),
    ("replay divergence: other build", "lab-manifest-mismatch", "SGO-LAB-005", '"reject-other-build"', None),
    ("unreported partial I/O", "net-cancel-progress", "SGO-FILE-003",
     "write cancelled after partial progress", "write cancelled with no progress"),
    ("incorrect return", "mem-capture-named-result-return", "SGO-MEM-002", "SGO_E_CAPTURE_NOT_FINAL", "SGO_E_WRONG"),
    ("incorrect defer", "def-panic-in-defer", "SGO-DEF-004", r'\"kind\":\"Panic\",\"message\":\"first\"',
     r'\"kind\":\"WrongKind\",\"message\":\"first\"'),
]


def load_runner():
    spec = importlib.util.spec_from_file_location("conformance", os.path.join(ROOT, "sgo", "scripts", "conformance.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def main():
    runner = load_runner()
    problems = []
    for cat, case, clause, code, wrong in CATEGORIES:
        src = os.path.join(CASES, case)
        text = open(os.path.join(src, "expect.json")).read()
        e = json.loads(text)
        if clause not in e.get("clauses", []):
            problems.append(f"{cat}: {case} does not cite {clause}")
        if code not in text:
            problems.append(f"{cat}: {case} does not pin {code}")
            continue
        if e["kind"] in ("run", "test") and e.get("exit", 0) == 0 and "stdout" not in e:
            problems.append(f"{cat}: {case} exits 0 without pinned output")
        if msg := runner.run_case(case, src, SGO):
            problems.append(f"{cat}: {case} fails: {msg}")
        # Self-test: the same case with its pinned code replaced must be reported as failing.
        if wrong is None:
            saved = dict(runner.LAB_REFUSALS)
            codes = list(saved.values())
            runner.LAB_REFUSALS.update({k: codes[(i + 1) % len(codes)] for i, k in enumerate(saved)})
            try:
                if not runner.run_case(case, src, SGO):
                    problems.append(f"self-test: {case} still passes with the lab refusal codes swapped; the check is vacuous")
            finally:
                runner.LAB_REFUSALS.update(saved)
            continue
        with tempfile.TemporaryDirectory() as tmp:
            d = os.path.join(tmp, case)
            shutil.copytree(src, d, symlinks=True)
            open(os.path.join(d, "expect.json"), "w").write(text.replace(code, wrong))
            if not runner.run_case(case, d, SGO):
                problems.append(f"self-test: {case} with {code} replaced by {wrong} still passes; the check is vacuous")
    for p in problems:
        print(p)
    print(f"test-negative: {'ok' if not problems else 'FAIL'}: {len(CATEGORIES)} plan §12.3 representatives fail "
          f"with their specified code and clause, and each fails the runner when its code is changed; "
          f"{len(problems)} problem(s)")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
