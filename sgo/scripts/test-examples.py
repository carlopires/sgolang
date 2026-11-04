#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""test-examples.py — the D66 examples build, test, run, and behave under the lab (W10).

For every example in sgo/examples (or in a release, with --root): sgo check ./..., sgo test ./...
(every test passes, quiescence included), sgo run of its main package(s) printing expected.txt,
and a lab sweep that passes every seed; for replay also: the sweep of ./buggy fails, the
committed trace replays to the buggy output, and replaying it against ./fixed diverges.

    test-examples.py [--sgo PATH] [--examples DIR]
"""
import sgoargs
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))  # no git needed: also runs against a release

# name -> (main packages that print expected.txt, lab sweep seeds)
EXAMPLES = {
    "pipeline": (["."], "0..63"),
    "service": (["."], "0..63"),
    "replay": (["./fixed"], "0..63"),
    "happyeyeballs": (["."], "0..63"),
}

# Happy Eyeballs lab scenarios (W18, D146): each main checks its own winner and RFC 8305 step, so
# the sweep needs no -expect (the order in which the two DNS answers arrive varies by schedule).
HE_SCENARIOS = ["silent-v6", "slow-aaaa", "prefer-v6", "slow-v6", "all-refused"]


def main():
    sgoargs.check(flags={"--sgo": "value", "--examples": "value"})  # strict command line (W12 R14)
    args = sys.argv[1:]
    sgo = os.path.join(ROOT, "pkg", "tool", "darwin_arm64", "sgo")
    examples = os.path.join(ROOT, "sgo", "examples")
    while args:
        if args[0] == "--sgo":
            sgo, args = args[1], args[2:]
        elif args[0] == "--examples":
            examples, args = args[1], args[2:]
        else:
            sys.exit(f"usage: test-examples.py [--sgo PATH] [--examples DIR]")
    problems = []
    env = {"PATH": "/usr/bin:/bin", "HOME": os.environ.get("HOME", "/tmp")}
    if os.environ.get("TMPDIR"):  # lets a shared or CI host isolate the driver's temporary files
        env["TMPDIR"] = os.environ["TMPDIR"]
    with tempfile.TemporaryDirectory() as tmp:
        for name, (mains, seeds) in EXAMPLES.items():
            d = os.path.join(tmp, name)
            shutil.copytree(os.path.join(examples, name), d, symlinks=True)
            expected = open(os.path.join(d, "expected.txt")).read()

            def run(*a):
                return subprocess.run([sgo, *a], cwd=d, env=env, capture_output=True, text=True, timeout=600)

            r = run("check", "./...")
            if r.returncode != 0:
                problems.append(f"{name}: sgo check: {r.stderr.strip()[:300]}")
            if name == "happyeyeballs":
                # The real run depends on the host: localhost is answered from /etc/hosts, which may
                # or may not list ::1 (W18 review Y1). Assert what holds on every host: exit 0, the
                # final line of expected.txt, well-formed race lines, a failed IPv6 attempt if one
                # was made, and the IPv4 connection. The exact attempt order is the lab's to check.
                r = run("run", ".")
                lines = r.stdout.splitlines()
                last = expected.splitlines()[-1]
                race = lines[:-1]
                ok = r.returncode == 0 and lines[-1:] == [last] and "he: connected 127.0.0.1:47970" in race
                ok = ok and all(re.fullmatch(r"he: (attempt|failed|connected) (\[::1\]|127\.0\.0\.1):47970", x) for x in race)
                ok = ok and ("he: attempt [::1]:47970" not in race or "he: failed [::1]:47970" in race)
                if not ok:
                    problems.append(f"{name}: sgo run: exit {r.returncode}, stdout {r.stdout!r}, want a race ending {last!r} (reference {expected!r}); {r.stderr[-200:]!r}")
                for sc in HE_SCENARIOS:
                    r = run("lab", "sweep", "-seeds", seeds, "./lab/" + sc)
                    if r.returncode != 0 or " 0 failed" not in r.stderr + r.stdout:
                        problems.append(f"{name}: lab sweep {sc}: exit {r.returncode}: {(r.stdout + r.stderr)[-300:]!r}")
                continue
            r = run("test", "./...")
            if r.returncode != 0 or "--- FAIL" in r.stdout or "--- PASS" not in r.stdout:
                problems.append(f"{name}: sgo test: exit {r.returncode}: {r.stdout[-400:]!r} {r.stderr[-300:]!r}")
            for m in mains:
                r = run("run", m)
                if r.returncode != 0 or r.stdout != expected:
                    problems.append(f"{name}: sgo run {m}: exit {r.returncode}, stdout {r.stdout!r}, want {expected!r}; {r.stderr[-200:]!r}")
                r = run("lab", "sweep", "-seeds", seeds, "-expect", "expected.txt", m)
                if r.returncode != 0 or " 0 failed" not in r.stderr + r.stdout:
                    problems.append(f"{name}: sgo lab sweep {m}: exit {r.returncode}: {(r.stdout + r.stderr)[-300:]!r}")
            if name == "replay":
                r = run("lab", "sweep", "-seeds", seeds, "-expect", "expected.txt", "-save", os.path.join(tmp, "f.jsonl"), "./buggy")
                if r.returncode == 0:
                    problems.append("replay: the sweep of ./buggy found no failing schedule")
                # The committed trace names the build that recorded it (D46), so it replays only with
                # that build. With any build, the same seed must give the same canonical events, and
                # that trace must replay to the buggy output and diverge against the fix.
                committed = open(os.path.join(d, "buggy.trace.jsonl")).read().splitlines()
                head = json.loads(committed[0])
                fresh_path = os.path.join(tmp, "seed.jsonl")
                run("lab", "sweep", "-seeds", f"{head['seed']}..{head['seed']}", "-expect", "expected.txt", "-save", fresh_path, "./buggy")
                fresh = open(fresh_path).read().splitlines() if os.path.exists(fresh_path) else []
                fh = json.loads(fresh[0]) if fresh else {}
                same_build = fh.get("build") == head["build"]
                fh.pop("build", None)
                head.pop("build", None)
                if not fresh or fh != head or fresh[1:] != committed[1:]:
                    problems.append("replay: the committed trace's seed no longer gives the same events")
                traces = ["buggy.trace.jsonl", fresh_path] if same_build else [fresh_path]
                for tr in traces:
                    r = run("lab", "replay", tr, "./buggy")
                    if r.returncode != 0 or r.stdout != "debit commit credit\n":
                        problems.append(f"replay: replay of {os.path.basename(tr)}: exit {r.returncode}, stdout {r.stdout!r}: {r.stderr[-300:]!r}")
                r = run("lab", "replay", fresh_path, "./fixed")
                if "SGO_E_LAB_DIVERGENCE" not in r.stderr:
                    problems.append(f"replay: replaying the buggy trace against ./fixed did not diverge: {r.stderr[-300:]!r}")
    for p in problems:
        print("test-examples: " + p)
    if problems:
        sys.exit(f"test-examples: {len(problems)} problem(s)")
    print(f"test-examples: ok: {', '.join(EXAMPLES)}: check, test, run, lab sweep; replay found, reproduced, and diverges after the fix")


if __name__ == "__main__":
    main()
