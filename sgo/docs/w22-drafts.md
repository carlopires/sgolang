# W22 clause drafts

Drafts for the W22 spec pass (freeze 0.17.0). Each one replaces a field of a frozen clause
and bumps its version. They are not in `sgo/spec/` yet, so the identity digest has not moved.

## SGO-CTL-004 v2 — Exits that cross nurseries (D159)

Implemented in `711ea7b88c`. Fixtures: `exit-check-cancels`, `exit-return-error-cancels`,
`exit-bare-return-cancels`, `exit-nil-return-joins`.

- **evaluation_and_value_semantics:** For `return e1, …, en`: evaluate the operands once, left
  to right, and hold the values. If the function's last result has type `error` and its held
  value (for a bare return, the named result) is not nil, cancel each crossed nursery from the
  innermost outward (SGO-CANCEL-001, cause `error exit by task N`). Then complete each crossed
  nursery from the innermost outward exactly as if its block had ended normally (SGO-NUR-003,
  SGO-NUR-004). Then run the function's deferred calls (SGO-DEF-001). Then return the held
  values. A failed `check` is such a return (SGO-ERR-004). For `break`/`continue` that leave
  nurseries: complete each crossed nursery innermost first, then transfer control. A `break` or
  `continue` whose target is inside the nursery does not complete it. Each execution of a
  nursery statement is a distinct scope with its own identity.
- **cancellation_and_cleanup:** An exit with a non-nil error cancels the nurseries it crosses
  before joining them. The cancellation is the owner's own, so it is consumed at each nursery's
  boundary (SGO-CANCEL-004): the nursery completes normally unless a child failed, and the
  returned error is unchanged. Every other exit (nil error, no error result, `break`,
  `continue`) joins without cancelling, and children still running keep the owner in JOINING.
- **positive_tests:** add fixture:exit-check-cancels, fixture:exit-return-error-cancels,
  fixture:exit-bare-return-cancels, fixture:exit-nil-return-joins.

## SGO-ERR-004 v2 — `check` (D159)

- **cancellation_and_cleanup:** add: "A failed `check` inside nurseries cancels the nurseries it
  crosses before joining them (SGO-CTL-004 v2)."
- **positive_tests:** add fixture:exit-check-cancels.

## Amendments owed at 0.17.0 (from the 0.16.2 erratum)

0.16.2 removed two mentions of W22 features from frozen clauses (review F1, F2). The 0.17.0
freeze re-adds them as clause versions:

- SGO-RES-003 v4: rule (3) — "…or the callback of `Spawn`, `SpawnWithout`, `Start`, `sgo.Go`, or
  `g.Go` inside the extent" (the escape pass already treats `sgo.Go` callbacks so, D161).
- SGO-RES-004 v4: `ok` is false on any cancellation, "an ancestor's or a deadline's (SGO-CANCEL-005)
  included".
- SGO-AUTH-001 and SGO-AUTH-003 new versions (D169, option a): remove "Every handle type
  `*sgo.K` has `h.LendTo(n *sgo.Nursery) *sgo.K` (revocable copy)", the `LendTo` precondition,
  "A lent handle is revoked when its nursery closes (A03)", and "a revoked handle" among the
  `MissingCapability` causes; add "Effect handles are shareable authority tokens: any task may
  hold one; authority is bounded by grants, ceilings, and attenuation, and is never revoked
  (D169)". fixture:auth-lend-revoked moves to negative tests only (a reject: `LendTo` is not
  admitted); acceptance case A03 is withdrawn.
- SGO-RES-003 v4 also lists fixture:res-defer-nursery-close (the file-based form of
  res-defer-nursery-capture, W21 errata follow-up) among its negative tests.

## SGO-WAIT-004 (D171)

Drafted in `sgo/spec/SPEC-WAIT.md` (status draft); freezes at 0.17.0.

## SGO-SPAWN-007 (D172) and SGO-SPAWN-006 amendment

Drafted in `sgo/spec/SPEC-SPAWN.md` (status draft); freeze at 0.17.0. SGO-SPAWN-006 now refuses a task-local result type (a slice result is returned as `sgo.Frozen`).

## Open before 0.17.0: deadlines in the lab (D170 follow-up)

`n.CancelAfter` ends a lab run with `UnsupportedLabEffect`. The obstacle: lab timers fire inside
`decide()`, run by the task holding the baton, and that task may be parking in a `select` with
the arms' channel locks held (`sgoLabBlock` is called before the park releases them), while
`sgoCancel` wakes waiters and takes those same locks. Options, to be settled with a model first
(D30):

1. **A lab driver goroutine.** When the next decision is an advance whose due timers include a
   deadline, `decide()` hands the baton to a runtime goroutine that is not a task. It holds no
   S-Go locks, runs `sgoCancel` for each due deadline (waking through `wakeLocked` as usual), then
   decides again and hands the baton to the chosen task. The trace records a `fire` per deadline,
   as for timers. Needs: a baton holder that is not an `sgoTask`, and the handoff in
   `sgoLabBlock`/`sgoLabPoint`/`sgoLabEndTask` to route through it.
2. **Deferred cancellation run by the next task.** The fire records the deadline as pending; the
   chosen task runs pending cancellations when it resumes, before its wait returns. Fails when no
   task is ready after the advance (everything is blocked on the deadline itself), unless a task
   is woken spuriously, which every wait protocol would have to tolerate.
3. **Park order.** Call `sgoLabBlock` after the wait's locks are released, so `decide()` never
   runs with channel locks held. Simplest code change, but it reopens the lost-wakeup window the
   current order closes, so it needs the model to show it safe.

Option 1 is the likely choice: it keeps every wait protocol unchanged.

**Done (2026-11-05): option 1.** `sgoLabDriver` (src/runtime/sgo_deadline.go), a lab-only runtime service (OWN-6: `lab-driver`, dormant in production). Fixtures lab-deadline-join and lab-deadline-select; production and lab agree on both. Still owed: the lab model (`sgo/model`) has no nurseries or cancellation, so this is pinned by fixtures and traces, not yet by a model (D30).
