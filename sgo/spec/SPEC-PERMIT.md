# SPEC-PERMIT — Reserve/commit permits

Wholly denied in subset 0 (`SGO_SPEC_ADMISSION.json` row `SPEC-PERMIT`: `subset0` = `denied`).

### SGO-PERMIT-001 — Reserve/commit permits are denied in subset 0
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-ERR-001; plan §7.2.
- **grammar_and_types:** Denied: reserve/commit/abort permits and any permit or obligation API. No form of these constructs, packages, or APIs is admitted in any package compiled for the target, including dependencies and unreachable code.
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
- **status:** draft
- **reference_closure:** SGO-CAP-001, SGO-CANCEL-001, SGO-CANCEL-002, SGO-SPAWN-004, SGO-NUR-003, SGO-MEM-002; D28, D29, D30; plan §7.
- **grammar_and_types:** `sgo.Channel[T]` is the only parameterized type admitted in subset 1; `T` must be a subset-1 sendable type (int, bool, string, error, D29). `ch := sgo.NewChannel[T](capacity int) *sgo.Channel[T]`; `p, err := ch.Reserve(task *sgo.Context) (*sgo.SendPermit[T], error)`; `err := p.Commit(v T) error`; `p.Abort()`; `v, ok, err := ch.Receive(task *sgo.Context) (T, bool, error)`; `ch.Close()`. User-defined type parameters stay denied (SGO-GEN-001).
- **preconditions:** `capacity >= 0`. `Reserve`/`Receive` take the calling task's own context. A permit may be committed or aborted only by the task that reserved it.
- **evaluation_and_value_semantics:** A permit is RESERVED → (COMMITTED | FAILED_NO_EFFECT | ABORTED), one terminal state shared by every alias. For capacity C > 0, `queued + reserved <= C` at all times and receives see values in commit order (FIFO). `Reserve` waits (a cancellation point) for free capacity, or for C = 0 for a receiver ready to rendezvous. `Commit` publishes `v` exactly once and returns nil, or returns a closed error with no effect if the channel was closed first (plan §7.3). `Abort` restores the reservation; aborting a terminal permit has no effect. `Receive` waits (a cancellation point) and returns the next value with `ok = true`, or `ok = false` once the channel is closed and empty.
- **ownership_and_lifecycle:** Permits are task-affine obligations. At owner-body exit of a nursery, the body's unresolved permits are aborted before the join; a task's unresolved permits are aborted before its outcome is published, each with a diagnostic in the record's chain (plan §7.2). A channel itself belongs to no nursery.
- **effect_and_capability_requirements:** None beyond the channel and permit handles (SGO-CAP-001 comparison rules).
- **concurrency_and_memory_order:** A successful `Commit` happens-before the `Receive` that returns its value. Close is linearized against commits as in plan §7.3.
- **allowed_nondeterminism:** Which waiting sender or receiver proceeds first; each history satisfies the invariants above.
- **failure_and_partial_progress:** Double commit, commit after abort, and use by another task fail with `SpawnContractViolation` and have no second effect. `Reserve` or `Receive` on a cancelled task returns a cancellation error with no reservation. `Reserve` on a closed channel returns a closed error. Negative capacity is `InvalidArgument`.
- **cancellation_and_cleanup:** Cancellation before reserve: no permit; after reserve and before commit: the waiting operation returns cancellation and the reservation is aborted; after commit: not retroactive (plan §7.3).
- **compile_link_checks:** `SGO_E_UNADMITTED` for other type arguments, user generics, native `chan`/`select` (until W07); `SGO_E_DISCARDED_ERROR` for unused `Reserve`/`Commit`/`Receive` errors.
- **runtime_guards:** Capacity invariant, single terminal state, task affinity, obligation resolution at body exit and task end.
- **tcb_assumptions:** Park/unpark and cancellation query (TCB-P02, P04); PENDING: the channel primitive's TCB entry.
- **positive_tests:** H01, H02, H03, H06, H07, H08, H10, H11, H12; PENDING: fixtures and the model's bounded histories (D30).
- **negative_tests:** H04, H05, H09; PENDING: fixtures.
- **implementation_coverage:** none yet — W06.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
