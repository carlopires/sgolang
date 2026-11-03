# W13 stage 1 — draft clauses (for owner approval)
These drafts are kept outside `sgo/spec/` on purpose: spec 0.7.0 is the release registry (D83), and adding drafts to it would change its admission digest. On approval they move into `SPEC-COMPOSITE.md` and `SPEC-USERTYPE.md` as frozen clauses together with new versions of SGO-COMP-001 (narrowed to what stays denied), SGO-UTYPE-001, SGO-CHAN-002 and SGO-PERMIT-002 (value element and payload types), and SGO-MEM-002 (a write to a field or element is a write to the variable), and the spec becomes 0.8.0 in one step. `spec-tool.py check` passed on this text when it was placed in `sgo/spec/` (field completeness, references, no implicit Go semantics).


### SGO-COMP-002 — Value structs and arrays
- **version:** 1
- **status:** draft
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
- **compile_link_checks:** `SGO_E_UNADMITTED` (SGO-COMP-001) for the still-denied forms above; `SGO_E_TYPE_MISMATCH` for ill-typed literals, fields, and comparisons; `SGO_E_FINAL_CAPTURE` (SGO-MEM-002) for a field or element assignment to a captured variable; duplicate fields and unknown field names are `SGO_E_TYPE_MISMATCH`. Contract digests (SGO-IMP-*) cover exported struct and array types, including field names, order, and types.
- **runtime_guards:** n/a — see SGO-COMP-003 for indexing.
- **tcb_assumptions:** None beyond the compiler's value copying (upstream code generation for value aggregates, under the existing TCB of SGO-TCB-001).
- **positive_tests:** G02; PENDING: fixtures comp-struct-value, comp-array-value, comp-struct-compare, comp-value-send, comp-value-capture, comp-array-range.
- **negative_tests:** G02; PENDING: fixtures comp-struct-func-field-denied, comp-struct-embed-denied, comp-array-slice-denied, comp-captured-field-write.
- **implementation_coverage:** PENDING: W13 stage 1 (types2 admission, conformance driver).
- **executed_evidence:** PENDING: W13 stage 1.
- **supported_execution_configurations:** production, lab.

### SGO-COMP-003 — Array indexing and bounds
- **version:** 1
- **status:** draft
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
- **positive_tests:** G07; PENDING: fixture comp-array-index.
- **negative_tests:** G07; PENDING: fixtures comp-array-bounds-exit-2, comp-array-bounds-child, comp-array-const-index.
- **implementation_coverage:** PENDING: W13 stage 1 (runtime failure kind, compiler hook).
- **executed_evidence:** PENDING: W13 stage 1.
- **supported_execution_configurations:** production, lab.


### SGO-UTYPE-002 — Named types and value-receiver methods
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-UTYPE-001, SGO-COMP-002, SGO-BIND-001, SGO-IMP-001; D87.
- **grammar_and_types:** Package-level type definitions `type T U`, where U is a value type (SGO-COMP-002), including a struct type literal; grouped declarations `type ( ... )` are admitted. T is a new named type with underlying type U, distinct from U and from every other named type. A value of T is assignable only to T; `T(x)` converts x to T when the underlying type of x's type is identical to U (and `U(t)` converts back); no other conversion to or from T exists, and none is implicit. Untyped constants are assignable to T when they are representable in U. Methods `func (r T) M(params) results` with a value receiver of a named type declared in the same package; the receiver may be unnamed or `_`. Method calls `x.M(...)` on values of type T. The method set of T is its declared methods; names of methods and fields of T must differ. Still denied (SGO-UTYPE-001): type aliases (`type T = U`), local (function-scope) type declarations, generic types, pointer receivers (stage 2), methods on types of other packages or on non-named types, method values (`x.M` not called) and method expressions (`T.M`), embedding, and interfaces (SPEC-IFACE).
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
- **positive_tests:** G03; PENDING: fixtures utype-named-int, utype-struct-methods, utype-cross-package, utype-conversion.
- **negative_tests:** G03; PENDING: fixtures utype-alias-denied, utype-pointer-receiver-denied, utype-method-value-denied, utype-local-type-denied, utype-foreign-method-denied.
- **implementation_coverage:** PENDING: W13 stage 1.
- **executed_evidence:** PENDING: W13 stage 1.
- **supported_execution_configurations:** production, lab.
