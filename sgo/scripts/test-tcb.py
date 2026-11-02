#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""test-tcb.py - the compiler admits the target TCB only by exact identity (SGO-TCB-002).

In a scratch copy of the toolchain root (src, sgo, pkg/tool), a program builds; then
  1. an edited runtime file,
  2. an added runtime file, and
  3. an edited file of another TCB package (sgo)
each fail with exactly one SGO_E_TCB_DIGEST diagnostic naming the file, and nothing is built from
the unreviewed source; after restoring the file the program builds again. The real tree and cache
are never touched.

Trust is a function of (import path, file name, digest), never of content (W12 R1): byte copies of
pinned TCB files placed in a user package (a subset, internal/stringslite, and a whole package,
internal/godebugs) go through admission and are rejected, through the driver and through a direct
compile invocation. Each copy's digest is first checked against TARGET_TCB.json, so the case
cannot pass vacuously when the TCB changes.
"""

import hashlib
import json
import glob
import os
import platform
import shutil
import subprocess
import sys
import tempfile

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
SPEC_VERSION = open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip()
GOOS = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
GOARCH = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[platform.machine()]
HOST = f"{GOOS}_{GOARCH}"
ARCH = GOARCH

MAIN = 'package main\n\nimport "sgo"\n\nfunc main(root *sgo.Context) error {\n\treturn nil\n}\n'


def copytree(src, dst):
    # Clone copies on APFS; plain copies elsewhere.
    if GOOS == "darwin":
        subprocess.run(["cp", "-cR", src, dst], check=True)
    else:
        shutil.copytree(src, dst, symlinks=True)


def pinned_copy(rel, dst):
    """Copy src/<rel> to dst, asserting it is byte-identical to its TARGET_TCB.json entry."""
    data = open(os.path.join(ROOT, "src", rel), "rb").read()
    tcb = {f["path"]: f["sha256"] for f in json.load(open(os.path.join(ROOT, "sgo", "TARGET_TCB.json")))["files"]}
    if tcb.get("src/" + rel) != hashlib.sha256(data).hexdigest():
        sys.exit(f"test-tcb: copy case: src/{rel} is not pinned with this digest; the case would be vacuous")
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    open(dst, "wb").write(data)
    return data.decode()


def copy_cases():
    tools = os.path.join(ROOT, "pkg", "tool", HOST)
    with tempfile.TemporaryDirectory() as tmp:
        tmp = os.path.realpath(tmp)
        env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, "cache")}
        # A subset of a TCB package, used from admitted code (the reviewer's probe).
        mod = os.path.join(tmp, "p2")
        src = pinned_copy("internal/stringslite/strings.go", os.path.join(mod, "sl", "strings.sgo"))
        line = next(i for i, l in enumerate(src.splitlines(), 1) if l.strip() == '"internal/bytealg"')
        open(os.path.join(mod, "sgo.mod"), "w").write("module example.com/p2\n\nsgo " + SPEC_VERSION + "\n")
        open(os.path.join(mod, "main.sgo"), "w").write(
            'package main\n\nimport (\n\t"example.com/p2/sl"\n\t"sgo"\n)\n\n'
            'func main(root *sgo.Context) error {\n\tout := root.Stdout()\n'
            '\treturn out.WriteString(stringslite.TrimPrefix("bypass: not admitted\\n", "bypass: "))\n}\n')
        r = subprocess.run([os.path.join(tools, "sgo"), "run", "."], cwd=mod, env=env, capture_output=True, text=True)
        want = f"sl/strings.sgo:{line}:2: SGO_E_UNADMITTED"
        if r.returncode == 0 or want not in r.stderr or "not admitted\n" in r.stdout:
            sys.exit(f"test-tcb: copied TCB file in a user package: want {want} and no run, got exit "
                     f"{r.returncode}:\n{r.stdout}{r.stderr}")
        # Every file of a TCB package that imports nothing: rejected by path, not by its imports.
        mod = os.path.join(tmp, "p3")
        pinned_copy("internal/godebugs/table.go", os.path.join(mod, "gd", "table.sgo"))
        open(os.path.join(mod, "sgo.mod"), "w").write("module example.com/p3\n\nsgo " + SPEC_VERSION + "\n")
        open(os.path.join(mod, "main.sgo"), "w").write(
            'package main\n\nimport (\n\t"example.com/p3/gd"\n\t"sgo"\n)\n\n'
            'func main(root *sgo.Context) error {\n\t_ = godebugs.All\n\treturn nil\n}\n')
        out = os.path.join(tmp, "p3.bin")
        r = subprocess.run([os.path.join(tools, "sgo"), "build", "-o", out, "."], cwd=mod, env=env,
                           capture_output=True, text=True)
        if r.returncode == 0 or "gd/table.sgo:" not in r.stderr or "SGO_E_UNADMITTED" not in r.stderr or os.path.exists(out):
            sys.exit(f"test-tcb: copied whole TCB package: want SGO_E_UNADMITTED and no binary, got exit "
                     f"{r.returncode}:\n{r.stdout}{r.stderr}")
        # The shipped compiler invoked directly (D77): same rule.
        r = subprocess.run([os.path.join(tools, "compile"), "-p", "example.com/p3/gd", "-o", os.path.join(tmp, "gd.a"),
                            os.path.join(mod, "gd", "table.sgo")], env=env, capture_output=True, text=True)
        if r.returncode == 0 or "SGO_E_UNADMITTED" not in r.stdout + r.stderr:
            sys.exit(f"test-tcb: direct compile of a copied TCB package under a user path: want SGO_E_UNADMITTED, "
                     f"got exit {r.returncode}:\n{r.stdout}{r.stderr}")


def main():
    copy_cases()
    with tempfile.TemporaryDirectory() as tmp:
        tmp = os.path.realpath(tmp)
        root = os.path.join(tmp, "goroot")
        os.makedirs(os.path.join(root, "pkg"))
        for d in ("src", "sgo"):
            copytree(os.path.join(ROOT, d), os.path.join(root, d))
        copytree(os.path.join(ROOT, "pkg", "tool"), os.path.join(root, "pkg", "tool"))
        sgo = os.path.join(root, "pkg", "tool", HOST, "sgo")
        mod = os.path.join(tmp, "prog")
        os.makedirs(mod)
        open(os.path.join(mod, "sgo.mod"), "w").write("module example.com/prog\n\nsgo " + SPEC_VERSION + "\n")
        open(os.path.join(mod, "main.sgo"), "w").write(MAIN)
        env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, "cache")}

        def build(label, want_file=None, want_msg=None):
            r = subprocess.run([sgo, "build", "-o", os.path.join(tmp, "p")], cwd=mod, env=env,
                               capture_output=True, text=True)
            out = r.stdout + r.stderr
            if want_file is None:
                if r.returncode != 0:
                    sys.exit(f"test-tcb: {label}: build failed:\n{out}")
                return
            lines = [l for l in out.splitlines() if l.strip()]
            want = f"{want_file}:1:1: SGO_E_TCB_DIGEST: "
            if r.returncode == 0 or len(lines) != 1 or not lines[0].startswith(want) or want_msg not in lines[0]:
                sys.exit(f"test-tcb: {label}: want one diagnostic {want}...{want_msg}, got exit {r.returncode}:\n{out}")

        build("clean tree")
        # W12 R8: a cached TCB archive is reused only if it matches the entry's manifest. Flip one
        # byte of the cached runtime archive: the next build must rebuild the entry (the archive
        # comes back byte for byte) instead of linking the altered one.
        cached = glob.glob(os.path.join(tmp, "cache", "sgo-driver", "tcb", "*", "runtime.a"))
        if len(cached) != 1 or not glob.glob(os.path.join(os.path.dirname(cached[0]), "manifest.sha256")):
            sys.exit(f"test-tcb: cached TCB entry: want one runtime.a with a manifest, got {cached}")
        good = open(cached[0], "rb").read()
        bad = bytearray(good)
        bad[len(bad) // 2] ^= 0xFF
        open(cached[0], "wb").write(bytes(bad))
        build("altered cached runtime archive")
        if open(cached[0], "rb").read() != good:
            sys.exit("test-tcb: an altered cached runtime archive was reused, not rebuilt")
        cases = [
            ("edited runtime file", os.path.join(root, "src", "runtime", "sgo_record.go"), "edit",
             "file runtime/sgo_record.go differs from its digest"),
            ("added runtime file", os.path.join(root, "src", "runtime", "sgo_extra.go"), "add",
             "file runtime/sgo_extra.go is not in sgo/TARGET_TCB.json"),
            ("edited sgo package file", os.path.join(root, "src", "sgo", "sgo.go"), "edit",
             "file sgo/sgo.go differs from its digest"),
            # The assembler does not check digests; the driver verifies every assembler input of
            # the TCB, headers included, transitively (W10).
            ("edited runtime assembly file", os.path.join(root, "src", "runtime", f"asm_{ARCH}.s"), "edit",
             f"assembler input runtime/asm_{ARCH}.s differs from its digest"),
            ("edited included header outside the TCB packages", os.path.join(root, "src", "runtime", "cgo", f"abi_{ARCH}.h"), "edit",
             f"assembler input runtime/cgo/abi_{ARCH}.h differs from its digest"),
            ("added runtime assembly file", os.path.join(root, "src", "runtime", "sgo_extra.s"), "add-asm",
             "assembler input runtime/sgo_extra.s is not in sgo/TARGET_TCB.json"),
        ]
        for label, path, how, msg in cases:
            if how == "edit":
                orig = open(path, "rb").read()
                open(path, "ab").write(b"\n// unreviewed change\n")
                build(label, path, msg)
                open(path, "wb").write(orig)
            elif how == "add-asm":
                open(path, "w").write("#include \"textflag.h\"\n\nTEXT \u00b7sgoExtraAsm(SB),NOSPLIT,$0-0\n\tRET\n")
                build(label, path, msg)
                os.remove(path)
            else:
                open(path, "w").write("package runtime\n\nfunc sgoExtra() {}\n")
                build(label, path, msg)
                os.remove(path)
            build(f"restored after {label}")
    print("test-tcb: ok: clean build; edited runtime file, added runtime file, edited sgo file, edited and "
          "added assembly file, and edited included header each fail with one SGO_E_TCB_DIGEST; restored trees build; "
          "an altered cached TCB archive is rebuilt, not reused; "
          "byte copies of TCB files under user paths (driver and direct compile) go through admission and are rejected")


if __name__ == "__main__":
    main()
