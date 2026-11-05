# SPEC-FLOAT — Floating point

`float64` is admitted from spec 0.20.0 (SGO-FLOAT-002, SGO-FLOAT-003; W25, D185, D187–D190): IEEE 754 binary64, rounded once per operation and never fused, so a program computes the same bits on every target. `float32`, the complex types, and imaginary literals stay denied (SGO-FLOAT-001).

### SGO-FLOAT-001 — float32, complex numbers, and imaginary literals are denied
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-ERR-001, SGO-FLOAT-002; D03, D187.
- **grammar_and_types:** Denied: `float32`, `complex64`, `complex128`, imaginary literals (`2i`), the built-in functions `real`, `imag`, and `complex`, and every operation on them. No form of these constructs is admitted in any package compiled for the target, including dependencies and unreachable code. (`float64` and floating-point literals are admitted by SGO-FLOAT-002; spec 0.19 and earlier denied them here.)
- **preconditions:** n/a — denial clause; there is nothing to use.
- **evaluation_and_value_semantics:** n/a — denied constructs never execute.
- **ownership_and_lifecycle:** n/a — denied constructs never execute.
- **effect_and_capability_requirements:** n/a — denied constructs never execute.
- **concurrency_and_memory_order:** n/a — denied constructs never execute.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Any use is a compile error; no object file is produced.
- **cancellation_and_cleanup:** n/a — denied constructs never execute.
- **compile_link_checks:** `SGO_E_UNADMITTED`, naming the construct and this clause.
- **runtime_guards:** n/a — rejected before execution.
- **tcb_assumptions:** n/a — no runtime involvement.
- **positive_tests:** n/a — denial clause.
- **negative_tests:** S16; fixture:float32-denied, fixture:float-complex-denied.
- **implementation_coverage:** `src/cmd/compile/internal/sgoadmit` (types and imaginary literals), W03; narrowed in W25.
- **executed_evidence:** W03 compiler-level denial; W25: conformance (sgo/evidence/w25/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-FLOAT-002 — The float64 type
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-FLOAT-001, SGO-FLOAT-003, SGO-TYPE-001, SGO-TYPE-002, SGO-VAL-004, SGO-COMP-002, SGO-COMP-006, SGO-UTYPE-001, SGO-STD-007; D185, D187, D188, D190.
- **grammar_and_types:** The predeclared type `float64` and user types whose underlying type is `float64` (`type Celsius float64`, SGO-UTYPE-001); floating-point literals: decimal (`1.5`, `1e9`, `.5`) and hexadecimal (`0x1p-2`, `0x1.8p1`), with `_` allowed only between digits, or between the `0x` prefix and a digit; the operators `+ - * /`, unary `+` and `-`, `== != < <= > >=`, and the assignment forms `= += -= *= /=` and `++ --`, on operands of identical type. There is no `%` on floats, no bitwise or shift operator on them, and no implicit conversion between `int` and `float64`: a mixed operation needs an explicit conversion (SGO-FLOAT-003). A `float64` may be a struct field, an array or slice element, a map value, a parameter, a result, and an element of a channel. A map key type that is `float64`, a user type over it, or a struct or array containing one at any depth, is denied.
- **preconditions:** None.
- **evaluation_and_value_semantics:** A `float64` value is an IEEE 754 binary64 value: a finite number (including `+0` and `-0`), `+Inf`, `-Inf`, or NaN. Each operation is the IEEE 754 operation rounded once to nearest, ties to even: `a + x*y` rounds the product and then the sum, on every target, and is never contracted into a fused multiply-add (D188; the one fused operation is `math.FMA`). Nothing fails: a nonzero divided by zero is an infinity with the sign of the quotient, `0/0`, `Inf-Inf`, and `0*Inf` are NaN, an overflow is a signed infinity, and an underflow is a subnormal or a signed zero. `-0` and `+0` compare equal; every comparison with a NaN is false except `!=`, which is true; a struct or array with a float field compares field by field, so it is unequal to itself if a field is NaN. The sign and payload of a NaN are not observable: a program sees one NaN, for which `math.Float64bits` returns `0x7FF8000000000000`, `math.Signbit` is false, and `strconv` and `fmt` print `NaN`. The zero value is `+0`. A constant expression is evaluated exactly (untyped constants, SGO-VAL-004) and is rounded once, to nearest-even, when it is given type `float64`. The results are identical on every supported target, in production and in the lab.
- **ownership_and_lifecycle:** A value type: assignment copies it, and it may cross tasks (SGO-COMP-004).
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** As any value (SGO-MEM-*).
- **allowed_nondeterminism:** None. In particular, none of the host's: no fused contraction, no extended precision, no NaN sign or payload.
- **failure_and_partial_progress:** No float operation fails. A conversion to an integer type may (SGO-FLOAT-003).
- **cancellation_and_cleanup:** n/a — arithmetic is not a cancellation point.
- **compile_link_checks:** `SGO_E_UNADMITTED` for a map key type containing `float64`, naming this clause; `SGO_E_CONST_OVERFLOW` for a constant that overflows `float64` when it is given that type; `SGO_E_TYPE_MISMATCH` for `%`, bitwise operators, and shifts on floats, and for mixed `int`/`float64` operands.
- **runtime_guards:** n/a — IEEE arithmetic without contraction.
- **tcb_assumptions:** The targets' floating-point units implement IEEE 754 binary64 with round-to-nearest-even and no traps (amd64 SSE2, arm64 FP); the compiler emits no fused contraction for code at positions in `.sgo` files (`ssa.(*Func).useFMA` is false there) and no x87 code.
- **positive_tests:** fixture:float-arith, fixture:float-special, fixture:float-compare, fixture:float-literals, fixture:float-usertype, fixture:type-float, fixture:lex-float-literal; `sgo/scripts/float-oracle.py` (the exact reference of D194).
- **negative_tests:** fixture:float-map-key-denied, fixture:float-struct-key-denied, fixture:float-mod-denied, fixture:float-bitwise-denied, fixture:float-mixed-denied, fixture:float-const-overflow.
- **implementation_coverage:** W25: `src/cmd/compile/internal/sgoadmit` (admission), `types2` (map keys), `ssa` (`useFMA`).
- **executed_evidence:** W25: conformance and the differential tests against Go (sgo/evidence/w25/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-FLOAT-003 — Conversions between float64 and integers
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-FLOAT-002, SGO-TYPE-002, SGO-TYPE-007, SGO-TYPE-008, SGO-VAL-001, SGO-VAL-004, SGO-DEF-003; D189.
- **grammar_and_types:** `float64(e)` where `e` is of type `int`, `byte`, or `rune` (or a user type over one); `int(f)`, `byte(f)`, and `rune(f)` where `f` is of type `float64` (or a user type over it); conversions between `float64` and user types over `float64`. `string(f)` is denied (text is `strconv.FormatFloat` and `fmt`).
- **preconditions:** None.
- **evaluation_and_value_semantics:** `float64(i)` is the binary64 value nearest `i`, ties to even (exact for |i| ≤ 2^53); it never fails. `int(f)` is `f` truncated toward zero, provided the result is in the range of `int` (-2^63 ≤ result < 2^63); `byte(f)` and `rune(f)` likewise, in the ranges of SGO-TYPE-007 and SGO-TYPE-008. A constant conversion follows SGO-VAL-004: the constant must be exactly representable in the target type.
- **ownership_and_lifecycle:** n/a — values.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** n/a — values.
- **allowed_nondeterminism:** None: never an implementation-defined result.
- **failure_and_partial_progress:** A conversion to an integer type of a NaN, an infinity, or a value whose truncation is outside the target's range fails with `IntegerOverflow` at the conversion's position, unwinding like `panic` (SGO-DEF-003, exit status 2 from the root).
- **cancellation_and_cleanup:** n/a — not a cancellation point.
- **compile_link_checks:** `SGO_E_TYPE_MISMATCH` for a constant conversion whose value is not an integer or not in the target's range; `SGO_E_UNADMITTED` for `string(f)` (SGO-TYPE-002).
- **runtime_guards:** A check before each conversion from `float64` to an integer type, against the exact bounds of the target type, NaN failing (`ssagen/sgo.go`).
- **tcb_assumptions:** As SGO-FLOAT-002.
- **positive_tests:** fixture:float-convert.
- **negative_tests:** fixture:float-convert-nan, fixture:float-convert-inf, fixture:float-convert-range, fixture:float-convert-byte-range, fixture:float-const-not-integral, fixture:float-string-denied.
- **implementation_coverage:** W25: `src/cmd/compile/internal/ssagen/sgo.go` (`sgoFloatConvCheck`), `sgoadmit`.
- **executed_evidence:** W25: conformance and the differential tests against Go (sgo/evidence/w25/README.md).
- **supported_execution_configurations:** production, lab.
