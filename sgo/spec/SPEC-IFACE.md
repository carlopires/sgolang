# SPEC-IFACE — Interfaces

Named interface types, `any`, implicit implementation, dynamic dispatch, comma-ok type assertions, and type switches are admitted from spec 0.14.0 (W17; D107, D127–D133, `sgo/docs/w17-drafts.md`). Every interface type other than `error` is task-local (SGO-COMP-004). The forms that make Go's interfaces error-prone stay denied (SGO-IFACE-001).

### SGO-IFACE-001 — Interface forms that stay denied
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-IFACE-002, SGO-IFACE-004, SGO-ERR-001, SGO-GEN-001; D05, D108, D127, D128, D129, D131.
- **grammar_and_types:** Denied: interface type literals outside a type declaration (D127); type-set elements, `~T`, unions, and embedded non-interface types, which make a constraint interface (generics, D108); `==` and `!=` between interface values, or between values holding interface values in a field or element, except comparison of an interface value with `nil` (D128); interface types as map keys; the single-value type assertion `x.(T)` (D129); method values and method expressions of interface types (D131). Version 1 denied interface types, dynamic dispatch, comma-ok assertions, and type switches; spec 0.14.0 admits them (SGO-IFACE-002 to SGO-IFACE-004).
- **preconditions:** n/a — denial clause; there is nothing to use.
- **evaluation_and_value_semantics:** n/a — denied constructs never execute.
- **ownership_and_lifecycle:** n/a — denied constructs never execute.
- **effect_and_capability_requirements:** n/a — denied constructs never execute.
- **concurrency_and_memory_order:** n/a — denied constructs never execute.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Any use is a compile error; no object file is produced.
- **cancellation_and_cleanup:** n/a — denied constructs never execute.
- **compile_link_checks:** `SGO_E_UNADMITTED`, naming the construct and this clause (the map-key refusal names SGO-COMP-006's key rule).
- **runtime_guards:** n/a — rejected before execution.
- **tcb_assumptions:** n/a — no runtime involvement.
- **positive_tests:** n/a — denial clause.
- **negative_tests:** S10; fixture:type-interface, fixture:type-error-compare, fixture:iface-literal-denied, fixture:iface-compare-denied, fixture:iface-map-key-denied, fixture:iface-assert-single-denied, fixture:iface-method-value-denied, fixture:iface-method-expr-denied, fixture:iface-union-denied, fixture:iface-constraint-denied, fixture:lex-denied-operators.
- **implementation_coverage:** `sgoadmit` (literals, type-set elements, single-value assertions); types2 `sgoComparisonOK`, `sgoTypeDecls`, `sgoMethodValue`, and the map-key rule of `sgoTypeRec`.
- **executed_evidence:** W17 conformance on darwin/arm64, linux/amd64, linux/arm64 (`sgo/evidence/w17/README.md`).
- **supported_execution_configurations:** production and lab.

### SGO-IFACE-002 — Interface types and implementation
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IFACE-001, SGO-IFACE-003, SGO-COMP-004, SGO-COMP-007, SGO-TYPE-006, SGO-BIND-004, SGO-UTYPE-002; D107, D127, D131, D132.
- **grammar_and_types:** A package-level type declaration `type I interface { M(params) results; ...; J }` declares a named interface type: methods with admitted parameter and result types, and embedded named interface types J. `any` is the empty interface. A type implements I when its method set includes every method of I: the method set of a named type `T` is its value-receiver methods, and that of `*T` adds the pointer-receiver methods (D131). Assignment, variable initialization, argument passing, return, send, and composite-literal elements convert a value implicitly to an interface type it implements; explicit conversions to interface types are denied (SGO-UTYPE-002). Two methods of one name with different signatures, through embedding, are a compile error. Interface types may be the types of struct fields, slice and array elements, map values, channel elements, parameters, and results; they are never value types (SGO-COMP-002), so never map keys.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** An interface value is nil, or a pair of a dynamic type and a dynamic value. Converting a value copies it into the interface value, as an assignment would; converting a pointer stores the pointer, so the interface value refers to the pointed-to variable. Converting a nil pointer gives the nil interface value: S-Go has no typed nil (D130). `nil` is a value of every interface type.
- **ownership_and_lifecycle:** The dynamic value of an interface value is owned by the task that holds the interface value.
- **effect_and_capability_requirements:** n/a — interface values carry no authority of their own; a capability handle is never an interface's dynamic value through an implicit conversion (SGO-TYPE-005 denies the conversions).
- **concurrency_and_memory_order:** Every interface type other than `error` is task-local whatever its dynamic value (D132): it is sent only as `sgo.Copy(x)`, never moved, captured by a spawned task, frozen, or converted to `error`, and SGO-COMP-004's rules for task-local types apply. `sgo.Copy` of an interface value deep-copies its dynamic value; `sgo.Copy` of nil is nil. `error` stays shareable (SGO-TYPE-006); a struct with an `error` field stays shareable, and one with another interface field is task-local. The sharing model checks this rule over every interleaving (`sgo/model/sharemodel.go`, kind `SIface`).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** n/a — static rule; a violation is a compile error.
- **cancellation_and_cleanup:** n/a — no runtime behavior of its own.
- **compile_link_checks:** `SGO_E_TYPE_MISMATCH` for a value that does not implement the target interface type, naming the missing method; `SGO_E_REDECLARED` (SGO-BIND-004) for a method declared twice through embedding; `SGO_E_UNCOPIED_SEND`, `SGO_E_TASK_LOCAL_CAPTURE`, and the other SGO-COMP-004 codes for interface values.
- **runtime_guards:** n/a — static rule.
- **tcb_assumptions:** Interface values use Go's itab and type-descriptor representation and the runtime's deep copy (`sgo_copy.go`, `copyIface`).
- **positive_tests:** fixture:iface-basic, fixture:iface-embed, fixture:iface-any, fixture:iface-pointer-receiver, fixture:iface-copy-send, fixture:iface-error-field, fixture:std-sort-interface; model: TestShareScenarios, TestShareIfaceSoundness.
- **negative_tests:** fixture:iface-not-implemented, fixture:iface-value-receiver-only, fixture:iface-send-denied, fixture:iface-capture-denied, fixture:iface-to-error-denied, fixture:iface-duplicate-method; model: TestShareIfaceGeneratorFindsMutations.
- **implementation_coverage:** `sgoadmit` interface declarations; types2 `sgoTaskLocal`, `sgoTypeRec`, `sgoTypeToIface`, `sgoNilable`; runtime `sgo_copy.go` and `sgo.Copy`.
- **executed_evidence:** W17 conformance on darwin/arm64, linux/amd64, linux/arm64 (`sgo/evidence/w17/README.md`).
- **supported_execution_configurations:** production and lab.

### SGO-IFACE-003 — Dynamic dispatch and nil
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IFACE-002, SGO-COMP-007, SGO-DEF-003, SGO-VAL-005, SGO-TYPE-006; D130, D131.
- **grammar_and_types:** `x.M(args)` for an interface-typed x and a method M of its type.
- **preconditions:** x is not nil.
- **evaluation_and_value_semantics:** The receiver x and the arguments are evaluated as for any call (SGO-VAL-005); the call then runs the dynamic type's method M with the dynamic value as receiver: the value itself for a value-receiver method, the pointer for a pointer-receiver method.
- **ownership_and_lifecycle:** n/a — a call does not change ownership.
- **effect_and_capability_requirements:** The effects of the method called.
- **concurrency_and_memory_order:** n/a — a call within one task.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** A method call on a nil interface value is the failure `NilDereference` at the call (SGO-COMP-007, SGO-DEF-003; message "method call on a nil interface value"); no part of the method runs. Because a nil pointer converts to the nil interface (D130), this is also the outcome for an interface built from a nil pointer, where Go would call the method with a nil receiver. A method call on a nil `error` keeps the failure `NilError` of SGO-TYPE-006.
- **cancellation_and_cleanup:** n/a — no cancellation point of its own.
- **compile_link_checks:** n/a — the failure is at run time.
- **runtime_guards:** The compiler checks the interface's type word before the call in S-Go code and calls `runtime.sgoNilInterfaceCall` with the position when it is nil.
- **tcb_assumptions:** The itab layout of the toolchain (`ssagen.getClosureAndRcvr`).
- **positive_tests:** fixture:iface-dispatch, fixture:iface-nil-pointer-is-nil, fixture:iface-pointer-receiver.
- **negative_tests:** fixture:iface-nil-call, fixture:type-nil-error-call.
- **implementation_coverage:** `walk.walkConvInterface` (nil pointer to nil interface), `ssagen.sgoNilIfaceCheck`, `runtime.sgoNilInterfaceCall`.
- **executed_evidence:** W17 conformance on darwin/arm64, linux/amd64, linux/arm64 (`sgo/evidence/w17/README.md`).
- **supported_execution_configurations:** production and lab.

### SGO-IFACE-004 — Type assertions and type switches
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IFACE-001, SGO-IFACE-002, SGO-CTL-001, SGO-COMP-004; D129, D132.
- **grammar_and_types:** `v, ok := x.(T)`, `v, ok = x.(T)`, and `var v, ok = x.(T)` for an interface-typed x, where T is a type that implements x's interface type, or an interface type. `switch v := x.(type) { case T1, T2: ...; case nil: ...; default: ... }` and `switch x.(type) { ... }`. An assertion or case whose concrete T cannot implement x's interface type is a compile error; a type listed in two cases is a compile error.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** Comma-ok: ok is true, and v is the dynamic value (for an interface T, the interface value re-typed as T), when x is not nil and its dynamic type is T or implements the interface type T; otherwise ok is false and v is T's zero value. Type switch: x is evaluated once; the first case whose type matches is chosen, `case nil` matches a nil x, and `default` runs otherwise. In a case with exactly one type, v has that type; in other cases v has x's type.
- **ownership_and_lifecycle:** An asserted v of a pointer type refers to the same variable as the interface value; both stay with the owning task (D132).
- **effect_and_capability_requirements:** n/a — no effects.
- **concurrency_and_memory_order:** n/a — within one task.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** None: an assertion never fails at run time.
- **cancellation_and_cleanup:** n/a — no cancellation point.
- **compile_link_checks:** `SGO_E_UNADMITTED` for the single-value form (SGO-IFACE-001); `SGO_E_TYPE_MISMATCH` for an impossible T; `SGO_E_DUPLICATE_CASE` for a repeated case type.
- **runtime_guards:** n/a — no failure path.
- **tcb_assumptions:** Go's assertion and type-switch lowering (`runtime.typeAssert`, `runtime.interfaceSwitch`, and the itab caches).
- **positive_tests:** fixture:iface-assert-ok, fixture:iface-type-switch, fixture:iface-type-switch-nil, fixture:iface-any.
- **negative_tests:** fixture:iface-assert-impossible, fixture:iface-switch-duplicate, fixture:iface-assert-single-denied.
- **implementation_coverage:** `sgoadmit` (comma-ok forms, type switches); types2 assertion and type-switch checks; Go's lowering.
- **executed_evidence:** W17 conformance on darwin/arm64, linux/amd64, linux/arm64 (`sgo/evidence/w17/README.md`).
- **supported_execution_configurations:** production and lab.
