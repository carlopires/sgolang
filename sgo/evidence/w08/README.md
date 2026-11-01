# W08 evidence — deterministic lab and replay (in progress)

| Field | Value |
|---|---|
| work_package_id | W08 |
| state | **in progress**: model, runtime scheduler, compiler and linker lab configuration, and the `sgo lab` driver are implemented and pass locally on darwin/arm64; Linux CI pending for `54fc8f99fd` (the `sgo-runtime` run 41315970127 at `88a5fa366b` failed on linux only: a stack buffer passed to `read` escapes there, fixed in `54fc8f99fd`). Spec 0.4.0 (SPEC-LAB) not yet drafted; open items below |
| spec_version | 0.3.0 (SPEC-LAB still denied; lab builds use the approved decisions D40–D50 pending the 0.4.0 clauses) |
| decisions | D40–D49 approved 2026-11-02; D50 (source-order select arm indices) approved 2026-11-02 |
| fork_commits | `74ace96b9f`, `394d3b59a6` (model); `d9af013b9a`, `212553de04` (runtime scheduler and hooks); `e282b1718a` (D50); `88a5fa366b` (lab build configuration, step counter, boot, replay); `b86c4123ff` (driver); `54fc8f99fd` (linux fix) |
| design | `sgo/docs/w08-plan.md` (canonical event protocol); runtime `src/runtime/sgo_lab.go`, `sgo_lab_sched.go`, `sgo_lab_run.go`, `sgo_lab_boot.go`; compiler `walk/sgo_lab.go`, `walk/select.go`; linker `ld/sgoid.go`; driver `src/cmd/sgo/lab.go`, `lab_cmds.go` |
| execution_configuration | lab (new): identity line `sgo1 <spec> <admission> <ABI> <prod\|lab\|tcb> <contract>`; build ID `.../config=lab/...` |

## What exists

- **Model** (`sgo/model/lab*.go`): baton scheduler over scripted tasks (spawn, join, sleep on virtual time, checkpoint, write, cancel, error, send, receive, close, select), choice before every operation and every K steps, step quota, `random`/`fifo` policies, PCG-DXSM with SplitMix64 streams. Ten scenarios with streams for seeds 0, 1, 2, 7, 42 pinned in `sgo/model/testdata/lab-streams.json` and its runtime copy.
- **Runtime**: one task holds the baton; every S-Go park commit reports to the lab (`sgoLabCommit`), so a task woken and chosen before it parks never parks; wakes go through `sgoWake`; virtual timers; select choice from the `sched` stream with source-order arms (D50); end-of-run records `LabDeadlock`, `LabQuotaExhausted`, `LabReplayDivergence`; replay takes every choice of a set with two or more elements from the trace.
- **Compiler and linker**: `-sgolab` adds a step at every function entry and loop iteration of S-Go code and disables inlining (step counts independent of inlining); lab, production, and trusted (`tcb`) objects are distinguished in the identity and never mix; `-sgolab` at link time checks every S-Go package and marks the runtime. Every S-Go select goes through `sgoselectgo` with source indices.
- **Driver**: `sgo lab run`, `replay`, `sweep` (header, canonical lines, footer digest; host sidecar; watchdog).

## Commands (local, darwin/arm64, at `b86c4123ff` + `54fc8f99fd`)

| Command | Exit | Result |
|---|---|---|
| `test-model.sh` | 0 | lab model tests, pinned streams, PCG vectors |
| `runtime-lane.sh -v -run TestSGoLab runtime` | 0 | `TestSGoLabStreams`: 50 runs (10 scenarios × 5 seeds) equal the model's streams and outcomes; mutating one pinned pick, or dropping the select order array, fails. `TestSGoLabReplay`: all 50 replay from their picks; an impossible pick is `LabReplayDivergence` |
| `runtime-lane.sh -run 'TestSGo\|TestAPI\|TestRandom\|TestMulAdd' runtime sgo`, and `-race -run TestSGo runtime` | 0 | no regression with the lab off |
| `test-lab.py` | 0 | run, determinism, sidecar, replay, divergence, refusal of altered and other-build traces, LabDeadlock, LabQuotaExhausted, D13 ordering bug found by a sweep and replayed, production binary ignores a manifest on fd 3 |
| `test-native-lowering.py` | 0 | no S-Go select bypasses `sgoselectgo` |
| `conformance.py run`, `conformance-compile.py`, `test-identity.py`, `test-flags.py`, `test-tcb.py`, `test-mutations.py`, spec/inventory/policy/twin/adoption/source-map gates | 0 | 280/280; no regression |

## Open (next)

1. Choice points still missing for `Event`, `CapacityLimiter`, readiness `Signal`/`Start`, and effect operations (D43); `Entropy` → `UnsupportedLabEffect`, `WallClock` = manifest epoch + virtual time (D45).
2. `LabDeadlock` record with the wait graph (D47).
3. Lab conformance lane (D01–D13, S14) and these cases as fixtures.
4. Spec 0.4.0: SPEC-LAB clauses superseding SGO-LAB-001 (proposed to the user before freezing); `sgoLabStep` then `internal` in `INVENTORY.map`.
5. Linux evidence for the lab lanes (CI), then ledgers and closure.
