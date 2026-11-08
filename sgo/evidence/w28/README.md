# W28 evidence — a TLS 1.3 client, pinned roots, and trust from granted files

Spec 0.23.0 (D199, D201). Branch `w28`; state: complete on darwin/arm64, linux/amd64 and linux/arm64.

## What landed

| Commit | Content |
| --- | --- |
| d828573990 | D201 approved (d), with the emergency-distrust amendments (a)–(d) |
| 04a59f2aa0 | W28 task entry and plan (`sgo/docs/w28-tls-plan.md`) |
| c3ad0ebb19 | `std/crypto/x509/roots`: the pinned Mozilla root set; `CertPool.AddCertWithDistrustAfter` (SGO-STD-030; SGO-STD-029 v2) |
| 4ddd23cd7f | `std/crypto/tls`: the TLS 1.3 client (SGO-STD-031); `sgo/scripts/test-tls.py`; fixture std-tls-alert |
| a8f04d3e11 | SGO-STD-031 is production only: the lab refuses `Entropy` (SGO-LAB-006) |
| 24aa3e12ea | Go's record-layer rules; 26 hostile plaintext scenarios against Go's client |
| 227b0c2732 | 13 hostile encrypted flights through a key-log proxy |
| 417f227af1 | `inventory.md` regenerated for the SGO-STD-031 digest |
| 8f00a6c76d | D201 step 2: `std/encoding/pem` (SGO-STD-032), `AppendCertsFromPEM`, distrusted keys per pool (SGO-STD-029 v3); fixture std-x509-pem-file; `sgo/scripts/test-std-pem.py` |
| 44dd126fc3 | D201 (c): binaries record their certificate trust; `sgo version -m` (SGO-BUILD-001 v2, SGO-TOOL-001 v3) |

## darwin/arm64

- Gates (conformance plus the check scripts) green on 4ddd23cd7f (23/23, conformance 810/810),
  227b0c2732 (22/23; the red one, test-mutations, was the stale `inventory.md` fixed in 417f227af1,
  green on rerun), 8f00a6c76d (23/23) and 44dd126fc3.
- `sgo/scripts/test-tls.py`, 50 cases:
  - 11 against Go's crypto/tls server: ECDSA P-256 and P-384, Ed25519 and RSA leaf keys;
    TLS_AES_128_GCM_SHA256, TLS_CHACHA20_POLY1305_SHA256 and TLS_AES_256_GCM_SHA384; ALPN; a
    requested client certificate; a 100 KiB reply; a wrong server name and an expired certificate
    fail with Go's errors, and the server sees Go's `bad certificate` alert.
  - 26 crafted plaintext replies (ServerHello and HelloRetryRequest faults, unexpected messages,
    alerts, ChangeCipherSpec and record faults): the S-Go client gives Go's client's error and sends
    the same alert. The first run found 5 record-layer differences, all ported from Go.
  - 13 mutations of Go's server flight through a proxy that decrypts and re-encrypts it with the
    server's key log: messages fragmented into 7-byte records and a KeyUpdate with update_requested
    succeed as with Go's client; a corrupt CertificateVerify signature, a mismatched signature
    scheme, a corrupt Finished, a corrupt and a malformed certificate, an empty certificate list, a
    repeated EncryptedExtensions, a missing CertificateVerify, an unrequested ALPN protocol and a
    corrupt record fail with Go's client's errors and alerts. All matched on the first run.
- `sgo/scripts/test-std-pem.py`: 247 generated PEM inputs per seed (valid, altered, truncated);
  Go's encoding/pem and `std/encoding/pem` agree on every block, rest length and re-encoding
  (seeds 1–4).
- `sgo/scripts/test-std-crypto.py` x509-roots stage: the pinned pool's 117 subjects, in order,
  identical to Go's pool from the same bundle.
- `sgo/scripts/test-identity.py`: `sgo version -m` reports the trust record of a program that links
  the roots, and none for one that does not link x509.
- `sgo/scripts/test-std.py`: 32 packages, 70 tests.

## Linux

Main at 5cc98a5a9b (0.23.0 freeze): sgo-w00 run 42478594681 green on linux/amd64 and linux/arm64 (conformance, all check scripts, test-tls, test-std-pem, test-std-crypto including x509-limbo, reference and stage-0 builds); sgo-spec run 42478594707 green; sgo-runtime run 42478636985 green (runtime lane, race lane).

## Not done

- A lab replay of a full handshake (a W28 deliverable): not possible as specified. The lab refuses
  `Entropy` (SGO-LAB-006), and the client's random, session ID and key share come from it, so a
  program that calls `tls.Client` does not run in the lab. Replaying a handshake needs a decision on
  deterministic entropy in the lab; the hostile-flight proxy of `test-tls.py` covers what a replay
  would check.
- An HTTPS example (a W28 deliverable): deferred. A runnable example needs an HTTPS server it may
  reach; CI has no network, and a loopback server needs a TLS server, which W28 does not build.
- Session resumption, 0-RTT, TLS 1.2 and client certificates (not planned for W28).
- Certificate Transparency checks: D201 (d), open, a separate decision.
