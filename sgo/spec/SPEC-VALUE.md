# SPEC-VALUE — Primitive values, conversions, equality, arithmetic and evaluation order

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §4 and decisions D02–D04.

### SGO-VAL-001 — Integer arithmetic
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-TYPE-001, SGO-DEF-003; D02; R3.
- **grammar_and_types:** Operators on `int` operands (both of type `int`): binary `+ - * / %`, bitwise `& | ^ &^`, shifts `<< >>` (right operand also `int`), unary `-` and `^` (bitwise complement), comparisons `== != < <= > >=` (result `bool`), and the statements `x op= e` and `x++`, `x--`. Unary `+` is admitted and is the identity.
- **preconditions:** For `/` and `%`: right operand ≠ 0. For shifts: `0 ≤ count ≤ 63`.
- **evaluation_and_value_semantics:** Results are the mathematical integer result, which must lie in [−2^63, 2^63−1]. `/` truncates toward zero; `%` satisfies `a == (a/b)*b + a%b` and has the sign of `a`. `<<` is multiplication by 2^count with the range check; `>>` is arithmetic (sign-propagating) shift and never overflows. Bitwise operators act on the two's-complement representation and never overflow. `x op= e` is `x = x op e` with `x` evaluated once. `x++` is `x += 1`; `x--` is `x -= 1`.
- **ownership_and_lifecycle:** n/a — values.
- **effect_and_capability_requirements:** n/a — pure operations.
- **concurrency_and_memory_order:** n/a — pure operations on task-local values.
- **allowed_nondeterminism:** None. Identical on all admitted targets.
- **failure_and_partial_progress:** An out-of-range result is a checked failure `IntegerOverflow`, including `MinInt / -1`, `MinInt % -1` (result would be 0 but the quotient overflows; S-Go treats this as `IntegerOverflow`), and `-MinInt`. Division or remainder by zero is `DivideByZero`. A shift count outside 0..63 is `ShiftRange`. The failure occurs before any assignment of the result; operands already evaluated stay evaluated.
- **cancellation_and_cleanup:** n/a — not a cancellation point.
- **compile_link_checks:** For constant operands the same conditions are compile errors (SGO-VAL-004). The compiler may remove a run-time check only when it proves the check cannot fail.
- **runtime_guards:** `IntegerOverflow`, `DivideByZero`, `ShiftRange`, each unwinding like `panic` (SGO-DEF-003) with the source position.
- **tcb_assumptions:** n/a — generated code; no runtime primitive beyond the failure path (SGO-TCB-001).
- **positive_tests:** S16; fixture:val-int-arith.
- **negative_tests:** S16; fixture:val-overflow-add, fixture:val-overflow-minint-div, fixture:val-div-zero, fixture:val-shift-range.
- **implementation_coverage:** W03: `src/cmd/compile/internal/ssagen/sgo.go`, `src/runtime/sgo_api.go`, `src/cmd/compile/internal/types2/sgo.go`.
- **executed_evidence:** conformance fixtures val-div-zero, val-int-arith, val-overflow-add, val-overflow-minint-div, val-shift-range (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-VAL-002 — Strings
- **version:** 3
- **status:** frozen
- **reference_closure:** SGO-TYPE-001, SGO-TYPE-008, SGO-COMP-005, SGO-ERR-003; D116, D117, D120.
- **grammar_and_types:** Operators on `string` operands: `+` (concatenation), comparisons `== != < <= > >=` (result `bool`), `s += e`, and the builtin `len(s) int`. From spec 0.9.0, `s[i]` (a `byte`), `s[i:j]`, and conversions between `string` and `[]byte` are admitted (SGO-COMP-005). From spec 0.12.0: `for i, r := range s`, `for i := range s`, and `for range s` (i an int, r a rune); `string(r)` of a rune; `[]rune(s)` and `string(rs)` (SGO-COMP-005). `string(i)` of an int and other conversions are denied (D120).
- **preconditions:** For `+`: the result length must not exceed the implementation limit recorded in the build metadata (at least 2^31−1 bytes).
- **evaluation_and_value_semantics:** Concatenation yields the bytes of the left operand followed by those of the right. Comparison is lexicographic by unsigned byte value; a proper prefix is less than the longer string. `len` is the number of bytes. `range` evaluates its operand once and visits its runes in order: i is the byte offset where each rune starts and r the decoded rune; an invalid or truncated sequence, an overlong form, or an encoded surrogate decodes to U+FFFD and advances one byte (D117), so a string of n bytes gives at most n iterations. `string(r)` is the UTF-8 encoding of r (1 to 4 bytes), or of U+FFFD for a surrogate (D116).
- **ownership_and_lifecycle:** Strings are immutable values; storage is managed by the runtime.
- **effect_and_capability_requirements:** n/a — pure operations.
- **concurrency_and_memory_order:** Immutable, therefore sendable.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Exceeding the length limit, or failing to allocate the result, is resource exhaustion (SGO-ERR-003). Decoding never fails: invalid input decodes to U+FFFD.
- **cancellation_and_cleanup:** n/a — not a cancellation point (nor is `range` over a string).
- **compile_link_checks:** `SGO_E_UNADMITTED` for `string(i)` of an int and for conversions between strings and types other than `[]byte`, `[]rune`, and rune.
- **runtime_guards:** Length limit check before allocation.
- **tcb_assumptions:** Runtime string allocation and concatenation primitives (SGO-TCB-001).
- **positive_tests:** S16; fixture:val-strings, fixture:val-string-index, fixture:comp-string-index-slice, fixture:val-string-range, fixture:val-string-range-invalid-utf8, fixture:val-string-range-index-only, fixture:val-string-of-rune.
- **negative_tests:** S16; fixture:type-string-of-int-denied, fixture:val-string-range-elem-type.
- **implementation_coverage:** W03: `src/cmd/compile/internal/ssagen/sgo.go`, `src/runtime/sgo_api.go`, `src/cmd/compile/internal/types2/sgo.go`; W15: `types2/sgo_flow.go` (`sgoRange`), `types2/sgo_values.go` (`sgoConversion`), runtime `decoderune`, `intstring`.
- **executed_evidence:** W03 (sgo/evidence/w03/README.md); W15 conformance (sgo/evidence/w15/README.md).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-VAL-003 — Booleans
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-TYPE-001, SGO-VAL-005.
- **grammar_and_types:** Operators on `bool` operands: `&&`, `||`, unary `!`, `==`, `!=`. Conditions of `if` and `for` and boolean `switch` tags have type `bool`.
- **preconditions:** n/a — total operations.
- **evaluation_and_value_semantics:** `a && b` evaluates `b` only if `a` is `true`; `a || b` evaluates `b` only if `a` is `false`.
- **ownership_and_lifecycle:** n/a — values.
- **effect_and_capability_requirements:** n/a — pure operations (except effects of evaluated operands).
- **concurrency_and_memory_order:** n/a — pure operations.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** n/a — cannot fail (operands may).
- **cancellation_and_cleanup:** n/a — not a cancellation point.
- **compile_link_checks:** `SGO_E_TYPE_MISMATCH` for non-`bool` operands or conditions.
- **runtime_guards:** n/a — cannot fail.
- **tcb_assumptions:** n/a — generated code.
- **positive_tests:** S16; fixture:val-bool.
- **negative_tests:** S16; fixture:val-int-condition.
- **implementation_coverage:** W03: `src/cmd/compile/internal/ssagen/sgo.go`, `src/runtime/sgo_api.go`, `src/cmd/compile/internal/types2/sgo.go`.
- **executed_evidence:** conformance fixtures val-bool, val-int-condition (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-VAL-004 — Constants
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-LEX-004, SGO-TYPE-002, SGO-VAL-001; R3.
- **grammar_and_types:** A constant expression consists of literals, constant names, `true`, `false`, the operators of SGO-VAL-001..003 applied to constant operands, and `len` of a constant string. Constants are untyped unless declared with a type (`const c int = 3`).
- **preconditions:** n/a — compile-time evaluation.
- **evaluation_and_value_semantics:** Untyped integer constants are evaluated exactly with unbounded precision; typed `int` constants must stay in the `int` range at every step. Division truncates toward zero. String and boolean constants follow SGO-VAL-002 and SGO-VAL-003.
- **ownership_and_lifecycle:** n/a — compile-time values.
- **effect_and_capability_requirements:** n/a — compile-time.
- **concurrency_and_memory_order:** n/a — compile-time.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Division by zero, a shift count outside 0..63 (or negative), a typed-constant overflow, or converting an unrepresentable untyped constant to `int` is a compile error.
- **cancellation_and_cleanup:** n/a — compile-time.
- **compile_link_checks:** `SGO_E_CONST_OVERFLOW`, `SGO_E_CONST_DIV_ZERO`, `SGO_E_CONST_SHIFT`, `SGO_E_UNADMITTED` (`iota`).
- **runtime_guards:** n/a — compile-time.
- **tcb_assumptions:** n/a — compiler only.
- **positive_tests:** S16; fixture:val-constants.
- **negative_tests:** S16; fixture:val-const-overflow, fixture:val-const-div-zero.
- **implementation_coverage:** W03: `src/cmd/compile/internal/ssagen/sgo.go`, `src/runtime/sgo_api.go`, `src/cmd/compile/internal/types2/sgo.go`.
- **executed_evidence:** conformance fixtures val-const-div-zero, val-const-overflow, val-constants (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-VAL-005 — Evaluation order
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-VAL-003, SGO-TYPE-003, SGO-MEM-001, SGO-SPAWN-001.
- **grammar_and_types:** n/a — dynamic rule over all expressions and statements.
- **preconditions:** n/a — dynamic rule.
- **evaluation_and_value_semantics:** Expressions are evaluated left to right, depth first: for a binary operator, the left operand completely, then the right (except the short-circuit cases of SGO-VAL-003); for a call, the function value (including a method receiver), then each argument left to right, then the call. In an assignment or declaration, every right-hand operand is evaluated, left to right, before any left-hand variable is assigned; then variables are assigned left to right. In `return e1, …, en`, operands are evaluated left to right before any nursery exit (SGO-CTL-004) and before deferred calls run. Because subset 0 has no pointers or shared mutable state, this order fully determines every result.
- **ownership_and_lifecycle:** n/a — dynamic rule.
- **effect_and_capability_requirements:** Effects of calls happen in this order.
- **concurrency_and_memory_order:** Within one task, evaluation is sequential in this order.
- **allowed_nondeterminism:** None within a task.
- **failure_and_partial_progress:** If an operand fails, operands to its left have been evaluated (with their effects), operands to its right have not, and no assignment of that statement has happened.
- **cancellation_and_cleanup:** n/a — order is unaffected by cancellation.
- **compile_link_checks:** The compiler must not reorder evaluation in an observable way (effects, failure point, failure kind).
- **runtime_guards:** n/a — dynamic rule.
- **tcb_assumptions:** n/a — generated code.
- **positive_tests:** S16, X05; fixture:val-eval-order.
- **negative_tests:** S16; fixture:val-eval-order-failure-point.
- **implementation_coverage:** W03: `src/cmd/compile/internal/ssagen/sgo.go`, `src/runtime/sgo_api.go`, `src/cmd/compile/internal/types2/sgo.go`.
- **executed_evidence:** conformance fixtures val-eval-order, val-eval-order-failure-point (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
