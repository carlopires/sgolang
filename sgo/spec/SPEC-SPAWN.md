# SPEC-SPAWN — Explicit nursery capability, callback admission and outcome reporting

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §9, decisions D11, resolution R5, plan §4.2, invariants OWN-1, OWN-2, OWN-5.

### SGO-SPAWN-001 — Call form
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-TYPE-005, SGO-VAL-005, SGO-MEM-002; D11.
- **grammar_and_types:** `n.Spawn(f)` where `n` has type `*sgo.Nursery` and `f` has type `func(*sgo.Context) error`. `Spawn` has no result; it is used as a call statement.
- **preconditions:** Static: types as above. Dynamic: SGO-SPAWN-002.
- **evaluation_and_value_semantics:** The receiver `n`, then the argument `f`, are evaluated once in the calling task. Then admission (SGO-SPAWN-002) and registration (SGO-SPAWN-003) happen, and `Spawn` returns without waiting for the child.
- **ownership_and_lifecycle:** The child belongs to `n`'s scope for its whole life (OWN-1).
- **effect_and_capability_requirements:** Requires the nursery handle (SGO-CAP-001). The child receives no other authority (SGO-SPAWN-003).
- **concurrency_and_memory_order:** Evaluation of `n` and `f` happens-before the child starts (SGO-MEM-004).
- **allowed_nondeterminism:** When the child first runs relative to the caller.
- **failure_and_partial_progress:** See SGO-SPAWN-002. Evaluating `f` (e.g. building a closure) has no effect that needs undoing.
- **cancellation_and_cleanup:** See SGO-SPAWN-002 (CANCELLING scope).
- **compile_link_checks:** `SGO_E_TYPE_MISMATCH` (wrong callback type), `SGO_E_DISCARDED_ERROR` does not apply (no result).
- **runtime_guards:** SGO-SPAWN-002.
- **tcb_assumptions:** Task creation primitive (SGO-TCB-001).
- **positive_tests:** S06, S08; fixture:spawn-basic, fixture:spawn-helper-with-handle.
- **negative_tests:** S06; fixture:spawn-wrong-callback-type.
- **implementation_coverage:** W02, W03: `src/cmd/compile/internal/types2/sgo.go`, `src/runtime/sgo_api.go`, `src/runtime/sgo_nursery.go`.
- **executed_evidence:** conformance fixtures spawn-basic, spawn-helper-with-handle, spawn-wrong-callback-type (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w02/README.md`; `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-SPAWN-002 — Admission checks
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-NUR-002, SGO-NUR-005, SGO-DEF-003, SGO-CANCEL-002; R5; plan §4.2.
- **grammar_and_types:** n/a — dynamic rule.
- **preconditions:** n/a — this clause defines the dynamic preconditions.
- **evaluation_and_value_semantics:** Admission succeeds only if all hold, checked atomically with registration: (a) `n`'s scope is OPEN or JOINING; (b) the calling task is a member of `n`'s scope — its owner (while executing the nursery's block or a function called from it), or a live task registered in that scope, or a descendant task registered in a scope nested inside it; (c) `n`'s scope is the calling task's current scope (SGO-NUR-005), so a task inside an inner nursery cannot spawn into an outer one. If (a) fails because the scope is CANCELLING, no child is created and `Spawn` returns normally; the caller observes cancellation at its next `Checkpoint` or `Sleep` (R5). If the scope is FINALIZING or CLOSED, or (b) or (c) fails, no child is created and the caller fails with `SpawnContractViolation`.
- **ownership_and_lifecycle:** A rejected admission leaves no partial registration.
- **effect_and_capability_requirements:** n/a — admission rule.
- **concurrency_and_memory_order:** Admission is serialized with the scope's transition to FINALIZING (OWN-2), so a child cannot be admitted into a scope that is about to close.
- **allowed_nondeterminism:** Whether a concurrent `Cancel` is ordered before or after a given admission; both outcomes are specified above.
- **failure_and_partial_progress:** `SpawnContractViolation` unwinds the caller (SGO-DEF-003); no child exists.
- **cancellation_and_cleanup:** Silent non-admission in CANCELLING (R5).
- **compile_link_checks:** None possible in general (handles are values); the compiler may reject obviously stale uses as a lint, never as a substitute for the run-time check.
- **runtime_guards:** Checks (a)–(c) on every call.
- **tcb_assumptions:** Scope membership and current-scope tracking (SGO-TCB-001).
- **positive_tests:** S07, O04; fixture:nur-child-spawns-during-join, fixture:spawn-cancelling-no-child.
- **negative_tests:** S06, S07; fixture:spawn-after-close, fixture:spawn-into-outer-from-inner, fixture:spawn-unrelated-task.
- **implementation_coverage:** W02: `src/cmd/compile/internal/types2/sgo.go`, `src/runtime/sgo_api.go`, `src/runtime/sgo_nursery.go`.
- **executed_evidence:** conformance fixtures nur-child-spawns-during-join, spawn-after-close, spawn-cancelling-no-child, spawn-into-outer-from-inner, spawn-unrelated-task (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w02/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-SPAWN-003 — Registration and start
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-SPAWN-002, SGO-CANCEL-004, SGO-CAP-001, SGO-MEM-004; OWN-2.
- **grammar_and_types:** n/a — dynamic rule.
- **preconditions:** Admission succeeded.
- **evaluation_and_value_semantics:** The child is assigned the next creation ID (a process-wide counter; the root task is 0) and registered as live in the scope before it can run (OWN-2). It then runs `f(task)` with a fresh `*sgo.Context` `task`.
- **ownership_and_lifecycle:** The child is live from registration until its callback ends (SGO-SPAWN-004) and its deferred calls have run.
- **effect_and_capability_requirements:** The child's context holds no capability (CAP-1): `task.Stdout()` fails with `MissingCapability` (SGO-PROC-003). The child can use only handles it captured or receives.
- **concurrency_and_memory_order:** Registration happens-before the child starts.
- **allowed_nondeterminism:** Scheduling of the child.
- **failure_and_partial_progress:** If the runtime cannot create the task, the process terminates with resource exhaustion (SGO-ERR-003); a child is never half-registered.
- **cancellation_and_cleanup:** The child's context is linked to `n`'s scope and all scopes that cancel it (SGO-CANCEL-004). A child admitted into a scope that is cancelled immediately afterwards starts normally and observes cancellation at its first `Checkpoint`/`Sleep`.
- **compile_link_checks:** n/a — dynamic rule.
- **runtime_guards:** n/a — registration cannot fail except by exhaustion.
- **tcb_assumptions:** Task creation and the goroutine underneath it are sealed runtime primitives (SGO-TCB-001); no other task creation path exists (OWN-6).
- **positive_tests:** O01, S06; fixture:spawn-basic, fixture:spawn-creation-ids.
- **negative_tests:** S06; fixture:proc-child-stdout.
- **implementation_coverage:** W02: `src/cmd/compile/internal/types2/sgo.go`, `src/runtime/sgo_api.go`, `src/runtime/sgo_nursery.go`.
- **executed_evidence:** conformance fixtures proc-child-stdout, spawn-basic, spawn-creation-ids (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w02/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-SPAWN-004 — Child outcomes
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-CANCEL-001, SGO-CANCEL-003, SGO-DEF-005, SGO-NUR-004, SGO-RES-001; D149.
- **grammar_and_types:** n/a — dynamic rule.
- **preconditions:** n/a — dynamic rule.
- **evaluation_and_value_semantics:** A child ends with exactly one outcome: callback returns `nil` → `Succeeded`; returns an error `e` with `sgo.IsCancelled(e)` → `Cancelled`; returns any other non-nil error → `Errored`; unwinds with a failure → `Failed`. The outcome includes the release of the task body's resources (SGO-RES-001), which runs after the callback returns or unwinds and before the outcome exists: a callback that returned `nil` with a failed release ends `Errored`, with kind `ReleaseError` and the first failed release's message; any other failed release joins the chain of the outcome. On `Errored` or `Failed`, the child's scope is cancelled (SGO-CANCEL-001) when the outcome is published, before the scope's join can complete. The outcome and its error or failure are retained for SGO-NUR-004. No outcome is discarded.
- **ownership_and_lifecycle:** The child stops being live when its outcome is published, after its deferred calls ran.
- **effect_and_capability_requirements:** n/a — outcome rule.
- **concurrency_and_memory_order:** Publication of the outcome happens-before the nursery completes (OWN-4).
- **allowed_nondeterminism:** Which of several failing siblings publishes first; the nursery outcome does not depend on it (SGO-NUR-004).
- **failure_and_partial_progress:** The child's effects remain.
- **cancellation_and_cleanup:** A child that returns a cancellation error is `Cancelled`, not a failure, even if it was not actually cancelled (it only produced that error by calling `Checkpoint`/`Sleep` on a cancelled context, SGO-CANCEL-003).
- **compile_link_checks:** n/a — dynamic rule.
- **runtime_guards:** Outcome classification in the runtime.
- **tcb_assumptions:** Outcome publication is atomic with live-count decrement (SGO-TCB-001).
- **positive_tests:** S08, O05; fixture:nur-child-error-exit-1, fixture:spawn-cancelled-outcome, fixture:def-child-panic-cancels-siblings, fixture:res-release-at-task-end, fixture:res-release-error-task.
- **negative_tests:** n/a — behavioral; checked by exact-outcome fixtures and model histories.
- **implementation_coverage:** W02: `src/cmd/compile/internal/types2/sgo.go`, `src/runtime/sgo_api.go`, `src/runtime/sgo_nursery.go`.
- **executed_evidence:** conformance fixtures def-child-panic-cancels-siblings, nur-child-error-exit-1, spawn-cancelled-outcome (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w02/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-SPAWN-005 — No bare go
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-LEX-003, SGO-ERR-001; OWN-5; constitution §4.
- **grammar_and_types:** The `go` statement is not part of S-Go. Any occurrence of the `go` keyword in statement position, in any file of any package compiled for the target (including unreachable code and dependencies), is rejected. It is not rewritten into a structured spawn.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** n/a — never executes.
- **ownership_and_lifecycle:** n/a — static rule.
- **effect_and_capability_requirements:** n/a — static rule.
- **concurrency_and_memory_order:** n/a — static rule.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Compile error; no object file.
- **cancellation_and_cleanup:** n/a — static rule.
- **compile_link_checks:** `SGO_E_UNSTRUCTURED_SPAWN` with source position, in the compiler and in source tools (formatter, vet) that feed builds (S19).
- **runtime_guards:** n/a — static rule.
- **tcb_assumptions:** Runtime-internal goroutines are created only through sealed service primitives, never by user-visible `go` (SGO-TCB-001, OWN-6).
- **positive_tests:** n/a — denial clause; acceptance is covered by SGO-SPAWN-001.
- **negative_tests:** S05, S19; fixture:lex-bare-go, fixture:spawn-go-in-dependency, fixture:spawn-go-unreachable.
- **implementation_coverage:** gate level only — `sgo/scripts/sgo-target` denies every program (W00); compiler rejection is W03.
- **executed_evidence:** W00 check `legacy Go program with bare go is denied` passed on darwin/arm64, linux/amd64, linux/arm64 (gate level).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-SPAWN-006 — Value-returning tasks
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-SPAWN-003, SGO-SPAWN-004, SGO-CHAN-002, SGO-COMP-004, SGO-RES-003, SGO-WAIT-003; D161, D168.
- **grammar_and_types:** `sgo.Go(n *sgo.Nursery, f func(*sgo.Context) (T, error)) *sgo.Task[T]`, with `T` inferred or written; `h.Wait(t *sgo.Context) (T, error)`; `h.Result() <-chan sgo.Result[T]`, where `sgo.Result[T]` has fields `Value T` and `Err error`. `T` must be a channel element type (SGO-CHAN-002, SGO-COMP-004) that is not task-local (SGO-COMP-004): the result passes from the child to its readers with no send site to copy at, so a slice, map, or pointer result is returned as `sgo.Frozen[T]` (SGO-COMP-009) or copied into a value type (D172). Resource handles are not task-local and are moved into `n` (D158).
- **preconditions:** As `n.Spawn` (SGO-SPAWN-003): the caller can spawn into `n`; `f` is not nil.
- **evaluation_and_value_semantics:** `sgo.Go` starts `f` as a child of `n` exactly as `n.Spawn` does and returns its handle. When `f` returns `(v, err)`, the result is published once. If `err` is nil and `v` is a resource, `v` is first moved into `n` (SGO-RES-003); a failed move becomes the result's error. `h.Wait(t)` waits for the result and returns it. Each `h.Result()` call returns a new channel that receives the result once it is published, for a select arm. Any number of reads, by any task of the nursery, see the same result.
- **ownership_and_lifecycle:** The child is a registered child of `n`: the nursery joins it like any other. The handle `*sgo.Task[T]` is not task-local (SGO-COMP-004): any task may capture it, and a reader may wait on it after the nursery has ended. When `T` is a resource type (SGO-RES-001), the handle and every result read from it (`Wait`, `Result`) are tied to `n`'s extent like the resource, which `Go` moved into `n` (SGO-RES-003); a child that ended without returning delivers its cancellation or failure as the result's error.
- **effect_and_capability_requirements:** n/a — as `n.Spawn`.
- **concurrency_and_memory_order:** Publishing the result happens-before every read that returns it.
- **allowed_nondeterminism:** As `n.Spawn`.
- **failure_and_partial_progress:** The child's error is data: a child that returns a non-nil error through `sgo.Go` ends normally and does not fail the nursery. A child that ends without returning (it is cancelled, or it fails) publishes a result whose error is its cancellation error (`sgo.IsCancelled` holds) or a `TaskFailed` error; a failure still fails the nursery as SGO-SPAWN-004 says.
- **cancellation_and_cleanup:** `Wait` and a receive from `Result()` are channel receives: a cancellation of a scope linked to the reader unwinds it (SGO-NATIVE).
- **compile_link_checks:** `SGO_E_TASK_LOCAL_CAPTURE` if `f` captures a task-local variable; `SGO_E_UNADMITTED` if `T` is not a channel element type, or is task-local; `SGO_E_RESOURCE_ESCAPES` if a handle with a resource `T`, or a result read from it, flows beyond `n`'s extent.
- **runtime_guards:** Spawn contract checks; the result is published once.
- **tcb_assumptions:** As SGO-SPAWN-003; the native channel send protocol (SGO-CHAN-002).
- **positive_tests:** fixture:task-value-wait, fixture:task-error-is-data, fixture:task-select-arm, fixture:task-cancelled-result, fixture:task-handle-shared, fixture:task-result-reader-cancelled.
- **negative_tests:** fixture:task-local-capture-denied, fixture:group-slice-result-denied, fixture:task-resource-result-escape.
- **implementation_coverage:** W22: `src/sgo/task.go`, `src/runtime/sgo_taskres.go`, `src/cmd/compile/internal/types2/sgo_check.go`, `src/cmd/compile/internal/types2/sgo_local.go`, `src/cmd/compile/internal/sgoadmit/admit_expr.go`.
- **executed_evidence:** conformance fixtures (darwin/arm64); Linux pending.
- **supported_execution_configurations:** production and lab.

### SGO-SPAWN-007 — Groups
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-SPAWN-006, SGO-SPAWN-001, SGO-NUR-004, SGO-CHAN-002, SGO-COMP-004, SGO-COMP-009, SGO-CANCEL-002, SGO-LAB-004, SGO-DEF-003; D172.
- **grammar_and_types:** `sgo.NewGroup[T](n *sgo.Nursery, limit int) *sgo.Group[T]`; `g.Go(t *sgo.Context, f func(*sgo.Context) (T, error))`; `g.Next() <-chan sgo.Result[T]`. `T` is restricted as in SGO-SPAWN-006: a channel element type that is not task-local. A slice result is returned as `sgo.Frozen[[]E]`.
- **preconditions:** `n` is non-nil and `limit ≥ 0` (0: no limit). For `g.Go`, `t` is the calling task's own context (as SGO-CANCEL-002), the calling task runs inside `n` (as `n.Spawn`, SGO-SPAWN-001), and `f` is non-nil.
- **evaluation_and_value_semantics:** `g.Go` first waits while `limit` children started by `g` are running, then starts `f` as a child of `n` exactly as `sgo.Go` does (SGO-SPAWN-006): a resource result with a nil error is moved into `n`; a child that ends without returning yields a result whose error is its cancellation or failure. A child of the group counts as running until it has published its result. `g.Next()` returns the group's one result channel: each receive on it, in a select arm or alone, takes the oldest result not yet received, in the order the children ended. Each result is received exactly once.
- **ownership_and_lifecycle:** The group's children are children of `n`: joined and cancelled with it, and a child's error result is data, not a failure of `n` (as SGO-SPAWN-006). Results are queued without bound until received; results never received are discarded when the group is no longer reachable. The group handle `*sgo.Group[T]` holds no runtime resource and is not task-local (SGO-COMP-004): any task inside `n` may capture it and call `g.Go` or receive on `g.Next()`.
- **effect_and_capability_requirements:** n/a — no external effect; the children's own effects need their own grants.
- **concurrency_and_memory_order:** A child's publication of its result happens before the receive that takes it. A child never waits to publish: an owner that stops reading cannot hold up its children or the nursery's join. At most `limit` children of the group run at once.
- **allowed_nondeterminism:** The order of results is the order the children end, which depends on scheduling in production; in the lab it is decided by the schedule and virtual time (SGO-LAB-004). Which waiting `g.Go` proceeds when a slot frees follows the channel wait protocol.
- **failure_and_partial_progress:** `limit < 0` fails with `InvalidArgument`. A nil `n` or `f`, or context misuse, fails with `SpawnContractViolation`. Starting into an ended nursery fails as `n.Spawn` does.
- **cancellation_and_cleanup:** The wait in `g.Go` is a cancellation point of `t`: if a scope linked to `t` is cancelled first, `g.Go` unwinds without starting `f`. A receive on `g.Next()` is a channel wait and a cancellation point.
- **compile_link_checks:** `SGO_E_UNADMITTED` if `T` is not a channel element type, or is task-local; `SGO_E_TASK_LOCAL_CAPTURE` if `f` captures a task-local variable (as SGO-SPAWN-006).
- **runtime_guards:** Argument and ownership checks; result delivery and refill under the result channel's lock; the limit counted by a native channel used as a semaphore.
- **tcb_assumptions:** The native channel wait protocol (SGO-NATIVE) and task spawning (SGO-SPAWN-001).
- **positive_tests:** fixture:lab-group-order, fixture:group-unread-results, fixture:group-select-next, fixture:group-frozen-slice.
- **negative_tests:** fixture:group-slice-result-denied, fixture:group-negative-limit, fixture:group-go-wait-cancelled.
- **implementation_coverage:** W22: `src/sgo/group.go`, `src/sgo/task.go`, `src/runtime/sgo_group.go`, `src/runtime/sgo_native_ops.go`, `src/runtime/sgo_chan.go`, `src/cmd/compile/internal/types2/sgo_check.go`, `src/cmd/compile/internal/sgoadmit/admit_expr.go`.
- **executed_evidence:** conformance fixtures (darwin/arm64); Linux pending.
- **supported_execution_configurations:** production and lab.

### SGO-SPAWN-008 — Request/reply calls (subset 3)
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-SPAWN-006, SGO-NATIVE-001, SGO-RES-001, SGO-RES-002, SGO-RES-003, SGO-CANCEL-002; D167, D174.
- **grammar_and_types:** `sgo.NewCall[Req, Resp](n *sgo.Nursery) *sgo.Call[Req, Resp]`; `(*sgo.Call[Req, Resp]).Call(t *sgo.Context, req Req) (Resp, error)`; `(*sgo.Call[Req, Resp]).Accept(t *sgo.Context) *sgo.Request[Req, Resp]`; `(*sgo.Request[Req, Resp]).Value() Req`; `(*sgo.Request[Req, Resp]).Reply(resp Resp, err error)`; `sgo.IsNoReply(err error) bool`. `*sgo.Call[Req, Resp]` is a shareable handle (D169); `*sgo.Request[Req, Resp]` is a resource handle (SGO-RES-001). `Req` and `Resp` follow the rule of `sgo.Go` results (SGO-SPAWN-006): no task-local types; and neither may hold a resource.
- **preconditions:** `Call` and `Accept` run in a task inside the nursery `n` of `NewCall`; else `MissingCapability`. `n` is not nil.
- **evaluation_and_value_semantics:** `Call` hands `req` to a server by rendezvous and returns the `resp` and `err` its server passes to `Reply`. `Accept` returns the next request handed over. Each request is answered at most once.
- **ownership_and_lifecycle:** An accepted request is a resource of the accepting task's body: tasks inside that body's scope may use it (SGO-RES-002), and it may not outlive it (SGO-RES-003). When the body ends with the request unanswered, its release answers it with the error `NoReply`.
- **effect_and_capability_requirements:** The call point handle; no ambient authority.
- **concurrency_and_memory_order:** The hand-over of a request happens before `Accept` returns it; `Reply` happens before the caller's `Call` returns.
- **allowed_nondeterminism:** Which waiting caller's request a server accepts first.
- **failure_and_partial_progress:** A caller whose server's handler ends without `Reply` gets `NoReply` (`sgo.IsNoReply`). A second `Reply` is a `SpawnContractViolation`.
- **cancellation_and_cleanup:** Both waits of `Call`, and `Accept`, are native channel waits (SGO-NATIVE): a waiting task in a cancelled scope unwinds. A caller cancelled before its request was accepted withdraws it: no server receives it. A caller cancelled after acceptance unwinds; the reply, when it comes, is dropped.
- **compile_link_checks:** `SGO_E_UNADMITTED` for a task-local or resource-holding `Req` or `Resp`; `SGO_E_RESOURCE_ESCAPES` for an accepted request that may outlive the accepting body.
- **runtime_guards:** The scope check of `Call` and `Accept`; the answered flag of a request.
- **tcb_assumptions:** `src/runtime/sgo_call.go`, `sgo_taskres.go`; `src/sgo/call.go`.
- **positive_tests:** fixture:call-basic, fixture:lab-call-withdraw; the model `sgo/model/callmodel.go` (CallExplore).
- **negative_tests:** fixture:call-request-escape, fixture:call-resource-denied; the model's seeded bugs (no NoReply release, late withdrawal, misrouted reply, a second Reply delivering).
- **implementation_coverage:** W23: runtime `src/runtime/sgo_call.go`; API `src/sgo/call.go`; compiler `sgoadmit/admit_expr.go`, `types2/sgo_check.go`, `sgo.go`, `sgo_values.go`, `sgo_resflow.go`.
- **executed_evidence:** pending (W23).
- **supported_execution_configurations:** production; lab.
