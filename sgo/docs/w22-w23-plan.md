# W22 and W23 — a language derived from Happy Eyeballs

Decisions D159–D167 (approved 2026-11-05). Origin: the Happy Eyeballs review
(`.work/happy-eyeballs-review.md`, findings HE-01..06) showed that every bug of the example lived in
plumbing the language made the user write: timer tasks and their tick events, a hand-built event
channel, sticky permission flags, explicit cancellation on every error path, and hand-written
whole-message reads. The goal is a language in which that program, and backend programs with the
same shape, are short and read like their specification.

## Acceptance programs

| Work package | Acceptance |
|---|---|
| W22 | `sgo/examples/happyeyeballs` rewritten on D159–D162: no timer tasks, no event channel, no `mayStart`, no explicit `n.Cancel()` on error paths; every existing and late-answer lab scenario passes, plus the two cases the current version cannot test (an older attempt failing after a newer one started; two connections succeeding before selection). |
| W23 | An HTTP/1.1 server example: a nursery per connection and per request, request deadlines (D162/D165), bounded fan-out to backends (D166), a database-style pool with leases and transactions (D152), a multiplexed backend connection (D167), graceful shutdown. |

## W22 — language core

| Step | Content | Evidence |
|---|---|---|
| 1 | Model: timers as select arms with virtual time, task handles bound to their nursery, deadline inheritance, error exits cancelling crossed nurseries. Seeded bugs: a stale timer firing, a handle read after its nursery, an inner scope extending a deadline, a check that joins without cancelling. | `sgo/model`, mutations caught |
| 2 | D159: lowering of `check` and error returns across nurseries cancels before the join. Amend SGO-CTL-004, SGO-ERR-004. | fixtures: error exit with a blocked child ends at once; nil return still joins; nested nurseries; defers order |
| 3 | D160: `t.After(ms)` as a select arm; runtime timer arms; lab virtual time and tie-break. | fixtures and lab scenarios: timer vs ready channel ties, timers in loops, cancellation during a timer arm |
| 4 | D161: `n.Go` and `*sgo.Task[T]`, `NewGroup`; select arm on a task; the flow pass treats `<-h` as a receive (D158); child errors as data. | fixtures: value results, error results that do not fail the nursery, resource results tied to the nursery, handle escapes refused |
| 5 | D162, D165: `CancelAfter`, `IsDeadline`, `Deadline()`; inheritance and shortening only. | fixtures, lab scenarios on virtual time |
| 6 | Happy Eyeballs rewrite; README; the two cases the 0.16 version could not test. | test-examples, lab sweeps |
| 7 | Spec freeze 0.17.0; evidence; review. Merge `w22-core` into main only after the freeze: guarantee-check refuses draft clauses in guaranteed families (SGO-WAIT-003 is in SPEC-WAIT, covered by G-OWN), so every W22 draft must be frozen first. | guarantee-check, sgo-spec on the merge commit |

## W23 — concurrency library

| Step | Content |
|---|---|
| 1 | D164: `ReadAll`, `ReadLine` on `Conn` and `File`; a split-write lab peer. |
| 2 | D166: bounded groups. |
| 3 | D167: `sgo.Call[Req, Resp]`; model of withdrawal on cancellation. |
| 4 | `Reader`/`Writer` interfaces over the I/O types; `Pool[T]` with leases. |
| 5 | HTTP/1.1 parsing (library, S-Go source) and the server example. |
| 6 | Freeze 0.18.0; evidence; review. |

## Later

D163 (tagged unions with exhaustive switch), TLS and crypto, JSON, HTTP/2, randomized lab network
scenarios (`sgo lab fuzz`: generated answer delays, blackholes, refusals per family, with stated
properties), and the open question raised in W21: effect handles still have `LendTo` (SPEC-AUTH)
while SGO-RES-002 says `LendTo` is gone from every handle type — decide whether effect capabilities
also move to scope sharing, and fix the clause wording either way.
