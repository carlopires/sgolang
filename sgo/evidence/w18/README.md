# W18 evidence — Happy Eyeballs (RFC 8305)

Decisions D143–D146 (owner-delegated, `sgo/SGO_DECISIONS.md`); plan `sgo/docs/happy-eyeballs-plan.md`;
spec 0.15.0: SGO-NET-002 v2, SGO-NET-003 v1 (dialers, host-name resolution, lab network table),
SGO-NET-004 v1 (connection slots).

## What was built

| step | what | where |
|---|---|---|
| 1 | executable model of the connection handoff: 24 variants, 23,418 states, no violation; 3 mutants caught | `sgo/model/handoff.go`, `handoff_test.go` |
| 2 | `sgo.Dialer`, `sgo.ConnSlot` | `src/runtime/sgo_netdial.go`, `src/sgo/io.go` |
| 3 | host-name connect grants; `Dialer.Resolve` (/etc/hosts, then DNS A/AAAA over UDP, cancellable) | `src/runtime/sgo_netresolve.go`, `sgo_iogrant.go`, `src/cmd/sgo/iogrant.go`, linker `-sgoiogrants` |
| 4 | lab network table `lab.net` (`resolve`, `peer`, `blackhole`) on virtual time; part of the trace header | `src/runtime/sgo_netfake.go`, `src/cmd/sgo/lab.go` |
| 5 | `examples/happyeyeballs`: `he.Connect`, a real-network demo, five lab scenarios | `sgo/examples/happyeyeballs` |

## Results (darwin/arm64, local)

- Real network: `sgo run .` resolves `localhost` (`[::1]`, `127.0.0.1`), the IPv6 attempt is
  refused, IPv4 wins at once; output pinned in `expected.txt` (5 identical runs).
- Real DNS: `Dialer.Resolve("example.com", "ip4")` answered over UDP from the resolv.conf server.
- Lab sweeps, seeds 1..200 each, 0 failures in 1,000 runs: silent-v6 (IPv4 starts at the 250ms
  attempt delay and wins), slow-aaaa (IPv4 after the 50ms resolution delay), prefer-v6 (AAAA
  within 50ms: IPv6 wins), slow-v6 (IPv4's head start wins, IPv6 cancelled), all-refused (§4
  interleave v6, v4, v6, then an error). Each scenario asserts its winner and its RFC step.
- Mutants of `he.sgo`, each caught by a sweep: IPv4 first (§4) and no resolution delay (§3)
  fail prefer-v6; no attempt timer (§5) fails silent-v6.
- Conformance: 8 new cases (net-dialer-race, -child-create, -other-endpoint, -not-lent,
  net-slot-second-put, -close-closes, -take-not-owner, -put-not-owner); net-grant-hostname
  re-pinned to the remaining denial (a host name in a listen grant).
- test-examples, test-lab, test-identity, test-tcb, test-flags, spec, guarantee, std-api,
  adoption checks: pass. linux/amd64 and linux/arm64: `go vet runtime sgo` clean on the lane.

## Review (note2, linux/amd64)

No blocker; four minor findings fixed in `eb84bca69d` and spec 0.15.1 (`1c4f7c93e8`), verified on
linux/amd64 (conformance 572/572, test-examples on a host without `::1 localhost`, runtime lane,
`FuzzSGoDNSAnswer` 30 s):

- Y1: the real-run check asserts only host-independent lines; the lab pins the attempt order.
- Y2: re-resolving adds no duplicate (address, port) to a dialer (`TestSGoResolveNoGrowth`).
- Y3: unspecified and IPv4-mapped addresses are never results, from hosts, DNS, or lab tables.
- Y4: DNS responses must echo the question; only the name's records and its CNAME chain count.

## Not done (documented limits)

No TCP retry of truncated DNS answers, no search domains, no RFC 6724 address sorting.
