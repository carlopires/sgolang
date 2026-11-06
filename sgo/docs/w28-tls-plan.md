# W28 plan — a TLS 1.3 client and its roots

Decisions: D199 (TLS 1.3 client in W28), D201 (pinned Mozilla roots in `std`, replaceable or
extended from a granted file, with emergency distrust by key). Spec minor version: 0.23.0.

## Scope

A TLS 1.3 client (RFC 8446) as S-Go source over `*sgo.Conn`, enough to reach typical public HTTPS
servers, and the trust inputs it needs. Not in W28: a TLS server, TLS 1.2 and earlier, session
resumption and 0-RTT, client certificates, Certificate Transparency checks (D201 (d), a later
decision), network revocation.

## What S-Go code supplies (no ambient inputs)

The client reads no clock, no system roots and no ambient randomness, as `std/crypto/x509`
(D200). A connection is made from a granted `*sgo.Conn` and a `Config` with:
- `ServerName` — the name to verify and send in SNI (required; an IP literal is verified against
  the certificate's IP SANs and not sent in SNI, as Go's).
- `RootCAs *x509.CertPool` — required; `std/crypto/x509/roots` supplies the pinned Mozilla set.
- `Time int` — Unix seconds for certificate validity (required).
- `Entropy *sgo.Entropy` — the granted entropy handle, for the client random and the key share
  (deterministic in the lab, so whole handshakes replay).
- `NextProtos []string` — ALPN, optional.

## Steps

1. **`std/crypto/x509/roots`** — the Mozilla root set as a frozen table of DER certificates
   (from Go's `golang.org/x/crypto/x509roots/fallback` bundle, at a pinned version), with
   `Mozilla() *x509.CertPool`. The bundle's version and digest are recorded so a binary's root set
   can be reported (D201 (c), tool support in step 6).
2. **TLS 1.3 key schedule and record layer** in `std/crypto/tls` (internal): HKDF-Expand-Label,
   Derive-Secret, traffic keys and IVs, the per-record nonce, AEAD record protection with
   TLS_AES_128_GCM_SHA256, TLS_AES_256_GCM_SHA384 and TLS_CHACHA20_POLY1305_SHA256, content type
   and padding, alert handling. Checked against the RFC 8448 traces.
3. **Handshake**: ClientHello (supported_versions, X25519 key share, signature_algorithms, SNI,
   ALPN), ServerHello, HelloRetryRequest (only toward X25519; anything else fails), Encrypted
   Extensions, Certificate (chain verified with `std/crypto/x509` against `RootCAs`, `Time` and
   `ServerName`), CertificateVerify (ECDSA P-256/P-384/P-521, RSA-PSS, Ed25519), Finished both
   ways; post-handshake NewSessionTicket ignored, KeyUpdate honoured; close_notify.
4. **API**: `tls.Client(conn *sgo.Conn, config Config) (*Conn, error)` performing the handshake;
   `(*Conn).Read(max)`, `ReadLine(max)`, `Write(s)`, `Close()` (sends close_notify, then closes
   the connection), `ConnectionState()` (version, suite, negotiated protocol, peer chain);
   `*tls.Conn` is a resource that owns the `*sgo.Conn` and satisfies `std/io`'s interfaces.
5. **Tests**: (a) RFC 8448 key-schedule and record vectors in the package test; (b) a
   differential harness in `sgo/scripts/test-std-crypto.py`: Go's crypto/tls servers on loopback
   with each suite, certificate key type, HRR, ALPN, large and fragmented records, KeyUpdate and
   close behaviour, and broken peers (bad chain, wrong name, expired, bad signature, bad Finished,
   unexpected messages); each case's outcome and error text compared with Go's crypto/tls client
   on the same server; (c) a lab fixture replaying a full handshake with lab entropy and a fake
   peer; (d) an example fetching a page over HTTPS in `sgo/examples`.
6. **D201 step 2** if time allows: `std/encoding/pem`, `CertPool.AppendCertsFromPEM`, distrusted
   keys by SPKI hash in the pool and the granted trust file, and the toolchain report of the root
   set and distrust list a binary was built with.

## Acceptance

Every differential case identical to Go's client, every RFC 8448 vector, conformance and the gates
green on darwin/arm64, linux/amd64 and linux/arm64, spec 0.23.0 frozen.
