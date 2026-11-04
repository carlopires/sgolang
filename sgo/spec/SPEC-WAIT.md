# SPEC-WAIT — Wait registration, terminal winner, park/unpark and lock order

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §12, resolution R8, plan §6.2, invariant CAN-2.

### SGO-WAIT-001 — Sleep
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-CANCEL-002, SGO-CANCEL-003, SGO-DEF-003, SGO-TIMER-001; R8; CAN-2.
- **grammar_and_types:** `task.Sleep(ms int) error` where `task` has type `*sgo.Context`.
- **preconditions:** `task` is the calling task's own context (as SGO-CANCEL-002); `ms ≥ 0`.
- **evaluation_and_value_semantics:** If a linked scope is already CANCELLING, returns a cancellation error immediately. Otherwise registers one wait and parks the task until either at least `ms` milliseconds of monotonic time have elapsed (returns `nil`) or a linked scope enters CANCELLING (returns a cancellation error). `Sleep(0)` returns `nil` unless cancelled; it may yield to other tasks.
- **ownership_and_lifecycle:** The wait registration belongs to the task and is removed exactly once, by whichever of expiry or cancellation wins (CAN-2).
- **effect_and_capability_requirements:** n/a — no external effect.
- **concurrency_and_memory_order:** A cancellation that happens-before the timer expiry wins only if it is observed before the expiry is committed; exactly one result is returned.
- **allowed_nondeterminism:** When cancellation and expiry race, either result may be returned.
- **failure_and_partial_progress:** `ms < 0` fails with `InvalidArgument` before any registration. Context misuse fails with `SpawnContractViolation`.
- **cancellation_and_cleanup:** `Sleep` is a cancellation point (SGO-CANCEL-002). Cancellation wakes the task promptly (bounded by scheduling, not by `ms`).
- **compile_link_checks:** `SGO_E_DISCARDED_ERROR` if the result is unused; a negative constant argument is a compile error `SGO_E_INVALID_ARGUMENT`.
- **runtime_guards:** Argument and context-ownership checks; one-winner registration.
- **tcb_assumptions:** Runtime timers and park/unpark with the atomic park-unlock handshake (plan §6.2, SGO-TCB-001).
- **positive_tests:** C01, C09; fixture:wait-sleep, fixture:cancel-wakes-sleep.
- **negative_tests:** C09; fixture:wait-negative-sleep.
- **implementation_coverage:** W04: `src/runtime/sgo_api.go`, `src/runtime/sgo_nursery.go`.
- **executed_evidence:** conformance fixtures cancel-wakes-sleep, wait-negative-sleep, wait-sleep (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w04/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-WAIT-002 — Join is a managed wait
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-NUR-003, SGO-WAIT-001.
- **grammar_and_types:** n/a — dynamic rule.
- **preconditions:** n/a — dynamic rule.
- **evaluation_and_value_semantics:** The owner waiting in JOINING or CANCELLING holds one wait registration, woken when the live-child count reaches zero. Lost wake-ups are impossible: the last child's completion and the owner's park use the same one-winner protocol as SGO-WAIT-001.
- **ownership_and_lifecycle:** The registration belongs to the owner and ends when the scope reaches FINALIZING.
- **effect_and_capability_requirements:** n/a — dynamic rule.
- **concurrency_and_memory_order:** As SGO-NUR-003.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** n/a — cannot fail.
- **cancellation_and_cleanup:** Not a cancellation point: cancellation of the scope does not end the owner's wait (SGO-NUR-003).
- **compile_link_checks:** n/a — dynamic rule.
- **runtime_guards:** n/a — runtime protocol.
- **tcb_assumptions:** Park/unpark (SGO-TCB-001).
- **positive_tests:** O02, O04; fixture:nur-join-all.
- **negative_tests:** n/a — protocol property; checked by the executable model and mutation tests.
- **implementation_coverage:** W02: `src/runtime/sgo_api.go`, `src/runtime/sgo_nursery.go`.
- **executed_evidence:** conformance fixtures nur-join-all (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w02/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-WAIT-003 — Timers as select arms
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-WAIT-001, SGO-CHAN-002, SGO-CANCEL-002, SGO-LAB-004, SGO-DEF-003; D160.
- **grammar_and_types:** `task.After(ms int) <-chan int` where `task` has type `*sgo.Context`, admitted only as the operand of a select receive arm: `case <-task.After(ms):` or `case v := <-task.After(ms):`. A single-arm select is admitted.
- **preconditions:** `task` is the calling task's own context (as SGO-CANCEL-002); `ms ≥ 0`.
- **evaluation_and_value_semantics:** The arm operand is evaluated with the other arm operands (SGO-CHAN-002), which makes the timer. The arm can proceed once at least `ms` milliseconds of monotonic time have elapsed since then, and receives `ms`. An arm that can proceed when the select starts competes with the other ready arms by the select's choice rule; `After(0)` is ready at once. A select parked on a timer arm is woken when the deadline passes, exactly as by a sender.
- **ownership_and_lifecycle:** The timer belongs to the task that made it and serves one select. When the select ends, by any arm or by cancellation, the timer is dropped: it is never delivered later and holds no runtime resource.
- **effect_and_capability_requirements:** n/a — no external effect. No clock grant is needed: a timer reveals elapsed time only by the order in which arms proceed (D30 guards reading the time).
- **concurrency_and_memory_order:** The timer's value is delivered at most once; exactly one arm of the select proceeds.
- **allowed_nondeterminism:** In production, when the deadline and another arm become ready together, either may proceed. In the lab the order is decided by virtual time and the select rules (SGO-LAB-004): timers fire at an advance in creation order.
- **failure_and_partial_progress:** `ms < 0` fails with `InvalidArgument` before the timer is made. Context misuse fails with `SpawnContractViolation`.
- **cancellation_and_cleanup:** A select with a timer arm is a cancellation point like any select; cancellation wakes it promptly (bounded by scheduling, not by `ms`) and drops the timer.
- **compile_link_checks:** `SGO_E_UNADMITTED` for `After` anywhere except as the operand of a select receive arm.
- **runtime_guards:** Argument and context-ownership checks; one-winner delivery under the channel's wait protocol.
- **tcb_assumptions:** Runtime timers and park/unpark (as SGO-WAIT-001); in the lab, virtual timers (SPEC-LAB).
- **positive_tests:** fixture:timer-arm-fires, fixture:timer-loses-to-ready, fixture:timer-zero, fixture:timer-in-loop, fixture:timer-cancelled, fixture:lab-timer-arm.
- **negative_tests:** fixture:timer-placement-denied, fixture:timer-negative.
- **implementation_coverage:** W22: `src/runtime/sgo_after.go`, `src/runtime/sgo_native_ops.go`, `src/sgo/sgo.go`, `src/cmd/compile/internal/types2/sgo_check.go`.
- **executed_evidence:** conformance fixtures (darwin/arm64); Linux pending.
- **supported_execution_configurations:** production and lab.

### SGO-WAIT-004 — Scope timers
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-WAIT-003, SGO-CHAN-002, SGO-RES-001, SGO-RES-002, SGO-RES-003, SGO-CANCEL-002, SGO-LAB-004, SGO-DEF-003; D171.
- **grammar_and_types:** `s.NewTimer(ms int) *sgo.Timer` where `s` has type `*sgo.Context` or `*sgo.Nursery`; methods `tm.C() <-chan int`, `tm.Reset(ms int)`, `tm.Stop()`, `tm.MoveTo(s Scope) error`. `*sgo.Timer` is a resource handle type (SGO-RES-001). `tm.C()` may be used wherever a receive-only channel of `int` is admitted, in any number of selects and receives.
- **preconditions:** For a context, `s` is the calling task's own context (as SGO-CANCEL-002). The calling task runs inside the scope `s` names, and a nursery has not ended. `ms ≥ 0`. `C`, `Reset`, and `Stop` are called by a task running inside the timer's scope (D150).
- **evaluation_and_value_semantics:** `NewTimer` makes a timer of scope `s`, armed for `ms` milliseconds of monotonic time from the call. An armed timer's channel has a value once its deadline has passed: the duration it was last armed with. One receive takes it, and the timer is then fired. Until it is fired, every select or receive on `tm.C()` sees the same deadline, whichever selects started or ended before. A parked receiver is woken when the deadline passes, as by a sender. `Reset(ms)` re-arms the timer for `ms` from the call, whether it was armed, fired, or stopped; a value of the previous arming that was not taken is discarded. `Stop()` disarms it: no receive on `tm.C()` proceeds until a `Reset`.
- **ownership_and_lifecycle:** A timer is a resource of the scope it was made in (SGO-RES-002). The scope disarms it when it ends, after its join. The resource flow rule (SGO-RES-003) keeps the handle inside the scope's extent; `MoveTo` moves it to an enclosing scope. Ending a select that used it, by any arm or by cancellation, does not disarm it (unlike SGO-WAIT-003).
- **effect_and_capability_requirements:** n/a — no external effect. No clock grant is needed, as SGO-WAIT-003: a timer reveals elapsed time only by the order in which receives proceed.
- **concurrency_and_memory_order:** Each arming's value is delivered at most once. `Reset` and `Stop` by one task and receives by others are ordered by the timer's channel lock: a receive that proceeds after a `Stop` or `Reset` completes sees its effect.
- **allowed_nondeterminism:** In production, when a deadline and another arm become ready together, either may proceed. In the lab, time is virtual (SGO-LAB-004): a timer fires at the advance to its deadline, in arming order with the other timers due then.
- **failure_and_partial_progress:** `ms < 0` fails with `InvalidArgument` before any effect. A task outside the timer's scope fails with `MissingCapability`; context misuse fails with `SpawnContractViolation`.
- **cancellation_and_cleanup:** A select or receive on `tm.C()` is a cancellation point like any channel wait; cancellation wakes it and leaves the timer armed. The timer holds no runtime resource after its scope ends.
- **compile_link_checks:** `SGO_E_RESOURCE_ESCAPES` for a timer flowing beyond its scope's extent (SGO-RES-003).
- **runtime_guards:** Argument checks; the scope check for `C`, `Reset`, `Stop`; `HandleClosed` for a use after the scope ended (unreachable from admitted code). A fire left from an earlier arming is ignored.
- **tcb_assumptions:** Runtime timers and park/unpark (as SGO-WAIT-003); in the lab, virtual timers (SPEC-LAB).
- **positive_tests:** fixture:timer-scope-across-selects, fixture:timer-scope-reset-stop, fixture:timer-scope-child, fixture:lab-timer-scope.
- **negative_tests:** fixture:timer-scope-negative, fixture:timer-scope-escape.
- **implementation_coverage:** W22: `src/runtime/sgo_timer.go`, `src/runtime/sgo_after.go`, `src/sgo/timer.go`, `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/types2/sgo_resflow.go`, `src/cmd/compile/internal/sgoadmit/admit_expr.go`, `src/cmd/compile/internal/noder/reader.go`.
- **executed_evidence:** conformance fixtures (darwin/arm64); Linux pending.
- **supported_execution_configurations:** production and lab.
