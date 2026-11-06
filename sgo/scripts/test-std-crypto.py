#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""test-std-crypto.py - std/bits, std/encoding and std/crypto against Go and Wycheproof (D197).

Differential part: for std/bits, std/encoding/{hex,base64,binary}, std/crypto/sha256 and
std/crypto/sha512, writes a Go program and an S-Go program over the same seeded inputs and
requires identical output line by line (SGO-STD-011..016).

Wycheproof part: runs the vectors that Go's own crypto tests use - github.com/c2sp/wycheproof at
the version pinned in src/crypto/internal/cryptotest/wycheproof/schemaversion.go, fetched with
the pinned bootstrap toolchain (`go mod download`, verified by the checksum database) - against
std/crypto/hmac, hkdf, chacha20poly1305 and curve25519 (SGO-STD-018, 019, 021, 022).

    test-std-crypto.py [--seed N]
"""

import json
import os
import random
import re
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
    cache = os.environ.get("SGO_CACHE", os.path.expanduser("~/.cache/sgolang"))
    pinned = json.load(open(os.path.join(ROOT, "sgo", "UPSTREAM.lock.json")))["bootstrap"]["pinned_version"]
    go = os.path.join(cache, "bootstrap", f"{pinned}-{goos}-{goarch}", "go", "bin", "go")
    return sgo, go


SGO, GO = tool_paths()


def wycheproof_dir():
    text = open(os.path.join(ROOT, "src/crypto/internal/cryptotest/wycheproof/schemaversion.go")).read()
    version = re.search(r'wycheproofVersion = "([^"]+)"', text).group(1)
    with tempfile.TemporaryDirectory() as d:
        open(os.path.join(d, "go.mod"), "w").write("module wpfetch\n\ngo 1.26\n")
        r = subprocess.run([GO, "mod", "download", "-json", "github.com/c2sp/wycheproof@" + version],
                           cwd=d, capture_output=True, text=True, timeout=600)
    if r.returncode:
        sys.exit("test-std-crypto: fetching Wycheproof failed:\n" + r.stdout[-800:] + r.stderr[-800:])
    return os.path.join(json.loads(r.stdout)["Dir"], "testvectors_v1")


def run_sgo(src):
    with tempfile.TemporaryDirectory() as d:
        open(os.path.join(d, "main.sgo"), "w").write(src)
        open(os.path.join(d, "sgo.mod"), "w").write(f"module example.com/stdcrypto\n\nsgo {VERSION}\n")
        return subprocess.run([SGO, "run", "."], cwd=d, capture_output=True, text=True, timeout=900)


def run_go(src):
    with tempfile.TemporaryDirectory() as d:
        open(os.path.join(d, "main.go"), "w").write(src)
        open(os.path.join(d, "go.mod"), "w").write("module stdcrypto\n\ngo 1.26\n")
        env = dict(os.environ, GOTOOLCHAIN="local", GOFLAGS="-mod=mod")
        return subprocess.run([GO, "run", "."], cwd=d, env=env, capture_output=True, text=True, timeout=900)


def gen_bits(rnd):
    pass
    edge = [0, 1, 2, 3, 0x7f, 0x80, 0xff, 0x7fffffff, 0x80000000, 0xfffffffe, 0xffffffff, 0x100000000,
            0x7fffffffffffffff, 0x8000000000000000, 0xfffffffffffffffe, 0xffffffffffffffff]
    vals = edge + [rnd.getrandbits(rnd.choice([8, 16, 31, 32, 33, 63, 64])) for _ in range(48)]
    v32 = sorted(set(v & 0xffffffff for v in vals))
    v64 = sorted(set(vals))
    ks = [-65, -64, -33, -32, -31, -1, 0, 1, 7, 31, 32, 33, 63, 64, 65]
    def lit(xs): return ', '.join(hex(x) for x in xs)
    body = '''
    	for _, x := range v32 {
    		for _, y := range v32 {
    			for _, c := range c01_32 {
    				s, co := bits.Add32(x, y, c)
    				d, bo := bits.Sub32(x, y, c)
    				out = append(out, u(s) + " " + u(co) + " " + u(d) + " " + u(bo))
    			}
    			h, l := bits.Mul32(x, y)
    			out = append(out, u(h) + " " + u(l))
    			if y != 0 && y > x {
    				q, r := bits.Div32(x, y-1, y)
    				out = append(out, u(q) + " " + u(r))
    			}
    		}
    		out = append(out, itoa(bits.Len32(x)) + " " + itoa(bits.LeadingZeros32(x)) + " " + itoa(bits.TrailingZeros32(x)) + " " + itoa(bits.OnesCount32(x)) + " " + u(bits.ReverseBytes32(x)))
    		for _, k := range ks {
    			out = append(out, u(bits.RotateLeft32(x, k)))
    		}
    	}
    	for _, x := range v64 {
    		for _, y := range v64 {
    			for _, c := range c01_64 {
    				s, co := bits.Add64(x, y, c)
    				d, bo := bits.Sub64(x, y, c)
    				out = append(out, w(s) + " " + w(co) + " " + w(d) + " " + w(bo))
    			}
    			h, l := bits.Mul64(x, y)
    			out = append(out, w(h) + " " + w(l))
    			if y > x {
    				q, r := bits.Div64(x, l, y)
    				out = append(out, w(q) + " " + w(r))
    			}
    		}
    		out = append(out, itoa(bits.Len64(x)) + " " + itoa(bits.LeadingZeros64(x)) + " " + itoa(bits.TrailingZeros64(x)) + " " + itoa(bits.OnesCount64(x)) + " " + w(bits.ReverseBytes64(x)))
    		for _, k := range ks {
    			out = append(out, w(bits.RotateLeft64(x, k)))
    		}
    	}
    '''
    tables = '\tv32 := []uint32{%s}\n\tv64 := []uint64{%s}\n\tks := []int{%s}\n\tc01_32 := []uint32{0, 1}\n\tc01_64 := []uint64{0, 1}\n' % (lit(v32), lit(v64), ', '.join(map(str, ks)))
    gosrc = '''package main

    import (
    	"fmt"
    	"math/bits"
    	"os"
    	"strconv"
    	"strings"
    )

    func u(x uint32) string { return strconv.FormatUint(uint64(x), 16) }
    func w(x uint64) string { return strconv.FormatUint(x, 16) }
    func itoa(i int) string { return strconv.Itoa(i) }

    func main() {
    	out := []string{}
    ''' + tables + body + '''	fmt.Fprint(os.Stdout, strings.Join(out, "\\n")+"\\n")
    }
    '''
    sgosrc = '''package main

    import (
    	"sgo"
    	"std/bits"
    	"std/strconv"
    	"std/strings"
    )

    func hex(x uint64) string {
    	if x == 0 {
    		return "0"
    	}
    	digits := "0123456789abcdef"
    	s := ""
    	for x != 0 {
    		s = digits[int(x&15):int(x&15)+1] + s
    		x = x >> 4
    	}
    	return s
    }

    func u(x uint32) string { return hex(uint64(x)) }
    func w(x uint64) string { return hex(x) }
    func itoa(i int) string { return strconv.Itoa(i) }

    func main(root *sgo.Context) error {
    	out := []string{}
    ''' + tables + body + '''	return root.Stdout().WriteString(strings.Join(out, "\\n") + "\\n")
    }
    '''
    return gosrc, sgosrc


def gen_encoding(rnd):
    pass
    B64 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/-_='
    strs = ['', 'f', 'fo', 'foo', 'foob', 'fooba', 'foobar', 'Zm9vYmFy', 'Zm9vYg==', 'Zm9vYg=', 'Zm9vYg', 'Zm9v\nYmFy', 'Zm9v\r\nYg==', '====', 'Z===', 'Zm==\n', 'Zm==x', 'abcdef', 'ABCDEF0123', 'abc', 'zz', '0x1f']
    for _ in range(900):
        n = rnd.randrange(0, 40)
        kind = rnd.random()
        if kind < 0.4:
            s = ''.join(rnd.choice(B64) for _ in range(n))
        elif kind < 0.7:
            s = ''.join(rnd.choice('0123456789abcdefABCDEFgG ') for _ in range(n))
        else:
            s = ''.join(chr(rnd.randrange(0, 128)) for _ in range(n))
        strs.append(s)
    def golit(s): return '"' + ''.join('\\x%02x' % ord(c) for c in s) + '"'
    lits = ', '.join(golit(s) for s in strs)
    body = '''
    	for _, s := range strs {
    		b := []byte(s)
    		hb, herr := hex.DecodeString(s)
    		out = append(out, "hex "+hex.EncodeToString(b)+" "+hex.EncodeToString(hb)+" "+errText(herr))
    		for _, enc := range encs {
    			db, derr := enc.DecodeString(s)
    			out = append(out, "b64 "+enc.EncodeToString(b)+" "+hex.EncodeToString(db)+" "+errText(derr)+" "+itoa(enc.EncodedLen(len(b)))+" "+itoa(enc.DecodedLen(len(s))))
    		}
    		if len(b) >= 8 {
    			out = append(out, "bin "+u64(binary.BigEndian.Uint64(b))+" "+u64(binary.LittleEndian.Uint64(b))+" "+u64(uint64(binary.BigEndian.Uint32(b)))+" "+u64(uint64(binary.LittleEndian.Uint32(b))))
    			p := make([]byte, 8)
    			binary.LittleEndian.PutUint64(p, binary.BigEndian.Uint64(b))
    			q := binary.BigEndian.AppendUint64([]byte{}, binary.LittleEndian.Uint64(b))
    			r := binary.LittleEndian.AppendUint32([]byte{}, binary.BigEndian.Uint32(b))
    			binary.BigEndian.PutUint32(p[0:4], binary.LittleEndian.Uint32(b))
    			out = append(out, "put "+hex.EncodeToString(p)+" "+hex.EncodeToString(q)+" "+hex.EncodeToString(r))
    		}
    	}
    '''
    gosrc = '''package main

    import (
    	"encoding/base64"
    	"encoding/binary"
    	"encoding/hex"
    	"fmt"
    	"strconv"
    	"strings"
    )

    func itoa(i int) string { return strconv.Itoa(i) }
    func u64(x uint64) string { return strconv.FormatUint(x, 16) }
    func errText(e error) string {
    	if e == nil {
    		return "nil"
    	}
    	return e.Error()
    }

    func main() {
    	out := []string{}
    	strs := []string{%s}
    	encs := []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.RawURLEncoding}
    %s	fmt.Print(strings.Join(out, "\\n") + "\\n")
    }
    ''' % (lits, body)
    sgosrc = '''package main

    import (
    	"sgo"
    	"std/encoding/base64"
    	"std/encoding/binary"
    	"std/encoding/hex"
    	"std/strconv"
    	"std/strings"
    )

    func itoa(i int) string { return strconv.Itoa(i) }

    func u64(x uint64) string {
    	if x == 0 {
    		return "0"
    	}
    	digits := "0123456789abcdef"
    	s := ""
    	for x != 0 {
    		s = digits[int(x&15):int(x&15)+1] + s
    		x = x >> 4
    	}
    	return s
    }

    func errText(e error) string {
    	if e == nil {
    		return "nil"
    	}
    	return e.Error()
    }

    func main(root *sgo.Context) error {
    	out := []string{}
    	strs := []string{%s}
    	encs := []base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.RawURLEncoding}
    %s	return root.Stdout().WriteString(strings.Join(out, "\\n") + "\\n")
    }
    ''' % (lits, body)
    return gosrc, sgosrc


def gen_sha256(rnd):
    pass
    msgs = [bytes(rnd.randrange(256) for _ in range(n)) for n in list(range(0, 130)) + [rnd.randrange(130, 3000) for _ in range(130)]]
    lit = ', '.join('{' + ', '.join(str(b) for b in m) + '}' for m in msgs)
    chunks = [1, 3, 63, 64, 65, 1000]
    body = '''
    	for _, m := range msgs {
    		a := %s.Sum256(m)
    		b := %s.Sum224(m)
    		out = append(out, hx(a[:])+" "+hx(b[:]))
    		for _, c := range chunks {
    			d := %s.New()
    			for i := 0; i < len(m); i = i + c {
    				j := i + c
    				if j > len(m) {
    					j = len(m)
    				}
    				_, _ = d.Write(m[i:j])
    			}
    			s1 := d.Sum([]byte{})
    			_, _ = d.Write(m)
    			s2 := d.Sum([]byte{7})
    			out = append(out, hx(s1)+" "+hx(s2))
    		}
    	}
    '''
    gosrc = '''package main

    import (
    	"crypto/sha256"
    	"encoding/hex"
    	"fmt"
    	"strings"
    )

    func hx(b []byte) string { return hex.EncodeToString(b) }

    func main() {
    	out := []string{}
    	msgs := [][]byte{%s}
    	chunks := []int{%s}
    %s	fmt.Print(strings.Join(out, "\\n") + "\\n")
    }
    ''' % (lit, ', '.join(map(str, chunks)), body % ('sha256', 'sha256', 'sha256'))
    sgosrc = '''package main

    import (
    	"std/crypto/sha256"
    	"sgo"
    	"std/encoding/hex"
    	"std/strings"
    )

    func hx(b []byte) string { return hex.EncodeToString(b) }

    func main(root *sgo.Context) error {
    	out := []string{}
    	msgs := [][]byte{%s}
    	chunks := []int{%s}
    %s	return root.Stdout().WriteString(strings.Join(out, "\\n") + "\\n")
    }
    ''' % (lit, ', '.join(map(str, chunks)), body % ('sha256', 'sha256', 'sha256'))
    return gosrc, sgosrc


def gen_sha512(rnd):
    pass
    msgs = [bytes(rnd.randrange(256) for _ in range(n)) for n in list(range(0, 258)) + [rnd.randrange(258, 3000) for _ in range(60)]]
    lit = ', '.join('{' + ', '.join(str(b) for b in m) + '}' for m in msgs)
    chunks = [1, 7, 127, 128, 129, 1000]
    body = '''
    	for _, m := range msgs {
    		a := %s.Sum512(m)
    		b := %s.Sum384(m)
    		c2 := sha512.Sum512_224(m)
    		c3 := sha512.Sum512_256(m)
    		out = append(out, hx(a[:])+" "+hx(b[:])+" "+hx(c2[:])+" "+hx(c3[:]))
    		for _, c := range chunks {
    			d := %s.New()
    			for i := 0; i < len(m); i = i + c {
    				j := i + c
    				if j > len(m) {
    					j = len(m)
    				}
    				_, _ = d.Write(m[i:j])
    			}
    			s1 := d.Sum([]byte{})
    			_, _ = d.Write(m)
    			s2 := d.Sum([]byte{7})
    			out = append(out, hx(s1)+" "+hx(s2))
    		}
    	}
    '''
    gosrc = '''package main

    import (
    	"crypto/sha512"
    	"encoding/hex"
    	"fmt"
    	"strings"
    )

    func hx(b []byte) string { return hex.EncodeToString(b) }

    func main() {
    	out := []string{}
    	msgs := [][]byte{%s}
    	chunks := []int{%s}
    %s	fmt.Print(strings.Join(out, "\\n") + "\\n")
    }
    ''' % (lit, ', '.join(map(str, chunks)), body % ('sha512', 'sha512', 'sha512'))
    sgosrc = '''package main

    import (
    	"std/crypto/sha512"
    	"sgo"
    	"std/encoding/hex"
    	"std/strings"
    )

    func hx(b []byte) string { return hex.EncodeToString(b) }

    func main(root *sgo.Context) error {
    	out := []string{}
    	msgs := [][]byte{%s}
    	chunks := []int{%s}
    %s	return root.Stdout().WriteString(strings.Join(out, "\\n") + "\\n")
    }
    ''' % (lit, ', '.join(map(str, chunks)), body % ('sha512', 'sha512', 'sha512'))
    return gosrc, sgosrc


def gen_ed25519(rnd):
    pass
    cases = [(bytes(rnd.randrange(256) for _ in range(32)), bytes(rnd.randrange(256) for _ in range(rnd.choice([0, 1, 31, 64, 200, rnd.randrange(1000)]))), rnd.randrange(64)) for _ in range(60)]
    lit = ', '.join('{[]byte{%s}, []byte{%s}, %d}' % (', '.join(map(str, s)), ', '.join(map(str, m)), f) for s, m, f in cases)
    body = '''
    	for _, c := range cs {
    		priv := %s
    		pub := %s
    		sig := %s(priv, c.msg)
    		ok := %s(pub, c.msg, sig)
    		bad := make([]byte, len(sig))
    		copy(bad, sig)
    		bad[c.flip] = bad[c.flip] ^ 1
    		ok2 := %s(pub, c.msg, bad)
    		out = append(out, hx(pub)+" "+hx(sig)+" "+tf(ok)+" "+tf(ok2))
    	}
    '''
    common = '''type tc struct {
    	seed []byte
    	msg  []byte
    	flip int
    }
    '''
    gosrc = '''package main

    import (
    	"crypto/ed25519"
    	"encoding/hex"
    	"fmt"
    	"strings"
    )

    %s
    func hx(b []byte) string { return hex.EncodeToString(b) }
    func tf(b bool) string { if b { return "T" }; return "F" }

    func main() {
    	out := []string{}
    	cs := []tc{%s}
    %s	fmt.Print(strings.Join(out, "\\n") + "\\n")
    }
    ''' % (common, lit, body % ('ed25519.NewKeyFromSeed(c.seed)', '[]byte(priv.Public().(ed25519.PublicKey))', 'ed25519.Sign', 'ed25519.Verify', 'ed25519.Verify'))
    sgosrc = '''package main

    import (
    	"std/crypto/ed25519"
    	"sgo"
    	"std/encoding/hex"
    	"std/strings"
    )

    %s
    func hx(b []byte) string {
    	return hex.EncodeToString(b)
    }

    func tf(b bool) string {
    	if b {
    		return "T"
    	}
    	return "F"
    }

    func main(root *sgo.Context) error {
    	out := []string{}
    	cs := []tc{%s}
    %s	return root.Stdout().WriteString(strings.Join(out, "\\n") + "\\n")
    }
    ''' % (common, lit, body % ('ed25519.NewKeyFromSeed(c.seed)', 'ed25519.Public(priv)', 'ed25519.Sign', 'ed25519.Verify', 'ed25519.Verify'))
    return gosrc, sgosrc


def gen_bigmod(rnd):
    pass
    P256 = 2**256 - 2**224 + 2**192 + 2**96 - 1
    N256 = 0xffffffff00000000ffffffffffffffffbce6faada7179e84f3b9cac2fc632551
    P384 = 2**384 - 2**128 - 2**96 + 2**32 - 1
    N384 = 0xffffffffffffffffffffffffffffffffffffffffffffffffc7634d81f4372ddf581a0db248b0a77aecec196accc52973
    mods = [P256, N256, P384, N384, 3, 2**64 - 59, 2**64 + 13]
    for bitsz in [1, 7, 63, 64, 65, 127, 128, 129, 255, 511, 1024, 2047, 2048]:
        for _ in range(2):
            m = rnd.getrandbits(bitsz) | (1 << (bitsz - 1)) | 1
            if m > 1: mods.append(m)
    def b(v, n=None):
        n = n if n is not None else (v.bit_length() + 7) // 8
        return v.to_bytes(n, 'big')
    cases = []
    for m in mods:
        size = (m.bit_length() + 7) // 8
        for _ in range(3):
            x, y = rnd.randrange(m), rnd.randrange(m)
            e = rnd.getrandbits(rnd.choice([1, 8, 64, 256]))
            over = rnd.getrandbits(m.bit_length())
            cases.append((m, x, y, e, over, size))
    def lit(bs): return '[]byte{' + ', '.join(map(str, bs)) + '}'
    rows = ', '.join('{%s, %s, %s, %s, %s}' % (lit(b(m)), lit(b(x, s)), lit(b(y, s)), lit(b(e)), lit(b(over))) for m, x, y, e, over, s in cases)
    gosrc = '''package main

    import (
    	"encoding/hex"
    	"fmt"
    	"math/big"
    	"strings"
    )

    type tc struct{ m, x, y, e, over []byte }

    func fixed(v *big.Int, n int) string {
    	out := make([]byte, n)
    	v.FillBytes(out)
    	return hex.EncodeToString(out)
    }

    func main() {
    	out := []string{}
    	cs := []tc{%s}
    	for _, c := range cs {
    		m := new(big.Int).SetBytes(c.m)
    		n := (m.BitLen() + 7) / 8
    		x := new(big.Int).SetBytes(c.x)
    		y := new(big.Int).SetBytes(c.y)
    		e := new(big.Int).SetBytes(c.e)
    		over := new(big.Int).SetBytes(c.over)
    		add := new(big.Int).Mod(new(big.Int).Add(x, y), m)
    		sub := new(big.Int).Mod(new(big.Int).Sub(x, y), m)
    		mul := new(big.Int).Mod(new(big.Int).Mul(x, y), m)
    		exp := new(big.Int).Exp(x, e, m)
    		f4 := new(big.Int).Exp(x, big.NewInt(65537), m)
    		red := new(big.Int).Mod(over, m)
    		out = append(out, strings.Join([]string{fixed(add, n), fixed(sub, n), fixed(mul, n), fixed(exp, n), fixed(f4, n), fixed(red, n)}, " "))
    	}
    	fmt.Print(strings.Join(out, "\\n") + "\\n")
    }
    ''' % rows
    sgosrc = '''package main

    import (
    	"std/crypto/bigmod"
    	"sgo"
    	"std/encoding/hex"
    	"std/strings"
    )

    type tc struct {
    	m    []byte
    	x    []byte
    	y    []byte
    	e    []byte
    	over []byte
    }

    func main(root *sgo.Context) error {
    	out := []string{}
    	cs := []tc{%s}
    	for _, c := range cs {
    		m, err := bigmod.NewModulus(c.m)
    		if err != nil {
    			return err
    		}
    		x, _ := bigmod.NewNat().SetBytes(c.x, m)
    		y, _ := bigmod.NewNat().SetBytes(c.y, m)
    		a, _ := bigmod.NewNat().SetBytes(c.x, m)
    		s, _ := bigmod.NewNat().SetBytes(c.x, m)
    		p, _ := bigmod.NewNat().SetBytes(c.x, m)
    		a.Add(y, m)
    		s.Sub(y, m)
    		p.Mul(y, m)
    		ex := bigmod.NewNat().Exp(x, c.e, m)
    		f4 := bigmod.NewNat().ExpShortVarTime(x, 65537, m)
    		red, rerr := bigmod.NewNat().SetOverflowingBytes(c.over, m)
    		r := "error"
    		if rerr == nil {
    			r = hex.EncodeToString(red.Bytes(m))
    		}
    		out = append(out, strings.Join([]string{hex.EncodeToString(a.Bytes(m)), hex.EncodeToString(s.Bytes(m)), hex.EncodeToString(p.Bytes(m)), hex.EncodeToString(ex.Bytes(m)), hex.EncodeToString(f4.Bytes(m)), r}, " "))
    	}
    	return root.Stdout().WriteString(strings.Join(out, "\\n") + "\\n")
    }
    ''' % rows
    return gosrc, sgosrc


def wyche_hmac_hkdf(WP):
    rows = []
    for bits, pkg, ctor in ((256, 'sha256', 'sha256.New'), (384, 'sha512', 'sha512.New384'), (512, 'sha512', 'sha512.New')):
        for g in json.load(open(os.path.join(WP, 'hmac_sha%d_test.json' % bits)))['testGroups']:
            for t in g['tests']:
                rows.append('{"hmac", "%s", %d, "%s", "%s", "%s", "", %d, "%s"}' % (ctor, t['tcId'], t['key'], t['msg'], t['tag'], g['tagSize'] // 8, t['result']))
        for g in json.load(open(os.path.join(WP, 'hkdf_sha%d_test.json' % bits)))['testGroups']:
            for t in g['tests']:
                rows.append('{"hkdf", "%s", %d, "%s", "%s", "%s", "%s", %d, "%s"}' % (ctor, t['tcId'], t['ikm'], t['salt'], t['okm'], t['info'], t['size'], t['result']))
    src = '''package main

    import (
    	"sgo"
    	"std/crypto/hkdf"
    	"std/crypto/hmac"
    	"std/crypto/sha256"
    	"std/crypto/sha512"
    	"std/encoding/hex"
    	"std/hash"
    	"std/strconv"
    )

    type vector struct {
    	kind   string
    	hash   string
    	id     int
    	a      string
    	b      string
    	want   string
    	info   string
    	size   int
    	result string
    }

    func ctor(name string) func() hash.Hash {
    	switch name {
    	case "sha512.New384":
    		return sha512.New384
    	case "sha512.New":
    		return sha512.New
    	}
    	return sha256.New
    }

    func unhex(s string) []byte {
    	b, _ := hex.DecodeString(s)
    	return b
    }

    func main(root *sgo.Context) error {
    	vs := []vector{%s}
    	pass := 0
    	bad := ""
    	for _, v := range vs {
    		ok := false
    		if v.kind == "hmac" {
    			m := hmac.New(ctor(v.hash), unhex(v.a))
    			_, _ = m.Write(unhex(v.b))
    			tag := m.Sum([]byte{})
    			ok = hmac.Equal(tag[:v.size], unhex(v.want))
    		} else {
    			k, err := hkdf.Key(ctor(v.hash), unhex(v.a), unhex(v.b), string(unhex(v.info)), v.size)
    			ok = err == nil && hex.EncodeToString(k) == v.want
    		}
    		if ok == (v.result != "invalid") {
    			pass++
    		} else {
    			bad = bad + " " + v.kind + "/" + v.hash + "#" + strconv.Itoa(v.id)
    		}
    	}
    	return root.Stdout().WriteString("wycheproof hmac+hkdf: " + strconv.Itoa(pass) + " of " + strconv.Itoa(len(vs)) + " agree" + bad + "\\n")
    }
    ''' % ', '.join(rows)
    return src


def wyche_chacha20poly1305(WP):
    rows = []
    for g in json.load(open(os.path.join(WP, 'chacha20_poly1305_test.json')))['testGroups']:
        for t in g['tests']:
            rows.append('{%d, "%s", "%s", "%s", "%s", "%s", "%s", %s}' % (t['tcId'], t['key'], t['iv'], t['aad'], t['msg'], t['ct'], t['tag'], 'true' if t['result'] != 'invalid' else 'false'))
    imp = '"std/crypto/chacha20poly1305"'
    src = '''package main

    import (
    	%s
    	"sgo"
    	"std/encoding/hex"
    	"std/strconv"
    )

    type vector struct {
    	id    int
    	key   string
    	iv    string
    	aad   string
    	msg   string
    	ct    string
    	tag   string
    	valid bool
    }

    func unhex(s string) []byte {
    	b, _ := hex.DecodeString(s)
    	return b
    }

    func main(root *sgo.Context) error {
    	vs := []vector{%s}
    	pass := 0
    	skipped := 0
    	bad := ""
    	for _, v := range vs {
    		key := unhex(v.key)
    		iv := unhex(v.iv)
    		if len(key) != chacha20poly1305.KeySize || len(iv) != chacha20poly1305.NonceSize {
    			skipped++
    			continue
    		}
    		a, err := chacha20poly1305.New(key)
    		if err != nil {
    			bad = bad + " new#" + strconv.Itoa(v.id)
    			continue
    		}
    		sealed := hex.EncodeToString(a.Seal([]byte{}, iv, unhex(v.msg), unhex(v.aad)))
    		pt, oerr := a.Open([]byte{}, iv, unhex(v.ct+v.tag), unhex(v.aad))
    		ok := false
    		if v.valid {
    			ok = sealed == v.ct+v.tag && oerr == nil && hex.EncodeToString(pt) == v.msg
    		} else {
    			ok = oerr != nil && oerr.Error() == "chacha20poly1305: message authentication failed"
    		}
    		if ok {
    			pass++
    		} else {
    			bad = bad + " #" + strconv.Itoa(v.id)
    		}
    	}
    	return root.Stdout().WriteString("wycheproof chacha20poly1305: " + strconv.Itoa(pass) + " of " + strconv.Itoa(len(vs)-skipped) + " agree (" + strconv.Itoa(skipped) + " with other key or nonce sizes skipped)" + bad + "\\n")
    }
    ''' % (imp, ', '.join(rows))
    return src


def wyche_x25519(WP):
    rows = []
    for g in json.load(open(os.path.join(WP, 'x25519_test.json')))['testGroups']:
        for t in g['tests']:
            rows.append('{%d, "%s", "%s", "%s"}' % (t['tcId'], t['private'], t['public'], t['shared']))
    imp = '"std/crypto/curve25519"'
    src = '''package main

    import (
    	%s
    	"sgo"
    	"std/encoding/hex"
    	"std/strconv"
    )

    type vector struct {
    	id     int
    	priv   string
    	pub    string
    	shared string
    }

    func unhex(s string) []byte {
    	b, _ := hex.DecodeString(s)
    	return b
    }

    func main(root *sgo.Context) error {
    	vs := []vector{%s}
    	pass := 0
    	bad := ""
    	zero := "0000000000000000000000000000000000000000000000000000000000000000"
    	for _, v := range vs {
    		got, err := curve25519.X25519(unhex(v.priv), unhex(v.pub))
    		ok := false
    		if v.shared == zero {
    			ok = err != nil && err.Error() == "bad input point: low order point"
    		} else {
    			ok = err == nil && hex.EncodeToString(got) == v.shared
    		}
    		if ok {
    			pass++
    		} else {
    			bad = bad + " #" + strconv.Itoa(v.id)
    		}
    	}
    	b, _ := curve25519.X25519(unhex("77076d0a7318a57d3c16c17251b26645df4c2f87ebc0992ab177fba51db92c2a"), curve25519.Basepoint())
    	rfc := hex.EncodeToString(b) == "8520f0098930a754748b7ddcb43ef75a0dbf3a0d26381af4eba4a98eaa9b4e6a"
    	return root.Stdout().WriteString("wycheproof x25519: " + strconv.Itoa(pass) + " of " + strconv.Itoa(len(vs)) + " agree; RFC 7748 6.1 base: " + strconv.FormatBool(rfc) + bad + "\\n")
    }
    ''' % (imp, ', '.join(rows))
    return src


def wyche_ecdsa(WP):
    files = [(f, 'P256' if 'secp256r1' in f else 'P384', f.split('_')[2], f.endswith('_p1363_test.json')) for f in sorted(os.listdir(WP)) if f.startswith(('ecdsa_secp256r1_sha', 'ecdsa_secp384r1_sha')) and f.endswith('_test.json') and f.split('_')[2] in ('sha256', 'sha384', 'sha512')]
    rows = []
    for f, curve, h, p1363 in files:
        for g in json.load(open(os.path.join(WP, f)))['testGroups']:
            pk = g['publicKey']['uncompressed']
            for t in g['tests']:
                rows.append('{"%s", "%s", %s, %d, "%s", "%s", "%s", %s}' % (curve, h, 'true' if p1363 else 'false', t['tcId'], pk, t['msg'], t['sig'], 'true' if t['result'] == 'valid' else 'false'))
    imp_e = '"std/crypto/ecdsa"'
    src = '''package main

    import (
    	%s
    	"sgo"
    	"std/crypto/sha256"
    	"std/crypto/sha512"
    	"std/encoding/hex"
    	"std/strconv"
    )

    type vector struct {
    	curve string
    	hash  string
    	p1363 bool
    	id    int
    	pk    string
    	msg   string
    	sig   string
    	valid bool
    }

    func unhex(s string) []byte {
    	b, _ := hex.DecodeString(s)
    	return b
    }

    func digest(name string, msg []byte) []byte {
    	if name == "sha256" {
    		d := sha256.Sum256(msg)
    		return d[:]
    	}
    	if name == "sha384" {
    		d := sha512.Sum384(msg)
    		return d[:]
    	}
    	d := sha512.Sum512(msg)
    	return d[:]
    }

    func main(root *sgo.Context) error {
    	vs := []vector{%s}
    	p256 := ecdsa.P256()
    	p384 := ecdsa.P384()
    	pass := 0
    	bad := ""
    	for _, v := range vs {
    		c := p256
    		if v.curve == "P384" {
    			c = p384
    		}
    		h := digest(v.hash, unhex(v.msg))
    		sig := unhex(v.sig)
    		got := false
    		if v.p1363 {
    			size := 32
    			if v.curve == "P384" {
    				size = 48
    			}
    			if len(sig) == 2*size {
    				got = ecdsa.Verify(c, unhex(v.pk), h, sig[:len(sig)/2], sig[len(sig)/2:])
    			}
    		} else {
    			got = ecdsa.VerifyASN1(c, unhex(v.pk), h, sig)
    		}
    		if got == v.valid {
    			pass++
    		} else {
    			bad = bad + " " + v.curve + "/" + v.hash + "#" + strconv.Itoa(v.id)
    		}
    	}
    	return root.Stdout().WriteString("wycheproof ecdsa: " + strconv.Itoa(pass) + " of " + strconv.Itoa(len(vs)) + " agree" + bad + "\\n")
    }
    ''' % (imp_e, ', '.join(rows))
    return src


def wyche_ed25519(WP):
    rows = []
    for g in json.load(open(os.path.join(WP, 'ed25519_test.json')))['testGroups']:
        pk = g['publicKey']['pk']
        for t in g['tests']:
            rows.append('{%d, "%s", "%s", "%s", %s}' % (t['tcId'], pk, t['msg'], t['sig'], 'true' if t['result'] == 'valid' else 'false'))
    src = '''package main

    import (
    	"std/crypto/ed25519"
    	"sgo"
    	"std/encoding/hex"
    	"std/strconv"
    )

    type vector struct {
    	id    int
    	pk    string
    	msg   string
    	sig   string
    	valid bool
    }

    func unhex(s string) []byte {
    	b, _ := hex.DecodeString(s)
    	return b
    }

    func main(root *sgo.Context) error {
    	vs := []vector{%s}
    	pass := 0
    	bad := ""
    	for _, v := range vs {
    		if ed25519.Verify(unhex(v.pk), unhex(v.msg), unhex(v.sig)) == v.valid {
    			pass++
    		} else {
    			bad = bad + " #" + strconv.Itoa(v.id)
    		}
    	}
    	priv := ed25519.NewKeyFromSeed(unhex("9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60"))
    	sig := ed25519.Sign(priv, []byte{})
    	rfc := hex.EncodeToString(ed25519.Public(priv)) == "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a" && hex.EncodeToString(sig) == "e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e065224901555fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b"
    	return root.Stdout().WriteString("wycheproof ed25519: " + strconv.Itoa(pass) + " of " + strconv.Itoa(len(vs)) + " agree; RFC 8032 test 1: " + strconv.FormatBool(rfc) + bad + "\\n")
    }
    ''' % ', '.join(rows)
    return src


def main():
    seed = 1
    if "--seed" in sys.argv:
        seed = int(sys.argv[sys.argv.index("--seed") + 1])
    failed = False
    for name, gen in (("bits", gen_bits), ("encoding", gen_encoding), ("sha256", gen_sha256), ("sha512", gen_sha512), ("ed25519", gen_ed25519), ("bigmod", gen_bigmod)):
        gosrc, sgosrc = gen(random.Random(seed))
        g, s = run_go(gosrc), run_sgo(sgosrc)
        if g.returncode or s.returncode:
            print(f"test-std-crypto: {name}: go exit {g.returncode}, sgo exit {s.returncode}\n{g.stderr[-800:]}{s.stderr[-1500:]}")
            failed = True
            continue
        gl, sl = g.stdout.splitlines(), s.stdout.splitlines()
        bad = [i for i in range(max(len(gl), len(sl))) if i >= len(gl) or i >= len(sl) or gl[i] != sl[i]]
        print(f"test-std-crypto: {name}: {len(gl)} results, {len(bad)} differ (seed {seed})")
        if bad:
            i = bad[0]
            print(f"  go : {gl[i] if i < len(gl) else None!r}\n  sgo: {sl[i] if i < len(sl) else None!r}")
            failed = True
    WP = wycheproof_dir()
    for name, gen in (("hmac+hkdf", wyche_hmac_hkdf), ("chacha20poly1305", wyche_chacha20poly1305), ("x25519", wyche_x25519), ("ecdsa", wyche_ecdsa), ("ed25519", wyche_ed25519)):
        r = run_sgo(gen(WP))
        line = r.stdout.strip()
        print("test-std-crypto: " + (line or f"{name}: exit {r.returncode}\n{r.stderr[-1500:]}"))
        m = re.search(r": (\d+) of (\d+) agree", line)
        tail = line.split(" agree", 1)[1] if " agree" in line else ""
        if r.returncode or not m or m.group(1) != m.group(2) or "#" in tail or "base: false" in tail or "test 1: false" in tail:
            failed = True
    if failed:
        sys.exit("test-std-crypto: FAIL")
    print("test-std-crypto: ok")


if __name__ == "__main__":
    main()
