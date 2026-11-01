# W08 plan — deterministic lab and replay (spec 0.4.0)

Approved decisions: D40–D49 (`sgo/SGO_DECISIONS.md`). Model first (D30); small verified commits, each with the full check cycle (spec-tool, tcb-manifest, inventory, policy report, build-stage0, conformance, conformance-compile, test-identity/flags/mutations/tcb/x10/fmt/native-lowering, runtime lane, model).

## The canonical event protocol (shared by model and runtime)

One stream per run. The model and the runtime emit the **same** canonical events for the same choices, so the runtime is checked against the model event by event, not only by outcome.

| Event | Fields | Emitted when |
|---|---|---|
| `choose` | `ready` (task IDs, ascending), `pick` | before each step: the scheduler hands the baton to `pick` |
| `select` | `task`, `arms` (indices of the arms that can proceed, ascending), `arm` (`-1`: default) | a `select` that can proceed now |
| `advance` | `to` (virtual ms) | no task is ready and a timer is pending: time jumps to the earliest deadline |
| `fire` | `task`, `timer` (timer ID) | each timer at that deadline, in `(deadline, timer ID)` order |
| `write` | `task`, `text` | `Writer.WriteString` |
| `end` | `task`, `outcome` | a task ends (`Succeeded`, `Errored`, `Failed:<kind>`, `Cancelled`) |
| `outcome` | `status`, `kind` | the run ends: exit status, or `LabDeadlock`, `LabQuotaExhausted`, `UnsupportedLabEffect` |

Every operation is one step of the running task, and a choice point follows each step (D43): the ready set is recomputed and the next task is chosen from the `sched` stream (or the trace).

## Steps

1. **Model.** `sgo/model/labmodel.go`, `labstep.go`, `labrun.go`, `labexplore.go`:
   - the lab scheduler over task scripts (work steps with the yield period K and the quota, sleep on virtual time, channel operations and select, write, checkpoint, nursery cancel, join);
   - PCG-DXSM streams derived by SplitMix64 (D44), with reference vectors;
   - run from a seed, producing the canonical stream and outcome;
   - replay from a stream with divergence detection (first mismatch, expected and actual ready sets);
   - bounded enumeration of all choice sequences (the outcome sets);
   - pinned golden streams for fixed seeds.
2. **Runtime.**
   - `src/runtime/sgo_lab.go`: the baton, the ready set, choice points hooked into every S-Go blocking and wake-up point;
   - virtual timers;
   - the step counter, yield period and quota;
   - a preallocated trace buffer;
   - the LabDeadlock wait graph and the quota and unsupported-effect outcomes;
   - the manifest (read only by lab binaries).

   Runtime-lane tests drive the same scenarios as the model through hooks and compare the canonical streams for the same seeds and replayed choices.
3. **Compiler and linker.**
   - Lab instrumentation (a step counter call at loop back-edges and function entries), only for lab builds.
   - Execution configuration in the build identity: a production link refuses lab objects and vice versa.
4. **Driver.**
   - `sgo lab run`, `sgo lab replay`, `sgo lab sweep`;
   - the manifest, JSONL writer and reader with the footer digest, the sidecar, and the watchdog.
5. **Spec 0.4.0.** SPEC-LAB clauses superseding SGO-LAB-001; CHANGELOG; every module declares `sgo 0.4.0`.
6. **Conformance.**
   - A lab lane in `conformance.py`, where a case declares a seed or a trace and the expected stream or outcome.
   - D01–D13, S14 (paired production and lab runs of the same programs).
7. **Evidence and ledgers.** `sgo/evidence/w08/README.md`, `SGO_TASKS.json`, `SGO_TEST_MATRIX.json`, policy report, TCB manifest.
