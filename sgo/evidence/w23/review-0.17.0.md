# 0.17.0 review (`.work/sgo-017-issues.md`): corrective work on `w23`

The review was made against main `7d95092eba` (spec 0.17.0). The fixes are on branch `w23` above
`dcf20e0fcd`; they reach main with the W23 merge after the 0.18.0 freeze (the clause amendments
are listed in `sgo/docs/w23-drafts.md`). The historical W22 record (`sgo/evidence/w22/README.md`)
is unchanged and does not validate this work.

| Finding | Status | Commit | Evidence |
|---|---|---|---|
| SGO17-01 unadmitted start never resolves | fixed | `663cccb977` | reproduced before the fix (production: process Deadlock; lab: LabDeadlock); fixture spawn-go-not-admitted (local cancel, inherited deadline, bounded group) |
| SGO17-02 shallow resource promotion | fixed | `533c11ae09` | reproduced before the fix (Reset on a wrapped timer: MissingCapability); fixture task-aggregate-resource-result (nested struct, array, repeated reference, already in the nursery, group) |
| SGO17-03 SGO-CTL-004 v1 contradicts D159 | amendment owed at 0.18.0 | `533c11ae09` (draft text) | confirmed by reading the clauses; fixture task-error-rollback-inner-nursery |
| SGO17-04 `Release(ok)` of a `Go` body | contract kept, documented | `533c11ae09`, `911d5c4918` | fixture task-error-rollback-inner-nursery (inner transaction rolls back, body commits, sibling not failed) |
| SGO17-05 result, group, timer contracts | documented | `911d5c4918` | API comments; behavior already covered by lab-task-result-once, group-unread-results, timer-scope-across-selects and the runtime lane TestSGoTaskResult |
| SGO17-06 user-resource sharing | spec narrowing owed at 0.18.0 (SGO-RES-004 v5) | `533c11ae09` | fixture res-user-capture-denied |
| SGO17-07 guarantees, roadmap, evidence | done | this commit | `guarantee-check.py` now fails on a stale `GUARANTEES.md` (one renderer for write and check; `--selftest` shows changed, dropped and added lines are rejected); G-OWN, G-FAIL, G-CANCEL, G-RES updated for W22; roadmap corrected |

Not addressed: a nil handle field (SGO17-02's nil case) cannot occur, since a handle-typed field
has no nil value (SGO-TYPE-005). User resources cannot be `Go` results (they are task-local,
SGO17-06), so promotion is exercised on built-in handles.

## Executed (darwin/arm64, stage-0 toolchain built from `533c11ae09` plus the comment-only `911d5c4918`)

| Command | Result |
|---|---|
| `conformance.py run` (full, at `533c11ae09`) | 687/687, 0 unexpected |
| `conformance.py run` task-error-rollback-inner-nursery, res-user-capture-denied, task-aggregate-resource-result | 3/3 |
| `conformance.py validate` | 689 cases |
| `test-examples.py` | ok: pipeline, service, replay, happyeyeballs (check, test, run, lab sweep) |
| `runtime-lane.sh -run TestSGo runtime` | ok |
| `runtime-lane.sh -race -run TestSGo runtime` | ok (67.7 s), no DATA RACE |
| `test-identity.py`, `spec-tool.py check`, `inventory-check.py`, `lock-alloc-check.py`, `generated-twin-check.py`, `tcb-manifest.py`, `license-audit.py` | ok |
| `guarantee-check.py --selftest` | ok |
| `guarantee-check.py` | the drift check passes; the two W23 draft clauses (SGO-FILE-004, SGO-SPAWN-008) fail it as designed until the 0.18.0 freeze |

Linux: not run for these commits. CI runs on main only, so Linux coverage comes with the W23 merge.
