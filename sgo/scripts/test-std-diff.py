#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""test-std-diff.py - differential test of the standard library against Go's (D125, SGO-STD-*).

Generates seeded inputs (edge cases, and random strings including invalid UTF-8), writes two
programs from one list of test snippets - a Go program calling Go's packages, and an S-Go program
calling std/ - builds and runs both, and requires their outputs to be identical line by line.
Each snippet prints one line. Documented deviations are encoded on the Go side: library error
messages quote their input with QuoteToASCII (SGO-STD-004), ParseInt has no bitSize, and the case
functions are ASCII-only (D122).

    test-std-diff.py [--seed N] [--strings N]
"""

import os
import random
import shutil
import subprocess
import sys
import tempfile

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
sys.path.insert(0, os.path.join(ROOT, "sgo", "scripts"))
VERSION = open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip()


def tool_paths():
    import platform
    goos = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
    goarch = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[platform.machine()]
    sgo = os.path.join(ROOT, "pkg", "tool", f"{goos}_{goarch}", "sgo")
    import json
    cache = os.environ.get("SGO_CACHE", os.path.expanduser("~/.cache/sgolang"))
    pinned = json.load(open(os.path.join(ROOT, "sgo", "UPSTREAM.lock.json")))["bootstrap"]["pinned_version"]
    # The pinned bootstrap toolchain (sgo/scripts/lib.sh), never a Go found on PATH.
    go = os.path.join(cache, "bootstrap", f"{pinned}-{goos}-{goarch}", "go", "bin", "go")
    return sgo, go


def lit(b):
    """A string literal valid in both Go and S-Go for the bytes b."""
    out = ['"']
    for c in b:
        if c == 0x22:
            out.append('\\"')
        elif c == 0x5C:
            out.append("\\\\")
        elif 0x20 <= c < 0x7F:
            out.append(chr(c))
        else:
            out.append("\\x%02x" % c)
    out.append('"')
    return "".join(out)


EDGE = [b"", b" ", b"a", b"ab", b"abc", b"aaa", b",", b"a,b,,c", b"0", b"-0", b"+5", b"0x1F", b"0X_1f",
        b"0b101", b"0o17", b"017", b"1_000", b"_1", b"1__0", b"9223372036854775807", b"-9223372036854775808",
        b"9223372036854775808", b"-9223372036854775809", b"18446744073709551615", b"18446744073709551616",
        b"9223372036854775808x", b"99999999999999999999x", b"zz", b"Zz", b"true", b"True", b"tRUE", b"1", b"F",
        "h\u00e9llo".encode(), "\u00e9\u00e9a\u00e9".encode(), b"\xff", b"a\xffb", b"\xe2\x82", b"\xe2\x82\xac",
        b"\xed\xa0\x80", b"\xf4\x90\x80\x80", b"\xc0\xaf", "\U0001F600!".encode(), b" \t\n x \r", "\u3000y\u2028".encode(),
        b"\xc2\x85z\xc2\xa0", b'"q"', b"'a'", b"'\\''", b'"\\x41\\u00e9\\n"', b'"\\777"', b"`raw\r`", b"'ab'", b"''",
        b'"\\uD800"', b'"\\U00110000"', b'"a\nb"', b'"\\q"', b"\"\xff\"", b"'\xff'", b"\x00\x01\x7f"]


def random_strings(rng, n):
    alphabet = [b"a", b"b", b",", b" ", b"0", b"1", b"x", b"_", b"-", "\u00e9".encode(), "\u3000".encode(),
                "\U0001F600".encode(), b"\xff", b"\xe2", b"\x82", b"\t", b'"', b"\\"]
    out = []
    for _ in range(n):
        k = rng.randint(0, 8)
        out.append(b"".join(rng.choice(alphabet) for _ in range(k)))
    return out


def snippets(rng, strs):
    """Pairs (go, sgo) of statement snippets; each prints exactly one line through ln."""
    T = []

    def both(code):
        T.append((code, code))

    for s in strs:
        a = lit(s)
        both(f"r, n := utf8.DecodeRuneInString({a}); ln(I(int(r)) + \",\" + I(n))")
        both(f"r, n := utf8.DecodeLastRuneInString({a}); ln(I(int(r)) + \",\" + I(n))")
        both(f"ln(I(utf8.RuneCountInString({a})) + B(utf8.ValidString({a})) + B(utf8.FullRuneInString({a})))")
        both(f"ln(Q(strings.TrimSpace({a})) + QL(strings.Fields({a})))")
        T.append((f"ln(Q(goUpper({a})) + Q(goLower({a})))", f"ln(Q(strings.ToUpperASCII({a})) + Q(strings.ToLowerASCII({a})))"))
        both(f"ln(Q(strconv.QuoteToASCII({a})))")
        both(f"v, err := strconv.Unquote({a}); ln(Q(v) + E(err))")
        both(f"v, err := strconv.Unquote(strconv.QuoteToASCII({a})); ln(Q(v) + E(err))")
        both(f"v, err := strconv.ParseBool({a}); ln(B(v) + E(err))")
        both(f"v, err := strconv.Atoi({a}); ln(I(v) + E(err))")
        for base in (0, 2, 8, 10, 16, 36, 1, 37):
            T.append((f"v, err := strconv.ParseInt({a}, {base}, 64); ln(I(int(v)) + E(err))",
                      f"v, err := strconv.ParseInt({a}, {base}); ln(I(v) + E(err))"))
        both(f"b := bytes.TrimSpace([]byte({a})); ln(B(b == nil) + Q(string(b)))")
        both(f"ln(QBL(bytes.Split([]byte({a}), []byte(\",\"))) + QBL(bytes.Split([]byte({a}), []byte(\"\"))))")
    pairs = []
    for _ in range(len(strs) * 2):
        s = rng.choice(strs)
        if s and rng.random() < 0.5:
            i = rng.randint(0, len(s))
            j = rng.randint(i, len(s))
            t = s[i:j]
        else:
            t = rng.choice(strs)
        pairs.append((s, t))
    for s, t in pairs:
        a, b = lit(s), lit(t)
        both(f"ln(I(strings.Index({a}, {b})) + I(strings.LastIndex({a}, {b})) + I(strings.Count({a}, {b})))")
        both(f"ln(B(strings.Contains({a}, {b})) + B(strings.HasPrefix({a}, {b})) + B(strings.HasSuffix({a}, {b})))")
        for n in (-1, 0, 1, 2, 3):
            both(f"ln(QL(strings.SplitN({a}, {b}, {n})))")
        for n in (-1, 1, 2):
            both(f"ln(Q(strings.Replace({a}, {b}, \"<>\", {n})))")
        both(f"ln(Q(strings.Trim({a}, {b})) + Q(strings.TrimLeft({a}, {b})) + Q(strings.TrimRight({a}, {b})))")
        both(f"x, y, ok := strings.Cut({a}, {b}); ln(Q(x) + Q(y) + B(ok))")
        both(f"ln(Q(strings.Join(strings.Split({a}, {b}), \"|\")))")
        T.append((f"ln(B(goEqualFold({a}, {b})))", f"ln(B(strings.EqualFoldASCII({a}, {b})))"))
        both(f"ln(B(bytes.Equal([]byte({a}), []byte({b}))) + I(bytes.Compare([]byte({a}), []byte({b}))) + I(bytes.Index([]byte({a}), []byte({b}))) + I(bytes.Count([]byte({a}), []byte({b}))))")
        r = rng.choice([0x61, 0x2C, 0xE9, 0xFFFD, 0x1F600, 0xD800])
        both(f"ln(I(strings.IndexRune({a}, {r})))")
    for _ in range(200):
        v = rng.choice([0, 1, -1, 2**63 - 1, -2**63, rng.randint(-2**63, 2**63 - 1), rng.randint(-1000, 1000)])
        base = rng.randint(2, 36)
        vl = "(-9223372036854775807 - 1)" if v == -2**63 else str(v)
        T.append((f"ln(strconv.FormatInt(int64({vl}), {base}) + strconv.Itoa({vl}))",
                  f"ln(strconv.FormatInt({vl}, {base}) + strconv.Itoa({vl}))"))
    for _ in range(60):
        n = rng.randint(0, 40)
        xs = [rng.randint(-5, 5) for _ in range(n)]
        ss = [lit(rng.choice(strs)) for _ in range(n)]
        il = "[]int{" + ", ".join(map(str, xs)) + "}"
        sl = "[]string{" + ", ".join(ss) + "}"
        both(f"x := {il}; sort.Ints(x); y := {sl}; sort.Strings(y); ln(IL(x) + QL(y) + I(sort.SearchInts(x, 1)) + I(sort.SearchStrings(y, \"b\")))")
        k = rng.randint(1, 5)
        T.append((f"ln(IL(goPerm({n}, {k})))", f"ln(IL(sort.Perm({n}, func(i, j int) bool {{ return i%{k} < j%{k} }})))"))
    return T


GO_PRELUDE = r'''package main

import (
	"bytes"
	"errors"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

var out strings.Builder

func ln(s string)        { out.WriteString(s); out.WriteByte('\n') }
func I(v int) string     { return strconv.Itoa(v) + ";" }
func B(v bool) string    { return strconv.FormatBool(v) + ";" }
func Q(s string) string  { return strconv.QuoteToASCII(s) + ";" }
func QL(a []string) string {
	if a == nil {
		return "nil;"
	}
	r := "["
	for _, s := range a {
		r += strconv.QuoteToASCII(s) + " "
	}
	return r + "];"
}
func QBL(a [][]byte) string {
	r := "["
	for _, s := range a {
		r += strconv.QuoteToASCII(string(s)) + strconv.Itoa(cap(s)) + " "
	}
	return r + "]" + strconv.Itoa(cap(a)) + ";"
}
func IL(a []int) string {
	r := "["
	for _, v := range a {
		r += strconv.Itoa(v) + " "
	}
	return r + "];"
}
func E(err error) string {
	if err == nil {
		return "nil;"
	}
	var ne *strconv.NumError
	if errors.As(err, &ne) {
		return "strconv." + ne.Func + ": parsing " + strconv.QuoteToASCII(ne.Num) + ": " + ne.Err.Error() + ";"
	}
	return err.Error() + ";"
}
func goUpper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'a' <= c && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}
func goLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
func goEqualFold(s, t string) bool { return goLower(s) == goLower(t) }
func goPerm(n, k int) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	sort.SliceStable(p, func(a, b int) bool { return p[a]%k < p[b]%k })
	return p
}

var _ = utf8.RuneError
var _ = bytes.Equal
'''

SGO_PRELUDE = r'''package main

import (
	"sgo"
	"std/bytes"
	"std/sort"
	"std/strconv"
	"std/strings"
	"std/utf8"
)

type outBuf struct {
	b strings.Builder
}

func (o *outBuf) ln(s string) {
	_, _ = o.b.WriteString(s)
	_ = o.b.WriteByte('\n')
}
func I(v int) string    { return strconv.Itoa(v) + ";" }
func B(v bool) string   { return strconv.FormatBool(v) + ";" }
func Q(s string) string { return strconv.QuoteToASCII(s) + ";" }
func QL(a []string) string {
	if a == nil {
		return "nil;"
	}
	r := "["
	for _, s := range a {
		r += strconv.QuoteToASCII(s) + " "
	}
	return r + "];"
}
func QBL(a [][]byte) string {
	r := "["
	for _, s := range a {
		r += strconv.QuoteToASCII(string(s)) + strconv.Itoa(cap(s)) + " "
	}
	return r + "]" + strconv.Itoa(cap(a)) + ";"
}
func IL(a []int) string {
	r := "["
	for _, v := range a {
		r += strconv.Itoa(v) + " "
	}
	return r + "];"
}
func E(err error) string {
	if err == nil {
		return "nil;"
	}
	return err.Error() + ";"
}
'''


def build_programs(T, chunk=150):
    go_funcs, sgo_funcs = [], []
    for k in range(0, len(T), chunk):
        part = T[k:k + chunk]
        go_funcs.append(f"func part{k}() {{\n" + "".join(f"\t{{\n\t\t{g}\n\t}}\n" for g, _ in part) + "}\n")
        body = "".join(f"\t{{\n\t\t{s.replace('ln(', 'o.ln(')}\n\t}}\n" for _, s in part)
        sgo_funcs.append(f"func part{k}(o *outBuf) {{\n{body}}}\n")
    go = GO_PRELUDE + "\n".join(go_funcs) + "\nfunc main() {\n" + "".join(f"\tpart{k}()\n" for k in range(0, len(T), chunk)) + "\tos.Stdout.WriteString(out.String())\n}\n"
    sgo = SGO_PRELUDE + "\n".join(sgo_funcs) + "\nfunc main(root *sgo.Context) error {\n\to := &outBuf{}\n" + "".join(f"\tpart{k}(o)\n" for k in range(0, len(T), chunk)) + "\treturn root.Stdout().WriteString(o.b.String())\n}\n"
    return go, sgo


def main():
    seed, nstr = 1, 120
    args = sys.argv[1:]
    while args:
        a = args.pop(0)
        if a == "--seed":
            seed = int(args.pop(0))
        elif a == "--strings":
            nstr = int(args.pop(0))
        else:
            sys.exit(__doc__)
    rng = random.Random(seed)
    strs = EDGE + random_strings(rng, nstr)
    T = snippets(rng, strs)
    go_src, sgo_src = build_programs(T)
    sgo, go = tool_paths()
    tmp = tempfile.mkdtemp(prefix="sgo-std-diff-")
    try:
        gdir, sdir = os.path.join(tmp, "go"), os.path.join(tmp, "sgo")
        os.makedirs(gdir)
        os.makedirs(sdir)
        open(os.path.join(gdir, "main.go"), "w").write(go_src)
        open(os.path.join(gdir, "go.mod"), "w").write("module stddiff\n\ngo 1.26\n")
        open(os.path.join(sdir, "main.sgo"), "w").write(sgo_src)
        open(os.path.join(sdir, "sgo.mod"), "w").write(f"module example.com/stddiff\n\nsgo {VERSION}\n")
        env = dict(os.environ, GOTOOLCHAIN="local", GOFLAGS="", GO111MODULE="on")
        g = subprocess.run([go, "run", "."], cwd=gdir, env=env, capture_output=True, text=True, timeout=600)
        if g.returncode != 0:
            sys.exit("test-std-diff: Go program failed:\n" + g.stderr[-3000:])
        b = subprocess.run([sgo, "build", "-o", "p", "."], cwd=sdir, capture_output=True, text=True, timeout=900)
        if b.returncode != 0:
            sys.exit("test-std-diff: S-Go build failed:\n" + b.stderr[-3000:])
        s = subprocess.run([os.path.join(sdir, "p")], cwd=sdir, capture_output=True, text=True, timeout=600)
        if s.returncode != 0:
            sys.exit("test-std-diff: S-Go program failed:\n" + s.stderr[-3000:])
        gl, sl = g.stdout.split("\n"), s.stdout.split("\n")
        bad = 0
        for i, (x, y) in enumerate(zip(gl, sl)):
            if x != y:
                bad += 1
                if bad <= 10:
                    print(f"test-std-diff: line {i}: {T[i][1]}\n  go:  {x}\n  sgo: {y}")
        if len(gl) != len(sl):
            bad += 1
            print(f"test-std-diff: {len(gl)} Go lines, {len(sl)} S-Go lines")
        if bad:
            sys.exit(f"test-std-diff: FAIL: {bad} differing line(s) of {len(T)} (seed {seed})")
        print(f"test-std-diff: ok: {len(T)} checks identical to Go's packages (seed {seed}, {len(strs)} strings)")
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    main()
