# SPEC-TYPE — Type system, function types, checked callable/effect summaries

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §3 and decisions D01, D04–D06.

### SGO-TYPE-001 — Admitted types
- **version:** 8
- **status:** frozen
- **reference_closure:** SGO-TYPE-003, SGO-TYPE-005, SGO-TYPE-006, SGO-PERMIT-002, SGO-READY-002, SGO-AUTH-003, SGO-FILE-002, SGO-FILE-003, SGO-NET-002, SGO-TYPE-007, SGO-TYPE-008; D04, D05, D06, D28, D29.
- **grammar_and_types:** The admitted types are exactly: `bool`; `byte` (SGO-TYPE-007, spec 0.9.0); `rune` (SGO-TYPE-008, spec 0.12.0); pointer, slice, and map types (SGO-COMP-005, SGO-COMP-006, SGO-COMP-007, spec 0.9.0); `sgo.Frozen[T]` (SGO-COMP-009, spec 0.10.0); `int`, a 64-bit two's-complement signed integer on every target; `string`, an immutable sequence of bytes; function types `func(P1, …, Pn) R` or `func(P1, …, Pn) (R1, …, Rm)` whose parameter and result types are admitted (SGO-TYPE-003); `error` (SGO-TYPE-006); and the pointer types of the capability types of SGO-TYPE-005 (`*sgo.Context`, `*sgo.Nursery`, `*sgo.Writer`, `*sgo.Ready`, `*sgo.Clock`, `*sgo.WallClock`, `*sgo.Random`, `*sgo.Entropy`, `*sgo.SimStore`, and `*sgo.Channel[T]`, `*sgo.SendPermit[T]` for the element types of SGO-PERMIT-002; spec 0.3.0 adds `*sgo.Event`, `*sgo.CapacityLimiter`, `*sgo.Token` of SGO-SYNC-002; spec 0.5.0 adds `*sgo.Dir`, `*sgo.Listing`, and `*sgo.File` of SGO-FILE-002 and SGO-FILE-003 and `*sgo.Conn` and `*sgo.Listener` of SGO-NET-002; spec 0.6.0 adds `*sgo.T` of SGO-TEST-002); and the channel types `chan T`, `<-chan T`, `chan<- T` of SGO-CHAN-002. Any other type name, type literal (array, slice, map, struct, pointer, interface; channel types other than those of SGO-CHAN-002), type parameter, or type declaration is denied. Other integer widths (including `int32` and `uint8`), floating-point, complex, `uintptr`, and `any` are denied.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** `int` arithmetic is defined by SGO-VAL-001, strings by SGO-VAL-002, booleans by SGO-VAL-003.
- **ownership_and_lifecycle:** Values of all admitted types are copied by assignment; `string` and function values may share immutable backing storage, which is never observable.
- **effect_and_capability_requirements:** Only the capability types carry authority (SGO-CAP-001).
- **concurrency_and_memory_order:** All admitted types are sendable (SGO-TYPE-004).
- **allowed_nondeterminism:** None. `int` has the same size and behavior on darwin/arm64, linux/amd64, and linux/arm64 (D22).
- **failure_and_partial_progress:** Use of a denied type is a compile error.
- **cancellation_and_cleanup:** n/a — static rule.
- **compile_link_checks:** `SGO_E_UNADMITTED` naming the denied type or type form.
- **runtime_guards:** n/a — static rule.
- **tcb_assumptions:** The runtime represents `int` as 64 bits on every admitted target.
- **positive_tests:** S16; fixture:type-admitted, fixture:type-slice, fixture:type-pointer.
- **negative_tests:** S16, S17; fixture:type-float, fixture:type-struct, fixture:type-interface, fixture:type-generic, fixture:type-int32-denied.
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/sgoadmit/admit.go`, `src/cmd/compile/internal/sgoadmit/admit_expr.go`.
- **executed_evidence:** conformance fixtures type-admitted, type-float, type-generic, type-interface, type-slice, type-struct (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-TYPE-002 — Identity and assignability
- **version:** 3
- **status:** frozen
- **reference_closure:** SGO-TYPE-001, SGO-VAL-004, SGO-COMP-002, SGO-UTYPE-002.
- **grammar_and_types:** Pointer, slice, and map types are identical when their element (and key) types are identical; `nil` is assignable to pointer, slice, and map types (SGO-COMP-008, spec 0.9.0); pointers compare by identity, and slices and maps compare only with `nil`. Two types are identical if they are the same predeclared or opaque type; the same named type (SGO-UTYPE-002), which is identical only to itself; array types with the same length and identical element types; struct type literals with the same sequence of fields, pairwise with the same names and identical types (two such types with an unexported field are identical only within one package); or function types with the same number of parameters and results and pairwise identical parameter and result types (parameter names do not matter). A value `v` is assignable to a variable of type `T` only if (a) `v`'s type is identical to `T`; or (b) `v` is an untyped constant representable as a value of `T`, where untyped integer constants are representable as `int` within its range, untyped string constants as `string`, untyped boolean constants as `bool`; or (c) `v` is the predeclared `nil` and `T` is `error`. These rules apply to assignment, initialization, argument passing, results, and comparison operands. There are no implicit conversions; the only explicit conversion expressions are those of SGO-UTYPE-002 (between types with identical underlying types).
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** An untyped constant takes the target type at the point of assignment. An untyped constant used where no type is given defaults to `int`, `string`, or `bool`.
- **ownership_and_lifecycle:** n/a — static rule.
- **effect_and_capability_requirements:** n/a — static rule.
- **concurrency_and_memory_order:** n/a — static rule.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** A non-assignable value is a compile error.
- **cancellation_and_cleanup:** n/a — static rule.
- **compile_link_checks:** `SGO_E_TYPE_MISMATCH`, `SGO_E_CONST_OVERFLOW`, `SGO_E_UNADMITTED` (conversion syntax).
- **runtime_guards:** n/a — static rule.
- **tcb_assumptions:** n/a — compiler only.
- **positive_tests:** S16; fixture:type-assign, fixture:comp-pointer-equality, fixture:comp-nil-slice-empty.
- **negative_tests:** S16; fixture:type-mismatch, fixture:type-nil-func, fixture:type-conversion, fixture:comp-slice-compare-denied, fixture:comp-map-compare-denied.
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/sgoadmit/admit.go`, `src/cmd/compile/internal/sgoadmit/admit_expr.go`.
- **executed_evidence:** conformance fixtures type-assign, type-conversion, type-mismatch, type-nil-func (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-TYPE-003 — Function values
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-TYPE-001, SGO-MEM-002, SGO-VAL-005; R3.
- **grammar_and_types:** A function value is a named package-level function, a function literal `func(params) results { body }`, or a variable of function type. Calls are `f(args)` with exactly as many arguments as parameters; a call to a function with multiple results may be used only as the whole right side of an assignment or declaration with matching count, or as a `return` operand list of the same types. Variadic parameters are denied. Function values cannot be compared (not even to `nil`), and there is no `nil` function value.
- **preconditions:** The called value is a function value; with SGO-BIND-003 every function-typed variable is initialized.
- **evaluation_and_value_semantics:** A call evaluates the function value, then the arguments left to right (SGO-VAL-005), then runs the body with parameters bound to copies of the arguments. Results are returned by value.
- **ownership_and_lifecycle:** A function literal's captured variables live as long as any value of that function literal (SGO-MEM-002).
- **effect_and_capability_requirements:** A call performs exactly the effects of its body; it has no implicit authority. Capability use is visible as uses of capability values (SGO-CAP-001).
- **concurrency_and_memory_order:** Function values are sendable because their captures are effectively final (SGO-MEM-002).
- **allowed_nondeterminism:** None beyond what the body does.
- **failure_and_partial_progress:** A failure in the body unwinds through the call (SGO-DEF-003); arguments already evaluated are not undone.
- **cancellation_and_cleanup:** A call is not a cancellation point; cancellation is observed only at SGO-CANCEL-002 and SGO-WAIT-001 points in the body.
- **compile_link_checks:** `SGO_E_ARG_COUNT`, `SGO_E_TYPE_MISMATCH`, `SGO_E_UNADMITTED` (variadic, function comparison).
- **runtime_guards:** n/a — no nil function values exist.
- **tcb_assumptions:** Closure representation and calling convention of the upstream compiler (SPEC-TCB).
- **positive_tests:** X11; fixture:type-func-values.
- **negative_tests:** X11; fixture:type-variadic, fixture:type-func-compare.
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/sgoadmit/admit.go`, `src/cmd/compile/internal/sgoadmit/admit_expr.go`.
- **executed_evidence:** conformance fixtures type-func-compare, type-func-values, type-variadic (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-TYPE-004 — Sendability
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-MEM-002, SGO-MEM-003, SGO-SPAWN-001; D01.
- **grammar_and_types:** A type is sendable if a value of it can be observed by more than one task without a data race. In subset 0 every admitted type is sendable: `bool`, `int`, `string`, and `error` values are immutable; capability handles are internally synchronized (SGO-TCB-001); function values are sendable because their captured variables are effectively final (SGO-MEM-002). A value may reach another task only through a `Spawn` callback's captures or through values passed by such a callback.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** n/a — static rule.
- **ownership_and_lifecycle:** n/a — static rule.
- **effect_and_capability_requirements:** n/a — static rule.
- **concurrency_and_memory_order:** This rule, with SGO-MEM-001 and SGO-MEM-002, is what makes data races impossible in subset 0 (SGO-MEM-003).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** n/a — no non-sendable type exists in subset 0; later subsets add the check.
- **cancellation_and_cleanup:** n/a — static rule.
- **compile_link_checks:** None needed in subset 0 beyond SGO-MEM-002; the rule is recorded so later subsets cannot add a shared mutable type without a sendability check.
- **runtime_guards:** n/a — static rule.
- **tcb_assumptions:** Capability handles are safe for concurrent use (SGO-TCB-001).
- **positive_tests:** S16; fixture:mem-share-immutable.
- **negative_tests:** S16; fixture:mem-capture-mutated.
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/sgoadmit/admit.go`, `src/cmd/compile/internal/sgoadmit/admit_expr.go`.
- **executed_evidence:** conformance fixtures mem-capture-mutated, mem-share-immutable (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-TYPE-005 — Opaque capability types
- **version:** 5
- **status:** frozen
- **reference_closure:** SGO-CAP-001, SGO-PROC-001, SGO-NUR-001, SGO-SPAWN-003, SGO-PROC-003, SGO-READY-002, SGO-PERMIT-002, SGO-AUTH-001, SGO-AUTH-003, SGO-FILE-002, SGO-FILE-003, SGO-NET-002.
- **grammar_and_types:** `sgo.Context`, `sgo.Nursery`, `sgo.Writer`, `sgo.Ready` (SGO-READY-002), the effect handles `sgo.Clock`, `sgo.WallClock`, `sgo.Random`, `sgo.Entropy`, `sgo.SimStore` (SGO-AUTH-003), the instances `sgo.Channel[T]`, `sgo.SendPermit[T]` (SGO-PERMIT-002), and `sgo.Event`, `sgo.CapacityLimiter`, `sgo.Token` (SGO-SYNC-002), and the I/O handles `sgo.Dir`, `sgo.Listing`, `sgo.File` (SGO-FILE-002, SGO-FILE-003), `sgo.Conn`, and `sgo.Listener` (SGO-NET-002), and the test handle `sgo.T` (SGO-TEST-002) are opaque types usable only behind `*` (for example `*sgo.Context`). User code may declare variables and parameters of these types, pass and capture them, and call their methods (SGO-CAP-001). It may not write the type without `*`, construct values, dereference, take addresses, compare (including with `nil`), or use them in any expression other than method calls, assignment, argument passing, and capture.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** Values come only from the runtime: `root` (SGO-PROC-001), the nursery binder (SGO-NUR-001), the `Spawn` callback parameter (SGO-SPAWN-003), `Context.Stdout` (SGO-PROC-003), the `Start` callback parameters (SGO-READY-002), the root accessors, attenuation, and `LendTo` of the effect handles (SGO-AUTH-002, SGO-AUTH-003), `sgo.NewChannel` and `Reserve` (SGO-PERMIT-002), and `sgo.NewEvent`, `sgo.NewCapacityLimiter`, and `Acquire` (SGO-SYNC-002), and `root.Dir` with the `Dir` methods, `root.Dial`, `root.Listen`, `Accept`, and the I/O handles' `LendTo` (SGO-FILE-002, SGO-FILE-003, SGO-NET-002). Channel values (SGO-CHAN-002) are not capability handles but follow the same no-nil and no-comparison rules. Copies refer to the same underlying object.
- **ownership_and_lifecycle:** The runtime owns the underlying objects. A handle may outlive the object's active period (for example a nursery after it closes); using it then is checked at run time by the method's clause.
- **effect_and_capability_requirements:** Holding a handle is the authority to use it (SGO-CAP-001).
- **concurrency_and_memory_order:** Handles are internally synchronized (SGO-TCB-001).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Violations of the static rules are compile errors; use of a handle outside its valid period is the run-time failure named in the method's clause.
- **cancellation_and_cleanup:** n/a — static rule; method clauses define cancellation behavior.
- **compile_link_checks:** `SGO_E_CAPABILITY_MISUSE` for construction, dereference, comparison, or `nil` of a capability type.
- **runtime_guards:** Per method (SGO-SPAWN-002, SGO-CANCEL-001, SGO-PROC-003).
- **tcb_assumptions:** The runtime never hands out a handle to an object of a different kind (SGO-TCB-001).
- **positive_tests:** S06; fixture:cap-pass-handles.
- **negative_tests:** S06; fixture:cap-nil-context, fixture:cap-compare-nursery, fixture:sync-nil-event-denied, fixture:sync-token-compare-denied.
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/sgoadmit/admit.go`, `src/cmd/compile/internal/sgoadmit/admit_expr.go`.
- **executed_evidence:** conformance fixtures cap-compare-nursery, cap-nil-context, cap-pass-handles, sync-nil-event-denied, sync-token-compare-denied (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-TYPE-006 — Sealed error
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-TYPE-002, SGO-CANCEL-003, SGO-SPAWN-004.
- **grammar_and_types:** `error` is a predeclared type. Its values are `nil`, values returned by `sgo.NewError(msg string) error`, and values returned by `sgo` functions (cancellation errors, SGO-CANCEL-003; write errors, SGO-PROC-003). Admitted operations: `e == nil`, `e != nil`, `nil == e`, `nil != e`, and `e.Error() string`. Comparing two non-nil-literal error operands, defining types that implement `error`, wrapping, and type assertions are denied.
- **preconditions:** `e.Error()` requires `e != nil`.
- **evaluation_and_value_semantics:** `sgo.NewError(msg)` returns a new non-nil error whose `Error()` is `msg`. `e.Error()` returns the message the error was created with.
- **ownership_and_lifecycle:** Error values are immutable.
- **effect_and_capability_requirements:** n/a — pure values.
- **concurrency_and_memory_order:** Immutable, therefore sendable.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Calling `Error()` on a `nil` error is a checked failure of kind `NilError` that unwinds like `panic` (SGO-DEF-003).
- **cancellation_and_cleanup:** n/a — pure values.
- **compile_link_checks:** `SGO_E_UNADMITTED` (error comparison, user error types, assertions).
- **runtime_guards:** `NilError` on `Error()` of `nil`.
- **tcb_assumptions:** n/a — error values are ordinary runtime objects.
- **positive_tests:** S08; fixture:type-errors.
- **negative_tests:** S08; fixture:type-error-compare, fixture:type-nil-error-call.
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/sgoadmit/admit.go`, `src/cmd/compile/internal/sgoadmit/admit_expr.go`.
- **executed_evidence:** conformance fixtures type-error-compare, type-errors, type-nil-error-call (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-TYPE-007 — The byte type
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-TYPE-001, SGO-TYPE-002, SGO-TYPE-008, SGO-VAL-001, SGO-COMP-005; D93, D120.
- **grammar_and_types:** `byte` is admitted: an integer type with values 0..255. Byte operands combine with byte operands under the operators of SGO-VAL-001 (shifts with an int count); untyped constants representable in 0..255 are assignable to byte. `int(b)` converts a byte to int; `byte(i)` converts an int to byte. Byte constants are written as integer literals or, when representable, as rune literals (`b == 'a'`, SGO-LEX-004 v2). `rune(b)` and `byte(r)` convert between byte and rune (SGO-TYPE-008). `uint8` and the other sized integer types stay denied (SGO-TYPE-001).
- **preconditions:** For `byte(i)`: `0 ≤ i ≤ 255`. For `byte(r)`: `r ≤ 255`.
- **evaluation_and_value_semantics:** Byte arithmetic is exact: the mathematical result of `+`, `-`, `*`, `/`, `%`, `<<` on bytes is the result if it lies in 0..255. `&`, `|`, `^`, `&^`, and `>>` cannot leave the range.
- **ownership_and_lifecycle:** n/a — a value type.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** n/a — a value type.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** A result outside 0..255, and `byte(i)` or `byte(r)` with an operand outside 0..255, is `IntegerOverflow` (SGO-VAL-001); division by zero is `DivideByZero`; a shift count outside 0..63 is `ShiftRange`.
- **cancellation_and_cleanup:** n/a — byte operations are not cancellation points.
- **compile_link_checks:** `SGO_E_CONST_OVERFLOW` for constants outside 0..255; `SGO_E_TYPE_MISMATCH` for mixing byte and int without a conversion.
- **runtime_guards:** `IntegerOverflow`, `DivideByZero`, `ShiftRange`.
- **tcb_assumptions:** The checked-arithmetic code generation of SGO-VAL-001, extended to 8-bit unsigned operands.
- **positive_tests:** G12; fixture:type-byte-arith, fixture:type-byte-convert.
- **negative_tests:** G12; fixture:type-byte-overflow, fixture:type-byte-underflow, fixture:type-byte-shift-overflow, fixture:type-byte-convert-overflow, fixture:type-byte-const-overflow, fixture:type-rune-to-byte-overflow.
- **implementation_coverage:** `noder/writer.go` (`sgoChecked`: int, byte, rune, and named types over them), `ssagen/sgo.go` (`sgoByteBinary`, `sgoConvCheck`), `ssagen/ssa.go` (OCONV).
- **executed_evidence:** W13 stage 2 conformance (sgo/evidence/w13/README.md); W15 (sgo/evidence/w15/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-TYPE-008 — The rune type
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-TYPE-001, SGO-TYPE-002, SGO-TYPE-007, SGO-VAL-001, SGO-VAL-002, SGO-VAL-004, SGO-LEX-004, SGO-COMP-005; D106, D116, D118, D120.
- **grammar_and_types:** `rune` is admitted: an integer type with values 0..0x10FFFF. Rune operands combine with rune operands under the operators of SGO-VAL-001 (shifts with an int count). Untyped constants (integer or rune) representable in 0..0x10FFFF are assignable to rune; a rune literal is an untyped rune constant whose default type is rune, assignable to int, byte, or rune (and named types over them) when representable (D118). Conversions (D120): `rune(i)` of an int, `int(r)`, `rune(b)` of a byte, `byte(r)`. Runes are comparable and ordered (map keys, `switch` cases). Named types over rune are admitted like named types over int. `rune` is its own admitted name: `int32` and the other sized integer types stay denied (SGO-TYPE-001).
- **preconditions:** For `rune(i)`: `0 ≤ i ≤ 0x10FFFF`. For `byte(r)`: `r ≤ 255`.
- **evaluation_and_value_semantics:** Rune arithmetic is exact: the mathematical result of `+`, `-`, `*`, `/`, `%`, `<<` on runes is the result if it lies in 0..0x10FFFF. `&`, `|`, `^`, `&^`, and `>>` cannot leave the range. Surrogate values (0xD800..0xDFFF) are ordinary rune values (D116); encoding one as text yields U+FFFD (SGO-VAL-002).
- **ownership_and_lifecycle:** n/a — a value type.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** A value type: shareable and sendable (SGO-TYPE-004).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** A result outside 0..0x10FFFF, `rune(i)` with i outside that range, and `byte(r)` with r > 255 are `IntegerOverflow` (SGO-VAL-001); division by zero is `DivideByZero`; a shift count outside 0..63 is `ShiftRange`.
- **cancellation_and_cleanup:** n/a — rune operations are not cancellation points.
- **compile_link_checks:** `SGO_E_CONST_OVERFLOW` for a constant of rune type outside 0..0x10FFFF; `SGO_E_TYPE_MISMATCH` for mixing rune with int or byte without a conversion; `SGO_E_UNADMITTED` for `string(i)` of an int (D120) and for `int32`.
- **runtime_guards:** `IntegerOverflow`, `DivideByZero`, `ShiftRange`.
- **tcb_assumptions:** The checked-arithmetic code generation of SGO-VAL-001 and SGO-TYPE-007, extended to a 32-bit operand with the upper bound 0x10FFFF.
- **positive_tests:** fixture:type-rune-arith, fixture:type-rune-convert, fixture:type-rune-untyped-const, fixture:type-rune-map-switch, fixture:type-rune-named, fixture:lex-rune-literals.
- **negative_tests:** fixture:type-rune-overflow, fixture:type-rune-underflow, fixture:type-rune-convert-overflow, fixture:type-rune-convert-negative, fixture:type-rune-to-byte-overflow, fixture:type-rune-shift-overflow, fixture:type-rune-const-overflow, fixture:type-rune-const-negative, fixture:type-rune-int-mismatch, fixture:type-string-of-int-denied, fixture:type-int32-denied.
- **implementation_coverage:** `sgoadmit/admit_expr.go` (rune admitted, int32 denied by name), `types2/sgo_values.go` (`sgoTypeRec`, `sgoConversion`), `types2/sgo_flow.go` (`sgoRuneConsts`), `noder/writer.go` (`sgoChecked`), `ssagen/sgo.go` (`sgoByteBinary`, `sgoNeg`, `sgoConvCheck`).
- **executed_evidence:** W15 conformance (sgo/evidence/w15/README.md).
- **supported_execution_configurations:** production, lab.
