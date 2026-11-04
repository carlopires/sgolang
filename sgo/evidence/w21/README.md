# W21 evidence — structured resources and `check` (spec 0.16.0)

Decisions D147–D156 and D158 (D157 withdrawn). Clauses frozen at 0.16.0: SGO-ERR-004, SGO-RES-001..005;
amended SGO-LEX-003 v2, SGO-CTL-001 v3, SGO-NUR-004 v2, SGO-SPAWN-004 v2, SGO-FILE-002 v4,
SGO-FILE-003 v2, SGO-NET-002 v3, SGO-NET-003 v3; superseded SGO-NET-004 v2.

## What was built

| Part | Where |
|---|---|
| `check` keyword, parsing, typing, lowering, `sgo fmt` | `src/cmd/compile/internal/syntax`, `types2/sgo_check.go`, `noder/writer.go`, `reader.go`, `src/cmd/sgo/fmt.go` |
| Scope resource lists, release after join in reverse order, release errors, `MoveTo`, `Own`, `sgo.MoveTo` | `src/runtime/sgo_res.go`, `sgo_nursery.go`, `sgo_scope.go`, `sgo_io.go` |
| Resource flow rule `SGO_E_RESOURCE_ESCAPES` (value and destination ties to a fixpoint, parameter pseudo-scopes, channels within their scope) | `src/cmd/compile/internal/types2/sgo_resflow.go` |
| Error path of resource results (`return nil, err`) | `types2/sgo.go`, `stmt.go`, `check.go` |
| API: acquiring calls name their scope; `LendTo`, `ConnSlot`, `HandleAbandoned` removed | `src/sgo/io.go` |
| Model of scope resources (416 programs, 77,736 states; 9 seeded bugs caught) | `sgo/model/scoperes*.go` |

## Executed on darwin/arm64 (2026-11-05)

- Conformance: every case, including the 37 `res-*` cases (release order, release after the join, on
  cancel and on unwind, release errors and chains, sharing, `Close` busy and early, moves, escapes by
  assignment, return, capture, and send, channels within a scope, user resources commit/rollback,
  move, order, panic, double `Own`, missing `Release`, pipeline of files, pool lease, error-path
  returns), the 13 `err-check-*` cases, and `net-dialer-race` rewritten as an ordinary nursery.
- Examples: pipeline, service, replay, and happyeyeballs (rewritten without a slot: attempts dial
  into the race nursery, the body moves the winner out, the nursery releases the losers) —
  `sgo check`, `sgo test`, `sgo run`, `sgo lab sweep` over every lab scenario.
- Runtime lane (`TestSGo*`), model tests (`TestScopes`, `TestScopesMutations`), linux/amd64 and
  linux/arm64 vet of `runtime` and `sgo`.
- Gates: spec-tool, test-identity, test-tcb, test-modules, generated-twin-check, legacy-audit,
  lock-alloc-check, inventory and policy reports.

## Runtime defects found by the W21 fixtures and fixed

- `MoveTo` into a task body dereferenced a nil nursery (`res-send-within-scope`).
- A cancelled nursery released with `ok = true`, committing instead of rolling back (`res-user-commit-rollback`).
- A `Release` could not use the resources acquired before it, because the joined nursery had left
  the task's scope stack before its release step (`res-own-order`).

## Known limits (documented in SGO-RES-003/005)

- The flow analysis is conservative: a send of a resource must be written where its channel is made;
  a helper cannot forward a resource from one parameter to a channel received as another.
- A user resource is found by identity, so it is a pointer.

## Linux

sgo-w00 run 41828619012 on `main` (commit `a1b82ddf8a`, which includes the 0.16.0 freeze, the
Happy Eyeballs review fixes, and the three new lab scenarios): passed on linux/amd64 and
linux/arm64 — conformance, sgo check/build agreement, negative-fixture honesty, identity, TCB,
cross-package nurseries (X10), sgo fmt, examples with every lab scenario, and the runtime lane.
sgo-runtime and sgo-spec passed on the same commits.

The first sgo-w00 run of the freeze (41822765026, commit `2c85b0e7f2`) failed on both runners at
X10: its test program named a function `check`, a reserved word since 0.16.0. Fixed in
`66d2589ec5` (the program was not an S-Go conformance case, so the local suite had not run it).
