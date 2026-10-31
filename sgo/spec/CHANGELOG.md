# S-Go specification changelog

Amendments of frozen clauses and version changes of the specification (status rules in `README.md`). Each entry names the clauses whose version changed and why.

## 0.2.0 — 2026-11-01

Subset 1 (D23): a strict superset of 0.1.0. Every 0.1.0 program keeps its meaning; constructs 0.1.0 denied and 0.2.0 admits are named below. The admission digest and toolchain identity change, so objects built for 0.1.0 are refused by a 0.2.0 toolchain (SGO-IMP-002), and every module declares `sgo 0.2.0`.

New frozen clauses (drafted in 2026-10/11, implemented and evidenced in W05 and W06):

- SGO-PERMIT-002 v1 — typed managed channels with send permits: `sgo.Channel[T]`, `sgo.SendPermit[T]`, `sgo.NewChannel[T]` for T in int, bool, string, error (D28, D29, D30).
- SGO-READY-002 v1 — `n.Start` with a readiness handshake, `*sgo.Ready` (D27).
- SGO-AUTH-001 v1 — capability handles and task ceilings; `n.SpawnWithout` (D24).
- SGO-AUTH-002 v1 — root grants declared in `sgo.mod` (`grant <kind>`), recorded in the build ID (D25).
- SGO-AUTH-003 v1 — effect kinds clock, wallclock, random, entropy, simstore, with attenuation and `LendTo` (D26). `simnet` is not part of 0.2.0.

Amended frozen clauses:

- SGO-TYPE-001 v1 → v2 — the admitted pointer types include the 0.2.0 capability types.
- SGO-TYPE-005 v1 → v2 — the opaque capability types include `sgo.Ready`, the effect handles, and the channel and permit instances; their values also come from `Start`, the root accessors, attenuation, `LendTo`, `NewChannel`, and `Reserve`.
- SGO-CAP-001 v1 → v2 — the capability model includes the 0.2.0 handles; effect operations also require the kind in the calling task's ceiling.
- SGO-GEN-001 v1 → v2 — user generics stay denied; the only admitted instantiations are the three parameterized members of package `sgo` of SGO-PERMIT-002.

Superseded denial clauses (kept for history; what they still deny is restated):

- SGO-PERMIT-001 v1 → v2, superseded by SGO-PERMIT-002 — every other permit or obligation API stays denied.
- SGO-READY-001 v1 → v2, superseded by SGO-READY-002 — every other readiness API (`n.SpawnReady` or similar) stays denied.

Decisions recorded in the 0.2.0 clauses during implementation:

- SGO-PERMIT-002: for capacity 0, a receive takes effect when a reservation is matched to it; a matched receiver whose task is cancelled completes when that permit resolves.
- SGO-READY-002: a child that returns `nil` without signalling makes `Start` return the `child <id> ended before ready` error.
- SGO-AUTH-001/003: `SpawnWithout`'s kind and argument preconditions are checked at run time (`InvalidArgument`).

Known limits of 0.2.0, candidates for a later version: a `*sgo.Random` can be obtained only by the root task (task affinity has no hand-off); a module named `sgo` is rejected as an import cycle (SGO-IMP-001) rather than with a dedicated diagnostic.

## 0.1.0 — 2026-10-30

Subset 0: 84 frozen clauses (W01).
