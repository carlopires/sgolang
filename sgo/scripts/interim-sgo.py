#!/usr/bin/env python3
"""interim-sgo.py — INTERIM stand-in for the sgo driver (W03 layer 5 replaces it).

    interim-sgo.py build -o PROG     build the S-Go module in the current directory

It lets conformance.py run real programs before the driver exists:

    sgo/scripts/conformance.py run --sgo sgo/scripts/interim-sgo.py

How: the runtime lane's GOROOT (upstream tools at the locked commit, built by runtime-lane.sh)
gets the fork's TCB packages overlaid (as runtime-lane.sh does); the module's .sgo sources are
copied into a temporary Go module as .go files (module path from sgo.mod); the lane's go command
builds it with -toolexec pointing back at this script, which substitutes the fork's stage-0
compile, asm, and link. Diagnostics are mapped back to the .sgo file names.

Known gaps, all owned by the real driver: sgo.mod validation (spec version), rejection of non-.sgo
sources (.s, .go files), build flags and environment rules, source positions relative to the
module root in runtime records. It is not a product path and is not used by any target build.
"""
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))
SGO = os.path.join(ROOT, "sgo")
CACHE = os.environ.get("SGO_CACHE", os.path.expanduser("~/.cache/sgolang"))
LOCK = json.load(open(os.path.join(SGO, "UPSTREAM.lock.json")))
LANE = os.path.join(CACHE, "runtime-lane", LOCK["upstream"]["tag"])
PLAT = {"Darwin": "darwin", "Linux": "linux"}[os.uname().sysname] + "_" + \
    {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[os.uname().machine]
FORK_TOOLS = os.path.join(ROOT, "pkg", "tool", PLAT)


def toolexec(argv):
    tool, args = argv[0], argv[1:]
    name = os.path.basename(tool)
    if name in ("compile", "asm", "link"):
        if name == "compile" and "-goversion" in args:
            # stage-0 tools run on the bootstrap runtime, so their runtime.Version differs.
            i = args.index("-goversion")
            del args[i:i + 2]
        tool = os.path.join(FORK_TOOLS, name)
    os.execv(tool, [tool] + args)


def overlay():
    pkgs = json.load(open(os.path.join(SGO, "TARGET_TCB.json")))["packages"]
    for p in pkgs:
        src, dst = os.path.join(ROOT, "src", p) + "/", os.path.join(LANE, "src", p) + "/"
        subprocess.check_call(["rsync", "-a", "--delete", "--exclude", "testdata", src, dst])


def build(out):
    if not os.path.exists(os.path.join(LANE, "bin", "go")):
        sys.exit("interim-sgo: no runtime lane toolchain; run sgo/scripts/runtime-lane.sh once")
    if not os.path.exists(os.path.join(FORK_TOOLS, "compile")):
        sys.exit("interim-sgo: no stage-0 compiler; run sgo/scripts/build-stage0.sh")
    mod = "example.com/main"
    if os.path.exists("sgo.mod"):
        m = re.search(r"^module\s+(\S+)", open("sgo.mod").read(), re.M)
        if m:
            mod = m.group(1)
    overlay()
    with tempfile.TemporaryDirectory() as tmp:
        for root, _, files in os.walk("."):
            for f in files:
                if f.endswith(".sgo"):
                    rel = os.path.normpath(os.path.join(root, f))
                    dst = os.path.join(tmp, rel[:-4] + ".go")
                    os.makedirs(os.path.dirname(dst), exist_ok=True)
                    shutil.copyfile(rel, dst)
        open(os.path.join(tmp, "go.mod"), "w").write(f"module {mod}\n\ngo 1.27\n")
        env = {"HOME": os.environ.get("HOME", "/tmp"), "PATH": os.path.join(LANE, "bin") + ":/usr/bin:/bin",
               "GOROOT": LANE, "GOTOOLCHAIN": "local", "GOFLAGS": "-mod=mod", "GOWORK": "off",
               "GOCACHE": os.path.join(CACHE, "gocache", "interim-sgo"), "GOENV": "off",
               "CGO_ENABLED": "0", "LANG": "C"}
        r = subprocess.run([os.path.join(LANE, "bin", "go"), "build", "-toolexec", os.path.abspath(__file__) + " --toolexec",
                            "-o", os.path.abspath(out), "."], cwd=tmp, env=env, capture_output=True, text=True)
        text = re.sub(r"(?m)^\./", "", r.stdout + r.stderr)
        text = re.sub(r"(?m)^(\S+?)\.go:(\d+:\d+): ", r"\1.sgo:\2: ", text)
        sys.stderr.write(text)
        return r.returncode


def main():
    if len(sys.argv) > 1 and sys.argv[1] == "--toolexec":
        toolexec(sys.argv[2:])
    args = sys.argv[1:]
    if not args or args[0] != "build" or "-o" not in args:
        sys.exit("usage: interim-sgo.py build -o PROG")
    return build(args[args.index("-o") + 1])


if __name__ == "__main__":
    sys.exit(main())
