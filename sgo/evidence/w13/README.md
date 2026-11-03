# W13 evidence — core data model

Plan: `sgo/docs/w13-data-model-plan.md`. Decisions D85–D92, approved 2026-11-02.
State: **in progress** — step 13.0 done; stage 1 (structs, arrays, named types) next.
Nothing of W13 is admitted: spec 0.7.0 still rejects every composite type with `SGO_E_UNADMITTED`.

## 13.0 — Sharing-rule model (G01: passed)

| Item | Where |
|---|---|
| Proposed compile-time rule (`CheckShare`) | `sgo/model/sharemodel.go` |
| Dynamic semantics: objects with owners, copy, move, freeze, spawn captures | `sgo/model/sharestep.go` |
| Exhaustive interleaving explorer; program generator | `sgo/model/shareexplore.go` |
| Scenarios, soundness, mutations | `sgo/model/sharemodel_test.go` |

**Property checked.** Every access to a mutable object is made by the task that currently owns it,
and ownership changes only when a moved or copied value is received; frozen objects are never
written. Exclusive ownership with channel hand-off implies no data race (SGO-MEM-003).

**Scope.** Bounded: programs of up to three task bodies, a few variables, one child reference per
object, three channels. The generator is type-directed, so most rejections come from the sharing
rule. This is model evidence for the rule's design, not a proof about the future compiler; the
compiler's checks are tested against the same cases when stage 2 lands.

**Results** (darwin-arm64, `sgo/scripts/test-model.sh`):
- 7 accepted scenarios covering the five programs' crossings (copy, move, value jobs, local
  aliasing, frozen table, frozen over a channel) explore without a violation; 7 rejected scenarios
  get the expected diagnostics (`SGO_E_UNCOPIED_SEND`, `SGO_E_TASK_LOCAL_CAPTURE`,
  `SGO_E_USE_AFTER_MOVE`, `SGO_E_MOVE_ALIASED` ×2, `SGO_E_FROZEN_WRITE` ×2).
- Soundness: 20,000 generated programs (seed 13), 17,362 accepted, every one explored completely
  (none reached the state bound) without a violation.
- Mutations, each detected: plain send of a Ref value, Ref capture by `Spawn`, move of an aliased
  variable, write through a frozen value (scenarios), and a shallow `sgo.Copy` (scenario `p1-copy`).

## Acceptance programs (G09: pending)

`sgo/testdata/w13-programs/` holds the five programs with their expected output. They become
conformance cases in the stage that admits their last construct.

## Stage 1 drafts (awaiting approval)

`sgo/docs/w13-stage1-drafts.md`: SGO-COMP-002 (value structs and arrays), SGO-COMP-003 (array
indexing, `BoundsViolation`), SGO-UTYPE-002 (named types, value-receiver methods). Kept outside
`sgo/spec/` so the 0.7.0 release registry and its admission digest do not move before approval.

## Stage 1 — value structs, arrays, named types (spec 0.8.0)

Drafts approved 2026-11-02 and frozen as SGO-COMP-002, SGO-COMP-003, SGO-UTYPE-002, with
SGO-COMP-001 v3, SGO-UTYPE-001 v2, SGO-TYPE-002 v2, SGO-DEF-003 v3, SGO-CHAN-002 v2,
SGO-PERMIT-002 v2, SGO-MEM-002 v3 (`sgo/spec/CHANGELOG.md` 0.8.0). One clarification against the
approved draft: the denial of method values applies to user-declared types; method values of
package sgo's handles were already admitted (fixture `auth-function-value`) and stay so.

| Piece | Where |
|---|---|
| Admission of the syntax | `sgoadmit/admit.go` (`typeDecl`, `structType`, `recvOK`), `admit_expr.go` |
| Value-type, index, literal, conversion, method-value rules | `types2/sgo_values.go`, wired in `sgo_flow.go` |
| Field/element assignment assigns the variable (MEM-002 v3) | `sgoAssignedRoot` |
| `BoundsViolation` | `ssagen/ssa.go` `boundsCheck` → `ssagen/sgo.go` `sgoBoundsFail` → `runtime.sgoBoundsViolation` |
| Constant index out of range | `types2/sgo.go` (`SGO_E_CONST_RANGE`) |
| Inventory | `sgo/spec/INVENTORY.map`: struct/array/index/range/type surfaces admitted under SGO-COMP-002 / SGO-UTYPE-002 |

Results (darwin-arm64): conformance 366 cases, 0 unexpected (25 new: `comp-*`, `utype-*`;
`utype-declaration` turned from a rejection into a run case); test-identity, test-tcb, test-flags,
test-check (350 cases, check and build agree), test-negative, test-fmt, test-mutations,
test-native-lowering, test-isolation pass. Matrix G02, G03, G07 (arrays part) passed. Linux: sgo-w00
on the 0.8.0 commit.
