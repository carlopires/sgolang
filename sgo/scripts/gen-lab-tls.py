#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""gen-lab-tls.py - record the lab TLS replay fixture (D202, SGO-LAB-004 v2, SGO-STD-031 v2).

The fixture sgo/testdata/conformance/lab-tls-replay is an S-Go program that runs a TLS 1.3
handshake on the lab's loopback network: the client is std/crypto/tls with the lab's seeded
Entropy; the server task replays an exchange with Go's crypto/tls server and checks each client
flight against it, byte for byte. Because Entropy is seeded, the client's bytes for seed 1 are
fixed, so the exchange is recorded in steps, each run in the lab:

  1. the program, with an empty recording, prints the ClientHello;
  2. sgo/scripts/labtls/recorder.go (Go's server, made deterministic: seeded Rand used for its
     key share by GODEBUG=cryptocustomrand=1, fixed time, Ed25519 certificates, no tickets)
     answers it with its flight;
  3. the program replays that flight and prints the client's Finished flight;
  4. the recorder, given both client flights, finishes the handshake and writes its data.

The filled program is the fixture. With --check, the recording is redone and compared with the
committed fixture (it must be identical: both sides are deterministic).

    gen-lab-tls.py [--check] [--sgo PATH]
"""

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
VERSION = open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip()
TMPL = os.path.join(ROOT, "sgo", "scripts", "labtls", "main.sgo.tmpl")
RECORDER = os.path.join(ROOT, "sgo", "scripts", "labtls", "recorder.go")
CASE = os.environ.get("LAB_TLS_OUT") or os.path.join(ROOT, "sgo", "testdata", "conformance", "lab-tls-replay")
SEED = "1"
SGOMOD = ("module example.com/lab-tls-replay\n\nsgo %s\ngrant listen 127.0.0.1:47995\n"
          "grant connect 127.0.0.1:47995\ngrant entropy\n")


def go():
    cache = os.environ.get("SGO_CACHE", os.path.expanduser("~/.cache/sgolang"))
    pinned = json.load(open(os.path.join(ROOT, "sgo", "UPSTREAM.lock.json")))["bootstrap"]["pinned_version"]
    goos = {"darwin": "darwin", "linux": "linux"}[sys.platform]
    goarch = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[os.uname().machine]
    return os.path.join(cache, "bootstrap", f"{pinned}-{goos}-{goarch}", "go", "bin", "go")


def fill(ca, ch="", sf="", fin="", app=""):
    s = open(TMPL).read()
    for k, v in (("@CA@", ca), ("@CH@", ch), ("@SF@", sf), ("@FIN@", fin), ("@APP@", app)):
        assert s.count(k) == 1, k
        s = s.replace(k, v)
    return s


def lab_run(sgo, src, work):
    d = os.path.join(work, "case")
    shutil.rmtree(d, ignore_errors=True)
    os.makedirs(d)
    open(os.path.join(d, "main.sgo"), "w").write(src)
    open(os.path.join(d, "sgo.mod"), "w").write(SGOMOD % VERSION)
    r = subprocess.run([sgo, "lab", "run", "-seed", SEED, "-trace", "t.jsonl", "."], cwd=d,
                       capture_output=True, text=True, timeout=600)
    return r


def recorded(r, what):
    m = re.search(r"^record %s ([0-9a-f]+)$" % what, r.stdout, re.M)
    if not m:
        sys.exit(f"gen-lab-tls: step '{what}': exit {r.returncode}, stdout {r.stdout!r}, stderr {r.stderr[-600:]!r}")
    return m.group(1)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--check", action="store_true")
    ap.add_argument("--sgo", default=os.path.join(ROOT, "pkg", "tool", "%s_%s" % (
        {"darwin": "darwin", "linux": "linux"}[sys.platform],
        {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[os.uname().machine]), "sgo"))
    a = ap.parse_args()
    with tempfile.TemporaryDirectory() as work:
        rec = os.path.join(work, "recorder")
        b = subprocess.run([go(), "build", "-o", rec, RECORDER], cwd=work, capture_output=True, text=True,
                           env=dict(os.environ, GOTOOLCHAIN="local", GOFLAGS=""))
        if b.returncode:
            sys.exit("gen-lab-tls: building the recorder failed:\n" + b.stderr)

        def recorder(*args):
            # Go 1.26 ignores a caller's random source in key generation unless asked; the
            # recorder needs its seeded Rand used, so the server's key share repeats.
            r = subprocess.run([rec] + list(args), capture_output=True, text=True, timeout=120,
                               env=dict(os.environ, GODEBUG="cryptocustomrand=1"))
            if r.returncode:
                sys.exit("gen-lab-tls: recorder: " + r.stderr)
            return r.stdout

        ca = recorder("-ca")
        ch = recorded(lab_run(a.sgo, fill(ca), work), "ch")
        sf = recorder("-ch", ch).split()[0]
        fin = recorded(lab_run(a.sgo, fill(ca, ch, sf), work), "fin")
        sf2, app = recorder("-ch", ch, "-fin", fin).split()
        if sf2 != sf:
            sys.exit("gen-lab-tls: the Go server's flight is not deterministic")
        src = fill(ca, ch, sf, fin, app)
        r = lab_run(a.sgo, src, work)
        want = "handshake TLS_AES_128_GCM_SHA256, read hello from go\n"
        if r.returncode != 0 or r.stdout != want:
            sys.exit(f"gen-lab-tls: replay: exit {r.returncode}, stdout {r.stdout!r}, stderr {r.stderr[-600:]!r}")
        mod = SGOMOD % VERSION
        if a.check:
            have = open(os.path.join(CASE, "main.sgo")).read()
            if have != src:
                sys.exit("gen-lab-tls: the committed fixture differs from a fresh recording")
            print(f"gen-lab-tls: ok: fresh recording matches the fixture ({len(ch)//2}+{len(fin)//2} client bytes, "
                  f"{len(sf)//2}+{len(app)//2} server bytes)")
            return
        os.makedirs(CASE, exist_ok=True)
        open(os.path.join(CASE, "main.sgo"), "w").write(src)
        open(os.path.join(CASE, "sgo.mod"), "w").write(mod)
        print(f"gen-lab-tls: wrote {os.path.relpath(CASE, ROOT)}: client {len(ch)//2}+{len(fin)//2} bytes, "
              f"server {len(sf)//2}+{len(app)//2} bytes")


if __name__ == "__main__":
    main()
