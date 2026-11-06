# W27 evidence — signatures, big numbers, AES-GCM, ASN.1 and X.509

Spec 0.22.0 (D199, D200). Branch `w27`; state: complete on darwin/arm64; Linux pending (W27 merge).

## What landed

| Commit | Content |
| --- | --- |
| 02d4b2dda4 | W27 task entry (D199) |
| 0be6e74fd8 | `std/crypto/ed25519` (SGO-STD-023) |
| 75b64007b4 | `std/crypto/bigmod`, `std/crypto/ecdsa` verify on P-256/P-384 (SGO-STD-024, 025) |
| 0dd8d36dd7 | `std/crypto/rsa` verify, bitsliced `std/crypto/aes`, `cipher.Block` and `NewGCM`, `std/encoding/asn1` (SGO-STD-020 v2, 026..028) |
| bdfca7e092, b10b27a319 | `std/crypto/x509` (SGO-STD-029); x509-limbo stage in `sgo/scripts/test-std-crypto.py`; fixture std-x509-basic |
| a856fddf2d | ECDSA on P-224 and P-521; x509 verifies them; Go-issued chains in the x509 stage |
| a037a944ff | D200: `std/crypto/x509` as built |

## darwin/arm64

- Gates (`conformance` plus the check scripts) green on 75b64007b4 and 0dd8d36dd7.
- `sgo/scripts/test-std-crypto.py`, identical to Go on generated inputs: `std/bits` 28426 results,
  `std/encoding` 6078, `std/crypto/sha256` 1820, `std/crypto/sha512` 2226, `std/crypto/ed25519` 60,
  `std/crypto/bigmod` 93, `std/crypto/aes` 40, `std/encoding/asn1` 400.
- Wycheproof, the vectors of Go's own crypto tests: ECDSA 6905 of 6905 (P-224, P-256, P-384, P-521;
  DER and IEEE P1363), RSA 4171 of 4171 (PKCS #1 v1.5 and PSS), AES-GCM 197 of 197, Ed25519 150 of
  150, plus the W26 sets (HMAC/HKDF 774, ChaCha20-Poly1305 316, X25519 518).
- X.509: the 9746 x509-limbo cases Go's `TestX509Limbo` runs (github.com/C2SP/x509-limbo at the
  version in `src/crypto/internal/cryptotest/x509limbo/schemaversion.go`) and 22 chains Go issues
  for every key type and signature algorithm it signs with, and their tampered variants: 9768
  cases, parse and verification results and error texts identical to Go's crypto/x509 (960
  limbo chains verify, 8711 are refused, 75 do not parse).
- `sgo/scripts/test-std.py`: 29 packages, 62 tests.

## Linux

Pending: the sgo-w00, sgo-spec and sgo-runtime runs at the W27 merge.
