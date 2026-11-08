#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""test-tls.py - std/crypto/tls against Go's crypto/tls server (SGO-STD-031).

For each case, builds a Go TLS 1.3 server with the pinned bootstrap toolchain, which makes a CA
and a leaf certificate for tls.test with the case's key type and listens on loopback; then runs
an S-Go client that trusts the CA, handshakes, writes a line, and reads the reply. Successful
cases must agree with the server on the cipher suite, the ALPN protocol and the data; failing
cases must report Go's client error, and the Go server must see Go's alert.

    test-tls.py [case ...]
"""

import os
import subprocess
import sys
import tempfile

ROOT = os.path.realpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
sys.path.insert(0, os.path.join(ROOT, "sgo", "scripts"))
VERSION = open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip()
PORT = 47990

SERVER = r"""// Command server is a Go crypto/tls TLS 1.3 server for the S-Go tls differential test.
// Usage: server -key ecdsa256|ecdsa384|ed25519|rsa -addr 127.0.0.1:N -cadir DIR [-alpn h2,x] [-clientauth] [-big]
package main

import (
	"bufio"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func key(kind string) crypto.Signer {
	switch kind {
	case "ecdsa384":
		k, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		return k
	case "ed25519":
		_, k, _ := ed25519.GenerateKey(rand.Reader)
		return k
	case "rsa":
		k, _ := rsa.GenerateKey(rand.Reader, 2048)
		return k
	}
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	return k
}

func main() {
	kind := flag.String("key", "ecdsa256", "leaf key type")
	addr := flag.String("addr", "127.0.0.1:0", "listen address")
	cadir := flag.String("cadir", ".", "where to write ca.der and now")
	alpn := flag.String("alpn", "", "ALPN protocols")
	clientAuth := flag.Bool("clientauth", false, "request a client certificate")
	bigReply := flag.Bool("big", false, "reply with 100 KiB")
	flag.Parse()
	now := time.Now()
	caKey := key("ecdsa256")
	caT := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "S-Go test CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caT, caT, caKey.Public(), caKey)
	if err != nil {
		panic(err)
	}
	ca, _ := x509.ParseCertificate(caDER)
	lk := key(*kind)
	lT := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "tls.test"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), DNSNames: []string{"tls.test"},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, KeyUsage: x509.KeyUsageDigitalSignature}
	lDER, err := x509.CreateCertificate(rand.Reader, lT, ca, lk.Public(), caKey)
	if err != nil {
		panic(err)
	}
	os.WriteFile(filepath.Join(*cadir, "ca.der"), caDER, 0o644)
	os.WriteFile(filepath.Join(*cadir, "now"), []byte(fmt.Sprint(now.Unix())), 0o644)
	cfg := &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{{Certificate: [][]byte{lDER}, PrivateKey: lk}}}
	if *alpn != "" {
		cfg.NextProtos = strings.Split(*alpn, ",")
	}
	if *clientAuth {
		cfg.ClientAuth = tls.RequestClientCert
	}
	ln, err := tls.Listen("tcp", *addr, cfg)
	if err != nil {
		panic(err)
	}
	fmt.Println("ready")
	c, err := ln.Accept()
	if err != nil {
		panic(err)
	}
	ln.Close()
	tc := c.(*tls.Conn)
	c.SetDeadline(time.Now().Add(20 * time.Second))
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		fmt.Println("server error:", err)
		os.Exit(1)
	}
	st := tc.ConnectionState()
	reply := strings.ToUpper(line)
	if *bigReply {
		reply = strings.Repeat("abcdefghijklmnop", 6400) + "\n"
	}
	c.Write([]byte(reply))
	fmt.Printf("server: %s alpn=%q got=%q\n", tls.CipherSuiteName(st.CipherSuite), st.NegotiatedProtocol, line)
	c.Close()
}
"""

def tool_paths():
    import json
    import platform
    goos = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
    goarch = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[platform.machine()]
    sgo = os.path.join(ROOT, "pkg", "tool", f"{goos}_{goarch}", "sgo")
    cache = os.environ.get("SGO_CACHE", os.path.expanduser("~/.cache/sgolang"))
    pinned = json.load(open(os.path.join(ROOT, "sgo", "UPSTREAM.lock.json")))["bootstrap"]["pinned_version"]
    go = os.path.join(cache, "bootstrap", f"{pinned}-{goos}-{goarch}", "go", "bin", "go")
    return sgo, go


SGO, GO = tool_paths()

CLIENT = """package main

import (
	"sgo"
	"std/crypto/tls"
	"std/crypto/x509"
	"std/encoding/hex"
	"std/strconv"
)

func main(root *sgo.Context) error {
	der, _ := hex.DecodeString("%(ca)s")
	ca, perr := x509.ParseCertificate(der)
	if perr != nil {
		return perr
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	c, err := root.Dial("127.0.0.1:%(port)d")
	if err != nil {
		return err
	}
	cfg := tls.Config{ServerName: "%(name)s", RootCAs: pool, Time: %(now)d, NextProtos: []string{%(alpn)s}, CipherSuites: []int{%(suites)s}}
	t, herr := tls.Client(c, root.Entropy(), cfg)
	if herr != nil {
		_ = t.Close()
		return root.Stdout().WriteString("client error: " + herr.Error() + "\\n")
	}
	if _, werr := t.Write("hello tls\\n"); werr != nil {
		return werr
	}
	got := ""
	for len(got) == 0 || got[len(got)-1] != '\\n' {
		s, rerr := t.Read(65536)
		if rerr != nil {
			return root.Stdout().WriteString("read error: " + rerr.Error() + "\\n")
		}
		got = got + s
	}
	if len(got) > 40 {
		got = "<" + strconv.Itoa(len(got)) + " bytes>"
	} else {
		got = got[:len(got)-1]
	}
	_ = t.Close()
	return root.Stdout().WriteString("client: " + t.CipherSuite() + " alpn=" + t.NegotiatedProtocol() + " got=" + got + "\\n")
}
"""

# name, server key, server ALPN, client ALPN, server flags, client server name, time offset,
# client suites, expected client line (prefix), expected server line (prefix)
CASES = [
    ("ecdsa-p256", "ecdsa256", "", "", [], "tls.test", 0, "",
     "client: TLS_AES_128_GCM_SHA256 alpn= got=HELLO TLS", 'server: TLS_AES_128_GCM_SHA256 alpn="" got="hello tls\\n"'),
    ("ecdsa-p384", "ecdsa384", "", "", [], "tls.test", 0, "",
     "client: TLS_AES_128_GCM_SHA256 alpn= got=HELLO TLS", "server: TLS_AES_128_GCM_SHA256"),
    ("ed25519", "ed25519", "", "", [], "tls.test", 0, "",
     "client: TLS_AES_128_GCM_SHA256 alpn= got=HELLO TLS", "server: TLS_AES_128_GCM_SHA256"),
    ("rsa-pss", "rsa", "", "", [], "tls.test", 0, "",
     "client: TLS_AES_128_GCM_SHA256 alpn= got=HELLO TLS", "server: TLS_AES_128_GCM_SHA256"),
    ("chacha20", "ecdsa256", "", "", [], "tls.test", 0, "0x1303",
     "client: TLS_CHACHA20_POLY1305_SHA256 alpn= got=HELLO TLS", "server: TLS_CHACHA20_POLY1305_SHA256"),
    ("aes256", "rsa", "", "", [], "tls.test", 0, "0x1302",
     "client: TLS_AES_256_GCM_SHA384 alpn= got=HELLO TLS", "server: TLS_AES_256_GCM_SHA384"),
    ("alpn", "ecdsa256", "h2,http/1.1", "http/1.1,h2", [], "tls.test", 0, "",
     "client: TLS_AES_128_GCM_SHA256 alpn=h2 got=HELLO TLS", 'server: TLS_AES_128_GCM_SHA256 alpn="h2"'),
    ("clientauth", "ecdsa256", "", "", ["-clientauth"], "tls.test", 0, "",
     "client: TLS_AES_128_GCM_SHA256 alpn= got=HELLO TLS", "server: TLS_AES_128_GCM_SHA256"),
    ("big-reply", "ed25519", "", "", ["-big"], "tls.test", 0, "",
     "client: TLS_AES_128_GCM_SHA256 alpn= got=<102401 bytes>", "server: TLS_AES_128_GCM_SHA256"),
    ("wrong-name", "ecdsa256", "", "", [], "other.test", 0, "",
     "client error: tls: failed to verify certificate: x509: certificate is valid for tls.test, not other.test",
     "server error: remote error: tls: bad certificate"),
    ("expired", "ecdsa256", "", "", [], "tls.test", 48 * 3600, "",
     "client error: tls: failed to verify certificate: x509: certificate has expired or is not yet valid",
     "server error: remote error: tls: bad certificate"),
]


def build_server(d):
    src = os.path.join(d, "server")
    os.makedirs(src)
    open(os.path.join(src, "main.go"), "w").write(SERVER)
    open(os.path.join(src, "go.mod"), "w").write("module tlsserver\n\ngo 1.26\n")
    exe = os.path.join(d, "tlsserver")
    env = dict(os.environ, GOTOOLCHAIN="local")
    r = subprocess.run([GO, "build", "-o", exe, "."], cwd=src, env=env, capture_output=True, text=True, timeout=600)
    if r.returncode:
        sys.exit("test-tls: building the Go server failed:\n" + r.stdout + r.stderr)
    return exe


def run_case(exe, d, case):
    name, key, salpn, calpn, flags, sname, off, suites, _, _ = case
    cd = os.path.join(d, name)
    os.makedirs(cd)
    srv = subprocess.Popen([exe, "-key", key, "-addr", "127.0.0.1:%d" % PORT, "-cadir", cd, "-alpn", salpn] + flags,
                           stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    if srv.stdout.readline().strip() != "ready":
        srv.kill()
        return "", "server did not start"
    ca = open(os.path.join(cd, "ca.der"), "rb").read().hex()
    now = int(open(os.path.join(cd, "now")).read()) + off
    m = os.path.join(cd, "client")
    os.makedirs(m)
    alpn = ", ".join('"%s"' % p for p in calpn.split(",") if p)
    open(os.path.join(m, "main.sgo"), "w").write(CLIENT % dict(ca=ca, port=PORT, name=sname, now=now, alpn=alpn, suites=suites))
    open(os.path.join(m, "sgo.mod"), "w").write(
        "module example.com/tlsclient\n\nsgo %s\ngrant connect 127.0.0.1:%d\ngrant entropy\n" % (VERSION, PORT))
    try:
        cl = subprocess.run([SGO, "run", "."], cwd=m, capture_output=True, text=True, timeout=600)
        cout = (cl.stdout + cl.stderr).strip()
    except subprocess.TimeoutExpired:
        cout = "client timed out"
    try:
        sout, _ = srv.communicate(timeout=30)
    except subprocess.TimeoutExpired:
        srv.kill()
        sout, _ = srv.communicate()
    return cout, sout.strip()


# --- Hostile servers -------------------------------------------------------------------------
# A Python server answers the ClientHello with crafted bytes; Go's crypto/tls client and the
# S-Go client run against each scenario, and their errors and the alert each sends must match.

HOSTILE_GO = """package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"
)

func main() {
	cfg := &tls.Config{ServerName: "tls.test", RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519}}
	d := &tls.Dialer{Config: cfg}
	_ = time.Second
	c, err := d.Dial("tcp", os.Args[1])
	if err != nil {
		fmt.Println("client error: " + err.Error())
		return
	}
	c.Close()
	fmt.Println("handshake ok")
}
"""

HOSTILE_SGO = """package main

import (
	"sgo"
	"std/crypto/tls"
	"std/crypto/x509"
)

func main(root *sgo.Context) error {
	c, err := root.Dial("127.0.0.1:%(port)d")
	if err != nil {
		return err
	}
	cfg := tls.Config{ServerName: "tls.test", RootCAs: x509.NewCertPool(), Time: 0, NextProtos: []string{}, CipherSuites: []int{}}
	t, herr := tls.Client(c, root.Entropy(), cfg)
	_ = t.Close()
	if herr != nil {
		return root.Stdout().WriteString("client error: " + herr.Error() + "\\n")
	}
	return root.Stdout().WriteString("handshake ok\\n")
}
"""

HRR_RANDOM = bytes.fromhex("cf21ad74e59a6111be1d8c021e65b891c2a211167abb8c5e079e09e2c8a8339c")


def u8(v):
    return bytes([v])


def u16(v):
    return bytes([(v >> 8) & 0xff, v & 0xff])


def u24(v):
    return bytes([(v >> 16) & 0xff, (v >> 8) & 0xff, v & 0xff])


def record(typ, payload):
    return bytes([typ, 3, 3]) + u16(len(payload)) + payload


def hs(typ, body):
    return u8(typ) + u24(len(body)) + body


def ext(typ, data):
    return u16(typ) + u16(len(data)) + data


def sv(v=0x0304):
    return ext(43, u16(v))


def ks(group=0x001d, key=None):
    key = os.urandom(32) if key is None else key
    return ext(51, u16(group) + u16(len(key)) + key)


def sh(sid, suite=0x1301, legacy=0x0303, comp=0, random=None, exts=None):
    random = os.urandom(32) if random is None else random
    exts = sv() + ks() if exts is None else exts
    body = u16(legacy) + random + u8(len(sid)) + sid + u16(suite) + u8(comp) + u16(len(exts)) + exts
    return record(22, hs(2, body))


def hrr(sid, suite=0x1301, exts=None):
    exts = sv() + ext(44, u16(4) + b"ckie") if exts is None else exts
    return sh(sid, suite=suite, random=HRR_RANDOM, exts=exts)


# name -> (first reply, second reply or None); each is a function of the ClientHello session ID.
SCENARIOS = [
    ("sh-tls12", lambda s: sh(s, exts=ks()), None),
    ("sh-version-302", lambda s: sh(s, exts=sv(0x0302) + ks()), None),
    ("sh-legacy-version", lambda s: sh(s, legacy=0x0301), None),
    ("sh-session-id", lambda s: sh(b"\x00" * 32), None),
    ("sh-compression", lambda s: sh(s, comp=1), None),
    ("sh-unoffered-suite", lambda s: sh(s, suite=0x1304), None),
    ("sh-tls12-suite", lambda s: sh(s, suite=0xc02f), None),
    ("sh-no-key-share", lambda s: sh(s, exts=sv()), None),
    ("sh-wrong-group", lambda s: sh(s, exts=sv() + ks(0x0017, b"\x04" + os.urandom(64))), None),
    ("sh-cookie", lambda s: sh(s, exts=sv() + ks() + ext(44, u16(4) + b"ckie")), None),
    ("sh-low-order-key", lambda s: sh(s, exts=sv() + ks(key=b"\x00" * 32)), None),
    ("sh-short-key", lambda s: sh(s, exts=sv() + ks(key=os.urandom(31))), None),
    ("sh-duplicate-ext", lambda s: sh(s, exts=sv() + sv() + ks()), None),
    ("sh-truncated", lambda s: record(22, hs(2, b"\x03\x03")), None),
    ("hrr-unnecessary", lambda s: hrr(s, exts=sv()), None),
    ("hrr-x25519", lambda s: hrr(s, exts=sv() + ext(51, u16(0x001d))), None),
    ("hrr-unsupported-group", lambda s: hrr(s, exts=sv() + ext(51, u16(0x0017))), None),
    ("hrr-twice", lambda s: hrr(s), lambda s: hrr(s)),
    ("hrr-changed-suite", lambda s: hrr(s, suite=0x1301), lambda s: sh(s, suite=0x1302)),
    ("hrr-changed-version", lambda s: hrr(s), lambda s: sh(s, exts=sv(0x0303) + ks())),
    ("ee-first", lambda s: record(22, hs(8, u16(0))), None),
    ("alert", lambda s: record(21, b"\x02\x28"), None),
    ("appdata-first", lambda s: record(23, b"x"), None),
    ("bad-record-type", lambda s: record(99, b"\x00"), None),
    ("record-overflow", lambda s: bytes([22, 3, 3, 0x50, 0x00]), None),
    ("bad-ccs", lambda s: record(20, b"\x02") + sh(s), None),
]


def read_record(conn):
    def full(n):
        b = b""
        while len(b) < n:
            chunk = conn.recv(n - len(b))
            if not chunk:
                raise EOFError
            b += chunk
        return b
    h = full(5)
    return h[0], full((h[3] << 8) | h[4])


def hostile_server(sock, first, second, result):
    import socket
    conn, _ = sock.accept()
    conn.settimeout(15)
    alerts = []
    try:
        typ, ch = read_record(conn)
        sid = ch[4 + 2 + 32 + 1:4 + 2 + 32 + 1 + ch[4 + 2 + 32]]
        conn.sendall(first(sid))
        if second is not None:
            while True:
                typ, data = read_record(conn)
                if typ == 21:
                    alerts.append(data[1])
                if typ == 22:
                    break
            conn.sendall(second(sid))
        while True:
            typ, data = read_record(conn)
            if typ == 21 and len(data) == 2:
                alerts.append(data[1])
    except (EOFError, ConnectionError, socket.timeout, OSError):
        pass
    conn.close()
    result.append(alerts)


def run_hostile(client, first, second):
    import socket
    import threading
    sock = socket.socket()
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind(("127.0.0.1", PORT))
    sock.listen(1)
    result = []
    th = threading.Thread(target=hostile_server, args=(sock, first, second, result))
    th.start()
    try:
        r = subprocess.run(client, capture_output=True, text=True, timeout=120)
        out = (r.stdout + r.stderr).strip()
    except subprocess.TimeoutExpired:
        out = "client timed out"
    th.join(30)
    sock.close()
    return out, (result[0] if result else None)


def build_hostile_clients(d):
    gdir = os.path.join(d, "hgo")
    os.makedirs(gdir)
    open(os.path.join(gdir, "main.go"), "w").write(HOSTILE_GO)
    open(os.path.join(gdir, "go.mod"), "w").write("module hostile\n\ngo 1.26\n")
    gexe = os.path.join(d, "hostile-go")
    r = subprocess.run([GO, "build", "-o", gexe, "."], cwd=gdir, env=dict(os.environ, GOTOOLCHAIN="local"),
                       capture_output=True, text=True, timeout=600)
    if r.returncode:
        sys.exit("test-tls: building the Go client failed:\n" + r.stdout + r.stderr)
    sdir = os.path.join(d, "hsgo")
    os.makedirs(sdir)
    open(os.path.join(sdir, "main.sgo"), "w").write(HOSTILE_SGO % dict(port=PORT))
    open(os.path.join(sdir, "sgo.mod"), "w").write(
        "module example.com/hostile\n\nsgo %s\ngrant connect 127.0.0.1:%d\ngrant entropy\n" % (VERSION, PORT))
    sexe = os.path.join(d, "hostile-sgo")
    r = subprocess.run([SGO, "build", "-o", sexe, "."], cwd=sdir, capture_output=True, text=True, timeout=600)
    if r.returncode:
        sys.exit("test-tls: building the S-Go client failed:\n" + r.stdout + r.stderr)
    return gexe, sexe


def hostile(d, only):
    gexe, sexe = build_hostile_clients(d)
    bad = 0
    for name, first, second in SCENARIOS:
        if only and name not in only:
            continue
        gout, galerts = run_hostile([gexe, "127.0.0.1:%d" % PORT], first, second)
        sout, salerts = run_hostile([sexe], first, second)
        ok = gout == sout and galerts == salerts and gout.startswith("client error")
        print("%-22s %s" % (name, "ok" if ok else "FAIL"))
        if not ok:
            bad += 1
            print("  go:   %s alerts=%s\n  s-go: %s alerts=%s" % (gout, galerts, sout, salerts))
    return bad


# --- Hostile encrypted flights --------------------------------------------------------------
# A proxy in front of Go's server decrypts the server's encrypted handshake flight with the
# server's key log, applies a mutation, and re-encrypts it; Go's client and the S-Go client
# must give the same output, and the Go server must see the same result from both.

MITM = r"""// Command mitm runs a Go crypto/tls TLS 1.3 server behind a proxy that decrypts the server's
// handshake flight with the server's key log, applies a mutation, and re-encrypts it, for the
// S-Go tls hostile tests (test-tls.py). Usage:
//
//	mitm -mode M -addr 127.0.0.1:N -cadir DIR
package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type keyLog struct {
	mu   sync.Mutex
	vals map[string][]byte
}

func (k *keyLog) Write(p []byte) (int, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, line := range strings.Split(strings.TrimSpace(string(p)), "\n") {
		f := strings.Fields(line)
		if len(f) == 3 {
			v, _ := hex.DecodeString(f[2])
			k.vals[f[0]] = v
		}
	}
	return len(p), nil
}

func (k *keyLog) get(label string) []byte {
	for i := 0; i < 500; i++ {
		k.mu.Lock()
		v := k.vals[label]
		k.mu.Unlock()
		if v != nil {
			return v
		}
		time.Sleep(10 * time.Millisecond)
	}
	panic("no key log entry " + label)
}

func expandLabel(secret []byte, label string, n int) []byte {
	full := "tls13 " + label
	info := []byte{byte(n >> 8), byte(n), byte(len(full))}
	info = append(info, full...)
	info = append(info, 0)
	out, err := hkdf.Expand(sha256.New, secret, string(info), n)
	if err != nil {
		panic(err)
	}
	return out
}

// half is one direction of AES-128-GCM record protection.
type half struct {
	aead   cipher.AEAD
	iv     []byte
	seq    uint64
	secret []byte
}

func newHalf(secret []byte) *half {
	b, _ := aes.NewCipher(expandLabel(secret, "key", 16))
	a, _ := cipher.NewGCM(b)
	return &half{aead: a, iv: expandLabel(secret, "iv", 12), secret: secret}
}

func (h *half) nonce() []byte {
	n := append([]byte{}, h.iv...)
	for i := 0; i < 8; i++ {
		n[11-i] ^= byte(h.seq >> (8 * i))
	}
	h.seq++
	return n
}

func (h *half) open(rec []byte) (byte, []byte) {
	p, err := h.aead.Open(nil, h.nonce(), rec[5:], rec[:5])
	if err != nil {
		panic(err)
	}
	i := len(p) - 1
	for p[i] == 0 {
		i--
	}
	return p[i], p[:i]
}

func (h *half) seal(typ byte, p []byte) []byte {
	inner := append(append([]byte{}, p...), typ)
	n := len(inner) + 16
	hdr := []byte{23, 3, 3, byte(n >> 8), byte(n)}
	return h.aead.Seal(hdr, h.nonce(), inner, hdr)
}

func readRecord(r io.Reader) ([]byte, error) {
	hdr := make([]byte, 5)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, err
	}
	body := make([]byte, int(hdr[3])<<8|int(hdr[4]))
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return append(hdr, body...), nil
}

func hsMsg(typ byte, body []byte) []byte {
	return append([]byte{typ, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}, body...)
}

func find(msgs [][]byte, typ byte) int {
	for i, m := range msgs {
		if m[0] == typ {
			return i
		}
	}
	panic(fmt.Sprintf("no message of type %d", typ))
}

// mutate rewrites the server's encrypted handshake messages; it returns the messages and the
// size of the records to send them in (0: one record).
func mutate(mode string, msgs [][]byte) ([][]byte, int) {
	flip := func(m []byte, i int) { m[i] ^= 0x40 }
	switch mode {
	case "cv-sig":
		m := msgs[find(msgs, 15)]
		flip(m, len(m)-1)
	case "cv-scheme":
		m := msgs[find(msgs, 15)]
		m[4], m[5] = 0x08, 0x07
	case "finished":
		m := msgs[find(msgs, 20)]
		flip(m, len(m)-1)
	case "cert-sig":
		m := msgs[find(msgs, 11)]
		n := int(m[8])<<16 | int(m[9])<<8 | int(m[10])
		flip(m, 11+n-1)
	case "cert-garbage":
		m := msgs[find(msgs, 11)]
		m[11] = 0x31
	case "empty-certs":
		msgs[find(msgs, 11)] = hsMsg(11, []byte{0, 0, 0, 0})
	case "dup-ee":
		i := find(msgs, 8)
		msgs = append(msgs[:i+1], append([][]byte{msgs[i]}, msgs[i+1:]...)...)
	case "no-cv":
		i := find(msgs, 15)
		msgs = append(msgs[:i], msgs[i+1:]...)
	case "ee-alpn":
		alpn := []byte{0, 16, 0, 5, 0, 3, 2, 'h', '2'}
		msgs[find(msgs, 8)] = hsMsg(8, append([]byte{0, byte(len(alpn))}, alpn...))
	case "fragmented":
		return msgs, 7
	}
	return msgs, 0
}

func proxy(mode string, client net.Conn, serverAddr string, kl *keyLog, wg *sync.WaitGroup) {
	defer wg.Done()
	server, err := net.Dial("tcp", serverAddr)
	if err != nil {
		panic(err)
	}
	go func() { io.Copy(server, client); server.(*net.TCPConn).CloseWrite() }()
	defer client.Close()
	var hs *half
	var msgs [][]byte
	var buf []byte
	sentFlight := false
	var app, appOut *half
	for {
		rec, err := readRecord(server)
		if err != nil {
			return
		}
		if rec[0] != 23 {
			client.Write(rec)
			continue
		}
		if !sentFlight {
			if hs == nil {
				hs = newHalf(kl.get("SERVER_HANDSHAKE_TRAFFIC_SECRET"))
			}
			_, p := hs.open(rec)
			buf = append(buf, p...)
			for len(buf) >= 4 && len(buf) >= 4+(int(buf[1])<<16|int(buf[2])<<8|int(buf[3])) {
				n := 4 + (int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3]))
				msgs = append(msgs, append([]byte{}, buf[:n]...))
				buf = buf[n:]
			}
			if len(msgs) == 0 || msgs[len(msgs)-1][0] != 20 {
				continue
			}
			out, size := mutate(mode, msgs)
			stream := bytes.Join(out, nil)
			enc := newHalf(hs.secret)
			var wire []byte
			if size == 0 {
				size = len(stream)
			}
			for len(stream) > 0 {
				k := size
				if k > len(stream) {
					k = len(stream)
				}
				wire = append(wire, enc.seal(22, stream[:k])...)
				stream = stream[k:]
			}
			if mode == "record-mac" {
				wire[len(wire)-1] ^= 1
			}
			client.Write(wire)
			sentFlight = true
			continue
		}
		if mode != "keyupdate" {
			client.Write(rec)
			continue
		}
		if app == nil {
			app = newHalf(kl.get("SERVER_TRAFFIC_SECRET_0"))
			appOut = newHalf(app.secret)
			client.Write(appOut.seal(22, hsMsg(24, []byte{1})))
			appOut = newHalf(expandLabel(app.secret, "traffic upd", 32))
		}
		typ, p := app.open(rec)
		client.Write(appOut.seal(typ, p))
	}
}

func main() {
	mode := flag.String("mode", "", "mutation")
	addr := flag.String("addr", "127.0.0.1:0", "proxy address")
	cadir := flag.String("cadir", ".", "where to write ca.der and now")
	flag.Parse()
	now := time.Now()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caT := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "S-Go test CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign}
	caDER, _ := x509.CreateCertificate(rand.Reader, caT, caT, caKey.Public(), caKey)
	ca, _ := x509.ParseCertificate(caDER)
	lk, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	lT := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "tls.test"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), DNSNames: []string{"tls.test"},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, KeyUsage: x509.KeyUsageDigitalSignature}
	lDER, _ := x509.CreateCertificate(rand.Reader, lT, ca, lk.Public(), caKey)
	os.WriteFile(filepath.Join(*cadir, "ca.der"), caDER, 0o644)
	os.WriteFile(filepath.Join(*cadir, "now"), []byte(fmt.Sprint(now.Unix())), 0o644)
	kl := &keyLog{vals: map[string][]byte{}}
	cfg := &tls.Config{MinVersion: tls.VersionTLS13, KeyLogWriter: kl, CipherSuites: nil,
		Certificates: []tls.Certificate{{Certificate: [][]byte{lDER}, PrivateKey: lk}}}
	sl, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	if err != nil {
		panic(err)
	}
	pl, err := net.Listen("tcp", *addr)
	if err != nil {
		panic(err)
	}
	fmt.Println("ready")
	var wg sync.WaitGroup
	go func() {
		c, err := pl.Accept()
		if err != nil {
			return
		}
		pl.Close()
		wg.Add(1)
		proxy(*mode, c, sl.Addr().String(), kl, &wg)
	}()
	c, err := sl.Accept()
	if err != nil {
		panic(err)
	}
	sl.Close()
	tc := c.(*tls.Conn)
	c.SetDeadline(time.Now().Add(20 * time.Second))
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		fmt.Println("server error:", err)
		c.Close()
		wg.Wait()
		return
	}
	st := tc.ConnectionState()
	c.Write([]byte(strings.ToUpper(line)))
	fmt.Printf("server: %s alpn=%q got=%q\n", tls.CipherSuiteName(st.CipherSuite), st.NegotiatedProtocol, line)
	c.Close()
	wg.Wait()
}
"""

GOCLI = r"""// Command gocli is Go's crypto/tls client for the S-Go tls tests; its output has the format of
// test-tls.py's S-Go client. Usage: gocli ADDR CA.der
package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
)

func main() {
	der, _ := os.ReadFile(os.Args[2])
	ca, err := x509.ParseCertificate(der)
	if err != nil {
		panic(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	cfg := &tls.Config{ServerName: "tls.test", RootCAs: pool, MinVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519}}
	c, err := tls.Dial("tcp", os.Args[1], cfg)
	if err != nil {
		fmt.Println("client error: " + err.Error())
		return
	}
	defer c.Close()
	if _, err := c.Write([]byte("hello tls\n")); err != nil {
		fmt.Println("write error: " + err.Error())
		return
	}
	got, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		fmt.Println("read error: " + err.Error())
		return
	}
	st := c.ConnectionState()
	fmt.Println("client: " + tls.CipherSuiteName(st.CipherSuite) + " alpn=" + st.NegotiatedProtocol + " got=" + strings.TrimSuffix(got, "\n"))
}
"""

MITM_MODES = ["none", "fragmented", "keyupdate", "cv-sig", "cv-scheme", "finished", "cert-sig",
              "cert-garbage", "empty-certs", "dup-ee", "no-cv", "ee-alpn", "record-mac"]


def go_build(d, name, src):
    sd = os.path.join(d, name + "-src")
    os.makedirs(sd)
    open(os.path.join(sd, "main.go"), "w").write(src)
    open(os.path.join(sd, "go.mod"), "w").write("module " + name + "\n\ngo 1.26\n")
    exe = os.path.join(d, name)
    r = subprocess.run([GO, "build", "-o", exe, "."], cwd=sd, env=dict(os.environ, GOTOOLCHAIN="local"),
                       capture_output=True, text=True, timeout=600)
    if r.returncode:
        sys.exit("test-tls: building %s failed:\n%s%s" % (name, r.stdout, r.stderr))
    return exe


def run_mitm(mitm, mode, d, client):
    cd = tempfile.mkdtemp(dir=d)
    srv = subprocess.Popen([mitm, "-mode", mode, "-addr", "127.0.0.1:%d" % PORT, "-cadir", cd],
                           stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    if srv.stdout.readline().strip() != "ready":
        srv.kill()
        return "server did not start", ""
    cmd = client(cd)
    try:
        r = subprocess.run(cmd[0], cwd=cmd[1], capture_output=True, text=True, timeout=600)
        cout = (r.stdout + r.stderr).strip()
    except subprocess.TimeoutExpired:
        cout = "client timed out"
    try:
        sout, _ = srv.communicate(timeout=30)
    except subprocess.TimeoutExpired:
        srv.kill()
        sout, _ = srv.communicate()
    return cout, sout.strip()


def sgo_client(cd):
    ca = open(os.path.join(cd, "ca.der"), "rb").read().hex()
    now = int(open(os.path.join(cd, "now")).read())
    m = os.path.join(cd, "client")
    os.makedirs(m)
    open(os.path.join(m, "main.sgo"), "w").write(CLIENT % dict(ca=ca, port=PORT, name="tls.test", now=now, alpn="", suites=""))
    open(os.path.join(m, "sgo.mod"), "w").write(
        "module example.com/tlsclient\n\nsgo %s\ngrant connect 127.0.0.1:%d\ngrant entropy\n" % (VERSION, PORT))
    return [SGO, "run", "."], m


def mitm_cases(d, only):
    mitm = go_build(d, "mitm", MITM)
    gocli = go_build(d, "gocli", GOCLI)
    bad = 0
    for mode in MITM_MODES:
        name = "mitm-" + mode
        if only and name not in only:
            continue
        gc, gs = run_mitm(mitm, mode, d, lambda cd: ([gocli, "127.0.0.1:%d" % PORT, os.path.join(cd, "ca.der")], cd))
        sc, ss = run_mitm(mitm, mode, d, sgo_client)
        want_ok = mode in ("none", "fragmented", "keyupdate")
        ok = gc == sc and gs == ss and gc.startswith("client: " if want_ok else "client error: ")
        print("%-22s %s" % (name, "ok" if ok else "FAIL"))
        if not ok:
            bad += 1
            print("  go:   %s | %s\n  s-go: %s | %s" % (gc, gs, sc, ss))
    return bad


def main():
    only = set(sys.argv[1:])
    bad = 0
    with tempfile.TemporaryDirectory() as d:
        exe = build_server(d)
        for case in CASES:
            if only and case[0] not in only:
                continue
            cout, sout = run_case(exe, d, case)
            ok = cout.startswith(case[8]) and sout.startswith(case[9])
            print("%-22s %s" % (case[0], "ok" if ok else "FAIL"))
            if not ok:
                bad += 1
                print("  client: %s\n  want:   %s\n  server: %s\n  want:   %s" % (cout, case[8], sout, case[9]))
        bad += hostile(d, only)
        bad += mitm_cases(d, only)
    print("test-tls: %d problem(s)" % bad)
    sys.exit(1 if bad else 0)


if __name__ == "__main__":
    main()
