#!/usr/bin/env python3
"""test-tcb.py - the compiler admits the target TCB only by exact identity (SGO-TCB-002).

In a scratch copy of the toolchain root (src, sgo, pkg/tool), a program builds; then
  1. an edited runtime file,
  2. an added runtime file, and
  3. an edited file of another TCB package (sgo)
each fail with exactly one SGO_E_TCB_DIGEST diagnostic naming the file, and nothing is built from
the unreviewed source; after restoring the file the program builds again. The real tree and cache
are never touched.
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

MAIN = 'package main\n\nimport "sgo"\n\nfunc main(root *sgo.Context) error {\n\treturn nil\n}\n'


def copytree(src, dst):
    # Clone copies on APFS; plain copies elsewhere.
    if GOOS == "darwin":
        subprocess.run(["cp", "-cR", src, dst], check=True)
    else:
        shutil.copytree(src, dst, symlinks=True)


def main():
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
        open(os.path.join(mod, "sgo.mod"), "w").write("module example.com/prog\n\nsgo 0.1.0\n")
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
        cases = [
            ("edited runtime file", os.path.join(root, "src", "runtime", "sgo_record.go"), "edit",
             "file runtime/sgo_record.go differs from its digest"),
            ("added runtime file", os.path.join(root, "src", "runtime", "sgo_extra.go"), "add",
             "file runtime/sgo_extra.go is not in sgo/TARGET_TCB.json"),
            ("edited sgo package file", os.path.join(root, "src", "sgo", "sgo.go"), "edit",
             "file sgo/sgo.go differs from its digest"),
        ]
        for label, path, how, msg in cases:
            if how == "edit":
                orig = open(path, "rb").read()
                open(path, "ab").write(b"\n// unreviewed change\n")
                build(label, path, msg)
                open(path, "wb").write(orig)
            else:
                open(path, "w").write("package runtime\n\nfunc sgoExtra() {}\n")
                build(label, path, msg)
                os.remove(path)
            build(f"restored after {label}")
    print("test-tcb: ok: clean build; edited runtime file, added runtime file, and edited sgo file "
          "each fail with one SGO_E_TCB_DIGEST; restored trees build")


if __name__ == "__main__":
    main()
