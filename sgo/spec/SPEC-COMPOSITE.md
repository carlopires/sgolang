# SPEC-COMPOSITE — Composite types

Value structs and arrays are admitted from spec 0.8.0 (SGO-COMP-002, SGO-COMP-003; W13 stage 1). Every other composite form stays denied (SGO-COMP-001).

### SGO-COMP-001 — Composite types outside SGO-COMP-002 are denied
- **version:** 3
- **status:** frozen
- **reference_closure:** SGO-ERR-001, SGO-COMP-002, SGO-COMP-003; D06, D20, D87.
- **grammar_and_types:** Denied: pointers and `&`/`*`, slices, maps, `make` (except `make` of a channel type, SGO-CHAN-002), `new`, `append`, `copy`, `delete`, `cap`, `clear`, slicing, string indexing, `range` over anything but a channel (SGO-CHAN-002) or an array (SGO-COMP-002), indexing of anything but an array (SGO-COMP-003), composite literals of types other than named struct types and array types, anonymous struct types, embedded fields, struct tags, blank fields, and struct fields or array elements of types that are not value types (SGO-COMP-002). No form of these constructs, packages, or APIs is admitted in any package compiled for the target, including dependencies and unreachable code.
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
- **positive_tests:** n/a — denial clause; SGO-COMP-002 and SGO-COMP-003 hold the admitted forms; slices, maps, and pointers are W13 stage 2 (D87; map order per D88).
- **negative_tests:** S16; fixture:type-slice, fixture:type-struct, fixture:type-pointer, fixture:val-string-index, fixture:ctl-range, fixture:val-string-range, fixture:comp-array-slice-denied, fixture:comp-anonymous-struct-denied, fixture:comp-struct-embed-denied.
- **implementation_coverage:** `sgoadmit/admit_expr.go` (syntax), `types2/sgo_flow.go` (`sgoRange`), `types2/sgo_values.go` (`sgoIndex`, `sgoValueType`).
- **executed_evidence:** conformance on darwin/arm64, linux/amd64, linux/arm64 (W03; W13 stage 1, sgo/evidence/w13/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-COMP-002 — Value structs and arrays
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-COMP-001, SGO-TYPE-001, SGO-BIND-003, SGO-VAL-005, SGO-MEM-002, SGO-CHAN-002, SGO-UTYPE-002, SGO-COMP-003; D86, D87, D89.
- **grammar_and_types:** A *value type* is `int`, `bool`, `string`, a struct type all of whose fields have value types, an array type `[N]T` whose element type T is a value type (N a non-negative constant of type int), or a named type (SGO-UTYPE-002) whose underlying type is a value type. Admitted: struct types `struct { F T; ... }` with value-type fields (exported or not, by the case of the name); array types as above; composite literals of these types, keyed (`T{F: v}`, `[N]T{i: v}`) or positional (`T{v1, v2}`, `[N]T{v1, v2}`, `[...]T{...}`), with omitted fields and elements set to their zero value; field selection `x.F` and field assignment `x.F = v`; element indexing `a[i]` and element assignment `a[i] = v` (SGO-COMP-003); `len(a)` of an array (a constant); `for i, v := range a` over an array (index then a copy of the element, in increasing index order); `==` and `!=` on values of the same struct or array type (fieldwise, elementwise, in declaration and index order; every value type is comparable). A struct or array is a *value*: assignment, passing as an argument, returning, sending, and capture copy the whole value. Value types are admitted as channel element types and as `sgo.Channel[T]` and permit payloads (amending SGO-CHAN-002, SGO-PERMIT-002). Still denied (SGO-COMP-001): fields or elements of any other type (function, channel, error, handle types), pointers, `&`, `new`, embedded fields, struct tags, anonymous struct types in type positions other than a named type's definition, blank (`_`) fields, slicing an array, and arrays used through a pointer.
- **preconditions:** n/a — static rules; dynamic indexing preconditions are in SGO-COMP-003.
- **evaluation_and_value_semantics:** The zero value of a struct has every field at its zero value; that of an array has every element at its zero value. A composite literal evaluates its element expressions left to right in source order (SGO-VAL-005), then builds the value. Assignment `x.F = v` and `a[i] = v` evaluate the operands as for other assignments (the index before the right-hand side), then replace that field or element only. Copying a value copies every field and element; no two variables ever share a struct or array.
- **ownership_and_lifecycle:** A struct or array value belongs to the variable or channel slot holding it. There is no reference to it, so it ends with its holder.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** Values cross tasks only as copies (send, commit, receive, capture). A captured variable of a value type is effectively final (SGO-MEM-002): a field or element assignment to it, anywhere, is an assignment to the variable. No new synchronization: value types are race-free for the same reason integers are (W13 model scenario `p3-value-jobs`, matrix G01).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Only indexing can fail (SGO-COMP-003). Building or copying a value cannot fail except by `ResourceExhausted` (SGO-ERR-*).
- **cancellation_and_cleanup:** n/a — no blocking operation; no cancellation point.
- **compile_link_checks:** `SGO_E_UNADMITTED` (SGO-COMP-001) for the still-denied forms above; `SGO_E_TYPE_MISMATCH` for ill-typed literals, fields, and comparisons; `SGO_E_CAPTURE_NOT_FINAL` (SGO-MEM-002) for a field or element assignment to a captured variable; duplicate fields and unknown field names are `SGO_E_TYPE_MISMATCH`. Contract digests (SGO-IMP-*) cover exported struct and array types, including field names, order, and types.
- **runtime_guards:** n/a — see SGO-COMP-003 for indexing.
- **tcb_assumptions:** None beyond the compiler's value copying (upstream code generation for value aggregates, under the existing TCB of SGO-TCB-001).
- **positive_tests:** G02; fixture:comp-struct-value, fixture:comp-array-value, fixture:comp-struct-compare, fixture:comp-value-send, fixture:comp-value-capture.
- **negative_tests:** G02, G04; fixture:comp-struct-func-field-denied, fixture:comp-struct-embed-denied, fixture:comp-array-slice-denied, fixture:comp-captured-field-write, fixture:comp-anonymous-struct-denied, fixture:comp-array-chan-elem-denied.
- **implementation_coverage:** admission: `src/cmd/compile/internal/sgoadmit/admit.go` (`typeDecl`, `structType`), `admit_expr.go` (array types, composite literals, element types); value-type rules: `src/cmd/compile/internal/types2/sgo_values.go` (`sgoValueType`, `sgoTypeDecls`, `sgoArrayType`, `sgoCompositeLit`, `sgoChanElemOK`); field and element assignment as assignment of the variable: `sgo_flow.go` (`sgoAssignedRoot`); code generation is upstream (value aggregates).
- **executed_evidence:** W13 stage 1 conformance (sgo/evidence/w13/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-COMP-003 — Array indexing and bounds
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-COMP-002, SGO-VAL-001, SGO-DEF-003, SGO-ERR-002; D91.
- **grammar_and_types:** `a[i]` where a is an array value or variable and i has type int. A constant index outside `0 ≤ i < len(a)` is a compile error.
- **preconditions:** `0 ≤ i < len(a)`, checked at run time when i is not constant.
- **evaluation_and_value_semantics:** a is evaluated, then i, then the bounds are checked, then the element is read or (for an assignment, after the right-hand side) written.
- **ownership_and_lifecycle:** n/a — no object is created.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** As SGO-COMP-002.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** An index outside the bounds is the checked failure `BoundsViolation` (a new kind of SGO-DEF-003), raised before any element is read or written; operands already evaluated stay evaluated. It unwinds like the integer failures of SGO-VAL-001, with the source position and the index and length in the diagnostic record (SGO-ERR-002).
- **cancellation_and_cleanup:** As any unwinding failure: defers run, the task fails, its nursery cancels the siblings (SGO-NUR-*).
- **compile_link_checks:** `SGO_E_CONST_RANGE` for a constant index out of range.
- **runtime_guards:** `BoundsViolation`.
- **tcb_assumptions:** The runtime's failure path for checked failures (SGO-DEF-003); the compiler's bounds check is never removed for S-Go packages unless it is proven redundant by the upstream prove pass (an elimination that never changes whether the failure occurs).
- **positive_tests:** G07; fixture:comp-array-index, fixture:comp-array-value.
- **negative_tests:** G07; fixture:comp-array-bounds-exit-2, fixture:comp-array-bounds-child, fixture:comp-array-const-index.
- **implementation_coverage:** `src/cmd/compile/internal/ssagen/ssa.go` (`boundsCheck`: in code from `.sgo` files the failure block calls `runtime.sgoBoundsViolation`), `ssagen/sgo.go` (`sgoBoundsFail`); `src/runtime/sgo_api.go` (`sgoBoundsViolation`); constant indexes: `types2/sgo.go` (`SGO_E_CONST_RANGE`); other operands: `types2/sgo_values.go` (`sgoIndex`).
- **executed_evidence:** W13 stage 1 conformance (sgo/evidence/w13/README.md).
- **supported_execution_configurations:** production, lab.
