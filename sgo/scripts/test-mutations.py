#!/usr/bin/env python3
"""test-mutations.py - the specification and inventory gates reject defective inputs (S02, S03, S18).

Each mutation is applied to a scratch copy of the repository (src, sgo, pkg/tool; a git repository
whose single commit is the unmodified copy, which is what spec-tool compares frozen clauses with),
checked, and reverted:

  S02  an unregistered runtime builtin, and an unregistered predeclared identifier
       -> inventory-check fails naming the surface
  S03  a removed clause, a TODO placeholder, a conflicting second version, an unknown reference
       -> spec-tool check fails naming the clause
  S18  a frozen spawn clause changed without a version bump -> spec-tool fails before and after
       `update`; an amended admission registry in the toolchain's GOROOT -> the driver refuses to
       build with SGO_E_SPEC_MISMATCH, and builds again once restored

The real tree is never modified.
"""

import os
import platform
import shutil
import subprocess
import sys
import tempfile

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
GOOS = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
GOARCH = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[platform.machine()]
HOST = f"{GOOS}_{GOARCH}"


def copytree(src, dst):
    if GOOS == "darwin":
        subprocess.run(["cp", "-cR", src, dst], check=True)
    else:
        shutil.copytree(src, dst, symlinks=True)


def edit(path, old, new, count=1):
    s = open(path).read()
    if old not in s:
        sys.exit(f"test-mutations: anchor not found in {path}: {old[:60]!r}")
    open(path, "w").write(s.replace(old, new, count))


def remove_clause(path, cid):
    s = open(path).read()
    i = s.index(f"### {cid} ")
    j = s.find("\n### ", i + 4)
    open(path, "w").write(s[:i] + (s[j + 1:] if j >= 0 else ""))


def duplicate_clause(path, cid):
    s = open(path).read()
    i = s.index(f"### {cid} ")
    j = s.find("\n### ", i + 4)
    block = s[i:j if j >= 0 else len(s)]
    open(path, "w").write(s + "\n" + block.replace("- **version:** 1", "- **version:** 2"))


def main():
    problems = []
    with tempfile.TemporaryDirectory() as tmp:
        tmp = os.path.realpath(tmp)
        repo = os.path.join(tmp, "repo")
        os.makedirs(os.path.join(repo, "pkg"))
        for d in ("src", "sgo"):
            copytree(os.path.join(ROOT, d), os.path.join(repo, d))
        copytree(os.path.join(ROOT, "pkg", "tool"), os.path.join(repo, "pkg", "tool"))
        env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, "cache"),
               "GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t",
               "GIT_COMMITTER_EMAIL": "t@t"}
        git = lambda *a: subprocess.run(["git"] + list(a), cwd=repo, env=env, check=True, capture_output=True)
        git("init", "-q")
        git("add", "sgo")
        git("commit", "-qm", "unmodified copy")

        def tool(*args):
            r = subprocess.run(list(args), cwd=repo, env=env, capture_output=True, text=True)
            return r.returncode, r.stdout + r.stderr

        def expect_fail(label, want, *args):
            code, out = tool(*args)
            if code == 0 or want not in out:
                problems.append(f"{label}: exit {code}, want failure mentioning {want!r}:\n{out[-600:]}")

        def revert():
            git("checkout", "-q", "--", "sgo")
            for rel in ("src/cmd/compile/internal/typecheck/_builtin/runtime.go",
                        "src/cmd/compile/internal/types2/universe.go"):
                shutil.copy(os.path.join(ROOT, rel), os.path.join(repo, rel))

        spec = [sys.executable, "sgo/scripts/spec-tool.py"]
        inv = [sys.executable, "sgo/scripts/inventory-check.py"]
        for args in (spec + ["check"], inv):
            code, out = tool(*args)
            if code != 0:
                sys.exit(f"test-mutations: unmodified copy fails {' '.join(args[1:])}:\n{out}")

        # S02: unregistered compiler surfaces.
        edit(os.path.join(repo, "src/cmd/compile/internal/typecheck/_builtin/runtime.go"),
             "func panicdivide()", "func sgoUnregisteredHook()\nfunc panicdivide()")
        expect_fail("S02 unregistered runtime builtin", "unmapped runtime-builtin sgoUnregisteredHook", *inv)
        revert()
        edit(os.path.join(repo, "src/cmd/compile/internal/types2/universe.go"),
             '\t_Recover: {"recover", 0, false, statement},',
             '\t_Recover: {"recover", 0, false, statement},\n\t_Spawn2:  {"spawn2", 1, false, statement},')
        expect_fail("S02 unregistered predeclared identifier", "unmapped universe spawn2", *inv)
        revert()

        # S03: missing, draft, conflicting, and unresolved clauses.
        remove_clause(os.path.join(repo, "sgo/spec/SPEC-WAIT.md"), "SGO-WAIT-002")
        expect_fail("S03 removed clause", "unknown clause SGO-WAIT-002", *spec, "check")
        revert()
        edit(os.path.join(repo, "sgo/spec/SPEC-TIMER.md"), "evaluation_and_value_semantics:** ",
             "evaluation_and_value_semantics:** TODO ")
        expect_fail("S03 TODO placeholder", "SGO-TIMER-001: evaluation_and_value_semantics: forbidden placeholder",
                    *spec, "check")
        revert()
        duplicate_clause(os.path.join(repo, "sgo/spec/SPEC-TIMER.md"), "SGO-TIMER-001")
        expect_fail("S03 conflicting versions", "SGO-TIMER-001: duplicate clause ID", *spec, "check")
        revert()
        edit(os.path.join(repo, "sgo/spec/SPEC-TIMER.md"), "reference_closure:** ",
             "reference_closure:** SGO-NOPE-999, ")
        expect_fail("S03 unresolved reference", "reference to unknown clause SGO-NOPE-999", *spec, "check")
        revert()

        # S18: an amendment must be propagated everywhere, starting with a version bump.
        edit(os.path.join(repo, "sgo/spec/SPEC-SPAWN.md"), "evaluation_and_value_semantics:** ",
             "evaluation_and_value_semantics:** (amended) ")
        expect_fail("S18 frozen clause changed (check)", "SGO_SPEC_ADMISSION.json is stale", *spec, "check")
        expect_fail("S18 frozen clause changed (update)", "frozen clause changed without a version bump",
                    *spec, "update")
        revert()

        mod = os.path.join(tmp, "prog")
        os.makedirs(mod)
        open(os.path.join(mod, "sgo.mod"), "w").write("module example.com/prog\n\nsgo 0.1.0\n")
        open(os.path.join(mod, "main.sgo"), "w").write(
            'package main\n\nimport "sgo"\n\nfunc main(root *sgo.Context) error {\n\treturn nil\n}\n')
        sgo = os.path.join(repo, "pkg", "tool", HOST, "sgo")

        def build():
            r = subprocess.run([sgo, "build", "-o", os.path.join(tmp, "p")], cwd=mod, env=env,
                               capture_output=True, text=True)
            return r.returncode, r.stdout + r.stderr

        code, out = build()
        if code != 0:
            sys.exit(f"test-mutations: unmodified toolchain build failed:\n{out}")
        adm = os.path.join(repo, "sgo", "SGO_SPEC_ADMISSION.json")
        orig = open(adm, "rb").read()
        open(adm, "wb").write(orig.rstrip(b"\n") + b"\n\n")
        code, out = build()
        if code == 0 or "SGO_E_SPEC_MISMATCH" not in out:
            problems.append(f"S18 amended admission registry: exit {code}, want SGO_E_SPEC_MISMATCH:\n{out}")
        open(adm, "wb").write(orig)
        code, out = build()
        if code != 0:
            problems.append(f"S18 restored admission registry: build failed:\n{out}")

    for p in problems:
        print("test-mutations:", p, file=sys.stderr)
    if problems:
        sys.exit(f"test-mutations: {len(problems)} problem(s)")
    print("test-mutations: ok: 2 inventory (S02), 4 spec (S03), and 3 amendment (S18) mutations rejected; "
          "unmodified copy passes")


if __name__ == "__main__":
    main()
