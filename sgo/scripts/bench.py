#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""bench.py — performance evidence (D67, W10).

    bench.py [--runs N] [--sgo PATH]

Builds each program in sgo/bench/sgo (sgo build) and its pair in sgo/bench/go (the isolated
upstream reference toolchain, go build), recording compile time, then runs every binary N times
(default 20) in fresh processes, recording wall time, user+system CPU time, and peak RSS from
wait4. Lab off versus lab on compares `sgo run` with `sgo lab run` of the same program (both
include the driver's build with a warm cache). Writes raw samples and host/toolchain identity to
sgo/evidence/w10/bench-<os>-<arch>.json and P50/P95 with spread to bench-<os>-<arch>.md.

Results are evidence only: no claim without samples, and no check is removed to improve a number.
Not measured: allocation counts (no allocation profile without changing the programs) and virtual
size (not reported by wait4).
"""
import json
import os
import platform
import shutil
import statistics
import subprocess
import sys
import tempfile
import time

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
GOOS = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
GOARCH = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[platform.machine()]
BENCH = os.path.join(ROOT, "sgo", "bench")
LAB = ["spawn", "rendezvous"]


def lock():
    return json.load(open(os.path.join(ROOT, "sgo", "UPSTREAM.lock.json")))["upstream"]


def run_once(argv, cwd, env):
    t0 = time.perf_counter()
    p = subprocess.Popen(argv, cwd=cwd, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
    _, status, ru = os.wait4(p.pid, 0)
    wall = time.perf_counter() - t0
    err = p.stderr.read().decode()
    p.stderr.close()
    if os.waitstatus_to_exitcode(status) != 0:
        sys.exit(f"bench: {' '.join(argv)} failed: {err[:300]}")
    rss = ru.ru_maxrss if GOOS == "darwin" else ru.ru_maxrss * 1024  # bytes on darwin, KiB on linux
    return {"wall_s": wall, "cpu_s": ru.ru_utime + ru.ru_stime, "maxrss_bytes": rss}


def summary(samples):
    out = {}
    for k in ("wall_s", "cpu_s", "maxrss_bytes"):
        xs = sorted(s[k] for s in samples)
        p95 = xs[min(len(xs) - 1, int(round(0.95 * (len(xs) - 1))))]
        out[k] = {"p50": statistics.median(xs), "p95": p95, "min": xs[0], "max": xs[-1]}
    return out


def main():
    args = sys.argv[1:]
    runs = 20
    sgo = os.path.join(ROOT, "pkg", "tool", f"{GOOS}_{GOARCH}", "sgo")
    while args:
        if args[0] == "--runs":
            runs, args = int(args[1]), args[2:]
        elif args[0] == "--sgo":
            sgo, args = args[1], args[2:]
        else:
            sys.exit("usage: bench.py [--runs N] [--sgo PATH]")
    up = lock()
    gobin = os.path.join(os.environ.get("SGO_CACHE", os.path.expanduser("~/.cache/sgolang")), "reference", up["tag"] + "-b01", "bin", "go")
    results = {"host": {"os": GOOS, "arch": GOARCH, "machine": platform.machine(), "cpus": os.cpu_count(),
                        "platform": platform.platform()},
               "toolchain": {"sgo": subprocess.check_output([sgo, "version"], text=True).strip(),
                             "reference_go": subprocess.check_output([gobin, "version"], text=True).strip(),
                             "upstream": up["commit"]},
               "runs": runs, "benchmarks": {}}
    with tempfile.TemporaryDirectory() as tmp:
        env = {"HOME": os.environ.get("HOME", tmp), "PATH": "/usr/bin:/bin"}
        genv = {**env, "GOROOT": os.path.dirname(os.path.dirname(gobin)), "GOCACHE": os.path.join(tmp, "gocache"),
                "GOFLAGS": "", "GOTOOLCHAIN": "local", "GO111MODULE": "off", "CGO_ENABLED": "0"}
        for name in sorted(os.listdir(os.path.join(BENCH, "sgo"))):
            src = os.path.join(tmp, "sgo-" + name)
            shutil.copytree(os.path.join(BENCH, "sgo", name), src)
            exe = os.path.join(tmp, name + ".sgo.bin")
            subprocess.run([sgo, "build", "-o", exe], cwd=src, env=env, check=True, capture_output=True)  # warm the TCB cache
            t0 = time.perf_counter()
            subprocess.run([sgo, "build", "-o", exe], cwd=src, env=env, check=True, capture_output=True)
            entry = {"sgo": {"compile_s": time.perf_counter() - t0, "samples": [run_once([exe], src, env) for _ in range(runs)]}}
            gsrc = os.path.join(BENCH, "go", name)
            if os.path.isdir(gsrc):
                gexe = os.path.join(tmp, name + ".go.bin")
                subprocess.run([gobin, "build", "-o", gexe, "main.go"], cwd=gsrc, env=genv, check=True, capture_output=True)
                t0 = time.perf_counter()
                subprocess.run([gobin, "build", "-a", "-o", gexe, "main.go"], cwd=gsrc, env=genv, check=True, capture_output=True)
                entry["go"] = {"compile_s_full_rebuild": time.perf_counter() - t0,
                               "samples": [run_once([gexe], gsrc, genv) for _ in range(runs)]}
            if name in LAB:
                entry["sgo_run_lab_off"] = {"samples": [run_once([sgo, "run", "."], src, env) for _ in range(runs)]}
                entry["sgo_lab_run"] = {"samples": [run_once([sgo, "lab", "run", "-seed", str(i), "."], src, env) for i in range(runs)]}
            for v in entry.values():
                v["summary"] = summary(v["samples"])
            results["benchmarks"][name] = entry
            print(f"bench: {name} done", file=sys.stderr)
    plat = f"{GOOS}-{GOARCH}"
    outdir = os.path.join(ROOT, "sgo", "evidence", "w10")
    os.makedirs(outdir, exist_ok=True)
    with open(os.path.join(outdir, f"bench-{plat}.json"), "w") as f:
        json.dump(results, f, indent=1)
        f.write("\n")
    lines = [f"# Benchmarks {plat} (D67)", "",
             f"{runs} fresh processes per row; P50 / P95 (min-max). Host: {results['host']['platform']}, {results['host']['cpus']} CPUs.",
             f"S-Go: `{results['toolchain']['sgo']}`. Go: `{results['toolchain']['reference_go']}` (isolated reference build).",
             "Lab rows run the driver (`sgo run` vs `sgo lab run`), so they include its build with a warm cache.",
             "Not measured: allocation counts, virtual size. Driver rows' CPU and RSS are the driver process's",
             "(including the children it waited for). The max of a row is usually its first, cold-cache run.",
             "Compile: S-Go is `sgo build` with the TCB cached (the user package and link only); Go is",
             "`go build -a` (the whole standard-library closure rebuilt): not comparable, recorded for completeness.", "",
             "| benchmark | variant | wall ms | CPU ms | peak RSS MiB | compile s (see above) |", "|---|---|---|---|---|---|"]
    for name, entry in results["benchmarks"].items():
        for var, v in entry.items():
            s = v["summary"]
            w, c, r = s["wall_s"], s["cpu_s"], s["maxrss_bytes"]
            comp = v.get("compile_s", v.get("compile_s_full_rebuild"))
            lines.append(f"| {name} | {var} | {w['p50']*1e3:.1f} / {w['p95']*1e3:.1f} ({w['min']*1e3:.1f}-{w['max']*1e3:.1f}) "
                         f"| {c['p50']*1e3:.1f} / {c['p95']*1e3:.1f} | {r['p50']/2**20:.1f} / {r['p95']/2**20:.1f} "
                         f"| {'' if comp is None else f'{comp:.2f}'} |")
    with open(os.path.join(outdir, f"bench-{plat}.md"), "w") as f:
        f.write("\n".join(lines) + "\n")
    print(f"bench: wrote sgo/evidence/w10/bench-{plat}.json and .md")


if __name__ == "__main__":
    main()
