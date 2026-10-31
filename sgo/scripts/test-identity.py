#!/usr/bin/env python3
"""test-identity.py — executable test of package identity checks (SGO-IMP-002).

The sgo driver always recompiles user packages, so stale or foreign objects cannot reach the
linker through it; this test drives compile and link directly, the way a caching driver or a
tampered build would, and checks:

  1. a main package compiled against lib v1, linked with lib v2 whose exported API changed:
     the linker reports SGO_E_STALE_CONTRACT
  2. lib v2 that changes only unexported code is also stale: importers may contain inlined bodies
     of lib, so the linker's object fingerprints require rebuilding them (SGO_E_STALE_CONTRACT)
  3. an archive whose headers lack the S-Go identity (as from a stock Go toolchain) is rejected
     by the importing compiler and by the linker with SGO_E_FOREIGN_OBJECT
  4. only the import contract recorded in main's identity symbol differs (object fingerprints
     still match, so only the S-Go contract check can see it): SGO_E_STALE_CONTRACT
  5. lib's identity names another spec version: the importing compiler (export data) and the
     linker (identity symbol) report SGO_E_SPEC_MISMATCH
Mutations keep every byte length, so stored fingerprints stay valid.

    test-identity.py
"""
import glob
import os
import subprocess
import sys
import tempfile

ROOT = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
PLAT = {"Darwin": "darwin", "Linux": "linux"}[os.uname().sysname] + "_" + \
    {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[os.uname().machine]
TOOLS = os.path.join(ROOT, "pkg", "tool", PLAT)
GOOS, GOARCH = PLAT.split("_")

LIB_V1 = 'package lib\n\nfunc Value() int { return helper() }\n\nfunc helper() int { return 1 }\n'
LIB_V2_API = 'package lib\n\nfunc Value() string { return "v2" }\n'
LIB_V2_INTERNAL = 'package lib\n\nfunc Value() int { return helper() + 1 }\n\nfunc helper() int { return 2 }\n'
MAIN = ('package main\n\nimport (\n\t"sgo"\n\n\t"example.com/idt/lib"\n)\n\n'
        'func main(root *sgo.Context) error {\n\t_ = lib.Value()\n\treturn nil\n}\n')


def run(args, cwd, env, expect_ok=True):
    r = subprocess.run(args, cwd=cwd, env=env, capture_output=True, text=True)
    if expect_ok and r.returncode != 0:
        sys.exit(f"test-identity: {' '.join(args[:1])} failed:\n{r.stdout}{r.stderr}")
    return r


def main():
    with tempfile.TemporaryDirectory() as tmp:
        tmp = os.path.realpath(tmp)
        env = {"HOME": tmp, "PATH": "/usr/bin:/bin", "SGO_CACHE": os.path.join(tmp, "cache"),
               "GOROOT": ROOT, "GOOS": GOOS, "GOARCH": GOARCH, "CGO_ENABLED": "0",
               "GOEXPERIMENT": "", "GODEBUG": ""}
        mod = os.path.join(tmp, "idt")
        os.makedirs(os.path.join(mod, "lib"))
        open(os.path.join(mod, "sgo.mod"), "w").write("module example.com/idt\n\nsgo 0.1.0\n")
        open(os.path.join(mod, "lib", "lib.sgo"), "w").write(LIB_V1)
        open(os.path.join(mod, "main.sgo"), "w").write(MAIN)
        # The driver builds the target TCB into the cache; this test reuses those archives.
        run([os.path.join(TOOLS, "sgo"), "build", "-o", os.path.join(tmp, "prog")], mod, env)
        # 0. The binary records its build metadata (SGO-BUILD-001) as its build ID.
        prog = open(os.path.join(tmp, "prog"), "rb").read()
        version = run([os.path.join(TOOLS, "sgo"), "version"], mod, env).stdout.split()
        want = (f"sgo1/spec={version[3]}/admission={version[5]}/target={GOOS}-{GOARCH}/"
                f"toolchain={version[1]}/upstream={version[7]}").encode()
        if b'Go build ID: "' + want + b'"' not in prog:
            sys.exit(f"test-identity: binary does not record its build metadata {want!r}")
        tcb = glob.glob(os.path.join(tmp, "cache", "sgo-driver", "tcb", "*", "complete"))
        if len(tcb) != 1:
            sys.exit("test-identity: TCB cache not found")
        cfg = {}
        for a in glob.glob(os.path.join(os.path.dirname(tcb[0]), "*.a")):
            cfg[os.path.basename(a)[:-2].replace("%", "/")] = a

        def importcfg(extra):
            p = os.path.join(tmp, "importcfg")
            with open(p, "w") as f:
                for k, v in sorted({**cfg, **extra}.items()):
                    f.write(f"packagefile {k}={v}\n")
            return p

        def compile_lib(src, out):
            open(os.path.join(mod, "lib", "lib.sgo"), "w").write(src)
            run([os.path.join(TOOLS, "compile"), "-o", out, "-p", "example.com/idt/lib", "-pack", "-complete",
                 "-importcfg", importcfg({}), "lib/lib.sgo"], mod, env)

        def compile_main(lib, out, expect_ok=True):
            return run([os.path.join(TOOLS, "compile"), "-o", out, "-p", "main", "-pack", "-complete",
                        "-importcfg", importcfg({"example.com/idt/lib": lib}), "main.sgo"], mod, env, expect_ok)

        def link(lib, main_a):
            return run([os.path.join(TOOLS, "link"), "-o", os.path.join(tmp, "out"), "-importcfg",
                        importcfg({"example.com/idt/lib": lib}), "-buildmode=exe", main_a], tmp, env, False)

        v1, main_a = os.path.join(tmp, "lib1.a"), os.path.join(tmp, "main.a")
        compile_lib(LIB_V1, v1)
        compile_main(v1, main_a)
        if (r := link(v1, main_a)).returncode != 0:
            sys.exit(f"test-identity: consistent link failed:\n{r.stdout}{r.stderr}")

        # 1. Exported API changed after main was compiled.
        v2 = os.path.join(tmp, "lib2.a")
        compile_lib(LIB_V2_API, v2)
        r = link(v2, main_a)
        if r.returncode == 0 or "SGO_E_STALE_CONTRACT" not in r.stdout + r.stderr:
            sys.exit(f"test-identity: stale contract not detected:\n{r.stdout}{r.stderr}")

        # 2. Unexported change only: still stale for the linker (inlining), same code.
        v3 = os.path.join(tmp, "lib3.a")
        compile_lib(LIB_V2_INTERNAL, v3)
        r = link(v3, main_a)
        if r.returncode == 0 or "SGO_E_STALE_CONTRACT" not in r.stdout + r.stderr:
            sys.exit(f"test-identity: unexported change not detected:\n{r.stdout}{r.stderr}")

        # 3. Foreign archive: S-Go marker removed from every object header.
        data = open(v1, "rb").read()
        marker = b" sgo:sgo-abi-1"
        if data.count(marker) < 2:
            sys.exit("test-identity: expected the S-Go marker in export data and object headers")
        foreign = os.path.join(tmp, "foreign.a")
        open(foreign, "wb").write(data.replace(marker, b" " * len(marker)))
        r = compile_main(foreign, os.path.join(tmp, "main2.a"), expect_ok=False)
        if r.returncode == 0 or "SGO_E_FOREIGN_OBJECT" not in r.stdout + r.stderr:
            sys.exit(f"test-identity: compiler accepted foreign export data:\n{r.stdout}{r.stderr}")
        r = link(foreign, main_a)
        if r.returncode == 0 or "SGO_E_FOREIGN_OBJECT" not in r.stdout + r.stderr:
            sys.exit(f"test-identity: linker accepted a foreign object:\n{r.stdout}{r.stderr}")

        # 4. Only the contract main recorded for lib differs.
        mdata = open(main_a, "rb").read()
        i = mdata.find(b"import example.com/idt/lib ")
        if i < 0:
            sys.exit("test-identity: main's identity symbol does not record lib")
        j = i + len(b"import example.com/idt/lib ")
        digest = mdata[j:j + 32]
        flipped = bytes(b"0123456789abcdef"[(b"0123456789abcdef".index(c) + 1) % 16] for c in digest)
        stale_main = os.path.join(tmp, "main-stale.a")
        open(stale_main, "wb").write(mdata[:j] + flipped + mdata[j + 32:])
        r = link(v1, stale_main)
        if r.returncode == 0 or "SGO_E_STALE_CONTRACT" not in r.stdout + r.stderr or "fingerprint" in r.stdout + r.stderr:
            sys.exit(f"test-identity: S-Go contract check did not report the recorded contract:\n{r.stdout}{r.stderr}")

        # 5. lib compiled for another spec version.
        other = os.path.join(tmp, "lib-other-spec.a")
        if data.count(b"sgo1 0.1.0 ") != 2:
            sys.exit("test-identity: expected lib's identity line in its export data and its object")
        open(other, "wb").write(data.replace(b"sgo1 0.1.0 ", b"sgo1 0.9.9 "))
        r = compile_main(other, os.path.join(tmp, "main3.a"), expect_ok=False)
        if r.returncode == 0 or "SGO_E_SPEC_MISMATCH" not in r.stdout + r.stderr:
            sys.exit(f"test-identity: compiler accepted another spec version:\n{r.stdout}{r.stderr}")
        r = link(other, main_a)
        if r.returncode == 0 or "SGO_E_SPEC_MISMATCH" not in r.stdout + r.stderr:
            sys.exit(f"test-identity: linker accepted another spec version:\n{r.stdout}{r.stderr}")
    print("test-identity: ok: build metadata, stale contract (exported/unexported change, recorded contract), "
          "foreign export data and object, spec mismatch in export data and object")
    return 0


if __name__ == "__main__":
    sys.exit(main())
