# S-Go specification changelog

Amendments of frozen clauses and version changes of the specification (status rules in `README.md`). Each entry names the clauses whose version changed and why.

## 0.21.0 — 2026-11-08

Unsigned integers, package-level tables, and the first cryptography (W26; D136, D196, D197). Every program valid under 0.20.1 keeps its meaning: the release only admits forms that were refused and adds packages.

New clauses:

- SGO-TYPE-009 — `uint32` and `uint64`: arithmetic, shifts, and conversions fail with `IntegerOverflow` (or `DivideByZero`) instead of wrapping; unary minus on them is denied (D196).
- SGO-STD-011 — `std/bits`: `Add/Sub/Mul/Div`, rotations, counts, and byte reversal on `uint32`/`uint64`, with the results of Go's `math/bits` (D196).
- SGO-STD-012..014 — `std/encoding/hex`, `std/encoding/base64`, `std/encoding/binary` (D197).
- SGO-STD-015, 016 — `std/crypto/sha256`, `std/crypto/sha512` (D197).
- SGO-STD-017 — `std/hash`: the `Hash` interface.
- SGO-STD-018, 019 — `std/crypto/hmac`, `std/crypto/hkdf`.
- SGO-STD-020 — `std/crypto/cipher`: the `AEAD` interface.
- SGO-STD-021, 022 — `std/crypto/chacha20poly1305` (RFC 8439) and `std/crypto/curve25519` (RFC 7748 X25519).

All are checked against Go's packages on generated inputs and against the Wycheproof vectors that Go's own crypto tests use (`sgo/scripts/test-std-crypto.py`).

Amended clauses:

- SGO-BIND-003 v2 — package-level tables: `var name = sgo.Freeze(lit)` with a constant composite literal, built once before `main`, frozen for every task (D136); the text now also names type declarations as SGO-UTYPE-002 admits them.
- SGO-TYPE-001 v11 — `uint32` and `uint64` are admitted types.

## 0.20.1 — 2026-11-07

Review fix (W25 review P3). Every program valid under 0.20.0 keeps its meaning: the text now names a type the compiler and runtime already treated as a built-in resource.

Amended:

- SGO-RES-001 v4 — the list of built-in resource types names `*sgo.Timer` (SGO-WAIT-004 already made a timer a resource of the scope it was made in).
- SGO-RES-002 v5 — a timer is shared within its scope like the other built-in resources (fixture `timer-scope-child`); `res-kind-check` now requires both lists to name exactly the runtime's built-in handle types.

## 0.20.0 — 2026-11-07

`float64` (W25; D185, D187–D194). The arithmetic is IEEE 754 binary64, rounded once per operation and never fused, so a program computes the same bits on every target; one NaN is observable; conversions to integers are checked. Not every 0.19 program keeps its meaning: `std/math`'s `Abs`, `Max`, `Min`, and `Pow` now take `float64`, and the integer versions are `IntAbs`, `IntMax`, `IntMin`, `IntPow` (D191). Every other 0.19 program keeps its meaning.

New clauses:

- SGO-FLOAT-002 — the `float64` type: literals, `+ - * /`, comparisons, IEEE 754 values with round-to-nearest-even and no contraction (the one fused operation is `math.FMA`), no float operation fails, one observable NaN, no float map keys (D187, D188, D190).
- SGO-FLOAT-003 — conversions: `float64(i)` rounds to nearest-even; `int(f)`, `byte(f)`, `rune(f)` truncate toward zero and fail with `IntegerOverflow` for NaN, infinities, and out-of-range values; `string(f)` is denied (D189).

Amended clauses:

- SGO-FLOAT-001 v2 — only `float32`, the complex types, imaginary literals, `real`, `imag`, and `complex` stay denied.
- SGO-LEX-004 v3 — decimal and hexadecimal floating-point literals.
- SGO-TYPE-001 v10 — `float64` is an admitted type.
- SGO-VAL-004 v2 — untyped floating-point constants are exact and rounded once when given type `float64`.
- SGO-COMP-002 v3 — `float64` is a value type.
- SGO-COMP-006 v2 — a map key type may not contain `float64`.
- SGO-STD-004 v2 — `strconv.ParseFloat` and `strconv.FormatFloat`.
- SGO-STD-007 v2 — `std/math` float functions: the exact set and Go's portable transcendentals, identical on every target; the integer functions renamed `Int*` (D191, D192).
- SGO-STD-009 v2 — `fmt` float verbs `%v %f %F %e %E %g %G %b %x %X`, the `+` flag, and precision for every verb (D193).
- SGO-RES-002 v4 — the tasks of a scope share its built-in resources; a user resource stays task-local and crosses tasks only by a move (W24 review P1).
- SGO-RES-004 v6 — the same exception stated in the user-resource clause (W24 review P1).
- SGO-RES-005 v2 — how helpers compose with sends: return the resource, send it from the scope that owns both (W24 review P2; parameter-lifetime summaries are D195, proposed).

## 0.19.1 — 2026-11-07

A plain error exit for command-line programs (D186). Every 0.19.0 program keeps its meaning.

Amended clauses:

- SGO-PROC-002 v3 — `sgo.ExitError(msg)`, returned from `main` itself, exits 1 with only `msg` and a newline on stderr and no record; `sgo.IsExitError`. From a child or wrapped, it is an ordinary error.

## 0.19.0 — 2026-11-07

Command-line programs as first-class (W24; D178–D184, D177 withdrawn). Every 0.18.0 program keeps its meaning: the additions are new APIs and grants, and the one change of behavior is that SIGINT and SIGTERM now cancel the root instead of ending the process outside S-Go's outcomes (SGO-PROC-005).

New clauses:

- SGO-PROC-005 — interrupts: the first SIGINT or SIGTERM cancels the root (`sgo.IsInterrupted`); the program unwinds with its own status; a second signal exits 130 or 143 (D179).
- SGO-LAB-007 — lab standard input, environment, and interrupts: `sgo lab run|sweep -stdin FILE -env NAME=VALUE -interrupt MS[,MS]`, recorded in the trace and replayed (D178–D180).
- SGO-STD-009 — `std/fmt`: `Sprintf`, `Fprintf`, `Errorf`, `Wrapf` over an explicit `[]any`, verbs `%s %q %d %x %c %t %v`, Go's format-error forms (D183).
- SGO-STD-010 — `std/time`: `Duration`, `ParseDuration`, `String`, the unit constants; no clock (D183).

Amended clauses:

- SGO-PROC-002 v2 — the statuses 130 and 143 after a second interrupt.
- SGO-PROC-003 v2 — `root.Stderr()`; `Write`, `IsTerminal`, `TerminalWidth`; a write to a closed pipe returns an error for which `sgo.IsBrokenPipe` holds, and SIGPIPE never ends the process (D178).
- SGO-PROC-004 v3 — `root.Args()` (D178), `root.Getenv` for `grant env NAME|PREFIX*` (D180), and `root.Stdin()` with `ReadLine`, `ReadAll`, `ReadSecret`, `IsTerminal` (D178).
- SGO-FILE-002 v5 — per-user application directories: `grant dir config|state|cache ro|rw` and `app NAME` (D182); argument paths: `grant path args ro|rw`, `root.ArgDir`, `root.OpenArg`, `root.CreateArg` (D181).
- SGO-NET-002 v4 — sockets in the application's runtime directory: `unix:@runtime/NAME` (D182).

Floating point stays denied (SGO-FLOAT-001): `std/fmt` has no float verbs and `std/strconv` no `ParseFloat`; admitting `float64` is D185, for a later week. The CLI library of D184 is an example, `sgo/examples/cli`, not part of the specification.

## 0.18.0 — 2026-11-06

The concurrency library and resource interfaces (W23; D173–D176) and the errata of the 0.17.0 review. Not every 0.17.0 program keeps its meaning: an error exit (a `return` with a non-nil error, a failing `check`) cancels each nursery it crosses, as implemented since W22 (SGO-CTL-004 v2); `Own` of a non-pointer value is refused; a start into a cancelling nursery publishes the cancellation instead of deadlocking.

New clauses:

- SGO-FILE-004 — whole-message reads: `ReadLine`, `ReadAll`, `sgo.IsTooLong`; one read turn per handle (D164, D173).
- SGO-SPAWN-008 — request/reply calls: `sgo.NewCall[Req, Resp](n)`, `Call`, `Accept`, `Request.Reply`, `sgo.IsNoReply` (D167, D174).
- SGO-SPAWN-009 — pools with leases: `sgo.NewPool[T](n)`, `Put`, `Acquire(s)`, `Lease.Value`, `Return`, `Drop` (D175).
- SGO-RES-006 — resource interfaces: an interface embedding `sgo.Resource` holds resources, is tied as what it holds, and is task-local (D176).
- SGO-STD-008 — `std/io`: `Reader`, `LineReader`, `Writer` and helpers over resource interfaces (D176).

Amended clauses:

- SGO-SPAWN-006 v2 — `Result()` channels are edge-triggered, one per call; a start into a cancelling nursery resolves to the cancellation; resources inside an aggregate result move into the nursery; a `Go` child's body is released with `Release(true)`.
- SGO-SPAWN-007 v2 — `g.Go` into a cancelling nursery publishes the cancellation and frees its slot.
- SGO-CTL-004 v2 — an error exit cancels each nursery it crosses; resources there see `Release(false)`; a normal exit joins without cancelling (D159).
- SGO-ERR-004 v3 — a failing `check` is such an error exit.
- SGO-RES-001 v3 — a resource may be converted to a resource interface, which is tied as its source.
- SGO-RES-004 v5 — a pointer to a user resource is task-local (`Own` gives lifetime, not sharing); `Own` takes a pointer (`SGO_E_TYPE_MISMATCH`); `Own` accepts the result of a constructor handed no reference.
- SGO-FILE-003 v3 — reads of one handle run one at a time (the read turn of SGO-FILE-004).
- SGO-IFACE-002 v2 — a handle may be the dynamic value of a resource interface.

## 0.17.0 — 2026-11-05

The language core for structured concurrency (W22; D159–D172). Not every 0.16.2 program keeps its meaning: `LendTo` is removed from effect handles (D169); `sgo.Go` refuses a task-local result type (a slice, map, or pointer result is returned as `sgo.Frozen`); a task or group handle delivering a resource may not leave the nursery the child started in.

New clauses:

- SGO-WAIT-003 — timers as `select` arms: `case <-t.After(ms):` (D160).
- SGO-WAIT-004 — scope timers: `n.NewTimer`, `t.NewTimer`, a resource of its scope, usable in any number of selects, with `Reset` and `Stop` (D171).
- SGO-SPAWN-006 — value-returning tasks: `sgo.Go(n, f)`, `h.Wait(t)`, `case r := <-h.Result():`; the child's error is data; task handles are shared by the nursery's tasks (D161, D168).
- SGO-SPAWN-007 — groups: `sgo.NewGroup[T](n, limit)`, `g.Go(t, f)`, results in completion order through `g.Next()`, queued without bound (D172).
- SGO-CANCEL-005 — nursery deadlines: `n.CancelAfter(ms)`, `t.Deadline()`, `sgo.IsDeadline`; also on virtual time in the lab (D162, D165, D170).

Amended clauses:

- SGO-RES-002 v3 — effect handles have no `LendTo` either (D169).
- SGO-RES-003 v4 — rule (3) admits the callbacks of `sgo.Go` and `g.Go` inside the extent; fixture:res-defer-nursery-close added.
- SGO-RES-004 v4 — `ok` is false on any cancellation, an ancestor's or a deadline's (SGO-CANCEL-005) included.
- SGO-AUTH-002 v4 — children use the file and network handles the root passes to them (no lending).
- SGO-AUTH-003 v3 — effect handles are shareable authority tokens, never revoked; `LendTo` and revocation are removed (D169). Acceptance case A03 (lend and revoke) is no longer referenced.

## 0.16.2 — 2026-11-05

Text errata from the review of 0.16.1 (F1, F2); no behaviour changes. Two amended clauses named features that 0.16.1 does not have.

Amended clauses:

- SGO-RES-003 v3 — rule (3) no longer names `sgo.Go`, which 0.16.1 does not define.
- SGO-RES-004 v3 — `ok` is false on any cancellation, an ancestor's included; the mention of deadlines, which 0.16.1 does not define, is removed.

## 0.16.1 — 2026-11-05

Errata from the W21 review (Z1–Z4): the resource flow rule could be evaded through pointers, interfaces, parameter and receiver stores, call results, `Own` of outer values, and `defer` inside nursery bodies; a nil resource result with a nil error, and a call on a nil handle, were misreported. Programs that relied on those holes no longer compile; every other 0.16.0 program keeps its meaning.

Amended clauses:

- SGO-RES-001 v2 — pointers and maps reaching a resource are resource types; a resource never becomes an interface value (except as an argument of package `sgo`).
- SGO-RES-002 v2 — `LendTo` is removed from resource handles and `*sgo.Dir`; effect handles keep it until D169.
- SGO-RES-003 v2 — stores through pointers, slices, maps, and call results are checked against where that memory may be reached from; stores between parameters are escapes; `defer` inside a nursery body may not use that nursery's resources.
- SGO-RES-004 v2 — `Own` requires a value made inside the scope; `ok` is false on any cancellation, ancestor's or deadline's included.
- SGO-ERR-004 v2 — `return nil, err` for a resource result checks `err` at run time (`InvalidArgument` at the `return`).
- SGO-COMP-007 v2 — a method call on a nil handle fails with `NilDereference` at the call.

## 0.16.0 — 2026-11-05

Structured resources and `check` (W21; D147–D158). Resources belong to scopes — nursery bodies and task bodies — which release them when they end, after their children, in reverse order of acquisition; tasks inside a scope share its resources; the compiler refuses every flow that would let a resource outlive its scope (`SGO_E_RESOURCE_ESCAPES`). `check c` propagates an error. Not every 0.15.1 program keeps its meaning: `LendTo`, `sgo.ConnSlot`, and the `HandleAbandoned` outcome are removed; `Accept`, `Dialer.Dial`, `Dir.Open`, and `Dir.Create` take a scope as their first argument; `check` is a reserved word.

New frozen clauses:

- SGO-ERR-004 v1 — `check`: checked propagation (D147); a function with a resource result returns only the error on its error path, and a written `return` may give `nil` for a resource result exactly when its error operand is not the literal `nil`.
- SGO-RES-001 v1 — scopes own resources; release after the join, in reverse acquisition order; failed releases are never dropped (`ReleaseError`, D148, D149).
- SGO-RES-002 v1 — sharing a resource within its scope; `Close` by any task inside it, `ResourceBusy` while another waits (D150).
- SGO-RES-003 v1 — a resource never outlives the name of its scope; `MoveTo` and `sgo.MoveTo` move it outward (D151, D158).
- SGO-RES-004 v1 — user resources: `s.Own(r)` and `Release(ok bool) error`, commit or roll back; a second `Own` fails with `InvalidArgument` (D152).
- SGO-RES-005 v1 — hand-off within a scope: channels may carry resources that outlive them; the deciding scope owns the candidates (D158).

Amended:

- SGO-LEX-003 v2 — `check` is a reserved word.
- SGO-CTL-001 v3 — `check c` is a statement and a use of the error of `c`.
- SGO-NUR-004 v2 — the release step between the join and the outcome; a failed release in an otherwise successful nursery unwinds with `ReleaseError`.
- SGO-SPAWN-004 v2 — a task's outcome includes the release of its body's resources.
- SGO-FILE-002 v4 — `Dir.LendTo` removed; `Open` and `Create` take a scope; a `Dir` is never released or lent.
- SGO-FILE-003 v2 — handle ownership by scopes (SGO-RES-001/002); `LendTo` and `HandleLent` removed; `MoveTo` added.
- SGO-NET-002 v3 — `Accept` takes a scope; children use the resources of the scopes they run in.
- SGO-NET-003 v3 — a dialer is a resource of the scope that made it; `Dial` takes a scope; a race is an ordinary nursery.

Superseded:

- SGO-NET-004 v2 — connection slots, superseded by SGO-RES-003 and SGO-RES-005 (D158; D157 withdrawn).

## 0.15.1 — 2026-11-04

Review fixes (W18 review Y2–Y4). Programs valid under 0.15.0 keep their meaning; `Dialer.Resolve` returns fewer addresses in three cases that widened authority or trusted unrelated data.

Amended:

- SGO-NET-003 v2 — unspecified (`0.0.0.0`, `::`) and IPv4-mapped addresses are never resolution results, and lab tables may not contain them; a DNS response must echo the question, and only records of the host or its CNAME chain count; re-resolving adds no duplicate address.

## 0.15.0 — 2026-11-04

Happy Eyeballs (W18; D143–D146, owner-delegated decisions). Every program valid under 0.14.1 keeps its meaning. A connect grant may now name a host, but such a grant gives `root.Dial` nothing: only a `Dialer` resolves and dials a name.

New frozen clauses:

- SGO-NET-003 v1 — dialers: delegated connect authority for granted endpoints (D143); host-name connect grants and `Dialer.Resolve` (/etc/hosts, then DNS A/AAAA over UDP; D145); the lab network table `lab.net` (D146).
- SGO-NET-004 v1 — connection slots: one connection handed between tasks, first `Put` wins (D144; model `sgo/model/handoff.go`).

Amended:

- SGO-NET-002 v2 — host names are admitted in connect grants for dialers (SGO-NET-003); `root.Dial` refuses them; listen grants stay IP or Unix.

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
