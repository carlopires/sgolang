#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""guarantee-check.py - the published claim set against its evidence (D76; Q04, S20).

    guarantee-check.py [--release]   check sgo/GUARANTEES.json; exit 1 on any problem
    guarantee-check.py render        write sgo/docs/GUARANTEES.md from the JSON

Checks:
  - schema: every guarantee has a statement, families, implementation paths that exist, and a
    scope drawn only from scope_vocabulary; model_explored needs stated model_bounds (S20);
  - coverage: every row of SGO_SPEC_ADMISSION.json belongs to exactly one guarantee; denied rows
    only to G-DENY and the registry status of each row agrees (admitted or denied);
  - clauses: every clause of a covered family is frozen or superseded, and no frozen clause still
    says 'none yet' in its progress fields;
  - cases: each family's acceptance cases are passed in SGO_TEST_MATRIX.json, except cases named
    by a known limitation and, without --release, cases owned by W12;
  - matrix honesty: a case whose last required work package recorded '<WP>: passed' must be
    passed, and a passed case must cite evidence files that exist;
  - stack: stack_implementation agrees with W11's status in SGO_TASKS.json.
"""
import sgoargs
import json
import os
import re
import sys

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
SGO = os.path.join(ROOT, "sgo")
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))


def load(name):
    with open(os.path.join(SGO, name)) as f:
        return json.load(f)


def clauses():
    """Clause id -> (family, status, progress text) parsed from sgo/spec/SPEC-*.md."""
    out = {}
    head = re.compile(r"^### (SGO-[A-Z]+-\d+) ")
    for fn in sorted(os.listdir(os.path.join(SGO, "spec"))):
        if not (fn.startswith("SPEC-") and fn.endswith(".md")):
            continue
        fam = fn[:-3]
        cur = None
        for line in open(os.path.join(SGO, "spec", fn)):
            m = head.match(line)
            if m:
                cur = m.group(1)
                out[cur] = {"family": fam, "status": None, "progress": ""}
            elif cur and line.startswith("- **status:**"):
                out[cur]["status"] = line.split("**status:**", 1)[1].strip()
            elif cur and (line.startswith("- **implementation_coverage:**") or
                          line.startswith("- **executed_evidence:**")):
                out[cur]["progress"] += line
    return out


def matrix_cases():
    m = load("SGO_TEST_MATRIX.json")
    found = {}

    def walk(o):
        if isinstance(o, dict):
            if "execution_status" in o:
                found[o["id"]] = o
            for v in o.values():
                walk(v)
        elif isinstance(o, list):
            for v in o:
                walk(v)
    walk(m)
    return found


EVPATH = re.compile(r"(sgo/(?:evidence|docs)/[\w./-]+?\.(?:md|json|jsonl))")


def check(release):
    errs = []
    g = load("GUARANTEES.json")
    reg = load("SGO_SPEC_ADMISSION.json")
    tasks = load("SGO_TASKS.json")
    cases = matrix_cases()
    cls = clauses()
    vocab = set(g["scope_vocabulary"])
    rows = {r["id"]: r for r in reg["rows"]}
    limited = set()
    for lim in g["known_limitations"]:
        for c in lim["case"].split():
            if c not in cases:
                errs.append(f"limitation {lim['id']}: unknown case {c}")
            limited.add(c)

    owner = {}
    for gu in g["guarantees"]:
        gid = gu["id"]
        for f in ("statement", "families", "scope", "implementation"):
            if not gu.get(f):
                errs.append(f"{gid}: missing {f}")
        for s in gu.get("scope", []):
            if s not in vocab:
                errs.append(f"{gid}: scope {s!r} not in scope_vocabulary")
        if "model_explored" in gu.get("scope", []) and not gu.get("model_bounds"):
            errs.append(f"{gid}: model_explored without model_bounds")
        if re.search(r"\b(prove[sd]?|proof|all schedules|always terminates)\b", gu.get("statement", ""), re.I):
            errs.append(f"{gid}: statement claims proof (S20); state the actual scope")
        for p in gu.get("implementation", []):
            if not os.path.exists(os.path.join(ROOT, p)):
                errs.append(f"{gid}: implementation path {p} does not exist")
        for fam in gu.get("families", []):
            if fam not in rows:
                errs.append(f"{gid}: unknown family {fam}")
            elif fam in owner:
                errs.append(f"{fam}: in both {owner[fam]} and {gid}")
            else:
                owner[fam] = gid

    for fam, row in rows.items():
        gid = owner.get(fam)
        if gid is None:
            errs.append(f"{fam}: in the admission registry but in no guarantee")
            continue
        want = "denied" if gid == "G-DENY" else "admitted"
        if row["status"] != want:
            errs.append(f"{fam}: registry status {row['status']!r}, guarantee {gid} needs {want!r} (SR2)")
        for cid in row["clause_ids"]:
            c = cls.get(cid)
            if c is None:
                errs.append(f"{fam}: clause {cid} not found in sgo/spec")
            elif c["status"] not in ("frozen", "superseded"):
                errs.append(f"{cid}: status {c['status']!r}; a published guarantee needs frozen clauses")
            elif c["status"] == "frozen" and "none yet" in c["progress"]:
                errs.append(f"{cid}: implementation_coverage/executed_evidence still say 'none yet' (SR3)")
        for cid in row["acceptance_case_ids"]:
            c = cases.get(cid)
            if c is None:
                errs.append(f"{fam}: unknown acceptance case {cid}")
                continue
            if c["execution_status"] == "passed" or cid in limited:
                continue
            if not release and "W12" in c["required_for_work_packages"]:
                continue
            errs.append(f"{fam}: acceptance case {cid} is {c['execution_status']}")

    for cid, c in cases.items():
        last = c["required_for_work_packages"][-1]
        ev = c.get("evidence", [])
        if c["execution_status"] != "passed" and any(e.startswith(last + ": passed") for e in ev):
            errs.append(f"{cid}: '{last}: passed' evidence but status {c['execution_status']} (SR1)")
        if c["execution_status"] == "passed":
            paths = [p for e in ev for p in EVPATH.findall(e)]
            if not paths:
                errs.append(f"{cid}: passed without an evidence file")
            for p in paths:
                if not os.path.exists(os.path.join(ROOT, p)):
                    errs.append(f"{cid}: evidence file {p} does not exist")

    w11 = next(t for t in tasks["tasks"] if t["id"] == "W11")
    if g["stack_implementation"]["w11_outcome"] != w11["status"]:
        errs.append(f"stack_implementation.w11_outcome {g['stack_implementation']['w11_outcome']!r} "
                    f"!= W11 status {w11['status']!r}")
    return errs


QUESTIONS = [
    ("What can I compile?", ["G-LANG", "G-MEM", "G-DENY"]),
    ("Which operations are cancellation-aware?", ["G-CANCEL", "G-CHAN", "G-IO"]),
    ("Which effects are mediated?", ["G-AUTH", "G-IO"]),
    ("What exactly can I replay?", ["G-LAB"]),
    ("What is rejected?", ["G-DENY", "G-TRUST", "G-TOOL"]),
    ("What happens on a stuck task?", ["G-CANCEL", "G-CHAN"]),
]


def render():
    g = load("GUARANTEES.json")
    reg = {r["id"]: r for r in load("SGO_SPEC_ADMISSION.json")["rows"]}
    byid = {x["id"]: x for x in g["guarantees"]}
    out = ["# S-Go guarantees", "",
           "Generated from `sgo/GUARANTEES.json` by `sgo/scripts/guarantee-check.py render`. Do not edit.",
           "", g["purpose"], "", "## Scope of a claim", ""]
    out += [f"- **{k}**: {v}" for k, v in g["scope_vocabulary"].items()]
    out += ["", "## Not claimed", ""] + [f"- {x}" for x in g["not_claimed"]]
    out += ["", "## Questions a release must answer (plan §15.2)", ""]
    for q, ids in QUESTIONS:
        out.append(f"- **{q}** See " + ", ".join(f"[{i}](#{i.lower()})" for i in ids) + ".")
    out.append("- **Which tests demonstrated each answer?** Each guarantee lists its acceptance cases; "
               "`SGO_TEST_MATRIX.json` holds their executed evidence.")
    out += ["", "## Guarantees", ""]
    for gu in g["guarantees"]:
        cs = sorted({c for f in gu["families"] for c in reg[f]["acceptance_case_ids"]})
        cl = [c for f in gu["families"] for c in reg[f]["clause_ids"]]
        out += [f"### {gu['id']}", "", gu["statement"], "",
                f"- **Scope:** {', '.join(gu['scope'])}"]
        if gu.get("model_bounds"):
            out.append(f"- **Model bounds:** {gu['model_bounds']}")
        out += [f"- **Families:** {', '.join(gu['families'])}",
                f"- **Clauses:** {', '.join(cl)}",
                f"- **Acceptance cases:** {', '.join(cs)}",
                f"- **Implementation:** " + ", ".join(f"`{p}`" for p in gu["implementation"])]
        out += [f"- **Limitation:** {x}" for x in gu.get("limitations", [])]
        out.append("")
    s = g["stack_implementation"]
    out += ["## Identity", ""] + [f"- **{k}:** {v}" for k, v in g["identity"].items()]
    out += [f"- **stack implementation:** {s['kind']} ({s['clause']}; W11 {s['w11_outcome']}, {s['evidence']})",
            f"- **targets:** {', '.join(g['targets'])}", "", "## Known limitations", ""]
    out += [f"- **{x['id']}** ({x['case']}): {x['text']}" for x in g["known_limitations"]]
    with open(os.path.join(SGO, "docs", "GUARANTEES.md"), "w") as f:
        f.write("\n".join(out) + "\n")


def main():
    sgoargs.check(flags={"--release": "bool"}, positional={"render"}, max_positional=1)  # strict command line (W12 R14)
    if sys.argv[1:] == ["render"]:
        render()
        return 0
    errs = check("--release" in sys.argv[1:])
    for e in errs:
        print("guarantee-check:", e, file=sys.stderr)
    if errs:
        print(f"guarantee-check: FAIL ({len(errs)} problems)", file=sys.stderr)
        return 1
    print("guarantee-check: ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
