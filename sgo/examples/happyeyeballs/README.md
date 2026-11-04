# Happy Eyeballs (RFC 8305-based) in S-Go

Happy Eyeballs is the example Nathaniel Smith uses in
[*Notes on structured concurrency*](https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/#nurseries-in-practice):
connect to a name that has IPv6 and IPv4 addresses, prefer IPv6, and never wait for a dead
family. It is a small algorithm with a lot of concurrent parts: two DNS queries, a resolution
timer, a growing set of connection attempts, one timer per attempt, and a winner. Getting it
right with free-running threads is notoriously hard: the losers must be stopped, their
connections closed, and nothing may outlive the call.

`he.Connect` (in `he/he.sgo`) does it in **one nursery**, the race, and one `select`:

| RFC 8305 | in `he.Connect` |
|---|---|
| §3 query AAAA and A at once; start on AAAA, or on A after a 50ms resolution delay | two tasks with results (`sgo.Go`) and a scope timer (`n.NewTimer`) |
| §4 interleave the families, IPv6 first | the nursery body picks the next address |
| §5 start the next attempt after 250ms, or at once when the latest attempt fails | a group of attempts (`sgo.NewGroup`) and one scope timer, re-armed per attempt |
| the first connection wins; the rest are abandoned | the body moves the first connection out, then cancels the race |

The nursery body is the only place that decides anything. It selects on the two query results,
the two timers, and the group's results, which arrive in completion order. A query result is a
`sgo.Frozen[[]string]`: the addresses cross from the query task to the body without a copy and
without sharing anything mutable. A timer is measured from when it was armed, not from each
`select`, so answers arriving in between do not stretch an RFC delay. Expected failures (a
refused address, a failed query) are results the body handles, not errors of the race.

### Who owns what (SGO-RES-001..005, SGO-WAIT-004, SGO-SPAWN-007)

1. The caller's scope owns the dialer (`root.Dialer`).
2. The queries, the attempts, and both timers belong to the race nursery, and every attempt dials
   **into the race nursery** (`d.Dial(n, addr)`): each candidate connection is a resource of the
   race.
3. An attempt's result carries a reference to its candidate; receiving it transfers nothing.
4. The body moves exactly one connection out, the winner, with `c.MoveTo(task)`.
5. Every exit from the race goes through one `n.Cancel()`: the remaining queries and attempts
   stop; the nursery joins them, disarms its timers, and releases every candidate still in it.

So two attempts that both connect are safe: only the selected one leaves the race, and the other
stays a resource of the race nursery, even if its result is never received. A child never waits to
publish its result (the group queues results), so an unread result cannot hold up the join. A
child waiting on a blackholed connection does wait until it is cancelled, which is why every exit
of the body, including a failed log write, cancels before it leaves the nursery, and why `main`
cancels its waiting server when the client fails.

### What S-Go guarantees here, and Go does not

* **Nothing outlives `Connect`.** It returns only after the race has joined every query, timer,
  and attempt. A forgotten attempt is not possible: there is no `go` statement.
* **Losing connections are released.** They belong to the race nursery, which releases them when
  it ends. No code closes them by hand, and none can keep one: the compiler refuses any flow of a
  candidate out of the race except the winner's explicit move (`SGO_E_RESOURCE_ESCAPES`).
* **Authority is explicit.** `sgo.mod` grants `connect example.test:443`. Only the root task holds
  the grant; it creates a `Dialer` for that name, which every task inside the root's scope may use.
  The dialer may dial only the addresses its own `Resolve` returned, and `root.Dial` refuses host
  names.
* **Cancellation is the timeout.** An attempt's connection timeout is its cancellation: a
  blackholed SYN costs nothing once the race is won.
* **The timing is testable.** `sgo lab` runs the race on virtual time against a scripted network
  (`lab/*/lab.net`), under many schedules. Each scenario checks the winner *and* the RFC step it
  is about, and the failing schedule of a broken rule replays exactly.

S-Go guarantees the ownership and cleanup. It does not make the coordinator's protocol logic
correct: the late-answer scenarios below were added after a review found that a timer firing
before the other family's answer lost its permission to start the next attempt.

## Run it

```
sgo run .                                         # real network: localhost, IPv4-only server
sgo lab sweep -seeds 0..199 ./lab/silent-v6       # IPv6 blackholed: IPv4 starts at 250ms and wins
sgo lab sweep -seeds 0..199 ./lab/slow-aaaa       # AAAA late: IPv4 starts after the 50ms delay
sgo lab sweep -seeds 0..199 ./lab/prefer-v6       # AAAA within 50ms: IPv6 first, and it wins
sgo lab sweep -seeds 0..199 ./lab/slow-v6         # IPv6 slower than IPv4's head start: IPv6 cancelled
sgo lab sweep -seeds 0..199 ./lab/all-refused     # everything refuses: an error, nothing left running
sgo lab sweep -seeds 0..199 ./lab/late-a-after-tick     # A answers after IPv6's 250ms: IPv4 starts at once
sgo lab sweep -seeds 0..199 ./lab/late-aaaa-after-tick  # AAAA answers after IPv4's 250ms: IPv6 starts at once
sgo lab sweep -seeds 0..199 ./lab/log-fails-pending     # the logger fails mid-race: pending attempts cancelled
sgo lab sweep -seeds 0..199 ./lab/older-fails-later     # IPv6 fails after IPv4 started: the race keeps IPv4
sgo lab sweep -seeds 0..199 ./lab/two-connected         # both connect before selection: one kept, one released
```

The real run resolves `localhost` from `/etc/hosts` (`::1`, then `127.0.0.1`). Other names go to
the resolv.conf servers over UDP. The server listens on IPv4 only, so the preferred `[::1]` is
refused and the race moves to IPv4 at once. `expected.txt` is that run on a host whose
`/etc/hosts` lists `::1 localhost`; where it lists only `127.0.0.1`, there is no IPv6 attempt and
the race is the IPv4 lines alone. The real run uses the fixed port 47970 (grants name exact ports);
the exact attempt order of every RFC rule is checked by the lab scenarios, which do not depend on
the host. The greeting is read until the peer closes: a `Read` may return fewer bytes than were
sent.

## Limits

This is an RFC 8305-based example, not a conforming implementation:

* No RFC 6724 destination ordering: addresses are tried in answer order, interleaved by family.
* After the latest attempt fails hard (refused), the next starts at once. RFC 8305 §5 asks for at
  least 10ms between attempt starts and makes no exception for hard failures; this example keeps
  the immediate fallback on purpose.
* No TCP fallback for truncated DNS answers and no search domains (SGO-NET-003).
* Not yet covered by a scenario: a greeting split across reads (the lab peer writes once; a
  library `ReadAll` is W23 work). An older attempt failing after a newer one started is
  `lab/older-fails-later` (a blackhole fails after 75 virtual seconds while the IPv4 peer answers at
  80); two attempts connecting before the race selects either is `lab/two-connected` (the winner
  depends on the schedule, and the sweep sees both).
