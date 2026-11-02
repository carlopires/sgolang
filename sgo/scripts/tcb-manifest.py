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
import re
import subprocess
import sys
import tempfile

ROOT = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
SGO = os.path.join(ROOT, "sgo")
MANIFEST = os.path.join(SGO, "TARGET_TCB.json")
TABLE = os.path.join(ROOT, "src", "cmd", "compile", "internal", "sgoadmit", "tcbtable.go")
LOCK = json.load(open(os.path.join(SGO, "UPSTREAM.lock.json")))
CACHE = os.environ.get("SGO_CACHE", os.path.expanduser("~/.cache/sgolang"))
TARGETS = ["darwin/arm64", "linux/amd64", "linux/arm64"]
# Trusted packages that are not dependencies of runtime but are part of the target TCB.
EXTRA_PACKAGES = ["sgo"]
EXTS = (".go", ".s", ".h")

# Trusted primitives relied on by subset 0 (SGO-TCB-001). "upstream" names the current implementation
# symbols in go1.27.3; "sgo" is the S-Go entry point added by the named work package.
PRIMITIVES = [
    {"id": "TCB-P01", "name": "task creation and registration", "clauses": ["SGO-SPAWN-003"], "upstream": ["newproc", "newproc1"], "sgo": "W02: sgoSpawn/sgoAdmit, sgoTaskStart, sgoTaskMain, sgoTaskExit (sgo_scope.go); W05: Start (sgo_ready.go), SpawnWithout (sgo_auth.go)", "callers": ["sgo package (Nursery.Spawn, Start, SpawnWithout)"]},
    {"id": "TCB-P02", "name": "park and unpark", "clauses": ["SGO-WAIT-001", "SGO-WAIT-002", "SGO-NUR-003"], "upstream": ["gopark", "goready"], "sgo": "W02: goparkunlock/goready in sgoNurseryJoin, sgoTaskExit", "callers": ["TCB-P01", "TCB-P03", "TCB-P05"]},
    {"id": "TCB-P03", "name": "scope state machine and join", "clauses": ["SGO-NUR-002", "SGO-NUR-003", "SGO-NUR-004"], "upstream": [], "sgo": "W02: sgoNurseryOpen, sgoNurseryJoin (sgo_scope.go)", "callers": ["compiler-generated nursery barriers", "sgo package"]},
    {"id": "TCB-P04", "name": "cancellation propagation and query", "clauses": ["SGO-CANCEL-001", "SGO-CANCEL-002", "SGO-CANCEL-004"], "upstream": [], "sgo": "W04", "callers": ["sgo package (Cancel, Checkpoint, Sleep)", "TCB-P03"]},
    {"id": "TCB-P05", "name": "monotonic clock, timers, sleep", "clauses": ["SGO-WAIT-001", "SGO-TIMER-001", "SGO-DRAIN-002"], "upstream": ["nanotime", "timer.modify", "timeSleep"], "sgo": "W04", "callers": ["sgo package (Sleep)", "TCB-P03 (drain budget)"]},
    {"id": "TCB-P06", "name": "stdout write", "clauses": ["SGO-PROC-003"], "upstream": ["write1"], "sgo": "W03", "callers": ["sgo package (Writer.WriteString)"]},
    {"id": "TCB-P07", "name": "process exit and diagnostic record", "clauses": ["SGO-PROC-002", "SGO-ERR-002", "SGO-ERR-003"], "upstream": ["exit", "fatalpanic", "throw"], "sgo": "W03", "callers": ["runtime start-up (main return)", "TCB-P08", "TCB-P03"]},
    {"id": "TCB-P08", "name": "failure unwinding and defers", "clauses": ["SGO-DEF-001", "SGO-DEF-002", "SGO-DEF-004", "SGO-DEF-005"], "upstream": ["gopanic", "deferreturn", "panicdivide", "panicshift"], "sgo": "W03", "callers": ["compiler-generated code"]},
    {"id": "TCB-P09", "name": "allocation and garbage collection", "clauses": ["SGO-MEM-001", "SGO-TCB-001"], "upstream": ["mallocgc", "newobject", "typedmemmove", "gcStart"], "sgo": "upstream", "callers": ["compiler-generated code", "runtime"]},
    {"id": "TCB-P10", "name": "stack growth and overflow detection", "clauses": ["SGO-STACK-001", "SGO-ERR-003"], "upstream": ["morestack", "newstack"], "sgo": "upstream", "callers": ["function prologues (stacksplit)"]},
    {"id": "TCB-P11", "name": "runtime service goroutines", "clauses": ["SGO-TCB-001"], "upstream": ["see services"], "sgo": "W02: sgoServiceStart/sgoServiceExit classify every newproc creation; enforcement fail-stops unenumerated creation (sgo_service.go)", "callers": ["runtime only"]},
    {"id": "TCB-P12", "name": "managed channels with send permits", "clauses": ["SGO-PERMIT-002"], "upstream": ["gopark", "goready"], "sgo": "W06: sgoChanReserve/Commit/Abort/Receive/Close, obligation resolution (sgo_chan.go)", "callers": ["sgo package (Channel, SendPermit)", "TCB-P01 (task end)", "TCB-P03 (body exit)"]},
    {"id": "TCB-P13", "name": "monotonic and wall-clock reads", "clauses": ["SGO-AUTH-003"], "upstream": ["nanotime", "time_now"], "sgo": "W05: sgo_sinceMS, sgo_wallMS (sgo_effects.go)", "callers": ["sgo package (Clock.Since, WallClock.Now)"]},
    {"id": "TCB-P14", "name": "OS cryptographic randomness", "clauses": ["SGO-AUTH-003"], "upstream": ["readRandom"], "sgo": "W05: sgo_entropy (sgo_effects.go)", "callers": ["sgo package (Entropy.Bytes)"]},
    {"id": "TCB-P15", "name": "simulated key-value store", "clauses": ["SGO-AUTH-003"], "upstream": [], "sgo": "W05: copy-on-write map, sgo_storeGet/sgo_storePut (sgo_effects.go)", "callers": ["sgo package (SimStore)"]},
    {"id": "TCB-P16", "name": "task ceilings and root grants", "clauses": ["SGO-AUTH-001", "SGO-AUTH-002"], "upstream": [], "sgo": "W05: ceiling fixed by sgoAdmit, sgoRequire, sgoGrantsInit from the linker's -sgogrants (sgo_auth.go)", "callers": ["sgo package (every effect operation, SpawnWithout, root accessors)", "TCB-P01"]},
    {"id": "TCB-P17", "name": "native channels and select", "clauses": ["SGO-CHAN-002", "SGO-CHAN-003", "SGO-CHAN-004"], "upstream": ["gopark", "goready", "mallocgc", "typedmemmove", "cheaprandn"], "sgo": "W07: sgomakechan, sgochansend1, sgochanrecv1/2, sgoclosechan, sgoselectgo/nbsend/nbrecv, sgoblock on the TCB-P12 channel; cancellation unwind sgoCancelUnwind (sgo_native.go, sgo_native_ops.go); Deadlock record (sgo_main.go)", "callers": ["compiled S-Go code (lowered by typecheck.LookupRuntime)", "TCB-P05 (unwinding joins)"]},
    {"id": "TCB-P18", "name": "Event and CapacityLimiter", "clauses": ["SGO-SYNC-002"], "upstream": ["gopark", "goready"], "sgo": "W07: sgoEventSet/Wait/IsSet, sgoLimiterAcquire, sgoTokenRelease, token resolution (sgo_sync.go)", "callers": ["sgo package (Event, CapacityLimiter, Token)", "TCB-P12 (obligation resolution)"]},
    {"id": "TCB-P19", "name": "generator ownership claim", "clauses": ["SGO-AUTH-003"], "upstream": ["atomic_casPointer"], "sgo": "W07: sgo_claimOwner (sgo_effects.go)", "callers": ["sgo package (Random methods)"]},
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
        return sorted(set(stored) | set(EXTRA_PACKAGES))
    pkgs = set(EXTRA_PACKAGES)
    for t in TARGETS:
        goos, goarch = t.split("/")
        env = {"HOME": os.environ["HOME"], "PATH": f"{ref}/bin:/usr/bin:/bin", "GOTOOLCHAIN": "local",
               "GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": "0", "GOFLAGS": "", "GOPROXY": "off"}
        out = subprocess.check_output([gobin, "list", "-deps", "-f", "{{.ImportPath}}", "runtime"], text=True, env=env, cwd=ref)
        pkgs.update(out.split())
    return sorted(pkgs)


def files(pkgs, with_upstream):
    result = []
    for p in pkgs:
        d = os.path.join(ROOT, "src", p)
        for n in sorted(os.listdir(d)):
            if not n.endswith(EXTS) or n.endswith("_test.go") or not os.path.isfile(os.path.join(d, n)):
                continue
            rel = f"src/{p}/{n}"
            data = open(os.path.join(ROOT, rel), "rb").read()
            result.append({"path": rel, "sha256": hashlib.sha256(data).hexdigest()})
    if not with_upstream:
        return result
    # Mark files identical to the upstream commit (needs history; only when writing the manifest).
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


def build(stored_pkgs, with_upstream):
    pkgs = closure(stored_pkgs)
    rows, errs = services(go_statements(pkgs))
    if errs:
        for e in errs:
            print(f"tcb-manifest: {e}", file=sys.stderr)
        die("runtime services do not match the enumerated list")
    fl = files(pkgs, with_upstream)
    hd = headers(fl)
    # The allowlist is versioned by the spec it belongs to and identified by its content: a digest
    # over every listed path and file digest, so any change to the TCB is a new allowlist.
    allow = hashlib.sha256("".join(f'{f["path"]} {f["sha256"]}\n' for f in sorted(fl + hd, key=lambda f: f["path"])).encode()).hexdigest()
    return {
        "schema_version": 1,
        "clause": "SGO-TCB-002",
        "spec_version": open(os.path.join(SGO, "spec", "VERSION")).read().strip(),
        "allowlist_digest": allow,
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
                    "files_changed_from_upstream": sum(1 for f in fl if not f.get("upstream_identical", True)),
                    "services": {s: sum(1 for r in rows if r["status"] == s) for s in ("active", "dormant", "not-built")}},
        "files": fl,
        # Assembly headers the TCB's .s files include from outside the TCB packages (W10): pinned
        # like the files, verified by the driver before it assembles (driver tcbtable.go).
        "headers": hd,
    }


INCLUDE = re.compile(r'^#include\s+"([^"]+)"', re.M)


def headers(fl):
    """The headers the listed .s and .h files include, resolved as the assembler does (next to
    the including file, then in runtime), that are not listed files themselves."""
    listed = {f["path"] for f in fl}
    seen, todo = set(), [f["path"] for f in fl if f["path"].endswith((".s", ".h"))]
    while todo:
        p = todo.pop()
        for h in INCLUDE.findall(open(os.path.join(ROOT, p)).read()):
            for c in (os.path.join(os.path.dirname(p), h), os.path.join("src", "runtime", h)):
                c = os.path.normpath(c)
                if os.path.exists(os.path.join(ROOT, c)):
                    if c not in seen:
                        seen.add(c)
                        todo.append(c)
                    break
    return [{"path": c, "sha256": hashlib.sha256(open(os.path.join(ROOT, c), "rb").read()).hexdigest()}
            for c in sorted(seen - listed)]


DRIVER_TABLE = os.path.join(ROOT, "src", "cmd", "sgo", "tcbtable.go")


def driver_table_source(manifest):
    """Go source of the driver's table of assembler inputs: every trusted .s file and header."""
    lines = ["// Code generated by sgo/scripts/tcb-manifest.py from sgo/TARGET_TCB.json; DO NOT EDIT.", "",
             "package main", "",
             "// tcbAsmList lists every trusted assembler input (.s files and the headers they include) as",
             "// \"path under src digest\", one per line; the driver verifies each before assembling (SGO-TCB-002).",
             "const tcbAsmList = `"]
    for f in sorted(manifest["files"] + manifest.get("headers", []), key=lambda f: f["path"]):
        if f["path"].endswith((".s", ".h")):
            lines.append(f'{f["path"][len("src/"):]} {f["sha256"]}')
    lines.append("`")
    return "\n".join(lines) + "\n"


def table_source(manifest):
    """Go source of the compiler's trusted-digest table (sgoadmit.Trusted)."""
    lines = ["// Code generated by sgo/scripts/tcb-manifest.py from sgo/TARGET_TCB.json; DO NOT EDIT.", "",
             "package sgoadmit", "",
             "// tcbDigests are the SHA-256 digests of every trusted target file (SGO-TCB-002).",
             "var tcbDigests = map[string]bool{"]
    for d in sorted({f["sha256"] for f in manifest["files"]}):
        lines.append(f'\t"{d}": true,')
    lines.append("}")
    lines += ["",
              "// tcbFileList lists every trusted Go file as \"import path/file name digest\", one per line; a",
              "// package whose import path is in tcbPackages must be compiled from exactly such files",
              "// (SGO_E_TCB_DIGEST otherwise).",
              "const tcbFileList = `"]
    for f in sorted(manifest["files"], key=lambda f: f["path"]):
        if f["path"].endswith(".go"):
            lines.append(f'{f["path"][len("src/"):]} {f["sha256"]}')
    lines.append("`")
    pkgs = sorted({os.path.dirname(f["path"])[len("src/"):] for f in manifest["files"]} | {"unsafe"})
    lines += ["", "// tcbPackageList lists the import paths of the TCB packages, one per line.",
              "const tcbPackageList = `"] + pkgs + ["`"]
    return "\n".join(lines) + "\n"


IDENTITY = os.path.join(ROOT, "src", "cmd", "compile", "internal", "sgoadmit", "identity.go")
# The driver embeds the same identity: it refuses a GOROOT whose spec or admission registry is not
# the one the toolchain was built for (S18).
DRIVER_IDENTITY = os.path.join(ROOT, "src", "cmd", "sgo", "identity.go")


def identity_source(package="sgoadmit"):
    """Go source of the S-Go identity (SGO-IMP-002) built into the compiler (package sgoadmit) and
    the driver (package main): spec version and admission digest."""
    spec = open(os.path.join(SGO, "spec", "VERSION")).read().strip()
    adm = hashlib.sha256(open(os.path.join(SGO, "SGO_SPEC_ADMISSION.json"), "rb").read()).hexdigest()[:16]
    return "\n".join(l for l in [
        "// Code generated by sgo/scripts/tcb-manifest.py from sgo/spec/VERSION and sgo/SGO_SPEC_ADMISSION.json; DO NOT EDIT.", "",
        f"package {package}", "",
        "// S-Go identity recorded in every compiled package and verified by importers and the linker",
        "// (SGO-IMP-002).",
        "//" if package == "main" else None,
        "// The driver refuses to build when the GOROOT's sgo/spec/VERSION or sgo/SGO_SPEC_ADMISSION.json" if package == "main" else None,
        "// differs from these values: the toolchain was built for another specification (S18)." if package == "main" else None,
        "const (",
        f'\tSpecVersion     = "{spec}"',
        f'\tAdmissionDigest = "{adm}"',
        '\tABI             = "sgo-abi-1" // S-Go ABI identity: lowering of nurseries, failures, checked arithmetic',
        ")", ""] if l is not None)


def main():
    stored = json.load(open(MANIFEST)) if os.path.exists(MANIFEST) else None
    write = "--write" in sys.argv
    new = build(stored["packages"] if stored else None, write)
    if write:
        with open(MANIFEST, "w") as f:
            json.dump(new, f, indent=1)
            f.write("\n")
        with open(TABLE, "w") as f:
            f.write(table_source(new))
        with open(DRIVER_TABLE, "w") as f:
            f.write(driver_table_source(new))
        with open(IDENTITY, "w") as f:
            f.write(identity_source())
        with open(DRIVER_IDENTITY, "w") as f:
            f.write(identity_source("main"))
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
    if not os.path.exists(TABLE) or open(TABLE).read() != table_source(stored):
        problems.append("compiler trust table src/cmd/compile/internal/sgoadmit/tcbtable.go is stale")
    if not os.path.exists(DRIVER_TABLE) or open(DRIVER_TABLE).read() != driver_table_source(stored):
        problems.append("driver trust table src/cmd/sgo/tcbtable.go is stale")
    old_h = {f["path"]: f["sha256"] for f in stored.get("headers", [])}
    cur_h = {f["path"]: f["sha256"] for f in new["headers"]}
    if old_h != cur_h:
        problems.append("assembly headers differ from the manifest")
    if not os.path.exists(IDENTITY) or open(IDENTITY).read() != identity_source():
        problems.append("compiler identity src/cmd/compile/internal/sgoadmit/identity.go is stale")
    if not os.path.exists(DRIVER_IDENTITY) or open(DRIVER_IDENTITY).read() != identity_source("main"):
        problems.append("driver identity src/cmd/sgo/identity.go is stale")
    if problems:
        for p in problems[:50]:
            print(f"tcb-manifest: {p}", file=sys.stderr)
        die(f"{len(problems)} runtime TCB difference(s) (SGO_E_TCB_DIGEST); review and run --write")
    print(f"tcb-manifest: ok: {stored['summary']}")


if __name__ == "__main__":
    main()
