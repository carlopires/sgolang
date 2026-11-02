#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""adoption-check.py — S04 / D19: the upstream Go-spec sections that S-Go clauses adapt are recorded with
digests in sgo/docs/clause-adoption-ledger.json; this check recomputes them from doc/go_spec.html at the
locked upstream commit.

    adoption-check.py           check (CI)
    adoption-check.py --write   recompute digests and regenerate the ledger (reviewed change)

A section's text is the HTML from its heading to the next heading of the same or higher level.
Adapted wording is reproduced in the S-Go clause itself; the clause never refers to the Go spec for meaning.
"""
import hashlib
import json
import os
import re
import subprocess
import sys

ROOT = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
LEDGER = os.path.join(ROOT, "sgo", "docs", "clause-adoption-ledger.json")
LOCK = json.load(open(os.path.join(ROOT, "sgo", "UPSTREAM.lock.json")))

# clause -> (Go spec section ids, what was adopted, what S-Go changed)
ADOPTIONS = {
    "SGO-LEX-002": (["Comments", "Tokens", "Semicolons", "Identifiers", "Operators_and_punctuation"],
                    "comment, token, identifier, and automatic-semicolon rules",
                    "operators outside subset 0 are recognized only to be denied"),
    "SGO-LEX-004": (["Integer_literals", "String_literals"],
                    "integer and string literal syntax",
                    "no legacy octal, rune, float, imaginary literals; escape set reduced"),
    "SGO-BIND-004": (["Declarations_and_scope", "Blocks"],
                     "block and scope structure",
                     "shadowing of enclosing locals and predeclared names is an error; unused locals are errors"),
    "SGO-VAL-001": (["Arithmetic_operators", "Integer_operators"],
                    "truncated division and remainder, shift and bitwise meaning",
                    "overflow, division by zero, and out-of-range shifts are checked failures (D02)"),
    "SGO-VAL-004": (["Constant_expressions"],
                    "exact untyped constant arithmetic",
                    "iota denied; typed int only"),
    "SGO-CTL-002": (["Expression_switches"],
                    "case evaluation order and matching",
                    "no fallthrough; tag types int, string, bool only"),
    "SGO-CTL-003": (["Terminating_statements"],
                    "terminating-statement rules",
                    "adds the nursery statement; unreachable statements are errors"),
}


def spec_html(write):
    """Check mode reads the working tree (works in shallow CI checkouts). Write mode also requires the
    working-tree file to equal the locked upstream commit's, so a check pass implies upstream text."""
    work = open(os.path.join(ROOT, "doc", "go_spec.html")).read()
    if write:
        up = subprocess.check_output(["git", "-C", ROOT, "show", f"{LOCK['upstream']['commit']}:doc/go_spec.html"], text=True)
        if up != work:
            sys.exit("adoption-check: error: doc/go_spec.html differs from the locked upstream commit")
    return work


def section(html, sid):
    m = re.search(r'<h([1-6]) id="%s">' % re.escape(sid), html)
    if not m:
        raise KeyError(sid)
    level = int(m.group(1))
    nxt = re.compile(r"<h([1-6]) id=")
    pos = m.end()
    for n in nxt.finditer(html, pos):
        if int(n.group(1)) <= level:
            return html[m.start():n.start()]
    return html[m.start():]


def build(write):
    html = spec_html(write)
    entries = []
    for clause, (ids, adopted, changed) in sorted(ADOPTIONS.items()):
        secs = [{"section": sid, "sha256": hashlib.sha256(section(html, sid).encode()).hexdigest()} for sid in ids]
        entries.append({"clause": clause, "source_sections": secs, "adopted": adopted, "sgo_changes": changed})
    return {"schema_version": 1, "decision": "D19", "source": "doc/go_spec.html",
            "upstream_commit": LOCK["upstream"]["commit"],
            "rule": "Adapted wording is reproduced and self-contained in each clause; no clause defers meaning to the Go spec.",
            "adoptions": entries}


def main():
    new = build("--write" in sys.argv)
    if "--write" in sys.argv:
        json.dump(new, open(LEDGER, "w"), indent=1)
        open(LEDGER, "a").write("\n")
        print(f"adoption-check: wrote {os.path.relpath(LEDGER, ROOT)}: {len(new['adoptions'])} clauses")
        return 0
    old = json.load(open(LEDGER))
    if old != new:
        print("adoption-check: error: ledger differs from the locked upstream spec sections (S04); review and --write", file=sys.stderr)
        return 1
    print(f"adoption-check: ok: {len(new['adoptions'])} adopting clauses, "
          f"{sum(len(e['source_sections']) for e in new['adoptions'])} digest-locked upstream sections")
    return 0


if __name__ == "__main__":
    sys.exit(main())
