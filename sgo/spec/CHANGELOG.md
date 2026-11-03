# S-Go specification changelog

Amendments of frozen clauses and version changes of the specification (status rules in `README.md`). Each entry names the clauses whose version changed and why.

## 0.14.1 — 2026-11-03

Review fix (W17 review X1). A type assertion or type-switch case to `error` from a task-local interface (`a.(error)` for `a any`, `case error` in a switch on `any`) was accepted and could turn a task-local value with an `Error` method into a shareable `error`; it is now refused with `SGO_E_TASK_LOCAL_CAPTURE`. Assertions from an `error` are unchanged. Every other program valid under 0.14.0 keeps its meaning; the admission digest and toolchain identity change, and every module declares `sgo 0.14.1`.

Amended:

- SGO-IFACE-004 v2 — an assertion or case to `error` is admitted only from an `error` operand.
- SGO-COMP-004 v5 — rule 4 names that assertion form.

## 0.14.0 — 2026-11-03

Interfaces (W17; D107, D127–D133; drafts approved 2026-11-03). Every program valid under 0.13.0 keeps its meaning, with one exception: comparing an `error` value with a non-nil value of a concrete type (`err == T{...}`) was accepted and is now refused, since interface values compare only with `nil` (D128). The admission digest and toolchain identity change, so objects built for 0.13.0 are refused by a 0.14.0 toolchain (SGO-IMP-002), and every module declares `sgo 0.14.0`.

New frozen clauses (family SPEC-IFACE, now admitted):

- SGO-IFACE-002 v1 — named interface types, `any`, implicit implementation by method sets (D127, D131); interface values are task-local except `error` (D132).
- SGO-IFACE-003 v1 — dynamic dispatch; no typed nil: a nil pointer converts to the nil interface, and a method call on a nil interface is `NilDereference` (D130).
- SGO-IFACE-004 v1 — comma-ok type assertions and type switches (D129).

Amended:

- SGO-IFACE-001 v2 — what stays denied: interface literals outside a declaration, type-set elements and constraint interfaces, `==` between interface values, interface map keys, the single-value assertion, method values and expressions of interface types.
- SGO-TYPE-001 v9 — named interface types and `any` join the admitted types.
- SGO-TYPE-006 v2 — `error` is the one shareable interface type: assertions, type switches, and wrapping (`errors.Unwrap`) on errors; `error` fields keep a struct shareable.
- SGO-COMP-004 v4 — interface types other than `error` are task-local; `sgo.Copy` deep-copies an interface's dynamic value, and `Copy` of nil is nil.
- SGO-CTL-001 v2 — type switches admitted; the denials of `select` and range forms, lifted by later clauses, are replaced by references to them.
- SGO-STD-001 v2 — `errors.Unwrap` (D133).
- SGO-STD-006 v2 — `sort.Interface` and the stable `sort.Sort` (D133).

## 0.13.0 — 2026-11-03

Standard library v1 (W16; D103, D108–D110, D121–D126; drafts approved 2026-11-03). Every program valid under 0.12.0 keeps its meaning, except that `std/strconv` errors now carry Go's messages (`strconv.Atoi: parsing "x": invalid syntax`, D121); the admission digest and toolchain identity change, so objects built for 0.12.0 are refused by a 0.13.0 toolchain (SGO-IMP-002), and every module declares `sgo 0.13.0`.

New frozen clauses (new family SPEC-STD):

- SGO-STD-001 v1 — `std/errors`: `New` (D121).
- SGO-STD-002 v1 — `std/strings`: search, split and join, trim, `Repeat`, `Replace`, ASCII case functions (D122), `Builder`.
- SGO-STD-003 v1 — `std/bytes`: the core byte-slice functions, with Go's aliasing, lengths, and capacities.
- SGO-STD-004 v1 — `std/strconv`: `Itoa`, `Atoi`, `ParseInt` (no bitSize), `FormatInt`, `ParseBool`, `FormatBool`, `QuoteToASCII`, `Unquote`; Go's error messages with the input quoted by `QuoteToASCII`.
- SGO-STD-005 v1 — `std/utf8`: Go's `unicode/utf8` API, decoding as D117.
- SGO-STD-006 v1 — `std/sort`: stable typed sorts, `Perm`, `Search` (D123).
- SGO-STD-007 v1 — `std/math`: `MaxInt`, `MinInt`, checked `Abs`, `Min`, `Max`, `Pow`.

Amended:

- SGO-IMP-004 v2 — the library of this version is the seven SGO-STD packages, each exporting exactly its clause's declarations (D126, checked by `std-api-check.py`).

## 0.12.0 — 2026-11-03

Runes and text iteration (W15; D106, D116–D120; drafts approved 2026-11-03). Every program valid under 0.11.0 keeps its meaning; the admission digest and toolchain identity change, so objects built for 0.11.0 are refused by a 0.12.0 toolchain (SGO-IMP-002), and every module declares `sgo 0.12.0`.

New frozen clause:

- SGO-TYPE-008 v1 — the rune type: a checked integer 0..0x10FFFF (`IntegerOverflow` outside it, including `rune(i)` and `byte(r)`); untyped rune constants default to rune and fit int, byte, or rune when representable (D118); conversions `rune(i)`, `int(r)`, `rune(b)`, `byte(r)` (D120); `int32` stays denied.

Amended:

- SGO-LEX-004 v2 — rune literals; `\u` and `\U` escapes in string and rune literals and `\'` in rune literals; a surrogate or a value above 0x10FFFF is `SGO_E_CONST_RANGE` (D119).
- SGO-TYPE-001 v8 — `rune` is admitted; the stale mention of `byte` among the denied types is removed.
- SGO-TYPE-007 v2 — `rune(b)`, `byte(r)`; byte constants may be written as rune literals.
- SGO-VAL-002 v3 — `range` over a string by rune (byte offset, rune; invalid UTF-8 decodes to U+FFFD and advances one byte, D117); `string(r)` (a surrogate encodes as U+FFFD, D116); `string(i)` of an int stays denied (D120).
- SGO-COMP-001 v5 — consequential: `range` over a string, `[]rune`, and rune conversions leave the list of denied composite forms; a string joins the admitted `range` operands.
- SGO-COMP-005 v2 — `[]rune(s)` (length and capacity equal the rune count) and `string(rs)`; conversions between slice types stay denied.

## 0.11.0 — 2026-11-03

Packages beyond the module (W14; D103–D105, D109–D115; drafts approved 2026-11-03): required modules with `require`/`replace`/`sgo.sum`, and the standard library `std/`. Every program valid under 0.10.1 keeps its meaning; the admission digest and toolchain identity change, so objects built for 0.10.1 are refused by a 0.11.0 toolchain (SGO-IMP-002), and every module declares `sgo 0.11.0`.

New frozen clauses:

- SGO-IMP-003 v1 — required modules: `require <path> vX.Y.Z`, `replace <path> => <dir>` (main module only) or `vendor/<path>/`; each module path once per build (D111); `sgo.sum` digests checked before any compile and written only by `sgo mod sum` (D113); dependency grants bounded by the main module's, no file or network grants in dependencies (D114).
- SGO-IMP-004 v1 — the standard library: the implicit module `std` from `$GOROOT/src/sgostd`, checked against its MANIFEST, compiled under the same rules as user code, importing only `std/` and `sgo` (D103, D115); first package `std/strconv` (`Itoa`, `Atoi`).

Amended:

- SGO-IMP-001 v3 — a program is built from its main module, the modules it requires, and `std`; imports resolve by longest module-path prefix; module paths under `std` are reserved; cycles are refused across modules.
- SGO-IMP-002 v2 — contract identity is checked across module boundaries; only TCB packages are cached.
- SGO-AUTH-002 v3 — a dependency's `grant` lines are a bound on the main module's grants, never a source of authority.

## 0.10.1 — 2026-11-03

Soundness fixes from the W13 review (V1, V2). Programs that 0.10.0 rejected stay rejected; two kinds of programs that 0.10.0 accepted are now rejected or copied differently, because 0.10.0 accepted them unsoundly.

Amended:

- SGO-COMP-004 v3 — (V1) `sgo.Copy` joins references only when they have the same type: a pointer to a field or element of an object and a pointer to (or slice of) the object itself are copied separately, each at its own type and size (0.10.0 could allocate one object at the size of another). (V2) rule 4: a value of a task-local type is never converted to `error`; error values are shared between tasks, so only shareable types implement `error` (0.10.0 let a task-local value cross tasks inside an error).

## 0.10.0 — 2026-11-03

Core data model, stage 3 (W13; D98–D101; draft approved 2026-11-03): frozen values. Every program valid under 0.9.0 keeps its meaning; the admission digest and toolchain identity change, so objects built for 0.9.0 are refused by a 0.10.0 toolchain (SGO-IMP-002), and every module declares `sgo 0.10.0`.

New frozen clause:

- SGO-COMP-009 v1 — `sgo.Frozen[T]`, `sgo.Freeze(x)` (one deep copy, D98), and `f.Get()`, readable only through access paths (selection, indexing, `len`, `range` binding values); any use that could write a frozen object or keep a reference into it is `SGO_E_FROZEN_WRITE` (D99). T holds no handle and no channel (D100); `Frozen` has no zero value, no `nil`, no `==`, and is shareable (D101).

Amended:

- SGO-COMP-004 v2 — `sgo.Frozen[T]` is a shareable type: captured by `Spawn` callbacks and sent or committed without `sgo.Copy`.
- SGO-GEN-001 v3 — the admitted instantiations include `sgo.Copy[T]`, `sgo.Frozen[T]`, `sgo.Freeze[T]`.
- SGO-TYPE-001 v7 — `sgo.Frozen[T]`.

## 0.9.0 — 2026-11-03

Core data model, stage 2 (W13; D93–D97; drafts approved 2026-11-02): task-local data — slices, insertion-ordered maps, pointers and pointer receivers — with the sharing rule enforced at compile time, `sgo.Copy`, the `byte` type, and the nil and data failure kinds. Every program valid under 0.8.0 keeps its meaning; the admission digest and toolchain identity change, so objects built for 0.8.0 are refused by a 0.9.0 toolchain (SGO-IMP-002), and every module declares `sgo 0.9.0`.

New frozen clauses:

- SGO-COMP-004 v1 — task-local types (pointers, slices, maps, and values holding them) and `sgo.Copy[T]`: task-local values are never captured by a function literal that may run in another task, never escape a function through a captured variable, and cross tasks only as a deep copy that preserves sharing and cycles (`SGO_E_TASK_LOCAL_CAPTURE`, `SGO_E_UNCOPIED_SEND`).
- SGO-COMP-005 v1 — slices, `[]byte`, string indexing and slicing, `make`, `append`, `copy`, `cap`; `BoundsViolation` and `InvalidArgument`.
- SGO-COMP-006 v1 — maps with insertion-order iteration (D88), the same in production and under `sgo lab`; value-type keys only; `NilMapWrite`.
- SGO-COMP-007 v1 — pointers, `&x`, `new`, pointer receivers; pointers to handles denied.
- SGO-COMP-008 v1 — nil of pointer, slice, and map types; `NilDereference`, `NilMapWrite`.
- SGO-TYPE-007 v1 — `byte`: checked arithmetic in 0..255.

Amended:

- SGO-COMP-001 v4, SGO-UTYPE-001 v3 — narrowed to what stays denied.
- SGO-COMP-002 v2 — channel, pointer, slice, and map fields and elements (D95).
- SGO-TYPE-001 v6, SGO-TYPE-002 v3 — `byte`, pointer/slice/map types, their identity and nil.
- SGO-UTYPE-002 v2 — methods with pointer receivers.
- SGO-MEM-001 v2, SGO-MEM-002 v4 — pointers name variables; `&x` of a captured variable (and a pointer-receiver call on it) assigns it, writes through references do not.
- SGO-VAL-002 v2 — string indexing, slicing, and `[]byte` conversions.
- SGO-LEX-002 v2 — `[ ]` and `...` admitted.
- SGO-DEF-003 v4 — `NilDereference`, `NilMapWrite`; `BoundsViolation` and `InvalidArgument` extended to slices and `make`.
- SGO-CHAN-002 v3, SGO-PERMIT-002 v3 — shareable and task-local elements; task-local values are sent only through `sgo.Copy`.

## 0.8.0 — 2026-11-02

Core data model, stage 1 (W13; D85–D92; drafts approved 2026-11-02): value structs and arrays, named value types, and value-receiver methods. Every program valid under 0.7.0 keeps its meaning; the admission digest and toolchain identity change, so objects built for 0.7.0 are refused by a 0.8.0 toolchain (SGO-IMP-002), and every module declares `sgo 0.8.0`.

New frozen clauses:

- SGO-COMP-002 v1 — value structs and arrays: fields and elements of value types (int, bool, string, value structs and arrays, named value types); composite literals, field and element access and assignment, `range` over arrays, `==`; always copied; admitted as channel elements and `sgo.Channel[T]` payloads.
- SGO-COMP-003 v1 — array indexing: the checked failure `BoundsViolation` (with position, index, and length); a constant index out of range is `SGO_E_CONST_RANGE`.
- SGO-UTYPE-002 v1 — package-level named value types and value-receiver methods; explicit conversions only between identical underlying types. Method values of user-declared types are denied; those of package sgo's handles stay admitted by their own clauses.

Amended:

- SGO-COMP-001 v3, SGO-UTYPE-001 v2 — narrowed to what stays denied (slices, maps, pointers, anonymous structs, embedding, aliases, pointer receivers, ...).
- SGO-TYPE-002 v2 — identity of named, array, and struct types; the explicit conversions of SGO-UTYPE-002.
- SGO-DEF-003 v3 — adds the failure kind `BoundsViolation`.
- SGO-CHAN-002 v2, SGO-PERMIT-002 v2 — value types as channel elements and payloads.
- SGO-MEM-002 v3 — an assignment to a field or element of a variable assigns the variable, so a captured struct or array is final as a whole.

## 0.7.0 — 2026-11-02

Release registry (D83): every implemented family is admitted, and the independent review's corrections are part of the text (W12, Q01). Programs keep their meaning except where a review fix made a hole an error; the admission digest and toolchain identity change, so objects built for 0.6.0 are refused by a 0.7.0 toolchain (SGO-IMP-002), and every module declares `sgo 0.7.0`.

Registry: the 30 rows that W03-W10 implemented, enforced, and tested move from `specified_not_admitted` to `admitted` (`admitted_in` 0.7.0, with the evidence files of their acceptance cases); the 8 denied rows stay denied. `sgo/GUARANTEES.json` and `guarantee-check.py` (in CI) check that each published guarantee rests on admitted rows, frozen clauses, and passed acceptance cases (SR2).

Amended:

- SGO-TCB-002 v2 — trust is a property of the triple (import path, file name, digest): a byte copy of a trusted file in a user package is user code (W12 R1); the compiler, importer, and linker each check it. The driver's cache of trusted archives is checked against a digest list before reuse: integrity against corruption and stale or partial entries, not tamper resistance (D-R8).
- SGO-MEM-002 v2 — the assignments that break effective finality are listed: also a `select` receive arm with `=` and a `return` with operands in a function whose named results are captured (W12 R5). A program that returned a value into a captured named result is now rejected with `SGO_E_CAPTURE_NOT_FINAL`.
- SGO-PROC-004 v2 — an S-Go program reads no runtime environment variable, `GOMAXPROCS` included (W12 R2, D84).
- SGO-FILE-002 v2, v3 — (v3 within 0.7.0, from the reviewer's re-check: a walk-mode `Create` that created a file reports `PathRaced`, W12 R11) `Open` and `Create` admit regular files only (`NotRegularFile`, W12 R4); on linux 5.6+ the kernel resolves paths beneath the grant (`openat2`), elsewhere the verified walk; `Create` truncates only after verification; `Mkdir` and `Remove`, and a walk-mode `Create` that created a file, report an effect a concurrent rename moved outside the grant as `PathRaced` (W12 R6, R11); `NoEffect` is false for exactly `IOError` and `PathRaced`.

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
