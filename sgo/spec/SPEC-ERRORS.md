# SPEC-ERRORS — Diagnostics, resource exhaustion and runtime-check versus compile-error policy

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §15, decision D16, resolution R9.

### SGO-ERR-001 — Compile-time diagnostics
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-LEX-001, SGO-TEST-001.
- **grammar_and_types:** n/a — toolchain rule.
- **preconditions:** n/a — toolchain rule.
- **evaluation_and_value_semantics:** Every rejection reports a stable code (`SGO_E_…`), the source position `file:line:column`, a message, and the clause or decision ID that requires it. Codes used by subset-0 clauses are defined in those clauses; `SGO_E_UNADMITTED` is the general denial, and its message names the denied construct. The compiler reports diagnostics in source order per file and file order by path. A diagnostic is never downgraded to a warning, and there is no flag that suppresses a rejection. Denial is an admission pass that runs before type checking: if it finds any denied construct, only admission diagnostics are reported for that package, one per outermost denied construct, in source order. The position of an `SGO_E_UNADMITTED` diagnostic is the first token that belongs only to the denied construct: the `[` of an index, slice, or type-parameter list; the `struct`, `chan`, `map`, `interface`, `select`, `range`, `goto`, `fallthrough`, or `type` keyword; the `&` of an address-of; the `...` of a variadic; the backslash of a denied escape; the literal of a denied literal; the identifier of a denied predeclared name, package member, or conversion type; the left operand of a denied comparison; the import spec of a denied import; the receiver `(` of a method declaration; the first byte of a denied file. A reference to a member of package `sgo` that is not admitted (`SGO_SPEC_SUBSET0.md` §0.2) is `SGO_E_UNADMITTED`, not `SGO_E_UNDEFINED`. Command-line errors (unknown commands or flags, denied flags, non-admitted targets) have no source position and are reported at the pseudo-position `sgo:1:1`.
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
- **status:** frozen
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
- **implementation_coverage:** W03: `src/cmd/compile/internal/sgoadmit/admit.go`, `src/runtime/sgo_record.go`, `src/runtime/sgo_main.go`.
- **executed_evidence:** conformance fixtures err-record-panic, nur-two-failures-lowest-id (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-ERR-003 — Resource exhaustion and runtime defects
- **version:** 1
- **status:** frozen
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
- **implementation_coverage:** W03: `src/cmd/compile/internal/sgoadmit/admit.go`, `src/runtime/sgo_record.go`, `src/runtime/sgo_main.go`.
- **executed_evidence:** conformance fixtures err-stack-overflow-exit-4 (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-ERR-004 — `check`: checked propagation
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-LEX-003, SGO-CTL-001, SGO-CTL-004, SGO-BIND-002, SGO-VAL-005, SGO-DEF-001, SGO-TYPE-005, SGO-TYPE-006, SGO-NUR-004, SGO-RES-001, SGO-RES-003; D147, D156.
- **grammar_and_types:** `check` is a reserved word. Two forms, each with a single call `c` whose last result has type `error`: the statement `check c`, and the whole right side of an assignment or short declaration, `x₁, …, xₙ = check c` or `x₁, …, xₙ := check c`, whose left side binds the results of `c` except the last (count and types as for an assignment of those results; `_` admitted per position). The enclosing function or function literal has a last result of type `error`.
- **preconditions:** n/a: a static rule.
- **evaluation_and_value_semantics:** `c` is evaluated once (SGO-VAL-005). Let `e` be its last result. If `e` is nil, the targets are assigned the other results and execution continues; the statement form discards them. Otherwise no target is assigned, and the enclosing function returns as the statement `return z₁, …, zₘ₋₁, e` would at that point, where each `zᵢ` is the zero value of the `i`th result type. That includes crossing nursery statements (SGO-CTL-004), assigning named results, and running deferred calls. A function with a result of a resource type (SGO-RES-001) returns only `e` on this path: its resource results are nil and are never observed by a `check` in the caller. A hand-written `return nil, err` is checked at run time instead (runtime_guards). A written `return` takes the same path: it may give `nil` for a resource result exactly when its last operand is not the literal `nil`.
- **ownership_and_lifecycle:** none of its own.
- **effect_and_capability_requirements:** those of `c`.
- **concurrency_and_memory_order:** none of its own; the implied return synchronizes as a written return does.
- **allowed_nondeterminism:** none.
- **failure_and_partial_progress:** as the implied `return`. A cancellation error propagates unchanged: `IsCancelled` still holds for the caller.
- **cancellation_and_cleanup:** none of its own; `c`'s cancellation points are unchanged.
- **compile_link_checks:** `SGO_E_RESERVED_WORD` (`check` as a name); `SGO_E_CHECK_NOT_ERROR` (the last result of `c` is not `error`, or `c` is not a call); `SGO_E_CHECK_NO_RESULT` (the enclosing function has no trailing `error` result); `SGO_E_CAPABILITY_MISUSE` (`nil` for a resource result in a `return` whose error operand is the literal `nil`); `SGO_E_UNADMITTED` (any other position: an operand, a condition, an argument, a `return` operand, an `if`, `for`, or `switch` init statement).
- **runtime_guards:** A `return` that gives the literal `nil` for a resource result checks at run time that its error operand is not nil; otherwise the task fails with `InvalidArgument` at the `return` ("a function returned a nil resource with a nil error"), so a resource result is nil only together with an error (W21 review Z4). `check` itself is lowered to the comparison and the return.
- **tcb_assumptions:** the lowering in the noder.
- **positive_tests:** fixture:err-check-value, fixture:err-check-multi, fixture:err-check-statement, fixture:err-check-assign-on-success, fixture:err-check-in-nursery, fixture:err-check-in-spawn, fixture:err-check-cancellation, fixture:res-return-error-path. Also fixture:res-return-nil-nil-runtime.
- **negative_tests:** fixture:err-check-not-error, fixture:err-check-no-result, fixture:err-check-in-expression, fixture:err-check-in-condition, fixture:err-check-in-init, fixture:err-check-reserved-word, fixture:res-return-nil-nil.
- **implementation_coverage:** W21.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab.
