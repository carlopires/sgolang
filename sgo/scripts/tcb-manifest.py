#!/usr/bin/env python3
"""tcb-manifest.py — SGO-TCB-002: the runtime is trusted target code only by exact source identity.

    tcb-manifest.py           check: every runtime-closure file matches sgo/TARGET_TCB.json, and the
                              runtime's go statements are exactly the enumerated services (CI)
    tcb-manifest.py --write   regenerate the manifest (a reviewed change: the diff is the review record)

The package closure is computed with the untouched reference toolchain (W00) for darwin/arm64,
linux/amd64, and linux/arm64 when it is available; otherwise the stored closure is reused.
"""
import hashlib
import json
import os
import subprocess
import sys
import tempfile

ROOT = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
SGO = os.path.join(ROOT, "sgo")
MANIFEST = os.path.join(SGO, "TARGET_TCB.json")
LOCK = json.load(open(os.path.join(SGO, "UPSTREAM.lock.json")))
CACHE = os.environ.get("SGO_CACHE", os.path.expanduser("~/.cache/sgolang"))
TARGETS = ["darwin/arm64", "linux/amd64", "linux/arm64"]
EXTS = (".go", ".s", ".h")

# Trusted primitives relied on by subset 0 (SGO-TCB-001). "upstream" names the current implementation
# symbols in go1.27.3; "sgo" is the S-Go entry point added by the named work package.
PRIMITIVES = [
    {"id": "TCB-P01", "name": "task creation and registration", "clauses": ["SGO-SPAWN-003"], "upstream": ["newproc", "newproc1"], "sgo": "W02", "callers": ["sgo package (Nursery.Spawn)"]},
    {"id": "TCB-P02", "name": "park and unpark", "clauses": ["SGO-WAIT-001", "SGO-WAIT-002", "SGO-NUR-003"], "upstream": ["gopark", "goready"], "sgo": "W02", "callers": ["TCB-P01", "TCB-P03", "TCB-P05"]},
    {"id": "TCB-P03", "name": "scope state machine and join", "clauses": ["SGO-NUR-002", "SGO-NUR-003", "SGO-NUR-004"], "upstream": [], "sgo": "W02", "callers": ["compiler-generated nursery barriers", "sgo package"]},
    {"id": "TCB-P04", "name": "cancellation propagation and query", "clauses": ["SGO-CANCEL-001", "SGO-CANCEL-002", "SGO-CANCEL-004"], "upstream": [], "sgo": "W04", "callers": ["sgo package (Cancel, Checkpoint, Sleep)", "TCB-P03"]},
    {"id": "TCB-P05", "name": "monotonic clock, timers, sleep", "clauses": ["SGO-WAIT-001", "SGO-TIMER-001", "SGO-DRAIN-002"], "upstream": ["nanotime", "timer.modify", "timeSleep"], "sgo": "W04", "callers": ["sgo package (Sleep)", "TCB-P03 (drain budget)"]},
    {"id": "TCB-P06", "name": "stdout write", "clauses": ["SGO-PROC-003"], "upstream": ["write1"], "sgo": "W03", "callers": ["sgo package (Writer.WriteString)"]},
    {"id": "TCB-P07", "name": "process exit and diagnostic record", "clauses": ["SGO-PROC-002", "SGO-ERR-002", "SGO-ERR-003"], "upstream": ["exit", "fatalpanic", "throw"], "sgo": "W03", "callers": ["runtime start-up (main return)", "TCB-P08", "TCB-P03"]},
    {"id": "TCB-P08", "name": "failure unwinding and defers", "clauses": ["SGO-DEF-001", "SGO-DEF-002", "SGO-DEF-004", "SGO-DEF-005"], "upstream": ["gopanic", "deferreturn", "panicdivide", "panicshift"], "sgo": "W03", "callers": ["compiler-generated code"]},
    {"id": "TCB-P09", "name": "allocation and garbage collection", "clauses": ["SGO-MEM-001", "SGO-TCB-001"], "upstream": ["mallocgc", "newobject", "typedmemmove", "gcStart"], "sgo": "upstream", "callers": ["compiler-generated code", "runtime"]},
    {"id": "TCB-P10", "name": "stack growth and overflow detection", "clauses": ["SGO-STACK-001", "SGO-ERR-003"], "upstream": ["morestack", "newstack"], "sgo": "upstream", "callers": ["function prologues (stacksplit)"]},
    {"id": "TCB-P11", "name": "runtime service goroutines", "clauses": ["SGO-TCB-001"], "upstream": ["see services"], "sgo": "W02 (sealed service primitive replacing go statements)", "callers": ["runtime only"]},
]

# Every go statement in the runtime closure (OWN-6), keyed by callee and file.
SERVICES = {
    ("bgsweep", "src/runtime/mgc.go"): ("active", "GC: background sweeper, started by gcenable at start-up"),
    ("bgscavenge", "src/runtime/mgc.go"): ("active", "GC: background scavenger, started by gcenable at start-up"),
    ("gcBgMarkWorker", "src/runtime/mgc.go"): ("active", "GC: background mark workers, started per GC cycle"),
    ("forcegchelper", "src/runtime/proc.go"): ("active", "GC: periodic forced GC helper, started at start-up"),
    ("updateMaxProcsGoroutine", "src/runtime/proc.go"): ("active", "scheduler: GOMAXPROCS updates from CPU limits"),
    ("runFinalizers", "src/runtime/mfinal.go"): ("dormant", "started only by SetFinalizer; unreachable from subset 0 (verify in W02)"),
    ("runCleanups", "src/runtime/mcleanup.go"): ("dormant", "started only by AddCleanup; unreachable from subset 0 (verify in W02)"),
    ("<func literal>", "src/runtime/signal_unix.go"): ("dormant", "ensureSigM, started only by os/signal (denied)"),
    ("<func literal>", "src/runtime/trace.go"): ("dormant", "execution tracer advancer, started only by runtime/trace (denied)"),
    ("<func literal>", "src/runtime/tracecpu.go"): ("dormant", "execution tracer CPU logger, started only by runtime/trace (denied)"),
    ("handleAsyncEvent", "src/runtime/lock_js.go"): ("not-built", "js/wasm only; not an admitted target"),
}


def die(msg):
    print(f"tcb-manifest: error: {msg}", file=sys.stderr)
    sys.exit(1)


def closure(stored):
    ref = os.path.join(CACHE, "reference", LOCK["upstream"]["tag"] + "-b01")
    gobin = os.path.join(ref, "bin", "go")
    if not os.path.exists(gobin):
        if stored is None:
            die(f"no reference toolchain at {ref} and no stored package closure; run sgo/scripts/build-reference.sh")
        return stored
    pkgs = set()
    for t in TARGETS:
        goos, goarch = t.split("/")
        env = {"HOME": os.environ["HOME"], "PATH": f"{ref}/bin:/usr/bin:/bin", "GOTOOLCHAIN": "local",
               "GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": "0", "GOFLAGS": "", "GOPROXY": "off"}
        out = subprocess.check_output([gobin, "list", "-deps", "-f", "{{.ImportPath}}", "runtime"], text=True, env=env, cwd=ref)
        pkgs.update(out.split())
    return sorted(pkgs)


def files(pkgs):
    result = []
    for p in pkgs:
        d = os.path.join(ROOT, "src", p)
        for n in sorted(os.listdir(d)):
            if not n.endswith(EXTS) or n.endswith("_test.go") or not os.path.isfile(os.path.join(d, n)):
                continue
            rel = f"src/{p}/{n}"
            data = open(os.path.join(ROOT, rel), "rb").read()
            result.append({"path": rel, "sha256": hashlib.sha256(data).hexdigest()})
    # Mark files identical to the upstream commit (blob hash comparison, one git call).
    commit = LOCK["upstream"]["commit"]
    proc = subprocess.run(["git", "-C", ROOT, "ls-tree", "-r", commit, "--", "src"], capture_output=True, text=True, check=True)
    upstream = {}
    for line in proc.stdout.splitlines():
        meta, path = line.split("\t", 1)
        upstream[path] = meta.split()[2]
    for f in result:
        local = subprocess.run(["git", "-C", ROOT, "hash-object", f["path"]], capture_output=True, text=True, check=True).stdout.strip()
        f["upstream_identical"] = upstream.get(f["path"]) == local
    return result


def go_statements(pkgs):
    boot = subprocess.check_output([os.path.join(SGO, "scripts", "fetch-bootstrap.sh")], text=True).strip()
    env = {"HOME": os.environ["HOME"], "PATH": "/usr/bin:/bin", "GOTOOLCHAIN": "local", "GOROOT": boot, "GOPROXY": "off",
           "GOFLAGS": "", "GOTELEMETRY": "off", "GOCACHE": os.path.join(CACHE, "gocache", "inventory")}
    with tempfile.TemporaryDirectory() as tmp:
        open(os.path.join(tmp, "go.mod"), "w").write("module extract\n\ngo 1.26\n")
        open(os.path.join(tmp, "extract.go"), "w").write(open(os.path.join(SGO, "scripts", "inventory", "extract.go")).read())
        subprocess.check_call([os.path.join(boot, "bin", "go"), "build", "-o", "extract", "."], cwd=tmp, env=env)
        dirs = [f"src/{p}" for p in pkgs if p != "unsafe"]
        out = subprocess.check_output([os.path.join(tmp, "extract"), "-gostmts", ROOT] + dirs, text=True)
    return [json.loads(l) for l in out.splitlines() if l.strip()]


def services(stmts):
    errs, rows = [], []
    seen = set()
    for s in stmts:
        key = (s["name"], s["file"])
        if key not in SERVICES:
            errs.append(f"un-enumerated go statement in the runtime: {s['name']} at {s['file']}:{s['line']} (OWN-6)")
            continue
        seen.add(key)
        status, purpose = SERVICES[key]
        rows.append({"callee": s["name"], "file": s["file"], "line": s["line"], "status": status, "purpose": purpose})
    for key in SERVICES:
        if key not in seen:
            errs.append(f"enumerated service {key[0]} in {key[1]} no longer exists; update the service list")
    return rows, errs


def build(stored_pkgs):
    pkgs = closure(stored_pkgs)
    rows, errs = services(go_statements(pkgs))
    if errs:
        for e in errs:
            print(f"tcb-manifest: {e}", file=sys.stderr)
        die("runtime services do not match the enumerated list")
    fl = files(pkgs)
    return {
        "schema_version": 1,
        "clause": "SGO-TCB-002",
        "approval": {"decision": "Runtime admitted as trusted target code by exact source identity",
                     "approved_by": "user", "approved_at": "2026-10-30"},
        "upstream_commit": LOCK["upstream"]["commit"],
        "targets": TARGETS,
        "rules": [
            "Every file below must match its SHA-256 exactly; any change requires regenerating this manifest in a reviewed commit.",
            "upstream_identical=false marks files S-Go changed; each such file needs a review note in the commit that changed it.",
            "Being listed grants nothing to user code: user-level unsafe, assembly, linkname, and cgo stay denied (SGO-NATIVE-001).",
            "Runtime go statements are exactly the enumerated services (OWN-6); a new one fails the check.",
        ],
        "packages": pkgs,
        "primitives": PRIMITIVES,
        "services": rows,
        "summary": {"packages": len(pkgs), "files": len(fl),
                    "files_changed_from_upstream": sum(1 for f in fl if not f["upstream_identical"]),
                    "services": {s: sum(1 for r in rows if r["status"] == s) for s in ("active", "dormant", "not-built")}},
        "files": fl,
    }


def main():
    stored = json.load(open(MANIFEST)) if os.path.exists(MANIFEST) else None
    new = build(stored["packages"] if stored else None)
    if "--write" in sys.argv:
        with open(MANIFEST, "w") as f:
            json.dump(new, f, indent=1)
            f.write("\n")
        print(f"tcb-manifest: wrote {os.path.relpath(MANIFEST, ROOT)}: {new['summary']}")
        return
    if stored is None:
        die("sgo/TARGET_TCB.json missing; run with --write")
    old = {f["path"]: f["sha256"] for f in stored["files"]}
    cur = {f["path"]: f["sha256"] for f in new["files"]}
    problems = [f"changed: {p}" for p in sorted(cur) if p in old and old[p] != cur[p]]
    problems += [f"new file: {p}" for p in sorted(set(cur) - set(old))]
    problems += [f"removed file: {p}" for p in sorted(set(old) - set(cur))]
    if stored["services"] != new["services"] or stored["primitives"] != new["primitives"]:
        problems.append("services or primitives differ from the manifest")
    if problems:
        for p in problems[:50]:
            print(f"tcb-manifest: {p}", file=sys.stderr)
        die(f"{len(problems)} runtime TCB difference(s) (SGO_E_TCB_DIGEST); review and run --write")
    print(f"tcb-manifest: ok: {new['summary']}")


if __name__ == "__main__":
    main()
