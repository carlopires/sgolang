// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command tlsrec is the server half of the lab TLS replay (D202): a Go crypto/tls TLS 1.3
// server made deterministic (seeded Rand, fixed time, Ed25519 certificates, no tickets), fed the
// S-Go client's recorded bytes. It prints the CA certificate (PEM), the server's flight (hex,
// everything written before it waits for the client's Finished) and, when the client's second
// flight is given, the application data it then writes (hex).
//
//	tlsrec -ca                      print the CA certificate
//	tlsrec -ch HEX                  print the server flight answering the ClientHello
//	tlsrec -ch HEX -fin HEX         print the server flight, a newline, and the application data
package main

import (
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"math/big"
	"math/rand/v2"
	"net"
	"os"
	"sync"
	"time"
)

var now = time.Unix(1800000000, 0)

type seeded struct{ r *rand.ChaCha8 }

// Read fills p from the seeded stream. A one-byte read is answered without using the stream:
// crypto/internal/randutil.MaybeReadByte reads one byte from a caller's reader at random (about
// half the time) before key generation, which would shift the stream and change the server's
// key share from run to run. No other read of the TLS server is one byte long.
func (s seeded) Read(p []byte) (int, error) {
	if len(p) == 1 {
		p[0] = 0
		return 1, nil
	}
	return s.r.Read(p)
}

func certs() (caDER []byte, leaf tls.Certificate) {
	caKey := ed25519.NewKeyFromSeed(make([]byte, 32))
	seed := make([]byte, 32)
	seed[0] = 1
	leafKey := ed25519.NewKeyFromSeed(seed)
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "lab replay CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(nil, ca, ca, caKey.Public(), caKey)
	if err != nil {
		panic(err)
	}
	caCert, _ := x509.ParseCertificate(caDER)
	lt := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "tls.test"}, DNSNames: []string{"tls.test"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(nil, lt, caCert, leafKey.Public(), caKey)
	if err != nil {
		panic(err)
	}
	return caDER, tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: leafKey}
}

// script is the client end: its Read returns the recorded client bytes, one flight at a time;
// when a flight is used up it reports that the server waits, then blocks for the next flight.
type script struct {
	mu      sync.Mutex
	in      []byte
	flights [][]byte
	waiting chan struct{}
	more    chan struct{}
	out     []byte
	closed  bool
}

func (s *script) Read(p []byte) (int, error) {
	s.mu.Lock()
	for len(s.in) == 0 {
		if len(s.flights) == 0 {
			s.mu.Unlock()
			s.waiting <- struct{}{}
			<-s.more
			s.mu.Lock()
			if len(s.flights) == 0 {
				s.mu.Unlock()
				return 0, io.EOF
			}
		}
		s.in, s.flights = s.flights[0], s.flights[1:]
	}
	n := copy(p, s.in)
	s.in = s.in[n:]
	s.mu.Unlock()
	return n, nil
}

func (s *script) Write(p []byte) (int, error) {
	s.mu.Lock()
	s.out = append(s.out, p...)
	s.mu.Unlock()
	return len(p), nil
}

func (s *script) snapshot() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.out...)
}

func (s *script) Close() error                     { return nil }
func (s *script) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (s *script) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (s *script) SetDeadline(time.Time) error      { return nil }
func (s *script) SetReadDeadline(time.Time) error  { return nil }
func (s *script) SetWriteDeadline(time.Time) error { return nil }

func main() {
	caOnly := flag.Bool("ca", false, "print the CA certificate")
	ch := flag.String("ch", "", "ClientHello flight (hex)")
	fin := flag.String("fin", "", "client second flight (hex)")
	flag.Parse()
	caDER, leaf := certs()
	if *caOnly {
		pem.Encode(os.Stdout, &pem.Block{Type: "CERTIFICATE", Bytes: caDER})
		return
	}
	chb, err := hex.DecodeString(*ch)
	if err != nil {
		panic(err)
	}
	s := &script{flights: [][]byte{chb}, waiting: make(chan struct{}), more: make(chan struct{})}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{leaf}, MinVersion: tls.VersionTLS13,
		Time: func() time.Time { return now }, Rand: seeded{rand.NewChaCha8([32]byte{7})},
		SessionTicketsDisabled: true, CurvePreferences: []tls.CurveID{tls.X25519},
	}
	srv := tls.Server(s, cfg)
	done := make(chan error, 1)
	go func() {
		err := srv.Handshake()
		if err == nil {
			_, err = srv.Write([]byte("hello from go\n"))
		}
		done <- err
	}()
	select {
	case <-s.waiting:
	case err := <-done:
		fmt.Fprintln(os.Stderr, "tlsrec: server ended before the client's Finished:", err)
		os.Exit(1)
	}
	flight := s.snapshot()
	fmt.Println(hex.EncodeToString(flight))
	if *fin == "" {
		return
	}
	finb, err := hex.DecodeString(*fin)
	if err != nil {
		panic(err)
	}
	s.mu.Lock()
	s.flights = append(s.flights, finb)
	s.mu.Unlock()
	s.more <- struct{}{}
	if err := <-done; err != nil {
		fmt.Fprintln(os.Stderr, "tlsrec: server:", err)
		os.Exit(1)
	}
	fmt.Println(hex.EncodeToString(s.snapshot()[len(flight):]))
}
