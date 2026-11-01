# W07 evidence — native channels, select, structured synchronization (spec 0.3.0)

| Field | Value |
|---|---|
| work_package_id | W07 |
| state | PENDING: CI run IDs |
| spec_version | 0.3.0 frozen (93 clauses: 90 frozen, 3 superseded); W07 adds SGO-CHAN-002, SGO-CHAN-003, SGO-CHAN-004, SGO-SYNC-002 and amends SGO-TYPE-001, SGO-TYPE-005, SGO-CAP-001, SGO-DEF-003, SGO-IMP-001, SGO-AUTH-003, SGO-COMP-001 (`sgo/spec/CHANGELOG.md`) |
| admission identity | registry content digest `10a823feff7a1669`; toolchain identity `83fee1764c753ec5` |
| fork_commit | `209abde18b` (all commands below), plus the closure commit (fault-injected native outcome runs and this record) |
| upstream_commit | `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` (go1.27.3) |
| host_and_toolchain | macOS 15.3 arm64; stage-0 by bootstrap go1.26.10; `sgo version`: `devel-6a04cea16ca1 spec 0.3.0 admission 83fee1764c753ec5 upstream 2aae1000ea1d (host-built)` |
| artifact_digests (sha256 prefix) | `sgo` 7be8afbef19032f7, `compile` 5d5b0941c658954f, `link` b8670ea32d4fcc64, `asm` 73c23712465ac0f9, `sgo/TARGET_TCB.json` 4f9586fdf4dfb4cc |
| execution_configuration | production (lab denied until W08); targets darwin/arm64, linux/amd64, linux/arm64 |
| design | `sgo/docs/w07-plan.md`; runtime: `src/runtime/sgo_native.go` (cancellation unwind, single decision `sgoSel`, rendezvous race edges), `sgo_native_ops.go` (operations, select), `sgo_sync.go` (Event, CapacityLimiter); TCB-P17, TCB-P18, TCB-P19 |
| next_dependency_ready_task | W08 (lab scheduler: records and replays the select choice and every interleaving) |

## Lanes

- **model** — `sgo/model/nativemodel.go`, `nativestep.go`, `nativeexplore.go`: native send (one step), receive, close, `select` with `default`, cancellation that unwinds a waiting operation (D33), `Event`, and `CapacityLimiter` with token obligations. After every step: buffer bounds, receive order equals commit order, no duplicate or fabricated value, at most one select arm per select, limiter occupancy equals held tokens and is within its size, no Wait returns before Set; at the end, no committed value lost and no token leaked. Ten reference scenarios are pinned in `sgo/model/testdata/native-outcomes.json`; seeded bad states show each invariant fires.
- **runtime (strict_native)** — `TestSGoNativeOutcomes` runs every model scenario 300 times with random yields and cancellation times, and 150 more times with yields injected at the fault points; every outcome must be in the model's set. Directed tests: select stress, cancellation unwind, empty select, GC and stack movement, Event, CapacityLimiter, and the race gate (`TestSGoNativeRace`, under `-race`).
- **compiler** — admission and type rules (`sgoadmit/admit_expr.go`, `types2/sgo.go`, `types2/sgo_flow.go`); lowering to the S-Go entry points in `typecheck.LookupRuntime`, which refuses Go's channel entry points for S-Go packages; `test-native-lowering.py` disassembles a two-package module using every admitted channel form.
- **sgo_production** — S-Go programs through the driver: 40 new conformance cases (26 channel and select, 12 Event and limiter, 2 generator hand-off) and 3 reclassified ones (`chan-select`, `ctl-select`, `chan-type`); `tcb-module-named-sgo` now reports `SGO_E_USAGE`.

## Commands (local, at `209abde18b`; logs in `logs/`)

| Command | Exit | Result |
|---|---|---|
| `test-model.sh` | 0 | model scenarios, pinned outcome sets (W06 and W07), seeded-bug detection |
| `runtime-lane.sh -v -count=1 -run 'TestSGoNative\|TestSGoEvent\|TestSGoLimiter' runtime` | 0 | 10 scenarios × 300 runs: every model outcome observed in each (1/1, 2/2, 3/3, 2/2, 2/2, 1/1, 2/2, 1/1, 2/2, 1/1); directed tests pass |
| `runtime-lane.sh -v -count=1 -run TestSGoNativeOutcomes runtime` (closure commit) | 0 | the same, plus 150 fault-injected runs per scenario: every model outcome observed, none outside the set (`logs/runtime-native-faults.log`) |
| `runtime-lane.sh -count=3 -cpu=1,4,16 -run 'TestSGo\|TestAPI' runtime sgo` | 0 | |
| `runtime-lane.sh -race -count=2 -run TestSGo runtime` | 0 | includes the race gate: 7 scenarios × 10 child processes |
| `runtime-lane.sh -short -count=1 runtime` (full upstream runtime suite) | 0 | |
| `conformance.py validate` | 0 | 280 cases (166 run, 114 reject); every clause-referenced fixture exists |
| `conformance.py run --sgo … --pending sgo/testdata/conformance-pending.json` | 0 | 280 passed; the pending list is empty |
| the same, 10 more times | 0 | 280 passed every time (`logs/repeat-conformance.log`) |
| `conformance-compile.py` | 0 | every eligible rejection; run cases clean |
| `test-native-lowering.py` | 0 | all 9 S-Go channel entry points called, no Go channel entry point; the self-test sees Go's entry points in a stock Go program |
| `test-identity.py`, `test-flags.py`, `test-mutations.py`, `test-tcb.py`, `test-x10.py`, `test-fmt.py` | 0 | |
| `spec-tool.py check`, `inventory-check.py`, `tcb-manifest.py`, `generated-twin-check.py`, `adoption-check.py`, `policy-report.py --check` | 0 | 93 clauses; 555 surfaces; 31 packages, 1041 files, 25 changed from upstream; 28 denied routes with cases |

Passed/failed/skipped/unrun: all passed; nothing skipped or unrun. No scenario of the native outcome set admits histories that block forever, so none was left to the model lane alone.

Problems found and fixed during W07, before the commits above:

- The first CI run of the closure commit (`sgo-runtime` 41302656249) failed under `-race`: the fault-injected pass added to TestSGoNativeOutcomes in that commit shared one `math/rand.Rand` across concurrently running tasks (a race in the test, reported by the detector). The callback now uses the package-level generator; `runtime-lane.sh -race -count=2 -run TestSGo runtime` passes locally.

- The cancellation-unwind test showed an owner could finish with an open nursery when the nursery's frame was not recorded; the compiler's nursery entry records it, and the test now uses that entry point.
- The race gate found a missing edge: for capacity 0 the receive also happens before the send completes. Each capacity-0 operation now releases into the channel when it starts and acquires from it when it completes (as Go does for unbuffered channels).
- A deadlock of every task printed Go's fatal error; it is now one `Deadlock` record with status 2 (SGO-CHAN-003).
- A child failure that replaced a cancellation unwind was reported with the owner's origin; the record now names the failing child, as a nursery exit would.

## Acceptance cases

| Case | Result | Evidence (strict_native and W07 lanes; the model and typed_channel lanes passed in W06) |
|---|---|---|
| H01 Buffered capacity invariant | passed | model buffer-bound invariant; `sgoChanCheck` on every native transition; `native-buffered`; TestSGoNativeGCStack (2,000 values, capacity 3, FIFO checked); fixture `native-buffered-fifo` |
| H02 Zero-capacity rendezvous | passed | model `native-rendezvous`, `native-select-*` (joint steps); a capacity-0 channel never queues (`sgoChanCheck`); fixture `native-rendezvous` |
| H03 Reserve then abort | passed | native send has no user-visible reservation: the value is boxed before the operation and reserve and commit are one step under the channel lock, so no reservation exists between user steps; a losing select send arm leaves no reservation (TestSGoNativeSelectStress, `select-send-arm`) |
| H04 Permit aliases | passed | no native permit value exists to alias; one terminal decision per operation (`sgoSel`, one CAS); fixture `select-duplicate-arms` (no duplicate effect) |
| H05 Cross-task permit misuse | passed | no native permit value exists; tokens, the native analog of an obligation, reject foreign use: `sync-token-foreign-release`, TestSGoLimiter |
| H06 Cancel before commit | passed | model `native-cancel-recv` (cancelled before the rendezvous: neither side takes effect); TestSGoNativeCancelUnwind; fixtures `native-cancel-unwind`, `native-range-cancel` |
| H07 Cancel after commit | passed | model: a completed operation is not undone by later cancellation (one decision per operation); fixture `sync-event-set-no-cancel` (an operation that need not wait does not observe cancellation) |
| H08 Close versus commit | passed | model `native-close-race` (exactly its three outcomes); fixtures `native-send-closed`, `native-close-drain`, `select-send-closed` |
| H09 Panic with permit | passed | no reservation is held across user code by a native operation; for tokens: `sync-token-abandoned` (released at task end, `TokenAbandoned` in the chain) |
| H10 Owner exits with permit | passed | as H09; tokens acquired in a nursery body are released before its join (`sgoTokensResolve` with the body's scope, TestSGoLimiter) |
| H11 Pointer value lifetime | passed | TestSGoNativeGCStack (GC every 97 values, stack growth on both sides; values are heap boxes, no stack pointer is shared while waiting); `error` values through channels: fixture `native-close-idempotent` |
| H12 Conservation under stress | passed | model conservation invariant; TestSGoNativeSelectStress (4 producers × 300 values through three-arm selects over two channels, two consumers: every value received exactly once, no registered arm left); TestSGoNativeOutcomes |
| N01 Select single winner | passed | model (at most one arm per select); `sgoSel` single decision; TestSGoNativeSelectStress; fixture `select-ready-arms` |
| N02 Select evaluation order | passed | fixture `select-eval-order` (operands and values evaluated once, in source order: `a1b2`) |
| N03 Nil, default, and empty select | passed | nil channels do not exist (`chan-type`, `chan-named-result-denied`); `select-default`, `native-select-default` (model); `select {}`: TestSGoNativeEmptySelect (waits for cancellation, then unwinds), `ctl-select` (root deadlock record) |
| N04 Close semantics | passed | `native-send-closed` (ClosedChannel, no effect), `native-close-drain` (buffered values then zero and ok = false), `native-close-idempotent`; cancellation stays distinct from closure (`native-cancel-unwind`); no raw Go channel path (`test-native-lowering.py`) |
| N05 Reflection and range | passed | `native-range`, `native-directional`, `native-chan-of-chan`; `reflect` stays denied (`reflect-import`), so `reflect.Select` and reflect channel operations are rejected before support is claimed; user generics stay denied (SGO-GEN-001), so no generic channel helper path exists; the compiler refuses `reflect_*` channel entry points for S-Go packages |
| N06 Duplicate channel cases | passed | channel locks deduplicated in address order; TestSGoNativeSelectStress (two arms on the same channel); fixture `select-duplicate-arms`; no leaked arm (`SGoNWaiters` is 0 afterwards) |
| N07 Channel memory ordering | passed | TestSGoNativeRace under `-race`: buffered send, rendezvous (both directions), close, select, Event, and limiter orderings report no race in 10 histories each; a deliberate unordered pair is reported; the nursery join edge is W04's |
| N08 Cancellation signal and recovery | passed | TestSGoNativeCancelUnwind (waiters removed before the unwind, defers run, own nursery joined first); the signal cannot be recovered: `recover` is not part of S-Go (SGO-DEF-002); fixtures `native-cancel-*`; cancellation stays requested (the nursery's scope stays CANCELLING) |
| N09 Fast path and stack movement | passed | TestSGoNativeGCStack; the fast paths (`sgoNativeTrySend`, `sgoNativeTryRecv`) are the same code as the waiting paths' completion, under the channel lock; no stack pointer is stored in a waiter (values are copied into heap boxes before waiting and out of them after waking); `-cpu=1,4,16` runs |
| S12 Unadmitted channel cannot execute | passed | W03 part (subset 0); W07 part: `test-native-lowering.py` (only S-Go entry points; self-test), `typecheck.LookupRuntime` refuses Go's entry points, denied forms `chan-len-denied`, `chan-cap-denied`, `chan-compare-denied`, `chan-elem-denied`, `chan-recv-only-close` |
| S16 Value and memory semantics closure | passed | W01 and W03 parts; W07 part: shared mutation stays impossible (no package variables, no pointers or aggregates, captured variables are final, SGO-MEM-002), so `sync` and atomics stay denied (`sync-mutex`, SGO-SYNC-001); channel and limiter orderings are specified (SGO-CHAN-002, SGO-SYNC-002) and checked by the race gate |

## Gate (plan W07)

- Native channels use the sole S-Go contract: every lowered channel operation calls an S-Go entry point (`test-native-lowering.py`); the compiler has no path to Go's `hchan`.
- Winner and loser cleanup: one decision per select; losing arms removed before select returns (TestSGoNativeSelectStress, `SGoNWaiters`).
- Cancellation transfer: a blocked operation unwinds with no effect, defers run, owned nurseries join first, a joined child failure stays primary (TestSGoNativeCancelUnwind, `native-cancel-*`).
- The race detector sees channel synchronization and still detects deliberate races (TestSGoNativeRace).
- Forced GC and stack movement during channel operations (TestSGoNativeGCStack).
- The implementation agrees with the model on all bounded reference histories, with and without fault injection (TestSGoNativeOutcomes).

## Decisions and open items

- Recorded in the 0.3.0 clauses: named results of channel type are a compile error; a whole-program deadlock is a `Deadlock` record with status 2; a child failure that replaces a cancellation unwind keeps the child's origin.
- The select choice is uniform but not reproducible in production (D34); the lab scheduler (W08) will record and replay it.
- Aggregate channel elements (structs, slices) and handles as elements stay denied until the decision deferred by D29 and D36.
- Registry rows SPEC-CHAN and SPEC-SYNC are `specified_not_admitted`, like every row; admission follows the policy of constitution §6.
