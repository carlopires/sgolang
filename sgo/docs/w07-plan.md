# W07 plan — native channels, select, structured synchronization (spec 0.3.0)

Approved decisions: D31–D39 (`sgo/SGO_DECISIONS.md`). Order follows D30 (model first) and the W05/W06 practice of small verified commits, each with the full check cycle (spec-tool, tcb-manifest, inventory, build-stage0, conformance, conformance-compile, test-identity/flags/mutations/tcb/x10/fmt, runtime lane, model, policy-report).

## Steps

1. **Model.** Extend `sgo/model/chanmodel.go` with native send (an uninterruptible reserve+commit), `select` (operands evaluated once, uniform choice among ready arms, losers leave no reservation, `default`), cancellation unwind of a waiting operation (D33), send on a closed channel (`ClosedChannel`), and channel `range`; add `Event` and `CapacityLimiter` with token obligations (D39). Pin the new reference outcome sets (`testdata/chan-outcomes.json`, runtime copy).
2. **Runtime.**
   - Native operations on the TCB-P12 channel (`sgo_chan.go`): one-step send, receive, close, range step.
   - Multi-channel `select` waiters with a single terminal decision per select (one CAS across arms, under the channel locks in address order, then `sgoSleepLock`); loser cleanup.
   - The cancellation unwind of D33 (a Cancelled outcome, defers run, owned nurseries joined).
   - `Event` and `CapacityLimiter` on the same waiter and obligation machinery (`TokenAbandoned`).
   - Fault injection and invariant checks as in W06; runtime-lane tests against the model.
3. **Compiler.**
   - Admit `chan T`, `<-chan T`, `chan<- T`, `make(chan T, n)`, `<-`, `close`, `select`, and channel `range` for the D36 element types, including channels of them.
   - Deny `len`/`cap` of channels, nil channel values (no-nil rule), and handles or aggregates as elements.
   - Lower every channel operation to the S-Go runtime entry points: no path may reach Go's `hchan` (`chansend`, `chanrecv`, `selectgo`, `closechan`); add a check that the lowered code of S-Go packages references none of them.
   - `ClosedChannel` failure kind.
4. **Package sgo.** `NewEvent`, `NewCapacityLimiter`, `*sgo.Event`, `*sgo.CapacityLimiter`, `*sgo.Token` (capability types); D38(a) generator hand-off (claim on first use).
5. **Driver.** D38(b): module paths equal to or under a TCB package path are `SGO_E_USAGE`.
6. **Spec 0.3.0.** Draft, then freeze: native channel clauses superseding SGO-CHAN-001; select; structured primitives; amendments of SGO-TYPE-001/005, SGO-CAP-001, SGO-DEF-003 (`ClosedChannel`), SGO-IMP-001 (module paths), SGO-AUTH-003 (generator hand-off); CHANGELOG; every fixture module declares `sgo 0.3.0`.
7. **Conformance.**
   - The `strict_native` lane of H01–H12.
   - N01–N09, S12, S16.
   - Reject cases for every still-denied form (`len`/`cap`, nil, handle elements, `sync`).
   - The W07 gate: the race detector sees channel synchronization and still detects deliberate races (runtime-lane tests); forced GC and stack movement during channel operations.
8. **Evidence and ledgers.** `sgo/evidence/w07/README.md`, `SGO_TASKS.json`, `SGO_TEST_MATRIX.json`, policy report (new primitives and denied routes), TCB manifest (new entries for select and the structured primitives).
