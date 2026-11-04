#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""spec-tool.py — validate S-Go specification clauses and keep the admission registry in sync (D18).

    spec-tool.py check    validate sgo/spec/*.md; fail if SGO_SPEC_ADMISSION.json is stale
    spec-tool.py update   validate, then write clause IDs, digests, spec_version, rows_digest
    spec-tool.py list     print one line per clause

Clause format (sgo/spec/README.md):

    ### SGO-LEX-001 — Title
    - **version:** 1
    - **status:** draft | frozen | superseded
    - **<field>:** text            (one per required field; continuation lines indented by two spaces)

Digests are SHA-256 over the clause's normative text (heading through the line before the next heading,
excluding the progress fields status, implementation_coverage, and executed_evidence), with LF line
endings, trailing whitespace stripped, and trailing blank lines removed.
"""
import sgoargs
import hashlib
import json
import os
import re
import subprocess
import sys

ROOT = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
SGO = os.path.join(ROOT, "sgo")
SPEC = os.path.join(SGO, "spec")
REGISTRY = os.path.join(SGO, "SGO_SPEC_ADMISSION.json")
MATRIX = os.path.join(SGO, "SGO_TEST_MATRIX.json")
CONFORMANCE = os.path.join(SGO, "testdata", "conformance")

HEAD = re.compile(r"^### (SGO-([A-Z0-9]+)-(\d{3})) — (.+)$")
FIELD = re.compile(r"^- \*\*([a-z_]+):\*\* ?(.*)$")
CLAUSE_REF = re.compile(r"\bSGO-[A-Z0-9]+-\d{3}\b")
CASE_REF = re.compile(r"\b[A-Z]\d{2}\b")
FIXTURE_REF = re.compile(r"\bfixture:([a-z0-9][a-z0-9_-]*)")
COMPUTED = {"clause_id", "normative_text_digest"}
FORBIDDEN = [(re.compile(r"\bTODO\b|\bTBD\b|\bFIXME\b"), "placeholder"),
             (re.compile(r"same as (upstream )?go\b|whatever (upstream|go) does|as in go\b", re.I), "implicit Go semantics")]
STATUSES = {"draft", "frozen", "superseded"}
# Progress fields: they change as work packages add implementations and evidence, so they are excluded
# from the normative digest. Everything else in a clause is normative and locked when frozen.
NON_NORMATIVE = {"status", "implementation_coverage", "executed_evidence"}


def die(msg):
    print(f"spec-tool: error: {msg}", file=sys.stderr)
    sys.exit(1)


def normalize(lines):
    out = [l.rstrip() for l in lines]
    while out and not out[-1]:
        out.pop()
    return "\n".join(out) + "\n"


def parse_file(path, family):
    clauses, cur, body, normative = [], None, [], []
    skipping = False
    lines = open(path, encoding="utf-8").read().replace("\r\n", "\n").split("\n")

    def close():
        if cur is not None:
            cur["text"] = normalize(body)
            cur["digest"] = hashlib.sha256(normalize(normative).encode()).hexdigest()
            clauses.append(cur)

    for n, line in enumerate(lines, 1):
        m = HEAD.match(line)
        if m or line.startswith("## ") or line.startswith("# "):
            close()
            cur, body = None, []
        if m:
            cur = {"id": m.group(1), "family_code": m.group(2), "title": m.group(4), "file": os.path.relpath(path, ROOT),
                   "line": n, "fields": {}}
            body, normative, skipping = [line], [line], False
            continue
        if cur is None:
            continue
        body.append(line)
        f = FIELD.match(line)
        if f:
            skipping = f.group(1) in NON_NORMATIVE
        elif not line.startswith("  "):
            skipping = False
        if not skipping:
            normative.append(line)
        if f:
            cur["fields"][f.group(1)] = f.group(2).strip()
            cur["_last"] = f.group(1)
        elif line.startswith("  ") and cur.get("_last") and line.strip():
            cur["fields"][cur["_last"]] += " " + line.strip()
    close()
    for c in clauses:
        c.pop("_last", None)
        c["family"] = family
    return clauses


def load():
    registry = json.load(open(REGISTRY))
    required = [f for f in registry["required_clause_fields"] if f not in COMPUTED]
    families = {r["id"]: r for r in registry["rows"]}
    cases = {c["id"] for c in json.load(open(MATRIX))["cases"]}
    if not os.path.isdir(SPEC):
        die(f"no specification directory {SPEC}")
    clauses = []
    for name in sorted(os.listdir(SPEC)):
        if not (name.startswith("SPEC-") and name.endswith(".md")):
            continue
        family = name[:-3]
        if family not in families:
            die(f"{name}: no registry row {family}")
        clauses += parse_file(os.path.join(SPEC, name), family)
    version_file = os.path.join(SPEC, "VERSION")
    spec_version = open(version_file).read().strip() if os.path.exists(version_file) else None
    return registry, required, families, cases, clauses, spec_version


def validate(required, families, cases, clauses):
    errors = []
    ids = {}
    for c in clauses:
        where = f"{c['file']}:{c['line']} {c['id']}"
        if c["id"] in ids:
            errors.append(f"{where}: duplicate clause ID (also {ids[c['id']]})")
        ids[c["id"]] = where
        f = c["fields"]
        status = f.get("status", "")
        if status not in STATUSES:
            errors.append(f"{where}: status must be one of {sorted(STATUSES)}, got {status!r}")
        if not re.fullmatch(r"[1-9]\d*", f.get("version", "")):
            errors.append(f"{where}: version must be a positive integer")
        for name in required:
            if name == "version":
                continue
            v = f.get(name)
            if v is None or not v:
                errors.append(f"{where}: missing field {name}")
                continue
            if re.fullmatch(r"n/?a\.?", v, re.I):
                errors.append(f"{where}: {name}: 'n/a' needs a reason ('n/a — <reason>')")
            for rx, what in FORBIDDEN:
                if rx.search(v):
                    errors.append(f"{where}: {name}: forbidden {what}: {v[:80]!r}")
            if status == "frozen" and v.upper().startswith("PENDING"):
                errors.append(f"{where}: {name}: a frozen clause cannot have PENDING content")
        for name in f:
            if name not in required and name not in ("status",):
                errors.append(f"{where}: unknown field {name}")
    known = set(ids)
    for c in clauses:
        where = f"{c['file']}:{c['line']} {c['id']}"
        text = " ".join(c["fields"].values())
        for ref in CLAUSE_REF.findall(text):
            if ref not in known:
                errors.append(f"{where}: reference to unknown clause {ref}")
        for fld in ("positive_tests", "negative_tests"):
            v = c["fields"].get(fld, "")
            # Format: "<case IDs, comma-separated>; <fixture refs / notes>" or "n/a — <reason>".
            if not re.match(r"n/a\b", v, re.I):
                head = v.split(";", 1)[0]
                for tok in [t.strip() for t in head.split(",") if t.strip()]:
                    if tok.startswith("fixture:"):
                        continue
                    if not CASE_REF.fullmatch(tok):
                        errors.append(f"{where}: {fld}: expected acceptance-case IDs before ';', got {tok!r}")
                    elif tok not in cases:
                        errors.append(f"{where}: {fld}: unknown acceptance case {tok}")
            for fx in FIXTURE_REF.findall(v):
                if c["fields"].get("status") == "frozen" and not os.path.isdir(os.path.join(CONFORMANCE, fx)):
                    errors.append(f"{where}: {fld}: fixture {fx} does not exist under sgo/testdata/conformance")
    # A frozen clause names only what exists: every `sgo.X` it mentions is an exported declaration
    # of package sgo (0.16.1 review F1). One-letter names are placeholders (`sgo.K`).
    sgo_decls = sgo_exports()
    for c in clauses:
        if c["fields"].get("status") != "frozen":
            continue
        text = " ".join(c["fields"].values())
        for name in sorted(set(SGO_NAME.findall(text))):
            if len(name) > 1 and name not in sgo_decls:
                errors.append(f"{c['file']}:{c['line']} {c['id']}: names sgo.{name}, which package sgo does not declare")
    return errors


SGO_NAME = re.compile(r"`[^`]*?\bsgo\.([A-Z]\w*)")


def sgo_exports():
    """The exported top-level names of package sgo (src/sgo, tests excluded)."""
    names = set()
    src = os.path.join(ROOT, "src", "sgo")
    for fn in os.listdir(src):
        if not fn.endswith(".go") or fn.endswith("_test.go"):
            continue
        text = open(os.path.join(src, fn)).read()
        names.update(re.findall(r"^(?:func(?: \([^)]*\))?|type|var|const)\s+([A-Z]\w*)", text, re.M))
        names.update(re.findall(r"^\t([A-Z]\w*)\s", text, re.M))  # const and var blocks
    return names


def computed_registry(registry, families, clauses, spec_version):
    reg = json.loads(json.dumps(registry))
    by_family = {}
    for c in clauses:
        by_family.setdefault(c["family"], []).append(c)
    for row in reg["rows"]:
        cs = sorted(by_family.get(row["id"], []), key=lambda c: c["id"])
        row["clause_ids"] = [c["id"] for c in cs]
        row["clause_digests"] = {c["id"]: c["digest"] for c in cs}
        row["clause_status"] = {c["id"]: c["fields"].get("status") for c in cs}
        row["clause_versions"] = {c["id"]: int(c["fields"].get("version", "0")) for c in cs}
        row["normative_text_digest"] = (hashlib.sha256("".join(c["digest"] for c in cs).encode()).hexdigest()
                                        if cs else None)
    reg["spec_version"] = spec_version
    canon = [{k: r.get(k) for k in ("id", "status", "subset0", "clause_ids", "normative_text_digest")} for r in reg["rows"]]
    # rows_digest identifies the admission decisions (status, scope, clause texts). The admission
    # digest the toolchain and every binary record is the SHA-256 of the whole registry file
    # (tcb-manifest.py writes it into identity.go; W12 R12).
    reg.pop("admission_digest", None)
    reg["rows_digest"] = hashlib.sha256(json.dumps(canon, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    return reg


def frozen_changes(old, clauses):
    """A frozen clause's text may only change together with a version bump."""
    errs = []
    olddig = {}
    for r in old["rows"]:
        for cid, d in (r.get("clause_digests") or {}).items():
            olddig[cid] = (d, (r.get("clause_status") or {}).get(cid), (r.get("clause_versions") or {}).get(cid))
    for c in clauses:
        prev = olddig.get(c["id"])
        if not prev or prev[1] != "frozen":
            continue
        version = int(c["fields"].get("version", "0"))
        if prev[0] != c["digest"] and (prev[2] is None or version <= prev[2]):
            errs.append(f"{c['file']}:{c['line']} {c['id']}: frozen clause changed without a version bump "
                        f"(still v{version}); bump the version and record the amendment in CHANGELOG")
        if prev[2] is not None and version < prev[2]:
            errs.append(f"{c['file']}:{c['line']} {c['id']}: version went down from v{prev[2]} to v{version}")
    return errs


def main():
    sgoargs.check(positional={"check", "update", "list"}, max_positional=1)  # strict command line (W12 R14)
    mode = sys.argv[1] if len(sys.argv) > 1 else "check"
    registry, required, families, cases, clauses, spec_version = load()
    errors = validate(required, families, cases, clauses)
    if mode == "list":
        for c in clauses:
            print(f"{c['id']}\t{c['fields'].get('status')}\tv{c['fields'].get('version')}\t{c['digest'][:12]}\t{c['title']}")
    if errors:
        for e in errors:
            print(f"spec-tool: {e}", file=sys.stderr)
        die(f"{len(errors)} problem(s) in {len(clauses)} clause(s)")
    new = computed_registry(registry, families, clauses, spec_version)
    if mode == "update":
        errs = frozen_changes(registry, clauses)
        if errs:
            for e in errs:
                print(f"spec-tool: {e}", file=sys.stderr)
            die("frozen clauses changed")
        with open(REGISTRY, "w") as f:
            json.dump(new, f, indent=2, ensure_ascii=False)
            f.write("\n")
        print(f"spec-tool: updated registry: {len(clauses)} clauses, spec_version={spec_version}, "
              f"rows_digest={new['rows_digest'][:16]}")
    elif mode == "check":
        if new != registry:
            die("SGO_SPEC_ADMISSION.json is stale; run sgo/scripts/spec-tool.py update")
        # The toolchain records the SHA-256 of the registry file as its admission digest (W12 R12).
        adm = hashlib.sha256(open(REGISTRY, "rb").read()).hexdigest()[:16]
        for gen in ("src/cmd/sgo/identity.go", "src/cmd/compile/internal/sgoadmit/identity.go"):
            text = open(os.path.join(ROOT, gen)).read()
            got = dict(re.findall(r'(SpecVersion|AdmissionDigest)\s*=\s*"([^"]*)"', text))
            if got.get("AdmissionDigest") != adm or got.get("SpecVersion") != spec_version:
                die(f"{gen} records spec {got.get('SpecVersion')} admission {got.get('AdmissionDigest')}, the registry is "
                    f"spec {spec_version} admission {adm}; run sgo/scripts/tcb-manifest.py --write")
        st = {}
        for c in clauses:
            st[c["fields"].get("status")] = st.get(c["fields"].get("status"), 0) + 1
        print(f"spec-tool: ok: {len(clauses)} clauses {st}, spec_version={spec_version}")
    elif mode != "list":
        die(f"unknown mode {mode}")


if __name__ == "__main__":
    main()
