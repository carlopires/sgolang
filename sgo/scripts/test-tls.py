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
            print("%-12s %s" % (case[0], "ok" if ok else "FAIL"))
            if not ok:
                bad += 1
                print("  client: %s\n  want:   %s\n  server: %s\n  want:   %s" % (cout, case[8], sout, case[9]))
    print("test-tls: %d problem(s)" % bad)
    sys.exit(1 if bad else 0)


if __name__ == "__main__":
    main()
