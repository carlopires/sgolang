# W08 evidence — deterministic lab and replay (spec 0.4.0)

| Field | Value |
|---|---|
| work_package_id | W08 |
| state | passed on darwin/arm64 (local, all commands below); linux/amd64 and linux/arm64: CI_PENDING |
| spec_version | 0.4.0 frozen (98 clauses: 94 frozen, 4 superseded); W08 adds SGO-LAB-002 to SGO-LAB-006 and supersedes SGO-LAB-001 (`sgo/spec/CHANGELOG.md`) |
| admission identity | registry content digest `ce0d9ead18412a3e`; toolchain identity `0b84789dcdb9938f` |
| decisions | D40–D49 approved 2026-11-02; D50 (source-order select arm indices) approved 2026-11-02; SPEC-LAB clauses approved 2026-11-02 |
| fork_commits | model `74ace96b9f`, `394d3b59a6`; runtime scheduler and hooks `d9af013b9a`, `212553de04`; D50 `e282b1718a`; lab build configuration, step counter, boot, replay `88a5fa366b`; driver `b86c4123ff`; linux fix `54fc8f99fd`; remaining choice points, lab effects, wait graph `a2b1e562f2`; spec 0.4.0 freeze and lab conformance lane `5730a3d30f`; S14 pairs and D13 fix `4b9fb8c7fa` (all commands below) |
| upstream_commit | `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` (go1.27.3) |
| host_and_toolchain | macOS 15.3 arm64; stage-0 by bootstrap go1.26.10; `sgo version`: `devel-0e6632bb7147 spec 0.4.0 admission 0b84789dcdb9938f upstream 2aae1000ea1d (host-built)` |
| artifact_digests (sha256 prefix) | `sgo` a9e8250415e01810, `compile` 576d15c9f6083cf3, `link` e2c777e493bf49d5, `asm` 73c23712465ac0f9, `sgo/TARGET_TCB.json` 1be7a62f21e42b86 |
| execution_configuration | production and lab (new). Identity line `sgo1 <spec> <admission> <ABI> <prod\|lab\|tcb> <contract>`; lab build ID `.../config=lab/...` |
| design | `sgo/docs/w08-plan.md` (canonical event protocol); runtime `src/runtime/sgo_lab.go` (state, PCG-DXSM, SplitMix64), `sgo_lab_sched.go` (decisions, baton, park commits, replay), `sgo_lab_run.go` (events, virtual Sleep, select), `sgo_lab_boot.go` (manifest fd 3, trace fd 4, records, wait graph); compiler `walk/sgo_lab.go` (step counter), `walk/select.go` (D50), `noder/sgo_identity.go`; linker `ld/sgoid.go`; driver `src/cmd/sgo/lab.go`, `lab_cmds.go` |
| next_dependency_ready_task | W09 |

## Lanes

- **model** — `sgo/model/lab*.go`: baton scheduler over scripted tasks (spawn, join, virtual Sleep, Checkpoint, write, cancel, error, send, receive, close, select); a choice point before every operation and every K steps; quota; `random`/`fifo`; PCG-DXSM with SplitMix64 streams checked against `math/rand/v2`. Ten scenarios with streams for seeds 0, 1, 2, 7, 42 are pinned in `sgo/model/testdata/lab-streams.json` and copied to the runtime.
- **runtime** — `TestSGoLabStreams` interprets every model scenario with runtime operations: all 50 streams and outcomes equal the model's; changing one pinned pick, or dropping the select order array, fails. `TestSGoLabReplay` replays all 50 from their picks with a different seed and checks a divergence. With the lab off the S-Go runtime tests and `-race` lane are unchanged.
- **compiler and linker** — `-sgolab` step instrumentation without inlining; configuration in the identity (lab and production objects never mix; trusted code is `tcb`); every S-Go `select` through `sgoselectgo` with source indices (`test-native-lowering.py`).
- **lab (driver)** — `sgo lab run`, `replay`, `sweep` with trace header, canonical lines, footer digest, host sidecar, and watchdog: `test-lab.py`, and 26 lab conformance cases covering D01–D13 and S14 (including 8 production cases run under the lab with identical outcomes, records, and output).

## Commands (local, darwin/arm64, at `4b9fb8c7fa`)

| Command | Exit | Result |
|---|---|---|
| `test-model.sh` | 0 | lab model tests, pinned streams, PCG vectors; earlier lanes unchanged |
| `runtime-lane.sh -run 'TestSGo\|TestAPI\|TestRandom\|TestMulAdd' runtime sgo` | 0 | includes `TestSGoLabStreams` (50/50) and `TestSGoLabReplay` |
| `runtime-lane.sh -race -run TestSGo runtime` | 0 | no race |
| `conformance.py run --sgo pkg/tool/darwin_arm64/sgo` | 0 | 306/306 (166 run, 114 reject, 26 lab); `selftest` ok |
| `test-lab.py` | 0 | run, determinism, sidecar, replay, divergence, refusals, LabDeadlock with wait graph, LabQuotaExhausted, UnsupportedLabEffect, WallClock epoch, Event and limiter sweep, D13, production binary ignores a manifest |
| `conformance-compile.py`, `test-identity.py`, `test-tcb.py`, `test-flags.py`, `test-mutations.py`, `test-native-lowering.py`, `test-fmt.py`, `test-x10.py` | 0 | no regression |
| `spec-tool.py check`, `inventory-check.py`, `policy-report.py --check`, `generated-twin-check.py`, `adoption-check.py`, `source-map-audit.py` | 0 | 98 clauses (94 frozen, 4 superseded); 556 surfaces mapped |

## Findings during W08

- Handing the baton before a task had parked could ready a goroutine that was not parked yet: every S-Go park commit now reports to the lab (`sgoLabCommit`), which aborts the park of a task already chosen.
- A run ending while a task held a wait's locks would have stopped under them: the commit aborts the park and the task stops on its own stack (`sgoLabAfterPark`).
- The select event first named arms by the runtime's case order (sends first, receives reversed): D50 fixes source order; walk sends every S-Go select through `sgoselectgo`.
- Step counts would have depended on inlining: lab builds do not inline.
- linux only: a stack buffer passed to `read` escapes (`sgo-runtime` run 41315970127), fixed in `54fc8f99fd`.

## Open

- The `supported_execution_configurations` field of the 0.3.0 clauses still reads "lab denied until W08"; SGO-LAB-002 supersedes it without amending those clauses (CHANGELOG 0.4.0). A later amendment may update the field.
- Lab binaries exit as soon as the run ends; a Go runtime throw in a lab binary (status 4) writes no trace (it runs on the system stack).
