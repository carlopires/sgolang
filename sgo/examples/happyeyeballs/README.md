# Happy Eyeballs (RFC 8305) in S-Go

Happy Eyeballs is the example Nathaniel Smith uses in
[*Notes on structured concurrency*](https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/#nurseries-in-practice):
connect to a name that has IPv6 and IPv4 addresses, prefer IPv6, and never wait for a dead
family. It is a small algorithm with a lot of concurrent parts: two DNS queries, a resolution
timer, a growing set of connection attempts, one timer per attempt, and a winner. Getting it
right with free-running threads is notoriously hard: the losers must be stopped, their
connections closed, and nothing may outlive the call.

`he.Connect` (in `he/he.sgo`, about 200 lines) does it in **one nursery**:

| RFC 8305 | in `he.Connect` |
|---|---|
| §3 query AAAA and A at once; start on AAAA, or on A after a 50ms resolution delay | two resolver children and a delay timer child |
| §4 interleave the families, IPv6 first | the nursery body picks the next address |
| §5 start the next attempt after 250ms, or at once when an attempt fails | one attempt child and one timer child per attempt |
| the first connection wins; the rest are abandoned | `ConnSlot.Put` (the first one wins), then `n.Cancel()` |

### What S-Go guarantees here, and Go does not

* **Nothing leaks.** `Connect` returns only after the nursery has joined every query, timer, and
  attempt. A forgotten attempt is not possible: there is no `go` statement.
* **Losing connections are closed.** A connection belongs to the task that dialed it. The loser's
  `Put` fails, and its task closes the connection. A connection still in the slot is closed with
  the slot. Handing a connection to a task that doesn't own it is a checked error.
* **Authority is explicit.** `sgo.mod` grants `connect example.test:443`. Only the root task holds
  the grant. It creates a `Dialer` for that name and lends it to the race's nursery. The dialer
  may dial only the addresses its own `Resolve` returned, and `root.Dial` refuses host names.
* **Cancellation is the timeout.** An attempt's connection timeout is its cancellation: a
  blackholed SYN costs nothing once the race is won.
* **The timing is testable.** `sgo lab` runs the race on virtual time against a scripted network
  (`lab/*/lab.net`), under many schedules. Each scenario checks the winner *and* the RFC step it
  is about. Breaking §3, §4, or §5 in `he.sgo` fails a sweep, and the failing schedule replays
  exactly.

## Run it

```
sgo run .                                  # real network: localhost, IPv4-only server
sgo lab sweep -seeds 0..199 ./lab/silent-v6   # IPv6 blackholed: IPv4 starts at 250ms and wins
sgo lab sweep -seeds 0..199 ./lab/slow-aaaa   # AAAA late: IPv4 starts after the 50ms delay
sgo lab sweep -seeds 0..199 ./lab/prefer-v6   # AAAA within 50ms: IPv6 first, and it wins
sgo lab sweep -seeds 0..199 ./lab/slow-v6     # IPv6 slower than IPv4's head start: IPv6 cancelled
sgo lab sweep -seeds 0..199 ./lab/all-refused # everything refuses: an error, nothing left running
```

The real run resolves `localhost` from `/etc/hosts` (`::1`, then `127.0.0.1`). Other names go to
the resolv.conf servers over UDP. The server listens on IPv4 only, so the preferred `[::1]` is
refused and the race moves to IPv4 at once. `expected.txt` is that run on a host whose
`/etc/hosts` lists `::1 localhost`; where it lists only `127.0.0.1`, there is no IPv6 attempt and
the race is the IPv4 lines alone. The real run uses the fixed port 47970 (grants name exact ports);
the exact attempt order of every RFC rule is checked by the lab scenarios, which do not depend on
the host.

Limits of this version (W18): no TCP fallback for truncated DNS answers, no search domains, and
no address sorting beyond family interleaving (RFC 6724 is not applied).
