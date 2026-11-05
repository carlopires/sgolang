# W23 evidence — the concurrency library and resource interfaces (spec 0.18.0)

Decisions D173–D176. Clauses frozen at 0.18.0: SGO-FILE-004, SGO-SPAWN-008, SGO-SPAWN-009,
SGO-RES-006, SGO-STD-008. Amended: SGO-SPAWN-006 v2, SGO-SPAWN-007 v2, SGO-CTL-004 v2,
SGO-ERR-004 v3, SGO-RES-001 v3, SGO-RES-004 v5, SGO-FILE-003 v3, SGO-IFACE-002 v2 (the 0.17.0
review errata SGO17-01 to SGO17-07, W22 review G1–G3 and H1–H3, W23 review M1–M4). Freeze commit
8da2d5eb2d; CHANGELOG entry "0.18.0".

## What was built

| Part | Where |
|---|---|
| Whole-message reads `ReadLine`, `ReadAll`, `sgo.IsTooLong`; one read turn per handle, so `Close` during a read is `ResourceBusy` (D164, D173) | `src/runtime/sgo_readbuf.go`, `sgo_io.go`, `src/sgo/io.go` |
| Request/reply calls `sgo.NewCall[Req, Resp]`, `Call`, `Accept`, `Request.Reply`, `sgo.IsNoReply`; withdrawal by cancellation before acceptance; an accepted request is a resource of the accepting body (D167, D174) | `src/sgo/call.go`, `src/runtime/sgo_call.go`, `types2/sgo_resflow.go` |
| Pools with leases `sgo.NewPool[T]`, `Put`, `Acquire(s)`, `Lease.Value`, `Return`, `Drop`; an item returns on unwind (D175) | `src/sgo/pool.go`, `src/runtime/sgo_pool.go`, `types2/sgo_resflow.go` |
| Resource interfaces: an interface embedding `sgo.Resource` holds resources, is tied as what it holds, and is task-local; call results tied to the innermost authority argument (C1, refined by M1), constructors handed no reference feed `Own` (M4) (D176) | `types2/sgo_resflow.go`, `sgo_local.go`, `sgo_frozen.go`, `src/sgo/sgo.go` |
| `std/io`: `Reader`, `LineReader`, `Writer`, `WriteString`, `Copy`; standard output usable as an `io.Writer` | `src/sgostd/io/` |
| Edge-triggered `Result()` channels; not-admitted starts publish the cancellation; aggregate resource results move into the nursery (SGO17-01, -02; G2, H1) | `src/runtime/sgo_taskres.go`, `sgo_group.go`, `sgo_res.go` |
| HTTP/1.1 server example: one parser over `std/io`, one task per connection, per-request read limit and handler deadline, client errors kept per connection, shutdown by one `Cancel` | `sgo/examples/httpserver` |

## Executed on darwin/arm64 (2026-11-06)

- Conformance 719/719 at 0.18.0, including the W23 cases: `file-read-lines`,
  `lab-net-read-line`, `lab-net-readline-close`, `call-*`, `lab-call-*`, `pool-*`, `res-iface-*`,
  `std-io-*`, `spawn-go-not-admitted`, `task-aggregate-resource-result`,
  `task-error-rollback-inner-nursery`, `exit-check-rollback`, `res-own-value-type`,
  `res-own-constructor`, `res-own-constructor-alias`, `frozen-value-with-release-ok`.
- Examples: pipeline, service, replay, happyeyeballs, httpserver — `sgo check`, `sgo test`,
  `sgo run`, `sgo lab sweep` (httpserver: 0..63, one outcome, including the read-limit close on
  virtual time).
- Gates: spec-tool (145 clauses: 138 frozen, 7 superseded, 0 draft), guarantee-check,
  test-identity, test-mutations, test-lab, test-tcb, test-fmt, test-std, test-flags,
  test-negative, test-modules, test-check, test-test, inventory, policy, legacy-audit,
  lock-alloc-check, res-kind-check, generated-twin-check, source-map-audit, std-api-check.

## Linux

Pending: sgo-w00 run 41938348712 and sgo-runtime run 41938348649 on main at 22599be98c (sgo-spec
run 41938348657: success).

## Models

- `sgo/model/callmodel.go`: request/reply hand-over, withdrawal before acceptance, `NoReply` when
  an accepting body ends unanswered.
- `sgo/model/poolmodel.go`: leases and returns, 1 422 states; four seeded bugs, each caught.
