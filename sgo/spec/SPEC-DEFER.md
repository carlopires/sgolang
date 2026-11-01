# SPEC-DEFER — Function defers, scoped finalizers, panic/recover and task exit

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §7, decisions D12, D13, resolution R4, and plan §4.5.

### SGO-DEF-001 — defer
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-CTL-004, SGO-DEF-004, SGO-VAL-005; D12; R4.
- **grammar_and_types:** `defer f(a1, …, an)` where `f` is a named function, a function-typed variable, or a function literal, and the call returns no result of type `error`. `defer` of a call returning an `error` is denied (`defer func() { _ = f() }()` discards explicitly). `defer` of `panic(…)` is denied. `n.Defer` scope finalizers are denied in subset 0.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** When the `defer` statement executes, the function value and arguments are evaluated (SGO-VAL-005) and the call is pushed on the function activation's defer list. When the function returns normally (after its return operands are evaluated and crossed nurseries complete, SGO-CTL-004) or unwinds (SGO-DEF-005), deferred calls run in last-in, first-out order. Results of deferred calls are discarded. When a frame that has active nursery statements unwinds, those nurseries complete (barriers) before that frame's deferred calls run.
- **ownership_and_lifecycle:** Deferred calls belong to the function activation that executed the `defer`, not to a nursery or block.
- **effect_and_capability_requirements:** A deferred call performs its body's effects; it may use capability handles it captured or received as arguments.
- **concurrency_and_memory_order:** Deferred calls run on the same task, after all children of nurseries in that frame have completed.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** A failure inside a deferred call follows SGO-DEF-004.
- **cancellation_and_cleanup:** Deferred calls run even when the task's scopes are cancelled; `Checkpoint`/`Sleep` inside them report cancellation as usual (subset 0 has no cancellation masking).
- **compile_link_checks:** `SGO_E_DISCARDED_ERROR`, `SGO_E_UNADMITTED` (`n.Defer`, `defer panic`).
- **runtime_guards:** n/a — failures come from the deferred calls themselves.
- **tcb_assumptions:** Upstream defer machinery (open-coded and runtime defers) runs each deferred call exactly once (SGO-TCB-001).
- **positive_tests:** X09; fixture:def-lifo, fixture:def-after-nursery.
- **negative_tests:** X09; fixture:def-error-result, fixture:def-scope-finalizer.
- **implementation_coverage:** none yet — W03.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-DEF-002 — panic
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-DEF-003, SGO-DEF-005; D13.
- **grammar_and_types:** `panic(msg)` where `msg` has type `string`. It is a statement (it has no value). `recover` is denied in subset 0.
- **preconditions:** n/a — always permitted.
- **evaluation_and_value_semantics:** Evaluates `msg`, then starts unwinding the current task with failure kind `Panic` and payload `msg` (SGO-DEF-005).
- **ownership_and_lifecycle:** n/a — control transfer.
- **effect_and_capability_requirements:** n/a — no external effect until the failure reaches the process boundary (SGO-PROC-002).
- **concurrency_and_memory_order:** Affects only the current task directly; other tasks are affected through nursery outcomes (SGO-NUR-004).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Effects performed before the panic remain.
- **cancellation_and_cleanup:** Unwinding runs nursery barriers and deferred calls (SGO-DEF-005).
- **compile_link_checks:** `SGO_E_TYPE_MISMATCH` for a non-string argument; `SGO_E_UNADMITTED` for `recover`.
- **runtime_guards:** n/a — panic is the failure.
- **tcb_assumptions:** Upstream panic machinery, restricted to the behavior described here (SGO-TCB-001).
- **positive_tests:** S08; fixture:def-panic-exit-2.
- **negative_tests:** S08; fixture:def-recover, fixture:def-panic-int.
- **implementation_coverage:** none yet — W03.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-DEF-003 — Checked failure kinds
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-VAL-001, SGO-SPAWN-002, SGO-WAIT-001, SGO-PROC-003, SGO-TYPE-006, SGO-NUR-004, SGO-DEF-005.
- **grammar_and_types:** n/a — dynamic rule.
- **preconditions:** n/a — dynamic rule.
- **evaluation_and_value_semantics:** The complete set of failure kinds that unwind a task in subset 0 is: `Panic` (SGO-DEF-002), `IntegerOverflow`, `DivideByZero`, `ShiftRange` (SGO-VAL-001), `SpawnContractViolation` (SGO-SPAWN-002, SGO-CANCEL-001), `InvalidArgument` (SGO-WAIT-001), `MissingCapability` (SGO-PROC-003), `NilError` (SGO-TYPE-006), `ChildError` (SGO-NUR-004); spec 0.3.0 adds `ClosedChannel` (a send on a closed channel, SGO-CHAN-002, SGO-CHAN-004). Each carries a message and the source position where it arose. All unwind exactly like `panic` (SGO-DEF-005).
- **ownership_and_lifecycle:** n/a — dynamic rule.
- **effect_and_capability_requirements:** n/a — dynamic rule.
- **concurrency_and_memory_order:** n/a — per-task rule.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** The failing operation has no effect; earlier effects remain.
- **cancellation_and_cleanup:** As SGO-DEF-005.
- **compile_link_checks:** n/a — run-time kinds; constant cases are compile errors per their clauses.
- **runtime_guards:** Each kind is raised only by the clause that defines it.
- **tcb_assumptions:** An admitted program cannot reach an upstream-only failure (e.g. nil map write, index out of range), because the constructs that cause them are denied. If the runtime nevertheless raises one, that is a TCB defect: the process terminates as fail-stop with exit status 4 and kind `RuntimeDefect` (SGO-ERR-003); it is never reported as an ordinary panic.
- **positive_tests:** S08; fixture:val-overflow-add, fixture:wait-negative-sleep, fixture:proc-child-stdout, fixture:native-send-closed.
- **negative_tests:** n/a — a list of kinds; each kind has negative fixtures in its defining clause.
- **implementation_coverage:** none yet — W03.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-DEF-004 — Failure during unwinding
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-DEF-001, SGO-DEF-003, SGO-ERR-002.
- **grammar_and_types:** n/a — dynamic rule.
- **preconditions:** n/a — dynamic rule.
- **evaluation_and_value_semantics:** If a deferred call fails while its task is already unwinding, the failure already in progress stays primary. The new failure is appended to the primary's chain (in occurrence order) and the remaining deferred calls still run. If a deferred call fails during a normal return, that failure becomes primary and unwinding starts from there.
- **ownership_and_lifecycle:** n/a — dynamic rule.
- **effect_and_capability_requirements:** n/a — dynamic rule.
- **concurrency_and_memory_order:** n/a — per-task rule.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Every failure is reported in the diagnostic chain (SGO-ERR-002); none is dropped.
- **cancellation_and_cleanup:** All deferred calls run exactly once.
- **compile_link_checks:** n/a — dynamic rule.
- **runtime_guards:** Chain recording in the runtime.
- **tcb_assumptions:** Upstream panic-in-defer handling is restricted to this rule (SGO-TCB-001).
- **positive_tests:** S08; fixture:def-panic-in-defer.
- **negative_tests:** n/a — behavioral rule checked by positive fixtures of the chain contents.
- **implementation_coverage:** none yet — W03.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-DEF-005 — Unwinding to the task boundary
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-DEF-001, SGO-NUR-004, SGO-SPAWN-004, SGO-PROC-002.
- **grammar_and_types:** n/a — dynamic rule.
- **preconditions:** n/a — dynamic rule.
- **evaluation_and_value_semantics:** Unwinding leaves frames from the innermost outward. In each frame, first the frame's active nursery statements complete from the innermost outward (cancelled by the unwind, SGO-NUR-004 rule 1), then the frame's deferred calls run (SGO-DEF-001). When unwinding leaves a `Spawn` callback, the child task ends with outcome `Failed` (SGO-SPAWN-004). When it leaves `main`, the process ends with exit status 2, or 1 for `ChildError` (SGO-PROC-002).
- **ownership_and_lifecycle:** No task outlives its nursery during unwinding (OWN-3).
- **effect_and_capability_requirements:** n/a — dynamic rule.
- **concurrency_and_memory_order:** Children of unwinding nurseries complete before the frame's deferred calls run.
- **allowed_nondeterminism:** None in which failure is primary (creation-ID order, SGO-NUR-004).
- **failure_and_partial_progress:** Effects already performed remain; no rollback.
- **cancellation_and_cleanup:** An unwind through a nursery cancels it (CANCELLING) before joining.
- **compile_link_checks:** n/a — dynamic rule; the lowering obligation is in SGO-CTL-004.
- **runtime_guards:** Runtime classification of child outcomes.
- **tcb_assumptions:** Upstream stack unwinding and Goexit-free task completion (SGO-TCB-001).
- **positive_tests:** X12, S08; fixture:def-child-panic-cancels-siblings, fixture:def-panic-exit-2.
- **negative_tests:** n/a — behavioral rule checked by positive fixtures.
- **implementation_coverage:** none yet — W03.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
