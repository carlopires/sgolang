# W26 evidence — unsigned integers, package-level tables, std/encoding and std/crypto

Spec 0.21.0 (D136, D196, D197). Branch `w26`; state: darwin/arm64 complete, Linux at the merge.

## What landed

| Commit | Content |
| --- | --- |
| 6f279a2aa5 | D136, D196, D197 decision records; W26 task entry |
| 3f25ebcd89 | `uint32`/`uint64` with checked arithmetic (SGO-TYPE-009) and `std/bits` (SGO-STD-011); 21 fixtures |
| 8c12d8af0e, 9fb9a8f0cc | package-level tables (SGO-BIND-003 v2): `types2/sgo_tables.go`, `sgoadmit/admit.go`; 9 `tbl-*` fixtures |
| 2958439d1a | `std/encoding/hex`, `base64`, `binary` (SGO-STD-012..014) |
| 1f58397295 | `std/crypto/sha256` (SGO-STD-015) |
| 436f6b6d7b | `std/hash`, `std/crypto/sha512`, `hmac`, `hkdf` (SGO-STD-016..019) |
| 9476a3fea7 | `std/crypto/cipher`, `chacha20poly1305`, `curve25519` (SGO-STD-020..022) |
| 9b269a22ff | `sgo/scripts/test-std-crypto.py`, wired into sgo-w00 |

## darwin/arm64

- Gates (`conformance` plus 21 check scripts) green on 436f6b6d7b and on 9476a3fea7; 802 conformance cases.
- `sgo/scripts/test-std-crypto.py`, seeds 1 and 2, all identical to Go: `std/bits` 28426 results,
  `std/encoding` 6078 / 6090, `std/crypto/sha256` 1820, `std/crypto/sha512` 2226.
- Wycheproof, the vectors of Go's own crypto tests (github.com/c2sp/wycheproof at the version in
  `src/crypto/internal/cryptotest/wycheproof/schemaversion.go`): HMAC and HKDF over SHA-256/384/512
  774 of 774; ChaCha20-Poly1305 316 of 316 (the 9 vectors with other key or nonce sizes skipped);
  X25519 518 of 518, every all-zero result refused.
- RFC vectors in the package tests: FIPS 180-4, RFC 4231-style HMAC, RFC 5869 case 1, RFC 8439
  section 2.8.2, RFC 7748 section 6.1. `sgo/scripts/test-std.py`: 22 packages, 47 tests.

## Linux

PENDING: sgo-w00 and sgo-spec on main after the merge.
