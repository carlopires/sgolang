# W18 — Happy Eyeballs (RFC 8305) in S-Go

Status: **approved 2026-11-03 under owner delegation** (D143–D146). Freezes at spec 0.15.0.

## Why

Nathaniel Smith's "Notes on structured concurrency" uses Happy Eyeballs as the example where
nurseries pay off. The algorithm starts connection attempts to a host's addresses with staggered
start times, keeps the first one that succeeds, and cancels the rest. With nurseries that is a
loop inside one scope: the scope waits for every attempt, and cancelling it cleans up every loser.
Written with unstructured goroutines, the same algorithm leaks attempts and sockets unless every
path is handled by hand.

S-Go 0.14.1 has the concurrency half (nurseries, cancellation, a cancellable `Dial`, `Sleep`,
`Event`, channels, `select`). Three capability rules blocked the rest:
1. only the root task may dial (SGO-NET-002);
2. a connection belongs forever to the task that dialed it, and is closed when that task ends
   (SGO-FILE-003);
3. host names and DNS are denied (SGO-NET-001).

## Decisions (D143–D146)

| ID | Piece | What it adds |
|---|---|---|
| D143 | `sgo.Dialer` | delegated connect authority: a handle limited to granted endpoints, lent to the attempts' nursery |
| D144 | `sgo.ConnSlot` | hand-up: the first attempt to `Put` its connection wins; the caller `Take`s it as its own |
| D145 | `Dialer.Resolve` | `grant connect HOST:PORT`; resolution of A or AAAA with cancellation; resolved addresses become dialable by that dialer |
| D146 | `lab.net` | a fake resolver table and slow or silent endpoints, so the race replays deterministically |

None of them weakens a guarantee:
- authority stays explicit (a dialer only narrows the root's grants);
- every connection still has exactly one owner at a time, and is closed if nobody takes it;
- handle lists stay touched only by their owner.

## The example (`sgo/examples/happyeyeballs`)

`he.Dial(ctx, d, host, port)` follows RFC 8305:
- **§3 resolution:** AAAA and A queries run in parallel. If A answers first, a Resolution Delay
  of 50 ms waits for AAAA.
- **§4 sorting:** the addresses are interleaved, IPv6 first, alternating families.
- **§5 connection attempts:** one attempt per address in a nursery, started 250 ms apart (the
  Connection Attempt Delay), or at once when the previous attempt fails. The first connection
  `Put` into the slot wins, and the nursery is cancelled. The losers' sockets are closed when
  their tasks end.

Checks:
- **unit tests** (`sgo test`) on localhost;
- **lab tests** with `lab.net`: a silent IPv6 address and a working IPv4 one give the IPv4 winner
  at exactly 250 ms of virtual time, the same on every seed;
- **a command** that connects to a host given on the command line and reports the winning
  address.

## Steps

1. Model of the hand-up (ownership, Put/Take races, scope exits), explored over every
   interleaving before the runtime change.
2. Runtime and `sgo`: `Dialer`, `ConnSlot`; conformance and runtime-lane tests.
3. Resolver: grants with host names; /etc/hosts and DNS over UDP; lab table.
4. `lab.net`: fake resolution, blackhole and delay endpoints.
5. The example, its tests, and the lab replay.
6. Spec 0.15.0: SGO-NET-001 v2, SGO-NET-002 v2, new SGO-NET-003 (dialer and resolution) and
   SGO-NET-004 (connection slot), SGO-LAB amendment; evidence; reviewer verification.
