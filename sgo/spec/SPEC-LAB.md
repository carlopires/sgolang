# SPEC-LAB — Deterministic lab

Denied in subsets 0 to 2 (spec 0.1.0 to 0.3.0, SGO-LAB-001). Spec 0.4.0 admits the lab execution configuration (SGO-LAB-002 to SGO-LAB-006, D40-D50), selected only by `sgo lab`; the source language is unchanged.

### SGO-LAB-001 — Deterministic lab are denied in subset 0
- **version:** 2
- **status:** superseded
- **reference_closure:** SGO-ERR-001, SGO-LAB-002; D22, D41; plan §9; SPEC-4.
- **grammar_and_types:** Superseded in spec 0.4.0 by SGO-LAB-002 to SGO-LAB-006, which admit the lab execution configuration through `sgo lab`. Still denied: any lab-only API, source construct, pragma, or build flag of `sgo build` or `sgo run`, and any way to enable the lab in a production binary.
- **preconditions:** n/a — denial clause; there is nothing to use.
- **evaluation_and_value_semantics:** n/a — denied constructs never execute.
- **ownership_and_lifecycle:** n/a — denied constructs never execute.
- **effect_and_capability_requirements:** n/a — denied constructs never execute.
- **concurrency_and_memory_order:** n/a — denied constructs never execute.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Any use is a compile error or a usage error; no object file is produced.
- **cancellation_and_cleanup:** n/a — denied constructs never execute.
- **compile_link_checks:** `SGO_E_UNADMITTED` or `SGO_E_USAGE`, naming the construct and this clause.
- **runtime_guards:** n/a — rejected before execution.
- **tcb_assumptions:** n/a — no runtime involvement.
- **positive_tests:** n/a — denial clause; the lab is tested under SGO-LAB-002 to SGO-LAB-006.
- **negative_tests:** S14; fixture:lab-flag.
- **implementation_coverage:** gate level W00; compiler-level denial W03; narrowed in W08 to the forms above (`cmd/sgo` accepts no lab flag outside `sgo lab`).
- **executed_evidence:** `sgo/evidence/w03/README.md`; `sgo/evidence/w08/README.md`.
- **supported_execution_configurations:** production; lab (SGO-LAB-002).

### SGO-LAB-002 — The lab execution configuration
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-LAB-001, SGO-TOOL-001, SGO-IMP-002, SGO-BUILD-001, SGO-PROC-002; D41.
- **grammar_and_types:** No source construct: the language of a lab program is the language of its production program. The configuration is selected only by `sgo lab run`, `sgo lab replay`, and `sgo lab sweep`, which build a **lab binary**; no flag of `sgo build` or `sgo run`, environment variable, file, or pragma selects it.
- **preconditions:** The program is admitted for production (the lab admits no construct production denies).
- **evaluation_and_value_semantics:** A lab binary runs the program under the lab scheduler (SGO-LAB-003) with the run manifest given by the driver; apart from the scheduling, time, and effect rules of SGO-LAB-003 and SGO-LAB-004, every clause applies unchanged.
- **ownership_and_lifecycle:** One run per process (D48). The manifest is fixed for the run.
- **effect_and_capability_requirements:** Grants are those of the production program (SGO-AUTH-002); their lab meaning is SGO-LAB-004.
- **concurrency_and_memory_order:** SGO-LAB-003.
- **allowed_nondeterminism:** None that depends on the host: given the manifest, the canonical event stream is a function of the program.
- **failure_and_partial_progress:** A lab binary started without a manifest ends as `RuntimeDefect` (status 4).
- **cancellation_and_cleanup:** Unchanged.
- **compile_link_checks:** The execution configuration (`prod`, `lab`, or `tcb` for trusted target code shared by both) is part of every S-Go package's identity; a package imports only packages of its own configuration or `tcb`, and the linker links a lab program only from lab and `tcb` packages, else `SGO_E_SPEC_MISMATCH` (SGO-IMP-002). A lab binary's build ID names `config=lab`.
- **runtime_guards:** A production binary has no lab activation path: it never reads a manifest.
- **tcb_assumptions:** `runtime/sgo_lab*.go` (baton scheduler, streams, trace); `cmd/link` sets the lab marker only for `-sgolab`.
- **positive_tests:** S14, D02; fixture:lab-repeat; `sgo/scripts/test-lab.py`.
- **negative_tests:** S14; fixture:lab-rejects-like-production; `test-lab.py` (production binary given a manifest).
- **implementation_coverage:** compiler `noder/sgo_identity.go`; linker `ld/sgoid.go`; driver `cmd/sgo/lab*.go`; runtime `sgo_lab_boot.go`.
- **executed_evidence:** `sgo/evidence/w08/README.md`.
- **supported_execution_configurations:** lab.

### SGO-LAB-003 — Lab scheduling, choice points, steps, and quota
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-LAB-002, SGO-NUR-001, SGO-SPAWN-001, SGO-CHAN-004, SGO-SYNC-002, SGO-WAIT-001, SGO-CANCEL-002; D42, D43, D49, D50.
- **grammar_and_types:** n/a — configuration clause.
- **preconditions:** n/a — configuration clause; it adds no operation with preconditions.
- **evaluation_and_value_semantics:** Exactly one task runs at a time. A **choice point** precedes every spawn and `Start`, nursery join, task end, channel operation and `select`, `Event` and `CapacityLimiter` operation, readiness `Signal`, `Sleep`, `Checkpoint`, effect operation, and `Writer.WriteString`; every `K` steps since the running task's last choice point is also one. A **step** is a function entry or a loop iteration of S-Go code (lab builds count them; production builds do not, and lab builds do not inline). At a choice point the **ready set** is the IDs (creation order, root 0) of the tasks that can run; the scheduler records it and chooses one: `random` (default) uniformly with the `sched` stream, `fifo` the task ready longest. A `select` whose ready arms are two or more records them and the chosen arm, as source indices of the `select` statement (D50); a waiting `select` completed by another task records the arm it completed.
- **ownership_and_lifecycle:** n/a — configuration clause; ownership rules are the production ones.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** The run is a single interleaving: every happens-before edge of the production clauses holds, and nothing else orders operations.
- **allowed_nondeterminism:** None given the manifest: the stream and the output are determined by the program, the seed, the policy, `K`, the quota, and the epoch.
- **failure_and_partial_progress:** The total steps of a run are bounded by the quota; SGO-LAB-006.
- **cancellation_and_cleanup:** Cancellation wakes waiting tasks as in production; they become ready and run when chosen.
- **compile_link_checks:** n/a — no source construct; the configuration checks are SGO-LAB-002.
- **runtime_guards:** n/a — the scheduler is the guard (SGO-LAB-003).
- **tcb_assumptions:** The S-Go runtime is the only source of user-visible blocking and waking; Go runtime goroutines never make user-visible decisions.
- **positive_tests:** D03, D07, D10; fixture:lab-ready-set, fixture:lab-select-source-order, fixture:lab-gc, fixture:lab-runnable-time; `sgo/model/labmodel_test.go`, runtime `TestSGoLabStreams`.
- **negative_tests:** D05, D11; fixture:lab-quota.
- **implementation_coverage:** runtime `sgo_lab_sched.go`, `sgo_native_ops.go`; compiler `walk/sgo_lab.go`, `walk/select.go`.
- **executed_evidence:** `sgo/evidence/w08/README.md`.
- **supported_execution_configurations:** lab.

### SGO-LAB-004 — Lab streams, virtual time, and effects
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-LAB-003, SGO-AUTH-002, SGO-RANDOM-001, SGO-WAIT-001; D26, D44, D45.
- **grammar_and_types:** n/a — configuration clause.
- **preconditions:** n/a — configuration clause; it adds no operation with preconditions.
- **evaluation_and_value_semantics:** The manifest's seed derives the `sched`, `fault`, and `data` streams by SplitMix64; each is PCG-DXSM with unbiased bounded sampling. Time is virtual: it starts at 0 and advances, only when no task is ready, to the earliest pending `Sleep` deadline; timers with the same deadline fire in timer-creation order. `Clock.Since` is virtual time; `WallClock.Now` is the manifest's epoch (UTC milliseconds, default 0) plus virtual time; `Random` is unchanged (seeded by the program); `SimStore` starts empty; `Writer.WriteString` writes and is recorded.
- **ownership_and_lifecycle:** n/a — configuration clause; ownership rules are the production ones.
- **effect_and_capability_requirements:** The production grants.
- **concurrency_and_memory_order:** SGO-LAB-003.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** `Entropy.Bytes` ends the run with `UnsupportedLabEffect` (SGO-LAB-006): entropy is unpredictable by definition and has no seeded substitute.
- **cancellation_and_cleanup:** A cancelled `Sleep` returns the cancellation error and its timer is dropped.
- **compile_link_checks:** n/a — no source construct; the configuration checks are SGO-LAB-002.
- **runtime_guards:** n/a — the scheduler is the guard (SGO-LAB-003).
- **tcb_assumptions:** `runtime/sgo_lab.go` generator; checked against `math/rand/v2` vectors (`sgo/model`).
- **positive_tests:** D01, D06, D07; fixture:lab-random, fixture:lab-timer-ties, fixture:lab-wallclock, fixture:lab-runnable-time.
- **negative_tests:** D09; fixture:lab-entropy.
- **implementation_coverage:** runtime `sgo_lab.go`, `sgo_lab_run.go`, `sgo_effects.go`.
- **executed_evidence:** `sgo/evidence/w08/README.md`.
- **supported_execution_configurations:** lab.

### SGO-LAB-005 — Trace, replay, and sweeps
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-LAB-002, SGO-LAB-003; D46, D48, D50.
- **grammar_and_types:** `sgo lab run [-seed N] [-sched random|fifo] [-k N] [-quota N] [-epoch MS] [-trace FILE] [-timeout S] pkg`; `sgo lab replay [-timeout S] TRACE pkg`; `sgo lab sweep -seeds A..B [-sched] [-k] [-quota] [-epoch] [-expect FILE] [-save FILE] [-timeout S] pkg`.
- **preconditions:** Replay: the trace was written by `sgo lab` for the same build ID.
- **evaluation_and_value_semantics:** A trace is JSONL: a header with the manifest (format, build ID, seed, policy, K, quota, epoch), one canonical line per event (`choose`, `select`, `advance`, `fire`, `write`, `end`, `outcome`; `sgo/docs/w08-plan.md`), and a footer with the event count, exit status, and SHA-256 of the canonical lines. Canonical lines hold no host data; host data (wall time, hang) go to a sidecar. Replay takes every choice whose set has two or more elements from the trace, never from a generator, and compares the whole stream; a sweep runs each seed in a fresh process and reports the distinct outcomes.
- **ownership_and_lifecycle:** n/a — configuration clause; ownership rules are the production ones.
- **effect_and_capability_requirements:** n/a — no effect beyond the production grants (SGO-LAB-004).
- **concurrency_and_memory_order:** n/a — the run is one interleaving (SGO-LAB-003).
- **allowed_nondeterminism:** None in canonical lines; the sidecar is host data.
- **failure_and_partial_progress:** A trace whose footer does not match is refused (`SGO_E_LAB_TRACE`), and one of another build before running (`SGO_E_LAB_MANIFEST`); a replay that differs reports the first differing event and the last matching one (`SGO_E_LAB_DIVERGENCE`). A sweep fails if a run's status is not 0 or its stdout differs from `-expect`, and saves the first failing trace. A passing sweep shows those schedules pass, not every schedule.
- **cancellation_and_cleanup:** n/a — host tooling; a run ends as SGO-LAB-006 says.
- **compile_link_checks:** n/a — no source construct; the configuration checks are SGO-LAB-002.
- **runtime_guards:** n/a — the scheduler is the guard (SGO-LAB-003).
- **tcb_assumptions:** The driver is host code (SGO-TOOL-001).
- **positive_tests:** D02, D13; fixture:lab-repeat, fixture:lab-find-bug; runtime `TestSGoLabReplay`.
- **negative_tests:** D04, D05, D12; fixture:lab-manifest-mismatch, fixture:lab-impossible-choice, fixture:lab-trace-integrity.
- **implementation_coverage:** driver `cmd/sgo/lab.go`, `lab_cmds.go`; runtime `sgo_lab_boot.go` (manifest on fd 3, canonical lines on fd 4).
- **executed_evidence:** `sgo/evidence/w08/README.md`.
- **supported_execution_configurations:** lab.

### SGO-LAB-006 — Lab outcomes
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-PROC-002, SGO-ERR-002, SGO-CHAN-003, SGO-LAB-003, SGO-LAB-004; D47.
- **grammar_and_types:** n/a — configuration clause.
- **preconditions:** n/a — configuration clause; it adds no operation with preconditions.
- **evaluation_and_value_semantics:** Every production outcome is unchanged. In addition: `LabDeadlock` when no task is ready and no timer is pending while a task is live (in a lab run, in place of `Deadlock`); its record carries the wait graph, one `LabWait` chain entry per live task with what it waits in and its scope chain. `LabQuotaExhausted` when the quota is used up; `UnsupportedLabEffect` (SGO-LAB-004); `LabReplayDivergence` when a replay's recorded choice is not in the set. Each is one diagnostic record and status 2, and the trace's last event is the outcome.
- **ownership_and_lifecycle:** n/a — configuration clause; ownership rules are the production ones.
- **effect_and_capability_requirements:** n/a — no effect beyond the production grants (SGO-LAB-004).
- **concurrency_and_memory_order:** n/a — the run is one interleaving (SGO-LAB-003).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** A run that does not reach a choice point within the driver's watchdog is killed and reported as a host hang (`SGO_E_LAB_HANG`, sidecar), never as a lab outcome.
- **cancellation_and_cleanup:** The run ends where it stops: no task runs on, and no further deferred call runs.
- **compile_link_checks:** n/a — no source construct; the configuration checks are SGO-LAB-002.
- **runtime_guards:** The lab decides at every choice point.
- **tcb_assumptions:** n/a — the outcomes are decided by the scheduler of SGO-LAB-003.
- **positive_tests:** D08; fixture:lab-deadlock.
- **negative_tests:** D08, D09, D11; fixture:lab-deadlock, fixture:lab-entropy, fixture:lab-quota, fixture:lab-host-hang.
- **implementation_coverage:** runtime `sgo_lab_sched.go`, `sgo_lab_boot.go`.
- **executed_evidence:** `sgo/evidence/w08/README.md`.
- **supported_execution_configurations:** lab.

### SGO-LAB-007 — Lab standard input, environment, and interrupts
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-LAB-003, SGO-LAB-004, SGO-LAB-005; D178, D179, D180.
- **grammar_and_types:** `sgo lab run [-stdin FILE] [-env NAME=VALUE]… [-interrupt MS[,MS]] … pkg [-- ARGS]` and `sgo lab sweep [-stdin FILE] [-env NAME=VALUE]… [-interrupt MS[,MS]] … pkg [-- ARGS]`; the program reads stdin through `root.Stdin()` (an `*sgo.Input`) and observes an interrupt as the cancellation `sgo.IsInterrupted` recognizes.
- **preconditions:** FILE is readable by the driver; it is host input, not a grant of the program.
- **evaluation_and_value_semantics:** The driver reads FILE once and records its text in the manifest; the program's standard input is exactly that text followed by end of input, and without `-stdin` it is empty. Reads return the text in order with no host timing, so a read never waits on the host. Replay takes the text from the trace, never from a file, and a sweep gives every seed the same text. The environment `root.Getenv` reads is exactly the `-env` entries, recorded in the manifest and reused by replay, never the host's; the grants of the build still decide which names may be read. Each `-interrupt` time is a virtual timer of the run, recorded in the manifest, which fires as a deadline does: when no task is ready and virtual time reaches it (SGO-LAB-003). The first cancels the root as a SIGINT does in production; the second ends the run with status 130 and outcome `Interrupted`.
- **ownership_and_lifecycle:** n/a — the text is part of the run's manifest; it is not a resource of the program.
- **effect_and_capability_requirements:** n/a — the stdin capability is held by the root context as in production; the lab only replaces its source.
- **concurrency_and_memory_order:** n/a — the run is one interleaving (SGO-LAB-003).
- **allowed_nondeterminism:** None: the input, the environment, and the interrupt times are fixed by the manifest.
- **failure_and_partial_progress:** An unreadable FILE, an `-env` entry that is not NAME=VALUE, or more than two or decreasing `-interrupt` times, is refused before the run (`SGO_E_USAGE`). A replay of a trace whose text differs from a given `-stdin` is refused (`SGO_E_LAB_MANIFEST`).
- **cancellation_and_cleanup:** n/a — a read of lab input never blocks, so cancellation is observed at the program's next checkpoint.
- **compile_link_checks:** n/a — no source construct.
- **runtime_guards:** n/a — the input is supplied by the boot sequence (SGO-LAB-002).
- **tcb_assumptions:** The driver is host code (SGO-TOOL-001).
- **positive_tests:** D02, D13; `sgo/scripts/test-cli.py` (lab stdin is read and replayed).
- **negative_tests:** n/a — draft; refusal fixtures are added at the freeze.
- **implementation_coverage:** driver `cmd/sgo/lab.go`, `lab_cmds.go`; runtime `sgo_lab_boot.go`, `sgo_stdin.go`.
- **executed_evidence:** n/a — draft; recorded at the 0.19.0 freeze.
- **supported_execution_configurations:** lab.
