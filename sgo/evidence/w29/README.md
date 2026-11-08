# W29 evidence: seeded entropy in the lab, and the lab replay of a TLS handshake

Spec 0.24.0 (D202). Branch `w29`; state: complete on darwin/arm64, linux/amd64 and linux/arm64.

## What changed

- **Entropy in a lab run** (SGO-LAB-004 v2). `Entropy.Bytes(k)` draws ceil(k/8) little-endian
  outputs of the seed's `data` stream (PCG-DXSM seeded with SplitMix64 outputs 5 and 6 of the
  seed, D44), in call order. A run and its replay draw the same bytes. Production `Entropy` is
  unchanged; a lab binary runs only under `sgo lab`, so seeded bytes cannot reach a production run.
- **No refused effect** (SGO-LAB-006 v2). Entropy was the only effect the lab refused, so the
  `UnsupportedLabEffect` outcome is gone from the spec and the runtime.
- **The TLS client in the lab** (SGO-STD-031 v2): the W28 deliverable that was not possible then.

## Checks

| Check | Result |
|---|---|
| `lab-entropy` (seed 1, repeat, replay) | the 17 bytes of two draws equal the same computation with Go's `math/rand/v2` PCG, run independently |
| `sgo/scripts/test-lab.py` | same seed, same bytes; another seed, other bytes; replay reproduces them |
| `lab-tls-replay` (seed 1, repeat, replay) | full TLS 1.3 handshake (TLS_AES_128_GCM_SHA256) against a recorded Go `crypto/tls` server, Go's data read; `sgo lab replay` reproduces all 9601 events |
| `sgo/scripts/gen-lab-tls.py --check` | a fresh recording equals the committed fixture (repeated runs; also with `HOME` moved away and the bootstrap reached only through `SGO_CACHE`) |
| conformance, all local gates | 23/23 on `617206a8f4` (conformance 811/811); after the freeze: below |

How the replay works: the client's bytes are fixed by the seed, so the exchange is recorded in four
lab steps against Go's server (ClientHello; the server's flight; the client's Finished; the server's
data). The server is made deterministic with a seeded `Rand`, `GODEBUG=cryptocustomrand=1` (Go 1.26
otherwise ignores a caller's random source for the key share), one-byte reads answered off the
stream (Go's `randutil.MaybeReadByte` reads one byte at random about half the time, which shifted
the key share between runs), a fixed time, Ed25519 certificates and no session tickets. In the
fixture the server task replays the recording and checks the client's 188 + 64 bytes against it
byte for byte; another seed's ClientHello differs from the recording and the run fails.

## Linux

Main at 9cb9fafaf7 (0.24.0 freeze): sgo-w00 run 42492251746 green on linux/amd64 and linux/arm64 (conformance, all check scripts, test-lab, gen-lab-tls --check, reference and stage-0 builds); sgo-spec run 42492251893 green; sgo-runtime run 42492316580 green (runtime lane, race lane).

## Not done

- An HTTPS example (W28): still deferred; it needs a reachable HTTPS server.
- Certificate Transparency: not enforced (D201 (d), decided "not now"; opt-in later if wanted).
