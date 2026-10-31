#!/usr/bin/env python3
"""conformance.py — S-Go conformance cases (SGO-TEST-001).

    conformance.py validate             check every case and that every fixture a clause names exists
    conformance.py run --sgo PATH [--pending FILE] [case ...]
                                        build/run every case with the given sgo driver and compare;
                                        cases listed in FILE (JSON: {case: work package}) are expected
                                        to fail until their work package lands, and an unexpected
                                        pass is an error (the list may only shrink)
    conformance.py selftest             test this runner against fake drivers

Case layout: sgo/testdata/conformance/<name>/{sgo.mod, *.sgo, ..., expect.json}. expect.json keys:
  kind               "run" | "reject"
  clauses            [clause IDs]
  exit               (run) expected exit status
  stdout             (run) exact stdout, or stdout_any_of: [allowed outputs]
  stderr_kind        (run, optional) "kind" of the runtime diagnostic record (SGO-ERR-002)
  stderr_message     (run, optional) "message" of the runtime diagnostic record
  stderr_exact       (run, optional) exact stderr
  diagnostics        (reject) [{"code", "file" (default main.sgo), "at" (text at the position) | "position"}]
  build_args         (optional) extra arguments to `sgo build`
  build_env, run_env (optional) environment variables added for build / run
  timeout_ms         (optional, default 10000) watchdog; expect_watchdog: true if the case must be killed
  procedure          (optional) "rebuild-after-edit": build once, replace each X with X.next, build again;
                     the expectations apply to the second build
  no_module          (optional) true if the case deliberately has no sgo.mod
Positions are computed from "at": the first occurrence (or "occurrence": n) of the text in the file,
reported as file:line:col with 1-based columns counted in bytes.
"""
import json
import os
import re
import subprocess
import sys
import tempfile

ROOT = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
SGO = os.path.join(ROOT, "sgo")
CASES = os.path.join(SGO, "testdata", "conformance")
REGISTRY = os.path.join(SGO, "SGO_SPEC_ADMISSION.json")
KEYS = {"kind", "clauses", "exit", "stdout", "stdout_any_of", "stderr_kind", "stderr_exact", "diagnostics",
        "build_args", "build_env", "run_env", "timeout_ms", "expect_watchdog", "procedure", "note", "no_module", "occurrence", "stderr_message"}
PROCEDURES = {"rebuild-after-edit"}  # build once, replace each X with X.next, build again; expectations apply to the second build
CODE = re.compile(r"^SGO_E_[A-Z_]+$")


def position(case_dir, d):
    f = d.get("file", "main.sgo")
    if "position" in d:
        return d["position"]
    data = open(os.path.join(case_dir, f), "rb").read()
    needle = d["at"].encode()
    idx, n = -1, d.get("occurrence", 1)
    for _ in range(n):
        idx = data.find(needle, idx + 1)
        if idx < 0:
            raise ValueError(f"'at' text {d['at']!r} (occurrence {n}) not found in {f}")
    line = data.count(b"\n", 0, idx) + 1
    col = idx - (data.rfind(b"\n", 0, idx) + 1) + 1
    return f"{f}:{line}:{col}"


def load_cases():
    cases = {}
    for name in sorted(os.listdir(CASES)):
        d = os.path.join(CASES, name)
        if os.path.isdir(d) and not name.startswith("_"):
            cases[name] = d
    return cases


def validate_case(name, d, clauses):
    errs = []
    p = os.path.join(d, "expect.json")
    if not os.path.exists(p):
        return [f"{name}: missing expect.json"]
    try:
        e = json.load(open(p))
    except ValueError as x:
        return [f"{name}: expect.json: {x}"]
    for k in e:
        if k not in KEYS:
            errs.append(f"{name}: unknown key {k!r}")
    # Files a case names (build arguments, diagnostic files) must be tracked by git: a fixture
    # matching a .gitignore pattern (stock.a did) exists only in the checkout that created it.
    named = [a for a in e.get("build_args", []) if not a.startswith("-")]
    named += [dg["file"] for dg in e.get("diagnostics", []) if "file" in dg]
    for f in named:
        rel = os.path.relpath(os.path.join(d, f), ROOT)
        tracked = subprocess.run(["git", "-C", ROOT, "ls-files", "--error-unmatch", rel], capture_output=True).returncode == 0
        if not tracked:
            errs.append(f"{name}: {f} is not tracked by git (check .gitignore; use git add -f)")
    if e.get("kind") not in ("run", "reject"):
        errs.append(f"{name}: kind must be run or reject")
    if not e.get("clauses"):
        errs.append(f"{name}: must name at least one clause")
    for c in e.get("clauses", []):
        if c not in clauses:
            errs.append(f"{name}: unknown clause {c}")
    if not os.path.exists(os.path.join(d, "sgo.mod")) and not e.get("no_module"):
        errs.append(f"{name}: missing sgo.mod")
    if e.get("kind") == "run":
        if not isinstance(e.get("exit"), int) and not e.get("expect_watchdog"):
            errs.append(f"{name}: run case needs integer exit")
        if ("stdout" in e) == ("stdout_any_of" in e) and not e.get("expect_watchdog"):
            errs.append(f"{name}: run case needs exactly one of stdout / stdout_any_of")
        if "diagnostics" in e:
            errs.append(f"{name}: run case cannot have diagnostics")
    if e.get("kind") == "reject":
        if not e.get("diagnostics"):
            errs.append(f"{name}: reject case needs diagnostics")
        for dg in e.get("diagnostics", []):
            if not CODE.match(dg.get("code", "")):
                errs.append(f"{name}: bad diagnostic code {dg.get('code')!r}")
            if ("at" in dg) == ("position" in dg):
                errs.append(f"{name}: diagnostic needs exactly one of at / position")
                continue
            try:
                position(d, dg)
            except (ValueError, OSError) as x:
                errs.append(f"{name}: {x}")
    if e.get("procedure") and e["procedure"] not in PROCEDURES:
        errs.append(f"{name}: unknown procedure {e['procedure']!r}")
    if not any(f.endswith(".sgo") or f.endswith(".go") or f.endswith(".a") for f in os.listdir(d)) and e.get("kind"):
        errs.append(f"{name}: no source files")
    return errs


def referenced_fixtures():
    refs = {}
    for f in sorted(os.listdir(os.path.join(SGO, "spec"))):
        if f.startswith("SPEC-") and f.endswith(".md"):
            for m in re.finditer(r"fixture:([a-z0-9][a-z0-9_-]*)", open(os.path.join(SGO, "spec", f)).read()):
                refs.setdefault(m.group(1), set()).add(f)
    return refs


def validate():
    reg = json.load(open(REGISTRY))
    clauses = {c for r in reg["rows"] for c in r["clause_ids"]}
    cases = load_cases()
    errs = []
    for name, d in cases.items():
        errs += validate_case(name, d, clauses)
    for fx, files in sorted(referenced_fixtures().items()):
        if fx not in cases:
            errs.append(f"fixture:{fx} named in {', '.join(sorted(files))} has no case directory")
    if errs:
        for e in errs:
            print(f"conformance: {e}", file=sys.stderr)
        print(f"conformance: error: {len(errs)} problem(s) in {len(cases)} case(s)", file=sys.stderr)
        return 1
    kinds = {}
    for d in cases.values():
        k = json.load(open(os.path.join(d, "expect.json")))["kind"]
        kinds[k] = kinds.get(k, 0) + 1
    print(f"conformance: ok: {len(cases)} cases {kinds}; every clause-referenced fixture exists")
    return 0


DIAG = re.compile(r"^(\S+:\d+:\d+): (SGO_E_[A-Z_]+):", re.M)
GUARD = os.path.join(os.environ.get("SGO_CACHE", os.path.expanduser("~/.cache/sgolang")), "guard", "bin")


def env_for(extra):
    env = {"HOME": os.environ.get("HOME", "/tmp"), "PATH": f"{GUARD}:/usr/bin:/bin", "LANG": "C", "LC_ALL": "C"}
    env.update(extra or {})
    return env


def run_case(name, src, sgo):
    """Returns '' on pass, otherwise a description of the mismatch."""
    import shutil
    e = json.load(open(os.path.join(src, "expect.json")))
    with tempfile.TemporaryDirectory() as tmp:
        d = os.path.join(tmp, name)
        shutil.copytree(src, d)
        build = [sgo, "build", "-o", os.path.join(tmp, "prog")] + e.get("build_args", [])
        benv = env_for(e.get("build_env"))
        if e.get("procedure") == "rebuild-after-edit":
            first = subprocess.run(build, cwd=d, env=benv, capture_output=True, text=True, timeout=120)
            if first.returncode != 0:
                return f"first build failed: {first.stderr.strip()[:200]}"
            for root, _, files in os.walk(d):
                for f in files:
                    if f.endswith(".next"):
                        os.replace(os.path.join(root, f), os.path.join(root, f[:-5]))
        b = subprocess.run(build, cwd=d, env=benv, capture_output=True, text=True, timeout=120)
        if e["kind"] == "reject":
            if b.returncode == 0:
                return "build succeeded; expected rejection"
            got = [(m.group(1), m.group(2)) for m in DIAG.finditer(b.stderr)]
            want = [(position(src, dg), dg["code"]) for dg in e["diagnostics"]]
            return "" if got == want else f"diagnostics {got}, want {want}"
        if b.returncode != 0:
            return f"build failed: {b.stderr.strip()[:300]}"
        try:
            r = subprocess.run([os.path.join(tmp, "prog")], cwd=d, env=env_for(e.get("run_env")),
                               capture_output=True, text=True, timeout=e.get("timeout_ms", 10000) / 1000)
        except subprocess.TimeoutExpired:
            return "" if e.get("expect_watchdog") else "watchdog: killed after timeout"
        if e.get("expect_watchdog"):
            return f"expected the watchdog to kill the program; it exited {r.returncode}"
        if r.returncode != e["exit"]:
            return f"exit {r.returncode}, want {e['exit']}; stderr {r.stderr.strip()[:200]}"
        outs = e["stdout_any_of"] if "stdout_any_of" in e else [e["stdout"]]
        if r.stdout not in outs:
            return f"stdout {r.stdout!r}, want one of {outs!r}"
        if "stderr_exact" in e and r.stderr != e["stderr_exact"]:
            return f"stderr {r.stderr!r}, want {e['stderr_exact']!r}"
        if "stderr_kind" in e or "stderr_message" in e:
            try:
                rec = json.loads(r.stderr.strip().splitlines()[-1])
            except (ValueError, IndexError):
                rec = {}
            for key in ("kind", "message"):
                if "stderr_" + key in e and rec.get(key) != e["stderr_" + key]:
                    return f"diagnostic {key} {rec.get(key)!r}, want {e['stderr_' + key]!r}"
        return ""


def run(sgo, only=None):
    cases = load_cases()
    results = {}
    for name, d in cases.items():
        if only and name not in only:
            continue
        try:
            results[name] = run_case(name, d, sgo)
        except subprocess.TimeoutExpired:
            results[name] = "build timed out"
    failed = {k: v for k, v in results.items() if v}
    for k, v in failed.items():
        print(f"FAIL {k}: {v}")
    print(f"conformance: {len(results) - len(failed)} passed, {len(failed)} failed of {len(results)}")
    return results


FAKE = r'''#!/usr/bin/env python3
# Fake sgo driver for the runner self-test. MODE=good reproduces each case's expectation;
# MODE=bad produces wrong results. It never compiles anything.
import json, os, sys
mode = os.path.basename(sys.argv[0]).split("-")[-1]
args = sys.argv[1:]
out = args[args.index("-o") + 1]
e = json.load(open("expect.json"))
if e["kind"] == "reject":
    for d in e["diagnostics"]:
        pos = d["position"]  # the self-test rewrites expectations with explicit positions
        print(f"{pos}: {d['code'] if mode == 'good' else 'SGO_E_WRONG'}: fake", file=sys.stderr)
    sys.exit(1)
stdout = e.get("stdout", (e.get("stdout_any_of") or [""])[0])
stderr = e.get("stderr_exact", "")
if not stderr and ("stderr_kind" in e or "stderr_message" in e):
    stderr = json.dumps({"sgo_diagnostic": 1, "kind": e.get("stderr_kind", "Panic"), "message": e.get("stderr_message", ""),
                         "position": None, "task_id": 0, "scope_id": None, "chain": []}) + "\n"
code = e.get("exit", 0) if mode == "good" else 99
with open(out, "w") as f:
    f.write("#!/usr/bin/env python3\nimport sys\nsys.stdout.write(" + repr(stdout) + ")\nsys.stderr.write(" + repr(stderr) + ")\nsys.exit(" + str(code) + ")\n")
os.chmod(out, 0o755)
'''


def selftest():
    """The runner must pass sampled cases with a driver that reproduces their expectations, and
    fail them with a driver that does not. Reject expectations are copied with explicit positions
    (resolved by position()) so the fake driver can print them."""
    real = position
    cases = load_cases()
    sample = [n for n in cases if json.load(open(os.path.join(cases[n], "expect.json"))).get("kind") == "run"
              and not json.load(open(os.path.join(cases[n], "expect.json"))).get("expect_watchdog")
              and not json.load(open(os.path.join(cases[n], "expect.json"))).get("procedure")][:5]
    rej = [n for n in cases if json.load(open(os.path.join(cases[n], "expect.json"))).get("kind") == "reject"
           and not json.load(open(os.path.join(cases[n], "expect.json"))).get("procedure")][:5]
    with tempfile.TemporaryDirectory() as tmp:
        for mode in ("good", "bad"):
            p = os.path.join(tmp, f"sgo-fake-{mode}")
            open(p, "w").write(FAKE)
            os.chmod(p, 0o755)
        ok = True
        for mode, want_pass in (("good", True), ("bad", False)):
            drv = os.path.join(tmp, f"sgo-fake-{mode}")
            for name in sample + rej:
                d = cases[name]
                e = json.load(open(os.path.join(d, "expect.json")))
                scratch = os.path.join(tmp, "cases", mode, name)
                import shutil
                shutil.copytree(d, scratch)
                if e["kind"] == "reject":
                    for dg in e["diagnostics"]:
                        dg["position"] = real(d, dg)
                        dg.pop("at", None)
                    json.dump(e, open(os.path.join(scratch, "expect.json"), "w"))
                res = run_case(name, scratch, drv)
                if (res == "") != want_pass:
                    ok = False
                    print(f"selftest: {mode} driver on {name}: {'unexpected failure ' + res if want_pass else 'not detected'}")
        n = len(sample) + len(rej)
        print(f"selftest: {'ok' if ok else 'FAILED'}: good driver passes and bad driver fails {n} sampled cases ({len(sample)} run, {len(rej)} reject)")
        return 0 if ok else 1


def main():
    if len(sys.argv) < 2 or sys.argv[1] not in ("validate", "run", "selftest"):
        print(__doc__)
        return 2
    if sys.argv[1] == "validate":
        return validate()
    if sys.argv[1] == "selftest":
        return selftest()
    if "--sgo" not in sys.argv:
        print("conformance: run needs --sgo PATH", file=sys.stderr)
        return 2
    args = sys.argv[2:]
    sgo = args[args.index("--sgo") + 1]
    del args[args.index("--sgo"):args.index("--sgo") + 2]
    pending = {}
    if "--pending" in args:
        i = args.index("--pending")
        pending = json.load(open(args[i + 1]))
        del args[i:i + 2]
    res = run(os.path.abspath(sgo), args or None)
    bad = 0
    for name, why in sorted(res.items()):
        if name in pending and not why:
            print(f"UNEXPECTED PASS {name}: remove it from the pending list ({pending[name]})")
            bad += 1
        elif why and name not in pending:
            bad += 1
    unknown = sorted(set(pending) - set(load_cases()))
    for name in unknown:
        print(f"PENDING UNKNOWN {name}: not a case")
    npend = sum(1 for name, why in res.items() if why and name in pending)
    print(f"conformance: {npend} pending (listed in --pending), {bad} unexpected result(s)")
    return 1 if bad or unknown else 0


if __name__ == "__main__":
    sys.exit(main())
