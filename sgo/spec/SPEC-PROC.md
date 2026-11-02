# SPEC-PROC — Process entry/exit status, stdout/stderr, arguments and environment

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §13, decisions D08, D09, D16, resolutions R6, R9.

### SGO-PROC-001 — Entry point
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-BIND-001, SGO-TYPE-005, SGO-CANCEL-004, SGO-PROC-002; D08.
- **grammar_and_types:** Package `main` declares exactly one function `main` with signature `func main(root *sgo.Context) error` (the parameter name is free). Any other signature, a Go-style `func main()`, or a missing `main` is a compile error. No other package may declare a package-level function named `main` with this signature as an entry point.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** The runtime starts the root task (creation ID 0), which calls `main(root)`. `root` holds the stdout capability (SGO-PROC-003) and is linked to no scope, so it reports cancellation only inside nurseries it executes (SGO-CANCEL-004). The process ends when `main` ends (SGO-PROC-002).
- **ownership_and_lifecycle:** No task outlives `main`: every task belongs to a nursery that closes before `main` returns (OWN-3).
- **effect_and_capability_requirements:** `root` is the only source of process capabilities.
- **concurrency_and_memory_order:** n/a — single root task.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** See SGO-PROC-002.
- **cancellation_and_cleanup:** Subset 0 has no external way to cancel `root`; OS signals are denied (their default actions terminate the process outside S-Go's outcomes).
- **compile_link_checks:** `SGO_E_MAIN_SIGNATURE`.
- **runtime_guards:** n/a — static rule.
- **tcb_assumptions:** Runtime start-up runs no user code before `main` (no `init`, SGO-BIND-003) (SGO-TCB-001).
- **positive_tests:** S06; fixture:proc-main-ok.
- **negative_tests:** S01; fixture:proc-go-main, fixture:proc-no-main.
- **implementation_coverage:** W03: `src/cmd/compile/internal/noder/sgo_main.go`, `src/runtime/sgo_main.go`, `src/sgo/sgo.go`.
- **executed_evidence:** conformance fixtures proc-go-main, proc-main-ok, proc-no-main (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-PROC-002 — Exit status
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-PROC-001, SGO-DEF-003, SGO-NUR-004, SGO-DRAIN-002, SGO-ERR-002, SGO-ERR-003; D08; R6.
- **grammar_and_types:** n/a — process rule.
- **preconditions:** n/a — process rule.
- **evaluation_and_value_semantics:** The process exit status is: 0 if `main` returns `nil`; 1 if `main` returns a non-nil error or a `ChildError` unwinds out of `main`; 2 if any other failure kind (SGO-DEF-003) unwinds out of `main`; 3 on `DrainBudgetExceeded` (SGO-DRAIN-002); 4 on resource exhaustion or `RuntimeDefect` (SGO-ERR-003). For statuses 1–4 exactly one diagnostic record is written to stderr (SGO-ERR-002); for a returned error, its kind is `MainError` and its message is `err.Error()`. Nothing is written to stderr on status 0.
- **ownership_and_lifecycle:** n/a — process rule.
- **effect_and_capability_requirements:** The exit is a runtime effect.
- **concurrency_and_memory_order:** All tasks have completed before statuses 0–2 are produced; statuses 3–4 are fail-stop.
- **allowed_nondeterminism:** None given the program's outcome.
- **failure_and_partial_progress:** Stdout writes that returned have reached the OS (SGO-PROC-003).
- **cancellation_and_cleanup:** Statuses 0–2: all deferred calls ran. Statuses 3–4: deferred calls of live tasks did not run.
- **compile_link_checks:** n/a — process rule.
- **runtime_guards:** Exit-status mapping in the runtime.
- **tcb_assumptions:** Process exit primitive (SGO-TCB-001).
- **positive_tests:** S08; fixture:proc-main-ok, fixture:proc-main-error-exit-1, fixture:nur-child-error-exit-1, fixture:def-panic-exit-2.
- **negative_tests:** n/a — mapping rule; each status has a positive fixture.
- **implementation_coverage:** W03: `src/cmd/compile/internal/noder/sgo_main.go`, `src/runtime/sgo_main.go`, `src/sgo/sgo.go`.
- **executed_evidence:** conformance fixtures def-panic-exit-2, nur-child-error-exit-1, proc-main-error-exit-1, proc-main-ok (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-PROC-003 — Stdout capability
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-CAP-001, SGO-TYPE-005, SGO-DEF-003, SGO-MEM-004; D09; CAN-3.
- **grammar_and_types:** `ctx.Stdout() *sgo.Writer` where `ctx` has type `*sgo.Context`; `w.WriteString(s string) error` where `w` has type `*sgo.Writer`. `print`, `println`, and every other output path are denied.
- **preconditions:** `Stdout` is called on `root`; `WriteString` may be called by any task holding `w`.
- **evaluation_and_value_semantics:** `root.Stdout()` returns the process's single writer (the same handle every time). `WriteString(s)` writes all bytes of `s` to the process's standard output, unbuffered: when it returns `nil`, every byte has been handed to the OS. If the OS reports an error, it returns a non-nil error whose message includes the number of bytes written before the error.
- **ownership_and_lifecycle:** The writer lives for the whole process.
- **effect_and_capability_requirements:** Holding `w` is the authority to write to stdout. Children start without it (SGO-SPAWN-003) and receive it only by capture or parameter.
- **concurrency_and_memory_order:** `WriteString` calls are atomic with respect to each other: the bytes of one call are never interleaved with another's. Calls are totally ordered (SGO-MEM-004).
- **allowed_nondeterminism:** The order of `WriteString` calls from different tasks that are not ordered by happens-before.
- **failure_and_partial_progress:** On an OS error, the bytes already written stay written (CAN-3); retrying is the caller's choice.
- **cancellation_and_cleanup:** `WriteString` is not a cancellation point and is not interrupted by cancellation.
- **compile_link_checks:** `SGO_E_UNADMITTED` (`print`, `println`, other output); `SGO_E_DISCARDED_ERROR` if the result is unused.
- **runtime_guards:** `Stdout()` on a context other than `root` fails with `MissingCapability`.
- **tcb_assumptions:** The stdout adapter performs complete writes under a lock (SGO-TCB-001).
- **positive_tests:** S06, S13; fixture:proc-hello, fixture:proc-writer-passed-to-child.
- **negative_tests:** S06, S13; fixture:proc-child-stdout, fixture:proc-println.
- **implementation_coverage:** W03, W05: `src/cmd/compile/internal/noder/sgo_main.go`, `src/runtime/sgo_main.go`, `src/sgo/sgo.go`.
- **executed_evidence:** conformance fixtures proc-child-stdout, proc-hello, proc-println, proc-writer-passed-to-child (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`; `sgo/evidence/w05/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-PROC-004 — Arguments and environment
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-ERR-001; D08.
- **grammar_and_types:** Subset 0 provides no access to command-line arguments, environment variables, stdin, files, or the working directory.
- **preconditions:** n/a — denial clause.
- **evaluation_and_value_semantics:** n/a — no API exists.
- **ownership_and_lifecycle:** n/a — no API exists.
- **effect_and_capability_requirements:** n/a — no API exists.
- **concurrency_and_memory_order:** n/a — no API exists.
- **allowed_nondeterminism:** None: a subset-0 program's behavior cannot depend on arguments or environment.
- **failure_and_partial_progress:** n/a — no API exists.
- **cancellation_and_cleanup:** n/a — no API exists.
- **compile_link_checks:** `SGO_E_UNADMITTED` for any attempt (e.g. importing `os`).
- **runtime_guards:** n/a — no API exists.
- **tcb_assumptions:** The runtime does not change user-visible behavior based on environment variables; upstream `GODEBUG`/`GOTRACEBACK`-style settings are ignored or fixed at build time (SGO-TCB-001).
- **positive_tests:** n/a — denial clause.
- **negative_tests:** S13; fixture:bind-import-fmt, fixture:proc-godebug-ignored.
- **implementation_coverage:** W00: `src/cmd/compile/internal/noder/sgo_main.go`, `src/runtime/sgo_main.go`, `src/sgo/sgo.go`.
- **executed_evidence:** conformance fixtures bind-import-fmt, proc-godebug-ignored (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w00/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
