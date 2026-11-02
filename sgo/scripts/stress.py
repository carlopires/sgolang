#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""stress.py - the W12 stress lanes (D80).

    stress.py [--seeds N] [--count N] [--cpus 1,4,16] [--skip-production] [--out FILE]

Lab: every runnable lab conformance case is swept over seeds 0..N-1 (default 1000) with the case's
own lab arguments (its pinned -seed dropped). Every run must end with the case's specified exit
status and no runtime crash; a case whose expected output does not depend on the seed must give it
on every seed, and a case with an admitted outcome set must stay inside it. Each sweep runs twice
and must report the same outcomes (determinism). Each D66 example is swept over 0..N-1 too.

Production: the native channel, cancellation, nursery, I/O, and network runtime tests run with
-count=N (default 20) at each GOMAXPROCS of --cpus (go test -cpu), and the same under the race
detector with a fifth of the count.

Reduced counts on a noisy host are allowed and recorded (plan §12.2); no scenario is dropped. The
JSON summary is written to --out (default $SGO_OUT/evidence/w12/stress-<os>-<arch>.json).
"""
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time

from sgotool import GOARCH, GOOS, ROOT

SGO = os.path.join(ROOT, "pkg", "tool", f"{GOOS}_{GOARCH}", "sgo")
CASES = os.path.join(ROOT, "sgo", "testdata", "conformance")
EXAMPLES = os.path.join(ROOT, "sgo", "examples")
# Cases outside the sweep lane, with the reason: they never run a program, or they hang by design.
SKIP = {"lab-flag": "compile-time refusal", "lab-pair-spawn-nil-callback": "compile-time refusal",
        "lab-rejects-like-production": "compile-time refusal",
        "lab-host-hang": "hangs by design; the timeout path is checked by conformance"}
# Cases whose output depends on the seed by design (only the exit status is fixed).
SEED_DEPENDENT = {"lab-ready-set", "lab-select-source-order"}
PROD_TESTS = "TestSGoNative|TestSGoChan|TestSGoPermit|TestSGoCancel|TestSGoNursery|TestSGoScope|TestSGoHandle|TestSGoDir|TestSGoResol|TestSGoRename|TestSGoNet|TestSGoReady|TestSGoSync"
OUTCOME = re.compile(r'^(pass|FAIL) exit=(\d+) runs=(\d+) first-seed=(\d+) stdout="((?:[^"\\]|\\.)*)"(?: record=(.*))?$', re.M)
CRASH = re.compile(r"fatal error:|SIGSEGV|unexpected fault address|runtime: ")


def arg(name, default):
    return sys.argv[sys.argv.index(name) + 1] if name in sys.argv else default


def lab_args(e):
    a = list(e.get("lab_args") or [])
    if "-seed" in a:
        i = a.index("-seed")
        del a[i:i + 2]
    return a


def sweep(d, args, seeds, env):
    r = subprocess.run([SGO, "lab", "sweep", "-seeds", f"0..{seeds - 1}"] + args + ["."], cwd=d, env=env,
                       capture_output=True, text=True, timeout=3600)
    text = r.stdout + r.stderr
    outs = []
    for m in OUTCOME.finditer(text):
        kind = json.loads(m.group(6)).get("kind") if m.group(6) else None
        outs.append((int(m.group(2)), json.loads('"' + m.group(5) + '"'), int(m.group(3)), kind))
    return outs, text


def lab_lane(seeds, env, results):
    problems = []
    for name in sorted(os.listdir(CASES)):
        if not name.startswith("lab-") or name in SKIP:
            continue
        e = json.load(open(os.path.join(CASES, name, "expect.json")))
        with tempfile.TemporaryDirectory() as tmp:
            d = os.path.join(tmp, name)
            shutil.copytree(os.path.join(CASES, name), d, symlinks=True)
            t0 = time.time()
            outs, text = sweep(d, lab_args(e), seeds, env)
            outs2, _ = sweep(d, lab_args(e), seeds, env)
        runs = sum(o[2] for o in outs)
        res = {"seeds": seeds, "runs": runs, "outcomes": len(outs), "seconds": round(time.time() - t0, 1)}
        results["lab"][name] = res
        if runs != seeds:
            problems.append(f"{name}: {runs} of {seeds} runs reported: {text[-300:]!r}")
            continue
        if CRASH.search(text):
            problems.append(f"{name}: runtime crash in the sweep: {CRASH.search(text).group(0)}")
        if outs != outs2:
            problems.append(f"{name}: two sweeps of the same seeds gave different outcomes")
        exits = {e["exit"]} if "exit" in e else {0}
        if "sweep" in e:
            admitted = {(ex, s) for s in e["sweep"]["outcomes"] for ex in (0, e.get("exit", 0))}
            bad = [(x, s) for x, s, _, _ in outs if (x, s) not in admitted]
        elif name in SEED_DEPENDENT or "stdout" not in e:
            bad = [(x, s) for x, s, _, _ in outs if x not in exits]
        else:
            bad = [(x, s) for x, s, _, _ in outs if (x, s) != (e["exit"], e["stdout"])]
        # A specified failure record keeps its kind on every seed.
        if "stderr_kind" in e:
            bad += [(x, k) for x, _, _, k in outs if k != e["stderr_kind"]]
        if bad:
            problems.append(f"{name}: outcomes outside the admitted set: {bad[:3]}")
    for ex in sorted(os.listdir(EXAMPLES)):
        d0 = os.path.join(EXAMPLES, ex)
        mains = [m for m in (".", "./fixed") if os.path.exists(os.path.join(d0, m, "main.sgo"))]
        for m in mains:
            r = subprocess.run([SGO, "lab", "sweep", "-seeds", f"0..{seeds - 1}", "-expect", "expected.txt", m],
                               cwd=d0, env=env, capture_output=True, text=True, timeout=3600)
            results["lab"][f"example {ex} {m}"] = {"seeds": seeds, "ok": r.returncode == 0}
            if r.returncode != 0 or " 0 failed" not in r.stdout + r.stderr:
                problems.append(f"example {ex} {m}: sweep failed: {(r.stdout + r.stderr)[-300:]!r}")
    return problems


def production_lane(count, cpus, results):
    problems = []
    lane = os.path.join(ROOT, "sgo", "scripts", "runtime-lane.sh")
    for label, args in [("native", ["-count", str(count), "-cpu", cpus]),
                        ("race", ["-race", "-count", str(max(1, count // 5)), "-cpu", cpus])]:
        t0 = time.time()
        r = subprocess.run([lane] + args + ["-timeout", "120m", "-run", PROD_TESTS, "runtime"], cwd=ROOT, capture_output=True, text=True)
        ok = r.returncode == 0 and re.search(r"^ok\s+runtime", r.stdout, re.M) is not None
        results["production"][label] = {"count": int(args[args.index("-count") + 1]), "cpus": cpus, "ok": ok,
                                         "seconds": round(time.time() - t0, 1)}
        if not ok:
            problems.append(f"production {label}: {(r.stdout + r.stderr)[-600:]}")
    return problems


def main():
    seeds, count, cpus = int(arg("--seeds", "1000")), int(arg("--count", "20")), arg("--cpus", "1,4,16")
    out = arg("--out", os.path.join(os.environ.get("SGO_OUT") or os.path.expanduser("~/.cache/sgolang/out"),
                                    "evidence", "w12", f"stress-{GOOS}-{GOARCH}.json"))
    results = {"platform": f"{GOOS}/{GOARCH}", "skipped": SKIP, "lab": {}, "production": {}}
    with tempfile.TemporaryDirectory() as home:
        env = {"HOME": home, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(home, "cache")}
        problems = lab_lane(seeds, env, results)
    if "--skip-production" not in sys.argv:
        problems += production_lane(count, cpus, results)
    results["problems"] = problems
    os.makedirs(os.path.dirname(out), exist_ok=True)
    json.dump(results, open(out, "w"), indent=1)
    for p in problems:
        print("stress: " + p)
    print(f"stress: {'ok' if not problems else 'FAIL'}: {len(results['lab'])} lab sweeps of {seeds} seeds; "
          f"production {results['production'] or 'skipped'}; {len(problems)} problem(s); summary {out}")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
