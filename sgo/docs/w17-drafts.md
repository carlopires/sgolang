# W17 drafts: interfaces (for approval)

Status: **draft, awaiting owner approval.** These clauses are kept here, outside `sgo/spec/`, so the
frozen 0.13.0 digest does not change. On approval they freeze at spec **0.14.0**. They implement
D107 (interfaces and the sharing rule, approved 2026-11-03) and follow D110's process. The plan
marks W17 as the riskiest package and asks for it to be **model first**: the sharing model is
extended and explored before any compiler work.

Summary: user-declared interface types with method sets, `any`, implicit implementation, dynamic
dispatch, the comma-ok type assertion, and type switches. Under D107 an ordinary interface value
is **task-local**: it crosses tasks only through `sgo.Copy`, which deep-copies its dynamic value.
`error` stays shareable. W17 leaves out the language features that make Go's interfaces
error-prone: the panicking assertion, typed nil, and comparison of interface values.

## Small decisions taken in these drafts (flagged for the owner)

| ID | Question | Draft choice |
|---|---|---|
| D127 | Which interface forms are admitted? | Named interface types (`type Reader interface { Read(p []byte) (int, error) }`) whose methods have admitted signatures; embedding another named interface type; and `any` (the empty interface). **Interface type literals** outside a declaration (`func f(x interface{ M() })`) stay denied, as anonymous structs are. Type sets and constraint syntax (`~int`, unions) stay denied, since they are generics (D108). |
| D128 | Can interface values be compared? | Only with `nil` (`x == nil`, `x != nil`). In Go, `==` on interfaces compares dynamic values, and panics at run time if the dynamic type is not comparable. S-Go makes that a compile-time refusal instead: no `==` between interface values, and interfaces are not map keys, the same rule `error` already has (SGO-TYPE-006). |
| D129 | Which type assertions are admitted? | The comma-ok form `v, ok := x.(T)` and type switches. The single-value form `x.(T)`, which panics in Go on a mismatch, is **denied**: every assertion handles the "no" case explicitly. An assertion whose `T` cannot implement the static interface is a compile error, as in Go. |
| D130 | What is a nil pointer converted to an interface? | **The nil interface**: S-Go has no "typed nil". Converting a nil `*T` to an interface type gives the nil interface value, so `x == nil` holds exactly when there is no value to call. Calling a method on a nil interface is the checked failure `NilDereference` (SGO-COMP-007), at the call. This deliberately differs from Go, where the interface would be non-nil and hold a nil pointer, and the clause says so. |
| D131 | Which methods count, and how are they called? | Go's method-set rules: a value type `T` implements with its value-receiver methods; `*T` implements with both. Dynamic dispatch calls the dynamic type's method. **Method values** and **method expressions** on interface types (`f := x.M`, `I.M`) stay denied, as they already are for named types (fixture `utype-method-value-denied`). A method is called, not stored. |
| D132 | How do interface values follow the sharing rule (D107)? | Every interface type other than `error` is task-local, whatever its dynamic value. It cannot be sent on a channel without `sgo.Copy`, captured by a spawned task, stored in a frozen value, or kept in a shareable struct. `sgo.Copy(x)` of an interface value deep-copies the dynamic value under the same rules as the dynamic type's own copy (SGO-COMP-004); `Copy(nil)` is nil. `error` stays shareable, and only shareable types convert to it (W13 V2). An interface type that embeds `error` is still task-local. |
| D133 | What does W17 add to the standard library? | Only what interfaces make directly possible and what is needed to test them: `errors.Unwrap(err)` (via an assertion to `interface{ Unwrap() error }`, written as a named interface in `std/errors`) and `sort.Sort(data sort.Interface)` with Go's three-method interface, stable like every S-Go sort (D123). `std/io` (`Reader`, `Writer`), `fmt`, and `errors.Is`/`As` get their own drafts. `Is` needs error comparison, which D128 keeps denied, and `As` needs generics or reflection. |

## Model first (before any compiler change)

The W13 share model (`sgo/model/sharemodel.go`, `shareexplore.go`) gains a fifth static kind,
**`SIface`**: an interface box whose dynamic value is a `Val`, `Ref` or `Frozen` object. It also
gains three operations:
- `SToIface` (`V := I(W)`), which boxes W's object, so the box aliases it;
- `SCall` (`V.M()`), dynamic dispatch, which may write the boxed object's referents, as `SToErr`'s
  `Error` may;
- `SAssert` (`W, ok := V.(T)`), which unboxes to an alias of the boxed object.

`CheckShare` gets the D132 rules, and `ExploreShare` checks the ownership invariant over every
interleaving, as in W13. The seeded-bug tests add these mutants, each of which the explorer must
catch with a concrete race:
- a plain send of an `SIface` holding a `Ref`;
- a spawn capture of an `SIface`;
- a shallow `sgo.Copy` of an interface;
- an assertion that hands the unboxed alias to another task.

Only after the model passes are the compiler rules written, and they implement exactly the
checked rule.

## Clauses

### SGO-IFACE-001: Interface forms that stay denied (v2; amends v1)
- **grammar_and_types:** Denied:
  - interface type literals outside a type declaration (D127);
  - type sets, `~T`, and unions (generics, D108);
  - `==` and `!=` between interface values, except comparison with `nil` (D128);
  - interface types as map keys;
  - the single-value type assertion `x.(T)` (D129);
  - method values and method expressions of interface types (D131).

  v1's denial of interface types, dynamic dispatch, comma-ok assertions and type switches is
  lifted (SGO-IFACE-002 to SGO-IFACE-004).
- **compile_link_checks:** `SGO_E_UNADMITTED`, naming the construct and the decision.
- **negative_tests:** fixture:iface-literal-denied, fixture:iface-compare-denied,
  fixture:iface-map-key-denied, fixture:iface-assert-single-denied,
  fixture:iface-method-value-denied, fixture:iface-union-denied.

### SGO-IFACE-002: Interface types and implementation (v1; new)
- **grammar_and_types:**
  - `type I interface { M(params) results; ...; J }`: methods with admitted signatures, and
    embedded named interface types.
  - `any` is the empty interface.
  - A type implements I when its method set includes every method of I (D131: the method set of
    `T` is its value-receiver methods; that of `*T` adds the pointer-receiver ones).
  - Assignment and argument passing convert implicitly to an interface type the value implements.
  - Duplicate methods with different signatures through embedding are a compile error.
- **evaluation_and_value_semantics:** An interface value is nil, or a pair of a dynamic type and a
  dynamic value. Converting a value copies it into the box, as an assignment would. Converting a
  nil pointer gives the nil interface (D130).
- **concurrency_and_memory_order:** Task-local (D132): SGO-COMP-004's rules for task-local types
  apply to every interface type except `error`.
- **compile_link_checks:**
  - `SGO_E_TYPE_MISMATCH` for a value that does not implement the target interface (naming the
    missing method);
  - `SGO_E_UNCOPIED_SEND`, `SGO_E_TASK_LOCAL_CAPTURE`, and the other SGO-COMP-004 codes for
    interface values.
- **positive_tests:** fixture:iface-basic, fixture:iface-embed, fixture:iface-any,
  fixture:iface-pointer-receiver, fixture:iface-copy-send.
- **negative_tests:** fixture:iface-not-implemented, fixture:iface-value-receiver-only,
  fixture:iface-send-denied, fixture:iface-capture-denied, fixture:iface-duplicate-method.

### SGO-IFACE-003: Dynamic dispatch and nil (v1; new)
- **evaluation_and_value_semantics:**
  - `x.M(args)` calls the dynamic type's method M with the dynamic value as receiver (the value
    for a value receiver, the pointer for a pointer receiver).
  - Arguments are evaluated as for any call (SGO-VAL-005).
- **failure_and_partial_progress:** A method call on a nil interface value is `NilDereference` at
  the call (SGO-COMP-007, D130).
- **positive_tests:** fixture:iface-dispatch, fixture:iface-nil-pointer-is-nil.
- **negative_tests:** fixture:iface-nil-call (run, exit 2, NilDereference).

### SGO-IFACE-004: Type assertions and type switches (v1; new)
- **grammar_and_types:**
  - `v, ok := x.(T)` and `v, ok = x.(T)` for an interface-typed x, where T is a type that
    implements x's interface or is an interface type.
  - `switch v := x.(type) { case T1, T2: ...; case nil: ...; default: ... }`.
  - An impossible T (a concrete type that cannot implement x's interface) is a compile error.
  - Duplicate cases are a compile error, as in a value switch.
- **evaluation_and_value_semantics:**
  - **Comma-ok:** ok is true and v is the dynamic value (or the interface value re-boxed, for an
    interface T) when x is non-nil and its dynamic type is, or implements, T. Otherwise ok is
    false and v is T's zero value.
  - **Type switch:** chooses the first case whose type matches, `nil` for a nil x, or `default`.
    In a single-type case, v has that type; in other cases, v has x's type.
  - **Aliasing:** the unboxed v aliases the boxed object, so it stays in the owning task (D132).
- **compile_link_checks:** `SGO_E_UNADMITTED` for the single-value form (D129);
  `SGO_E_TYPE_MISMATCH` for an impossible T; `SGO_E_DUPLICATE_CASE`.
- **positive_tests:** fixture:iface-assert-ok, fixture:iface-type-switch,
  fixture:iface-type-switch-nil.
- **negative_tests:** fixture:iface-assert-impossible, fixture:iface-switch-duplicate.

### Amendments
- **SGO-TYPE-001 v9:** named interface types and `any` join the admitted types.
- **SGO-COMP-004 v4:** interface types other than `error` are task-local. `sgo.Copy` of an
  interface value deep-copies its dynamic value. A struct with an interface field is task-local.
- **SGO-TYPE-006 v2:** `error` is now one interface type among others. Its shareability is the
  exception D132 keeps, and user types still implement it only if they are shareable.
- **SGO-CTL-001 v2:** type switches are admitted (SGO-IFACE-004). The stale "`for … range` is
  denied" sentence is replaced by references to the clauses that admit each range form.
- **SGO-STD-001 v2 and SGO-STD-006 v2:** `errors.Unwrap` and `sort.Sort`/`sort.Interface` (D133).

## Implementation plan (after approval)

1. **Model:** `SIface`, `SToIface`, `SCall`, `SAssert` in `sgo/model/share*.go`; the D132 checker
   rules; exhaustive exploration of the reference scenarios; the four mutants caught. This step is
   committed on its own before any compiler change.
2. **Admission and types** (`sgoadmit`, `types2/sgo*.go` only, not the generated twins):
   - admit interface declarations, `any`, conversions, comma-ok assertions and type switches;
   - deny the D127–D131 forms;
   - extend the task-local classification (`sgo_local.go`) to interface types.
3. **Lowering:**
   - dispatch, assertions and switches reuse Go's itab code;
   - nil pointer to nil interface (D130) is one comparison at the conversion;
   - `sgo.Copy` gains the interface case in the runtime's deep copy (`sgo_copy.go`, a TCB change
     with a regenerated manifest).
4. **Library:** `errors.Unwrap`, `sort.Sort` and `sort.Interface`, with `std-api-check`, `sgo test`
   and test-std-diff coverage.
5. **Conformance:** the fixtures named above (about 30 new). The fixtures that currently deny
   interfaces (`type-interface`) flip.
6. **Mutation tests:**
   - the single-value assertion admitted;
   - a typed nil kept (D130 removed);
   - an interface send without `Copy` admitted;
   - a shallow interface `Copy`.
7. **Freeze at 0.14.0:** registry, identity, inventory (the itab and assertion runtime routines
   become internal surfaces), policy, `sgo.mod` sweep, CHANGELOG, Linux CI evidence in
   `sgo/evidence/w17/README.md`, and reviewer verification.

## Not in W17

- Generics and constraint interfaces (D108).
- `std/io`, `fmt`, `errors.Is`/`As` (separate drafts).
- Reflection (stays denied).
- Embedding interfaces in structs (struct embedding stays denied).
- Sending interface values without a copy (D132).
