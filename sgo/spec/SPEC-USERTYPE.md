# SPEC-USERTYPE — User-defined types and methods

Named value types and value-receiver methods are admitted from spec 0.8.0 (SGO-UTYPE-002; W13 stage 1). Every other form stays denied (SGO-UTYPE-001).

### SGO-UTYPE-001 — User-defined types and methods outside SGO-UTYPE-002 are denied
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-ERR-001, SGO-UTYPE-002; D05, D87.
- **grammar_and_types:** Denied: type aliases, function-scope type declarations, type definitions whose underlying type is not a value type (SGO-COMP-002), pointer receivers, methods of types of other packages or of package sgo, method values and method expressions of user-declared types, and embedding. No form of these constructs, packages, or APIs is admitted in any package compiled for the target, including dependencies and unreachable code.
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
- **positive_tests:** n/a — denial clause; SGO-UTYPE-002 holds the admitted forms; pointer receivers are W13 stage 2 (D87).
- **negative_tests:** S16; fixture:utype-method, fixture:utype-alias-denied, fixture:utype-local-type-denied, fixture:utype-pointer-receiver-denied, fixture:utype-method-value-denied.
- **implementation_coverage:** `sgoadmit/admit.go` (`typeDecl`, `recvOK`, function-scope declarations), `types2/sgo_values.go` (`sgoTypeDecls`, `sgoMethodValue`).
- **executed_evidence:** conformance on darwin/arm64, linux/amd64, linux/arm64 (W03; W13 stage 1, sgo/evidence/w13/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-UTYPE-002 — Named types and value-receiver methods
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-UTYPE-001, SGO-COMP-002, SGO-BIND-001, SGO-IMP-001, SGO-AUTH-001, SGO-TYPE-002; D87.
- **grammar_and_types:** Package-level type definitions `type T U`, where U is a value type (SGO-COMP-002), including a struct type literal; grouped declarations `type ( ... )` are admitted. T is a new named type with underlying type U, distinct from U and from every other named type. A value of T is assignable only to T; `T(x)` converts x to T when the underlying type of x's type is identical to U (and `U(t)` converts back); no other conversion to or from T exists, and none is implicit. Untyped constants are assignable to T when they are representable in U. Methods `func (r T) M(params) results` with a value receiver of a named type declared in the same package; the receiver may be unnamed or `_`. Method calls `x.M(...)` on values of type T. The method set of T is its declared methods; names of methods and fields of T must differ. Still denied (SGO-UTYPE-001): type aliases (`type T = U`), local (function-scope) type declarations, generic types, pointer receivers (stage 2), methods on types of other packages or on non-named types, method values (`x.M` not called) and method expressions (`T.M`) of user-declared types (method values of package sgo's handles stay admitted by their own clauses, for example SGO-AUTH-001), embedding, and interfaces (SPEC-IFACE).
- **preconditions:** n/a — static rules.
- **evaluation_and_value_semantics:** A method call evaluates the receiver expression, then the arguments left to right, then calls the method with a copy of the receiver; the method cannot change the caller's value. The zero value of T is the zero value of U.
- **ownership_and_lifecycle:** As SGO-COMP-002: values of named types are values.
- **effect_and_capability_requirements:** A method needs no capability by itself; effects inside it follow their own clauses (SPEC-AUTH, SPEC-FILE, SPEC-NET), through capabilities passed as parameters.
- **concurrency_and_memory_order:** As SGO-COMP-002. A method's receiver is a copy, so calls from different tasks never share it.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** As any function call.
- **cancellation_and_cleanup:** As any function call.
- **compile_link_checks:** `SGO_E_UNADMITTED` for the still-denied forms; `SGO_E_TYPE_MISMATCH` for missing conversions and duplicate method or field names; contract digests cover exported named types and their exported methods' signatures.
- **runtime_guards:** n/a — static.
- **tcb_assumptions:** None.
- **positive_tests:** G03; fixture:utype-named-int, fixture:utype-struct-methods, fixture:utype-cross-package, fixture:utype-conversion, fixture:utype-declaration.
- **negative_tests:** G03; fixture:utype-alias-denied, fixture:utype-pointer-receiver-denied, fixture:utype-method-value-denied, fixture:utype-local-type-denied, fixture:utype-conversion-denied, fixture:utype-method.
- **implementation_coverage:** admission: `sgoadmit/admit.go` (`typeDecl`, `recvOK`); `types2/sgo_values.go` (`sgoTypeDecls`, `sgoMethodValue`, `sgoConversion`); contract digests of exported types and methods: `noder/sgo_identity.go` (`sgoContract`).
- **executed_evidence:** W13 stage 1 conformance (sgo/evidence/w13/README.md).
- **supported_execution_configurations:** production, lab.
