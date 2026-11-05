# W25 plan — admit float64 (D185)

D185 was approved in principle on 2026-11-07 ("(c) for the future"). This plan turns it into decisions D187–D194 for
approval, then into work in the order the W13 language weeks used: a reference model first, then
admission, lowering, std, and a freeze (spec 0.20.0). Earlier groundwork: D140 (PROPOSED, the stdlib
parity plan) recommended float64 only, IEEE 754, checked float-to-int conversion, no float map keys,
and no FMA contraction. This plan adopts those recommendations and makes each one a decision.

## Why it is a language change, not a library

Go's floating point is not deterministic across S-Go's targets, and S-Go promises one outcome per
program and input (lab replay, conformance across darwin/arm64, linux/amd64, linux/arm64):

- **Fused multiply-add.** The compiler fuses `a + x*y` into one rounding: always on arm64
  (`ARM64.rules`, `FMADDD`), on amd64 only at GOAMD64 ≥ v3 (`VFMADD231SD`). The same source gives
  different bits on the two architectures. There is one choke point, `ssa.(*Func).useFMA`.
- **Assembly in `math`.** Go's `Exp`, `Log`, `Floor`, `Hypot`, and others have per-architecture
  assembly, so `math.Exp(x)` may differ in the last bit between amd64 and arm64.
- **Float-to-int conversion.** Go leaves an out-of-range or NaN conversion implementation-defined
  (it differs between amd64 and arm64). S-Go's integers are checked (SGO-VAL-*), and the conversion
  must be too.
- **Map keys.** A NaN key can be inserted any number of times and never found.

## Decisions (approved 2026-11-07)

### D187 — The type: `float64` only
`float64`, IEEE 754 binary64. `float32`, `complex64`, `complex128`, `real`, `imag`, and `complex`
stay denied (SGO-FLOAT-001 keeps them). Float literals (`1.5`, `1e9`, `0x1p-2`) are admitted;
imaginary literals are not. A user type over `float64` (`type Celsius float64`) is admitted as user
types over `int` are (SGO-UTYPE-*).

### D188 — Arithmetic: IEEE 754 with round-to-nearest-even, no traps, no fusion
`+ - * /`, unary `-`, and comparisons follow IEEE 754 with round-to-nearest-even. Division by zero
and overflow give the infinities and NaN as values: nothing fails (unlike `int`, where overflow
fails). `-0.0` and `+0.0` are distinct values that compare equal. `%` is denied on floats, as in Go
(`math.Mod` instead). No operation is fused in S-Go code on any target (`useFMA` is false in S-Go
mode): each operation rounds once. An explicit fused multiply-add is `math.FMA`.
- Alternative: make division by zero and overflow fail, like `int`. Rejected: it breaks the IEEE
  identities that numeric code relies on (`1/x` where x underflows to 0), and NaN or Inf from
  parsing are values anyway.

### D189 — Conversions: checked to `int`, rounded from `int`
- `int(f)` truncates toward zero and fails with `IntegerOverflow` (exit 2) if `f` is NaN, an
  infinity, or out of range for `int`. The same holds for `byte(f)` and `rune(f)` (the ranges of
  SGO-TYPE-007/008).
- `float64(i)` rounds to nearest-even (exact up to 2^53): never fails.
- Constants follow Go: an untyped float constant is exact, and is rounded when given type
  `float64`; a constant that overflows `float64`, or a non-integral constant converted to `int`, is
  a compile error (`SGO_E_CONST_OVERFLOW`).
- `string(f)` is not a conversion (it is denied); text goes through `strconv`/`fmt`.

### D190 — Comparison and map keys
`==`, `!=`, `<`, `<=`, `>`, `>=` are admitted with IEEE meaning (`NaN != NaN`; every ordered
comparison with NaN is false). A map key type that is `float64`, or a struct or array containing one,
is denied (`SGO_E_UNADMITTED`): a NaN key would make the map's deterministic iteration order
(SGO-COMP-006) contain entries that can never be found. A struct with a float field remains
comparable with `==`.

### D191 — `std/math` gets the float functions; the integer ones are renamed
`std/math` today has `Abs`, `Min`, `Max`, `Pow` on `int`. Go's names are the float ones. Options:
- **(a) Recommended:** the float functions take Go's names (`math.Abs(x float64)`), and the integer
  ones move to `std/math` as `AbsInt`, `MinInt`, `MaxInt`, `PowInt`. This breaks 0.19 programs that
  call the four integer functions, and the freeze fixes every example and fixture. 0.x permits it,
  and Go code then ports unchanged.
- (b) Keep the integer names and give the float ones a prefix (`math.FAbs`).
- (c) Put the float functions in a new package `std/fmath`.

### D192 — Which `math` functions, and how exact
- **Exact (correctly rounded, hardware or exact algorithm):** `Abs`, `Copysign`, `Signbit`,
  `Floor`, `Ceil`, `Trunc`, `Round`, `RoundToEven`, `Mod`, `Remainder`, `Sqrt`, `FMA`, `Min`,
  `Max`, `Inf`, `NaN`, `IsNaN`, `IsInf`, `Float64bits`, `Float64frombits`, `Nextafter`, `Frexp`,
  `Ldexp`, `Modf`, `MaxFloat64`, `SmallestNonzeroFloat64`, `Pi`, `E`.
- **Transcendental (`Exp`, `Log`, `Log2`, `Log10`, `Pow`, `Sin`, `Cos`, `Tan`, `Atan`, `Atan2`,
  `Sinh`, `Tanh`, `Hypot`, `Cbrt`, `Exp2`, `Log1p`, `Expm1`, `Asin`, `Acos`):** written in S-Go
  from Go's portable implementations (no assembly). The results are identical on every target, but
  not always correctly rounded: Go's documented error bounds (≤ 1 ulp) are the specification's.
  - Option: ship only the exact set in W25 and the transcendentals in a later week. Recommended:
    both in W25, because without `Pow` and `Log` most numeric programs cannot be written.

### D193 — Text: `strconv` and `fmt`
- `strconv.ParseFloat(s string) (float64, error)`: Go's syntax (decimal, hex `0x1p-2`, `inf`,
  `nan`, underscores only per Go's rules) and correct rounding. There is no bitSize parameter
  (float64 only, like `ParseInt` without bitSize). The error forms are Go's, quoted as
  `QuoteToASCII` (SGO-STD-004).
- `strconv.FormatFloat(f float64, fmt byte, prec int) string`: `'f'`, `'e'`, `'g'` (and `'b'`,
  `'x'`), and `prec -1` for the shortest form that round-trips, identical to Go's.
- `fmt`: `%f %e %g %v` with precision (`%.2f`), width, and the `-`, `0`, and `+` flags, identical to
  Go's.
- These are ported from Go's `strconv` (`ftoa`, Ryu shortest, `eisel_lemire`, `atof`) as S-Go source,
  and differentially tested against Go.

### D194 — Evidence: an exact oracle and differential tests
The model step of this week is an oracle rather than a scheduler model: a reference
implementation of D188–D190 in exact rational arithmetic (Python `fractions`, independent of the
compiler and of Go's code). It checks:
1. arithmetic results bit for bit on generated operands (normals, subnormals, ±0, ±Inf, NaN,
   boundary cases), including expressions of the form `a + x*y` that a fused implementation would
   round differently;
2. float-to-int conversion outcomes (the value, or `IntegerOverflow`);
3. `ParseFloat`/`FormatFloat` round trips and correct rounding.

The compiled programs run on all three targets in CI, and their outputs must be byte-identical
across targets (the fused/unfused difference shows up here if `useFMA` is not disabled).
`test-std-diff.py` gains the float functions and verbs against Go's (with Go's fusion avoided by
explicit `float64()` roundings on the Go side).

## Work order (after approval)

1. **Oracle and tests first** (D194): the generator and the exact oracle; expected outputs of the
   conformance cases.
2. **Admission:** SGO-FLOAT-001 v2 (what stays denied), new SGO-FLOAT-002 (type, literals,
   arithmetic, comparison), SGO-FLOAT-003 (conversions), and amendments to SGO-TYPE-001 (admitted
   types), SGO-COMP-006 (map keys), and SGO-VAL-* (constants); `sgoadmit` and `types2`.
3. **Lowering:** `useFMA` off in S-Go mode; checked float-to-int conversion in `walk` (the pattern
   of the checked integer conversions); `math.FMA` intrinsic kept.
4. **std:** `std/math` float functions (the D191 option chosen) and transcendentals; `strconv`
   `ParseFloat`/`FormatFloat`; `fmt` float verbs; `test-std-diff.py`.
5. **CLI library:** the `Float` flag kind in `sgo/examples/cli` (D184's deferred item).
6. **Freeze 0.20.0:** fixtures (`float-*`), gates, Linux CI, evidence `sgo/evidence/w25`.

Risk: high (a new value kind through the compiler and std). The largest items are the `strconv`
port and the transcendentals, both of them mechanical ports with differential tests.

## Also at the 0.20.0 freeze: W24 review P1 (resource text) and P2 (library composition)

From the note2 review of `d868fdc459` (message 1348):

- **SGO-RES-002 v4** (P1): the sharing sentence is restricted to built-in resource handles. Added:
  "A user resource (SGO-RES-004) is task-local (SGO-COMP-004): only the task that owns it, or the
  task its result was moved to (SGO-SPAWN-006), uses it; it reaches another task only by a move.
  Shared mutable state between tasks is a server task with request/reply calls (SGO-SPAWN-008) or
  channels of copied values." The implementation is unchanged: it already refuses capture
  (`res-user-capture-denied`), a same-nursery send (`res-user-send-in-scope`), and `sgo.Copy`.
- **SGO-RES-004 v6** (P1): `ownership_and_lifecycle` states the same exception instead of only
  referring to SGO-RES-001. New run case `res-user-via-call` shows the recommended pattern.
- **SGO-RES-005** (P2, option 1): `compile_link_checks` keeps the refusal; a guide note says that
  helpers return resources and the caller sends them in the scope that owns both. Option 2
  (parameter-relation summaries in export data) is D195, proposed.
