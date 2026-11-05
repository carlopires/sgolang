# W25 evidence — float64 (spec 0.20.0)

Decisions D185 (approved in principle 2026-11-07) and D187–D194 (approved 2026-11-07, D191 option
a; plan `sgo/docs/w25-float-plan.md`). New clauses frozen at 0.20.0: SGO-FLOAT-002, SGO-FLOAT-003.
Amended: SGO-FLOAT-001 v2, SGO-LEX-004 v3, SGO-TYPE-001 v10, SGO-VAL-004 v2, SGO-COMP-002 v3,
SGO-COMP-006 v2, SGO-STD-004 v2, SGO-STD-007 v2, SGO-STD-009 v2; and from the W24 review (note2,
message 1348) SGO-RES-002 v4, SGO-RES-004 v6 (P1) and SGO-RES-005 v2 (P2). CHANGELOG entry
"0.20.0". The registry row SPEC-FLOAT is admitted in 0.20.0.

| Decision | What | Commits on `w25` |
|---|---|---|
| D194 | exact oracle `sgo/scripts/float-oracle.py` (rational arithmetic, independent of Go and the host) | ff984f86db |
| D187–D190 | admission of `float64` and literals; no float map keys; checked float-to-int conversion; no fused contraction in `.sgo` code (`useFMA` false) | 5fcc8ba9d2 |
| D191, D192 | `std/math` float core (`bits.sgo`, `float.sgo`), integer functions renamed `Int*` | 2ff942a9ad |
| D192 | `math.FMA` in pure S-Go (30-bit limbs, one rounding) | 68af77c8af |
| D192 | transcendentals ported from Go's portable code | b1230bd678 |
| D192 | `Sin` `Cos` `Tan` `Sincos` with Go's Payne-Hanek reduction in 16-bit limbs | 4f13ffc019 |
| D193 | `strconv.ParseFloat` / `FormatFloat` (Go's exact multiprecision path) | 263158d70e |
| D193 | `fmt` float verbs, `+`, precision; `test-std-diff` float coverage | c9a7ce3192 |
| D184 (deferred item) | `Float` flag kind in `sgo/examples/cli` | 4321967b35 |
| — | clauses and 25 fixtures; W24 review P1/P2 fixtures `res-user-send-in-scope`, `res-user-via-call` | c10663a344, 216d328d99 |
| — | README refresh (note2, for the user), `readme-example` | 14cb1a4fe8 |

## Results (darwin/arm64)

- The reference for the float functions is Go 1.26's own portable code: Go's `src/math` copied
  without assembly and built for amd64 v1 (no fused multiply-add; stock Go on arm64 fuses `x*y+z`
  and uses assembly for `Exp` and `Log`, so it is not a fair reference), run under Rosetta.
  - `std/math` exact functions: 419 inputs × 16 functions bit-identical.
  - `math.FMA`: 3000 cases (cancellation, subnormal, overflow, random) identical to the hardware FMA.
  - Transcendentals: 1433 inputs × 25 results (`Exp Exp2 Expm1 Log Log10 Log2 Log1p Atan Asin
    Acos Sinh Cosh Tanh Cbrt Acosh Asinh Atanh Sin Cos Tan Sincos Pow Atan2 Hypot`), 35825 values,
    bit-identical, including 306 trigonometric arguments of 2^29 and above (Payne-Hanek).
- `strconv` against Go's `strconv`: 45600 `FormatFloat` cases (570 values × 8 formats ×
  10 precisions) and 4073 `ParseFloat` inputs (round trips, halfway and subnormal boundaries,
  overflow, 800-digit and longer numbers, hexadecimal, underscores, malformed text) identical,
  error texts included.
- `sgo/scripts/test-std-diff.py`: 11698 checks identical to Go's packages on seeds 1–6, now with
  float arguments, the `+` flag, precisions in random `fmt` formats, and 800 strconv float checks.
- Conformance fixtures (25 new; `type-float` and `lex-float-literal` now run): `float-arith`
  (prints 0 for `-1 + x*y` with x·y rounding to 1, which a fused multiply-add would print as
  -2^-60), `float-special` (signed infinities and zeros; the runtime ties `tiny/2` → 0 and
  `tiny*1.5` → 2·tiny), `float-compare`, `float-literals`, `float-convert`, `float-usertype`;
  failing conversions `float-convert-nan`, `-inf`, `-range`, `-byte-range` (IntegerOverflow,
  exit 2); refusals `float-map-key-denied`, `float-struct-key-denied`, `float-mod-denied`,
  `float-bitwise-denied`, `float-mixed-denied`, `float-const-overflow`, `float-const-not-integral`,
  `float-string-denied`, `float32-denied`, `float-complex-denied`; `std-math-float`,
  `std-strconv-float`, `std-fmt-float` (each output identical to Go's on the same program);
  `readme-example`.
- `sgo/examples/cli`: `TestFloat`; `sgo/scripts/test-examples.py` green (all six examples, lab
  sweeps).

## Results at the 0.20.0 freeze

- darwin/arm64 at 7a9e070f07 (same toolchain as 9da365995c): conformance 768/768; the 21 gates all
  0, including `test-std-diff` (11698 checks identical to Go's packages) and `test-check`.
- linux/amd64 and linux/arm64 on main at 9da365995c: sgo-w00 run 42072666080 (both runners: stage0,
  conformance, identity, lab, examples), sgo-spec run 42072511789, sgo-runtime run 42072511595. The
  conformance fixtures print the bits of float results, so a target that fused or rounded
  differently fails them; none did.
- sgo-spec run 42069282143 on 7a9e070f07 failed license-audit: the std/math ports keep Go's
  header and the Sun notice, which the audit did not know. Fixed in 23eb2e42aa (an added file
  that names its upstream source must keep that file's header and notices verbatim).
- sgo-runtime run 42070519003 on 23eb2e42aa failed once on linux/arm64: TestSGoTaskResult
  expected a select between a ready result and a ready timer to pick the result within five
  tries, but select chooses at random (1 in 32). The test now takes the result with the
  result arm alone (9da365995c); the runtime was not at fault.

## Open

- D195 (proposed): parameter-lifetime summaries in export data, so a helper may forward a resource
  from one parameter to a channel received as another (W24 review P2, option 2).
