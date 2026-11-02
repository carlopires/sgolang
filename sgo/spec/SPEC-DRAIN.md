# SPEC-DRAIN — Cleanup budget, non-cooperation, fail-stop and diagnostic behavior

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §11, resolution R7, plan §6.3–§6.4, invariant CAN-5.

### SGO-DRAIN-001 — No false success
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-NUR-003, SGO-DRAIN-002, SGO-CANCEL-002; CAN-5.
- **grammar_and_types:** n/a — dynamic rule.
- **preconditions:** n/a — dynamic rule.
- **evaluation_and_value_semantics:** A nursery never completes while a registered child is live. A child that never reaches a cancellation point keeps its scope in CANCELLING (or JOINING) indefinitely, unless the process is terminated by SGO-DRAIN-002 or from outside.
- **ownership_and_lifecycle:** No task is ever removed from its scope while live, and the program never continues after abandoning a live task (plan §6.4).
- **effect_and_capability_requirements:** n/a — dynamic rule.
- **concurrency_and_memory_order:** n/a — dynamic rule.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Non-cooperation results in waiting or in fail-stop, never in a successful nursery completion.
- **cancellation_and_cleanup:** This is the guarantee cancellation gives in subset 0: correct outcomes at cancellation points, no orphaned tasks, no false success.
- **compile_link_checks:** n/a — dynamic rule.
- **runtime_guards:** Join requires a zero live count (SGO-NUR-003).
- **tcb_assumptions:** The runtime never kills an individual task (SGO-TCB-001).
- **positive_tests:** C06; fixture:drain-cooperative-child.
- **negative_tests:** C07; fixture:drain-noncooperative-budget.
- **implementation_coverage:** W04: `src/runtime/sgo_nursery.go`, `src/runtime/sgo_main.go`, `src/cmd/sgo/test.go`, `src/cmd/sgo/lab.go`.
- **executed_evidence:** conformance fixtures drain-cooperative-child, drain-noncooperative-budget (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w04/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-DRAIN-002 — Drain budget
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-DRAIN-001, SGO-PROC-002, SGO-ERR-002, SGO-TOOL-001; R7.
- **grammar_and_types:** Build-time setting `sgo build -drain-budget=<ms>`, `<ms>` a positive decimal integer of milliseconds. Absent means no budget. It is recorded in the binary's build metadata.
- **preconditions:** n/a — build setting.
- **evaluation_and_value_semantics:** With a budget `B`, each scope that enters CANCELLING starts a monotonic-clock timer of `B` ms. If the scope has not reached FINALIZING when the timer expires, the runtime writes a diagnostic record of kind `DrainBudgetExceeded` naming the scope, its live child creation IDs, and the cancellation cause, and terminates the process with exit status 3. No user code runs after the decision.
- **ownership_and_lifecycle:** The timer belongs to the scope and is removed when the scope reaches FINALIZING.
- **effect_and_capability_requirements:** Termination is a runtime effect, not a user capability.
- **concurrency_and_memory_order:** The expiry check and the transition to FINALIZING are serialized: a scope that reaches FINALIZING first is not failed.
- **allowed_nondeterminism:** Whether a child that finishes at nearly the same instant as expiry is counted; both outcomes are as specified (normal completion or exit 3).
- **failure_and_partial_progress:** Effects already performed remain. Stdout is unbuffered (SGO-PROC-003), so every `WriteString` that returned has reached the OS; a write in progress at the decision is either completed or not started, never torn.
- **cancellation_and_cleanup:** Deferred calls of live tasks do not run (fail-stop).
- **compile_link_checks:** `sgo build` rejects a malformed or non-positive budget. No environment variable or run-time flag changes the budget.
- **runtime_guards:** The per-scope timer.
- **tcb_assumptions:** Monotonic clock and timer primitives on all targets (SGO-TCB-001, D22).
- **positive_tests:** C07; fixture:drain-noncooperative-budget.
- **negative_tests:** S01; fixture:drain-budget-env-ignored.
- **implementation_coverage:** W03, W04: `src/runtime/sgo_nursery.go`, `src/runtime/sgo_main.go`, `src/cmd/sgo/test.go`, `src/cmd/sgo/lab.go`.
- **executed_evidence:** conformance fixtures drain-budget-env-ignored, drain-noncooperative-budget (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`; `sgo/evidence/w04/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-DRAIN-003 — Host watchdog
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-TEST-001, SGO-DRAIN-002.
- **grammar_and_types:** n/a — test-runner rule.
- **preconditions:** n/a — test-runner rule.
- **evaluation_and_value_semantics:** The conformance runner enforces a wall-clock limit per case. If it kills a program, the result is recorded as `watchdog`, distinct from exit status 3 and from any S-Go outcome.
- **ownership_and_lifecycle:** n/a — external to the program.
- **effect_and_capability_requirements:** n/a — external to the program.
- **concurrency_and_memory_order:** n/a — external to the program.
- **allowed_nondeterminism:** n/a — external to the program.
- **failure_and_partial_progress:** A `watchdog` result fails the case unless the case expects it (e.g. a non-cooperative child without a drain budget).
- **cancellation_and_cleanup:** n/a — external to the program.
- **compile_link_checks:** n/a — external to the program.
- **runtime_guards:** n/a — external to the program.
- **tcb_assumptions:** n/a — host tool, not target code.
- **positive_tests:** C08; fixture:drain-noncooperative-no-budget.
- **negative_tests:** n/a — runner behavior; checked by the runner's own tests.
- **implementation_coverage:** W01: `src/runtime/sgo_nursery.go`, `src/runtime/sgo_main.go`, `src/cmd/sgo/test.go`, `src/cmd/sgo/lab.go`.
- **executed_evidence:** conformance fixtures drain-noncooperative-no-budget (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w01/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
