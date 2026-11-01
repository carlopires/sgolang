#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""W08: the lab execution configuration end to end through the sgo driver (D41-D49).

Checks, with real lab binaries:
  - sgo lab run writes a trace (header, canonical lines, footer digest, host sidecar) and exits
    with the program's status; the same seed gives the same trace;
  - sgo lab replay reproduces a run from its trace; a trace whose choices were changed (footer
    recomputed) diverges (SGO_E_LAB_DIVERGENCE); an altered trace (SGO_E_LAB_TRACE) and a trace of
    another build (SGO_E_LAB_MANIFEST) are refused before running;
  - LabDeadlock and LabQuotaExhausted end a run with one diagnostic record and status 2 (D47);
  - D13: a sweep finds the schedule of an ordering bug, saves its trace, and replay reproduces it;
  - a production binary given a lab manifest on fd 3 behaves exactly as without it (D41).
"""

import hashlib
import json
import os
import platform
import subprocess
import sys
import tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
GOOS = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
GOARCH = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}[platform.machine()]
SGO = os.path.join(ROOT, "pkg", "tool", f"{GOOS}_{GOARCH}", "sgo")
VERSION = open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip()

PROGRAMS = {
    "rendezvous": '''package main

import "sgo"

func main(root *sgo.Context) error {
	out := root.Stdout()
	c := make(chan string)
	nursery (n) {
		n.Spawn(func(task *sgo.Context) error {
			c <- "ping"
			c <- "pong"
			return nil
		})
		a := <-c
		b := <-c
		if err := out.WriteString(a + " " + b + "\\n"); err != nil {
			return err
		}
	}
	return nil
}
''',
    "deadlock": '''package main

import "sgo"

func main(root *sgo.Context) error {
	c := make(chan int)
	<-c
	return nil
}
''',
    "spin": '''package main

import "sgo"

func main(root *sgo.Context) error {
	n := 0
	for {
		n = n + 1
		if n < 0 {
			return sgo.NewError("unreachable")
		}
	}
}
''',
    "order": '''package main

import "sgo"

func main(root *sgo.Context) error {
	out := root.Stdout()
	nursery (n) {
		n.Spawn(func(task *sgo.Context) error {
			return out.WriteString("a")
		})
		n.Spawn(func(task *sgo.Context) error {
			return out.WriteString("b")
		})
	}
	return out.WriteString("\\n")
}
''',
}


def fail(msg):
    sys.exit(f"test-lab: {msg}")


def sgo(args, cwd, env, **kw):
    return subprocess.run([SGO] + args, cwd=cwd, env=env, capture_output=True, text=True, timeout=600, **kw)


def records(stderr):
    return [json.loads(l) for l in stderr.splitlines() if l.startswith('{"sgo_diagnostic"')]


def rewrite(src, dst, edit):
    """Copies trace src to dst with edit applied to its canonical lines and the footer recomputed."""
    lines = open(src).read().splitlines()
    head, events, foot = lines[0], lines[1:-1], json.loads(lines[-1])
    events = edit(events)
    foot["events"] = len(events)
    foot["sha256"] = hashlib.sha256("".join(e + "\n" for e in events).encode()).hexdigest()
    open(dst, "w").write("\n".join([head] + events + [json.dumps(foot, separators=(",", ":"))]) + "\n")


def main():
    with tempfile.TemporaryDirectory() as tmp:
        tmp = os.path.realpath(tmp)
        env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, "cache"), "GOROOT": ROOT}
        dirs = {}
        for name, src in PROGRAMS.items():
            d = os.path.join(tmp, name)
            os.makedirs(d)
            open(os.path.join(d, "sgo.mod"), "w").write(f"module example.com/{name}\n\nsgo {VERSION}\n")
            open(os.path.join(d, "main.sgo"), "w").write(src)
            dirs[name] = d

        # Run, determinism, sidecar.
        d = dirs["rendezvous"]
        r = sgo(["lab", "run", "-seed", "7", "-trace", "a.jsonl", "."], d, env)
        if r.returncode != 0 or r.stdout != "ping pong\n":
            fail(f"lab run: exit {r.returncode}, stdout {r.stdout!r}, stderr {r.stderr!r}")
        r = sgo(["lab", "run", "-seed", "7", "-trace", "b.jsonl", "."], d, env)
        a, b = open(os.path.join(d, "a.jsonl")).read(), open(os.path.join(d, "b.jsonl")).read()
        if a != b:
            fail("two runs of one seed wrote different traces")
        head = json.loads(a.splitlines()[0])
        if "config=lab" not in head["build"] or head["seed"] != 7:
            fail(f"trace header {head}")
        host = json.load(open(os.path.join(d, "a.jsonl.host.json")))
        if host["hang"] or host["status"] != 0 or "wall_ms" not in host:
            fail(f"sidecar {host}")
        if "wall" in a or tmp in a:
            fail("host data in the trace")

        # Replay, divergence, refusals.
        r = sgo(["lab", "replay", "a.jsonl", "."], d, env)
        if r.returncode != 0 or "replay reproduced" not in r.stderr:
            fail(f"replay: exit {r.returncode}, stderr {r.stderr!r}")

        def flip(events):
            for i, e in enumerate(events):
                ev = json.loads(e)
                if ev.get("e") == "choose" and len(ev["ready"]) > 1:
                    ev["pick"] = [x for x in ev["ready"] if x != ev["pick"]][0]
                    events[i] = json.dumps(ev, separators=(",", ":"))
                    return events
            fail("no choice to change in the trace")
        rewrite(os.path.join(d, "a.jsonl"), os.path.join(d, "flip.jsonl"), flip)
        r = sgo(["lab", "replay", "flip.jsonl", "."], d, env)
        if r.returncode != 1 or "SGO_E_LAB_DIVERGENCE" not in r.stderr or "last matching event" not in r.stderr:
            fail(f"changed choice: exit {r.returncode}, stderr {r.stderr!r}")
        open(os.path.join(d, "cut.jsonl"), "w").write("".join(l + "\n" for l in a.splitlines()[:-3] + a.splitlines()[-1:]))
        r = sgo(["lab", "replay", "cut.jsonl", "."], d, env)
        if r.returncode != 1 or "SGO_E_LAB_TRACE" not in r.stderr:
            fail(f"altered trace: exit {r.returncode}, stderr {r.stderr!r}")
        lines = a.splitlines()
        other = dict(head, build=head["build"].replace("config=lab", "config=lab/drain=1"))
        open(os.path.join(d, "other.jsonl"), "w").write("\n".join([json.dumps(other)] + lines[1:]) + "\n")
        r = sgo(["lab", "replay", "other.jsonl", "."], d, env)
        if r.returncode != 1 or "SGO_E_LAB_MANIFEST" not in r.stderr:
            fail(f"trace of another build: exit {r.returncode}, stderr {r.stderr!r}")

        # Lab outcomes (D47).
        for name, args, kind in [("deadlock", [], "LabDeadlock"), ("spin", ["-quota", "5000"], "LabQuotaExhausted")]:
            r = sgo(["lab", "run"] + args + ["-trace", "t.jsonl", "."], dirs[name], env)
            recs = records(r.stderr)
            last = json.loads(open(os.path.join(dirs[name], "t.jsonl")).read().splitlines()[-2])
            if r.returncode != 2 or [x["kind"] for x in recs] != [kind] or last != {"e": "outcome", "status": 2, "kind": kind}:
                fail(f"{name}: exit {r.returncode}, records {recs}, last event {last}")

        # D13: a sweep finds the ordering bug; replay reproduces it.
        d = dirs["order"]
        open(os.path.join(d, "want.txt"), "w").write("ab\n")
        r = sgo(["lab", "sweep", "-seeds", "0..31", "-expect", "want.txt", "-save", "fail.jsonl", "."], d, env)
        if r.returncode != 1 or 'stdout="ba\\n"' not in r.stdout or 'stdout="ab\\n"' not in r.stdout:
            fail(f"sweep: exit {r.returncode}, stdout {r.stdout!r}")
        r = sgo(["lab", "replay", "fail.jsonl", "."], d, env)
        if r.returncode != 0 or r.stdout != "ba\n":
            fail(f"replay of the saved failure: exit {r.returncode}, stdout {r.stdout!r}, stderr {r.stderr!r}")

        # D41: a production binary has no lab activation path.
        d = dirs["order"]
        r = sgo(["build", "-o", "prod", "."], d, env)
        if r.returncode != 0:
            fail(f"build: {r.stderr}")
        man = os.path.join(tmp, "manifest")
        open(man, "w").write("seed 1\npolicy random\nk 1\nquota 10\n")
        trace = os.path.join(tmp, "prod-trace")
        with open(man) as mf, open(trace, "w") as tf:
            hi_m, hi_t = os.dup2(mf.fileno(), 20), os.dup2(tf.fileno(), 21)

            def lab_fds():  # in the child: the manifest on fd 3, the trace on fd 4
                os.dup2(hi_m, 3)
                os.dup2(hi_t, 4)
            p = subprocess.run([os.path.join(d, "prod")], capture_output=True, text=True, pass_fds=(hi_m, hi_t),
                               preexec_fn=lab_fds, timeout=60)
            os.close(hi_m)
            os.close(hi_t)
        if p.returncode != 0 or p.stdout not in ("ab\n", "ba\n") or p.stderr or os.path.getsize(trace) != 0:
            fail(f"production binary with a manifest: exit {p.returncode}, stdout {p.stdout!r}, stderr {p.stderr!r}")
        print("test-lab: ok: run, determinism, sidecar, replay, divergence, refusals, LabDeadlock, LabQuotaExhausted, "
              "D13 sweep and replay, production binary ignores a manifest")


if __name__ == "__main__":
    main()
