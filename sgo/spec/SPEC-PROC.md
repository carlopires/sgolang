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
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-PROC-001, SGO-PROC-005, SGO-DEF-003, SGO-NUR-004, SGO-DRAIN-002, SGO-ERR-002, SGO-ERR-003; D08, D179; R6.
- **grammar_and_types:** n/a — process rule.
- **preconditions:** n/a — process rule.
- **evaluation_and_value_semantics:** The process exit status is: 0 if `main` returns `nil`; 1 if `main` returns a non-nil error or a `ChildError` unwinds out of `main`; 2 if any other failure kind (SGO-DEF-003) unwinds out of `main`; 3 on `DrainBudgetExceeded` (SGO-DRAIN-002); 4 on resource exhaustion or `RuntimeDefect` (SGO-ERR-003). For statuses 1–4 exactly one diagnostic record is written to stderr (SGO-ERR-002); for a returned error, its kind is `MainError` and its message is `err.Error()`. Nothing is written to stderr on status 0. An interrupt (SGO-PROC-005) is a cancellation of the root: the program's status is that of its unwind. A second interrupt during the unwind ends the process at once with status 130 (SIGINT) or 143 (SIGTERM), outside S-Go's statuses, writing no record.
- **ownership_and_lifecycle:** n/a — process rule.
- **effect_and_capability_requirements:** The exit is a runtime effect.
- **concurrency_and_memory_order:** All tasks have completed before statuses 0–2 are produced; statuses 3–4, 130, and 143 are fail-stop.
- **allowed_nondeterminism:** None given the program's outcome.
- **failure_and_partial_progress:** Stdout and stderr writes that returned have reached the OS (SGO-PROC-003).
- **cancellation_and_cleanup:** Statuses 0–2: all deferred calls ran. Statuses 3–4, 130, and 143: deferred calls of live tasks did not run.
- **compile_link_checks:** n/a — process rule.
- **runtime_guards:** Exit-status mapping in the runtime.
- **tcb_assumptions:** Process exit primitive (SGO-TCB-001).
- **positive_tests:** S08; fixture:proc-main-ok, fixture:proc-main-error-exit-1, fixture:nur-child-error-exit-1, fixture:def-panic-exit-2; `sgo/scripts/test-cli.py` (a second SIGINT or SIGTERM: 130, 143).
- **negative_tests:** n/a — mapping rule; each status has a positive test.
- **implementation_coverage:** W03: `src/cmd/compile/internal/noder/sgo_main.go`, `src/runtime/sgo_main.go`, `src/sgo/sgo.go`; W24: `src/runtime/sgo_interrupt.go`.
- **executed_evidence:** conformance fixtures def-panic-exit-2, nur-child-error-exit-1, proc-main-error-exit-1, proc-main-ok (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`; `sgo/scripts/test-cli.py` (darwin/arm64, linux/amd64, linux/arm64), `sgo/evidence/w24/README.md`.
- **supported_execution_configurations:** production; lab (SGO-LAB-007 injects interrupts).

### SGO-PROC-003 — Standard output and error
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-CAP-001, SGO-TYPE-005, SGO-DEF-003, SGO-MEM-004, SGO-STD-008; D09, D178; CAN-3.
- **grammar_and_types:** `ctx.Stdout() *sgo.Writer` and `ctx.Stderr() *sgo.Writer` where `ctx` has type `*sgo.Context`; on `w *sgo.Writer`: `WriteString(s string) error`, `Write(s string) (int, error)`, `IsTerminal() bool`, `TerminalWidth() int`; `sgo.IsBrokenPipe(err error) bool`. A `*sgo.Writer` converts to `io.Writer` (SGO-STD-008). `print`, `println`, and every other output path are denied.
- **preconditions:** `Stdout` and `Stderr` are called on `root`; the writer's methods may be called by any task holding `w`.
- **evaluation_and_value_semantics:** `root.Stdout()` and `root.Stderr()` return the process's single writer of each stream (the same handle every time). `WriteString(s)` writes all bytes of `s` to the stream, unbuffered: when it returns `nil`, every byte has been handed to the OS; `Write(s)` does the same and returns the number of bytes written. If the OS reports an error, the result is an I/O error with its kind whose progress is the number of bytes written before it (SGO-ERR-001). A write whose reader has gone (a closed pipe) returns an error for which `sgo.IsBrokenPipe` holds (kind `BrokenPipe`); SIGPIPE never ends the process, so a program may end quietly, and otherwise `main` returning the error exits with status 1 (SGO-PROC-002). `IsTerminal` reports whether the stream is a terminal and `TerminalWidth` its width in columns (0 when not a terminal or unknown).
- **ownership_and_lifecycle:** The writers live for the whole process.
- **effect_and_capability_requirements:** Holding `w` is the authority to write to its stream. Children start without it (SGO-SPAWN-003) and receive it only by capture or parameter.
- **concurrency_and_memory_order:** Writes to one stream are atomic with respect to each other: the bytes of one call are never interleaved with another's. Calls are totally ordered (SGO-MEM-004).
- **allowed_nondeterminism:** The order of writes from different tasks that are not ordered by happens-before; between the two streams, the order the reader sees.
- **failure_and_partial_progress:** On an OS error, the bytes already written stay written (CAN-3); retrying is the caller's choice.
- **cancellation_and_cleanup:** A write is not a cancellation point and is not interrupted by cancellation.
- **compile_link_checks:** `SGO_E_UNADMITTED` (`print`, `println`, other output); `SGO_E_DISCARDED_ERROR` if the result is unused.
- **runtime_guards:** `Stdout()` or `Stderr()` on a context other than `root` fails with `MissingCapability`.
- **tcb_assumptions:** The stream adapters perform complete writes under a lock and ignore SIGPIPE (SGO-TCB-001).
- **positive_tests:** S06, S13; fixture:proc-hello, fixture:proc-writer-passed-to-child, fixture:proc-stderr; `sgo/scripts/test-cli.py` (a broken pipe ends quietly under `| head -1`; terminal detection and width on a pseudo-terminal).
- **negative_tests:** S06, S13; fixture:proc-child-stdout, fixture:proc-println, fixture:proc-stderr-child.
- **implementation_coverage:** W03, W05: `src/cmd/compile/internal/noder/sgo_main.go`, `src/runtime/sgo_main.go`, `src/sgo/sgo.go`; W24: `src/runtime/sgo_proc.go`.
- **executed_evidence:** conformance fixtures proc-child-stdout, proc-hello, proc-println, proc-writer-passed-to-child, proc-stderr, proc-stderr-child (darwin/arm64, linux/amd64, linux/arm64); `sgo/scripts/test-cli.py`; `sgo/evidence/w03/README.md`; `sgo/evidence/w05/README.md`; `sgo/evidence/w24/README.md`.
- **supported_execution_configurations:** production; lab (the streams are captured; `IsTerminal` is false and `TerminalWidth` 0).

### SGO-PROC-004 — Arguments, environment, and standard input
- **version:** 3
- **status:** frozen
- **reference_closure:** SGO-ERR-001, SGO-CAP-001, SGO-FILE-004, SGO-IMP-002, SGO-LAB-007; D08, D84, D178, D180.
- **grammar_and_types:** `root.Args() []string`; `root.Getenv(name string) (string, bool)`; `root.Stdin() *sgo.Input`, and on `in *sgo.Input`: `Read(max int) (string, error)`, `ReadLine(max int) (string, error)`, `ReadAll(max int) (string, error)`, `ReadSecret(max int) (string, error)`, `IsTerminal() bool`. In the main module's `sgo.mod`: `grant env NAME` or `grant env PREFIX*`, NAME (and PREFIX) of `[A-Z0-9_]`, not starting with a digit, at most 64 bytes. There is no other access to the process's environment.
- **preconditions:** The methods are called on `root`.
- **evaluation_and_value_semantics:** `Args` returns a fresh copy of the arguments after the program's name; they carry no authority (SGO-FILE-002 aside, for `grant path args`). `Getenv(name)` returns the variable's value and whether it is set, for a name a grant covers (equal to NAME, or starting with PREFIX). `Stdin` returns the process's standard input: the reads are those of a handle (SGO-FILE-003, SGO-FILE-004), on bytes a runtime pump reads from fd 0 with blocking reads, so fd 0's flags and the parent's open file are unchanged and the bytes read belong to this process. `ReadSecret` reads a line as `ReadLine` does with the terminal's echo off while it reads, restored on every exit (return, cancellation, interrupt, failure); on a non-terminal it is `ReadLine`. `IsTerminal` reports whether stdin is a terminal. No environment variable changes the runtime's behavior.
- **ownership_and_lifecycle:** The input handle lives for the whole process; `Args` copies belong to the caller.
- **effect_and_capability_requirements:** The root holds the arguments, the granted environment, and stdin; children receive only what the root passes them (SGO-CAP-001).
- **concurrency_and_memory_order:** Reads of stdin run one at a time (the read turn of SGO-FILE-004).
- **allowed_nondeterminism:** The arguments, the environment, and the input are the process's: the program's behavior may depend on them.
- **failure_and_partial_progress:** As SGO-FILE-004 for reads; at the end of the input, an error for which `sgo.IsEOF` holds.
- **cancellation_and_cleanup:** A read of stdin is a cancellation point: cancellation or an interrupt (SGO-PROC-005) ends it.
- **compile_link_checks:** `SGO_E_UNADMITTED` for any other access (e.g. importing `os`); `SGO_E_USAGE` (in `sgo.mod`) for a malformed env grant. The grants are canonical I/O grant lines (`env NAME`), part of the build identity (SGO-IMP-002); the linker accepts only that form.
- **runtime_guards:** A context other than `root` fails with `MissingCapability`; so does `Getenv` of a name no grant covers.
- **tcb_assumptions:** An S-Go program reads no runtime environment variable: `GODEBUG`, `GOTRACEBACK`, `GOGC`, `GOMEMLIMIT`, `GOMAXPROCS` (D84), and every other upstream runtime setting are ignored, and their effects are fixed at build time (SGO-TCB-001). The linker marks every binary it links as an S-Go program; the runtime reads its environment settings only in a binary without the mark (W12 R2). The stdin pump and the terminal echo control (SGO-TCB-001).
- **positive_tests:** S13; fixture:proc-args, fixture:proc-env-granted, fixture:proc-env-ignored-ok, fixture:proc-stdin; `sgo/scripts/test-cli.py` (installed-binary arguments, idle stdin ends at its deadline, ReadSecret hides the secret).
- **negative_tests:** S13; fixture:bind-import-fmt, fixture:proc-godebug-ignored, fixture:proc-env-ignored, fixture:proc-args-child, fixture:proc-stdin-child, fixture:proc-env-ungranted, fixture:proc-env-grant-bad.
- **implementation_coverage:** W00: `src/cmd/compile/internal/noder/sgo_main.go`, `src/runtime/sgo_main.go`, `src/sgo/sgo.go`; W24: `src/runtime/sgo_stdin.go`, `src/runtime/sgo_proc.go`, `src/cmd/sgo/iogrant.go`.
- **executed_evidence:** conformance fixtures bind-import-fmt, proc-godebug-ignored, proc-env-ignored, proc-env-ignored-ok, proc-args, proc-args-child, proc-env-granted, proc-env-ungranted, proc-env-grant-bad, proc-stdin, proc-stdin-child (darwin/arm64, linux/amd64, linux/arm64); `sgo/scripts/test-cli.py`; `sgo/evidence/w00/README.md`; `sgo/evidence/w24/README.md`.
- **supported_execution_configurations:** production; lab (SGO-LAB-007: the arguments after `--`, `-env`, and `-stdin`, recorded in the trace).

### SGO-PROC-005 — Interrupts
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-PROC-001, SGO-PROC-002, SGO-CANCEL-001, SGO-CANCEL-003, SGO-NUR-004, SGO-RES-002, SGO-LAB-007; D179.
- **grammar_and_types:** `sgo.IsInterrupted(err error) bool`. No other signal API exists: signals stay denied (SGO-PROC-001).
- **preconditions:** None.
- **evaluation_and_value_semantics:** The first SIGINT or SIGTERM cancels the root scope with a cancellation for which `sgo.IsInterrupted` and `sgo.IsCancelled` hold, whose message is `cancelled: interrupted (SIGINT)` or `cancelled: interrupted (SIGTERM)`. The program unwinds through its nurseries as for any cancellation, releasing every resource in the order its scopes guarantee, and exits with the status of that unwind (SGO-PROC-002): a `main` returning the cancellation exits 1 with a `MainError` record. A second SIGINT or SIGTERM during the unwind ends the process at once, with status 130 or 143. Other signals keep their default actions; SIGPIPE is ignored (SGO-PROC-003).
- **ownership_and_lifecycle:** n/a — the interrupt is an event, not a value.
- **effect_and_capability_requirements:** None: every program receives interrupts.
- **concurrency_and_memory_order:** The cancellation is published from a runtime thread, never from the signal handler; it happens-before every task's observation of it (SGO-CANCEL-001).
- **allowed_nondeterminism:** When the interrupt arrives relative to the program's tasks.
- **failure_and_partial_progress:** As for cancellation: operations in progress end at their cancellation points (SGO-CANCEL-003).
- **cancellation_and_cleanup:** The unwind is a cancellation of the root: deferred calls run and resources are released; after a second interrupt they do not.
- **compile_link_checks:** n/a — runtime rule.
- **runtime_guards:** The signal handler only records the signal and wakes the interrupt thread; the deadlock detector counts the thread as idle.
- **tcb_assumptions:** The interrupt thread and its signal routing (the service "interrupt", SGO-TCB-001).
- **positive_tests:** C05; `sgo/scripts/test-cli.py` (SIGINT and SIGTERM unwind a reader and a sleeper, closing their nursery; a second signal ends the process with 130 or 143; lab interrupts replay); the model `sgo/model/interruptmodel.go` (an interrupt racing scope ends).
- **negative_tests:** C05; `sgo/scripts/test-cli.py` (a second signal ends the unwind with 130 or 143 instead of its status).
- **implementation_coverage:** W24: `src/runtime/sgo_interrupt.go`, `src/runtime/sgo_interrupt_darwin.go`, `src/runtime/sgo_interrupt_note.go`, `src/runtime/signal_unix.go`, `src/sgo/sgo.go`.
- **executed_evidence:** `sgo/scripts/test-cli.py` (darwin/arm64, linux/amd64, linux/arm64); `sgo/model` tests; `sgo/evidence/w24/README.md`.
- **supported_execution_configurations:** production; lab (SGO-LAB-007 injects interrupts at virtual times).
