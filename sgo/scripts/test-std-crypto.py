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
std/crypto/hmac, hkdf, chacha20poly1305, curve25519, ed25519, ecdsa, rsa and aes-gcm (SGO-STD-018,
019, 020, 021, 022, 023, 025, 026, 027).

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
XCRYPTO = [l.split()[1] for l in open(os.path.join(ROOT, "src", "go.mod")) if "golang.org/x/crypto" in l][0]


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
        open(os.path.join(d, "go.mod"), "w").write("module stdcrypto\n\ngo 1.26\n\nrequire golang.org/x/crypto " + XCRYPTO + "\n")
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


def gen_aes(rnd):
    pass
    cases = []
    for ks in (16, 24, 32):
        for n in range(12):
            key = bytes(rnd.getrandbits(8) for _ in range(ks))
            blk = bytes(rnd.getrandbits(8) for _ in range(16))
            nonce = bytes(rnd.getrandbits(8) for _ in range(12))
            if n == 0:
                nonce = nonce[:8] + b'\xff\xff\xff\xfe'
            pt = bytes(rnd.getrandbits(8) for _ in range(rnd.choice([0, 1, 15, 16, 17, 31, 64, 100, 257])))
            ad = bytes(rnd.getrandbits(8) for _ in range(rnd.choice([0, 1, 13, 16, 40])))
            cases.append((key, blk, nonce, pt, ad))
    for bad in (0, 15, 17, 33):
        cases.append((bytes(bad), bytes(16), bytes(12), b'', b''))
    def lit(bs): return '[]byte{' + ', '.join(map(str, bs)) + '}'
    rows = ', '.join('{%s, %s, %s, %s, %s}' % tuple(lit(x) for x in c) for c in cases)
    gosrc = '''package main

    import (
    	"crypto/aes"
    	"crypto/cipher"
    	"encoding/hex"
    	"fmt"
    )

    type tc struct{ key, blk, nonce, pt, ad []byte }

    func main() {
    	for _, c := range []tc{%s} {
    		b, err := aes.NewCipher(c.key)
    		if err != nil {
    			fmt.Println("error", err.Error())
    			continue
    		}
    		e := make([]byte, 16)
    		d := make([]byte, 16)
    		b.Encrypt(e, c.blk)
    		b.Decrypt(d, c.blk)
    		g, _ := cipher.NewGCM(b)
    		ct := g.Seal([]byte{7}, c.nonce, c.pt, c.ad)
    		pt, err := g.Open([]byte{}, c.nonce, ct[1:], c.ad)
    		ok := err == nil && hex.EncodeToString(pt) == hex.EncodeToString(c.pt)
    		ct[len(ct)-1] ^= 1
    		_, err2 := g.Open([]byte{}, c.nonce, ct[1:], c.ad)
    		fmt.Println(hex.EncodeToString(e), hex.EncodeToString(d), hex.EncodeToString(ct), ok, err2.Error())
    	}
    }
    ''' % rows
    sgosrc = '''package main

    import (
    	"std/crypto/aes"
    	"std/crypto/cipher"
    	"sgo"
    	"std/encoding/hex"
    )

    type tc struct {
    	key   []byte
    	blk   []byte
    	nonce []byte
    	pt    []byte
    	ad    []byte
    }

    func main(root *sgo.Context) error {
    	out := ""
    	for _, c := range []tc{%s} {
    		b, err := aes.NewCipher(c.key)
    		if err != nil {
    			out = out + "error " + err.Error() + "\\n"
    			continue
    		}
    		e := make([]byte, 16)
    		d := make([]byte, 16)
    		b.Encrypt(e, c.blk)
    		b.Decrypt(d, c.blk)
    		g, _ := cipher.NewGCM(b)
    		ct := g.Seal([]byte{7}, c.nonce, c.pt, c.ad)
    		pt, err1 := g.Open([]byte{}, c.nonce, ct[1:], c.ad)
    		ok := "false"
    		if err1 == nil && hex.EncodeToString(pt) == hex.EncodeToString(c.pt) {
    			ok = "true"
    		}
    		ct[len(ct)-1] = ct[len(ct)-1] ^ 1
    		_, err2 := g.Open([]byte{}, c.nonce, ct[1:], c.ad)
    		out = out + hex.EncodeToString(e) + " " + hex.EncodeToString(d) + " " + hex.EncodeToString(ct) + " " + ok + " " + err2.Error() + "\\n"
    	}
    	return root.Stdout().WriteString(out)
    }
    ''' % rows
    return gosrc, sgosrc


def gen_asn1(rnd):
    pass
    def enc_len(n, long=False):
        if n < 128 and not long:
            return bytes([n])
        b = n.to_bytes(max(1, (n.bit_length() + 7) // 8), 'big')
        return bytes([0x80 | len(b)]) + b
    def el(tag, content, long=False):
        return bytes([tag]) + enc_len(len(content), long) + content
    def rbytes(n): return bytes(rnd.getrandbits(8) for _ in range(n))
    def digits(n, lo=0, hi=99): return ('%0' + str(n) + 'd') % rnd.randint(lo, hi)
    def timestr(gen):
        y = digits(4, 0, 9999) if gen else digits(2)
        v = rnd.random() < 0.7
        parts = [y, digits(2, 1, 12) if v else digits(2, 0, 13), digits(2, 1, 31) if v else digits(2, 0, 32), digits(2, 0, 23) if v else digits(2, 0, 24), digits(2, 0, 59) if v else digits(2, 0, 60)]
        if gen or rnd.random() < 0.8:
            parts.append(digits(2, 0, 59) if v else digits(2, 0, 61))
        if rnd.random() < 0.1:
            parts.append('.5')
        z = rnd.choice(['Z', 'Z', 'Z', '+0000', '-0000', '+0130', '-1145', '+2400', '+2500', '+0160', 'z', '', 'Z0'])
        return (''.join(parts) + z).encode()
    cases = []
    for _ in range(400):
        k = rnd.randrange(14)
        if k == 0: x = el(1, rnd.choice([b'\x00', b'\xff', b'\x01', b'', b'\x00\x00']))
        elif k in (1, 2):
            c = rnd.choice([rbytes(rnd.randint(1, 10)), b'\x00' + rbytes(rnd.randint(0, 3)), b'\xff' + rbytes(rnd.randint(0, 3)), b'', b'\x80' + bytes(7), b'\x7f' + b'\xff' * 7])
            x = el(2 if k == 1 else 10, c)
        elif k == 3:
            arcs = [rnd.randint(0, 2), rnd.randint(0, 39)] + [rnd.choice([rnd.randint(0, 127), rnd.randint(0, 2**28), rnd.randint(0, 2**35)]) for _ in range(rnd.randint(0, 5))]
            def b128(v):
                out = [v & 0x7f]; v >>= 7
                while v: out.append(0x80 | (v & 0x7f)); v >>= 7
                return bytes(reversed(out))
            c = b128(40 * arcs[0] + arcs[1]) + b''.join(b128(a) for a in arcs[2:])
            if rnd.random() < 0.2: c = rnd.choice([b'\x80\x01', c[:-1] + bytes([c[-1] | 0x80]), b''])
            x = el(6, c)
        elif k == 4:
            pad = rnd.choice([0, 0, 3, 7, 8]); body = rbytes(rnd.randint(0, 4))
            if body and rnd.random() < 0.7: body = body[:-1] + bytes([body[-1] & (0xff ^ ((1 << min(pad, 7)) - 1))])
            x = el(3, bytes([pad]) + body)
        elif k == 5: x = el(24, timestr(True))
        elif k == 6: x = el(23, timestr(False))
        elif k == 7: x = el(0x30, el(2, b'\x05') + el(4, rbytes(3)), long=rnd.random() < 0.3)
        elif k == 8: x = el(0xa0, el(2, rbytes(rnd.randint(1, 3)))) + el(0xa1, el(4, rbytes(2))) + el(0xa2, el(1, b'\xff'))
        elif k == 9: x = el(0x30, rbytes(200))
        elif k == 10: x = bytes([0x30, 0x81, 0x05]) + rbytes(5)
        elif k == 11: x = bytes([0x30, 0x82, 0x00, 0x90]) + rbytes(0x90)
        elif k == 12: x = bytes([0x1f, 0x01, 0x00])
        else: x = rbytes(rnd.randint(0, 6))
        if rnd.random() < 0.1 and x: x = x[:rnd.randrange(len(x))]
        cases.append(x)
    def lit(bs): return '[]byte{' + ', '.join(map(str, bs)) + '}'
    rows = ', '.join(lit(c) for c in cases)
    gosrc = '''package main

    import (
    	"encoding/asn1"
    	"encoding/hex"
    	"fmt"
    	"strings"
    	"time"

    	"golang.org/x/crypto/cryptobyte"
    	cbasn1 "golang.org/x/crypto/cryptobyte/asn1"
    )

    func sp(a ...interface{}) string {
    	return strings.TrimSuffix(fmt.Sprintln(a...), "\\n")
    }

    func main() {
    	for _, in := range [][]byte{%s} {
    		var r []string
    		{
    			s := cryptobyte.String(in)
    			var out cryptobyte.String
    			var tag cbasn1.Tag
    			ok := s.ReadAnyASN1(&out, &tag)
    			r = append(r, sp(ok, int(tag), hex.EncodeToString(out), len(s)))
    		}
    		{
    			s := cryptobyte.String(in)
    			var out cryptobyte.String
    			ok := s.ReadASN1Element(&out, cbasn1.SEQUENCE)
    			r = append(r, sp(ok, hex.EncodeToString(out)))
    		}
    		{
    			s := cryptobyte.String(in)
    			var b bool
    			ok := s.ReadASN1Boolean(&b)
    			r = append(r, sp(ok, b))
    		}
    		{
    			s := cryptobyte.String(in)
    			var i int64
    			ok := s.ReadASN1Integer(&i)
    			if !ok {
    				i = 0
    			}
    			r = append(r, sp(ok, i))
    		}
    		{
    			s := cryptobyte.String(in)
    			var b []byte
    			ok := s.ReadASN1Integer(&b)
    			r = append(r, sp(ok, hex.EncodeToString(b)))
    		}
    		{
    			s := cryptobyte.String(in)
    			var e int
    			ok := s.ReadASN1Enum(&e)
    			r = append(r, sp(ok, e))
    		}
    		{
    			s := cryptobyte.String(in)
    			var o asn1.ObjectIdentifier
    			ok := s.ReadASN1ObjectIdentifier(&o)
    			str := ""
    			if ok {
    				str = o.String()
    			}
    			r = append(r, sp(ok, str))
    		}
    		{
    			s := cryptobyte.String(in)
    			var b asn1.BitString
    			ok := s.ReadASN1BitString(&b)
    			bits := ""
    			for i := 0; i < b.BitLength; i++ {
    				bits += fmt.Sprint(b.At(i))
    			}
    			r = append(r, sp(ok, b.BitLength, hex.EncodeToString(b.Bytes), bits))
    		}
    		{
    			s := cryptobyte.String(in)
    			var b []byte
    			ok := s.ReadASN1BitStringAsBytes(&b)
    			r = append(r, sp(ok, hex.EncodeToString(b)))
    		}
    		{
    			s := cryptobyte.String(in)
    			var t time.Time
    			ok := s.ReadASN1GeneralizedTime(&t)
    			u := int64(0)
    			if ok {
    				u = t.Unix()
    			}
    			r = append(r, sp(ok, u))
    		}
    		{
    			s := cryptobyte.String(in)
    			var t time.Time
    			ok := s.ReadASN1UTCTime(&t)
    			u := int64(0)
    			if ok {
    				u = t.Unix()
    			}
    			r = append(r, sp(ok, u))
    		}
    		{
    			s := cryptobyte.String(in)
    			var i int64
    			ok1 := s.ReadOptionalASN1Integer(&i, cbasn1.Tag(0).Constructed().ContextSpecific(), int64(-7))
    			var o []byte
    			var p bool
    			ok2 := s.ReadOptionalASN1OctetString(&o, &p, cbasn1.Tag(1).Constructed().ContextSpecific())
    			var b bool
    			ok3 := s.ReadOptionalASN1Boolean(&b, cbasn1.Tag(2).Constructed().ContextSpecific(), true)
    			ok4 := s.SkipOptionalASN1(cbasn1.SEQUENCE)
    			r = append(r, sp(ok1, i, ok2, hex.EncodeToString(o), p, ok3, b, ok4, s.PeekASN1Tag(cbasn1.INTEGER), s.Empty()))
    		}
    		fmt.Println(strings.Join(r, " | "))
    	}
    }
    ''' % rows
    sgosrc = '''package main

    import (
    	"std/encoding/asn1"
    	"sgo"
    	"std/encoding/hex"
    	"std/strconv"
    )

    func b2s(b bool) string {
    	if b {
    		return "true"
    	}
    	return "false"
    }

    func main(root *sgo.Context) error {
    	all := ""
    	for _, in := range [][]byte{%s} {
    		r := ""
    		{
    			s := asn1.String(in)
    			var out asn1.String = asn1.String{}
    			var tag asn1.Tag = 0
    			ok := s.ReadAnyASN1(&out, &tag)
    			r = r + b2s(ok) + " " + strconv.Itoa(int(tag)) + " " + hex.EncodeToString(out) + " " + strconv.Itoa(len(s))
    		}
    		{
    			s := asn1.String(in)
    			var out asn1.String = asn1.String{}
    			ok := s.ReadASN1Element(&out, asn1.SEQUENCE)
    			r = r + " | " + b2s(ok) + " " + hex.EncodeToString(out)
    		}
    		{
    			s := asn1.String(in)
    			b := false
    			ok := s.ReadASN1Boolean(&b)
    			r = r + " | " + b2s(ok) + " " + b2s(b)
    		}
    		{
    			s := asn1.String(in)
    			i := 0
    			ok := s.ReadASN1Integer(&i)
    			if !ok {
    				i = 0
    			}
    			r = r + " | " + b2s(ok) + " " + strconv.Itoa(i)
    		}
    		{
    			s := asn1.String(in)
    			b := []byte{}
    			ok := s.ReadASN1IntegerBytes(&b)
    			r = r + " | " + b2s(ok) + " " + hex.EncodeToString(b)
    		}
    		{
    			s := asn1.String(in)
    			e := 0
    			ok := s.ReadASN1Enum(&e)
    			r = r + " | " + b2s(ok) + " " + strconv.Itoa(e)
    		}
    		{
    			s := asn1.String(in)
    			var o asn1.ObjectIdentifier = asn1.ObjectIdentifier{}
    			ok := s.ReadASN1ObjectIdentifier(&o)
    			str := ""
    			if ok {
    				str = o.String()
    			}
    			r = r + " | " + b2s(ok) + " " + str
    		}
    		{
    			s := asn1.String(in)
    			b := asn1.BitString{Bytes: []byte{}, BitLength: 0}
    			ok := s.ReadASN1BitString(&b)
    			bits := ""
    			for i := 0; i < b.BitLength; i++ {
    				bits = bits + strconv.Itoa(b.At(i))
    			}
    			r = r + " | " + b2s(ok) + " " + strconv.Itoa(b.BitLength) + " " + hex.EncodeToString(b.Bytes) + " " + bits
    		}
    		{
    			s := asn1.String(in)
    			b := []byte{}
    			ok := s.ReadASN1BitStringAsBytes(&b)
    			r = r + " | " + b2s(ok) + " " + hex.EncodeToString(b)
    		}
    		{
    			s := asn1.String(in)
    			u := 0
    			ok := s.ReadASN1GeneralizedTime(&u)
    			if !ok {
    				u = 0
    			}
    			r = r + " | " + b2s(ok) + " " + strconv.Itoa(u)
    		}
    		{
    			s := asn1.String(in)
    			u := 0
    			ok := s.ReadASN1UTCTime(&u)
    			if !ok {
    				u = 0
    			}
    			r = r + " | " + b2s(ok) + " " + strconv.Itoa(u)
    		}
    		{
    			s := asn1.String(in)
    			i := 0
    			ok1 := s.ReadOptionalASN1Integer(&i, asn1.Tag(0).Constructed().ContextSpecific(), -7)
    			o := []byte{}
    			p := false
    			ok2 := s.ReadOptionalASN1OctetString(&o, &p, asn1.Tag(1).Constructed().ContextSpecific())
    			b := false
    			ok3 := s.ReadOptionalASN1Boolean(&b, asn1.Tag(2).Constructed().ContextSpecific(), true)
    			ok4 := s.SkipOptionalASN1(asn1.SEQUENCE)
    			r = r + " | " + b2s(ok1) + " " + strconv.Itoa(i) + " " + b2s(ok2) + " " + hex.EncodeToString(o) + " " + b2s(p) + " " + b2s(ok3) + " " + b2s(b) + " " + b2s(ok4) + " " + b2s(s.PeekASN1Tag(asn1.INTEGER)) + " " + b2s(s.Empty())
    		}
    		all = all + r + "\\n"
    	}
    	return root.Stdout().WriteString(all)
    }
    ''' % rows
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
    names = {'secp224r1': 'P224', 'secp256r1': 'P256', 'secp384r1': 'P384', 'secp521r1': 'P521'}
    files = [(f, names[f.split('_')[1]], f.split('_')[2], f.endswith('_p1363_test.json')) for f in sorted(os.listdir(WP)) if f.startswith('ecdsa_') and f.split('_')[1] in names and f.endswith('_test.json') and f.split('_')[2] in ('sha224', 'sha256', 'sha384', 'sha512')]
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
    	if name == "sha224" {
    		d := sha256.Sum224(msg)
    		return d[:]
    	}
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
    	p224 := ecdsa.P224()
    	p256 := ecdsa.P256()
    	p384 := ecdsa.P384()
    	p521 := ecdsa.P521()
    	pass := 0
    	bad := ""
    	for _, v := range vs {
    		c := p256
    		size := 32
    		if v.curve == "P224" {
    			c = p224
    			size = 28
    		}
    		if v.curve == "P384" {
    			c = p384
    			size = 48
    		}
    		if v.curve == "P521" {
    			c = p521
    			size = 66
    		}
    		h := digest(v.hash, unhex(v.msg))
    		sig := unhex(v.sig)
    		got := false
    		if v.p1363 {
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


def wyche_aes_gcm(WP):
    rows = []
    skipped = 0
    for g in json.load(open(os.path.join(WP, 'aes_gcm_test.json')))['testGroups']:
        for t in g['tests']:
            # NewGCM: 96-bit nonces and 128-bit tags only, as Go's crypto/cipher NewGCM.
            if g['ivSize'] != 96 or g['tagSize'] != 128 or g['keySize'] not in (128, 192, 256):
                skipped += 1
                continue
            rows.append('{%d, "%s", "%s", "%s", "%s", "%s", "%s", %s}' % (t['tcId'], t['key'], t['iv'], t['aad'], t['msg'], t['ct'], t['tag'], 'true' if t['result'] == 'valid' else 'false'))
    imp_a = '"std/crypto/aes"'
    imp_c = '"std/crypto/cipher"'
    src = '''package main

    import (
    	%s
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
    	bad := ""
    	for _, v := range vs {
    		b, err := aes.NewCipher(unhex(v.key))
    		if err != nil {
    			bad = bad + " #" + strconv.Itoa(v.id)
    			continue
    		}
    		g, _ := cipher.NewGCM(b)
    		sealed := hex.EncodeToString(g.Seal([]byte{}, unhex(v.iv), unhex(v.msg), unhex(v.aad)))
    		pt, err2 := g.Open([]byte{}, unhex(v.iv), unhex(v.ct+v.tag), unhex(v.aad))
    		good := sealed == v.ct+v.tag && err2 == nil && hex.EncodeToString(pt) == v.msg
    		if v.valid == good || (!v.valid && err2 != nil) {
    			pass++
    		} else {
    			bad = bad + " #" + strconv.Itoa(v.id)
    		}
    	}
    	return root.Stdout().WriteString("aes-gcm: " + strconv.Itoa(pass) + " of " + strconv.Itoa(len(vs)) + " agree" + bad + "\\n")
    }
    ''' % (imp_a, imp_c, ', '.join(rows))
    return src


def wyche_rsa(WP):
    rows = []
    hashes = {'SHA-256': 1, 'SHA-384': 2, 'SHA-512': 3}
    for f in sorted(os.listdir(WP)):
        m1 = re.match(r'rsa_signature_(\d+)_sha(256|384|512)_test\.json$', f)
        m2 = re.match(r'rsa_pss_(\d+)_sha(256|384|512)_mgf1_(\d+)_test\.json$', f)
        if not (m1 or m2):
            continue
        for g in json.load(open(os.path.join(WP, f)))['testGroups']:
            h = hashes[g['sha']]
            if m2 and g.get('mgfSha') != g['sha']:
                continue
            pss = 1 if m2 else 0
            slen = g.get('sLen', 0) if m2 else 0
            n, e = g['publicKey']['modulus'], int(g['publicKey']['publicExponent'], 16)
            for t in g['tests']:
                valid = t['result'] == 'valid'
                # As Go's rsa_wycheproof_test.go (filesOverrideToPassZeroSLen): a salt length of 0 is
                # PSSSaltLengthAuto, so these signatures with other salt lengths verify.
                if f == 'rsa_pss_2048_sha256_mgf1_0_test.json' and t['tcId'] in (67, 68, 69, 70):
                    valid = True
                rows.append('{%d, %d, %d, "%s", %d, "%s", "%s", %d, %s, "%s"}' % (pss, h, slen, n, e, t['msg'], t['sig'], t['tcId'], 'true' if valid else 'false', f[:32]))
    imp = '"std/crypto/rsa"'
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
    	pss   int
    	hash  int
    	slen  int
    	n     string
    	e     int
    	msg   string
    	sig   string
    	id    int
    	valid bool
    	file  string
    }

    func unhex(s string) []byte {
    	b, _ := hex.DecodeString(s)
    	return b
    }

    func digest(h int, msg []byte) []byte {
    	if h == 2 {
    		d := sha512.Sum384(msg)
    		return d[:]
    	}
    	if h == 3 {
    		d := sha512.Sum512(msg)
    		return d[:]
    	}
    	d := sha256.Sum256(msg)
    	return d[:]
    }

    func main(root *sgo.Context) error {
    	vs := []vector{%s}
    	pass := 0
    	bad := ""
    	for _, v := range vs {
    		pub := &rsa.PublicKey{N: unhex(v.n), E: v.e}
    		h := rsa.Hash(v.hash)
    		d := digest(v.hash, unhex(v.msg))
    		var err error = nil
    		if v.pss == 1 {
    			err = rsa.VerifyPSS(pub, h, d, unhex(v.sig), v.slen)
    		} else {
    			err = rsa.VerifyPKCS1v15(pub, h, d, unhex(v.sig))
    		}
    		if (err == nil) == v.valid {
    			pass++
    		} else {
    			bad = bad + " " + v.file + "#" + strconv.Itoa(v.id)
    		}
    	}
    	return root.Stdout().WriteString("wycheproof rsa: " + strconv.Itoa(pass) + " of " + strconv.Itoa(len(vs)) + " agree" + bad + "\\n")
    }
    ''' % (imp, ', '.join(rows))
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

LIMBO_GEN = '// Command gen reads x509-limbo\'s limbo.json, applies the skips of Go\'s TestX509Limbo, runs\n// Go\'s crypto/x509 on each case, and writes cases.txt (inputs for the S-Go harness) and\n// go.txt (Go\'s verdict per case).\npackage main\n\nimport (\n\t"crypto/x509"\n\t"encoding/hex"\n\t"encoding/json"\n\t"encoding/pem"\n\t"fmt"\n\t"os"\n\t"strings"\n\t"time"\n)\n\ntype peer struct {\n\tKind  string `json:"kind"`\n\tValue string `json:"value"`\n}\n\ntype testcase struct {\n\tID                     string   `json:"id"`\n\tFeatures               []string `json:"features"`\n\tSignatureAlgorithms    []string `json:"signature_algorithms"`\n\tKeyUsage               []string `json:"key_usage"`\n\tExtendedKeyUsage       []string `json:"extended_key_usage"`\n\tValidationKind         string   `json:"validation_kind"`\n\tExpectedPeerName       *peer    `json:"expected_peer_name"`\n\tExpectedPeerNames      []peer   `json:"expected_peer_names"`\n\tTrustedCerts           []string `json:"trusted_certs"`\n\tUntrustedIntermediates []string `json:"untrusted_intermediates"`\n\tPeerCertificate        string   `json:"peer_certificate"`\n\tValidationTime         *string  `json:"validation_time"`\n\tExpectedResult         string   `json:"expected_result"`\n}\n\nvar ekuMap = map[string]x509.ExtKeyUsage{\n\t"anyExtendedKeyUsage": x509.ExtKeyUsageAny,\n\t"clientAuth":          x509.ExtKeyUsageClientAuth,\n\t"codeSigning":         x509.ExtKeyUsageCodeSigning,\n\t"emailProtection":     x509.ExtKeyUsageEmailProtection,\n\t"OCSPSigning":         x509.ExtKeyUsageOCSPSigning,\n\t"serverAuth":          x509.ExtKeyUsageServerAuth,\n\t"timeStamping":        x509.ExtKeyUsageTimeStamping,\n}\n\nfunc has(l []string, s string) bool {\n\tfor _, x := range l {\n\t\tif x == s {\n\t\t\treturn true\n\t\t}\n\t}\n\treturn false\n}\n\nfunc ders(pems []string) []string {\n\tout := []string{}\n\tfor _, p := range pems {\n\t\trest := []byte(p)\n\t\tfor {\n\t\t\tvar b *pem.Block\n\t\t\tb, rest = pem.Decode(rest)\n\t\t\tif b == nil {\n\t\t\t\tbreak\n\t\t\t}\n\t\t\tif b.Type == "CERTIFICATE" && len(b.Headers) == 0 {\n\t\t\t\tout = append(out, hex.EncodeToString(b.Bytes))\n\t\t\t}\n\t\t}\n\t}\n\treturn out\n}\n\nfunc pool(hexes []string) *x509.CertPool {\n\tp := x509.NewCertPool()\n\tfor _, h := range hexes {\n\t\td, _ := hex.DecodeString(h)\n\t\tif c, err := x509.ParseCertificate(d); err == nil {\n\t\t\tp.AddCert(c)\n\t\t}\n\t}\n\treturn p\n}\n\nfunc result(err error) string {\n\tif err == nil {\n\t\treturn "ok"\n\t}\n\treturn "err " + strings.ReplaceAll(err.Error(), "\\n", " ")\n}\n\nfunc main() {\n\tdata, err := os.ReadFile(os.Args[1])\n\tif err != nil {\n\t\tpanic(err)\n\t}\n\tvar limbo struct {\n\t\tTestcases []testcase `json:"testcases"`\n\t}\n\tif err := json.Unmarshal(data, &limbo); err != nil {\n\t\tpanic(err)\n\t}\n\tnow := time.Now().Unix()\n\tcases, _ := os.Create(os.Args[2])\n\tgores, _ := os.Create(os.Args[3])\n\tdefer cases.Close()\n\tdefer gores.Close()\n\tfor _, tc := range limbo.Testcases {\n\t\tif has(tc.Features, "has-crl") || has(tc.Features, "max-chain-depth") || has(tc.Features, "name-constraint-dn") ||\n\t\t\tlen(tc.SignatureAlgorithms) != 0 || len(tc.KeyUsage) != 0 && !has(tc.KeyUsage, "digitalSignature") {\n\t\t\tcontinue\n\t\t}\n\t\tdns := ""\n\t\tif tc.ExpectedPeerName != nil && tc.ValidationKind == "SERVER" {\n\t\t\tswitch tc.ExpectedPeerName.Kind {\n\t\t\tcase "DNS":\n\t\t\t\tdns = tc.ExpectedPeerName.Value\n\t\t\tcase "IP":\n\t\t\t\tdns = "[" + tc.ExpectedPeerName.Value + "]"\n\t\t\tdefault:\n\t\t\t\tcontinue\n\t\t\t}\n\t\t}\n\t\tekus, ekuStr, skip := []x509.ExtKeyUsage{}, []string{}, false\n\t\tfor _, e := range tc.ExtendedKeyUsage {\n\t\t\tu, ok := ekuMap[e]\n\t\t\tif !ok {\n\t\t\t\tskip = true\n\t\t\t}\n\t\t\tekus = append(ekus, u)\n\t\t\tekuStr = append(ekuStr, fmt.Sprint(int(u)))\n\t\t}\n\t\tnames := []string{}\n\t\tif tc.ValidationKind == "CLIENT" {\n\t\t\tfor _, n := range tc.ExpectedPeerNames {\n\t\t\t\tif n.Kind != "IP" && n.Kind != "DNS" {\n\t\t\t\t\tskip = true\n\t\t\t\t}\n\t\t\t\tnames = append(names, n.Value)\n\t\t\t}\n\t\t}\n\t\tif skip {\n\t\t\tcontinue\n\t\t}\n\t\tvt := now\n\t\tif tc.ValidationTime != nil {\n\t\t\tt, err := time.Parse(time.RFC3339, *tc.ValidationTime)\n\t\t\tif err != nil {\n\t\t\t\tpanic(err)\n\t\t\t}\n\t\t\tvt = t.Unix()\n\t\t}\n\t\tpeerDER := ders([]string{tc.PeerCertificate})\n\t\tif len(peerDER) != 1 {\n\t\t\tcontinue\n\t\t}\n\t\troots, ints := ders(tc.TrustedCerts), ders(tc.UntrustedIntermediates)\n\t\tfmt.Fprintf(cases, "%s\\t%s\\t%d\\t%s\\t%s\\t%s\\t%s\\t%s\\n", tc.ID, dns, vt, strings.Join(ekuStr, ","),\n\t\t\tstrings.Join(names, ","), peerDER[0], strings.Join(roots, ","), strings.Join(ints, ","))\n\t\td, _ := hex.DecodeString(peerDER[0])\n\t\tc, perr := x509.ParseCertificate(d)\n\t\tif perr != nil {\n\t\t\tfmt.Fprintf(gores, "%s\\tparse %s\\n", tc.ID, result(perr))\n\t\t\tcontinue\n\t\t}\n\t\t_, verr := c.Verify(x509.VerifyOptions{DNSName: dns, Intermediates: pool(ints), Roots: pool(roots),\n\t\t\tCurrentTime: time.Unix(vt, 0).UTC(), KeyUsages: ekus})\n\t\tline := tc.ID + "\\tverify " + result(verr)\n\t\tfor _, n := range names {\n\t\t\tline += "\\thost " + result(c.VerifyHostname(n))\n\t\t}\n\t\tfmt.Fprintln(gores, line)\n\t}\n}\n'

LIMBO_SGO = 'package main\n\nimport (\n\t"sgo"\n\t"std/crypto/x509"\n\t"std/encoding/hex"\n\t"std/strconv"\n\t"std/strings"\n)\n\nfunc result(err error) string {\n\tif err == nil {\n\t\treturn "ok"\n\t}\n\treturn "err " + strings.ReplaceAll(err.Error(), "\\n", " ")\n}\n\nfunc pool(hexes string, afters string) *x509.CertPool {\n\tp := x509.NewCertPool()\n\tif hexes == "" {\n\t\treturn p\n\t}\n\tl := strings.Split(hexes, ",")\n\ta := strings.Split(afters, ",")\n\tfor i := 0; i < len(l); i++ {\n\t\td, _ := hex.DecodeString(l[i])\n\t\tc, err := x509.ParseCertificate(d)\n\t\tif err == nil {\n\t\t\tif i < len(a) && a[i] != "" {\n\t\t\t\tt, _ := strconv.Atoi(a[i])\n\t\t\t\tp.AddCertWithDistrustAfter(c, t)\n\t\t\t} else {\n\t\t\t\tp.AddCert(c)\n\t\t\t}\n\t\t}\n\t}\n\treturn p\n}\n\nfunc one(line string) string {\n\tf := strings.Split(line, "\\t")\n\tid := f[0]\n\tafter := ""\n\tif len(f) > 8 {\n\t\tafter = f[8]\n\t}\n\tvt, _ := strconv.Atoi(f[2])\n\tekus := []x509.ExtKeyUsage{}\n\tif f[3] != "" {\n\t\te := strings.Split(f[3], ",")\n\t\tfor i := 0; i < len(e); i++ {\n\t\t\tv, _ := strconv.Atoi(e[i])\n\t\t\tekus = append(ekus, x509.ExtKeyUsage(v))\n\t\t}\n\t}\n\td, _ := hex.DecodeString(f[5])\n\tc, perr := x509.ParseCertificate(d)\n\tif perr != nil {\n\t\treturn id + "\\tparse " + result(perr)\n\t}\n\t_, verr := c.Verify(x509.VerifyOptions{DNSName: f[1], Intermediates: pool(f[7], ""), Roots: pool(f[6], after),\n\t\tCurrentTime: vt, KeyUsages: ekus, CertificatePolicies: []x509.OID{}})\n\tout := id + "\\tverify " + result(verr)\n\tif f[4] != "" {\n\t\tnames := strings.Split(f[4], ",")\n\t\tfor i := 0; i < len(names); i++ {\n\t\t\tout += "\\thost " + result(c.VerifyHostname(names[i]))\n\t\t}\n\t}\n\treturn out\n}\n\nfunc main(root *sgo.Context) error {\n\tin := root.Stdin()\n\tout := root.Stdout()\n\tfor {\n\t\tline, err := in.ReadLine(1 << 20)\n\t\tif err != nil {\n\t\t\tif sgo.IsEOF(err) {\n\t\t\t\treturn nil\n\t\t\t}\n\t\t\treturn err\n\t\t}\n\t\tcheck out.WriteString(one(line) + "\\n")\n\t}\n}\n'


CHAINS_GEN = '// Command chains issues root -> intermediate -> leaf chains with each key type and signature\n// algorithm Go\'s crypto/x509 signs with, plus tampered variants, and writes them in the\n// x509-limbo harness format with Go\'s verdicts.\npackage main\n\nimport (\n\t"crypto"\n\t"crypto/ecdsa"\n\t"crypto/ed25519"\n\t"crypto/elliptic"\n\t"crypto/rand"\n\t"crypto/rsa"\n\t"crypto/x509"\n\t"crypto/x509/pkix"\n\t"encoding/hex"\n\t"fmt"\n\t"math/big"\n\t"os"\n\t"strings"\n\t"time"\n)\n\ntype kind struct {\n\tname string\n\tkey  func() crypto.Signer\n\talgo x509.SignatureAlgorithm\n}\n\nfunc ec(c elliptic.Curve) func() crypto.Signer {\n\treturn func() crypto.Signer { k, _ := ecdsa.GenerateKey(c, rand.Reader); return k }\n}\n\nfunc rsaKey() crypto.Signer { k, _ := rsa.GenerateKey(rand.Reader, 2048); return k }\n\nfunc edKey() crypto.Signer { _, k, _ := ed25519.GenerateKey(rand.Reader); return k }\n\nfunc issue(tmpl, parent *x509.Certificate, pub crypto.PublicKey, signer crypto.Signer) []byte {\n\tder, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, signer)\n\tif err != nil {\n\t\tpanic(err)\n\t}\n\treturn der\n}\n\nfunc result(err error) string {\n\tif err == nil {\n\t\treturn "ok"\n\t}\n\treturn "err " + strings.ReplaceAll(err.Error(), "\\n", " ")\n}\n\nfunc main() {\n\tkinds := []kind{\n\t\t{"rsa-sha256", rsaKey, x509.SHA256WithRSA}, {"rsa-sha384", rsaKey, x509.SHA384WithRSA}, {"rsa-sha512", rsaKey, x509.SHA512WithRSA},\n\t\t{"rsapss-sha256", rsaKey, x509.SHA256WithRSAPSS}, {"rsapss-sha512", rsaKey, x509.SHA512WithRSAPSS},\n\t\t{"p224", ec(elliptic.P224()), x509.ECDSAWithSHA256}, {"p256", ec(elliptic.P256()), x509.ECDSAWithSHA256},\n\t\t{"p384", ec(elliptic.P384()), x509.ECDSAWithSHA384}, {"p521", ec(elliptic.P521()), x509.ECDSAWithSHA512},\n\t\t{"p521-sha256", ec(elliptic.P521()), x509.ECDSAWithSHA256}, {"ed25519", edKey, x509.PureEd25519},\n\t}\n\tcases, _ := os.Create(os.Args[1])\n\tgores, _ := os.Create(os.Args[2])\n\tdefer cases.Close()\n\tdefer gores.Close()\n\tnow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)\n\tserial := int64(1)\n\tfor _, k := range kinds {\n\t\trootKey, intKey, leafKey := k.key(), k.key(), k.key()\n\t\ttmpl := func(cn string, ca bool) *x509.Certificate {\n\t\t\tserial++\n\t\t\tt := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: cn},\n\t\t\t\tNotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), SignatureAlgorithm: k.algo,\n\t\t\t\tBasicConstraintsValid: true, IsCA: ca}\n\t\t\tif ca {\n\t\t\t\tt.KeyUsage = x509.KeyUsageCertSign\n\t\t\t} else {\n\t\t\t\tt.DNSNames = []string{"example.com"}\n\t\t\t\tt.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}\n\t\t\t}\n\t\t\treturn t\n\t\t}\n\t\trt := tmpl(k.name+" root", true)\n\t\trootDER := issue(rt, rt, rootKey.Public(), rootKey)\n\t\troot, _ := x509.ParseCertificate(rootDER)\n\t\tintDER := issue(tmpl(k.name+" int", true), root, intKey.Public(), rootKey)\n\t\tinter, _ := x509.ParseCertificate(intDER)\n\t\tleafDER := issue(tmpl("leaf", false), inter, leafKey.Public(), intKey)\n\t\tbad := append([]byte{}, leafDER...)\n\t\tbad[len(bad)-3] ^= 1\n\t\ttype variant struct {\n\t\t\tid    string\n\t\t\tleaf  []byte\n\t\t\tafter int64 // distrust-after for the root, Unix seconds; 0 for none\n\t\t}\n\t\tvs := []variant{{k.name + "::chain", leafDER, 0}, {k.name + "::tampered", bad, 0}}\n\t\tif k.name == "p256" || k.name == "rsa-sha256" {\n\t\t\tvs = append(vs, variant{k.name + "::distrust-after-later", leafDER, now.Add(-30 * time.Minute).Unix()},\n\t\t\t\tvariant{k.name + "::distrust-after-earlier", leafDER, now.Add(-2 * time.Hour).Unix()})\n\t\t}\n\t\tfor _, v := range vs {\n\t\t\tafterField := ""\n\t\t\tif v.after != 0 {\n\t\t\t\tafterField = fmt.Sprint(v.after)\n\t\t\t}\n\t\t\tfmt.Fprintf(cases, "%s\\t%s\\t%d\\t%s\\t%s\\t%s\\t%s\\t%s\\t%s\\n", v.id, "example.com", now.Unix(), "", "",\n\t\t\t\thex.EncodeToString(v.leaf), hex.EncodeToString(rootDER), hex.EncodeToString(intDER), afterField)\n\t\t\tc, err := x509.ParseCertificate(v.leaf)\n\t\t\tif err != nil {\n\t\t\t\tfmt.Fprintf(gores, "%s\\tparse %s\\n", v.id, result(err))\n\t\t\t\tcontinue\n\t\t\t}\n\t\t\troots, ints := x509.NewCertPool(), x509.NewCertPool()\n\t\t\tif v.after == 0 {\n\t\t\t\troots.AddCert(root)\n\t\t\t} else {\n\t\t\t\tdistrustAfter := time.Unix(v.after, 0).UTC()\n\t\t\t\troots.AddCertWithConstraint(root, func(chain []*x509.Certificate) error {\n\t\t\t\t\tfor _, c := range chain {\n\t\t\t\t\t\tif c.NotBefore.After(distrustAfter) {\n\t\t\t\t\t\t\treturn fmt.Errorf("certificate issued after distrust-after date %q", distrustAfter)\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n\t\t\t\t\treturn nil\n\t\t\t\t})\n\t\t\t}\n\t\t\tints.AddCert(inter)\n\t\t\t_, verr := c.Verify(x509.VerifyOptions{DNSName: "example.com", Roots: roots, Intermediates: ints, CurrentTime: now})\n\t\t\tfmt.Fprintf(gores, "%s\\tverify %s\\n", v.id, result(verr))\n\t\t}\n\t}\n}\n'


def limbo_dir():
    text = open(os.path.join(ROOT, "src/crypto/internal/cryptotest/x509limbo/schemaversion.go")).read()
    version = re.search(r'X509LimboVersion = "([^"]+)"', text).group(1)
    with tempfile.TemporaryDirectory() as d:
        open(os.path.join(d, "go.mod"), "w").write("module limbofetch\n\ngo 1.26\n")
        r = subprocess.run([GO, "mod", "download", "-json", "github.com/C2SP/x509-limbo@" + version],
                           cwd=d, capture_output=True, text=True, timeout=900)
        return json.loads(r.stdout)["Dir"]


def x509_limbo():
    """Run Go's x509-limbo case selection, and chains Go issues, through Go's crypto/x509 and std/crypto/x509."""
    with tempfile.TemporaryDirectory() as d:
        gd, sd = os.path.join(d, "gen"), os.path.join(d, "harness")
        os.makedirs(gd)
        os.makedirs(sd)
        open(os.path.join(gd, "main.go"), "w").write(LIMBO_GEN)
        open(os.path.join(gd, "go.mod"), "w").write("module limbogen\n\ngo 1.26\n")
        cases, gores = os.path.join(d, "cases.txt"), os.path.join(d, "go.txt")
        env = dict(os.environ, GOFLAGS="-mod=mod")
        g = subprocess.run([GO, "run", ".", os.path.join(limbo_dir(), "limbo.json"), cases, gores],
                           cwd=gd, env=env, capture_output=True, text=True, timeout=1800)
        if g.returncode:
            return False, "x509-limbo: go exit %d\n%s" % (g.returncode, g.stderr[-1500:])
        # Issued chains for every key type and signature algorithm Go signs with, tampered too.
        cd = os.path.join(d, "chains")
        os.makedirs(cd)
        open(os.path.join(cd, "main.go"), "w").write(CHAINS_GEN)
        open(os.path.join(cd, "go.mod"), "w").write("module chains\n\ngo 1.26\n")
        cc, cg = os.path.join(d, "chains-cases.txt"), os.path.join(d, "chains-go.txt")
        g = subprocess.run([GO, "run", ".", cc, cg], cwd=cd, env=env, capture_output=True, text=True, timeout=900)
        if g.returncode:
            return False, "x509-limbo: chains go exit %d\n%s" % (g.returncode, g.stderr[-1500:])
        with open(cases, "a") as out:
            out.write(open(cc).read())
        with open(gores, "a") as out:
            out.write(open(cg).read())
        open(os.path.join(sd, "main.sgo"), "w").write(LIMBO_SGO)
        open(os.path.join(sd, "sgo.mod"), "w").write(f"module example.com/limbo\n\nsgo {VERSION}\n")
        b = subprocess.run([SGO, "build", "-o", os.path.join(d, "limbo"), "."], cwd=sd, capture_output=True, text=True, timeout=900)
        if b.returncode:
            return False, "x509-limbo: sgo build exit %d\n%s" % (b.returncode, (b.stdout + b.stderr)[-1500:])
        with open(cases) as inp:
            s = subprocess.run([os.path.join(d, "limbo")], stdin=inp, capture_output=True, text=True, timeout=1800)
        gl, sl = open(gores).read().splitlines(), s.stdout.splitlines()
        bad = [i for i in range(max(len(gl), len(sl))) if i >= len(gl) or i >= len(sl) or gl[i] != sl[i]]
        msg = "x509-limbo: %d cases, %d differ" % (len(gl), len(bad))
        if s.returncode:
            msg += " (sgo exit %d: %s)" % (s.returncode, s.stderr[-800:])
        if bad:
            i = bad[0]
            msg += "\n  go : %r\n  sgo: %r" % (gl[i] if i < len(gl) else None, sl[i] if i < len(sl) else None)
        return not bad and s.returncode == 0 and len(gl) > 0, msg


ROOTS_GO = """package main

import (
	"crypto/x509"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/x509roots/fallback/bundle"
)

func main() {
	p := x509.NewCertPool()
	n := 0
	for r := range bundle.Roots() {
		c, err := x509.ParseCertificate(r.Certificate)
		if err != nil {
			panic(err)
		}
		if r.Constraint == nil {
			p.AddCert(c)
		} else {
			p.AddCertWithConstraint(c, r.Constraint)
		}
		n++
	}
	fmt.Println("roots", n)
	for _, s := range p.Subjects() {
		fmt.Println(hex.EncodeToString(s))
	}
}
"""

ROOTS_SGO = """package main

import (
	"sgo"
	"std/crypto/x509/roots"
	"std/encoding/hex"
	"std/strconv"
)

func main(root *sgo.Context) error {
	p := roots.Mozilla()
	out := "roots " + strconv.Itoa(roots.Count) + "\\n"
	s := p.Subjects()
	for i := 0; i < len(s); i++ {
		out += hex.EncodeToString(s[i]) + "\\n"
	}
	return root.Stdout().WriteString(out)
}
"""


def x509_roots():
    """std/crypto/x509/roots against Go's fallback pool built from the same pinned bundle."""
    chk = subprocess.run([sys.executable, os.path.join(ROOT, "sgo", "scripts", "gen-roots.py"), "--check"],
                         capture_output=True, text=True, timeout=900)
    if chk.returncode:
        return False, "x509-roots: " + (chk.stdout + chk.stderr).strip()
    version = re.search(r'VERSION = "([^"]+)"', open(os.path.join(ROOT, "sgo", "scripts", "gen-roots.py")).read()).group(1)
    with tempfile.TemporaryDirectory() as d:
        open(os.path.join(d, "main.go"), "w").write(ROOTS_GO)
        open(os.path.join(d, "go.mod"), "w").write("module rootsdiff\n\ngo 1.26\n\nrequire golang.org/x/crypto/x509roots/fallback " + version + "\n")
        g = subprocess.run([GO, "run", "."], cwd=d, env=dict(os.environ, GOFLAGS="-mod=mod"), capture_output=True, text=True, timeout=900)
    s = run_sgo(ROOTS_SGO)
    if g.returncode or s.returncode:
        return False, "x509-roots: go exit %d, sgo exit %d\n%s%s" % (g.returncode, s.returncode, g.stderr[-800:], s.stderr[-800:])
    same = g.stdout == s.stdout
    return same, "x509-roots: %s, %d subjects %s" % (version, len(g.stdout.splitlines()) - 1, "identical" if same else "DIFFER")


def main():
    seed = 1
    if "--seed" in sys.argv:
        seed = int(sys.argv[sys.argv.index("--seed") + 1])
    failed = False
    for name, gen in (("bits", gen_bits), ("encoding", gen_encoding), ("sha256", gen_sha256), ("sha512", gen_sha512), ("ed25519", gen_ed25519), ("bigmod", gen_bigmod), ("aes", gen_aes), ("asn1", gen_asn1)):
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
    for name, gen in (("hmac+hkdf", wyche_hmac_hkdf), ("chacha20poly1305", wyche_chacha20poly1305), ("x25519", wyche_x25519), ("ecdsa", wyche_ecdsa), ("aes-gcm", wyche_aes_gcm), ("rsa", wyche_rsa), ("ed25519", wyche_ed25519)):
        r = run_sgo(gen(WP))
        line = r.stdout.strip()
        print("test-std-crypto: " + (line or f"{name}: exit {r.returncode}\n{r.stderr[-1500:]}"))
        m = re.search(r": (\d+) of (\d+) agree", line)
        tail = line.split(" agree", 1)[1] if " agree" in line else ""
        if r.returncode or not m or m.group(1) != m.group(2) or "#" in tail or "base: false" in tail or "test 1: false" in tail:
            failed = True
    for stage in (x509_limbo, x509_roots):
        ok, msg = stage()
        print("test-std-crypto: " + msg)
        if not ok:
            failed = True
    if failed:
        sys.exit("test-std-crypto: FAIL")
    print("test-std-crypto: ok")


if __name__ == "__main__":
    main()
