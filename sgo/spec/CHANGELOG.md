# S-Go specification changelog

Amendments of frozen clauses and version changes of the specification (status rules in `README.md`). Each entry names the clauses whose version changed and why.

## 0.6.0 — 2026-11-02

Tooling: `sgo check` and `sgo test` (W10, D61-D64, D69). Programs keep their meaning; the admission digest and toolchain identity change, so objects built for 0.5.0 are refused by a 0.6.0 toolchain (SGO-IMP-002), and every module declares `sgo 0.6.0`. A test binary's build ID names the configuration `test`, and the main package `sgo test` generates records the configuration `testmain` in its identity; neither appears in any other program.

New frozen clauses:

- SGO-TOOL-002 v1 — `sgo check [-json] [packages]`: the front end of `sgo build` without code generation, with exactly the diagnostics `build` reports (D63).
- SGO-TEST-002 v1 — `sgo test [-v] [-run regexp] [packages]`: `func TestX(t *sgo.T) error` in `_test.sgo` files, which are part of a package only under `sgo test`; one process per test with its selection on descriptor 3, a fresh `T.TempDir`, the module's grants, and a fixed 60 s watchdog; `sgo.StartTest` admitted only in the generated main (`-sgotestmain`); tests of package `main` and lab tests are not in 0.6.0 (D62, D69).
- SGO-TEST-003 v1 — test quiescence: obligations a test's root task leaves are resolved and fail a test that returned nil with `LeakedWork`; a returned error stays primary with them in its chain (D64, D69).

Amended:

- SGO-TOOL-001 v2 — the command list names `sgo lab` (spec 0.4.0), `sgo check`, and `sgo test`; any other command or flag is still rejected.
- SGO-TYPE-001 v5 and SGO-TYPE-005 v5 — `*sgo.T` is a capability handle type.

## 0.5.0 — 2026-11-02

Confined files and network endpoints (W09, D51-D60). Programs without I/O grants keep their meaning; the admission digest and toolchain identity change, so objects built for 0.4.0 are refused by a 0.5.0 toolchain (SGO-IMP-002), and every module declares `sgo 0.5.0`. A program with file or network grants records their digest in its build ID (`io=`).

New frozen clauses (W09, model first per D30):

- SGO-FILE-002 v1 — directory grants (`grant dir NAME PATH ro|rw`) and confined paths: held directories, component-wise resolution without symlinks and with chain verification, the `Dir` and `Listing` API (D52, D53, D59 amended by the user: `List` returns `*sgo.Listing`).
- SGO-FILE-003 v1 — the I/O handle contract of `File`, `Conn`, and `Listener`: ownership, lending only to the owner's nurseries, `HandleLent`, `HandleClosed` and no reuse, abandonment, progress (`sgo.Progress`, `sgo.NoEffect`, `sgo.IsEOF`), and which waits are cancellation points (D54-D56).
- SGO-NET-002 v1 — endpoint grants for TCP (exact IPv4 and IPv6 literals) and Unix domain stream sockets (`unix:PATH`), `Dial`, `Listen`, `Accept`, `PeerUID`, stale-socket removal, and the lab loopback network (D57, D58, D60).

Superseded:

- SGO-FILE-001 v2 and SGO-NET-001 v2 — superseded by the clauses above; ambient file and network packages, DNS, UDP, abstract and datagram sockets, descriptor passing, and every access outside the new APIs stay denied.

Amended:

- SGO-TYPE-001 v4 and SGO-TYPE-005 v4 — the I/O handle types `*sgo.Dir`, `*sgo.Listing`, `*sgo.File`, `*sgo.Conn`, `*sgo.Listener` are capability handle types, and where their values come from.
- SGO-AUTH-002 v2 — `sgo.mod` also holds the file and network grants; they are not effect kinds and do not enter ceilings; only the root task holds them.

## 0.4.0 — 2026-11-02

The lab execution configuration (W08, D40-D50). The source language is unchanged: every 0.3.0 program keeps its meaning in production, and the lab is selected only by `sgo lab run`, `sgo lab replay`, and `sgo lab sweep`. The admission digest and toolchain identity change, so objects built for 0.3.0 are refused by a 0.4.0 toolchain (SGO-IMP-002), and every module declares `sgo 0.4.0`. The S-Go identity of every package now names its execution configuration (`prod`, `lab`, or `tcb`).

New frozen clauses (W08, model first per D30):

- SGO-LAB-002 v1 — the lab execution configuration: lab binaries built only by `sgo lab`; configurations never mix; a production binary has no lab activation path (D41).
- SGO-LAB-003 v1 — scheduling: one task runs at a time; a choice point before every listed operation and every K steps of S-Go code; `random` and `fifo` policies; select arms by source index (D42, D43, D49, D50).
- SGO-LAB-004 v1 — streams, virtual time, and effects: PCG-DXSM streams from the seed by SplitMix64; virtual time and timer ties; WallClock from the manifest epoch; Entropy unsupported (D44, D45).
- SGO-LAB-005 v1 — trace, replay, and sweeps (D46, D48).
- SGO-LAB-006 v1 — lab outcomes: LabDeadlock with the wait graph, LabQuotaExhausted, UnsupportedLabEffect, LabReplayDivergence; host hangs stay outside (D47).

Superseded denial clauses:

- SGO-LAB-001 v1 → v2, superseded by SGO-LAB-002 to SGO-LAB-006 — lab-only APIs, source constructs, pragmas, and flags of `sgo build`/`sgo run` stay denied.

Execution configurations: the `supported_execution_configurations` field of the 0.3.0 clauses reads "lab denied until W08 (SPEC-LAB)". From 0.4.0, SGO-LAB-002 states that every clause applies unchanged in a lab run except for the scheduling, time, and effect rules of SGO-LAB-003 and SGO-LAB-004; those clauses are not amended.

## 0.3.0 — 2026-11-02

Subset 2 (D31): a strict superset of 0.2.0. Every 0.2.0 program keeps its meaning; constructs 0.2.0 denied and 0.3.0 admits are named below. The admission digest and toolchain identity change, so objects built for 0.2.0 are refused by a 0.3.0 toolchain (SGO-IMP-002), and every module declares `sgo 0.3.0`.

New frozen clauses (W07, model first per D30):

- SGO-CHAN-002 v1 — native channels: `chan T`, `<-chan T`, `chan<- T` for T in int, bool, string, error, and channels of them (D36); `make`, send, receive, `close` (idempotent), channel `range`; no nil channels; `len`, `cap`, and comparison denied; send on a closed channel fails with `ClosedChannel` (D32, D35). Native channels are the managed channels of SGO-PERMIT-002.
- SGO-CHAN-003 v1 — cancellation unwind of a blocked native operation (D33); a whole-program deadlock is one `Deadlock` record with status 2.
- SGO-CHAN-004 v1 — `select`: operands evaluated once in source order, uniform choice among ready arms, exactly one arm takes effect, `default`, `select {}` (D34).
- SGO-SYNC-002 v1 — `sgo.Event` and `sgo.CapacityLimiter` with task-affine `*sgo.Token` obligations and `TokenAbandoned` (D39). Go's `sync` and `sync/atomic` stay denied (SGO-SYNC-001, D37).

Amended frozen clauses:

- SGO-TYPE-001 v2 → v3 — the admitted types include the channel types of SGO-CHAN-002 and the handles of SGO-SYNC-002.
- SGO-TYPE-005 v2 → v3 — the capability types include `sgo.Event`, `sgo.CapacityLimiter`, `sgo.Token`; channel values follow the same no-nil and no-comparison rules.
- SGO-CAP-001 v2 → v3 — the capability model includes the SGO-SYNC-002 handles; channel values carry no authority.
- SGO-DEF-003 v1 → v2 — the failure kinds include `ClosedChannel`.
- SGO-IMP-001 v1 → v2 — a module path may not equal, contain, or lie under a TCB package path; `SGO_E_USAGE` at the module directive (D38(b); `tcb-module-named-sgo` was an import cycle).
- SGO-AUTH-003 v1 → v2 — a generator returned by `Split` is unowned until its first use, which claims it (D38(a)).
- SGO-COMP-001 v1 → v2 — `make` of a channel type and `range` over a channel are admitted; every other form stays denied.

Superseded denial clauses:

- SGO-CHAN-001 v1 → v2, superseded by SGO-CHAN-002 and SGO-CHAN-004 — `len`/`cap` of channels, nil channels, other element types, channel comparison, and reflection over channels stay denied.

Cases whose meaning changed (D31): `chan-select` and `ctl-select` move from reject to run (`select {}` in the root task deadlocks); `chan-type` now rejects the nil literal; `tcb-module-named-sgo` reports `SGO_E_USAGE` at `sgo.mod`.

Decisions recorded in the 0.3.0 clauses during implementation:

- SGO-CHAN-002: a named result of channel type is a compile error (it would start as a nil channel).
- SGO-CHAN-003: a whole-program deadlock (every task waits and none can be woken) ends with a `Deadlock` record and status 2; before 0.3.0 the runtime printed Go's fatal error.
- SGO-CHAN-003: when a child failure joined during a cancellation unwind becomes primary, the record keeps the child's origin.

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
