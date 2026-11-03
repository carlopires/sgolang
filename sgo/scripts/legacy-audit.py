#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""legacy-audit.py - the no-legacy-path audit of what ships (D77, S17; W12 R3 residual D-R3).

Every flag and environment input of the shipped tools must be S-Go-specified, refused with a
test, or inert with a test; sgo/docs/legacy-audit.json is the reviewed classification.

Flags (compile, asm, link): the tool's flag table (from -h) must equal the reviewed list, so a
flag added upstream fails until it is classified. The tool's allowlist (read from its source) must
name only flags the sgo driver writes. Every other flag is run against the shipped tool and must be
refused with SGO_E_USAGE and no output.

Environment: every variable named in the tools' sources must be classified. Refused variables are
run and must be refused; inert ones are set (one at a time, to a value that would matter if read)
for an sgo build and for a direct compile and link, which must produce byte-identical objects and
binaries. The release archive's contents are checked by make-release.py (D77), not here.

A self-test runs one admitted flag as if it were refused and one refused flag as if it were
admitted, and requires the harness to report both, so it cannot pass vacuously.
"""
import glob
import hashlib
import importlib.util
import json
import os
import re
import subprocess
import sys
import tempfile
import time

from sgotool import GOARCH, GOOS, ROOT, buildid

TOOLS = os.path.join(ROOT, "pkg", "tool", f"{GOOS}_{GOARCH}")
AUDIT = json.load(open(os.path.join(ROOT, "sgo", "docs", "legacy-audit.json")))
SPEC = open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip()
MAIN = 'package main\n\nimport "sgo"\n\nfunc main(root *sgo.Context) error {\n\treturn nil\n}\n'
ALLOWLIST_SRC = {
    "compile": ("src/cmd/compile/internal/base/sgoflags.go", r"SGoFlags(?:SGo|TCB)\s*=\s*\[\]string\{([^}]*)\}"),
    "asm": ("src/cmd/asm/main.go", r"sgoAsmFlags\s*=\s*\[\]string\{([^}]*)\}"),
    "link": ("src/cmd/link/internal/ld/sgoidcfg.go", r"sgoLinkFlags\s*=\s*\[\]string\{([^}]*)\}"),
}
# Scripts that write evidence or artifacts: each must refuse an unknown argument (exit 2) and answer
# --help (exit 0) without running.
STRICT_SCRIPTS = ["stress.py", "make-release.py", "bench.py", "conformance.py", "conformance-compile.py",
                  "guarantee-check.py", "inventory-check.py", "license-audit.py", "lock-alloc-check.py",
                  "policy-report.py", "rejection-report.py", "spec-tool.py", "tcb-manifest.py", "adoption-check.py",
                  "test-check.py", "test-examples.py", "test-isolation.py", "test-test.py"]
ENV_DIRS = ["src/cmd/sgo", "src/cmd/compile", "src/cmd/asm", "src/cmd/link", "src/cmd/internal", "src/internal/buildcfg"]
ENV_RE = re.compile(r'(?:os\.Getenv|os\.LookupEnv|envOr|syscall\.Getenv)\("([A-Za-z0-9_]+)"')


def run(args, cwd, env):
    return subprocess.run(args, cwd=cwd, env=env, capture_output=True, text=True)


def flag_table(tool):
    r = run([os.path.join(TOOLS, tool), "-help"], ROOT, {"PATH": "/usr/bin:/bin"})
    return sorted(set(re.findall(r"^  -(\S+)", r.stdout + r.stderr, re.M)))


def flag_types(tool):
    r = run([os.path.join(TOOLS, tool), "-help"], ROOT, {"PATH": "/usr/bin:/bin"})
    out = {}
    for m in re.finditer(r"^  -(\S+)(?: (\S+))?", r.stdout + r.stderr, re.M):
        out[m.group(1)] = m.group(2) or ""
    return out


def allowlist(tool):
    path, pat = ALLOWLIST_SRC[tool]
    src = open(os.path.join(ROOT, path)).read()
    names = set()
    for m in re.finditer(pat, src):
        names |= set(re.findall(r'"([^"]+)"', m.group(1)))
    return names


def driver_writes(flag):
    src = "".join(open(p).read() for p in glob.glob(os.path.join(ROOT, "src", "cmd", "sgo", "*.go"))
                  if not p.endswith("_test.go"))
    return re.search(r'"-' + re.escape(flag) + r'(=|")', src) is not None


def sha(path):
    return hashlib.sha256(open(path, "rb").read()).hexdigest() if os.path.exists(path) else None


class Bench:
    """A module, the driver's TCB archives, and direct compile and link command lines."""

    def __init__(self, tmp):
        self.tmp = tmp
        self.env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, "cache")}
        self.mod = os.path.join(tmp, "m")
        os.makedirs(self.mod)
        open(os.path.join(self.mod, "sgo.mod"), "w").write(f"module example.com/m\n\nsgo {SPEC}\n")
        open(os.path.join(self.mod, "main.sgo"), "w").write(MAIN)
        self.binary = self.sgo_build({})
        if self.binary is None:
            sys.exit("legacy-audit: reference sgo build failed")
        man = glob.glob(os.path.join(tmp, "cache", "sgo-driver", "tcb", "*", "manifest.sha256"))
        self.cfg = os.path.join(tmp, "importcfg")
        with open(self.cfg, "w") as f:
            for a in sorted(glob.glob(os.path.join(os.path.dirname(man[0]), "*.a"))):
                f.write(f"packagefile {os.path.basename(a)[:-2].replace('%', '/')}={a}\n")
        self.obj, self.exe = self.direct({})
        if self.obj is None or self.exe is None:
            sys.exit("legacy-audit: reference direct compile and link failed")
        self.asm_src = os.path.join(tmp, "a.s")
        open(self.asm_src, "w").write("TEXT ·f(SB),0,$0-0\n\tRET\n")

    def sgo_build(self, extra, name="ref"):
        out = os.path.join(self.tmp, "bin-" + name)
        r = run([os.path.join(TOOLS, "sgo"), "build", "-o", out], self.mod, {**self.env, **extra})
        return sha(out) if r.returncode == 0 else None

    def compile_args(self, out):
        return ["-o", out, "-p", "main", "-pack", "-complete", "-importcfg", self.cfg, "main.sgo"]

    def direct(self, extra, name="ref"):
        a = os.path.join(self.tmp, f"main-{name}.a")
        exe = os.path.join(self.tmp, f"exe-{name}")
        env = {**self.env, **extra}
        r = run([os.path.join(TOOLS, "compile")] + self.compile_args(a), self.mod, env)
        if r.returncode != 0:
            return None, None
        r = run([os.path.join(TOOLS, "link"), "-o", exe, "-importcfg", self.cfg, "-buildmode=exe", buildid(), a],
                self.mod, env)
        return sha(a), sha(exe) if r.returncode == 0 else None

    def tool_cmd(self, tool, flagargs, out):
        if tool == "compile":
            return [os.path.join(TOOLS, "compile")] + flagargs + self.compile_args(out)
        if tool == "asm":
            return [os.path.join(TOOLS, "asm"), "-o", out, "-p", "main"] + flagargs + [self.asm_src]
        a = os.path.join(self.tmp, "main-ref.a")
        return ([os.path.join(TOOLS, "link"), "-o", out, "-importcfg", self.cfg, "-buildmode=exe", buildid()]
                + flagargs + [a])


def refusal(bench, tool, args, label, extra_env=None):
    """'' if the tool refuses args with SGO_E_USAGE and writes nothing, else the problem."""
    out = os.path.join(bench.tmp, "refused.out")
    if os.path.exists(out):
        os.remove(out)
    r = run(bench.tool_cmd(tool, args, out), bench.mod, {**bench.env, **(extra_env or {})})
    text = r.stdout + r.stderr
    if r.returncode == 0 or "SGO_E_USAGE" not in text:
        return f"{tool} {label}: exit {r.returncode}, want SGO_E_USAGE:\n{text[-400:]}"
    if os.path.exists(out):
        return f"{tool} {label}: refused but wrote output"
    return ""


def check_archive(tmp):
    import tarfile
    out = os.path.join(tmp, "release")
    os.makedirs(out, exist_ok=True)
    r = run([sys.executable, os.path.join(ROOT, "sgo", "scripts", "make-release.py"), "--out", out], ROOT,
            dict(os.environ))
    if r.returncode != 0:
        return [f"archive: make-release.py failed: {(r.stdout + r.stderr)[-300:]}"]
    arch = glob.glob(os.path.join(out, "sgo-*.tar.gz"))
    problems = []
    # R13: the release tool refuses what it does not specify and never writes into the tree.
    before = set(glob.glob(os.path.join(ROOT, "sgo-*.tar.gz")))
    for args, want in ((["--bogus-flag", "--out", os.path.join(tmp, "bogus")], 2), (["--help"], 0)):
        rr = run([sys.executable, os.path.join(ROOT, "sgo", "scripts", "make-release.py")] + args, ROOT, dict(os.environ))
        wrote = glob.glob(os.path.join(tmp, "bogus", "*.tar.gz")) + sorted(set(glob.glob(os.path.join(ROOT, "sgo-*.tar.gz"))) - before)
        if rr.returncode != want or wrote:
            problems.append(f"archive: make-release.py {' '.join(args)}: exit {rr.returncode} (want {want}), wrote {wrote}")
    tcb = json.load(open(os.path.join(ROOT, "sgo", "TARGET_TCB.json")))
    want_src = {f["path"] for f in tcb["files"]}
    # The standard library ships as source with its MANIFEST (SGO-IMP-004): not TCB, but exactly the
    # git-tracked files of src/sgostd, as make-release.py copies them.
    want_std = set(subprocess.check_output(["git", "ls-files", "src/sgostd"], cwd=ROOT, text=True).split())
    spec = importlib.util.spec_from_file_location("mkrel", os.path.join(ROOT, "sgo", "scripts", "make-release.py"))
    mk = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mk)
    headers = set(mk.asm_headers(tcb))
    tools = {f"pkg/tool/{GOOS}_{GOARCH}/{t}" for t in ("sgo", "compile", "asm", "link")}
    srcs = set()
    with tarfile.open(arch[0]) as t:
        for m in t.getmembers():
            rel = m.name.split("/", 1)[1] if "/" in m.name else ""
            if not rel:
                continue
            top = rel.split("/")[0]
            if top not in AUDIT["archive"]["top_level"]:
                problems.append(f"archive: unexpected top-level entry {rel}")
            if m.issym():
                if rel != "bin/sgo":
                    problems.append(f"archive: unexpected symlink {rel}")
            elif m.isfile():
                if m.mode & 0o111 and rel not in tools:
                    problems.append(f"archive: unexpected executable {rel}")
                if rel.startswith("src/"):
                    srcs.add(rel)
                    if rel not in want_src and rel not in headers and rel not in want_std:
                        problems.append(f"archive: {rel} is not a TCB or standard-library file")
    for f in sorted(want_src - srcs):
        problems.append(f"archive: TCB file {f} missing")
    for f in sorted(want_std - srcs):
        problems.append(f"archive: standard-library file {f} missing")
    return problems


def flag_args(tool, name, typ, tmp):
    v = AUDIT["flags"]["values"].get(tool, {}).get(name)
    if v == "@EMPTYJSON":
        v = os.path.join(tmp, "empty.json")
        open(v, "w").write('{"Patterns": {}, "Files": {}}')
    if v is not None:
        return [f"-{name}={v}"]
    return [f"-{name}"] if typ == "" else [f"-{name}=x"]


def info_flag(bench, tool, name):
    """'' if the tool prints and exits without writing output, else the problem."""
    out = os.path.join(bench.tmp, "info.out")
    if os.path.exists(out):
        os.remove(out)
    r = run(bench.tool_cmd(tool, [f"-{name}"], out), bench.mod, bench.env)
    if r.returncode != 0 or os.path.exists(out) or not (r.stdout + r.stderr).startswith(tool + " version"):
        return f"{tool} -{name}: want the version printed and no output, got exit {r.returncode}"
    return ""


def main():
    problems = []
    checked = {"refused_flags": 0, "driver_flags": 0, "refused_env": 0, "inert_env": 0, "driver_commands": 0}
    with tempfile.TemporaryDirectory() as tmp:
        bench = Bench(os.path.realpath(tmp))

        # Flags.
        for tool in ("compile", "asm", "link"):
            table, reviewed = flag_table(tool), sorted(AUDIT["flags"][tool])
            for f in sorted(set(table) - set(reviewed)):
                problems.append(f"{tool} -{f}: in the tool's flag table but not classified in legacy-audit.json")
            for f in sorted(set(reviewed) - set(table)):
                problems.append(f"{tool} -{f}: classified in legacy-audit.json but not in the tool's flag table")
            allowed = allowlist(tool)
            for f in sorted(allowed):
                if f not in table:
                    problems.append(f"{tool} -{f}: allowlisted but not a flag of the tool")
                elif not driver_writes(f) and not (tool == "compile" and f == "m"):
                    problems.append(f"{tool} -{f}: allowlisted but the sgo driver never writes it")
                checked["driver_flags"] += 1
            types = flag_types(tool)
            info = set(AUDIT["flags"]["info"].get(tool, []))
            for f in table:
                if f in allowed:
                    continue
                if f in info:
                    if msg := info_flag(bench, tool, f):
                        problems.append(msg)
                    continue
                if msg := refusal(bench, tool, flag_args(tool, f, types.get(f, ""), bench.tmp), f"-{f}"):
                    problems.append(msg)
                checked["refused_flags"] += 1

        # Environment.
        vars_ = AUDIT["env"]["vars"]
        found = {}
        for d in ENV_DIRS:
            for p in glob.glob(os.path.join(ROOT, d, "**", "*.go"), recursive=True):
                if p.endswith("_test.go") or "/testdata/" in p:
                    continue
                for name in ENV_RE.findall(open(p, encoding="utf-8").read()):
                    found.setdefault(name, os.path.relpath(p, ROOT))
        for name, where in sorted(found.items()):
            if name not in vars_:
                problems.append(f"env {name}: read in {where} but not classified in legacy-audit.json")
        bad = {"GOEXPERIMENT": "noregabiargs", "GOCOMPILEDEBUG": "loopvarhash=1",
               "GOFIPS140": "latest", "GOAMD64": "v3", "GOARM64": "v8.1"}
        for name, c in sorted(vars_.items()):
            if c["class"] == "refused":
                if name == "GOAMD64" and GOARCH != "amd64" or name == "GOARM64" and GOARCH != "arm64":
                    continue
                tools = c.get("tools") or (["compile"] if name == "GOCOMPILEDEBUG" else ["compile", "asm", "link"])
                v = c.get("value") or bad[name]
                for tool in tools:
                    if msg := refusal(bench, tool, [], f"{name}={v}", {name: v}):
                        problems.append(msg)
                checked["refused_env"] += 1
            elif c["class"] == "inert":
                v = c["value"]
                b = bench.sgo_build({name: v}, name)
                if b != bench.binary:
                    problems.append(f"env {name}={v}: sgo build output differs (or failed)")
                o, e = bench.direct({name: v}, name)
                if (o, e) != (bench.obj, bench.exe):
                    problems.append(f"env {name}={v}: direct compile/link output differs (or failed)")
                checked["inert_env"] += 1
            elif c["class"] not in ("pinned", "driver", "other-target", "unshipped"):
                problems.append(f"env {name}: unknown class {c['class']!r}")

        # The sgo driver: its hand-written flag parsers (D77).
        drv = AUDIT["driver"]
        src = {p: open(p).read() for p in glob.glob(os.path.join(ROOT, "src", "cmd", "sgo", "*.go"))
               if not p.endswith("_test.go")}
        parsed = set()
        for text in src.values():
            parsed |= set(re.findall(r'\bname == "([a-z][a-z0-9-]*)"', text))
            parsed |= set(re.findall(r'\ba == "-([a-z][a-z0-9-]*)"', text))
            parsed |= set(re.findall(r'HasPrefix\(a, "-([a-z][a-z0-9-]*)="\)', text))
        parsed -= set(drv["not_flags"])
        spec = "".join(open(p).read() for p in glob.glob(os.path.join(ROOT, "sgo", "spec", "*.md")))
        for f in sorted(parsed - set(drv["flags"])):
            problems.append(f"sgo -{f}: parsed by the driver but not classified in legacy-audit.json")
        for f in sorted(set(drv["flags"]) - parsed):
            problems.append(f"sgo -{f}: classified in legacy-audit.json but the driver does not parse it")
        for f in sorted(drv["flags"]):
            if not re.search(r"`[^`]*-" + re.escape(f) + r"\b", spec):
                problems.append(f"sgo -{f}: not named in the specification (sgo/spec)")
        for cmd in drv["commands"]:
            r = run([os.path.join(TOOLS, "sgo")] + cmd + ["-gcflags=-l", "."], bench.mod, bench.env)
            if r.returncode == 0 or "SGO_E_USAGE" not in r.stdout + r.stderr:
                problems.append(f"sgo {' '.join(cmd)} -gcflags=-l: exit {r.returncode}, want SGO_E_USAGE")
        checked["driver_commands"] = len(drv["commands"])

        # The scripts that write evidence or artifacts refuse what they do not specify (W12 R13, R14).
        for sc in STRICT_SCRIPTS:
            path = os.path.join(ROOT, "sgo", "scripts", sc)
            for args, want in ((["--bogus-flag"], 2), (["--help"], 0)):
                t0 = time.time()
                r = run([sys.executable, path] + args, ROOT, {**dict(os.environ), "SGO_OUT": os.path.join(bench.tmp, "strict")})
                wrote = os.path.exists(os.path.join(bench.tmp, "strict"))
                if r.returncode != want or wrote or time.time() - t0 > 20:
                    problems.append(f"{sc} {args[0]}: exit {r.returncode} (want {want}), wrote output {wrote}, "
                                    f"{time.time() - t0:.0f}s")

        # The release archive: four executables, the TCB sources, nothing else (D77, D61).
        problems += check_archive(bench.tmp)

        # Self-test: the refusal check must reject an admitted command line, and the tool must still
        # refuse a known refused flag, so neither direction passes vacuously.
        if refusal(bench, "compile", [], "(no flag, self-test)") == "":
            problems.append("self-test: an admitted compile command line counted as refused; the harness is vacuous")
        if refusal(bench, "compile", ["-wb=false"], "-wb=false (self-test)") != "":
            problems.append("self-test: compile -wb=false was not refused")

    for p in problems:
        print(p)
    print(f"legacy-audit: {'ok' if not problems else 'FAIL'}: {checked['driver_flags']} driver flags written by sgo, "
          f"{checked['refused_flags']} other flags refused, {checked['refused_env']} target settings refused, "
          f"{checked['inert_env']} inert variables without effect on sgo build and direct compile/link; "
          f"sgo: {len(AUDIT['driver']['flags'])} specified flags, {checked['driver_commands']} commands refuse unknown flags; "
          f"release archive: four executables and the TCB sources; "
          f"{len(problems)} problem(s)")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
