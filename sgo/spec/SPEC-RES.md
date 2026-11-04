# SPEC-RES — Structured resources

Resources belong to scopes, not to tasks or variables (D147–D158). A scope is the body of a nursery statement or of a task; it releases its resources when it ends, after its children have ended, in the reverse order of their acquisition. Tasks inside a scope share its resources. A resource never outlives the name of its scope: the compiler checks every flow (`SGO_E_RESOURCE_ESCAPES`), and `MoveTo` is the one way a resource leaves a nested scope. User types become resources with `s.Own(r)` and a `Release(ok bool) error` method, which commits or rolls back. Hand-off between tasks uses only scopes, `MoveTo`, and channels (D158): there is no slot or lending construct.

API at 0.16.0: a method on a scope acquires into that scope (`root.Listen`, `n.Dial`, `n.Dialer`); every other acquiring call takes the scope first (`l.Accept(s)`, `d.Dial(s, addr)`, `dir.Open(s, path)`, `dir.Create(s, path)`). `sgo.Scope` is implemented by `*sgo.Context` and `*sgo.Nursery`. `LendTo`, `HandleAbandoned`, and `sgo.ConnSlot` are removed. `Dir` is a capability, not a resource: it is never released, and every task that can name it may use it.

### SGO-RES-001 — Scopes own resources
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-NUR-001, SGO-NUR-004, SGO-SPAWN-004, SGO-FILE-003, SGO-NET-002, SGO-NET-003, SGO-RES-002, SGO-RES-003, SGO-RES-004; D148, D149, D153, D155; model `sgo/model/scoperes.go`.
- **grammar_and_types:** A *scope* is the body of a nursery statement, named by its nursery `n`, or the body of a task, named by its context (`root`, or the parameter of a spawned callback). `sgo.Scope` is the interface both implement. A *resource* is a value of a built-in resource type (`*sgo.File`, `*sgo.Listener`, `*sgo.Conn`, `*sgo.Dialer`) or a user resource (SGO-RES-004). A *resource type* is one of these; a struct, array, or slice type holding one; or a channel, `sgo.Channel`, or `sgo.SendPermit` type whose element holds one (SGO-RES-005).
- **preconditions:** An acquiring call runs in a task inside the named scope (the scope or a scope nested in it), and the scope has not ended.
- **evaluation_and_value_semantics:** Every resource is acquired into exactly one scope, named by the call: a method of a scope acquires into it, and every other acquiring call takes the scope as its first argument. Scopes are nested lexically: a nursery statement's body is inside the scope its statement runs in, and a spawned task's body is inside the nursery it was spawned into. A nursery statement ends in three steps: (1) join its children (SGO-NUR-001); (2) release its resources, in the reverse order of their acquisition or of their `MoveTo` into it; (3) complete with the outcome of SGO-NUR-004. A task body releases its resources in the same order after its function returns or unwinds, and before its outcome exists. Releasing a built-in resource closes it; releasing a user resource calls its `Release` (SGO-RES-004). An explicit `Close` releases a resource early and removes it from its scope.
- **ownership_and_lifecycle:** A resource belongs to its scope, not to a task or a variable: copies of a resource value name the same resource. It is released exactly once, by `Close` or by its scope's end.
- **effect_and_capability_requirements:** Acquiring needs the authority of the call (grants, and the root task for root-only calls); releasing needs none.
- **concurrency_and_memory_order:** The release of a scope happens after every task that ran inside it has ended (step 1) and before the outcome is published. The effects of a release are visible to the code after the nursery statement and to the joiner of a task.
- **allowed_nondeterminism:** none added.
- **failure_and_partial_progress:** A failed release is never dropped (D149). In a nursery whose body and children all succeeded, the nursery unwinds with kind `ReleaseError`, whose message is that of the first failed release in release order; the later ones join its chain. Otherwise each failed release is appended to the chain of the primary outcome. In a task body: a task whose function returned nil ends `Errored` with the first failed release, and the others join its chain; otherwise all join the chain. Releases continue after a failed one.
- **cancellation_and_cleanup:** A scope releases its resources whether it ends normally, by failure, or by cancellation. Releases are not cancellation points; they are bounded by the drain budget (SPEC-DRAIN, D155).
- **compile_link_checks:** `SGO_E_RESOURCE_ESCAPES` (SGO-RES-003). An acquiring call whose scope argument is not a scope is a type error.
- **runtime_guards:** Per-scope resource lists in acquisition order. The scope check at acquisition: a task outside the named scope, or a nursery that has begun releasing, fails the task with `MissingCapability`.
- **tcb_assumptions:** `src/runtime/sgo_res.go`, `sgo_scope.go`, `sgo_nursery.go`, `sgo_io.go`.
- **positive_tests:** fixture:res-release-at-nursery-end, fixture:res-release-at-task-end, fixture:res-release-lifo, fixture:res-release-after-join, fixture:res-close-early, fixture:res-release-on-cancel, fixture:res-struct-holds; TestScopes (model).
- **negative_tests:** fixture:res-release-error-nursery, fixture:res-release-error-task, fixture:res-release-error-chain, fixture:res-release-on-unwind; TestScopesMutations (model).
- **implementation_coverage:** W21: runtime `src/runtime/sgo_res.go`; API `src/sgo/io.go`.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab (releases are lab boundaries, as closes are).

### SGO-RES-002 — Sharing a resource within its scope
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-RES-001, SGO-NET-002, SGO-FILE-003, SGO-CANCEL-002; D150.
- **grammar_and_types:** There is no `LendTo`: the methods are removed from every handle type and from `*sgo.Dir`.
- **preconditions:** The caller runs inside the resource's scope.
- **evaluation_and_value_semantics:** Every task running inside a resource's scope may use it: the task that acquired it, and every task spawned, at any depth, into a nursery inside the scope. Concurrent operations on a socket admit one waiting reader and one waiting writer; other tasks wait their turn. Operations on a file are serialized. `Close` by any task inside the scope releases the resource early, unless another task is waiting in an operation on it.
- **ownership_and_lifecycle:** as SGO-RES-001.
- **effect_and_capability_requirements:** Using a resource needs no grant beyond the one it was acquired with.
- **concurrency_and_memory_order:** Each operation is atomic with respect to `Close`. A `Close` that succeeds happens before every later operation on the resource, and each of those fails with `HandleClosed`.
- **allowed_nondeterminism:** which waiting task is served first.
- **failure_and_partial_progress:** `ResourceBusy` (no effect) for a `Close` while another task waits on the resource; `HandleClosed` for any use after a `Close`. A task outside the scope cannot name the resource (SGO-RES-003); were it to reach one, its use would fail with `MissingCapability` (runtime guard).
- **cancellation_and_cleanup:** A waiting operation is a cancellation point of the waiting task only.
- **compile_link_checks:** `LendTo` is undefined (a type error).
- **runtime_guards:** the scope check on every operation; the busy check on `Close`.
- **tcb_assumptions:** `src/runtime/sgo_io.go`, `sgo_net.go`.
- **positive_tests:** fixture:res-share-child-uses, fixture:res-share-nested-nursery, fixture:res-share-two-readers.
- **negative_tests:** fixture:res-close-busy, fixture:res-use-after-close.
- **implementation_coverage:** W21: runtime `src/runtime/sgo_io.go`, `sgo_net.go`.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab.

### SGO-RES-003 — A resource never outlives the name of its scope; `MoveTo`
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-RES-001, SGO-RES-005, SGO-COMP-004, SGO-COMP-009, SGO-CHAN-001, SGO-MEM-001, SGO-ERR-004; D151, D158.
- **grammar_and_types:** Every expression of a resource type is *tied* to a scope name: the scope argument (or receiver) of the call that acquired it, the scope it was moved to (`r.MoveTo(s)` on built-in resources, `sgo.MoveTo(r, s)` for any resource), the scope that owns it (`s.Own(r)`), or, for a parameter or result, a scope of the caller (a function's resource parameters and results are tied to scopes its caller is in; two parameters' scopes are not comparable). A composite value holding no resource is tied to no scope.
- **preconditions:** For a move to `s`: the caller runs inside the resource's scope, and `s` encloses that scope.
- **evaluation_and_value_semantics:** The static rule: an expression tied to scope name `s` may flow only within the lexical extent of `s`, which is the nursery statement of `n`, the function literal of a callback's context, or the function, for a scope parameter or `root`. Denied: (1) assignment to a variable declared outside that extent; (2) `return` from inside it, unless the scope belongs to the function's caller; (3) capture by a function literal, unless the literal is called in place (including by `defer`) or is the callback of `Spawn`, `SpawnWithout`, or `Start` inside the extent; (4) a send on a channel that may outlive the scope (SGO-RES-005); (5) `sgo.Copy` of a value holding a resource. A move takes the resource from its scope's list to the end of `s`'s list, so `s` releases it; it is the only way a resource leaves a nested scope, and it re-ties the moved variable to `s` from that point on.
- **ownership_and_lifecycle:** After a move, the resource belongs to `s`. Tasks inside the old scope but outside `s` cannot name it after their scope ends, because the static rule bounds them.
- **effect_and_capability_requirements:** none.
- **concurrency_and_memory_order:** A move happens before the end of the old scope's release step. Two moves of different resources are independent.
- **allowed_nondeterminism:** none.
- **failure_and_partial_progress:** `MissingCapability` (no effect) if `s` does not enclose the resource's scope, or if the caller is outside the resource's scope; `HandleClosed` if the resource was closed.
- **cancellation_and_cleanup:** A move is not a cancellation point.
- **compile_link_checks:** `SGO_E_RESOURCE_ESCAPES` at the flowing expression, for every denied flow. The analysis is conservative: a variable's contents are tied as narrowly as anything stored in it, and a channel as widely as any channel stored in it, computed to a fixpoint over the function.
- **runtime_guards:** the enclosing-scope check of every move.
- **tcb_assumptions:** the flow pass `src/cmd/compile/internal/types2/sgo_resflow.go`; `src/runtime/sgo_res.go`.
- **positive_tests:** fixture:res-move-to-nursery, fixture:res-move-from-child, fixture:res-struct-holds, fixture:res-return-error-path, fixture:res-user-move; examples/happyeyeballs.
- **negative_tests:** fixture:res-escape-assign, fixture:res-escape-return, fixture:res-escape-capture, fixture:res-user-escape, fixture:res-move-inward; TestScopesMutations (model).
- **implementation_coverage:** W21: compiler `src/cmd/compile/internal/types2/sgo_resflow.go`; runtime `src/runtime/sgo_res.go`.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab.

### SGO-RES-004 — User resources: `Own` and `Release`
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-RES-001, SGO-RES-003, SGO-DEF-004, SGO-IFACE-002; D152, D155, D158.
- **grammar_and_types:** `sgo.Releaser` is the interface `Release(ok bool) error`. `s.Own(r)` takes a value implementing it and ties `r` to `s`. A user resource is found by identity, so it is a pointer.
- **preconditions:** The caller runs inside `s`; `r` is not a resource of any scope.
- **evaluation_and_value_semantics:** `Own` appends `r` to `s`'s list as an acquisition. When `s` releases it, `r.Release(ok)` runs in the task ending the scope, with `ok` true when the scope ends normally and false when it is unwinding (a failed child, a failure, or cancellation), as known after the join. Strict reverse order means that resources acquired after `r` are released before it, and those acquired before it after it, so a `Release` finds the resources acquired before its `Own` still open, and may use them: the releasing task counts as inside the scope until its release step ends. A user resource has no `Close` of its own; a method that releases early must make `Release` a no-op afterwards.
- **ownership_and_lifecycle:** as SGO-RES-001. A user resource's fields may hold built-in resources, which their own scopes release, or refer to resources of enclosing scopes (a lease).
- **effect_and_capability_requirements:** those of the `Release` body.
- **concurrency_and_memory_order:** `Release` runs after the join; no other task inside the scope runs. Tasks sharing a scope share its user resources, and a user resource serializes its own methods (D158): the runtime adds no lock.
- **allowed_nondeterminism:** none.
- **failure_and_partial_progress:** An error returned by `Release` is a failed release (SGO-RES-001). A panic in `Release` is a failure (SGO-DEF-004). A second `Own` of a value that is a resource of a scope fails the task with `InvalidArgument`.
- **cancellation_and_cleanup:** `Release` is not cancellable; it is bounded by the drain budget (D155).
- **compile_link_checks:** `Own` of a value whose type lacks `Release(bool) error` is a type error (`SGO_E_TYPE_MISMATCH`); `SGO_E_RESOURCE_ESCAPES` applies to user resource types from their `Own` on.
- **runtime_guards:** the identity check against double registration; the scope check of SGO-RES-001.
- **tcb_assumptions:** `src/runtime/sgo_res.go`.
- **positive_tests:** fixture:res-user-commit-rollback, fixture:res-own-order, fixture:res-user-move, fixture:res-pool-lease.
- **negative_tests:** fixture:res-own-no-release, fixture:res-own-twice, fixture:res-release-error-nursery, fixture:res-own-release-panic.
- **implementation_coverage:** W21: runtime `src/runtime/sgo_res.go`; API `src/sgo/io.go`.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab.

### SGO-RES-005 — Hand-off within a scope: channels and the deciding scope
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-RES-001, SGO-RES-002, SGO-RES-003, SGO-CHAN-001, SGO-PERMIT-001, SGO-NET-003; D158.
- **grammar_and_types:** A native channel, `*sgo.Channel[E]`, or `*sgo.SendPermit[E]` whose element type `E` holds a resource is a resource type: it is tied to the extent where it was made (`make` or `sgo.NewChannel`), and a permit is tied as its channel is.
- **preconditions:** For a send of `v`: the channel's scope lies within `v`'s scope.
- **evaluation_and_value_semantics:** A value tied to scope `s` may be sent (`ch <- v`, or `p.Commit(v)` on a permit) only on a channel that cannot outlive `s`; a received value is tied as its channel is. This admits the idiom for choosing among concurrent candidates: acquire every candidate into the scope that decides (for a race, the race nursery), report each to its body on a channel made in that scope, move the chosen one outward with a move (SGO-RES-003), and let the scope's end release the others.
- **ownership_and_lifecycle:** A send transfers nothing: the value stays a resource of its scope, whichever task receives it.
- **effect_and_capability_requirements:** none.
- **concurrency_and_memory_order:** as SGO-CHAN-001 and SGO-PERMIT-001.
- **allowed_nondeterminism:** which candidate reports first.
- **failure_and_partial_progress:** as the channel operations.
- **cancellation_and_cleanup:** Candidates that lose are released when the deciding scope ends, also when it is cancelled.
- **compile_link_checks:** `SGO_E_RESOURCE_ESCAPES` for a send on a channel that may outlive the value's scope. A channel parameter's lifetime is not comparable with another parameter's, so a helper cannot forward a resource from one parameter to a channel received as another.
- **runtime_guards:** none (static).
- **tcb_assumptions:** the flow pass `src/cmd/compile/internal/types2/sgo_resflow.go`.
- **positive_tests:** fixture:res-send-within-scope, fixture:res-pipeline-files, fixture:net-dialer-race; examples/happyeyeballs (real run and every lab scenario).
- **negative_tests:** fixture:res-send-outlives.
- **implementation_coverage:** W21: compiler `src/cmd/compile/internal/types2/sgo_resflow.go`.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab.
