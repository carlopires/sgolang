# SPEC-ERRORS — Diagnostics, resource exhaustion and runtime-check versus compile-error policy

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §15, decision D16, resolution R9.

### SGO-ERR-001 — Compile-time diagnostics
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-LEX-001, SGO-TEST-001.
- **grammar_and_types:** n/a — toolchain rule.
- **preconditions:** n/a — toolchain rule.
- **evaluation_and_value_semantics:** Every rejection reports a stable code (`SGO_E_…`), the source position `file:line:column`, a message, and the clause or decision ID that requires it. Codes used by subset-0 clauses are defined in those clauses; `SGO_E_UNADMITTED` is the general denial, and its message names the denied construct. The compiler reports diagnostics in source order per file and file order by path. A diagnostic is never downgraded to a warning, and there is no flag that suppresses a rejection.
- **ownership_and_lifecycle:** n/a — toolchain rule.
- **effect_and_capability_requirements:** n/a — toolchain rule.
- **concurrency_and_memory_order:** n/a — toolchain rule.
- **allowed_nondeterminism:** None: the set and order of diagnostics is deterministic for a given input.
- **failure_and_partial_progress:** Any error diagnostic means no object file for the package and no linked program.
- **cancellation_and_cleanup:** n/a — toolchain rule.
- **compile_link_checks:** This clause defines the reporting format; conformance cases compare codes and positions exactly (SGO-TEST-001).
- **runtime_guards:** n/a — toolchain rule.
- **tcb_assumptions:** n/a — toolchain only.
- **positive_tests:** S03; fixture:err-diagnostic-format.
- **negative_tests:** S01, S17; fixture:err-no-suppression-flag.
- **implementation_coverage:** gate level only — `sgo-target` reports `SGO_E_UNADMITTED` for every program (W00).
- **executed_evidence:** W00 gate checks passed on all three targets.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-ERR-002 — Runtime diagnostic record
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-PROC-002, SGO-DEF-003, SGO-DEF-004, SGO-NUR-004; R9.
- **grammar_and_types:** n/a — output format.
- **preconditions:** The process is ending with status 1–4.
- **evaluation_and_value_semantics:** Exactly one line is written to stderr: a JSON object with keys in this order: `"sgo_diagnostic"` (the integer 1), `"kind"` (one of the kinds of SGO-DEF-003, `MainError`, `DrainBudgetExceeded`, `ResourceExhausted`, `RuntimeDefect`), `"message"` (string), `"position"` (`"file:line:col"` relative to the module root, or `null`), `"task_id"` (integer creation ID, root = 0), `"scope_id"` (integer or `null`), `"chain"` (array of objects with keys `"kind"`, `"message"`, `"position"`, in occurrence order). Strings are JSON-escaped UTF-8; there is no whitespace outside strings; the line ends with `\n`.
- **ownership_and_lifecycle:** n/a — output format.
- **effect_and_capability_requirements:** Written by the runtime, not by user code.
- **concurrency_and_memory_order:** Written after all other output of the process for statuses 1–2; for statuses 3–4 after the fail-stop decision.
- **allowed_nondeterminism:** None given the outcome: no addresses, wall-clock times, or goroutine IDs appear.
- **failure_and_partial_progress:** If writing the record fails, the exit status is still produced.
- **cancellation_and_cleanup:** n/a — output format.
- **compile_link_checks:** n/a — run-time output.
- **runtime_guards:** n/a — output format.
- **tcb_assumptions:** Runtime failure reporting replaced by this format; upstream tracebacks are not printed (SGO-TCB-001).
- **positive_tests:** S08; fixture:err-record-panic, fixture:nur-two-failures-lowest-id.
- **negative_tests:** n/a — format rule; positive fixtures compare the record byte for byte.
- **implementation_coverage:** none yet — W03.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-ERR-003 — Resource exhaustion and runtime defects
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-PROC-002, SGO-ERR-002; D16.
- **grammar_and_types:** n/a — runtime rule.
- **preconditions:** n/a — runtime rule.
- **evaluation_and_value_semantics:** Stack overflow, heap exhaustion, exceeding the string-length limit (SGO-VAL-002), or exceeding the task-count limit terminates the process with exit status 4 and a record of kind `ResourceExhausted` naming the resource. A violated runtime invariant (SGO-NUR-002, SGO-NUR-003, SGO-DEF-003) terminates with status 4 and kind `RuntimeDefect`. These are not failures of the task: they cannot be observed by user code, and deferred calls do not run.
- **ownership_and_lifecycle:** n/a — fail-stop.
- **effect_and_capability_requirements:** n/a — runtime rule.
- **concurrency_and_memory_order:** Fail-stop stops all tasks.
- **allowed_nondeterminism:** Whether a resource limit is reached at all depends on the host; the reaction is fixed.
- **failure_and_partial_progress:** Effects already performed remain.
- **cancellation_and_cleanup:** No cleanup runs.
- **compile_link_checks:** Limits are recorded in build metadata and run manifests.
- **runtime_guards:** Stack, heap, string, and task-count checks.
- **tcb_assumptions:** Upstream stack-overflow and out-of-memory detection, with their reports replaced by this record (SGO-TCB-001).
- **positive_tests:** S08; fixture:err-stack-overflow-exit-4.
- **negative_tests:** n/a — fail-stop rule; checked by the positive fixture.
- **implementation_coverage:** none yet — W03.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
