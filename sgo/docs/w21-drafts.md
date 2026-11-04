# W21 clause drafts — structured resources and `check` (spec 0.16.0)

Decisions D147–D156; plan `sgo/docs/structured-resources-plan.md`. The model
`sgo/model/scoperes.go` explores 416 programs (77,736 states) and checks the five properties that
SGO-RES-001..004 rely on; 9 seeded protocol bugs are each caught with a schedule.

These drafts are for approval. On approval they move into `sgo/spec/` and freeze at 0.16.0.

## API at 0.16.0 (acquiring calls name their scope)

A method on a scope acquires into that scope. Every other acquiring call takes the scope first.
`sgo.Scope` is implemented by `*sgo.Context` and `*sgo.Nursery`.

| 0.15.1 | 0.16.0 |
|---|---|
| `root.Listen(addr) (*Listener, error)` | unchanged (into `root`'s body); `n.Listen(addr)` into `n` (root's nurseries) |
| `root.Dial(addr) (*Conn, error)` | unchanged; `n.Dial(addr)` |
| `root.Dialer(eps) *Dialer` | unchanged; `n.Dialer(eps)` |
| `l.Accept() (*Conn, error)` | `l.Accept(s Scope) (*Conn, error)` |
| `d.Dial(addr) (*Conn, error)` | `d.Dial(s Scope, addr) (*Conn, error)` |
| `dir.Open(path)` / `dir.Create(path)` | `dir.Open(s, path)` / `dir.Create(s, path)` |
| `x.LendTo(n)` (every handle, `Dir`) | removed (D150) |
| `root.NewConnSlot()`, `ConnSlot` | removed (D153); use `MoveTo` |
| — | `r.MoveTo(s Scope)` on every built-in resource (D151) |
| — | `s.Own(r)` for user resources (D152) |
| `x.Close() error` | unchanged (optional early release) |

`Dir` is a capability, not a resource. It is not released and is never lent: a `Dir` is usable
by every task that can name it.

---

### SGO-ERR-004 — `check`: checked propagation
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-LEX-003, SGO-CTL-001, SGO-CTL-004, SGO-BIND-002, SGO-VAL-005, SGO-DEF-001, SGO-TYPE-006, SGO-NUR-004; D147, D156.
- **grammar_and_types:** `check` is a reserved word. Two forms, each with a single call `c` whose last result has type `error`: the statement `check c`, and the whole right side of an assignment or short declaration, `x₁, …, xₙ = check c` or `x₁, …, xₙ := check c`, whose left side binds the results of `c` except the last (count and types as for an assignment of those results; `_` admitted per position). The enclosing function or function literal has a last result of type `error`.
- **preconditions:** n/a: a static rule.
- **evaluation_and_value_semantics:** `c` is evaluated once (SGO-VAL-005). Let `e` be its last result. If `e` is nil, the targets are assigned the other results and execution continues; the statement form discards them. Otherwise no target is assigned, and the enclosing function returns as the statement `return z₁, …, zₘ₋₁, e` would at that point, where each `zᵢ` is the zero value of the `i`th result type. That includes crossing nursery statements (SGO-CTL-004), assigning named results, and running deferred calls. A function with a result of a resource type (SGO-RES-001) returns only `e` on this path; its other results are never observed (SGO-RES-003).
- **ownership_and_lifecycle:** none of its own.
- **effect_and_capability_requirements:** those of `c`.
- **concurrency_and_memory_order:** none of its own; the implied return synchronizes as a written return does.
- **allowed_nondeterminism:** none.
- **failure_and_partial_progress:** as the implied `return`. A cancellation error propagates unchanged: `IsCancelled` still holds for the caller.
- **cancellation_and_cleanup:** none of its own; `c`'s cancellation points are unchanged.
- **compile_link_checks:** `SGO_E_RESERVED_WORD` (`check` as a name); `SGO_E_CHECK_NOT_ERROR` (the last result of `c` is not `error`, or `c` is not a call); `SGO_E_CHECK_NO_RESULT` (the enclosing function has no trailing `error` result); `SGO_E_UNADMITTED` (any other position: an operand, a condition, an argument, a `return` operand, an `if`, `for`, or `switch` init statement).
- **runtime_guards:** none: lowered to the comparison and the return.
- **tcb_assumptions:** the lowering in the noder.
- **positive_tests:** fixture:err-check-value, fixture:err-check-multi, fixture:err-check-statement, fixture:err-check-assign-on-success, fixture:err-check-in-nursery, fixture:err-check-in-spawn, fixture:err-check-cancellation.
- **negative_tests:** fixture:err-check-not-error, fixture:err-check-no-result, fixture:err-check-in-expression, fixture:err-check-in-condition, fixture:err-check-in-init, fixture:err-check-reserved-word.
- **implementation_coverage:** W21.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab.

### SGO-RES-001 — Scopes own resources
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-NUR-001, SGO-NUR-004, SGO-SPAWN-004, SGO-FILE-003, SGO-NET-002, SGO-NET-003, SGO-RES-002, SGO-RES-003, SGO-RES-004; D148, D149, D153; model `sgo/model/scoperes.go`.
- **grammar_and_types:** A *scope* is the body of a nursery statement, named by its nursery `n`, or the body of a task, named by its context (`root`, or the parameter of a spawned callback). `sgo.Scope` is the interface both implement. A *resource* is a value of a built-in resource type (`*sgo.File`, `*sgo.Listener`, `*sgo.Conn`, `*sgo.Dialer`) or a user resource (SGO-RES-004). A *resource type* is one of these, or a struct, array, or slice type holding one, or a function type, if the function literal captures one.
- **preconditions:** An acquiring call runs in a task inside the named scope (the scope or a scope nested in it), and the scope has not ended.
- **evaluation_and_value_semantics:** Every resource is acquired into exactly one scope, named by the call: a method of a scope acquires into it, and every other acquiring call takes the scope as its first argument. Scopes are nested lexically: a nursery statement's body is inside the scope its statement runs in, and a spawned task's body is inside the nursery it was spawned into. A nursery statement ends in three steps:
  1. join its children (SGO-NUR-001);
  2. release its resources, in the reverse order of their acquisition (or of their `MoveTo` into it);
  3. complete with the outcome of SGO-NUR-004 v2.

  A task body releases its resources in the same order after its function returns or unwinds, and before its outcome exists. Releasing a built-in resource closes it; releasing a user resource calls its `Release` (SGO-RES-004). An explicit `Close` releases a resource early and removes it from its scope.
- **ownership_and_lifecycle:** A resource belongs to its scope, not to a task or a variable: copies of a resource value name the same resource. It is released exactly once, by `Close` or by its scope's end.
- **effect_and_capability_requirements:** Acquiring needs the authority of the call (grants, the root task for root-only calls); release needs none.
- **concurrency_and_memory_order:** The release of a scope happens after every task that ran inside it has ended (step 1), and before the outcome is published. Effects of a release are visible to the code after the nursery statement and to the joiner of a task.
- **allowed_nondeterminism:** none added.
- **failure_and_partial_progress:** A failed release is never dropped (D149). In a nursery: if the body and every child succeeded, the nursery unwinds with kind `ReleaseError`, whose message is the first failed release's in release order; otherwise each failed release is appended to the chain of the primary outcome. In a task body: a task whose function returned nil ends `Errored` with the first failed release, and the others join its chain; otherwise all join the chain. Releases continue after a failed one.
- **cancellation_and_cleanup:** A scope releases its resources whether it ends normally, by failure, or by cancellation. Releases are not cancellation points; they are bounded by the drain budget (SPEC-DRAIN, D155).
- **compile_link_checks:** `SGO_E_RESOURCE_ESCAPES` (SGO-RES-003). An acquiring call whose scope argument is not a scope is a type error.
- **runtime_guards:** Per-scope resource lists in acquisition order. The scope check at acquisition: a task outside the named scope, or a scope that has ended, fails the task with `MissingCapability`.
- **tcb_assumptions:** `src/runtime/sgo_res.go`, `sgo_scope.go`, `sgo_io.go`.
- **positive_tests:** fixture:res-release-at-nursery-end, fixture:res-release-at-task-end, fixture:res-release-lifo, fixture:res-release-after-join, fixture:res-close-early, fixture:res-release-on-unwind, fixture:res-release-on-cancel; model TestScopes.
- **negative_tests:** fixture:res-release-error-nursery, fixture:res-release-error-task, fixture:res-release-error-chain, fixture:res-acquire-ended-scope; model TestScopesMutations.
- **implementation_coverage:** W21.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab (releases are lab boundaries, as closes are).

### SGO-RES-002 — Sharing a resource within its scope
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-RES-001, SGO-NET-002, SGO-FILE-003, SGO-CANCEL-002; D150.
- **grammar_and_types:** No `LendTo`: the methods are removed from every handle type and from `*sgo.Dir`.
- **preconditions:** The caller runs inside the resource's scope.
- **evaluation_and_value_semantics:** Every task running inside a resource's scope may use it: the task that acquired it, and every task spawned, at any depth, into a nursery inside the scope. Concurrent operations on a socket admit one waiting reader and one waiting writer; other tasks wait their turn. Operations on a file are serialized. `Close` by any task inside the scope releases the resource early, unless another task is waiting in an operation on it.
- **ownership_and_lifecycle:** as SGO-RES-001.
- **effect_and_capability_requirements:** Using a resource needs no grant beyond the one it was acquired with.
- **concurrency_and_memory_order:** Each operation is atomic with respect to `Close`. A `Close` that succeeds happens before every later operation on the resource, and each of those fails with `HandleClosed`.
- **allowed_nondeterminism:** which waiting task is served first.
- **failure_and_partial_progress:** `ResourceBusy` (no effect) for a `Close` while another task waits on the resource; `HandleClosed` for any use after a `Close`. A task outside the scope cannot name the resource (SGO-RES-003); if one did, its use would fail with `MissingCapability` (runtime guard).
- **cancellation_and_cleanup:** A waiting operation is a cancellation point of the waiting task only.
- **compile_link_checks:** `LendTo` is undefined (a type error).
- **runtime_guards:** the scope check on every operation; the busy check on `Close`.
- **tcb_assumptions:** `src/runtime/sgo_io.go`, `sgo_net.go`.
- **positive_tests:** fixture:res-share-child-uses, fixture:res-share-nested-nursery, fixture:res-share-two-readers.
- **negative_tests:** fixture:res-close-busy, fixture:res-use-after-close.
- **implementation_coverage:** W21.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab.

### SGO-RES-003 — A resource never outlives the name of its scope; `MoveTo`
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-RES-001, SGO-COMP-004, SGO-COMP-009, SGO-CHAN-001, SGO-MEM-001; D151.
- **grammar_and_types:** Every expression of a resource type is *tied* to a scope name: the scope argument (or receiver) of the call that acquired it, the scope it was moved to, or, for a parameter or result, the caller's (a function's resource parameters and results are tied to the scope its caller is in). `r.MoveTo(s)` re-ties `r` to `s`.
- **preconditions:** For `r.MoveTo(s)`: the caller runs inside `r`'s scope, and `s` encloses that scope.
- **evaluation_and_value_semantics:** The static rule: an expression tied to scope name `s` may flow only within the lexical extent of `s`. That extent is the nursery statement of `n`, the function literal of a callback's `t`, or the function, for a scope parameter or `root`. The following are denied:
  - assignment to a variable declared outside that extent;
  - `return` from inside it, unless the function's caller is in the scope (a parameter scope);
  - capture by a function literal that is not spawned inside it, or that escapes it;
  - a send on a channel, or `sgo.Copy`, of a resource type.

  `MoveTo` moves the resource from its scope's list to the end of `s`'s list, so `s` releases it. It is the only way a resource leaves a nested scope.
- **ownership_and_lifecycle:** After `MoveTo`, the resource belongs to `s`. Tasks inside the old scope but outside `s` cannot name it after their scope ends, because the static rule bounds them.
- **effect_and_capability_requirements:** none.
- **concurrency_and_memory_order:** `MoveTo` happens before the end of the old scope's release step. Two `MoveTo`s of different resources are independent.
- **allowed_nondeterminism:** none.
- **failure_and_partial_progress:** `MissingCapability` (no effect) if `s` does not enclose `r`'s scope, or if the caller is outside `r`'s scope; `HandleClosed` if `r` was closed.
- **cancellation_and_cleanup:** `MoveTo` is not a cancellation point.
- **compile_link_checks:** `SGO_E_RESOURCE_ESCAPES` for every denied flow, at the flowing expression.
- **runtime_guards:** the enclosing-scope check in `MoveTo`.
- **tcb_assumptions:** the flow pass `src/cmd/compile/internal/types2/sgo_flow.go` (resource ties); `src/runtime/sgo_res.go`.
- **positive_tests:** fixture:res-move-to-nursery, fixture:res-move-from-child, fixture:res-return-parameter-scope, fixture:res-struct-holds; examples/happyeyeballs.
- **negative_tests:** fixture:res-escape-assign, fixture:res-escape-return, fixture:res-escape-capture, fixture:res-escape-send, fixture:res-move-inward, fixture:res-move-sibling; model mutations move-any-scope, escape-unchecked.
- **implementation_coverage:** W21.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab.

### SGO-RES-004 — User resources: `Own` and `Release`
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-RES-001, SGO-RES-003, SGO-DEF-004, SGO-IFACE-002; D152, D155.
- **grammar_and_types:** A named type whose method set includes `Release(ok bool) error` is a user resource type. `s.Own(r)` takes a value of such a type; its result is `r`, tied to `s`.
- **preconditions:** The caller runs inside `s`; `r` is not already owned.
- **evaluation_and_value_semantics:** `Own` appends `r` to `s`'s list as an acquisition. When `s` releases it, `r.Release(ok)` runs in the task ending the scope, with `ok` true when the scope is ending normally, and false when it is unwinding (a failed child, a failure, or cancellation), as known after the join. Strict reverse order means that resources acquired after `r` are released before it, and those acquired before it after it. So a constructor that acquires handles and then calls `Own` finds them open in `Release`. A user resource has no `Close` of its own; a method that releases early must make `Release` a no-op afterwards.
- **ownership_and_lifecycle:** as SGO-RES-001; the resource's fields may hold built-in resources, which their own scope releases.
- **effect_and_capability_requirements:** those of the `Release` body.
- **concurrency_and_memory_order:** `Release` runs after the join; no other task inside the scope runs.
- **allowed_nondeterminism:** none.
- **failure_and_partial_progress:** An error returned by `Release` is a failed release (SGO-RES-001). A panic in `Release` is a failure (SGO-DEF-004); the remaining releases still run.
- **cancellation_and_cleanup:** `Release` is not cancellable; it is bounded by the drain budget (D155).
- **compile_link_checks:** `Own` of a type without `Release(bool) error` is a type error; `SGO_E_RESOURCE_ESCAPES` applies to user resource types.
- **runtime_guards:** a double `Own` fails the task with `InvalidArgument`.
- **tcb_assumptions:** `src/runtime/sgo_res.go`.
- **positive_tests:** fixture:res-own-commit, fixture:res-own-rollback-on-failure, fixture:res-own-rollback-on-cancel, fixture:res-own-order.
- **negative_tests:** fixture:res-own-no-release, fixture:res-own-twice, fixture:res-own-release-error, fixture:res-own-release-panic.
- **implementation_coverage:** W21.
- **executed_evidence:** `sgo/evidence/w21/README.md`.
- **supported_execution_configurations:** production; lab.

---

## Amendments

| clause | change |
|---|---|
| SGO-LEX-003 v+1 | `check` is a reserved word |
| SGO-CTL-001 v+1 | `check c` is a use of `c`'s error; the expression-statement denial names it |
| SGO-NUR-004 v2 | the release step: the outcome order gains, between "a child errored" and "complete normally", "a release failed: unwind with `ReleaseError`"; failed releases are appended to the chain otherwise |
| SGO-SPAWN-004 v+1 | a task's outcome includes its body's releases (a nil return with a failed release is `Errored`) |
| SGO-FILE-003 v2 | ownership text replaced by SGO-RES-001/002; `HandleAbandoned` removed; `Open`/`Create` take a scope |
| SGO-NET-002 v3 | `Accept` takes a scope; `LendTo` removed |
| SGO-NET-003 v3 | `Dialer.Dial` takes a scope; `n.Dialer`; `LendTo` removed |
| SGO-NET-004 | superseded by SGO-RES-003 (`MoveTo`) |
| SGO-DEF-001 | unchanged (`n.Defer` stays denied; D154) |

## Fixture changes

The fixtures that expect `HandleAbandoned` now expect a clean exit or `ReleaseError`. The
`LendTo` and `ConnSlot` fixtures are rewritten with `MoveTo`, or removed with their clause. Every
example is rewritten (S5).
