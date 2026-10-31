# W04 evidence — cancellation, waiting, and budgets

| Field | Value |
|---|---|
| work_package_id | W04 |
| state | passed on darwin/arm64 (local, all commands below) and linux/amd64, linux/arm64 (CI `sgo-runtime` run 41217543649 at `1b2779e5cb`, the last runtime change: S-Go runtime tests ×3 with -cpu=1,4, race detector, upstream runtime -short; `sgo-w00` run 41217878447 at `ad60fbe22d`) |
| spec_version | 0.1.0 (84 frozen clauses; no clause changed in W04) |
| spec_digest / admission_digest | `5558fdec248c016e` |
| fork_commit | `ad60fbe22d` |
| upstream_commit | `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` (go1.27.3) |
| host_and_toolchain | macOS 15.3 arm64; stage-0 by bootstrap go1.26.10; `sgo version`: `devel-3aab8920728b spec 0.1.0 admission 5558fdec248c016e upstream 2aae1000ea1d (host-built)` |
| artifact_digests (sha256 prefix) | `sgo` a4cb92235bf9cb2d, `compile` fb4187ea6de19cee, `link` 761092a7aff6922c, `asm` 73c23712465ac0f9, `sgo/TARGET_TCB.json` a2f431774779541f |
| execution_configuration | production (lab denied until W08); targets darwin/arm64, linux/amd64, linux/arm64; drain budget per binary (`-drain-budget=<ms>`, recorded in the build ID as `drain=<ms\|none>`) |
| host_target_build_identity | `sgo1/spec=0.1.0/admission=5558fdec248c016e/drain=<ms\|none>/target=<os>-<arch>/toolchain=<id>/upstream=<commit>` |
| benchmark_configuration | none |
| design | `sgo/docs/runtime-design.md` §5 (lock order), §9 (cancellation, Sleep, drain budget) |
| next_dependency_ready_task | W05 (capabilities and readiness) and W06 (modeled and typed permit channels), both depending only on W04 |

## Commands (local, at `ad60fbe22d`; logs and exit codes in `logs/`)

| Command | Exit | Result |
|---|---|---|
| `spec-tool.py check`, `inventory-check.py`, `tcb-manifest.py` | 0 | 84 frozen clauses; 546 surfaces; 31 packages, 1029 files, 13 changed from upstream |
| `conformance.py validate` | 0 | 194 cases (101 run, 93 reject) |
| `conformance.py run --sgo … --pending sgo/testdata/conformance-pending.json` | 0 | 194 passed; the pending list is empty |
| `conformance-compile.py` | 0 | 79/79 eligible rejections; 97 run cases clean |
| `test-identity.py`, `test-tcb.py`, `test-x10.py`, `test-flags.py`, `test-mutations.py`, `test-fmt.py` | 0 | build ID now includes the drain budget; fmt: 201 files, 97 run cases rerun unchanged |
| `runtime-lane.sh -count=5 -cpu=1,4,16 -run 'TestSGo\|TestAPI' runtime sgo` | 0 | |
| `runtime-lane.sh -race -count=3 -run TestSGo runtime` | 0 | |
| `runtime-lane.sh -v -run 'TestSGoCancel\|TestSGoTimer\|TestSGoSleep\|TestSGoOwner\|TestSGoGoexit' runtime` | 0 | 12 tests; timer-vs-cancellation race won by the timer 428 times, by cancellation 2572 times |
| `runtime-lane.sh -short runtime` (full upstream runtime suite) | 0 | |

Additionally, every timing-sensitive conformance case (39) was run 25 times and every new W04 case 30 times: all identical. That repetition found one W03 fixture whose expectation predated cancellation (`err-chain-nested-child`, fixed in `02fa8b1958`); CI runs of `b302963f3b` failed for that reason and for two runtime-test issues fixed in `1b2779e5cb` (see below).

Passed/failed/skipped/unrun: all passed; nothing skipped or unrun.

## Acceptance cases

"Subset 0" marks where spec 0.1.0 denies the construct the case exercises (approved policy): the case passes through its rejection plus the parts subset 0 can express.

| Case | Evidence | Result |
|---|---|---|
| P01 single escaping child panic | def-child-panic-sibling-cleanup, def-child-panic-cancels-siblings, err-chain-* | passed (sibling cancelled, its defers run, original payload primary) |
| P02 recovered child panic | def-recover | passed for subset 0 (`recover` denied; no user recovery exists) |
| P03 multiple escaping panics | nur-two-failures-lowest-id, nur-primary-not-by-time, err-chain-nested-child, TestSGoOutcomes | passed |
| P04 owner panic precedence | def-unwind-cancels-children, err-chain-owner-and-children | passed (finalizer failures: subset 0, finalizers denied) |
| P05 nil and replacement panics | def-panic-in-defer, def-panic-int, def-recover | passed for subset 0 |
| P06 task exit, raw Goexit | proc-raw-goexit, TestSGoGoexitIsDefect | passed for subset 0 (`sgo.ExitTask` and the test runner are not in spec 0.1.0) |
| P07 owner blocked during child failure | cancel-owner-woken-by-child-failure, TestSGoOwnerWokenByChildFailure | passed |
| P08 recovery after nursery unwind | def-unwind-cancels-children, def-after-nursery, def-recover | passed for subset 0 (nurseries drain before defers; `recover` denied) |
| C01 cancel before registration | TestSGoCancelBeforeRegistration, spawn-cancelling-no-child | passed |
| C02 cancel before park window | TestSGoCancelBeforePark (2000 races), cancel-wakes-sleep | passed |
| C03 cancel versus readiness | TestSGoCancelVersusTimer (3000 races, both sides won) | passed for subset 0 (readiness denied, SGO-READY-001; timer expiry is the only readiness) |
| C04 repeated cancellation | TestSGoCancelFirstCauseSticks, cancel-idempotent | passed |
| C05 nested inherited cancellation | TestSGoCancelNestedInherited, TestSGoCancelPropagatesNested, cancel-consumed-at-boundary, nur-inner-cancel-not-outer, cancel-propagates-nested, cancel-error-messages | passed |
| C06 cancelled admission | TestSGoCancelledAdmission, spawn-cancelling-no-child | passed |
| C07 cleanup shielding and quota | def-scope-finalizer; R5 refuses spawns into cancelled scopes (C06) | passed for subset 0 (cleanup contexts and quotas are not in spec 0.1.0) |
| C08 timer lifecycle | TestSGoTimerLifecycle, TestSGoSleepDuration | passed |
| C09 non-cooperative task | drain-noncooperative-budget (exit 3, DrainBudgetExceeded), drain-noncooperative-no-budget (watchdog), drain-cooperative-child, drain-budget-env-ignored | passed |
| C10 sticky recovered cancellation | TestSGoCancelFirstCauseSticks, TestSGoCancelNestedInherited | passed for subset 0 (`recover` denied; cancellation stays effective) |
| C11 primary failure during cleanup | def-unwind-cancels-children, err-chain-owner-and-children | passed |
| S07 checked nested-scope capability | cancel-unrelated-task, TestSGoCancelMembership, spawn-after-close, TestSGoInvalidHandles (W02) | passed (cleanup no-spawn rule: finalizers denied in subset 0) |
| S08 spawn callback failures | nur-child-error-exit-1, def-child-panic-sibling-cleanup, err-chain-child-errors, spawn-nil-callback | passed |

## Changed invariants

- New TCB file `runtime/sgo_cancel.go` (primitives TCB-P04, TCB-P05); hooks in `sgo_scope.go` (ChildFailed on publish, R5 for scopes cancelling in effect, drain timer stop), `sgo_nursery.go` (Unwind before the unwinding join), `sgo_main.go` (drain budget init), `sgo_api.go` (package `sgo` entry points), `panic.go` (Goexit guard limited to S-Go programs).
- Lock graph: scope → stdout (drain decision only); sleep lock alone; no static rank needed (design §5).
- Linker: `-sgodrainbudget` is the only way to set the budget; `-X` remains refused. Driver: `-drain-budget` for `build` and `run`; build ID gains `drain=`.
- A failing child now cancels its scope, so code that spawns after a sibling failed gets no child (R5). Two earlier tests relied on the opposite and were corrected: `TestSGoOutcomes` (W02) and the `err-chain-nested-child` fixture (W03).

## Known limitations

- The runtime test binary built by the upstream toolchain held two itabs for `(*sgoCancelErr, error)`; `sgoIsCancelled` therefore compares type descriptors (through `any`). The root cause in the upstream toolchain was not investigated; S-Go binaries were not affected.
- `cancel-propagates-nested` orders the cancellation after the grandchild's spawn with a 200 ms sleep, and `err-chain-nested-child` orders the sibling's failure with one; exact orderings are covered by runtime tests with synchronization.
- Cleanup contexts with quotas, scope finalizers, readiness handshakes, permits, `recover`, and `sgo.ExitTask` are denied by spec 0.1.0; the cases that exercise them pass their subset-0 parts only.
