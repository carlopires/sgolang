# Happy Eyeballs (RFC 8305-based) in S-Go

Happy Eyeballs is the example Nathaniel Smith uses in
[*Notes on structured concurrency*](https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/#nurseries-in-practice):
connect to a name that has IPv6 and IPv4 addresses, prefer IPv6, and never wait for a dead
family. It is a small algorithm with a lot of concurrent parts: two DNS queries, a resolution
timer, a growing set of connection attempts, one timer per attempt, and a winner. Getting it
right with free-running threads is notoriously hard: the losers must be stopped, their
connections closed, and nothing may outlive the call.

`he.Connect` (in `he/he.sgo`) does it in **one nursery**, the race:

| RFC 8305 | in `he.Connect` |
|---|---|
| §3 query AAAA and A at once; start on AAAA, or on A after a 50ms resolution delay | two resolver children and a delay timer child |
| §4 interleave the families, IPv6 first | the nursery body picks the next address |
| §5 start the next attempt after 250ms, or at once when the latest attempt fails | one attempt child and one timer child per attempt |
| the first connection wins; the rest are abandoned | the body moves the first reported connection out, then cancels the race |

One coordinator, the nursery body, holds the address queues, decides when to start an attempt,
picks the winner, and logs. Every child does one thing and reports it on one channel made in the
race nursery. Expected failures (a refused address, a failed query) are events the coordinator
handles, not errors of the race.

### Who owns what (SGO-RES-001..005)

1. The caller's scope owns the dialer (`root.Dialer`).
2. Resolvers and attempts run inside the race nursery, and every attempt dials **into the race
   nursery** (`d.Dial(n, addr)`): each candidate connection is a resource of the race.
3. The channel carries references to candidates; sending one transfers nothing.
4. The coordinator moves exactly one connection out, the winner, with `c.MoveTo(task)`.
5. Every exit from the race goes through one `n.Cancel()`: the remaining queries, timers, and
   attempts stop; the nursery joins them; then it releases every candidate still in it.

So two attempts that both connect are safe: only the selected one leaves the race, and the other
stays a resource of the race nursery, even if its attempt has already returned or its report is
never read. Ownership alone does not end the race, though: a nursery joins its children, and a
child waiting on a blackholed connection waits until it is cancelled. That is why every exit of
the coordinator, including a failed log write or receive, cancels before it leaves the nursery,
and why `main` cancels its waiting server when the client fails.

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
* Not yet covered by a scenario: an older attempt failing after a newer one started (the lab
  network cannot schedule a delayed refusal), and a greeting split across reads (the lab peer
  writes once). Both mechanisms are replaced in the W22 rewrite (timers as `select` arms, and a
  library `ReadAll`), where they get their tests.
