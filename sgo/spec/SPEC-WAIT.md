# SPEC-WAIT — Wait registration, terminal winner, park/unpark and lock order

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §12, resolution R8, plan §6.2, invariant CAN-2.

### SGO-WAIT-001 — Sleep
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-CANCEL-002, SGO-CANCEL-003, SGO-DEF-003, SGO-TIMER-001; R8; CAN-2.
- **grammar_and_types:** `task.Sleep(ms int) error` where `task` has type `*sgo.Context`.
- **preconditions:** `task` is the calling task's own context (as SGO-CANCEL-002); `ms ≥ 0`.
- **evaluation_and_value_semantics:** If a linked scope is already CANCELLING, returns a cancellation error immediately. Otherwise registers one wait and parks the task until either at least `ms` milliseconds of monotonic time have elapsed (returns `nil`) or a linked scope enters CANCELLING (returns a cancellation error). `Sleep(0)` returns `nil` unless cancelled; it may yield to other tasks.
- **ownership_and_lifecycle:** The wait registration belongs to the task and is removed exactly once, by whichever of expiry or cancellation wins (CAN-2).
- **effect_and_capability_requirements:** n/a — no external effect.
- **concurrency_and_memory_order:** A cancellation that happens-before the timer expiry wins only if it is observed before the expiry is committed; exactly one result is returned.
- **allowed_nondeterminism:** When cancellation and expiry race, either result may be returned.
- **failure_and_partial_progress:** `ms < 0` fails with `InvalidArgument` before any registration. Context misuse fails with `SpawnContractViolation`.
- **cancellation_and_cleanup:** `Sleep` is a cancellation point (SGO-CANCEL-002). Cancellation wakes the task promptly (bounded by scheduling, not by `ms`).
- **compile_link_checks:** `SGO_E_DISCARDED_ERROR` if the result is unused; a negative constant argument is a compile error `SGO_E_INVALID_ARGUMENT`.
- **runtime_guards:** Argument and context-ownership checks; one-winner registration.
- **tcb_assumptions:** Runtime timers and park/unpark with the atomic park-unlock handshake (plan §6.2, SGO-TCB-001).
- **positive_tests:** C01, C09; fixture:wait-sleep, fixture:cancel-wakes-sleep.
- **negative_tests:** C09; fixture:wait-negative-sleep.
- **implementation_coverage:** none yet — W04.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-WAIT-002 — Join is a managed wait
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-NUR-003, SGO-WAIT-001.
- **grammar_and_types:** n/a — dynamic rule.
- **preconditions:** n/a — dynamic rule.
- **evaluation_and_value_semantics:** The owner waiting in JOINING or CANCELLING holds one wait registration, woken when the live-child count reaches zero. Lost wake-ups are impossible: the last child's completion and the owner's park use the same one-winner protocol as SGO-WAIT-001.
- **ownership_and_lifecycle:** The registration belongs to the owner and ends when the scope reaches FINALIZING.
- **effect_and_capability_requirements:** n/a — dynamic rule.
- **concurrency_and_memory_order:** As SGO-NUR-003.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** n/a — cannot fail.
- **cancellation_and_cleanup:** Not a cancellation point: cancellation of the scope does not end the owner's wait (SGO-NUR-003).
- **compile_link_checks:** n/a — dynamic rule.
- **runtime_guards:** n/a — runtime protocol.
- **tcb_assumptions:** Park/unpark (SGO-TCB-001).
- **positive_tests:** O02, O04; fixture:nur-join-all.
- **negative_tests:** n/a — protocol property; checked by the executable model and mutation tests.
- **implementation_coverage:** none yet — W02.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
