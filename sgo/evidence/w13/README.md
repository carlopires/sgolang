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
