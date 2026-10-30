# SPEC-TCB — Runtime/GC/service/assembly/intrinsic contracts and sealed callers

Subset 0. Derived from plan §8.1, §10.2, stage map §2 (T0), invariant OWN-6, and constitution §2 ("trust does not waive specification").

### SGO-TCB-001 — Trusted primitives used by subset 0
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-TCB-002, SGO-SPAWN-003, SGO-NUR-003, SGO-CANCEL-001, SGO-WAIT-001, SGO-TIMER-001, SGO-PROC-003, SGO-PROC-002, SGO-ERR-003; D22; OWN-6.
- **grammar_and_types:** n/a — runtime contract.
- **preconditions:** Each primitive is reachable only from compiler-generated code for its clause or from the `sgo` package's implementation, never from user source.
- **evaluation_and_value_semantics:** Subset-0 clauses rely on exactly these primitives, each specified by the clause cited: task creation and registration (SGO-SPAWN-003); scope state, join, park/unpark (SGO-NUR-002, SGO-NUR-003, SGO-WAIT-002); cancellation propagation and query (SGO-CANCEL-001, SGO-CANCEL-002); monotonic timers and sleep (SGO-WAIT-001, SGO-TIMER-001); stdout write (SGO-PROC-003); process exit and diagnostic record (SGO-PROC-002, SGO-ERR-002); failure unwinding and defers (SGO-DEF-001..005); memory allocation and garbage collection, including keeping captured variables and runtime records alive (SGO-MEM-001, GC-1, GC-2); stack growth and overflow detection (SGO-STACK-001, SGO-ERR-003). Runtime-internal goroutines (GC workers, timers, signal handling) are created only by sealed service primitives with an enumerated purpose, never by `go` in user-visible code (OWN-6).
- **ownership_and_lifecycle:** Runtime records for scopes, tasks, waits, and timers are heap objects reachable from audited GC roots (plan §5.3).
- **effect_and_capability_requirements:** Only the stdout write and process exit have external effects.
- **concurrency_and_memory_order:** Each primitive establishes the happens-before edges of SGO-MEM-004 on darwin/arm64, linux/amd64, and linux/arm64, using the runtime's atomics with explicit ordering (no reliance on amd64 total store order, D22).
- **allowed_nondeterminism:** Scheduling; nothing else visible to user code.
- **failure_and_partial_progress:** A primitive that cannot satisfy its contract terminates the process as `RuntimeDefect` or `ResourceExhausted` (SGO-ERR-003); it never returns an unspecified result.
- **cancellation_and_cleanup:** The runtime never kills an individual task (SGO-DRAIN-001).
- **compile_link_checks:** The list of primitives and their callers is recorded in `sgo/TARGET_TCB.json` (W01 step 4); the inventory check (W01 step 3) fails if compiler-generated code calls a runtime entry point that is not listed.
- **runtime_guards:** Internal assertions per primitive; violations are `RuntimeDefect`.
- **tcb_assumptions:** This clause is the TCB boundary; it relies on SGO-TCB-002 for how the runtime source itself is admitted.
- **positive_tests:** S11; fixture:spawn-basic, fixture:wait-sleep, fixture:proc-hello.
- **negative_tests:** S11; inventory check failure on an unlisted runtime call (W01 step 3).
- **implementation_coverage:** none yet — W01 step 4 (manifest), W02–W04 (implementation).
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-TCB-002 — Admission of the runtime source
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-TCB-001, SGO-IMP-002, SGO-NATIVE-001; stage map §2–§3; OWN-6. Approved by the user on 2026-10-30.
- **grammar_and_types:** n/a — build-trust rule.
- **preconditions:** n/a — build-trust rule.
- **evaluation_and_value_semantics:** The runtime's package closure on the admitted targets (30 packages: `runtime`, its `internal/…` dependencies, `math/bits`, `unsafe`) is Go source using `unsafe`, assembly, and linkname. It is admitted as trusted target code only by exact identity, recorded in `sgo/TARGET_TCB.json`: every `.go`, `.s`, and `.h` file of the closure (excluding tests) with its SHA-256 and whether it is identical to the upstream commit; the trusted primitives of SGO-TCB-001 with their clauses, upstream implementation symbols, and allowed callers; and every `go` statement in the closure as an enumerated runtime service with its purpose and status (active, dormant, not-built). Changing any listed file, adding or removing a file, or adding or removing a runtime `go` statement fails the check until the manifest is regenerated in a commit that reviews the change. Being listed grants nothing to user code: user-level `unsafe`, assembly, linkname, and cgo stay denied (SGO-NATIVE-001).
- **ownership_and_lifecycle:** The manifest is owned by the repository; each regeneration is a reviewed commit.
- **effect_and_capability_requirements:** n/a — build-trust rule.
- **concurrency_and_memory_order:** n/a — build-trust rule.
- **allowed_nondeterminism:** None: admission is a function of file digests.
- **failure_and_partial_progress:** A mismatch fails the check (CI now; the compiler from W03) with `SGO_E_TCB_DIGEST`; nothing is built from unreviewed runtime source.
- **cancellation_and_cleanup:** n/a — build-trust rule.
- **compile_link_checks:** `SGO_E_TCB_DIGEST` (runtime file or service list differs from the manifest). W01: `sgo/scripts/tcb-manifest.py` in CI. W03: the compiler and linker verify the same digests before accepting runtime packages, and refuse a runtime entry point called from a caller not listed for its primitive (`SGO_E_TCB_CALLER`).
- **runtime_guards:** n/a — build time.
- **tcb_assumptions:** This clause defines how trust is granted.
- **positive_tests:** S11; `tcb-manifest.py` check passes on the reviewed manifest.
- **negative_tests:** S11, S15; self-tests: an edited runtime file and an added runtime `go` statement both fail the check.
- **implementation_coverage:** W01 — `sgo/scripts/tcb-manifest.py`, `sgo/TARGET_TCB.json` (1,020 files, 11 enumerated services); compiler/linker enforcement W03.
- **executed_evidence:** 2026-10-30, darwin/arm64: check passed; both negative self-tests failed as required.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
