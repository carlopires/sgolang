#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""test-std-pem.py - std/encoding/pem against Go's encoding/pem (SGO-STD-032, D201).

Generates a seeded corpus of PEM inputs - valid blocks with and without headers, CRLF line
ends, long and short lines, leading and trailing text, a mismatched END line, and random
truncations and byte changes - and requires Go's encoding/pem and std/encoding/pem to decode
every input to the same blocks, rest lengths and EncodeToMemory output, line by line.

    test-std-pem.py [--seed N]
"""
import base64, json, os, platform, random, subprocess, sys, tempfile

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
VERSION = open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip()
GOOS = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
GOARCH = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[platform.machine()]
SGO = os.path.join(ROOT, "pkg", "tool", GOOS + "_" + GOARCH, "sgo")
PINNED = json.load(open(os.path.join(ROOT, "sgo", "UPSTREAM.lock.json")))["bootstrap"]["pinned_version"]
GO = os.path.join(os.environ.get("SGO_CACHE", os.path.expanduser("~/.cache/sgolang")), "bootstrap",
                  PINNED + "-" + GOOS + "-" + GOARCH, "go", "bin", "go")


def corpus(rnd):
    out = []
    def blk(t, data, headers=(), width=64, crlf=False, end=None):
        nl = "\r\n" if crlf else "\n"
        s = "-----BEGIN %s-----%s" % (t, nl)
        for k, v in headers:
            s += "%s: %s%s" % (k, v, nl)
        if headers:
            s += nl
        b = base64.b64encode(data).decode()
        for i in range(0, len(b), width):
            s += b[i:i + width] + nl
        s += "-----END %s-----%s" % (end or t, nl)
        return s
    types = ["CERTIFICATE", "RSA PRIVATE KEY", "PUBLIC KEY", "X", ""]
    for _ in range(120):
        data = os.urandom(rnd.randint(0, 200)) if rnd.random() < 0.5 else bytes(rnd.getrandbits(8) for _ in range(rnd.randint(0, 120)))
        t = rnd.choice(types)
        h = [] if rnd.random() < 0.6 else [("Proc-Type", "4,ENCRYPTED"), ("DEK-Info", "AES-128-CBC,0011"), ("Z", " spaced ")][:rnd.randint(1, 3)]
        s = blk(t, data, h, rnd.choice([64, 76, 16, 1000]), rnd.random() < 0.3, None if rnd.random() < 0.85 else "OTHER")
        if rnd.random() < 0.4:
            s = rnd.choice(["junk\n", "", "  \n", "-----BEGIN X-----\n"]) + s + rnd.choice(["", "trailer", "\n\n", blk("SECOND", b"abc")])
        out.append(s.encode())
    for i in range(len(out)):
        s = bytearray(out[i])
        r = rnd.random()
        if r < 0.15 and len(s) > 2:
            s = s[:rnd.randint(1, len(s) - 1)]
        elif r < 0.30 and len(s) > 2:
            s[rnd.randrange(len(s))] = rnd.choice(b" \t\n\r:-=A*")
        elif r < 0.40:
            s = s.replace(b"\n", b" \n", rnd.randint(1, 4))
        out.append(bytes(s))
    out += [b"", b"-----BEGIN A-----\n-----END A-----\n", b"-----BEGIN A-----\n-----END A-----", b"-----BEGIN A-----\nK: V\n-----END A-----\n",
            b"-----BEGIN A-----\n====\n-----END A-----\n", b"x-----BEGIN A-----\nAA==\n-----END A-----\n", b"-----BEGIN A-----\nAA==\n-----END A----- x\n"]
    return out


GO_SRC = """package main

import (
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"sort"
	"strings"
)

var inputs = []string{%s}

func main() {
	for i, h := range inputs {
		data, _ := hex.DecodeString(h)
		for {
			b, rest := pem.Decode(data)
			if b == nil {
				fmt.Printf("%%d nil rest=%%d\\n", i, len(rest))
				break
			}
			keys := []string{}
			for k := range b.Headers {
				keys = append(keys, k+"="+b.Headers[k])
			}
			sort.Strings(keys)
			fmt.Printf("%%d %%x [%%s] %%x rest=%%d\\n", i, []byte(b.Type), strings.Join(keys, ","), b.Bytes, len(rest))
			fmt.Printf("%%d enc %%x\\n", i, pem.EncodeToMemory(b))
			data = rest
		}
	}
}
"""

SGO_SRC = """package main

import (
	"std/encoding/pem"
	"sgo"
	"std/encoding/hex"
	"std/sort"
	"std/strconv"
	"std/strings"
)

var inputs = sgo.Freeze([]string{%s})

func main(root *sgo.Context) error {
	out := root.Stdout()
	for i := 0; i < len(inputs.Get()); i++ {
		data, _ := hex.DecodeString(inputs.Get()[i])
		n := strconv.Itoa(i)
		for {
			b, rest := pem.Decode(data)
			if b == nil {
				check out.WriteString(n + " nil rest=" + strconv.Itoa(len(rest)) + "\\n")
				break
			}
			keys := []string{}
			for k, v := range b.Headers {
				keys = append(keys, k+"="+v)
			}
			sort.Strings(keys)
			check out.WriteString(n + " " + hex.EncodeToString([]byte(b.Type)) + " [" + strings.Join(keys, ",") + "] " + hex.EncodeToString(b.Bytes) + " rest=" + strconv.Itoa(len(rest)) + "\\n")
			check out.WriteString(n + " enc " + hex.EncodeToString(pem.EncodeToMemory(b)) + "\\n")
			data = rest
		}
	}
	return nil
}
"""


def main():
    seed = 1
    if len(sys.argv) == 3 and sys.argv[1] == "--seed":
        seed = int(sys.argv[2])
    inputs = corpus(random.Random(seed))
    lits = ", ".join('"%s"' % x.hex() for x in inputs)
    with tempfile.TemporaryDirectory() as d:
        g = os.path.join(d, "g")
        os.makedirs(g)
        open(os.path.join(g, "main.go"), "w").write(GO_SRC % lits)
        open(os.path.join(g, "go.mod"), "w").write("module pemdiff\n\ngo 1.26\n")
        gr = subprocess.run([GO, "run", "."], cwd=g, capture_output=True, text=True, env=dict(os.environ, GOTOOLCHAIN="local"))
        s = os.path.join(d, "s")
        os.makedirs(s)
        open(os.path.join(s, "main.sgo"), "w").write(SGO_SRC % lits)
        open(os.path.join(s, "sgo.mod"), "w").write("module example.com/pemdiff\n\nsgo %s\n" % VERSION)
        sr = subprocess.run([SGO, "run", "."], cwd=s, capture_output=True, text=True)
    if gr.returncode or sr.returncode:
        sys.exit("test-std-pem: go:\n" + gr.stderr[-2000:] + "\ns-go:\n" + sr.stdout[-1000:] + sr.stderr[-2000:])
    gl, sl = gr.stdout.splitlines(), sr.stdout.splitlines()
    bad = [(a, b) for a, b in zip(gl, sl) if a != b]
    n = len(bad) + abs(len(gl) - len(sl))
    print("test-std-pem: seed %d, %d inputs, %d lines, %d differ" % (seed, len(inputs), len(gl), n))
    for a, b in bad[:6]:
        print("  go:   " + a[:200] + "\n  s-go: " + b[:200])
    sys.exit(1 if n else 0)


main()
