# Invariant-to-test map (W01)

Where each invariant of plan §3 is checked. "Model" = `sgo/model` (`Check` runs after every transition of every explored interleaving); "Mutation" = the deliberately wrong semantics the model must reject (`TestMutationsDetected`); conformance fixtures run from W03.

| Invariant | Model check (`explore.go` `Check`) | Scenarios (`scenarios_test.go`) | Mutation that must be caught | Clauses |
|---|---|---|---|---|
| OWN-1 one owning scope | every task's scope exists and lists it | all spawning scenarios | — (structural) | SGO-SPAWN-003 |
| OWN-2 registered before runnable | unfinished task is live in its scope | normal-join, descendant-during-join | MutLateRegister | SGO-SPAWN-003 |
| OWN-3 closure means no live work | CLOSED ⇒ no live children; exit 0–2 ⇒ all tasks done | all | MutEarlyJoin | SGO-NUR-003 |
| OWN-4 child exit before join returns | CLOSED ⇒ every child finished | all | MutEarlyJoin | SGO-NUR-003, SGO-MEM-004 |
| OWN-5 no bare go | n/a in model (static) | — | — | SGO-SPAWN-005; W00 gate check; fixtures W03 |
| OWN-6 sealed runtime services | n/a in model | — | — | SGO-TCB-002; `tcb-manifest.py` (11 enumerated services) |
| CAN-1 sticky, idempotent | scope states only move forward; first cause kept | cancel-propagates, outer-cancel-reaches-inner | — | SGO-CANCEL-001 |
| CAN-1 per-scope stickiness | via outcomes | cancel-consumed-at-boundary, inner-cancel-not-outer | MutTaskStickyCancel | SGO-CANCEL-002, SGO-CANCEL-004 |
| CAN-2 one terminal disposition per wait | each wait resolved at most once | sleep-cancel-race, cancel-propagates | MutDoubleWake | SGO-WAIT-001 |
| CAN-3 reported progress | n/a in model (no partial effects) | — | — | SGO-PROC-003 (fixtures W03) |
| CAN-4 cleanup keeps primary failure | owner failure stays primary | owner-failure-primary | — | SGO-NUR-004 rule 1, SGO-DEF-004 |
| CAN-5 no false success on drain overrun | no CLOSED with live children; non-cooperative child ⇒ stuck or exit 3 | noncooperative-budget, noncooperative-no-budget | MutEarlyJoin | SGO-DRAIN-001, SGO-DRAIN-002 |
| Outcome determinism (lowest creation ID) | via outcome sets | two-failures-lowest-id, child-panic-cancels-sibling | MutLatestFailure | SGO-NUR-004 |
| Failing child cancels its scope | property `scope0CauseIsChild` | child-panic-cancels-sibling, owner-woken-by-child-failure | MutNoCancelOnFail | SGO-SPAWN-004 |
| Spawn admission (current scope) | via outcomes | spawn-into-outer-from-inner, spawn-when-cancelling | MutNoCurrentScopeCheck | SGO-SPAWN-002 |
| Exits crossing nurseries join | via outcomes and OWN-3 | return-crossing-nursery | MutEarlyJoin | SGO-CTL-004 |
| SPEC-1..3 closed contract set | n/a in model | — | — | `spec-tool.py`, `inventory-check.py` |
| GC-1, GC-2, CH-*, CAP-*, LAB-*, SPEC-4, SPEC-5 | n/a in model | — | — | later work packages (W02, W05–W08), SGO-BUILD-001 |

`TestCheckRejectsInvalidHistories` additionally feeds hand-corrupted states to `Check` (closed scope with a live child, a state moving backwards, a wait resolved twice, a task missing from its scope); all are rejected.
