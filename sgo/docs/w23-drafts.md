# W23 drafts — owed for the 0.18.0 freeze

Plan: `sgo/docs/w22-w23-plan.md` (W23: concurrency library). Decisions: D164 (via D173), D166 (landed
as D172 in W22), D167.

## New clauses (draft)

- **SGO-FILE-004 — Whole-message reads** (D164, D173): `ReadLine`, `ReadAll`, `sgo.IsTooLong`; the
  handle's read turn and read buffer. Cases: file-read-lines, lab-net-read-line.

## Amendments owed at 0.18.0 (frozen clauses; a new version each)

- **SGO-SPAWN-006 v2** (W22 review G2): `h.Result()` returns the task's one result channel, the
  same for every call. When the child ends, the runtime latches the result: every receive, from
  any reader, any number of times, gets it without consuming it, and a `select` arm on it proceeds
  on every iteration after the end (level-triggered). Receivers parked before the end are all woken
  with it. `Wait` receives from the same channel. Cases: lab-task-result-latch; runtime lane
  TestSGoTaskResultLatch (10 000 `Result` calls make no allocation; five parked receivers).
- **SGO-SPAWN-006 / SGO-SPAWN-007 positive and negative tests** (W22 review G1): `*sgo.Task[T]` and
  `*sgo.Group[T]` written in types (parameters, channel elements, declarations); a resource-delivering
  handle parameter is tied like any resource. Cases: spawn-task-param, spawn-task-param-escape.
- **SGO-FILE-003** concurrency text: reads of one handle now run one at a time (the read turn of
  SGO-FILE-004); writes keep the per-call serialization.
