#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""inventory-check.py — W01 step 3: every compiler/runtime surface maps to a clause (default deny).

    inventory-check.py          check; fail if any rule is violated or sgo/docs/inventory.md is stale
    inventory-check.py --write  check and regenerate sgo/docs/inventory.md

Surfaces are extracted from the fork's sources by sgo/scripts/inventory/extract.go, built with the
pinned bootstrap Go. Rules come from sgo/spec/INVENTORY.map (format documented there).
"""
import sgoargs
import collections
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile

ROOT = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
SGO = os.path.join(ROOT, "sgo")
MAP = os.path.join(SGO, "spec", "INVENTORY.map")
REGISTRY = os.path.join(SGO, "SGO_SPEC_ADMISSION.json")
DOC = os.path.join(SGO, "docs", "inventory.md")
VERDICTS = {"admit", "deny", "internal"}
SOURCE_KINDS = {"token", "operator", "syntax-node", "universe"}


def extract():
    boot = subprocess.check_output([os.path.join(SGO, "scripts", "fetch-bootstrap.sh")], text=True).strip()
    env = {"HOME": os.environ["HOME"], "PATH": "/usr/bin:/bin", "GOTOOLCHAIN": "local", "GOROOT": boot,
           "GOPROXY": "off", "GOFLAGS": "", "GOTELEMETRY": "off",
           "GOCACHE": os.path.join(os.environ.get("SGO_CACHE", os.path.expanduser("~/.cache/sgolang")), "gocache", "inventory")}
    with tempfile.TemporaryDirectory() as tmp:
        with open(os.path.join(tmp, "go.mod"), "w") as f:
            f.write("module extract\n\ngo 1.26\n")
        src = open(os.path.join(SGO, "scripts", "inventory", "extract.go")).read()
        with open(os.path.join(tmp, "extract.go"), "w") as f:
            f.write(src)
        subprocess.check_call([os.path.join(boot, "bin", "go"), "build", "-o", "extract", "."], cwd=tmp, env=env)
        out = subprocess.check_output([os.path.join(tmp, "extract"), ROOT], text=True)
    return [json.loads(l) for l in out.splitlines() if l.strip()]


def load_rules(errors):
    rules = []
    for n, line in enumerate(open(MAP), 1):
        body, _, note = line.partition("#")
        if not body.strip():
            continue
        parts = body.split()
        if len(parts) != 4:
            errors.append(f"INVENTORY.map:{n}: expected 4 fields, got {len(parts)}")
            continue
        kind, rx, verdict, clause = parts
        try:
            pat = re.compile(rx)
        except re.error as e:
            errors.append(f"INVENTORY.map:{n}: bad regex {rx!r}: {e}")
            continue
        if verdict not in VERDICTS:
            errors.append(f"INVENTORY.map:{n}: unknown verdict {verdict!r}")
        rules.append({"line": n, "kind": kind, "rx": pat, "verdict": verdict, "clause": clause,
                      "note": note.strip(), "hits": 0})
    return rules


def main():
    sgoargs.check(flags={"--write": "bool"})  # strict command line (W12 R14)
    write = "--write" in sys.argv
    errors = []
    registry = json.load(open(REGISTRY))
    clause_family = {cid: r for r in registry["rows"] for cid in r["clause_ids"]}
    rules = load_rules(errors)
    for r in rules:
        fam = clause_family.get(r["clause"])
        if fam is None:
            errors.append(f"INVENTORY.map:{r['line']}: unknown clause {r['clause']}")
        elif r["verdict"] in ("admit", "internal") and fam.get("subset0") == "denied" and fam.get("status") == "denied":
            errors.append(f"INVENTORY.map:{r['line']}: {r['verdict']} into wholly denied family {fam['id']}")
        if r["verdict"] == "internal" and r["kind"] in SOURCE_KINDS and not r["note"]:
            errors.append(f"INVENTORY.map:{r['line']}: 'internal' for source-visible kind {r['kind']} needs a note")
    surfaces = extract()
    mapped = []
    for s in surfaces:
        rule = next((r for r in rules if r["kind"] == s["kind"] and r["rx"].fullmatch(s["name"])), None)
        if rule is None:
            errors.append(f"unmapped {s['kind']} {s['name']} ({s['file']}:{s['line']})")
            continue
        rule["hits"] += 1
        mapped.append(dict(s, verdict=rule["verdict"], clause=rule["clause"], note=rule["note"]))
    for r in rules:
        if r["hits"] == 0:
            errors.append(f"INVENTORY.map:{r['line']}: rule matches no surface ({r['kind']} {r['rx'].pattern})")
    if errors:
        for e in errors:
            print(f"inventory: {e}", file=sys.stderr)
        print(f"inventory: error: {len(errors)} problem(s) over {len(surfaces)} surfaces", file=sys.stderr)
        sys.exit(1)

    counts = collections.Counter((m["kind"], m["verdict"]) for m in mapped)
    kinds = sorted({m["kind"] for m in mapped})
    commit = subprocess.check_output(["git", "-C", ROOT, "rev-parse", "HEAD"], text=True).strip()
    lines = ["# Surface inventory (generated)", "",
             "Generated by `sgo/scripts/inventory-check.py` from the fork's sources and `sgo/spec/INVENTORY.map` "
             f"(W01 step 3). spec_version `{registry['spec_version']}`, admission digest `{hashlib.sha256(open(os.path.join(ROOT, 'sgo', 'SGO_SPEC_ADMISSION.json'), 'rb').read()).hexdigest()[:16]}` (SHA-256 of `SGO_SPEC_ADMISSION.json`, as `sgo version` and every binary record it). "
             "Every surface maps to exactly one clause; unmapped surfaces and unused rules fail the check.", "",
             "| Kind | admit | internal | deny | total |", "|---|---|---|---|---|"]
    for k in kinds:
        a, i, d = (counts[(k, v)] for v in ("admit", "internal", "deny"))
        lines.append(f"| {k} | {a} | {i} | {d} | {a + i + d} |")
    lines.append(f"| **all** | {sum(v for (k, vv), v in counts.items() if vv == 'admit')} | "
                 f"{sum(v for (k, vv), v in counts.items() if vv == 'internal')} | "
                 f"{sum(v for (k, vv), v in counts.items() if vv == 'deny')} | {len(mapped)} |")
    for k in kinds:
        lines += ["", f"## {k}", "", "| Surface | Verdict | Clause | Source |", "|---|---|---|---|"]
        for m in sorted((m for m in mapped if m["kind"] == k), key=lambda m: (m["verdict"], m["name"])):
            lines.append(f"| `{m['name']}` | {m['verdict']} | {m['clause']} | `{m['file']}:{m['line']}` |")
    text = "\n".join(lines) + "\n"
    if write:
        os.makedirs(os.path.dirname(DOC), exist_ok=True)
        open(DOC, "w").write(text)
        print(f"inventory: wrote {os.path.relpath(DOC, ROOT)}")
    elif not os.path.exists(DOC) or open(DOC).read() != text:
        print("inventory: error: sgo/docs/inventory.md is stale; run sgo/scripts/inventory-check.py --write", file=sys.stderr)
        sys.exit(1)
    print(f"inventory: ok: {len(mapped)} surfaces mapped by {len(rules)} rules "
          f"(admit {sum(1 for m in mapped if m['verdict'] == 'admit')}, "
          f"internal {sum(1 for m in mapped if m['verdict'] == 'internal')}, "
          f"deny {sum(1 for m in mapped if m['verdict'] == 'deny')})")


if __name__ == "__main__":
    main()
