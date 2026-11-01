# SPEC-SYNC — Shared-memory synchronization

Go's shared-memory synchronization stays denied in every subset (SGO-SYNC-001, D37). Spec 0.3.0 adds the structured primitives `sgo.Event` and `sgo.CapacityLimiter` (SGO-SYNC-002, D39), which coordinate tasks without shared state.

### SGO-SYNC-001 — Shared-memory synchronization are denied in subset 0
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-ERR-001; D01.
- **grammar_and_types:** Denied: `sync`, `sync/atomic`, and any mutex, condition variable, wait group, once, or atomic API. No form of these constructs, packages, or APIs is admitted in any package compiled for the target, including dependencies and unreachable code.
- **preconditions:** n/a — denial clause; there is nothing to use.
- **evaluation_and_value_semantics:** n/a — denied constructs never execute.
- **ownership_and_lifecycle:** n/a — denied constructs never execute.
- **effect_and_capability_requirements:** n/a — denied constructs never execute.
- **concurrency_and_memory_order:** n/a — denied constructs never execute.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Any use is a compile error; no object file is produced.
- **cancellation_and_cleanup:** n/a — denied constructs never execute.
- **compile_link_checks:** `SGO_E_UNADMITTED`, naming the construct and this clause.
- **runtime_guards:** n/a — rejected before execution.
- **tcb_assumptions:** n/a — no runtime involvement.
- **positive_tests:** n/a — denial clause; admission is planned for later subset (sendability rules, D01).
- **negative_tests:** S16; fixture:sync-mutex.
- **implementation_coverage:** gate level only — `sgo/scripts/sgo-target` denies every program (W00); compiler-level denial is W03.
- **executed_evidence:** W00 gate checks passed on darwin/arm64, linux/amd64, linux/arm64 (gate level; not construct-specific).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-SYNC-002 — Event and CapacityLimiter (subset 2)
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-SYNC-001, SGO-TYPE-005, SGO-CAP-001, SGO-CANCEL-002, SGO-SPAWN-004, SGO-PERMIT-002, SGO-DEF-003, SGO-CHAN-003; D37, D39.
- **grammar_and_types:** `ev := sgo.NewEvent() *sgo.Event` with `ev.Set()`, `err := ev.Wait(task *sgo.Context) error`, and `ev.IsSet() bool`. `lim := sgo.NewCapacityLimiter(n int) *sgo.CapacityLimiter` with `tok, err := lim.Acquire(task *sgo.Context) (*sgo.Token, error)` and `tok.Release()`. `*sgo.Event`, `*sgo.CapacityLimiter`, and `*sgo.Token` are capability handle types (SGO-TYPE-005): no nil value, no comparison. The error results of `Wait` and `Acquire` must be used (SGO-CTL-001). No other member, and no semaphore, lock, or condition variable, is admitted (D39).
- **preconditions:** `n >= 1`. `Wait` and `Acquire` take the calling task's own context. A token is released only by the task that acquired it, at most once.
- **evaluation_and_value_semantics:** An event is unset until its first `Set`; `Set` on a set event has no effect. `Wait` returns nil at once if the event is set, else waits until it is set. A limiter has `n` slots; `Acquire` takes a free slot at once when no acquirer is waiting, else waits in FIFO order; `Release` passes the slot directly to the oldest waiting acquirer, else frees it. At every moment the slots held equal the tokens not yet released, and never exceed `n`.
- **ownership_and_lifecycle:** Events and limiters belong to no task or nursery. A token is a task-affine obligation like a send permit: an unreleased token is released at the exit of the nursery body it was acquired in, before the join, and at the end of its task, before the task's outcome is published, each with a `TokenAbandoned` entry in the task's failure sequence (shown in the chain of the diagnostic record if the task fails or returns an error).
- **effect_and_capability_requirements:** None beyond the handles.
- **concurrency_and_memory_order:** `Set` happens before every `Wait` that returns nil and every `IsSet` that returns true. A `Release` happens before the `Acquire` that obtains the released slot.
- **allowed_nondeterminism:** The interleaving of operations of different tasks; each history is one admitted by the model `sgo/model/nativemodel.go` (no Wait returning before Set; FIFO slots; no leaked token).
- **failure_and_partial_progress:** `NewCapacityLimiter` with `n < 1` fails with `InvalidArgument`. Releasing a token twice, or releasing another task's token, is a `SpawnContractViolation` and has no effect on the limiter.
- **cancellation_and_cleanup:** `Wait` and `Acquire` are cancellation points: one that must wait while the task is cancelled (SGO-CANCEL-002) returns the cancellation error with no effect (no slot taken). One that can proceed without waiting does not observe cancellation. A cancelled or failing task never leaks a slot (ownership above).
- **compile_link_checks:** `SGO_E_CAPABILITY_MISUSE` for nil or comparison of the handles; `SGO_E_DISCARDED_ERROR` for unused `Wait`/`Acquire` errors; `SGO_E_UNADMITTED` for other members.
- **runtime_guards:** Limiter occupancy within `0..n` under the limiter lock (a violation is a runtime defect, SGO-ERR-003); token affinity and single release.
- **tcb_assumptions:** The managed-channel lock, waiter records, and cancellation registry (TCB-P12, TCB-P04) and the obligation resolution of TCB-P12, used by `src/runtime/sgo_sync.go` (TCB-P18).
- **positive_tests:** fixture:sync-event-wakes-all, fixture:sync-event-wait-cancelled, fixture:sync-event-set-no-cancel, fixture:sync-limiter-handoff, fixture:sync-limiter-acquire-cancelled, fixture:sync-token-abandoned; TestSGoEvent, TestSGoLimiter; the model's event and limiter histories (TestSGoNativeOutcomes).
- **negative_tests:** fixture:sync-token-double-release, fixture:sync-token-foreign-release, fixture:sync-limiter-zero, fixture:sync-event-discarded-error, fixture:sync-token-compare-denied, fixture:sync-nil-event-denied, fixture:sync-mutex.
- **implementation_coverage:** W07: model `sgo/model/nativemodel.go`; runtime `src/runtime/sgo_sync.go`; API `src/sgo/sync.go`; admission `sgoadmit/admit_expr.go` (`sgoMembers`, `sgoHandleTypes`); handle rules `types2/sgo.go` (`isSgoHandle`).
- **executed_evidence:** PENDING: W07 evidence record.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
