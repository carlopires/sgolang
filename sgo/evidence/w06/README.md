# W06 evidence — modeled and typed permit channels

| Field | Value |
|---|---|
| work_package_id | W06 |
| state | passed on darwin/arm64 (local, all commands below) and linux/amd64, linux/arm64: `sgo-runtime` run 41247313905 at `43d8f028d4`, the last runtime change (S-Go runtime tests with -cpu=1,4, race detector, upstream runtime -short); `sgo-spec` run 41247962195 and `sgo-w00` run 41247962181 at `1158afe4ad` (builds, compile-level and driver conformance including the 16 typed-channel cases, identity, fmt, TCB, X10, flag and mutation gates). `sgo-w00` run 41247313847 at `43d8f028d4` failed on the compile-level conformance step, fixed in `1158afe4ad` (below) |
| spec_version | 0.1.0 frozen (84 clauses) plus draft SGO-PERMIT-002 (typed channels, D28/D29), which W06 implements; it freezes with spec 0.2.0 |
| admission identity | registry content digest `f024ab9487ade5c1`; toolchain identity (SHA-256 of `sgo/SGO_SPEC_ADMISSION.json`) `db7dd52da7ca298f` |
| fork_commit | `43d8f028d4` (implementation), `1158afe4ad` (`conformance-compile.py` fix, below) |
| upstream_commit | `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` (go1.27.3) |
| host_and_toolchain | macOS 15.3 arm64; stage-0 by bootstrap go1.26.10; `sgo version`: `devel-86267bca0e5f spec 0.1.0 admission db7dd52da7ca298f upstream 2aae1000ea1d (host-built)` |
| artifact_digests (sha256 prefix) | `sgo` 8c279566ce59d4b2, `compile` 8c7c28f39234b139, `link` 761092a7aff6922c, `asm` 73c23712465ac0f9, `sgo/TARGET_TCB.json` 1554ada127310223 |
| execution_configuration | production (lab denied until W08); targets darwin/arm64, linux/amd64, linux/arm64 |
| design | `sgo/docs/runtime-design.md` §5 (lock order), §10 (state, linearization points, waiting and cancellation, obligations, guards, selection decisions; TCB-P12) |
| next_dependency_ready_task | W05 (capabilities and readiness, depends on W04); W07 (native channels and select) needs W03, W05, W06 |

## Lanes

- **model** — `sgo/model/chanmodel.go`: a pure state machine of reserve/commit/abort/receive/close for buffered and zero-capacity channels, with sender scripts for abort, cancellation racing every step, task end with an unresolved permit, and a second alias's commit. `ExploreChan` enumerates every interleaving and checks after each step: queued + reserved ≤ capacity, receive order equals commit order, no value received twice or without a commit, no value committed twice; at the end, every committed value is received or queued, every permit is terminal, and the diagnostic count equals the abandoned and misused permits. Ten reference scenarios' outcome sets are pinned in `sgo/model/testdata/chan-outcomes.json`.
- **runtime** — `src/runtime/sgo_chan.go` checked against the model: every outcome of every run must be in the model's set for the same scenario (`src/runtime/testdata/sgo-chan-outcomes.json`, a copy that `TestChanGoldenOutcomes` keeps identical), with and without fault injection; plus directed, lifetime, and conservation tests.
- **typed_channel** — S-Go programs using `sgo.Channel[T]` through the driver: 16 conformance fixtures (12 run, 4 reject). The `strict_native` lane of H01–H12 belongs to W07.

## Commands (local, at `43d8f028d4` plus the closure commit's script fix; logs and exit codes in `logs/`)

| Command | Exit | Result |
|---|---|---|
| `test-model.sh` | 0 | model scenarios (largest 71,549 states / 24,480 complete histories), golden outcomes, seeded-bug detection |
| `runtime-lane.sh -v -count=1 -run TestSGoChan runtime` | 0 | 7 tests; model outcomes observed per scenario, 300 runs each: 1/1, 9/9, 3/3, 3/3, 1/1, 1/1, 23/28, 6/6, 14/15, 1/1; with fault injection, 150 runs each: the same except 20/28 for `mixed-cancel-end-alias` (no run produced an outcome outside the model's set) |
| `runtime-lane.sh -count=5 -cpu=1,4,16 -run 'TestSGo\|TestAPI' runtime sgo` | 0 | |
| `runtime-lane.sh -race -count=3 -run TestSGo runtime` | 0 | |
| `runtime-lane.sh -short runtime` (full upstream runtime suite) | 0 | |
| `conformance.py validate` | 0 | 210 cases (113 run, 97 reject) |
| `conformance.py run --sgo … --pending sgo/testdata/conformance-pending.json` | 0 | 210 passed; the pending list is empty |
| the 16 `permit-*` cases, 25 times | 0 | identical every time |
| `conformance-compile.py` | 0 | 83/83 eligible rejections; 109 run cases clean (after the fix below) |
| `spec-tool.py check`, `inventory-check.py`, `tcb-manifest.py`, `generated-twin-check.py` | 0 | 89 clauses (84 frozen, 5 draft); 546 surfaces; 31 packages, 1031 files, 15 changed from upstream |
| `test-identity.py`, `test-flags.py`, `test-mutations.py`, `test-tcb.py`, `test-x10.py`, `test-fmt.py` | 0 | (run before the implementation commit, unchanged since) |

`conformance-compile.py` first failed (`exit=1`, recorded in `logs/exit-codes.txt` before the fix): it compiled only `src/sgo/sgo.go` as package sgo, so the new `chan.go` was missing and the 12 new run cases reported `undefined: sgo.NewChannel`. It now compiles every non-test file of the package.

Passed/failed/skipped/unrun: all passed; nothing skipped or unrun.

## Acceptance cases

| Case | Result | Evidence |
|---|---|---|
| H01 Buffered capacity invariant | passed | model invariant over all interleavings; `sgoChanCheck` after every transition; TestSGoChanFIFO (100 values, capacity 3); fixture `permit-buffered-fifo` |
| H02 Zero-capacity rendezvous | passed | model scenarios `rendezvous-*`; runtime matches receivers (Waiting → Matched) before a reservation exists, so no unmatched publication; `sgoChanCheck` asserts a C = 0 channel queues nothing; fixture `permit-rendezvous` |
| H03 Reserve then abort | passed | model `buffered-abort`; fixture `permit-reserve-abort` (double abort has no effect, the next Reserve on capacity 1 succeeds, nothing published) |
| H04 Permit aliases | passed | model `SendAliasDouble`; runtime scenario with a recovered violation and diagnostics count; fixture `permit-double-commit` (SpawnContractViolation, no second effect) |
| H05 Cross-task permit misuse | passed | TestSGoChanTaskAffinity (foreign commit rejected, permit unchanged); fixture `permit-foreign-task` |
| H06 Cancel before commit | passed | model `SendCancelRace` (before reserve: no permit; after reserve: aborted); fixture `permit-cancel-before-commit` |
| H07 Cancel after commit | passed | model (commit not retroactive); fixture `permit-cancel-after-commit` |
| H08 Close versus commit | passed | model: exactly the three outcomes of plan §7.3; fixture `permit-close-versus-commit` |
| H09 Panic with permit | passed | TestSGoChanPanicWithPermit (reservation released before the outcome, capacity reusable); fixture `permit-panic-with-permit` (record chain contains `PermitAbandoned`) |
| H10 Owner exits with permit | passed | TestSGoChanTaskAffinity (body-exit abort before the join); fixture `permit-owner-exits` (the child's Reserve proceeds) |
| H11 Pointer value lifetime | passed | TestSGoChanValueLifetime (queued pointer values survive 5 GCs, consumed ones are collected; the dequeued slot is cleared); fixture `permit-error-values` |
| H12 Conservation under stress | passed | model conservation invariant; TestSGoChanConservation (unique IDs, 6 senders × 20, 3 receivers, close at a random point, aborts; capacities 0, 1, 3; 20 rounds) |

## Gate (plan W06)

- No duplicated or lost published values: model invariants; TestSGoChanConservation; FIFO and fixtures.
- No capacity leak: `sgoChanCheck`; every runtime scenario ends quiescent (no reservation, no parked waiter).
- No unresolved permit after task completion: resolution at body exit, task end, and main return; TestSGoChanPanicWithPermit, `permit-owner-exits`, `permit-panic-with-permit`.
- No successful closed-channel publication contrary to the defined ordering: model `buffered-close-race`, `permit-close-versus-commit`.
- Implementation and model agree on all bounded reference histories: TestSGoChanModel and TestSGoChanModelFaults (yields injected at all 8 points before and after each terminal transition, each point reached).

## Decisions and open items

- For C = 0, a receive takes effect when a reservation is matched to it; a matched receiver whose task is cancelled completes when that permit resolves (bounded by the sender's task). Recorded in SGO-PERMIT-002 (draft); unmatched waiters return cancellation promptly.
- SGO-PERMIT-002 remains draft until spec 0.2.0 is frozen (with the AUTH/READY drafts, the simnet clause, and the remaining TCB entries).
- Receive permits and native `chan`/`select` are W07.
