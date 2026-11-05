# W22 evidence — the language core from Happy Eyeballs (spec 0.17.0)

Decisions D159–D172. Clauses frozen at 0.17.0: SGO-WAIT-003, SGO-WAIT-004, SGO-SPAWN-006,
SGO-SPAWN-007, SGO-CANCEL-005; amended SGO-RES-002 v3, SGO-RES-003 v4, SGO-RES-004 v4,
SGO-AUTH-002 v4, SGO-AUTH-003 v3. D159 (error exits cancel crossed nurseries) landed on main
before the freeze, with the W21 errata.

## What was built

| Part | Where |
|---|---|
| Timers as `select` arms, `case <-t.After(ms):` (D160) | `src/runtime/sgo_after.go`, `sgo_native_ops.go`, `src/sgo/sgo.go`, `types2/sgo_check.go` |
| Scope timers `n.NewTimer` / `t.NewTimer`: a resource of the scope, kept across selects, `Reset`, `Stop`, `MoveTo` (D171) | `src/runtime/sgo_timer.go`, `src/sgo/timer.go`, `types2/sgo.go`, `sgo_resflow.go`, `noder/reader.go` |
| Value-returning tasks `sgo.Go`, `Task[T]`, `Result[T]` (D161, D168); task-local results refused; handles shareable; resource-delivering handles tied to their nursery | `src/sgo/task.go`, `src/runtime/sgo_taskres.go`, `types2/sgo_check.go`, `sgo_values.go`, `sgo_resflow.go` |
| Groups `sgo.NewGroup`, `g.Go`, `g.Next` (D172): limit as a channel semaphore, results in completion order with an unbounded overflow list | `src/sgo/group.go`, `src/runtime/sgo_group.go`, `sgo_native_ops.go`, `sgo_chan.go` |
| Deadlines `n.CancelAfter`, `t.Deadline`, `sgo.IsDeadline` (D162, D165, D170); in the lab, a deadline driver that cancels with no lock held (runtime service `lab-driver`, OWN-6) | `src/runtime/sgo_deadline.go`, `sgo_lab.go`, `sgo_lab_sched.go`, `sgo_service.go`, `sgo/scripts/tcb-manifest.py` |
| Effect handles without `LendTo` or revocation (D169, option a) | `src/sgo/effects.go`, `src/sgo/io.go`, `src/runtime/sgo_effects.go` |
| Happy Eyeballs rewritten on timers and groups: one nursery, one `select` (`he.Connect` 133 → 93 lines) | `sgo/examples/happyeyeballs/he/he.sgo` |

## Executed on darwin/arm64 (2026-11-05)

- Conformance 671/671 at 0.17.0, including the W22 cases: `timer-*` (After), `timer-scope-*`,
  `lab-timer-scope`, `task-*`, `group-*`, `lab-group-order`, `deadline-*`, `lab-deadline-join`,
  `lab-deadline-select`, `res-defer-nursery-close`, and `auth-lend-revoked` as a reject.
- Examples: pipeline, service, replay, happyeyeballs — `sgo check`, `sgo test`, `sgo run`,
  `sgo lab sweep` over every lab scenario.
- Runtime lane (`TestSGo*`), linux/amd64 and linux/arm64 vet of `runtime` and `sgo`.
- Gates: spec-tool (140 clauses: 133 frozen, 7 superseded), guarantee-check, test-identity,
  test-mutations, test-lab, test-tcb, test-fmt, inventory, policy, legacy-audit, lock-alloc-check,
  generated-twin-check, source-map-audit, license-audit, adoption-check, std-api-check.

## Linux

sgo-w00 run 41884028104 (linux/amd64, linux/arm64) on main at ba71ee9d4f: success on both runners (conformance, runtime lane, lab, examples including the two new Happy Eyeballs scenarios, identity, TCB). The first run on the freeze (41880232544) passed 670 of 671 on linux/arm64; the failure was a race in the `task-cancelled-result` fixture itself, fixed in 1a18d09d1b.

## Model (`sgo/model/w22model.go`, `w22model_test.go`)

Exhaustive exploration, run by `sgo/scripts/test-model.sh` (46 model tests pass):

| Explorer | Scope | Properties |
|---|---|---|
| `GroupExplore` | an owner starts K children through `g.Go` with limit L (0: none) and reads R results; a one-slot buffer with an overflow list; 23,830 states | G1 at most L children run at once; G2 the results received are a prefix of the publication order, each received once; G3 every run ends (no deadlock), however few results the owner reads; G4 the buffer is never empty while results are queued |
| `TimerExplore` | a timer armed, re-armed (`Reset`) or stopped at chosen times, with the production fires of every arming arriving, stale ones included; 2,664 runs | T1 a value is delivered only at or after the current deadline; T2 at most one value per arming; T3 a stopped timer delivers nothing until a `Reset` |
| `DriverCheck` | the lab deadline driver against every lock a parked task may hold | D1 the driver cancels holding no lock (D170) |
| `CancelTreeExplore` | nested nurseries (up to 3), each with a sleeping child and an optional deadline, ending normally or by an error exit crossing every open nursery; 1,609 states | C1 a woken child observes the cause of the first cancelled enclosing nursery, so an inherited deadline reports as a deadline (D165); C2 only a nursery's deadline or a crossing exit cancels it; C3 every run closes every nursery (D159); C4 no deadline cancels a closed nursery |

Seeded bugs, each caught by the property named (`TestW22Mutations`): no refill (G4), overwrite a
full buffer (G2), no slot release (G3), refill from the newest (G2), stale timer fire (T1), inline
cancel in the lab (D1), no deadline inheritance (C1), cancellation not seen inside (C1), error exit
without cancel (C3), deadline after close (C4), cancelling the parent too (C2).

The lab model (`labmodel.go`) still has no nurseries or cancellation; lab deadlines and groups are
pinned by the W22 fixtures, lab cases, and traces (D30), and the cancellation rules by the model
above.

## Happy Eyeballs acceptance

`he.Connect` uses no timer tasks, no event channel, and no explicit cancellation on error paths
(one `n.Cancel()` ends the decided race). Lab scenarios: silent-v6, slow-aaaa, prefer-v6, slow-v6,
all-refused, late-a-after-tick, late-aaaa-after-tick, log-fails-pending, and the two the 0.16
version could not test: `older-fails-later` (an older attempt fails after a newer one started)
and `two-connected` (two attempts connect before selection; the sweep sees both winners). Each
sweeps 64 seeds in `test-examples.py`.

## Open

- Review of the 0.17.0 freeze requested from the reviewer (2026-11-05).
- A greeting split across reads is not a scenario (the lab peer writes once); `ReadAll` is W23.
