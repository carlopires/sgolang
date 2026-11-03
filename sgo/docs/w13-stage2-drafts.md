# W13 stage 2 — draft clauses (for owner approval)

Stage 2 of D87: slices, `[]byte`, string indexing and slicing, maps with insertion order (D88),
pointers and pointer receivers, all **task-local**, crossing tasks only through `sgo.Copy` (D86,
D89), with defined failures for bounds, nil, and invalid lengths (D91). Kept outside `sgo/spec/`
until approved, as for stage 1; on approval they are frozen together at spec 0.9.0.

The rules follow the sharing-rule model (`sgo/model/sharemodel.go`, matrix G01): `Ref` there is a
*task-local type* here, `SCopy` is `sgo.Copy`, and the model's rejected scenarios are the
diagnostics `SGO_E_TASK_LOCAL_CAPTURE` and `SGO_E_UNCOPIED_SEND`.

## Decisions needed with these drafts (D93–D97)

Each is recorded in `sgo/SGO_DECISIONS.md` as PROPOSED with this recommendation.

| ID | Question | Recommendation |
|---|---|---|
| D93 | How do bytes appear? `[]byte` and `s[i]` need an element type, and `byte` is denied today. | Admit `byte` as a type with checked arithmetic: a result outside 0..255 is `IntegerOverflow`, like `int`. `int(b)` always succeeds; `byte(i)` fails with `IntegerOverflow` when i is outside 0..255. No `rune`: `range` over a string stays denied; iterate `[]byte(s)` instead. |
| D94 | A function literal that captures a task-local variable could reach another task as a function value (passed to a helper that spawns it). | Such a literal is *task-local* itself: it may be called, deferred, or assigned to a local variable (which is then task-local too), but not passed as an argument, returned, or captured by a `Spawn` callback (`SGO_E_TASK_LOCAL_CAPTURE`). Function values that capture nothing task-local are unaffected. |
| D95 | May structs and arrays hold channels? P2's request carries its reply channel. | Yes: a channel type becomes a *shareable* field and element type. A struct or array of value and channel types is shareable: copied like a value, and its channels are shared, as channels already are. Channels stay incomparable, so such a struct has no `==`. |
| D96 | Is `nil` a value of pointer, slice, and map types? Zero values of fields produce nil anyway. | Yes, for those three kinds only (still not for channels or handles): `nil` literals and `== nil` / `!= nil`. Reading a nil slice or map behaves as empty; writing a nil map is `NilMapWrite`; dereferencing a nil pointer is `NilDereference` (D91). |
| D97 | P1 as written captures `sgo.Copy(in)` in a `Spawn` callback: that is a *move into a closure*, which is stage 4. | Rewrite P1 for stage 2: each worker's input is a value array (`[3]string`), so the capture is a copy; the partial maps still return through a channel with `sgo.Copy`. The narrow move stays in stage 4 (D87). |

## Clauses

### SGO-COMP-004 — Task-local types and `sgo.Copy`
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-COMP-002, SGO-COMP-005, SGO-COMP-006, SGO-COMP-007, SGO-MEM-002, SGO-MEM-003, SGO-SPAWN-001, SGO-CHAN-002, SGO-PERMIT-002; D86, D89, D94, D95.
- **grammar_and_types:** A *task-local type* is a slice type, a map type, a pointer type, or a struct or array type with a field or element of a task-local type, or a named type whose underlying type is one of these. A *shareable type* is a value type (SGO-COMP-002), a channel type, or a struct or array all of whose fields or elements are shareable (D95). `sgo.Copy[T any](x T) T` is admitted for every T that is task-local or shareable; for a shareable T it returns x. Task-local types are admitted as channel element types, `sgo.Channel[T]` payloads, fields, elements, parameters, results, and local variables. Three rules confine them to one task: (1) a function literal passed to `Spawn`, directly or through any function literal it contains, does not capture a variable of a task-local type; (2) a function literal that captures a variable of a task-local type (or a variable holding such a literal) is itself task-local: it may be called, deferred, or assigned to a local variable, which is then task-local; it is not passed as an argument, returned, or captured by a `Spawn` callback (D94); (3) the operand of a send statement, of a `select` send arm, and of `SendPermit.Commit` whose type is task-local is a call `sgo.Copy(...)`.
- **preconditions:** n/a — static rules.
- **evaluation_and_value_semantics:** `sgo.Copy(x)` returns a deep copy: every slice, map, and pointed-to object reachable from x is copied, in a depth-first walk in field, element, and insertion order; two references to one object in x are two references to one copied object in the result, and cycles are preserved. Map copies keep insertion order (SGO-COMP-006). Channels and strings in x are shared, not copied. The result shares no mutable memory with x.
- **ownership_and_lifecycle:** Every object of a task-local type is owned by the task that created it, or by the task that received it as the result of `sgo.Copy` through a channel or permit. It lives as long as it is reachable.
- **effect_and_capability_requirements:** None. Handles are not task-local types and cannot be fields or elements (SGO-COMP-002).
- **concurrency_and_memory_order:** Rules (1)–(3) ensure no object of a task-local type is reachable from two tasks: values cross only as copies made by the sender before the send, which the receive then owns (happens-before of SGO-CHAN-002 and SGO-PERMIT-002). This is the property the sharing-rule model checks (G01: no access by a task other than the owner, on every interleaving of 17,362 accepted generated programs).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** `sgo.Copy` fails only by `ResourceExhausted`, before the send; the original is unchanged.
- **cancellation_and_cleanup:** `sgo.Copy` is not a cancellation point.
- **compile_link_checks:** `SGO_E_TASK_LOCAL_CAPTURE` for rules (1) and (2), at the captured name and naming the `Spawn` call or the escaping use; `SGO_E_UNCOPIED_SEND` for rule (3), at the operand, naming `sgo.Copy`.
- **runtime_guards:** n/a — static rules; `sgo.Copy` walks the runtime type of its argument.
- **tcb_assumptions:** `sgo.Copy` is implemented in the runtime (a typed deep copy over the runtime type descriptors) and is part of the TCB.
- **positive_tests:** G05; PENDING: fixtures comp-copy-send, comp-copy-commit, comp-copy-deep, comp-copy-sharing, comp-copy-cycle, comp-local-closure-call.
- **negative_tests:** G04, G05; PENDING: fixtures comp-spawn-capture-slice, comp-spawn-capture-map, comp-spawn-capture-pointer, comp-spawn-capture-nested, comp-local-closure-arg, comp-local-closure-return, comp-uncopied-send, comp-uncopied-commit, comp-uncopied-select.
- **implementation_coverage:** PENDING: W13 stage 2 (types2 rules; runtime deep copy).
- **executed_evidence:** PENDING: W13 stage 2.
- **supported_execution_configurations:** production, lab.

### SGO-COMP-005 — Slices, `[]byte`, and string indexing
- **version:** 1
- **status:** draft
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
- **positive_tests:** G10; PENDING: fixtures comp-slice-basic, comp-slice-append-alias, comp-slice-three-index, comp-slice-copy, comp-slice-range, comp-bytes-string, comp-string-index-slice, comp-slice-of-array.
- **negative_tests:** G10, G07; PENDING: fixtures comp-slice-bounds, comp-slice-make-negative, comp-string-range-denied, comp-slice-compare-denied, comp-slice-const-range.
- **implementation_coverage:** PENDING: W13 stage 2.
- **executed_evidence:** PENDING: W13 stage 2.
- **supported_execution_configurations:** production, lab.

### SGO-COMP-006 — Maps with insertion order
- **version:** 1
- **status:** draft
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
- **compile_link_checks:** `SGO_E_UNADMITTED` for key types that are not value types and for map values of handle types; `SGO_E_TYPE_MISMATCH` for duplicate constant keys and map comparisons other than with nil. Code of S-Go packages reaches maps only through the S-Go ordered-map entry points; the compiler fails with an internal error if Go's map entry points are requested for an S-Go package, so no randomized-order path exists (as SGO-CHAN-002 does for channels).
- **runtime_guards:** `NilMapWrite`, `InvalidArgument`.
- **tcb_assumptions:** The runtime's ordered map (a hash index over an insertion-ordered entry array) is part of the TCB.
- **positive_tests:** G06; PENDING: fixtures comp-map-basic, comp-map-order, comp-map-delete-reinsert, comp-map-range-mutate, comp-map-struct-key, comp-map-copy-order, lab-map-order.
- **negative_tests:** G06; PENDING: fixtures comp-map-nil-write, comp-map-key-denied, comp-map-dup-key, comp-map-compare-denied.
- **implementation_coverage:** PENDING: W13 stage 2.
- **executed_evidence:** PENDING: W13 stage 2.
- **supported_execution_configurations:** production, lab.

### SGO-COMP-007 — Pointers and pointer receivers
- **version:** 1
- **status:** draft
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
- **positive_tests:** G03, G11; PENDING: fixtures comp-pointer-basic, comp-pointer-field, comp-pointer-receiver, comp-pointer-list, comp-pointer-new, comp-pointer-equality.
- **negative_tests:** G11, G07; PENDING: fixtures comp-pointer-nil-deref, comp-pointer-nil-method, comp-pointer-handle-denied, comp-pointer-captured-address.
- **implementation_coverage:** PENDING: W13 stage 2.
- **executed_evidence:** PENDING: W13 stage 2.
- **supported_execution_configurations:** production, lab.

### SGO-COMP-008 — Nil values and data failures
- **version:** 1
- **status:** draft
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
- **positive_tests:** G07; PENDING: fixtures comp-nil-slice-empty, comp-nil-map-read.
- **negative_tests:** G07; PENDING: fixtures comp-slice-bounds, comp-pointer-nil-deref, comp-map-nil-write, comp-slice-make-negative, comp-nil-chan-denied.
- **implementation_coverage:** PENDING: W13 stage 2.
- **executed_evidence:** PENDING: W13 stage 2.
- **supported_execution_configurations:** production, lab.

### SGO-TYPE-007 — The byte type
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-TYPE-001, SGO-TYPE-002, SGO-VAL-001, SGO-COMP-005; D93.
- **grammar_and_types:** `byte` is admitted: an integer type with values 0..255. Byte operands combine with byte operands under the operators of SGO-VAL-001 (shifts with an int count); untyped constants representable in 0..255 are assignable to byte. `int(b)` converts a byte to int; `byte(i)` converts an int to byte. Byte constants are written as integer literals (rune literals stay denied, SGO-LEX-004). `uint8` and the other sized integer types stay denied (SGO-TYPE-001).
- **preconditions:** For `byte(i)`: `0 ≤ i ≤ 255`.
- **evaluation_and_value_semantics:** Byte arithmetic is exact: the mathematical result of `+`, `-`, `*`, `/`, `%`, `<<` on bytes is the result if it lies in 0..255. `&`, `|`, `^`, `&^`, and `>>` cannot leave the range.
- **ownership_and_lifecycle:** n/a — a value type.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** n/a — a value type.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** A result outside 0..255, and `byte(i)` with i outside 0..255, is `IntegerOverflow` (SGO-VAL-001); division by zero is `DivideByZero`; a shift count outside 0..63 is `ShiftRange`.
- **cancellation_and_cleanup:** n/a — byte operations are not cancellation points.
- **compile_link_checks:** `SGO_E_CONST_OVERFLOW` for constants outside 0..255; `SGO_E_TYPE_MISMATCH` for mixing byte and int without a conversion.
- **runtime_guards:** `IntegerOverflow`, `DivideByZero`, `ShiftRange`.
- **tcb_assumptions:** The checked-arithmetic code generation of SGO-VAL-001, extended to 8-bit unsigned operands.
- **positive_tests:** G12; PENDING: fixtures type-byte-arith, type-byte-convert.
- **negative_tests:** G12; PENDING: fixtures type-byte-overflow, type-byte-convert-overflow, type-byte-const-overflow.
- **implementation_coverage:** PENDING: W13 stage 2.
- **executed_evidence:** PENDING: W13 stage 2.
- **supported_execution_configurations:** production, lab.

## New acceptance cases

G10 (slices, `[]byte`, string indexing and slicing), G11 (pointers and pointer receivers), G12 (the
byte type) join G04–G07 in `SGO_TEST_MATRIX.json` when the drafts are approved.

## What changes in existing clauses at 0.9.0

SGO-COMP-001 v4 (denies only what stays denied: `range` over strings, `[]rune`, map keys that are
not value types, pointer conversions, slices/maps/pointers of handle types, anonymous structs,
embedding, tags); SGO-UTYPE-001 v3 (pointer receivers admitted by SGO-COMP-007); SGO-UTYPE-002 v2
(methods of `*T`); SGO-TYPE-001 v2 (`byte`); SGO-TYPE-002 v3 (nil of pointer, slice, map; slice and
map identity); SGO-DEF-003 v4 (`NilDereference`, `NilMapWrite`); SGO-CHAN-002 v3 and SGO-PERMIT-002
v3 (task-local and shareable element types, sends through `sgo.Copy`); SGO-MEM-002 v4 (`&x` of a
captured variable); SGO-COMP-002 v2 (channel fields and elements as shareable, D95).
