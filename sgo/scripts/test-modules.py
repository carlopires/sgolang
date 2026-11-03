#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""test-modules.py — tool tests of required modules and the standard library (SGO-IMP-003, SGO-IMP-004).

sgo mod sum / sgo mod verify (D113): builds never write sgo.sum; sgo mod sum writes exactly the
recorded file; verify accepts it and refuses a vendored edit, as the build does.

The standard library (D115), in a scratch copy of the toolchain root: an edited, an added, and a
removed std file are each refused with SGO_E_STD_MANIFEST; a std package listed in a regenerated
MANIFEST is still compiled under the full rules: importing a Go standard-library package or a
module package is refused (SGO-IMP-004), and so is a denied construct.
"""

import hashlib
import os
import platform
import shutil
import subprocess
import sys
import tempfile

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
CASES = os.path.join(ROOT, "sgo", "testdata", "conformance")
GOOS = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
GOARCH = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[platform.machine()]
HOST = f"{GOOS}_{GOARCH}"


def copytree(src, dst):
    if GOOS == "darwin":
        subprocess.run(["cp", "-cR", src, dst], check=True)
    else:
        shutil.copytree(src, dst, symlinks=True)


def fail(msg):
    sys.exit("test-modules: " + msg)


def run(sgo, args, cwd, env):
    r = subprocess.run([sgo] + args, cwd=cwd, env=env, capture_output=True, text=True, timeout=300)
    return r.returncode, r.stdout + r.stderr


def expect_code(label, sgo, args, cwd, env, code, where=None):
    rc, out = run(sgo, args, cwd, env)
    if rc == 0 or code not in out or (where and where not in out):
        fail(f"{label}: want {code}{' at ' + where if where else ''}, got exit {rc}:\n{out}")


def expect_ok(label, sgo, args, cwd, env):
    rc, out = run(sgo, args, cwd, env)
    if rc != 0:
        fail(f"{label}: want success, got exit {rc}:\n{out}")
    return out


def write_manifest(std):
    lines = []
    for d, dirs, files in os.walk(std):
        for f in files:
            if f.endswith(".sgo") and not f.endswith("_test.sgo"):
                p = os.path.join(d, f)
                lines.append((os.path.relpath(p, std).replace(os.sep, "/"), hashlib.sha256(open(p, "rb").read()).hexdigest()))
    open(os.path.join(std, "MANIFEST"), "w").write("".join(f"{h}  {r}\n" for r, h in sorted(lines)))


def main():
    with tempfile.TemporaryDirectory() as tmp:
        tmp = os.path.realpath(tmp)
        env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, "cache")}
        sgo = os.path.join(ROOT, "pkg", "tool", HOST, "sgo")

        # sgo mod sum / verify.
        prog = os.path.join(tmp, "prog")
        shutil.copytree(os.path.join(CASES, "mod-require-vendor"), prog)
        recorded = open(os.path.join(prog, "sgo.sum")).read()
        os.remove(os.path.join(prog, "sgo.sum"))
        expect_code("build without sgo.sum", sgo, ["build", "-o", os.path.join(tmp, "p")], prog, env, "SGO_E_MODULE_SUM")
        if os.path.exists(os.path.join(prog, "sgo.sum")):
            fail("a build wrote sgo.sum")
        expect_code("verify without sgo.sum", sgo, ["mod", "verify"], prog, env, "SGO_E_MODULE_SUM")
        expect_ok("mod sum", sgo, ["mod", "sum"], prog, env)
        if open(os.path.join(prog, "sgo.sum")).read() != recorded:
            fail("sgo mod sum wrote a different sgo.sum than the fixture records")
        expect_ok("mod verify", sgo, ["mod", "verify"], prog, env)
        expect_ok("build after mod sum", sgo, ["build", "-o", os.path.join(tmp, "p")], prog, env)
        lib = os.path.join(prog, "vendor", "example.com", "lib", "lib.sgo")
        open(lib, "a").write("\n// edited after sgo mod sum\n")
        expect_code("verify after vendored edit", sgo, ["mod", "verify"], prog, env, "SGO_E_MODULE_SUM")
        expect_code("build after vendored edit", sgo, ["build", "-o", os.path.join(tmp, "p")], prog, env, "SGO_E_MODULE_SUM")
        expect_code("mod sum outside a module", sgo, ["mod", "sum"], tmp, env, "SGO_E_")
        expect_code("mod with a bad subcommand", sgo, ["mod", "tidy"], prog, env, "SGO_E_USAGE")

        # The standard library, in a scratch toolchain root.
        root = os.path.join(tmp, "goroot")
        os.makedirs(os.path.join(root, "pkg"))
        for d in ("src", "sgo"):
            copytree(os.path.join(ROOT, d), os.path.join(root, d))
        copytree(os.path.join(ROOT, "pkg", "tool"), os.path.join(root, "pkg", "tool"))
        ssgo = os.path.join(root, "pkg", "tool", HOST, "sgo")
        std = os.path.join(root, "src", "sgostd")
        user = os.path.join(tmp, "user")
        shutil.copytree(os.path.join(CASES, "std-import"), user)
        build = ["build", "-o", os.path.join(tmp, "p")]
        expect_ok("std clean", ssgo, build, user, env)

        sc = os.path.join(std, "strconv", "strconv.sgo")
        good = open(sc, "rb").read()
        open(sc, "ab").write(b"\n// edited\n")
        expect_code("std edited file", ssgo, build, user, env, "SGO_E_STD_MANIFEST", "std/strconv/strconv.sgo:1:1")
        open(sc, "wb").write(good)
        extra = os.path.join(std, "strconv", "extra.sgo")
        open(extra, "w").write("package strconv\n")
        expect_code("std added file", ssgo, build, user, env, "SGO_E_STD_MANIFEST", "std/strconv/extra.sgo:1:1")
        os.remove(extra)
        open(os.path.join(std, "strconv", "other.sgo"), "w").write("package strconv\n")
        write_manifest(std)
        os.remove(os.path.join(std, "strconv", "other.sgo"))
        expect_code("std removed file", ssgo, build, user, env, "SGO_E_STD_MANIFEST", "std/strconv/other.sgo:1:1")
        write_manifest(std)
        expect_ok("std restored", ssgo, build, user, env)

        # A std package listed in the MANIFEST is compiled under the full rules.
        bad = {
            "gostd": ('package bad\n\nimport "strings"\n\n// F is f.\nfunc F() string {\n\treturn strings.ToUpper("x")\n}\n',
                      "SGO_E_UNADMITTED", 'std/gostd/bad.sgo:3:8'),
            "modimp": ('package bad\n\nimport "example.com/lib"\n\n// F is f.\nfunc F() string {\n\treturn lib.Greeting("x")\n}\n',
                       "SGO_E_UNADMITTED", 'std/modimp/bad.sgo:3:8'),
            "denied": ('package bad\n\n// F is f.\nfunc F() {\n\tgo F()\n}\n', "SGO_E_", 'std/denied/bad.sgo:5:'),
        }
        for name, (src, code, where) in bad.items():
            os.makedirs(os.path.join(std, name))
            open(os.path.join(std, name, "bad.sgo"), "w").write(src)
            write_manifest(std)
            app = os.path.join(tmp, "app-" + name)
            os.makedirs(app)
            open(os.path.join(app, "sgo.mod"), "w").write(open(os.path.join(user, "sgo.mod")).read().replace("std-import", "app-" + name))
            open(os.path.join(app, "main.sgo"), "w").write(
                f'package main\n\nimport (\n\t"sgo"\n\t"std/{name}"\n)\n\nfunc main(root *sgo.Context) error {{\n\t_ = bad.F\n\treturn nil\n}}\n')
            expect_code(f"std package {name}", ssgo, build, app, env, code, where)
            shutil.rmtree(os.path.join(std, name))
            write_manifest(std)
    print("test-modules: ok: sgo mod sum/verify (no build writes sgo.sum; edits refused); std edited, added, and "
          "removed files refused; std packages importing Go std or a module, or using a denied construct, refused")


if __name__ == "__main__":
    main()
