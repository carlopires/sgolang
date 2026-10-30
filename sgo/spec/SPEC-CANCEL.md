# SPEC-CANCEL — Sticky cancellation, owner/ancestor propagation, progress boundaries

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §10, resolution R1, plan §6.1, invariants CAN-1..CAN-4.

### SGO-CANCEL-001 — Cancellation request and propagation
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-NUR-002, SGO-NUR-005, SGO-SPAWN-002, SGO-SPAWN-004, SGO-DEF-005, SGO-DEF-003; R1; CAN-1.
- **grammar_and_types:** `n.Cancel()` where `n` has type `*sgo.Nursery`; no result; a call statement.
- **preconditions:** The caller may call `Cancel` on `n` only if it could spawn into `n` (SGO-SPAWN-002 conditions (b) and (c)); a call in state CANCELLING, FINALIZING, or CLOSED by such a caller has no effect.
- **evaluation_and_value_semantics:** If `n`'s scope is OPEN or JOINING, it moves to CANCELLING and records the cause `Cancel` with the caller's creation ID. The same transition, with cause `ChildFailed(id)`, happens when a child of the scope ends `Errored` or `Failed` (SGO-SPAWN-004), and with cause `Unwind` when the owner's body unwinds through the nursery (SGO-DEF-005). Only the first cause is recorded; later requests have no effect (sticky and idempotent, CAN-1). A scope in CANCELLING *cancels* every task linked to it: its owner while executing inside it, every task registered in it, and, transitively, every scope those tasks are executing inside (each of which also moves to CANCELLING with cause `Ancestor`).
- **ownership_and_lifecycle:** Cancellation does not stop tasks; it makes their cancellation points report cancellation (SGO-CANCEL-002).
- **effect_and_capability_requirements:** Requires the nursery handle (SGO-CAP-001).
- **concurrency_and_memory_order:** The transition is linearizable; it happens-before every cancellation observation it causes (SGO-MEM-004).
- **allowed_nondeterminism:** Order relative to concurrent admissions and completions; each ordering is specified (SGO-SPAWN-002, SGO-SPAWN-004).
- **failure_and_partial_progress:** A call by a task that may not spawn into `n` fails with `SpawnContractViolation` (SGO-DEF-003) and has no effect.
- **cancellation_and_cleanup:** Cancellation is never undone. A copied handle or context cannot reset it.
- **compile_link_checks:** `SGO_E_TYPE_MISMATCH` for wrong receiver; nothing else statically.
- **runtime_guards:** Membership check as in SGO-SPAWN-002.
- **tcb_assumptions:** Cancellation propagation through the runtime scope tree (SGO-TCB-001).
- **positive_tests:** C01, C02, S07; fixture:cancel-basic, fixture:cancel-idempotent, fixture:cancel-propagates-nested.
- **negative_tests:** S07; fixture:cancel-unrelated-task.
- **implementation_coverage:** none yet — W04.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-CANCEL-002 — Observation at checkpoints
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-CANCEL-001, SGO-CANCEL-003, SGO-CANCEL-004, SGO-WAIT-001; plan §6.1.
- **grammar_and_types:** `task.Checkpoint() error` where `task` has type `*sgo.Context`.
- **preconditions:** `task` is the context of the calling task (the `root` of `main` or the parameter of the running `Spawn` callback); using another task's context fails with `SpawnContractViolation`.
- **evaluation_and_value_semantics:** Returns `nil` if no scope currently linked to the calling task (SGO-CANCEL-004) is CANCELLING; otherwise returns a cancellation error (SGO-CANCEL-003) naming the cause of the outermost cancelled linked scope. Stickiness is per scope: a linked scope that is CANCELLING stays CANCELLING until it closes, so every later `Checkpoint` and `Sleep` reports it again while it is linked. When a cancelled inner nursery closes, it is no longer linked, and its cancellation stops affecting the owner (plan §6.1: local cancellation is consumed at its matching boundary); cancellation of the task's own spawn scope or of an enclosing nursery is never consumed this way.
- **ownership_and_lifecycle:** n/a — query.
- **effect_and_capability_requirements:** n/a — no external effect.
- **concurrency_and_memory_order:** Observes every cancellation that happened-before it.
- **allowed_nondeterminism:** A cancellation concurrent with the call may or may not be observed by that call; if not, it is observed by the next one.
- **failure_and_partial_progress:** n/a — cannot fail except by the context-ownership check.
- **cancellation_and_cleanup:** `Checkpoint` and `Sleep` are the only cancellation points in subset 0. Code that never reaches one is never interrupted (cooperative model); a non-cooperative child is handled by SGO-DRAIN-001.
- **compile_link_checks:** `SGO_E_DISCARDED_ERROR` if the result is not used.
- **runtime_guards:** Context-ownership check.
- **tcb_assumptions:** Per-task cancellation state readable without locks on the fast path (SGO-TCB-001).
- **positive_tests:** C01, C03; fixture:cancel-checkpoint-loop.
- **negative_tests:** C03; fixture:cancel-foreign-context.
- **implementation_coverage:** none yet — W04.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-CANCEL-003 — Cancellation errors
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-TYPE-006, SGO-SPAWN-004, SGO-CANCEL-001.
- **grammar_and_types:** `sgo.IsCancelled(err error) bool`.
- **preconditions:** n/a — total function.
- **evaluation_and_value_semantics:** `IsCancelled(e)` is `true` exactly when `e` is a cancellation error produced by `Checkpoint` or `Sleep`; `false` for `nil` and for errors from `sgo.NewError` or writes. The error's message is `cancelled: <cause>`, where cause is `Cancel by task <id>`, `child <id> failed`, `unwind`, or `ancestor scope <id> cancelled`.
- **ownership_and_lifecycle:** Immutable error value.
- **effect_and_capability_requirements:** n/a — pure.
- **concurrency_and_memory_order:** Immutable, sendable.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** n/a — cannot fail.
- **cancellation_and_cleanup:** Returning a cancellation error from a `Spawn` callback makes the child `Cancelled`, not `Errored` (SGO-SPAWN-004).
- **compile_link_checks:** `SGO_E_DISCARDED_ERROR` does not apply (result is `bool`).
- **runtime_guards:** n/a — total function.
- **tcb_assumptions:** n/a — ordinary runtime values.
- **positive_tests:** C04; fixture:spawn-cancelled-outcome.
- **negative_tests:** C04; fixture:cancel-newerror-not-cancelled.
- **implementation_coverage:** none yet — W04.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-CANCEL-004 — Owner and ancestry
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-CANCEL-001, SGO-NUR-005; plan §6.1.
- **grammar_and_types:** n/a — dynamic rule.
- **preconditions:** n/a — dynamic rule.
- **evaluation_and_value_semantics:** The scopes *linked* to a task are: the scope it was spawned into (none for the root task) and every nursery it is currently executing inside. The task's context reports cancellation if any linked scope is CANCELLING. When a task leaves a nursery (the nursery closes), that nursery stops being linked, and its cancellation no longer affects the task (SGO-CANCEL-002). Cancelling an inner scope never cancels an outer scope or the owner's own spawn scope. Example: `main` cancels an inner nursery `n`, its children return cancellation errors, `n` closes normally (all outcomes `Cancelled`), and `main` then continues with no pending cancellation.
- **ownership_and_lifecycle:** n/a — linkage rule.
- **effect_and_capability_requirements:** n/a — linkage rule.
- **concurrency_and_memory_order:** As SGO-CANCEL-001.
- **allowed_nondeterminism:** None beyond SGO-CANCEL-002.
- **failure_and_partial_progress:** n/a — linkage rule.
- **cancellation_and_cleanup:** The owner of a nursery sees a child failure at its next cancellation point inside the nursery's block, so a blocked owner (in `Sleep`) is woken (plan §6.1).
- **compile_link_checks:** n/a — dynamic rule.
- **runtime_guards:** Linkage tracking in the runtime.
- **tcb_assumptions:** Per-task scope stack (SGO-TCB-001).
- **positive_tests:** C05, S07; fixture:cancel-owner-woken-by-child-failure, fixture:nur-inner-cancel-not-outer, fixture:cancel-consumed-at-boundary.
- **negative_tests:** n/a — behavioral; checked by positive fixtures and model histories.
- **implementation_coverage:** none yet — W04.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
