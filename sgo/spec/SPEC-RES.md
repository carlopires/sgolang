# SPEC-RES — Structured resources

Resources belong to scopes, not to tasks or variables (D147–D158). A scope is the body of a nursery statement or of a task; it releases its resources when it ends, after its children have ended, in the reverse order of their acquisition. Tasks inside a scope share its resources. A resource never outlives the name of its scope: the compiler checks every flow (`SGO_E_RESOURCE_ESCAPES`), and `MoveTo` is the one way a resource leaves a nested scope. User types become resources with `s.Own(r)` and a `Release(ok bool) error` method, which commits or rolls back. Hand-off between tasks uses only scopes, `MoveTo`, and channels (D158): there is no slot or lending construct.

API at 0.16.0: a method on a scope acquires into that scope (`root.Listen`, `n.Dial`, `n.Dialer`); every other acquiring call takes the scope first (`l.Accept(s)`, `d.Dial(s, addr)`, `dir.Open(s, path)`, `dir.Create(s, path)`). `sgo.Scope` is implemented by `*sgo.Context` and `*sgo.Nursery`. `LendTo`, `HandleAbandoned`, and `sgo.ConnSlot` are removed. `Dir` is a capability, not a resource: it is never released, and every task that can name it may use it.

### SGO-RES-001 — Scopes own resources
- **version:** 4
- **status:** frozen
- **reference_closure:** SGO-NUR-001, SGO-NUR-004, SGO-SPAWN-004, SGO-FILE-003, SGO-NET-002, SGO-NET-003, SGO-RES-002, SGO-RES-003, SGO-RES-004, SGO-RES-006; D148, D149, D153, D155; model `sgo/model/scoperes.go`.
- **grammar_and_types:** A *scope* is the body of a nursery statement, named by its nursery `n`, or the body of a task, named by its context (`root`, or the parameter of a spawned callback). `sgo.Scope` is the interface both implement. A *resource* is a value of a built-in resource type (`*sgo.File`, `*sgo.Listener`, `*sgo.Conn`, `*sgo.Dialer`, `*sgo.Timer`) or a user resource (SGO-RES-004); a value of a resource interface type (SGO-RES-006) holds one. A *resource type* is one of these; a struct, array, or slice type holding one; or a channel, `sgo.Channel`, or `sgo.SendPermit` type whose element holds one (SGO-RES-005). A pointer to, or a map whose keys or values are, a value of a resource type is itself a resource type. A value of a resource type is never converted to an interface type that is not a resource interface (`SGO_E_RESOURCE_ESCAPES` at the conversion), except as an argument of package `sgo` (`Own`, `MoveTo`): such an interface value is not tied to a scope (W21 review Z1). A conversion to a resource interface ties the interface value as its source (SGO-RES-006).
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
- **negative_tests:** fixture:res-release-error-nursery, fixture:res-release-error-task, fixture:res-release-error-chain, fixture:res-release-on-unwind; TestScopesMutations (model). Also fixture:res-escape-any.
- **implementation_coverage:** W21: runtime `src/runtime/sgo_res.go`; API `src/sgo/io.go`.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab (releases are lab boundaries, as closes are).

### SGO-RES-002 — Sharing a resource within its scope
- **version:** 5
- **status:** frozen
- **reference_closure:** SGO-RES-001, SGO-NET-002, SGO-FILE-003, SGO-WAIT-004, SGO-CANCEL-002; D150.
- **grammar_and_types:** There is no `LendTo` on resources: the methods are removed from every resource handle type and from `*sgo.Dir`. Effect handles (SPEC-AUTH) have no `LendTo` either: they are shareable authority tokens, never revoked (D169).
- **preconditions:** The caller runs inside the resource's scope.
- **evaluation_and_value_semantics:** Every task running inside a built-in resource's scope (`*sgo.File`, `*sgo.Listener`, `*sgo.Conn`, `*sgo.Dialer`, `*sgo.Timer`; SGO-RES-001) may use it: the task that acquired it, and every task spawned, at any depth, into a nursery inside the scope. A user resource (SGO-RES-004) is task-local (SGO-COMP-004): only the task that owns it, or the task its result was moved to (SGO-SPAWN-006), uses it; it reaches another task only by a move. Shared mutable state between tasks is a server task with request/reply calls (SGO-SPAWN-008) or channels of copied values. Concurrent operations on a socket admit one waiting reader and one waiting writer; other tasks wait their turn. Operations on a file are serialized. `Close` by any task inside the scope releases the resource early, unless another task is waiting in an operation on it.
- **ownership_and_lifecycle:** as SGO-RES-001.
- **effect_and_capability_requirements:** Using a resource needs no grant beyond the one it was acquired with.
- **concurrency_and_memory_order:** Each operation is atomic with respect to `Close`. A `Close` that succeeds happens before every later operation on the resource, and each of those fails with `HandleClosed`.
- **allowed_nondeterminism:** which waiting task is served first.
- **failure_and_partial_progress:** `ResourceBusy` (no effect) for a `Close` while another task waits on the resource; `HandleClosed` for any use after a `Close`. A task outside the scope cannot name the resource (SGO-RES-003); were it to reach one, its use would fail with `MissingCapability` (runtime guard).
- **cancellation_and_cleanup:** A waiting operation is a cancellation point of the waiting task only.
- **compile_link_checks:** `LendTo` is undefined (a type error).
- **runtime_guards:** the scope check on every operation; the busy check on `Close`.
- **tcb_assumptions:** `src/runtime/sgo_io.go`, `sgo_net.go`.
- **positive_tests:** fixture:res-share-child-uses, fixture:res-share-nested-nursery, fixture:res-share-two-readers, fixture:res-user-via-call, fixture:timer-scope-child.
- **negative_tests:** fixture:res-close-busy, fixture:res-use-after-close, fixture:res-user-send-in-scope, fixture:res-user-capture-denied.
- **implementation_coverage:** W21: runtime `src/runtime/sgo_io.go`, `sgo_net.go`.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab.

### SGO-RES-003 — A resource never outlives the name of its scope; `MoveTo`
- **version:** 4
- **status:** frozen
- **reference_closure:** SGO-RES-001, SGO-RES-005, SGO-COMP-004, SGO-COMP-009, SGO-CHAN-001, SGO-MEM-001, SGO-ERR-004; D151, D158.
- **grammar_and_types:** Every expression of a resource type is *tied* to a scope name: the scope argument (or receiver) of the call that acquired it, the scope it was moved to (`r.MoveTo(s)` on built-in resources, `sgo.MoveTo(r, s)` for any resource), the scope that owns it (`s.Own(r)`), or, for a parameter or result, a scope of the caller (a function's resource parameters and results are tied to scopes its caller is in; two parameters' scopes are not comparable). A composite value holding no resource is tied to no scope.
- **preconditions:** For a move to `s`: the caller runs inside the resource's scope, and `s` encloses that scope.
- **evaluation_and_value_semantics:** The static rule: an expression tied to scope name `s` may flow only within the lexical extent of `s`, which is the nursery statement of `n`, the function literal of a callback's context, or the function, for a scope parameter or `root`. Denied: (1) assignment to a variable declared outside that extent; (2) `return` from inside it, unless the scope belongs to the function's caller; (3) capture by a function literal, unless the literal is called in place, is deferred while the extent is the deferring function's own (a task body or a scope parameter), or is the callback of `Spawn`, `SpawnWithout`, `Start`, `sgo.Go`, or `g.Go` inside the extent; (4) a send on a channel that may outlive the scope (SGO-RES-005); (5) `sgo.Copy` of a value holding a resource. A move takes the resource from its scope's list to the end of `s`'s list, so `s` releases it; it is the only way a resource leaves a nested scope, and it re-ties the moved variable to `s` from that point on. A deferred call inside a nursery statement may not mention a resource tied to that nursery or to a scope nested inside the function: it runs when the function returns, after the nursery released the resource. A store whose target is reached through a pointer, a slice, or a map (`p.f = r`, `s[i] = r`, `*p = r`, `x.p.f = r`) is checked against the extent from which that memory may be reached, not against the declaration of the root variable: for a single step through the root, the extent of the root's top-level pointee; for a deeper path, of any memory reachable through the root. An allocation (`&T{}`, `new`, `make`, a slice or map literal) is reachable from the extent that makes it; an assignment of a reference value makes the target's extent reach the source's memory, and aliases share it; parameters and receivers reach the caller's memory, so storing one parameter's resource into another parameter's or the receiver's memory is an escape. Memory of unknown origin (a call result, a package-level variable) may be reached from anywhere, so a store through it (`get(p).f = r`, `p.self().f = r`) admits only a value tied to no scope. A value of a resource type counts as tied to the scope that declares its variable even before it holds an owned resource (W21 review Z1, Z3).
- **ownership_and_lifecycle:** After a move, the resource belongs to `s`. Tasks inside the old scope but outside `s` cannot name it after their scope ends, because the static rule bounds them.
- **effect_and_capability_requirements:** none.
- **concurrency_and_memory_order:** A move happens before the end of the old scope's release step. Two moves of different resources are independent.
- **allowed_nondeterminism:** none.
- **failure_and_partial_progress:** `MissingCapability` (no effect) if `s` does not enclose the resource's scope, or if the caller is outside the resource's scope; `HandleClosed` if the resource was closed.
- **cancellation_and_cleanup:** A move is not a cancellation point.
- **compile_link_checks:** `SGO_E_RESOURCE_ESCAPES` at the flowing expression, for every denied flow. The analysis is conservative: a variable's contents are tied as narrowly as anything stored in it, and a channel as widely as any channel stored in it, computed to a fixpoint over the function.
- **runtime_guards:** the enclosing-scope check of every move.
- **tcb_assumptions:** the flow pass `src/cmd/compile/internal/types2/sgo_resflow.go`; `src/runtime/sgo_res.go`.
- **positive_tests:** fixture:res-move-to-nursery, fixture:res-move-from-child, fixture:res-struct-holds, fixture:res-return-error-path, fixture:res-user-move; examples/happyeyeballs. Also fixture:res-local-holder-ok, fixture:res-defer-task-scope, fixture:res-call-result-untied-ok.
- **negative_tests:** fixture:res-escape-assign, fixture:res-escape-return, fixture:res-escape-capture, fixture:res-user-escape, fixture:res-move-inward; TestScopesMutations (model). Also fixture:res-escape-alias, fixture:res-escape-addr, fixture:res-escape-param-store, fixture:res-escape-receiver-store, fixture:res-escape-call-result, fixture:res-escape-method-result, fixture:res-defer-nursery-capture, fixture:res-defer-nursery-close.
- **implementation_coverage:** W21: compiler `src/cmd/compile/internal/types2/sgo_resflow.go`; runtime `src/runtime/sgo_res.go`.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab.

### SGO-RES-004 — User resources: `Own` and `Release`
- **version:** 6
- **status:** frozen
- **reference_closure:** SGO-RES-001, SGO-RES-003, SGO-DEF-004, SGO-IFACE-002, SGO-COMP-004; D152, D155, D158.
- **grammar_and_types:** `sgo.Releaser` is the interface `Release(ok bool) error`. `s.Own(r)` takes a value implementing it and ties `r` to `s`. A user resource is found by identity, so it is a pointer.
- **preconditions:** The caller runs inside `s`; `r` is not a resource of any scope. The operand of `s.Own(x)` was made inside the extent of `s`: an allocation there, a call of a declared function or method (outside package `sgo`) whose arguments and receiver carry no reference — no pointer, slice, map, channel, interface, function value, or handle — and which therefore returns only memory it allocated (W23 review M4), or a local assigned only such values; `Own` of a value named outside the scope, or reachable from outer memory, is `SGO_E_RESOURCE_ESCAPES` at the operand (W21 review Z2).
- **evaluation_and_value_semantics:** `Own` appends `r` to `s`'s list as an acquisition. When `s` releases it, `r.Release(ok)` runs in the task ending the scope, with `ok` true when the scope ends normally and false when it is unwinding (a failed child, a failure, or cancellation), as known after the join. Strict reverse order means that resources acquired after `r` are released before it, and those acquired before it after it, so a `Release` finds the resources acquired before its `Own` still open, and may use them: the releasing task counts as inside the scope until its release step ends. A user resource has no `Close` of its own; a method that releases early must make `Release` a no-op afterwards.
- **ownership_and_lifecycle:** as SGO-RES-001, except for sharing: a user resource is task-local (SGO-COMP-004), so unlike a built-in resource (SGO-RES-002) the tasks inside its scope do not share it; only its owning task, or the task its result was moved to (SGO-SPAWN-006), uses it, and it crosses tasks only by a move. A user resource's fields may hold built-in resources, which their own scopes release, or refer to resources of enclosing scopes (a lease).
- **effect_and_capability_requirements:** those of the `Release` body.
- **concurrency_and_memory_order:** `Release` runs after the join; no other task inside the scope runs. A pointer to a user resource is task-local (SGO-COMP-004): only the task that holds it, and the scope's release, use it. Registration with `Own` gives lifetime, not sharing or synchronization, and implementing `Release` grants no exception to the capture rules; the runtime adds no lock.
- **allowed_nondeterminism:** none.
- **failure_and_partial_progress:** An error returned by `Release` is a failed release (SGO-RES-001). A panic in `Release` is a failure (SGO-DEF-004). A second `Own` of a value that is a resource of a scope fails the task with `InvalidArgument`.
- **cancellation_and_cleanup:** `Release` is not cancellable; it is bounded by the drain budget (D155). `ok` is false whenever the scope is cancelled, including by an ancestor's cancellation or a deadline (SGO-CANCEL-005), even if its body and every child end normally.
- **compile_link_checks:** `Own` of a value whose type lacks `Release(bool) error`, or is not a pointer type, is a type error (`SGO_E_TYPE_MISMATCH`); `SGO_E_RESOURCE_ESCAPES` applies to user resource types from their `Own` on.
- **runtime_guards:** the identity check against double registration; the scope check of SGO-RES-001.
- **tcb_assumptions:** `src/runtime/sgo_res.go`.
- **positive_tests:** fixture:res-user-commit-rollback, fixture:res-own-order, fixture:res-user-move, fixture:res-pool-lease, fixture:res-user-via-call. Also fixture:res-own-fresh, fixture:res-own-constructor.
- **negative_tests:** fixture:res-own-no-release, fixture:res-own-twice, fixture:res-release-error-nursery, fixture:res-own-release-panic. Also fixture:res-own-outer-var, fixture:res-own-outer-field, fixture:res-own-value-type, fixture:res-user-capture-denied, fixture:res-own-constructor-alias, fixture:res-user-send-in-scope.
- **implementation_coverage:** W21: runtime `src/runtime/sgo_res.go`; API `src/sgo/io.go`.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab.

### SGO-RES-005 — Hand-off within a scope: channels and the deciding scope
- **version:** 2
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
- **compile_link_checks:** `SGO_E_RESOURCE_ESCAPES` for a send on a channel that may outlive the value's scope. A channel parameter's lifetime is not comparable with another parameter's, so a helper cannot forward a resource from one parameter to a channel received as another. The supported composition: a helper returns the resource (SGO-RES-001 ties the result to its scope), and the caller, in the scope that owns both the resource and the channel, sends it (fixture:res-pipeline-files). Summaries of parameter lifetimes in export data, which would admit such helpers across packages, are proposed as D195.
- **runtime_guards:** none (static).
- **tcb_assumptions:** the flow pass `src/cmd/compile/internal/types2/sgo_resflow.go`.
- **positive_tests:** fixture:res-send-within-scope, fixture:res-pipeline-files, fixture:net-dialer-race; examples/happyeyeballs (real run and every lab scenario).
- **negative_tests:** fixture:res-send-outlives.
- **implementation_coverage:** W21: compiler `src/cmd/compile/internal/types2/sgo_resflow.go`.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab.

### SGO-RES-006 — Resource interfaces (subset 3)
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-RES-001, SGO-RES-002, SGO-RES-003, SGO-RES-005, SGO-IFACE-001, SGO-IFACE-002, SGO-COMP-004; D132, D176.
- **grammar_and_types:** `sgo.Resource` is an interface type with no methods. An interface type is a *resource interface* when it embeds `sgo.Resource`, directly or through an embedded resource interface. A resource interface is a resource type (SGO-RES-001). Every other interface, `any` and `error` included, is not, and never holds a resource.
- **preconditions:** n/a: static rules.
- **evaluation_and_value_semantics:** A value converts to a resource interface whose method set it has. The interface value holds the same value; a method call dispatches to the dynamic value's method. A resource, or a value of a resource interface, converts to no interface that is not a resource interface. A type assertion or type-switch case from a resource interface to an interface that is not a resource interface is refused; to a resource interface or a concrete type it is admitted. A nil resource interface value is admitted; a method call on it is `NilDereference`. `==` on resource interface values is refused (SGO-IFACE-001).
- **ownership_and_lifecycle:** A conversion moves nothing: a resource stays in its scope's list and is released once, by that scope (SGO-RES-001). A resource interface value is tied as what it holds: the converted value's tie, and for a value asserted from it, the interface value's tie; a resource-free value converts to an untied interface value. Every flow rule of resources applies (SGO-RES-003, SGO-RES-005). The result of a call of a function or method outside package `sgo` is tied to the innermost of the ties of its authority arguments — scope arguments, arguments holding resources (handles, user resources, resource interfaces, handles delivering resources, and aggregates or pointers of them), and the receiver — and to the caller's current extent if the callee or an argument is a function value or if two of the ties cannot be ordered. With no authority argument tied to a scope (none, or only untied ones such as standard output) and no function value, the result is untied: the callee holds no scope, scoped handle, or closure, and there are no package variables, so it can reach no scope's resource. A user resource is a value of a pointer type whose method set has `Release(bool) error` (SGO-RES-004); a value type is never one, wherever the rule asks whether a type holds a resource (the frozen rule, SGO-COMP-009, included).
- **effect_and_capability_requirements:** n/a: a conversion grants nothing; the dynamic value's own rules apply to each call.
- **concurrency_and_memory_order:** n/a: resource interface values are task-local (D132, SGO-COMP-004); a concrete handle crosses into a child and is converted there.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** As the dynamic value's methods.
- **cancellation_and_cleanup:** As the dynamic value's methods.
- **compile_link_checks:** `SGO_E_RESOURCE_ESCAPES` (a resource or resource interface value becoming another interface; an assertion to one; a resource interface value outliving its tie; `sgo.Copy`); `SGO_E_UNADMITTED` (in a frozen value, or as a map key); `SGO_E_TASK_LOCAL_CAPTURE` (capture by a child); `SGO_E_UNADMITTED` (`==`).
- **runtime_guards:** `NilDereference` for a method call on a nil value.
- **tcb_assumptions:** `src/cmd/compile/internal/types2/sgo_resflow.go` (`sgoResIface`, `callTie`), `sgo_local.go` (conversions and assertions).
- **positive_tests:** fixture:std-io-streams, fixture:res-iface-basic, fixture:res-iface-wrapper-outlives-inner, fixture:res-iface-narrow-assert, fixture:res-iface-nil-call.
- **negative_tests:** fixture:res-iface-escape, fixture:res-iface-to-any-denied, fixture:res-iface-assert-to-error-denied, fixture:res-iface-copy-denied, fixture:res-iface-capture-denied, fixture:res-iface-closure-callee, fixture:res-iface-wrapper-escape, fixture:res-iface-compare-denied, fixture:res-iface-freeze-denied, fixture:res-iface-map-key-denied, fixture:std-io-escape.
- **implementation_coverage:** W23: `src/sgo/io.go` (`Resource`); compiler `sgoadmit/admit_expr.go`, `types2/sgo_resflow.go`, `types2/sgo_local.go`.
- **executed_evidence:** conformance fixtures (darwin/arm64); Linux at the W23 merge.
- **supported_execution_configurations:** production; lab.
