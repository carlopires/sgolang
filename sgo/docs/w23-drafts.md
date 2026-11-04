# W23 drafts — owed for the 0.18.0 freeze

Plan: `sgo/docs/w22-w23-plan.md` (W23: concurrency library). Decisions: D164 (via D173), D166 (landed
as D172 in W22), D167.

## New clauses (draft)

- **SGO-FILE-004 — Whole-message reads** (D164, D173): `ReadLine`, `ReadAll`, `sgo.IsTooLong`; the
  handle's read turn and read buffer. Cases: file-read-lines, lab-net-read-line.

- **SGO-SPAWN-008 — Request/reply calls** (D167, D174): `sgo.NewCall[Req, Resp](n)`, `Call`,
  `Accept`, `Request.Value`, `Request.Reply`, `sgo.IsNoReply`; rendezvous hand-over (withdrawal by
  cancellation before acceptance), an accepted request as a resource of the accepting body
  (`NoReply` when it ends unanswered). Model `sgo/model/callmodel.go`. Cases: call-basic,
  lab-call-withdraw, call-request-escape, call-resource-denied. Deferred: acceptance through a
  `select` arm.

- **SGO-SPAWN-009** (D175): pools with leases. `sgo.NewPool[T](n)`, `Put`, `Acquire(s)`,
  `Lease.Value`/`Return`/`Drop`; a lease is a resource of `s`, which lies inside the pool's
  nursery; `Put` is checked as a send. Cases: pool-leases, pool-resource-items, pool-lease-outlives,
  pool-put-outlives, pool-value-escape, pool-lease-after-return, pool-acquire-outside; model
  `poolmodel.go` (1 422 states, four mutants).

- **SGO-RES-006 (new, D176)**: resource interfaces. An interface embedding `sgo.Resource` is a
  resource type; conversion to it ties as the source; no conversion or assertion from a resource
  to a non-resource interface; values task-local; constructor results tied as narrowly as their
  resource arguments jointly; results of calls through a resource interface tied to the receiver.
  Predeclared `sgo.Reader`, `sgo.LineReader`, `sgo.Writer`. Amends SGO-RES-003 v? (Z1 holds for
  non-resource interfaces) and SGO-IFACE (conversion and assertion rules).

## Amendments owed at 0.18.0 (frozen clauses; a new version each)

- **SGO-SPAWN-006 v2** (W22 review G2, H1): each `h.Result()` call returns its own channel, which
  delivers the task's result once (edge-triggered): a `select` loop that keeps one channel takes
  that arm once and then waits on its other arms, so the 0.17.0 meaning of an arm on `h.Result()`
  is unchanged. A call registers nothing: a result channel is listed on its task only while a
  receiver is parked on it, so abandoned channels are garbage and the list is bounded by the
  receivers parked now. Receivers parked before the end are all woken, one per channel. After its
  value, a `Result()` channel never becomes ready again: a plain receive on it is an ordinary
  wait, ended by cancellation or reported by `LabDeadlock`. `Wait` uses a fresh channel per call. A level-triggered latch was tried at `69eaccd6f3` and withdrawn:
  S-Go has no nil channel to switch an arm off, so every select loop over it spun (review H1).
  Cases: lab-task-result-once; runtime lane TestSGoTaskResult (1 000 abandoned select arms leave
  nothing listed; five parked receivers; a kept channel delivers once while a timer arm keeps
  firing; the mutant that never marks a channel taken fails it).
- **SGO-SPAWN-006 / SGO-SPAWN-007 positive and negative tests** (W22 review G1): `*sgo.Task[T]` and
  `*sgo.Group[T]` written in types (parameters, channel elements, declarations); a resource-delivering
  handle parameter is tied like any resource. Cases: spawn-task-param, spawn-task-param-escape.
- **SGO-FILE-004** (W22 review H2, H3): a task holding the read turn is waiting on the handle, so
  `Close` is `ResourceBusy` while a `ReadLine`/`ReadAll` runs; the handle is checked again once the
  turn is taken. An unterminated last line of exactly `max` bytes is a line; one more byte is
  `TooLong`. Cases: lab-net-readline-close, file-read-lines; runtime lane TestSGoReadLineClose (a
  Close from a test hook between two reads of one ReadLine is ResourceBusy; the mutant without the
  read-turn check fails it).
- **SGO-FILE-003** concurrency text: reads of one handle now run one at a time (the read turn of
  SGO-FILE-004); writes keep the per-call serialization.

## Amendments owed at 0.18.0 from the 0.17.0 review (`.work/sgo-017-issues.md`)

- **SGO-SPAWN-006 v2 / SGO-SPAWN-007 v2, not admitted** (SGO17-01, fixed at `663cccb977`): a
  value-returning start (`sgo.Go`, `Group.Go`) into a nursery that is CANCELLING, itself or through
  an ancestor, admits no child and never runs `f`; its handle resolves at once, and exactly once, to
  the scope's cancellation error (`IsCancelled`; `IsDeadline` when the cause is a deadline, local or
  inherited). The admission decision and the outcome are one step under the scope lock, so a
  start racing a `Cancel` either runs `f` or resolves to the cancellation, never neither. A group's
  slot is released by that publication, so later submissions do not wait. 0.17.0 published nothing:
  `Wait` deadlocked and the slot leaked. Case: spawn-go-not-admitted.
- **SGO-SPAWN-006 v2, aggregate results** (SGO17-02, fixed with this slice): on a nil error, every
  resource of the child's body that the result holds is moved into `n`: the result itself, or the
  fields and elements of a struct or array result, at any depth. The walk follows the result's type
  and stays inside the value: a pointer is recognized (a built-in handle by its type, a user
  resource by its registration) and never followed, so channels, task and group handles and other
  references are not traversed. Each resource moves once (a handle held twice moves once), in
  acquisition order, so the nursery's reverse-acquisition release order holds. A resource that is
  not in the child's body (already in `n`, in an enclosing scope, or released) stays where it is:
  ownership never moves inward. A handle-typed field cannot be nil (SGO-TYPE-005), so there is no
  nil case. A move cannot fail at that point (the child runs inside its body, `n` encloses it and is
  not releasing while the child is live); were it to fail, its error would be the result's and the
  resources already moved stay in `n`. Case: task-aggregate-resource-result.
- **SGO-SPAWN-006 v2, `Release(ok)` of a `Go` child's body** (SGO17-04, contract kept): an error
  returned through `Go` is data. The child's body ends normally, so its own resources are released
  with `Release(true)`. A body that needs rollback on its error owns the resource in an inner
  nursery: the failing `check` crosses that nursery, cancels it (D159), and its resources see
  `Release(false)`, while the caller still receives the error as the result. Case to add:
  task-error-rollback-inner-nursery.
- **SGO-CTL-004 v2** (SGO17-03): v1 says "A normal exit joins; it never cancels" and "Exits do
  not cancel", which contradicts D159 as implemented since W22. v2: a `return` whose error result
  is not nil, and a failing `check` (through SGO-ERR-004), cancel every nursery they cross before
  joining it; the cancellation is consumed at each crossed nursery's boundary (SGO-CANCEL-004), the
  held return values are returned unchanged, and resources of a crossed nursery see
  `Release(false)`. A normal exit (end of body, `return` with a nil error, `break`, `continue`)
  joins without cancelling. Cases: exit-check-cancels, exit-return-error-cancels,
  exit-bare-return-cancels, exit-nil-return-joins; to add: exit-check-rollback (a crossed nursery's
  user resource sees `Release(false)` and the error propagates unchanged).
- **SGO-ERR-004 v3** (SGO17-03): "crossing nursery statements (SGO-CTL-004)" names the v2 error
  exit: a failing `check` cancels each nursery it crosses.
- **SGO-RES-004 v5** (SGO17-06): "Tasks sharing a scope share its user resources" overstates the
  admitted model: a pointer to a user resource is task-local (SGO-COMP-004), so only the task that
  holds it, and the scope's release, use it. Registration with `Own` gives lifetime, not sharing or
  synchronization; implementing `Release` grants no exception to the capture rules. Case to
  keep/add: res-user-capture-denied.
- **Documentation** (SGO17-05, SGO17-07): API comments and the user guide on `Spawn` versus `Go`
  errors; a child's cancellation result versus cancellation of its reader; `Result()` (one channel
  per call; store it for a select loop) versus `Next()` (one consuming queue); `After` versus a
  scope timer; a group's limit bounds running children, not queued results; deadlines are
  inherited and cannot be extended. `GUARANTEES.md` and the roadmap regenerated, with a
  non-mutating drift check.
