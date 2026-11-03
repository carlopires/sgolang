# SPEC-COMPOSITE — Composite types

Value structs and arrays are admitted from spec 0.8.0 (SGO-COMP-002, SGO-COMP-003; W13 stage 1). Every other composite form stays denied (SGO-COMP-001).

### SGO-COMP-001 — Composite types outside SGO-COMP-002 are denied
- **version:** 4
- **status:** frozen
- **reference_closure:** SGO-ERR-001, SGO-COMP-002, SGO-COMP-003, SGO-COMP-004, SGO-COMP-005, SGO-COMP-006, SGO-COMP-007; D06, D20, D87, D93–D97.
- **grammar_and_types:** Denied (spec 0.9.0): pointer, slice, and map types of handle types and pointers to them (SGO-COMP-007); pointer conversions; `clear`; `len` and `cap` of channels (SGO-CHAN-002); `range` over a string, `[]rune`, and rune conversions; `range` over anything but a channel (SGO-CHAN-002), an array (SGO-COMP-002), a slice (SGO-COMP-005), or a map (SGO-COMP-006); map key types that are not value types (SGO-COMP-006); composite literals of types other than named struct types and array, slice, and map types; anonymous struct types, embedded fields, struct tags, blank fields, and struct fields or elements of types not admitted by SGO-COMP-002 and SGO-COMP-004. No form of these constructs, packages, or APIs is admitted in any package compiled for the target, including dependencies and unreachable code.
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
- **positive_tests:** n/a — denial clause; SGO-COMP-002 to SGO-COMP-008 hold the admitted forms.
- **negative_tests:** S16; fixture:type-struct, fixture:ctl-range, fixture:val-string-range, fixture:comp-anonymous-struct-denied, fixture:comp-struct-embed-denied, fixture:comp-string-range-denied, fixture:comp-map-key-denied, fixture:comp-pointer-handle-denied, fixture:chan-cap-denied.
- **implementation_coverage:** `sgoadmit/admit_expr.go` (syntax), `types2/sgo_flow.go` (`sgoRange`), `types2/sgo_values.go` (`sgoIndex`, `sgoValueType`).
- **executed_evidence:** conformance on darwin/arm64, linux/amd64, linux/arm64 (W03; W13 stage 1, sgo/evidence/w13/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-COMP-002 — Value structs and arrays
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-COMP-001, SGO-TYPE-001, SGO-BIND-003, SGO-VAL-005, SGO-MEM-002, SGO-CHAN-002, SGO-UTYPE-002, SGO-COMP-003; D86, D87, D89.
- **grammar_and_types:** From spec 0.9.0, struct fields and array elements may also have channel types and the task-local types of SGO-COMP-004 (pointers, slices, maps); a struct or array with such a field or element is not a value type, and one holding a channel has no `==` (D95). A *value type* is `int`, `bool`, `string`, a struct type all of whose fields have value types, an array type `[N]T` whose element type T is a value type (N a non-negative constant of type int), or a named type (SGO-UTYPE-002) whose underlying type is a value type. Admitted: struct types `struct { F T; ... }` with value-type fields (exported or not, by the case of the name); array types as above; composite literals of these types, keyed (`T{F: v}`, `[N]T{i: v}`) or positional (`T{v1, v2}`, `[N]T{v1, v2}`, `[...]T{...}`), with omitted fields and elements set to their zero value; field selection `x.F` and field assignment `x.F = v`; element indexing `a[i]` and element assignment `a[i] = v` (SGO-COMP-003); `len(a)` of an array (a constant); `for i, v := range a` over an array (index then a copy of the element, in increasing index order); `==` and `!=` on values of the same struct or array type (fieldwise, elementwise, in declaration and index order; every value type is comparable). A struct or array is a *value*: assignment, passing as an argument, returning, sending, and capture copy the whole value. Value types are admitted as channel element types and as `sgo.Channel[T]` and permit payloads (amending SGO-CHAN-002, SGO-PERMIT-002). Still denied (SGO-COMP-001): fields or elements of any other type (function, channel, error, handle types), pointers, `&`, `new`, embedded fields, struct tags, anonymous struct types in type positions other than a named type's definition, blank (`_`) fields, slicing an array, and arrays used through a pointer.
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
- **positive_tests:** G02; fixture:comp-struct-value, fixture:comp-array-value, fixture:comp-struct-compare, fixture:comp-value-send, fixture:comp-value-capture, fixture:comp-array-slice, fixture:comp-array-chan-elem.
- **negative_tests:** G02, G04; fixture:comp-struct-func-field-denied, fixture:comp-struct-embed-denied, fixture:comp-captured-field-write, fixture:comp-anonymous-struct-denied.
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

### SGO-COMP-004 — Task-local types and `sgo.Copy`
- **version:** 3
- **status:** frozen
- **reference_closure:** SGO-COMP-002, SGO-COMP-005, SGO-COMP-006, SGO-COMP-007, SGO-MEM-002, SGO-MEM-003, SGO-SPAWN-001, SGO-CHAN-002, SGO-PERMIT-002; D86, D89, D94, D95.
- **grammar_and_types:** A *task-local type* is a slice type, a map type, a pointer type, or a struct or array type with a field or element of a task-local type, or a named type whose underlying type is one of these. A *shareable type* is a value type (SGO-COMP-002), a channel type, `sgo.Frozen[T]` (SGO-COMP-009, spec 0.10.0), or a struct or array all of whose fields or elements are shareable (D95). `sgo.Copy[T any](x T) T` is admitted for every T that is task-local or shareable; for a shareable T it returns x. Task-local types are admitted as channel element types, `sgo.Channel[T]` payloads, fields, elements, parameters, results, and local variables. Four rules confine them to one task: (1) a function literal passed to `Spawn`, directly or through any function literal it contains, does not capture a variable of a task-local type; (2) a function literal that captures a variable of a task-local type (or a variable holding such a literal) is itself task-local: it may be called, deferred, or assigned to a local variable, which is then task-local; it is not passed as an argument, returned, or captured by a `Spawn` callback (D94); (3) the operand of a send statement, of a `select` send arm, and of `SendPermit.Commit` whose type is task-local is a call `sgo.Copy(...)`; (4) a value of a task-local type is never converted to an interface type (`error`): error values are shared between tasks, so only shareable types may implement `error` (spec 0.10.1, W13 review V2).
- **preconditions:** n/a — static rules.
- **evaluation_and_value_semantics:** `sgo.Copy(x)` returns a deep copy: every slice, map, and pointed-to object reachable from x is copied, in a depth-first walk in field, element, and insertion order; two references to one object in x (pointers of the same type to the same variable, the same map, or slices of the same element type with the same first element and capacity) are two references to one copied object in the result, and cycles are preserved; slices that overlap otherwise are copied separately, and a pointer to a field or element of an object and a pointer to (or slice of) the object itself are copied separately (spec 0.10.1, W13 review V1). Map copies keep insertion order (SGO-COMP-006). Channels and strings in x are shared, not copied. The result shares no mutable memory with x.
- **ownership_and_lifecycle:** Every object of a task-local type is owned by the task that created it, or by the task that received it as the result of `sgo.Copy` through a channel or permit. It lives as long as it is reachable.
- **effect_and_capability_requirements:** None. Handles are not task-local types and cannot be fields or elements (SGO-COMP-002).
- **concurrency_and_memory_order:** Rules (1)–(3) ensure no object of a task-local type is reachable from two tasks: values cross only as copies made by the sender before the send, which the receive then owns (happens-before of SGO-CHAN-002 and SGO-PERMIT-002). This is the property the sharing-rule model checks (G01: no access by a task other than the owner, on every interleaving of 17,362 accepted generated programs).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** `sgo.Copy` fails only by `ResourceExhausted`, before the send; the original is unchanged.
- **cancellation_and_cleanup:** `sgo.Copy` is not a cancellation point.
- **compile_link_checks:** `SGO_E_TASK_LOCAL_CAPTURE` for rules (1) and (2), at the captured name and naming the `Spawn` call or the escaping use; `SGO_E_UNCOPIED_SEND` for rule (3), at the operand, naming `sgo.Copy`.
- **runtime_guards:** n/a — static rules; `sgo.Copy` walks the runtime type of its argument.
- **tcb_assumptions:** `sgo.Copy` is implemented in the runtime (a typed deep copy over the runtime type descriptors) and is part of the TCB.
- **positive_tests:** G05; fixture:comp-copy-send, fixture:comp-copy-commit, fixture:comp-copy-deep, fixture:comp-copy-sharing, fixture:comp-copy-cycle, fixture:comp-local-closure-call, fixture:comp-map-copy-order, fixture:frozen-over-channel, fixture:comp-copy-interior-pointer, fixture:comp-error-shareable.
- **negative_tests:** G04, G05; fixture:comp-spawn-capture-slice, fixture:comp-spawn-capture-map, fixture:comp-spawn-capture-pointer, fixture:comp-spawn-capture-nested, fixture:comp-local-closure-arg, fixture:comp-local-closure-return, fixture:comp-uncopied-send, fixture:comp-uncopied-commit, fixture:comp-uncopied-select, fixture:comp-error-task-local, fixture:comp-error-task-local-return, fixture:comp-error-task-local-send, fixture:comp-error-pointer-receiver.
- **implementation_coverage:** `src/cmd/compile/internal/types2/sgo_local.go` (rules 1–3), `sgo_values.go` (`sgoTaskLocal`, `sgoFieldType`); `src/sgo/copy.go` (`Copy`), `src/runtime/sgo_copy.go` (`sgo_copy`).
- **executed_evidence:** W13 stage 2 conformance (sgo/evidence/w13/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-COMP-005 — Slices, `[]byte`, and string indexing
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-COMP-002, SGO-COMP-003, SGO-COMP-004, SGO-COMP-008, SGO-TYPE-007, SGO-VAL-002, SGO-VAL-005; D87, D91, D93.
- **grammar_and_types:** Slice types `[]T` for any admitted T other than a handle type. Constructors: slice literals `[]T{...}` (keyed or positional), `make([]T, n)` and `make([]T, n, c)` with n and c of type int, slicing `a[i:j]`, `a[i:]`, `a[:j]`, `a[:]`, and `a[i:j:k]` of a slice or of an addressable array variable, `[]byte(s)` for a string s. Operations: `s[i]` and `s[i] = v`, `len(s)`, `cap(s)`, `append(s, v...)` and `append(s, t...)` (including `append(b, str...)` for a `[]byte` b and string str), `copy(dst, src)` (also `copy(b, str)`), `clear(s)`, `for i := range s`, `for i, v := range s`, `s == nil`, `s != nil`, and `string(b)` for a `[]byte` b. Strings: `str[i]` (a byte), `str[i:j]` and the other two-index forms (a string). `range` over a string, `[]rune`, and conversions between strings and anything but `[]byte` stay denied. Slices are not comparable except to nil.
- **preconditions:** For `s[i]`: `0 ≤ i < len(s)`. For `a[i:j:k]`: `0 ≤ i ≤ j ≤ k ≤ cap(a)` (for strings and arrays, `len` in place of `cap`; omitted bounds are 0, `len(a)`, and `cap(a)`). For `make`: `0 ≤ n ≤ c`.
- **evaluation_and_value_semantics:** A slice is a view of an underlying array: a pointer, a length, and a capacity. Slicing and assignment copy the view, not the elements, so two slices of one array see each other's element writes (within one task, SGO-COMP-004). `append` of n elements writes in place when `len(s)+n ≤ cap(s)`, and otherwise allocates a new array whose capacity is at least `len(s)+n`, chosen by a fixed function of the element size, `cap(s)`, and n (the same on every run, target, and configuration), and copies; so whether the result shares the array with s is determined by the capacity, and programs that need independence use a fresh `make` or `sgo.Copy`. `copy` copies `min(len(dst), len(src))` elements as if through a temporary, and returns that count. `range` evaluates its operand once and visits indices 0..len−1 of that view in order. `make` zero-fills. A string index yields the byte at that offset; a string slice shares no mutable memory (strings are immutable). `[]byte(s)` and `string(b)` copy.
- **ownership_and_lifecycle:** As SGO-COMP-004: a slice and its array are owned by one task.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** Task-local (SGO-COMP-004). No synchronization is involved.
- **allowed_nondeterminism:** None: the capacity chosen by a growing `append` is a fixed function of its inputs, so aliasing after `append` is the same on every run and target.
- **failure_and_partial_progress:** An index or slice bound outside its precondition is `BoundsViolation` (SGO-COMP-003, SGO-COMP-008) before any element is read or written. `make` with n < 0, c < n, or c < 0 is `InvalidArgument`. Allocation failure is `ResourceExhausted`. An `append` that fails leaves s unchanged.
- **cancellation_and_cleanup:** No cancellation points.
- **compile_link_checks:** `SGO_E_CONST_RANGE` for constant bounds out of range or out of order; `SGO_E_UNADMITTED` for `range` over strings, `[]rune`, other string conversions, and slices of handle types; `SGO_E_TYPE_MISMATCH` for slice comparisons other than with nil.
- **runtime_guards:** `BoundsViolation`, `InvalidArgument`.
- **tcb_assumptions:** The runtime's slice allocation, growth (`growslice`, deterministic in its inputs), and copy routines.
- **positive_tests:** G10; fixture:comp-slice-basic, fixture:comp-slice-append-alias, fixture:comp-slice-three-index, fixture:comp-slice-copy, fixture:comp-slice-range, fixture:comp-bytes-string, fixture:comp-string-index-slice, fixture:comp-slice-of-array.
- **negative_tests:** G10, G07; fixture:comp-slice-bounds, fixture:comp-slice-index-bounds, fixture:comp-slice-make-negative, fixture:comp-string-range-denied, fixture:comp-slice-compare-denied, fixture:comp-slice-const-range.
- **implementation_coverage:** admission `sgoadmit/admit_expr.go`; `types2/sgo_values.go` (`sgoIndex`, `sgoConversion`), `sgo_flow.go` (`sgoRange`); bounds `ssagen/ssa.go` `boundsCheck` → `runtime.sgoSliceBoundsViolation`; `make` `walk/builtin.go` → `runtime.sgoInvalidMake`.
- **executed_evidence:** W13 stage 2 conformance (sgo/evidence/w13/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-COMP-006 — Maps with insertion order
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-COMP-002, SGO-COMP-004, SGO-COMP-008, SGO-LAB-001; D88, D91.
- **grammar_and_types:** Map types `map[K]V` where K is a value type (SGO-COMP-002; comparable) and V any admitted type other than a handle type. Constructors: map literals `map[K]V{k: v, ...}` and `make(map[K]V)` (an optional size hint `make(map[K]V, n)`, n of type int). Operations: `m[k]`, `v, ok := m[k]`, `m[k] = v` (and `op=`, `++`, `--`), `delete(m, k)`, `len(m)`, `clear(m)`, `for k := range m`, `for k, v := range m`, `m == nil`, `m != nil`. Maps are not comparable except to nil. A duplicate constant key in a literal is a compile error.
- **preconditions:** For a write (`m[k] = v`, `op=`, `++`, `--`): m is not nil. For `make` with a hint: n ≥ 0.
- **evaluation_and_value_semantics:** A map holds entries in *insertion order*: a new key is appended at the end; assigning to an existing key keeps its position; `delete` removes the entry, and a later insertion of the same key appends it at the end; `clear` empties the map. A read of a missing key yields the zero value of V (and `ok = false`). `range` visits entries in insertion order. An entry deleted before it is visited is not visited; an entry inserted during the range is visited after the entries that preceded it; assigning to a visited or unvisited key does not move it. This order is identical in every run, on every target, in production and in the lab (D88). Literal entries are inserted in source order. Map copies (`sgo.Copy`) keep the order.
- **ownership_and_lifecycle:** As SGO-COMP-004.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** Task-local (SGO-COMP-004).
- **allowed_nondeterminism:** None. (Go's randomized iteration order is not an S-Go behavior.)
- **failure_and_partial_progress:** A write to a nil map is `NilMapWrite` (SGO-COMP-008), before the right-hand side is stored (after it is evaluated). A negative hint is `InvalidArgument`. Allocation failure is `ResourceExhausted`; the map is unchanged.
- **cancellation_and_cleanup:** No cancellation points.
- **compile_link_checks:** `SGO_E_UNADMITTED` for key types that are not value types and for map values of handle types; `SGO_E_TYPE_MISMATCH` for duplicate constant keys and map comparisons other than with nil. Code of S-Go packages reaches maps only through the S-Go ordered-map entry points; the compiler lowers every map operation whose position is in a `.sgo` file to those entry points (`walk/sgo_map.go`), so no randomized-order path exists.
- **runtime_guards:** `NilMapWrite`, `InvalidArgument`.
- **tcb_assumptions:** The runtime's ordered map (a hash index over an insertion-ordered entry array) is part of the TCB.
- **positive_tests:** G06; fixture:comp-map-basic, fixture:comp-map-order, fixture:comp-map-delete-reinsert, fixture:comp-map-range-mutate, fixture:comp-map-struct-key, fixture:comp-map-copy-order, fixture:lab-map-order.
- **negative_tests:** G06; fixture:comp-map-nil-write, fixture:comp-map-key-denied, fixture:comp-map-dup-key, fixture:comp-map-compare-denied.
- **implementation_coverage:** `src/cmd/compile/internal/walk/sgo_map.go` and the map lowering in `walk/{walk,expr,builtin,range}.go`; `src/runtime/sgo_map.go` (ordered map: append-only slots with sequence numbers, open-addressing index, compaction and iteration by sequence number).
- **executed_evidence:** W13 stage 2 conformance (sgo/evidence/w13/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-COMP-007 — Pointers and pointer receivers
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-COMP-002, SGO-COMP-004, SGO-COMP-008, SGO-UTYPE-002, SGO-MEM-002; D87, D91, D96.
- **grammar_and_types:** Pointer types `*T` for any admitted T other than a handle type (handles are already pointers, SGO-TYPE-005). `&x` of an addressable operand (a local variable, a field or element of one, an element of a slice, a dereferenced pointer), `&T{...}` of a composite literal, and `new(T)`. `*p` and `*p = v`, field selection and element indexing through pointers (`p.F`, `p[i]` for a pointer to an array), `p == q`, `p != q`, `p == nil`, `p != nil`. Methods with pointer receivers `func (r *T) M(...)` on named types of the package (amending SGO-UTYPE-002); a method of `*T` is called on an addressable T value (`x.M()` means `(&x).M()`) and on a `*T`; a method of T is called on a `*T` (`p.M()` means `(*p).M()`). Method values and expressions stay denied. Pointer arithmetic, `unsafe`, and conversions between pointer types are denied (SGO-NATIVE-001).
- **preconditions:** For `*p`, `p.F`, `p[i]`, and a call of a T method through p: p is not nil.
- **evaluation_and_value_semantics:** `&x` refers to the variable x itself; writes through it change x. `&T{...}` and `new(T)` allocate a new object (zero-filled for `new`). Taking the address of a local variable makes it outlive its function if the pointer does; that changes nothing observable. Pointer equality is identity of the referenced variable.
- **ownership_and_lifecycle:** As SGO-COMP-004. A variable whose address is taken is task-local even if its type is a value type: SGO-MEM-002 already forbids writing a captured variable, and `&x` of a captured variable is denied (it would allow such a write).
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** Task-local (SGO-COMP-004).
- **allowed_nondeterminism:** None. Pointer values have no printed or ordered form.
- **failure_and_partial_progress:** A dereference of nil (also implicit, in `p.F`, `p[i]`, or a call through p) is `NilDereference` (SGO-COMP-008), before anything is read or written.
- **cancellation_and_cleanup:** No cancellation points.
- **compile_link_checks:** `SGO_E_UNADMITTED` for pointers to handle types and for pointer conversions; `SGO_E_CAPTURE_NOT_FINAL` for `&x` of a captured variable (SGO-MEM-002).
- **runtime_guards:** `NilDereference`.
- **tcb_assumptions:** The compiler's nil checks for S-Go code are never removed unless proven redundant (as for bounds checks, SGO-COMP-003).
- **positive_tests:** G03, G11; fixture:comp-pointer-basic, fixture:comp-pointer-field, fixture:comp-pointer-receiver, fixture:comp-pointer-list, fixture:comp-pointer-new, fixture:comp-pointer-equality.
- **negative_tests:** G11, G07; fixture:comp-pointer-nil-deref, fixture:comp-pointer-nil-method, fixture:comp-pointer-handle-denied, fixture:comp-pointer-captured-address, fixture:comp-pointer-captured-method.
- **implementation_coverage:** admission `sgoadmit/admit.go` (`recvOK`), `admit_expr.go`; `types2/sgo_local.go` (rule 3); nil checks `ssagen/sgo.go` (`sgoNilCheck` → `runtime.sgoNilDereference`).
- **executed_evidence:** W13 stage 2 conformance (sgo/evidence/w13/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-COMP-008 — Nil values and data failures
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-COMP-003, SGO-COMP-005, SGO-COMP-006, SGO-COMP-007, SGO-TYPE-002, SGO-DEF-003, SGO-ERR-002, SGO-WAIT-001; D91, D96.
- **grammar_and_types:** `nil` is a value of pointer, slice, and map types (amending SGO-TYPE-002, which admits it for `error` only; it stays denied for channels, function types, and handles). The zero value of a pointer, slice, or map field or element is nil.
- **preconditions:** n/a — the failures below are the outcomes of violated preconditions of SGO-COMP-005, SGO-COMP-006, and SGO-COMP-007.
- **evaluation_and_value_semantics:** A nil slice has length and capacity 0 and behaves as an empty slice in every operation (`append` to it allocates). A nil map behaves as an empty map for reads, `len`, `delete`, `clear`, and `range`.
- **ownership_and_lifecycle:** n/a — nil refers to nothing.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** n/a — nil refers to no shared object.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** The data failures are checked failure kinds of SGO-DEF-003 (amended): `BoundsViolation` (index or slice bounds, SGO-COMP-003, SGO-COMP-005), `NilDereference` (SGO-COMP-007), `NilMapWrite` (SGO-COMP-006); `make` with an invalid length, capacity, or hint is the existing `InvalidArgument` (SGO-WAIT-001). Each is raised before the operation has any effect, carries the source position and a message naming the operands (index and length; bounds and capacity), and unwinds like `panic`. None of them is ever reported as `RuntimeDefect` or with a Go traceback.
- **cancellation_and_cleanup:** As any unwinding failure.
- **compile_link_checks:** `SGO_E_UNADMITTED` for nil of other types (unchanged).
- **runtime_guards:** `BoundsViolation`, `NilDereference`, `NilMapWrite`, `InvalidArgument`.
- **tcb_assumptions:** The runtime failure path of SGO-DEF-003; the compiler emits S-Go failure calls (not Go's panics) for these checks in code from `.sgo` files.
- **positive_tests:** G07; fixture:comp-nil-slice-empty, fixture:comp-nil-map-read.
- **negative_tests:** G07; fixture:comp-slice-bounds, fixture:comp-pointer-nil-deref, fixture:comp-map-nil-write, fixture:comp-slice-make-negative, fixture:comp-nil-chan-denied.
- **implementation_coverage:** `types2/sgo.go` (`sgoNilable`); `runtime/sgo_api.go`, `runtime/sgo_map.go` (failure kinds).
- **executed_evidence:** W13 stage 2 conformance (sgo/evidence/w13/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-COMP-009 — Frozen values
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-COMP-002, SGO-COMP-004, SGO-COMP-005, SGO-COMP-006, SGO-COMP-007, SGO-MEM-003, SGO-CHAN-002, SGO-PERMIT-002, SGO-BIND-003; D86, D90, D98–D101.
- **grammar_and_types:** `sgo.Frozen[T]` is a type of package sgo for any admitted T that contains no handle type and no channel (D100). `sgo.Freeze(x T) sgo.Frozen[T]` and the method `f.Get() T` are its only operations. `f.Get()` is admitted only as the head of an *access path*: a chain of field selections, index expressions (`a[i]`, `m[k]`, `s[i:j]` is denied), and `len(...)` applied to it, or as the operand of `range` (D99). The value of an access path must be a value type (SGO-COMP-002), a `Frozen[U]`, or the operand of a further selection, index, `len`, or `range` within the same path. `range` over an access path binds iteration variables of value type only (the key of a map is always a value type, SGO-COMP-006). `sgo.Frozen[T]` has no `==`, no `nil`, and no zero value: a variable of it is initialized by `sgo.Freeze` or by another `Frozen[T]` (D101). It is a shareable type (SGO-COMP-004): a function literal of any task may capture it, and it is sent and committed without `sgo.Copy`. A `Frozen[U]` may be a struct field, an array, slice, or channel element, or a map value; a struct or array holding one is shareable when its other fields or elements are, and has no `==`.
- **preconditions:** n/a — static rule; `sgo.Freeze` has no precondition.
- **evaluation_and_value_semantics:** `sgo.Freeze(x)` evaluates x, makes the deep copy of SGO-COMP-004 (sharing and cycles preserved), and returns a `Frozen[T]` referring to it (D98). Later changes to x, or to anything x refers to, do not affect the frozen value. `f.Get()` yields the frozen value without copying it; an access path reads through it. Every read of a frozen value, by any task, at any time, observes the contents it had when `Freeze` returned.
- **ownership_and_lifecycle:** The frozen object is owned by no task; it lives as long as any `Frozen[T]` referring to it is reachable, and is reclaimed by the collector.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** No task can write a frozen object (the access-path rule makes every write a compile error), so concurrent reads by any number of tasks are race-free (SGO-MEM-003). The copy made by `Freeze` happens before `Freeze` returns, and a `Frozen[T]` reaches another task only by capture at `Spawn` or through a channel, both of which order it after the copy (SGO-SPAWN-001, SGO-CHAN-003). Runtime map reads and iteration do not write the map (SGO-COMP-006 implementation), so frozen maps are read without synchronization.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** `Freeze` fails only by `ResourceExhausted`. Reads through an access path fail as their operations do: `BoundsViolation`, `NilDereference` (SGO-COMP-008).
- **cancellation_and_cleanup:** n/a — `Freeze` and reads do not block.
- **compile_link_checks:** `SGO_E_FROZEN_WRITE` for any use of a frozen-derived task-local value outside an access path (assignment, argument, result, send, capture, `sgo.Copy`, write, `append`, `delete`, slicing); `SGO_E_UNADMITTED` for `Frozen[T]` with a handle or channel in T, `==` on `Frozen`, and `Get` as a method value; `SGO_E_TYPE_MISMATCH` for `nil` of a `Frozen` type; `SGO_E_UNINITIALIZED` for an uninitialized `Frozen` variable or named result.
- **runtime_guards:** None beyond SGO-COMP-008: the guarantee is static.
- **tcb_assumptions:** `sgo_copy` (SGO-COMP-004) produces a deep copy sharing nothing with its argument; runtime map reads and iteration do not write.
- **positive_tests:** G08; fixture:w13-p5-table (P5), fixture:frozen-over-channel, fixture:frozen-range-map, fixture:frozen-nested, fixture:frozen-caller-mutates, fixture:frozen-value-type.
- **negative_tests:** G08; fixture:frozen-write-field, fixture:frozen-write-map, fixture:frozen-append, fixture:frozen-escape-var, fixture:frozen-escape-arg, fixture:frozen-escape-return, fixture:frozen-escape-send, fixture:frozen-whole-escape, fixture:frozen-range-ref, fixture:frozen-addr, fixture:frozen-ptr-method, fixture:frozen-method-value, fixture:frozen-handle-denied, fixture:frozen-compare-denied, fixture:frozen-named-result.
- **implementation_coverage:** `src/sgo/frozen.go` (`Frozen`, `Freeze`, `Get`; the copy is `runtime.sgo_copy`); `src/cmd/compile/internal/types2/sgo_frozen.go` (`sgoFrozenPaths`: the access-path rule; `sgoFrozenElemOK`, `sgoFreezeCall`: D100), `sgo_values.go` (Frozen is shareable, no `==`), `sgo_flow.go` (named results); admission `sgoadmit/admit_expr.go`.
- **executed_evidence:** W13 stage 3 conformance (sgo/evidence/w13/README.md).
- **supported_execution_configurations:** production, lab.
