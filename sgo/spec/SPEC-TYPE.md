# SPEC-TYPE — Type system, function types, checked callable/effect summaries

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §3 and decisions D01, D04–D06.

### SGO-TYPE-001 — Admitted types
- **version:** 3
- **status:** frozen
- **reference_closure:** SGO-TYPE-003, SGO-TYPE-005, SGO-TYPE-006, SGO-PERMIT-002, SGO-READY-002, SGO-AUTH-003; D04, D05, D06, D28, D29.
- **grammar_and_types:** The admitted types are exactly: `bool`; `int`, a 64-bit two's-complement signed integer on every target; `string`, an immutable sequence of bytes; function types `func(P1, …, Pn) R` or `func(P1, …, Pn) (R1, …, Rm)` whose parameter and result types are admitted (SGO-TYPE-003); `error` (SGO-TYPE-006); and the pointer types of the capability types of SGO-TYPE-005 (`*sgo.Context`, `*sgo.Nursery`, `*sgo.Writer`, `*sgo.Ready`, `*sgo.Clock`, `*sgo.WallClock`, `*sgo.Random`, `*sgo.Entropy`, `*sgo.SimStore`, and `*sgo.Channel[T]`, `*sgo.SendPermit[T]` for the element types of SGO-PERMIT-002; spec 0.3.0 adds `*sgo.Event`, `*sgo.CapacityLimiter`, `*sgo.Token` of SGO-SYNC-002); and the channel types `chan T`, `<-chan T`, `chan<- T` of SGO-CHAN-002. Any other type name, type literal (array, slice, map, struct, pointer, interface; channel types other than those of SGO-CHAN-002), type parameter, or type declaration is denied. Other integer widths, floating-point, complex, `byte`, `rune`, `uintptr`, and `any` are denied.
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
- **positive_tests:** S16; fixture:type-admitted.
- **negative_tests:** S16, S17; fixture:type-float, fixture:type-slice, fixture:type-struct, fixture:type-interface, fixture:type-generic.
- **implementation_coverage:** none yet — W03 (types2 admission filter).
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-TYPE-002 — Identity and assignability
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-TYPE-001, SGO-VAL-004.
- **grammar_and_types:** Two types are identical if they are the same predeclared or opaque type, or function types with the same number of parameters and results and pairwise identical parameter and result types (parameter names do not matter). A value `v` is assignable to a variable of type `T` only if (a) `v`'s type is identical to `T`; or (b) `v` is an untyped constant representable as a value of `T`, where untyped integer constants are representable as `int` within its range, untyped string constants as `string`, untyped boolean constants as `bool`; or (c) `v` is the predeclared `nil` and `T` is `error`. These rules apply to assignment, initialization, argument passing, results, and comparison operands. There are no implicit conversions and no explicit conversion expressions.
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
- **positive_tests:** S16; fixture:type-assign.
- **negative_tests:** S16; fixture:type-mismatch, fixture:type-nil-func, fixture:type-conversion.
- **implementation_coverage:** none yet — W03.
- **executed_evidence:** none yet.
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
- **implementation_coverage:** none yet — W03.
- **executed_evidence:** none yet.
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
- **implementation_coverage:** none yet — W03.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-TYPE-005 — Opaque capability types
- **version:** 3
- **status:** frozen
- **reference_closure:** SGO-CAP-001, SGO-PROC-001, SGO-NUR-001, SGO-SPAWN-003, SGO-PROC-003, SGO-READY-002, SGO-PERMIT-002, SGO-AUTH-001, SGO-AUTH-003.
- **grammar_and_types:** `sgo.Context`, `sgo.Nursery`, `sgo.Writer`, `sgo.Ready` (SGO-READY-002), the effect handles `sgo.Clock`, `sgo.WallClock`, `sgo.Random`, `sgo.Entropy`, `sgo.SimStore` (SGO-AUTH-003), the instances `sgo.Channel[T]`, `sgo.SendPermit[T]` (SGO-PERMIT-002), and `sgo.Event`, `sgo.CapacityLimiter`, `sgo.Token` (SGO-SYNC-002) are opaque types usable only behind `*` (for example `*sgo.Context`). User code may declare variables and parameters of these types, pass and capture them, and call their methods (SGO-CAP-001). It may not write the type without `*`, construct values, dereference, take addresses, compare (including with `nil`), or use them in any expression other than method calls, assignment, argument passing, and capture.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** Values come only from the runtime: `root` (SGO-PROC-001), the nursery binder (SGO-NUR-001), the `Spawn` callback parameter (SGO-SPAWN-003), `Context.Stdout` (SGO-PROC-003), the `Start` callback parameters (SGO-READY-002), the root accessors, attenuation, and `LendTo` of the effect handles (SGO-AUTH-002, SGO-AUTH-003), `sgo.NewChannel` and `Reserve` (SGO-PERMIT-002), and `sgo.NewEvent`, `sgo.NewCapacityLimiter`, and `Acquire` (SGO-SYNC-002). Channel values (SGO-CHAN-002) are not capability handles but follow the same no-nil and no-comparison rules. Copies refer to the same underlying object.
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
- **implementation_coverage:** none yet — W03.
- **executed_evidence:** none yet.
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
- **implementation_coverage:** none yet — W03.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
