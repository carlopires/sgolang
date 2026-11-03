# W13 stage 3 drafts — frozen values (for decisions D98–D101)

Status: **draft, awaiting owner approval.** These clauses live here, outside `sgo/spec/`, so the
frozen 0.9.0 digest does not move; on approval they freeze at spec 0.10.0. They implement D86 form
"frozen" and D90 (`sgo.Frozen[T]`, a distinct type checked at compile time) for acceptance
program P5 (`sgo/testdata/w13-programs/p5-table`, expected `sum 40\n`).

The sharing model (`sgo/model/sharemodel.go`) already explores freeze: scenarios `p5-frozen-table`
and `frozen-over-channel` are accepted, `reject-frozen-write` and `reject-frozen-child-wr` are
refused with `SGO_E_FROZEN_WRITE`, and the mutation "allow a write through a frozen value" is caught.

## The problem stage 3 has to solve

`sgo.Freeze(x)` must let any number of tasks read one structure without copying it and without
locks. `x` may hold slices, maps, and pointers. Whatever `Frozen[T]` hands back, a reader must not
be able to write through those references, nor keep one in a variable where a later write (or a
send, or a `sgo.Copy` that the reader then mutates) could reach the shared object. Go's type system
has no read-only reference, so the check has to be S-Go's own.

## Decisions needed with these drafts (D98–D101)

| ID | Question | Recommendation |
|---|---|---|
| D98 | What does `sgo.Freeze(x)` do to `x`? | Freeze makes one deep copy (the `sgo.Copy` of SGO-COMP-004) and seals it; `x` stays the caller's mutable value. One copy per freeze, none per reader. When the narrow move of stage 4 lands, `sgo.Freeze(move(x))` can skip the copy. Rejected: freezing `x` in place (needs the stage-4 "no other live reference" analysis now). |
| D99 | How are frozen contents read? | **Access paths.** `f.Get()` may appear only at the head of a selector/index chain (`f.Get().weights[k]`, `f.Get().items[i].Name`, `len(f.Get().items)`) or as the operand of `range`. The chain's result must be a value type (SGO-COMP-002), a `Frozen[U]`, or a channel; a task-local result (a slice, map, pointer, or value holding one) may only be indexed or selected further, passed to `len`, or ranged over. `range` over a frozen slice or map binds iteration variables of value type only. Every other use — assigning, passing, returning, sending, capturing, `sgo.Copy`-ing, writing (`=`, `op=`, `++`, `delete`, `append`) — of a frozen-derived reference is `SGO_E_FROZEN_WRITE`. Rejected: `Get` returning a deep copy (defeats the point); flow-sensitive taint tracking of frozen-derived variables (larger, worse diagnostics). |
| D100 | Which T may be frozen? | Any admitted type that holds no handle and no channel (handles and channels are already shareable or have their own rules). Freezing a value type is allowed and simply shares it. |
| D101 | Identity and zero value | `Frozen[T]` has no zero value of its own (it must be initialized, SGO-BIND-003), no `nil`, and no `==`. It is shareable: captured by `Spawn` callbacks and sent on channels or committed without `sgo.Copy`. |

## Clauses

### SGO-COMP-009 — Frozen values
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-COMP-002, SGO-COMP-004, SGO-COMP-005, SGO-COMP-006, SGO-COMP-007, SGO-MEM-003, SGO-CHAN-002, SGO-PERMIT-002, SGO-BIND-003; D86, D90, D98–D101.
- **grammar_and_types:** `sgo.Frozen[T]` is a type of package sgo for any admitted T that contains no handle type and no channel (D100). `sgo.Freeze(x T) sgo.Frozen[T]` and the method `f.Get() T` are its only operations. `f.Get()` is admitted only as the head of an *access path*: a chain of field selections, index expressions (`a[i]`, `m[k]`, `s[i:j]` is denied), and `len(...)` applied to it, or as the operand of `range` (D99). The value of an access path must be a value type (SGO-COMP-002), a `Frozen[U]`, or the operand of a further selection, index, `len`, or `range` within the same path. `range` over an access path binds iteration variables of value type only (the key of a map is always a value type, SGO-COMP-006). `sgo.Frozen[T]` has no `==`, no `nil`, and no zero value: a variable of it is initialized by `sgo.Freeze` or by another `Frozen[T]` (D101). It is a shareable type (SGO-COMP-004): a function literal of any task may capture it, and it is sent and committed without `sgo.Copy`. A struct field, array element, slice element, or map value of type `Frozen[U]` makes the containing type task-local (it holds a reference) unless the container is itself frozen.
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
- **positive_tests:** G08; fixture:frozen-share-table (P5), fixture:frozen-over-channel, fixture:frozen-range-map, fixture:frozen-nested, fixture:frozen-caller-mutates.
- **negative_tests:** G08; fixture:frozen-write-field, fixture:frozen-write-map, fixture:frozen-append, fixture:frozen-escape-var, fixture:frozen-escape-arg, fixture:frozen-escape-return, fixture:frozen-escape-send, fixture:frozen-range-ref, fixture:frozen-handle-denied, fixture:frozen-compare-denied.
- **implementation_coverage:** to be filled at freeze (planned: `src/sgo/frozen.go`; `types2/sgo_frozen.go` for the access-path rule; admission in `sgoadmit`).
- **executed_evidence:** to be filled at freeze.

## What changes in existing clauses at 0.10.0

SGO-COMP-004 v2 (shareable types include `Frozen[T]`); SGO-CHAN-002 v4 and SGO-PERMIT-002 v4
(`Frozen[T]` elements need no `sgo.Copy`); SGO-TYPE-001 v7 (`sgo.Frozen[T]`); SGO-GEN-001 (the
type-parameter list gains `sgo.Frozen`, as it has `sgo.Channel` and `sgo.SendPermit`).

## Acceptance

G08 ("Frozen values") passes; P5 (`w13-p5-table`) runs with `sum 40\n`. Mutations to run: the
access-path check accepts every use; `Freeze` without the copy (the caller's later write must show
through and fail `frozen-caller-mutates`).
