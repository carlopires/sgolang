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
test-native-lowering, test-isolation, test-lab, test-x10, test-test, test-examples, and the runtime
lane (`TestSGo|TestAPI`, runtime and sgo) pass. Matrix G02, G03, G07 (arrays part) passed.

Mutations of the new checks (each rebuilt into stage 0 and run against its fixtures; restored and
rebuilt after):

| Mutation | Detected by |
|---|---|
| `sgoValueType` accepts every type | comp-struct-func-field-denied, comp-array-chan-elem-denied (build succeeded) |
| `sgoAssignedRoot` sees only bare names | comp-captured-field-write (build succeeded) |
| bounds failures in `.sgo` code use Go's panic path | comp-array-bounds-exit-2, comp-array-bounds-child (RuntimeDefect, exit 4) |

CI on `28d6c0aa56` (spec 0.8.0): sgo-spec 41566921097 success; sgo-runtime 41566921103 success
(linux lane and race lane); sgo-w00 41566921092: every S-Go step passed on linux/amd64 and
linux/arm64 (conformance, check, test, examples, release archive, identity, fmt, TCB, X10, S12, lab,
negative fixtures, spec gates, benchmarks); the upstream baseline step (B03) was still running when
this was recorded.

## Stage 2 — spec 0.9.0 (`51f4424bd7`)

Drafts approved 2026-11-02 (D93–D97) and frozen at spec 0.9.0: SGO-COMP-004 (task-local types,
capture and send rules, `sgo.Copy`), SGO-COMP-005 (slices, `[]byte`, string indexing and slicing),
SGO-COMP-006 (insertion-ordered maps), SGO-COMP-007 (pointers, pointer receivers), SGO-COMP-008
(nil, `NilDereference`, `NilMapWrite`), SGO-TYPE-007 (`byte`), and 13 amended clauses (CHANGELOG
0.9.0). 79 new conformance cases; seven former denials (type-slice, type-pointer, val-string-index,
comp-array-slice, comp-array-chan-elem, utype-pointer-receiver, lex-denied-operators now on `~`)
updated.

darwin-arm64: conformance 428/428 (245 run, 146 reject, 31 lab, 6 test), runtime lane
(`TestSGo|TestAPI`, runtime and sgo), test-identity, test-tcb, spec-tool, inventory (567 surfaces),
guarantee, license, lock-alloc, adoption, generated-twin, and policy checks pass. Matrix G04, G05,
G06, G07, G10, G11, G12 passed; G08 (frozen values) is stage 3.

Mutations (each rebuilt into stage 0 with a regenerated manifest, run against its fixtures,
restored and rebuilt after):

| Mutation | Detected by |
|---|---|
| `sgoTaskLocal` reports no type task-local (capture and send rules off) | comp-spawn-capture-slice, comp-spawn-capture-map, comp-uncopied-send, comp-local-closure-arg (build succeeded) |
| `sgo_copy` returns its argument (shallow) | comp-copy-deep, comp-copy-send, comp-copy-sharing, comp-copy-cycle (wrong output) |
| `.sgo` map operations use Go's maps | comp-map-order, comp-map-delete-reinsert, comp-map-range-mutate (Go order), comp-map-nil-write (RuntimeDefect, exit 4) |

CI on `51f4424bd7`: sgo-spec 41574250765 success; sgo-runtime 41574250776 success (linux lane and
race lane); sgo-w00 41574250875 success on linux/amd64 and linux/arm64 (conformance 428 cases, check,
test, examples, release archive, identity, fmt, TCB, X10, S12, lab, negative fixtures, spec gates).

## Stage 3 — spec 0.10.0 (`bd20e81d0c`)

Draft approved 2026-11-03 (D98–D101) and frozen at spec 0.10.0: SGO-COMP-009 (`sgo.Frozen[T]`,
`sgo.Freeze`, access paths, `SGO_E_FROZEN_WRITE`); amended SGO-COMP-004 v2, SGO-GEN-001 v3,
SGO-TYPE-001 v7. Acceptance program P5 runs as `w13-p5-table` (`sum 40`): one frozen table read by
four tasks. 21 new conformance cases (6 run, 15 reject). With stage 2, all five W13 acceptance
programs pass.

darwin-arm64: conformance 454/454 (256 run, 161 reject, 31 lab, 6 test), test-identity, test-tcb,
runtime lane, spec-tool, inventory, guarantee, license, lock-alloc, adoption, generated-twin, and
policy checks pass. Matrix G08 passed.

Mutations (rebuilt into stage 0 with a regenerated manifest, run, restored and rebuilt after):

| Mutation | Detected by |
|---|---|
| the access-path rule accepts every use of `f.Get()` | frozen-write-field, frozen-write-map, frozen-escape-var, frozen-escape-arg, frozen-append, frozen-range-ref (build succeeded) |
| `sgo.Freeze` shares its argument instead of copying it | frozen-caller-mutates (wrong output), w13-p5-table (`sum 436`) |

## Review (13.4) — findings V1–V3 and fixes

Reviewer job note2, at `fb4d84ce23` on linux/amd64: conformance 454/454, runtime lane and race lane
green, and three findings:

| ID | Severity | Finding | Fix |
|---|---|---|---|
| V1 | blocker | `sgo.Copy` keyed copied objects by address alone: `&b.x` and `b` shared one allocation sized for an int (out-of-bounds heap writes) | `c09c5c56e1`: objects keyed by (address, type), slices by (array, element type, cap); interface values with user dynamic types deep-copied; SGO-COMP-004 v3 (spec 0.10.1) states that interior aliasing is not preserved |
| V2 | blocker | a task-local named type converted to `error` crossed tasks by capture, `chan error`, `Commit`, or returns, with no `sgo.Copy` | `c09c5c56e1` + `0a82e925c1`: implicit conversions of task-local values to an interface are `SGO_E_TASK_LOCAL_CAPTURE` (a pass over recorded types in `types2/sgo_local.go`; `assignments.go` stays as upstream, a go/types generator input); SGO-COMP-004 v3 rule 4 |
| V3 | minor | map lowering chosen per source position: a position-less node could fall back to Go's map | `74f4284f9d`: chosen by package mode; package sgo using a map is a compiler fatal; `test-map-lowering.py` in CI |

Tests and mutants: `comp-copy-interior-pointer` and the runtime tests `TestSGoCopyInteriorPointer`
(10 000 copies under GC percent 1) and `TestSGoCopyCycleAndSharing` catch the address-only key;
`comp-error-task-local` (`-return`, `-send`, `-arg`, `-literal`) and `comp-error-pointer-receiver`
catch the disabled conversion check; `test-map-lowering.py` catches Go maps in S-Go packages
(comp-map-basic called `mapaccess1_faststr`, `mapassign_faststr`, `mapclear`, `mapdelete_faststr`).
darwin-arm64 at `0a82e925c1`: conformance 462/462, generated-twin-check ok. Reviewer re-check on
linux/amd64: V1, V3 verified at `c09c5c56e1`/`74f4284f9d`; V2 reopened at `0a82e925c1` for
multi-value calls and `range =`, and V4 found (`range =` assigned a captured variable, SGO-MEM-002):
both fixed in `98b664e3d2` (tuple elements checked through one helper; `range` with `=` denied at
admission) and verified there, including the reviewer's tuple-branch mutant. Conformance 469/469.

Sharing model (G01) extended for V2: kind `SErr` and statement `V := error(W)` (an immutable box;
`Error` may write what a referencing box points to). `GenShareErr` (error values, a channel of
errors, reads and writes through them; `GenShare`'s stream unchanged, so 17,362 of 20,000 stays
reproducible): 17,235 of 20,000 accepted, all explored without a violation. Mutant `ref-error`
(accept `error(Ref)`) is caught by the scenario `reject-ref-error` and, independently, by the
generated programs (`TestShareErrGeneratorFindsRefError`). `range =` needs no model construct: it
is denied again.
Reviewer rerun on linux/amd64 at `6fa477acca` (pinned go1.26.10): TestShareScenarios, TestShareSoundness
(17,362 of 20,000), TestShareErrSoundness (17,235 of 20,000), TestShareErrGeneratorFindsRefError and
TestShareMutations pass with identical counts. The W13 review is closed: V1–V4 verified, nothing open.


## Stage 2 drafts (approved 2026-11-02; frozen at 0.9.0 above)


`sgo/docs/w13-stage2-drafts.md`: SGO-COMP-004 (task-local types, `sgo.Copy`, capture and send
rules), SGO-COMP-005 (slices, `[]byte`, string indexing), SGO-COMP-006 (maps with insertion order),
SGO-COMP-007 (pointers, pointer receivers), SGO-COMP-008 (nil, `NilDereference`, `NilMapWrite`),
SGO-TYPE-007 (`byte`), with decisions D93–D97. `spec-tool.py check` on the drafts placed in
`sgo/spec/` reports only the not-yet-added matrix cases G10–G12.

## Closure

W13 is done (D102: stage 4, move, deferred). Closing CI on `98b664e3d2` (review fixes V1–V4):
sgo-spec 41621313134 success; sgo-w00 41621313273: every S-Go step passed on linux/amd64 and linux/arm64 (conformance
469 cases, map lowering, check, test, examples, release archive, identity, fmt, TCB, X10, S12, lab,
negative fixtures, spec gates, benchmarks); the upstream baseline step (B03) was still running at
closure. Matrix G01–G12 passed. The next work
is the standard-library foundations plan (`sgo/docs/stdlib-foundations-plan.md`, D103–D110).
