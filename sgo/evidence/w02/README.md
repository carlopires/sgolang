# W02 evidence — runtime ownership, join, and GC-safe state

| Field | Value |
|---|---|
| work_package_id | W02 |
| state | passed on darwin/arm64 (local), linux/amd64 and linux/arm64 (CI `sgo-runtime` run 41109703265) |
| spec_version | 0.1.0 (no clause changed) |
| fork_commit | `ba43750d6c` |
| test lane | `sgo/scripts/runtime-lane.sh`: upstream go1.27.3 tool sources + fork TCB overlay (digest `fb046b29596e2779`); host-only, never a target |
| TCB changes | 4 runtime files differ from upstream (`sgo/TARGET_TCB.json`): `sgo_scope.go`, `sgo_service.go` (new), `proc.go`, `runtime2.go`; review notes in the commit messages of `5b865618` and `ba43750d` |
| design | `sgo/docs/runtime-design.md` |
| next_dependency_ready_task | W03 — nursery compiler path and exit semantics |

## Acceptance cases

| Case | Test (`src/runtime/sgo_test.go`) | Result |
|---|---|---|
| O01 brace-equivalent join | TestSGoJoin | passed |
| O02 nested scope ownership | TestSGoNestedOwnership | passed |
| O03 explicit delegation | TestSGoDelegation | passed |
| O04 invalid spawn capability | TestSGoInvalidHandles (nil, nil callback, closed, ancestor bypass, unrelated task, non-task) | passed; cancellation admission is W04 |
| O05 descendant admission during join | TestSGoDescendantDuringJoin (300 iterations) | passed |
| O06 goroutine record reuse | TestSGoGoroutineReuse (2000 tasks, then 2000 plain goroutines) | passed |
| O07 GC root pressure | TestSGoGCRoots (payloads survive 5 GCs; scope and payload finalized after quiescence) | passed |
| O08 movable parent stack | TestSGoMovableStack (spawn from a 4000-frame-deep stack, GC/shrink) | passed |
| O09 join visibility | TestSGoJoinVisibility, also under -race | passed; removing the race annotations makes -race report a race (checked) |
| O10 runtime service separation | TestSGoServiceCensus, TestSGoServiceEnforcement | passed |
| S06 no implicit authority | TestSGoNoAmbientAuthority | W02 part passed (runtime); language part W03 |
| S07 checked nested-scope capability | TestSGoInvalidHandles | W02 part passed; cleanup no-spawn rule needs scope finalizers (later) |
| S11 target TCB is specified | TCB manifest + enforcement child tests (plain go, spoofed creator) | W02 part passed; compiler caller checks W03 |

Additional checks: two source mutations (early join, registration after publication) are caught by the runtime's invariant checks; tests pass with `-count=20 -cpu=1,4,16`; the full upstream runtime `-short` suite passes on all three targets.

## Limitations

- Enforcement of unenumerated goroutine creation is off in the test lane (its tests are Go code); W03 enables it for every S-Go program at start-up.
- The scope lock is unranked; a static lock rank is added when W04 takes scope locks along a scope chain.
- The 32-bit `g` size in `sizeof_test.go` is updated but unverified (no admitted 32-bit target).
