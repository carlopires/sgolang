# SPEC-PERMIT — Reserve/commit permits

Denied in subset 0 (spec 0.1.0, `SGO_SPEC_ADMISSION.json` row `SPEC-PERMIT`: `subset0` = `denied`). Spec 0.2.0 admits typed channels with send permits (SGO-PERMIT-002), which supersedes SGO-PERMIT-001.

### SGO-PERMIT-001 — Reserve/commit permits are denied in subset 0
- **version:** 2
- **status:** superseded
- **reference_closure:** SGO-ERR-001, SGO-PERMIT-002; plan §7.2.
- **grammar_and_types:** Superseded in spec 0.2.0 by SGO-PERMIT-002, which admits send permits of `sgo.Channel[T]`. Still denied: every other reserve/commit/abort permit and permit or obligation API (for example a package-level `sgo.Reserve`). No form of these constructs, packages, or APIs is admitted in any package compiled for the target, including dependencies and unreachable code.
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
- **positive_tests:** n/a — denial clause; admission is planned for W06 (plan §7.2).
- **negative_tests:** S12; fixture:permit-api.
- **implementation_coverage:** gate level only — `sgo/scripts/sgo-target` denies every program (W00); compiler-level denial is W03.
- **executed_evidence:** W00 gate checks passed on darwin/arm64, linux/amd64, linux/arm64 (gate level; not construct-specific).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-PERMIT-002 — Typed managed channels with send permits (subset 1)
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-CAP-001, SGO-CANCEL-001, SGO-CANCEL-002, SGO-SPAWN-004, SGO-NUR-003, SGO-MEM-002; D28, D29, D30; plan §7.
- **grammar_and_types:** `sgo.Channel[T]` is the only parameterized type admitted in subset 1; `T` must be a subset-1 sendable type (int, bool, string, error, D29). `ch := sgo.NewChannel[T](capacity int) *sgo.Channel[T]`; `p, err := ch.Reserve(task *sgo.Context) (*sgo.SendPermit[T], error)`; `err := p.Commit(v T) error`; `p.Abort()`; `v, ok, err := ch.Receive(task *sgo.Context) (T, bool, error)`; `ch.Close()`. User-defined type parameters stay denied (SGO-GEN-001).
- **preconditions:** `capacity >= 0`. `Reserve`/`Receive` take the calling task's own context. A permit may be committed or aborted only by the task that reserved it.
- **evaluation_and_value_semantics:** A permit is RESERVED → (COMMITTED | FAILED_NO_EFFECT | ABORTED), one terminal state shared by every alias. For capacity C > 0, `queued + reserved <= C` at all times and receives see values in commit order (FIFO). `Reserve` waits (a cancellation point) for free capacity, or for C = 0 for a receiver ready to rendezvous. `Commit` publishes `v` exactly once and returns nil, or returns a closed error with no effect if the channel was closed first (plan §7.3). `Abort` restores the reservation; aborting a terminal permit has no effect. `Receive` waits (a cancellation point) and returns the next value with `ok = true`, or `ok = false` once the channel is closed and empty.
- **ownership_and_lifecycle:** Permits are task-affine obligations. At owner-body exit of a nursery, the body's unresolved permits are aborted before the join; a task's unresolved permits are aborted before its outcome is published, each with a diagnostic in the record's chain (plan §7.2). A channel itself belongs to no nursery.
- **effect_and_capability_requirements:** None beyond the channel and permit handles (SGO-CAP-001 comparison rules).
- **concurrency_and_memory_order:** A successful `Commit` happens-before the `Receive` that returns its value. Close is linearized against commits as in plan §7.3.
- **allowed_nondeterminism:** Which waiting sender or receiver proceeds first; each history satisfies the invariants above.
- **failure_and_partial_progress:** Double commit, commit after abort, and use by another task fail with `SpawnContractViolation` and have no second effect. `Reserve` or `Receive` on a cancelled task returns a cancellation error with no reservation. `Reserve` on a closed channel returns a closed error. Negative capacity is `InvalidArgument`.
- **cancellation_and_cleanup:** Cancellation before reserve: no permit; after reserve and before commit: the waiting operation returns cancellation and the reservation is aborted; after commit: not retroactive (plan §7.3). For capacity 0, a receive takes effect when a reservation is matched to it: a matched receiver whose task is cancelled completes when that permit resolves (the value on commit; on abort it waits again, or returns cancellation; `ok = false` if the channel was closed); its wait is bounded by the sender's task, whose unresolved permits are aborted at task end. Unmatched waiting senders and receivers return cancellation promptly.
- **compile_link_checks:** `SGO_E_UNADMITTED` for other type arguments, user generics, native `chan`/`select` (until W07); `SGO_E_DISCARDED_ERROR` for unused `Reserve`/`Commit`/`Receive` errors.
- **runtime_guards:** Capacity invariant, single terminal state, task affinity, obligation resolution at body exit and task end.
- **tcb_assumptions:** Park/unpark and cancellation query (TCB-P02, P04); the managed-channel primitive TCB-P12 (`src/runtime/sgo_chan.go`, linearization notes in `sgo/docs/runtime-design.md` §10).
- **positive_tests:** H01, H02, H03, H06, H07, H08, H10, H11, H12; fixture:permit-buffered-fifo, fixture:permit-rendezvous, fixture:permit-reserve-abort, fixture:permit-cancel-before-commit, fixture:permit-cancel-after-commit, fixture:permit-close-versus-commit, fixture:permit-owner-exits, fixture:permit-error-values; the model's bounded histories (`sgo/model/chanmodel.go`, D30).
- **negative_tests:** H04, H05, H09; fixture:permit-double-commit, fixture:permit-foreign-task, fixture:permit-panic-with-permit, fixture:permit-negative-capacity, fixture:permit-elem-denied, fixture:permit-value-type-denied, fixture:permit-discarded-error, fixture:permit-compare-denied.
- **implementation_coverage:** W06: model `sgo/model/chanmodel.go`; runtime `src/runtime/sgo_chan.go`; API `src/sgo/chan.go`; admission `sgoadmit/admit_expr.go` (`sgoInstance`), handle rules `types2/sgo.go` (`isSgoHandle`).
- **executed_evidence:** `sgo/evidence/w06/README.md` (model lane, runtime lane against the model's outcome sets with fault injection, typed-channel conformance fixtures).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
