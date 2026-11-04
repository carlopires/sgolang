# W23 drafts — owed for the 0.18.0 freeze

Plan: `sgo/docs/w22-w23-plan.md` (W23: concurrency library). Decisions: D164 (via D173), D166 (landed
as D172 in W22), D167.

## New clauses (draft)

- **SGO-FILE-004 — Whole-message reads** (D164, D173): `ReadLine`, `ReadAll`, `sgo.IsTooLong`; the
  handle's read turn and read buffer. Cases: file-read-lines, lab-net-read-line.

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
