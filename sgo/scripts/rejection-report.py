#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""rejection-report.py — the intentional rejection report over the upstream language tests (D68, T05).

    rejection-report.py [--sgo PATH] [--check]

The untouched upstream suite runs only with the reference toolchain (B03). This report copies
each top-level upstream language test (test/*.go) as the main.sgo of a scratch module, runs
`sgo check` on it, and records the outcome: admitted, or the first rejection's code, clause, and
message. A file that sgo check admits and that has an expected output (test/<name>.out) is built
and run by S-Go and must match; an unexplained mismatch is a problem (exit 1). No upstream test is
changed or added to a target lane to make a number better.

Writes sgo/evidence/w10/rejection-report.json (per file) and .md (by code); --check compares the
outcomes with the committed report instead of writing it.
"""
import sgoargs
import collections
import concurrent.futures
import json
import os
import platform
import re
import subprocess
import sys
import tempfile

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
GOOS = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
GOARCH = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[platform.machine()]
SPEC = open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip()
DIAG = re.compile(r"^(\S+?):(\d+):(\d+): (SGO_E_[A-Z_]+): (.*)$", re.M)
CLAUSE = re.compile(r"\((SGO-[A-Z]+-\d+)[^)]*\)")
OUT = os.path.join(ROOT, "sgo", "evidence", "w10")


def one(sgo, name, tmp):
    d = os.path.join(tmp, name)
    os.makedirs(d)
    open(os.path.join(d, "sgo.mod"), "w").write(f"module example.com/upstream/{name}\n\nsgo {SPEC}\n")
    open(os.path.join(d, "main.sgo"), "wb").write(open(os.path.join(ROOT, "test", name + ".go"), "rb").read())
    env = {"HOME": os.environ.get("HOME", tmp), "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, ".cache")}
    r = subprocess.run([sgo, "check", "."], cwd=d, env=env, capture_output=True, text=True, timeout=300)
    if r.returncode == 0:
        res = {"outcome": "admitted"}
        out = os.path.join(ROOT, "test", name + ".out")
        if os.path.exists(out):
            b = subprocess.run([sgo, "run", "."], cwd=d, env=env, capture_output=True, text=True, timeout=300)
            res["run"] = "match" if b.returncode == 0 and b.stdout == open(out).read() else f"MISMATCH exit {b.returncode}"
        return name, res
    m = DIAG.search(r.stderr + r.stdout)
    if not m:
        return name, {"outcome": "rejected", "code": "(no diagnostic)", "message": (r.stderr + r.stdout).strip()[:200]}
    c = CLAUSE.search(m.group(5))
    return name, {"outcome": "rejected", "code": m.group(4), "clause": c.group(1) if c else "",
                  "position": f"{m.group(2)}:{m.group(3)}", "message": m.group(5)[:200]}


def main():
    sgoargs.check(flags={"--check": "bool", "--sgo": "value"})  # strict command line (W12 R14)
    args = sys.argv[1:]
    sgo = os.path.join(ROOT, "pkg", "tool", f"{GOOS}_{GOARCH}", "sgo")
    check = "--check" in args
    if "--sgo" in args:
        sgo = args[args.index("--sgo") + 1]
    names = sorted(f[:-3] for f in os.listdir(os.path.join(ROOT, "test")) if f.endswith(".go"))
    results = {}
    with tempfile.TemporaryDirectory() as tmp:
        tmp = os.path.realpath(tmp)
        # Warm the TCB cache once so the parallel checks share it.
        one(sgo, names[0], os.path.join(tmp, "warm") if os.makedirs(os.path.join(tmp, "warm")) is None else tmp)
        with concurrent.futures.ThreadPoolExecutor(max_workers=os.cpu_count() or 4) as ex:
            for name, res in ex.map(lambda n: one(sgo, n, tmp), names):
                results[name] = res
    mismatches = [n for n, r in results.items() if r.get("run", "match") != "match"]
    by_code = collections.Counter(r.get("code", "admitted") for r in results.values())
    report = {"spec_version": SPEC, "upstream_dir": "test", "files": len(results),
              "admitted": sum(1 for r in results.values() if r["outcome"] == "admitted"),
              "by_code": dict(sorted(by_code.items(), key=lambda kv: (-kv[1], kv[0]))), "results": results}
    path = os.path.join(OUT, "rejection-report.json")
    if check:
        old = json.load(open(path))["results"]
        diff = [n for n in results if results[n].get("code") != old.get(n, {}).get("code") or results[n]["outcome"] != old.get(n, {}).get("outcome")]
        if diff:
            print("rejection-report: outcomes changed: " + ", ".join(diff[:20]))
            sys.exit(1)
    else:
        os.makedirs(OUT, exist_ok=True)
        with open(path, "w") as f:
            json.dump(report, f, indent=1, ensure_ascii=False)
            f.write("\n")
        clauses = collections.defaultdict(collections.Counter)
        for r in results.values():
            if r["outcome"] == "rejected":
                clauses[r["code"]][r.get("clause", "")] += 1
        lines = [f"# Intentional rejection report (D68, T05), spec {SPEC}", "",
                 f"`sgo check` over the {len(results)} top-level upstream language tests (`test/*.go`, each copied as the",
                 "`main.sgo` of a scratch module). The untouched upstream suite runs only with the reference toolchain",
                 "(B03); no upstream test is changed or moved into a target lane.", "",
                 f"Admitted: {report['admitted']}. Admitted with an expected output and run by S-Go: "
                 f"{sum(1 for r in results.values() if 'run' in r)}, mismatches: {len(mismatches)}.", "",
                 "First rejection of each file, by code (clause named in the diagnostic):", "",
                 "| code | files | clauses |", "|---|---|---|"]
        for code, n in report["by_code"].items():
            cl = ", ".join(f"{c or '-'} ({k})" for c, k in clauses.get(code, collections.Counter()).most_common(4))
            lines.append(f"| {code} | {n} | {cl} |")
        admitted = sorted(n for n, r in results.items() if r["outcome"] == "admitted")
        stubs = [n for n in admitted if re.search(r"^package ignored\b", open(os.path.join(ROOT, "test", n + ".go")).read(), re.M)]
        lines += ["", f"Admitted files: {', '.join(admitted) or 'none'}. Of these, {len(stubs)} are `package ignored` stubs of",
                  "directory tests (their code is in `<name>.dir/`, which this report does not cover); the rest declare",
                  "no code that needs admitting. Scope: top-level `test/*.go` only; subdirectories and `.dir` trees",
                  "are not included.", "", "Per-file outcomes: `rejection-report.json`."]
        with open(os.path.join(OUT, "rejection-report.md"), "w") as f:
            f.write("\n".join(lines) + "\n")
    print(f"rejection-report: {len(results)} files, {report['admitted']} admitted, {len(mismatches)} mismatches; "
          + ", ".join(f"{k} {v}" for k, v in list(report["by_code"].items())[:6]))
    if mismatches:
        print("rejection-report: MISMATCH: " + ", ".join(mismatches))
        sys.exit(1)


if __name__ == "__main__":
    main()
