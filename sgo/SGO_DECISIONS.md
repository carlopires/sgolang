# S-Go open design decisions

**Version:** 1 · **Date:** 2026-10-30 · **Status:** D01–D22 APPROVED by the user on 2026-10-30 (D01–D21 as recommended; D22 as the user specified); D23–D30 APPROVED on 2026-10-31 as recommended; D31–D39 APPROVED on 2026-11-01 as recommended.

## Purpose and precedence

The constitution fixes the user's requirements; the execution plan fixes architecture. Several questions that shape the specification are still open or delegated to the implementing agent. Each is listed here with options and a recommendation so the user can decide **before** W01 freezes clauses.

Once approved, a decision sits between the constitution and the execution plan in precedence. A decision that conflicts with the plan amends the plan; it never weakens the constitution. Rejected options stay recorded with their reason.

Most decisions also define **subset 0**, the closed language subset admitted by the first vertical slice (W00–W04, plan §15.1). Anything not admitted by a decision below is denied in subset 0. `SGO_SPEC_SUBSET0.md` (to be drafted after approval) turns approved decisions into normative clauses.

Legend: **[USER]** = needs an explicit user choice (changes language semantics). **[DEFAULT]** = the agent may proceed on the recommendation unless the user objects.

## Summary

| ID | Question | Decision | Kind | Status |
|---|---|---|---|---|
| D01 | How are data races excluded? | Static: no shared mutable memory between tasks in subset 0 | USER | APPROVED |
| D02 | Integer overflow | Trap (checked runtime failure); explicit wrapping ops later | USER | APPROVED |
| D03 | Floating point | Denied in subset 0 | DEFAULT | APPROVED |
| D04 | Types admitted in subset 0 | `bool`, `int` (64-bit), `string`, func values, sealed `error` | USER | APPROVED |
| D05 | User types, methods, interfaces, generics | Denied in subset 0 | DEFAULT | APPROVED |
| D06 | Pointers, slices, maps, arrays, structs | Denied in subset 0; map order decided later (D20) | DEFAULT | APPROVED |
| D07 | Package-level state and `init` | Constants only; no mutable package vars, no `init` | USER | APPROVED |
| D08 | Program entry and exit | `func main(root *sgo.Context) error`; specified exit codes | USER | APPROVED |
| D09 | Output effect | `root.Stdout()` capability; `print`/`println` rejected | USER | APPROVED |
| D10 | Test harness | Whole-program golden tests; no `testing` package in subset 0 | DEFAULT | APPROVED |
| D11 | Spawn/nursery spelling | Keep `nursery (n) { }` and `n.Spawn(func(*sgo.Context) error)` | USER | APPROVED |
| D12 | `defer` | Function-scoped (plan default); admitted in subset 0 | DEFAULT | APPROVED |
| D13 | `panic` / `recover` | `panic(string)` admitted; `recover` denied in subset 0 | USER | APPROVED |
| D14 | Control flow | `if`/`for`/`switch`/`return`/labeled `break`/`continue`; no `goto` | DEFAULT | APPROVED |
| D15 | Packages and imports | Multiple user packages; only `sgo` from the library | DEFAULT | APPROVED |
| D16 | Resource exhaustion | Stack overflow / OOM are fail-stop, not recoverable | DEFAULT | APPROVED |
| D17 | User-level contracts/proofs (Bend-style laws) | Out of scope for v1; revisit after core release | USER | APPROVED |
| D18 | Specification format and identity | Markdown clauses with stable IDs + JSON admission digest | DEFAULT | APPROVED |
| D19 | Reusing Go-spec text | Rewrite in S-Go clauses; quote with attribution where useful | DEFAULT | APPROVED |
| D20 | Map iteration order (post-subset 0) | Deterministic insertion order | USER (can wait) | APPROVED |
| D21 | Upstream pin | `go1.27.3` instead of `go1.27.1` | DEFAULT | APPROVED |
| D22 | Execution targets | darwin/arm64 first (development); linux/amd64 is the real target; linux/arm64 first-class | USER | APPROVED |

---

## D01 — Data-race exclusion **[USER]** · APPROVED 2026-10-30

**Why it matters.** The constitution forbids unrestricted unspecified behavior. In Go, a data race on a multi-word value (string, slice, interface) can tear and corrupt memory — exactly that. The plan (§1.4) requires W01 to settle races but gives no mechanism. This decision shapes SPEC-MEM, SPEC-TYPE, and the runtime.

**Options.**
- **(a) Static exclusion.** Tasks cannot share mutable memory; data crosses tasks only by copy (later: channels, ownership transfer). Strongest; needs compiler checks; makes many Go idioms invalid.
- **(b) Specified racy outcomes.** Allow sharing; define every racy read as returning some previously written value; ban sharing of types that can tear. Requires memory-model work comparable to Java/C++ and still permits logic races.
- **(c) Detection with fail-stop.** Rejected: a detector can miss races, so the behavior of an undetected race remains unspecified.
- **(d) Synchronization-only sharing.** Shared state only behind admitted sync types (mutex/atomic cells). Sound only if access outside them is also statically impossible, so it reduces to (a) plus library types.

**Recommendation: (a) for subset 0, extended by (d) later.**
Subset 0 rule set (to become clauses in SPEC-MEM/SPEC-SPAWN):
1. A function literal passed to `Spawn` may capture only variables that are **never assigned after capture**, in the parent or any task (checked at compile time, like Java's "effectively final").
2. Captured values must be of a **sendable** type. In subset 0 every admitted type (D04) is immutable or a capability, so all are sendable; the rule exists for later subsets.
3. No mutable package-level variables (D07), no pointers (D06).
4. Consequence: data races are statically impossible in subset 0. Tasks communicate only via their `error` result and effects (D09). Channels arrive in W06/W07 with their own sendability rules.

---

## D02 — Integer overflow **[USER]** · APPROVED 2026-10-30

**Options.** (a) wrap two's-complement (Go behavior); (b) trap: overflow is a specified checked runtime failure; (c) both, selected by operator spelling.

**Recommendation: (b)** for `+ - * /`, unary minus, and conversions. Wrapping arithmetic is added later as explicit library functions (e.g. `sgo.WrapAdd`), not a mode. Division by zero and `MinInt / -1` are the same checked failure. Shift counts outside `0..63` are a checked failure. Trap failures behave like `panic` (D13) with a distinct, specified failure kind.

Cost: a branch per operation; the optimizer may remove proven-safe checks. Wrapping is still fully specified, so (a) is acceptable if the user prefers Go-like arithmetic; the choice is about safety and taste, not admissibility.

## D03 — Floating point **[DEFAULT]** · APPROVED 2026-10-30

Denied in subset 0. When admitted: IEEE 754 binary64/binary32, round-to-nearest-even, specified NaN comparison and NaN-to-int conversion outcomes (checked failure), and no fused multiply-add unless explicitly specified, so results are reproducible across lab and production.

## D04 — Types admitted in subset 0 **[USER]** · APPROVED 2026-10-30

| Type | Notes |
|---|---|
| `bool` | `&&`, `||`, `!`, `==`, `!=`. |
| `int` | Exactly 64-bit signed. Other integer widths denied. Overflow per D02. |
| `string` | Immutable byte sequence; `len`, `+`, `==`, `!=`, `<` etc. (bytewise). Indexing, slicing, and `range` over strings denied (they need byte/rune types and bounds rules). |
| function values | Needed for `Spawn` callbacks; closures obey D01 capture rules. No function equality. |
| `error` | **Sealed**: values come only from `nil` and `sgo.NewError(msg string) error`; operations are `== nil`, `!= nil`, `.Error() string`. No user implementations (interfaces are denied, D05). |
| `*sgo.Nursery`, `*sgo.Context` | Opaque capabilities; no `nil` literal of these types in user code, no comparison, no conversion. |

The unnamed constants `true`, `false`, untyped integer and string constants are admitted with S-Go-specified constant rules (arbitrary-precision constant arithmetic, overflow on conversion to `int` is a compile error).

## D05 — User types, methods, interfaces, generics **[DEFAULT]** · APPROVED 2026-10-30

Denied in subset 0. Each is a large clause family (method sets, dynamic dispatch and effect summaries for interface calls, instantiation closure for generics). They are admitted later only with the indirect-call/effect rules of plan §1.4. The `sgo` package's own methods (`Spawn`, `Stdout`, etc.) are compiler-known and specified individually.

## D06 — Pointers, slices, maps, arrays, structs **[DEFAULT]** · APPROVED 2026-10-30

Denied in subset 0. Structs and arrays of sendable types are likely the next admission (value types, no aliasing). Pointers, slices, and maps wait for the aliasing rules of D01's later extension. Map iteration order is D20.

## D07 — Package-level state and `init` **[USER]** · APPROVED 2026-10-30

**Recommendation:** subset 0 admits package-level `const` and function declarations only. Package-level `var` and `func init()` are denied. Rationale: mutable globals are shared memory (D01) and initialization order is a large clause family. Later, immutable package-level values with a specified initialization order may be admitted.

---

## D08 — Program entry and exit **[USER]** · APPROVED 2026-10-30

Plan §4.2 says the root task gets no implicit process-lifetime nursery, but does not say how `main` gets any authority. Go's `func main()` has none to give.

**Recommendation:**

```go
func main(root *sgo.Context) error
```

- `root` carries the process's initial capabilities (in subset 0: stdout only, D09) and its cancellation.
- `func main()` with the Go signature is rejected (`SGO_E_MAIN_SIGNATURE`).
- Process exit status is specified:

| Outcome | Exit status |
|---|---|
| `main` returns `nil` | 0 |
| `main` returns a non-nil `error` | 1; the message goes to the diagnostic stream |
| Unrecovered panic or checked runtime failure (D02, D13) | 2 |
| Drain-contract fail-stop (plan §6.4) | 3 |
| Resource exhaustion (D16) | 4 |

The numbers are proposals; the important part is that each outcome is distinct and specified.

## D09 — Output effect **[USER]** · APPROVED 2026-10-30

Nothing in the bundle specifies how a program produces output. Go's `print`/`println` builtins write to stderr in an implementation-defined format, which fails the constitution.

**Recommendation:** `root.Stdout()` returns an opaque `*sgo.Writer` capability with `WriteString(s string) error`. A child obtains it only through capture of the handle (D01) or an explicit parameter, never ambiently. Concurrent writes from tasks are atomic per call and ordered by a specified linearization; the lab records them. `print`, `println`, and `fmt` are denied. The diagnostic stream (stderr) is written only by the runtime in subset 0.

## D10 — Test harness **[DEFAULT]** · APPROVED 2026-10-30

**Recommendation:** subset-0 conformance tests are whole S-Go programs with golden expectations: exit status, stdout bytes, and the compile diagnostic code for rejection cases. A small host-side runner (external tooling, plan §10.2) executes them. The `testing` package and `sgo test` are denied in subset 0 and specified later with W10.

## D11 — Spawn and nursery spelling **[USER]** · APPROVED 2026-10-30

**Options.**
- (a) Keep the plan default: `nursery (n) { ... }` block plus method `n.Spawn(func(task *sgo.Context) error)`.
- (b) A `spawn n f(args)` statement keyword.
- (c) Library only: `sgo.WithNursery(func(n *sgo.Nursery) error)`. Rejected by plan §5.4: a hidden closure changes `return`/`break`/`defer` targets.

**Recommendation: (a).** One new keyword; spawning stays an ordinary, checkable method call; callbacks carry an explicit context. The anonymous form `nursery { }` is denied in subset 0 (it adds no capability and can come later).

## D12 — `defer` **[DEFAULT]** · APPROVED 2026-10-30

Keep the plan §4.5 default: `defer` is function-scoped; nursery barriers run before that frame's defers during unwinding. Admit `defer f()` (call of a function value or named function, arguments evaluated at the `defer` statement) in subset 0, because the first slice exercises panic and cleanup ordering. `n.Defer` scope finalizers are denied in subset 0.

## D13 — `panic` and `recover` **[USER]** · APPROVED 2026-10-30

**Recommendation:** admit `panic(msg string)` only (the builtin takes `any` in Go; interfaces are denied). **Deny `recover` in subset 0.** Consequences:
- Every panic and checked runtime failure in a child terminates that child, cancels its siblings, and re-panics at the nursery join (plan §4.5), so the first-slice panic scenario is covered.
- The plan's §4.6 cancellation-unwind interception question does not arise in subset 0, because nothing can intercept the unwind. That removes a large clause family from the first freeze.
- `recover` is admitted later together with `sgo.IsCancellation` and its interaction rules.

## D14 — Control flow **[DEFAULT]** · APPROVED 2026-10-30

Admit `if`/`else`, `for` (three-clause, condition-only, infinite), expression `switch` on `int`/`string`/`bool` without `fallthrough`, `return`, and labeled/unlabeled `break` and `continue`, including exits that cross nursery boundaries (plan §4.4). Deny `goto`, `fallthrough`, `select`, type switches, and `for range`.

## D15 — Packages and imports **[DEFAULT]** · APPROVED 2026-10-30

Admit multiple user packages in one module, each passing admission with recorded contract digests. That is needed for the first-slice "dependency-stale contract" rejection case. The only library import is the canonical `sgo` package (exact import path fixed in SPEC-BIND). Import aliases, dot and blank imports are denied. Unused imports and variables are compile errors (an S-Go rule, not a Go inheritance).

## D16 — Resource exhaustion **[DEFAULT]** · APPROVED 2026-10-30

Stack overflow, heap exhaustion, and exceeding scope quotas are fail-stop: the process terminates with exit status 4 (D08) and a runtime diagnostic. They are not panics and cannot be recovered. Limits are recorded in run manifests; the lab can inject them deterministically.

## D17 — User-level contracts and proofs **[USER]** · APPROVED 2026-10-30

Bend 2 ([R18]) shows an approach: properties stated as types (`LAWS.bend`) that the compiler forces code to prove. The constitution only says a proof or runtime-checking discipline is needed if such contracts exist, and forbids claiming automatic verification of arbitrary algorithms.

**Recommendation:** no user-level pre/postconditions or proofs in v1. S-Go's guarantee is that every admitted construct has a specification and enforced legality, not that programs meet their own functional specs. Revisit after the core release; a later design could add laws/proofs as a separate admitted feature with its own kernel and checking discipline.

## D18 — Specification format and identity **[DEFAULT]** · APPROVED 2026-10-30

- Normative text lives in Markdown under `sgo/spec/`, one file per family (`SPEC-LEX.md`, ...).
- Clause IDs: `SGO-<FAMILY>-<NNN>` (e.g. `SGO-SPAWN-004`), never reused; each clause records version and status (`draft`, `frozen`, `superseded`).
- Clause digest: SHA-256 over the clause text normalized to LF line endings with trailing whitespace stripped.
- `SGO_SPEC_ADMISSION.json` rows reference clause IDs and digests; `spec_version` and `admission_digest` are computed by a checked-in tool, never edited by hand.
- Each clause covers the seven contract dimensions of constitution §3, or states a reason why one does not apply.

## D19 — Reusing Go-spec text **[DEFAULT]** · APPROVED 2026-10-30

Write S-Go clauses in S-Go's own terms. Where Go spec wording is a good starting point, quote or adapt it with attribution and a record of the source revision, after checking the license terms of the Go documentation. Adapted text is S-Go text from then on: it does not follow later Go changes and carries no "same as Go" fallback (constitution §3).

## D20 — Map iteration order **[USER, can wait]** · APPROVED 2026-10-30

Not needed for subset 0. Go randomizes map iteration, which the lab must reject (plan §9.2). **Recommendation when maps are admitted:** deterministic insertion order. It removes a whole lab nondeterminism source and an inventory exception, at some memory/performance cost. Alternative: specified unordered iteration as allowed nondeterminism, recorded by the lab.

## D21 — Upstream pin **[DEFAULT]** · APPROVED 2026-10-30

Checked 2026-10-30 against `go.googlesource.com/go`:

| Tag | Commit |
|---|---|
| `go1.27.1` (current default) | `862c888e612ac346c7c4d99c9392bdfd265f33b0` |
| `go1.27.3` (latest patch) | `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` |

**Recommendation:** pin `go1.27.3`, the latest patch release in the same minor line (likely security/bug fixes, same source map). W00 still verifies source/bootstrap checksums and records the reason in `UPSTREAM.lock.json`.

---

## D22 — Execution targets **[USER]** · APPROVED 2026-10-30

User decision: *"let's do macOS directly first, because I'm working from it, so it is easy to test results from here. linux/amd64 is the real target, so any decision must take this into consideration. linux/arm64 is also first-class."*

| Target | Role |
|---|---|
| darwin/arm64 | **First development target.** The first slice is built, run, and debugged natively here (the user's host). |
| linux/amd64 | **The real (production) target.** Every design decision must be valid for it. No work package gate passes on darwin/arm64 evidence alone once the work package touches target-dependent code. |
| linux/arm64 | **First-class target.** Same standing as linux/amd64 for conformance; not deferred to "later". |

Rules that follow:
1. **No darwin-only design.** Runtime, scheduler, timer, I/O, and TCB contracts are written against all three targets. A darwin mechanism (kqueue, Mach, libSystem calls) is an adapter behind a target-neutral contract, never the contract itself.
2. **Memory ordering.** arm64 has a weaker memory model than amd64. Developing on arm64 first exposes ordering bugs early, but code must not rely on amd64 TSO either: synchronization uses the runtime's atomics with specified ordering.
3. **Linux evidence.** Every work package that changes compiler/runtime/target behavior records conformance evidence on linux/amd64 and linux/arm64 as well as darwin/arm64 before its gate passes. GitHub Actions provides hosted linux/amd64 and linux/arm64 runners for the public repository; a local Linux VM is an alternative. Cross-compilation alone is not evidence of execution (plan §1.2).
4. **Darwin specifics.** Darwin calls go through libSystem in the upstream runtime (no raw syscalls); that boundary is a TCB adapter (SPEC-TCB). Linux uses raw syscalls in the upstream runtime; both paths need individual contracts.
5. **Plan §1.2 is amended accordingly** (first target, later-target row).

---

# Subset 1 (W05, W06) — APPROVED 2026-10-31 as recommended

Spec 0.1.0 (subset 0) denies the features W05 and W06 implement: readiness handshakes (SGO-READY-001), reserve/commit permits (SGO-PERMIT-001), every clock and timer API but `Sleep` (SGO-TIMER-001), randomness (SPEC-RANDOM), generics (SGO-GEN-001), and slices, composite literals, `&`, and package variables (SGO-COMP-001, SGO-BIND-003). W05/W06 therefore start with a specification amendment, and new user-visible API is a language change that needs approval (plan §13: major changes "cannot be slipped into a patch without an explicit decision"; D01–D22 precedent). Approved by the user on 2026-10-31 as recommended.

## D23 — Spec version and subset 1 **[DEFAULT]** · APPROVED 2026-10-31

**Recommendation.** Subset 1 is spec **0.2.0**, a strict superset of 0.1.0: every 0.1.0 program keeps its meaning (the 194 conformance cases stay valid, reject cases whose construct becomes admitted move to positive cases with a recorded reason). Denial clauses that 0.2.0 admits are replaced by new clauses with a version bump and a CHANGELOG entry (spec-tool's frozen-clause rule). The toolchain identity changes with the admission digest, so 0.1.0 objects are refused by a 0.2.0 toolchain (SGO-IMP-002), as designed. One spec version covers W05 and W06; W07 (native channels) gets 0.3.0.

## D24 — Authority representation **[USER]** · APPROVED 2026-10-31

**Why it matters.** This is the user-visible capability model (plan §8.2) and constrains every later effect (files, network in W09).

**Options.**
- **(a) Root-held capability handles, passed explicitly** — the pattern subset 0 already uses for stdout (`root.Stdout()`, SGO-PROC-003): `root.Clock()`, `root.Random()`, … are available only on the root context, as values of opaque types, and code that needs an effect takes the handle as a parameter. Attenuation returns a narrower handle (`clk.Monotonic()` — no wall time); revocation is scoped: `n.Lend(h)` gives a handle valid only while nursery `n` is open (use after close fails before any effect).
- **(b) Task ceilings only** — spawn declares the grants a child may use (`n.SpawnWith(grants, f)`), effects check the calling task's ceiling. No handles in signatures; authority is ambient within a task.
- **(c) Both** — handles carry authority and every effect also checks the task's inherited ceiling (plan §8.2: "effective authority is bounded by both").

**Recommendation: (c), with (a) as the source-level shape.** Handles keep authority visible in signatures (least privilege is checkable by reading code); the per-task ceiling, inherited at spawn and only narrowed, makes a handle smuggled to a task that was spawned without that grant fail before its effect (A02, A10). Root grants come from the program's `sgo.mod` (D25), never from a public constructor; handles are opaque, cannot be forged, compare by identity only (like `*sgo.Nursery`, SGO-CAP-001), and have a stable trace ID.

## D25 — Root grant policy **[USER]** · APPROVED 2026-10-31

**Options.** (a) every program gets every subset-1 effect; (b) the program declares the effects it may use in `sgo.mod` (`grant clock`, `grant random`, …), the driver records them in the build ID, and `root.X()` for an undeclared grant fails with `MissingCapability`; (c) a separate runner policy file supplied at run time.

**Recommendation: (b).** The policy is reviewable with the source, fixed per binary (like the drain budget), and needs no run-time configuration (no environment variable or flag may widen it, consistent with SGO-TOOL-001). (c) can be added later for deployment policies without changing programs.

## D26 — First effects **[USER]** · APPROVED 2026-10-31

**Recommendation (plan §8.3 order):**
1. **Clock:** monotonic time in ms since program start (`clk.Since()`), and a separate wall-clock grant (`clk.Wall()`, UTC ms since the epoch). Monotonic and wall are distinct grants so code can be given one without the other.
2. **Deterministic randomness:** `rnd := root.Random(seed)` gives a seeded PRNG handle (algorithm fixed by the spec, e.g. PCG, so sequences are reproducible across targets); **secure entropy** is a separate grant (`root.Entropy()`, OS CSPRNG) and is never substituted by the seeded generator (A08).
3. **Simulated effects:** an in-memory key/value "file" store and loopback message endpoint as fake adapters for testing grants, attenuation, revocation, and domains (A01–A05) without real I/O; real files/network stay W09.

This needs ints and strings only (D29); values like random bytes are returned as strings.

## D27 — Readiness handshake **[USER]** · APPROVED 2026-10-31

**Recommendation (Trio's `start`, plan W05):** `err := n.Start(func(task *sgo.Context, ready *sgo.Ready) error)`. The child is registered in `n` before it runs; the caller waits (a managed wait, cancellation point) until the child calls `ready.Signal()` exactly once (R01), or ends/fails/is cancelled first, in which case `Start` returns that child's error or a cancellation error and the child's outcome is still published to `n` normally (R02, R04: no detached or unaccounted child). A second `Signal` is a `SpawnContractViolation` diagnostic in the child and does not change the accepted readiness (R03). Ready signalling after the waiter was cancelled is accepted and ignored.

## D28 — Typed permit channels without general generics **[USER]** · APPROVED 2026-10-31

**Why it matters.** Plan §7.1 wants a typed managed-channel API before native `chan` (W07); subset 0 has no generics, so "typed" needs a decision.

**Options.**
- **(a) Admit a generics subset in 0.2.0** (type parameters on functions and types, no constraints beyond `any`/comparable): large frontend and spec work (GEN clauses, instantiation, export), delays W06 substantially.
- **(b) One built-in parameterized type, `sgo.Channel[T]`,** whose element type is checked by the compiler (like `chan T` itself), while user-defined generics stay denied. `ch := sgo.NewChannel[T](capacity)`; `p, err := ch.Reserve(task)` → `*sgo.SendPermit[T]` with `p.Commit(v)` / `p.Abort()`; `v, ok, err := ch.Receive(task)`; `ch.Close()`. Element types limited to subset-1 sendable types (D29).
- **(c) Untyped channels of `string` only** in W06: smallest, but not the typed oracle the plan asks for.

**Recommendation: (b).** It gives the plan's typed oracle with the native-channel element typing W07 will need, without opening user generics. Permits follow plan §7.2/7.3 exactly (task-affine, aliases share one terminal state, abandoned permits aborted at task end with a diagnostic, body-exit abort before join, close-vs-commit linearization). Receive permits stay internal (plan §7.4).

## D29 — Value types in subset 1 **[USER]** · APPROVED 2026-10-31

**Options.** (a) no new value types: subset-1 APIs use ints, bools, strings, errors, and the new handles; (b) also admit immutable structs and slices of sendable types (requires D01's sharing rules to be extended to aggregates and a copy/immutability story); (c) full Go value types.

**Recommendation: (a) for 0.2.0, (b) as its own decision before W07/W09.** W05/W06's acceptance cases need only scalars and strings. Aggregates change the data-race exclusion of D01 (subset 0 is race-free because every admitted type is immutable or a capability), so they deserve their own design pass rather than riding along.

## D30 — Model first, then runtime **[DEFAULT]** · APPROVED 2026-10-31

**Recommendation.** W06 extends the executable model (`sgo/model`) with channels, permits, close, and cancellation, explores bounded histories, and the runtime implementation is checked against the model's reference histories (plan §7.1, W06 gate) before any public permit is admitted. W05's readiness protocol gets the same treatment for R01–R04 races. Neither work package changes subset-0 behavior.

# Subset 2 (W07) — APPROVED 2026-11-01 as recommended

Native channels, `select`, and cancellation transfer (plan §7, W07). Spec 0.2.0 denies all of it (SGO-CHAN-001). These decisions fix the language-design choices the 0.3.0 clauses need; the plan fixes only the outcome (native operations follow the §7 protocol, never raw Go behavior).

## D31 — Spec version and scope of subset 2 **[DEFAULT]** · APPROVED 2026-11-01

**Recommendation.** Subset 2 is spec **0.3.0**, a strict superset of 0.2.0 (as D23): every 0.2.0 program keeps its meaning; SGO-CHAN-001 is superseded by the native-channel clauses; reject cases whose construct becomes admitted move to positive cases with a recorded reason; objects built for 0.2.0 are refused by a 0.3.0 toolchain (SGO-IMP-002). 0.3.0 covers W07 only: native channel types, send, receive, `close`, `select`, and channel `range`, plus the carry-overs of D38. It follows D30: the executable model gains native operations and `select` first, and the runtime is checked against its reference histories.

## D32 — Native `chan` and `sgo.Channel` **[USER]** · APPROVED 2026-11-01

**Why it matters.** Two channel forms could diverge in semantics, or one could become a raw Go path.

**Options.**
- **(a) One protocol.** Native channels are the managed channels of W06 (TCB-P12): `ch <- v` is a reserve and commit that the sender cannot interleave (it waits like `Reserve`, then publishes once); `v, ok := <-ch` and `v := <-ch` are `Receive`. `sgo.Channel[T]` stays, as the explicit-permit form for code that must reserve before it has the value; both forms share element types (D36), close, and cancellation rules.
- **(b) Native only.** Retire `sgo.Channel[T]` and its permits once native channels exist (a 0.3.0 incompatibility with 0.2.0, contrary to D23).
- **(c) Separate implementations.** Native channels on Go's `hchan` with added checks.

**Recommendation: (a).** One runtime protocol keeps the W06 model and evidence valid for both forms, and keeps 0.3.0 a superset. (c) is the raw-Go path the plan forbids.

## D33 — Cancellation of a blocked native operation **[USER]** · APPROVED 2026-11-01

**Why it matters.** `sgo.Channel` operations return a cancellation error; `ch <- v` and `<-ch` have no error result, so cancellation of a task blocked in them needs another channel to the task.

**Options.**
- **(a) Cancellation unwind.** A send, receive, `range` step, or blocking `select` that waits while a scope linked to the task is CANCELLING (as SGO-CANCEL-002) ends the operation with no effect and unwinds the task: deferred calls run and the task ends **Cancelled** (not Failed: it does not cancel its nursery, SGO-SPAWN-004), exactly as if it had returned the cancellation error. A nursery owner unwinding this way joins its children first (SGO-DEF-005). Operations that can proceed without waiting do not observe cancellation.
- **(b) Explicit only.** Admit a blocking native operation only inside a `select` that also has a cancellation arm (for example `case <-task.Done():`); a plain blocking `<-ch` is a compile error.
- **(c) Implicit arm.** Every blocking native operation is a `select` with a hidden cancellation arm that sets a result flag the program must check.

**Recommendation: (a).** It is the plan's "cancellation unwind from safe user boundaries", keeps channel code as short as Go's, and its outcome (Cancelled, defers run, owned children joined) is already defined for returned cancellation errors. The unwind starts only at a channel wait, an existing cancellation point, so no code is interrupted elsewhere. (b) makes every pipeline stage verbose; (c) adds a check that is easy to forget.

## D34 — `select` **[USER]** · APPROVED 2026-11-01

**Recommendation.**
- **Evaluation:** all channel operands and send values are evaluated once, in source order, before any arm is chosen (as Go).
- **Choice:** if several arms can proceed, one is chosen uniformly at random by the runtime; the program cannot observe the generator. Production does not promise a reproducible choice; the lab scheduler (W08) records and replays it. The alternative, the first ready arm in source order, is deterministic but starves later arms, which Go's semantics were designed to avoid.
- **`default`:** admitted; with `default`, `select` never waits and never observes cancellation.
- **Cancellation:** a `select` without `default` that waits is a cancellation point with the unwind of D33; there is no special cancellation arm (and no `task.Done()` channel in 0.3.0).
- **Linearization:** exactly one arm takes effect; a send arm that loses leaves no reservation (W07 gate: winner/loser cleanup).

## D35 — `close`, closed channels, nil channels, `len`/`cap`, `range`, directions **[USER]** · APPROVED 2026-11-01

**Recommendation.**
- **`close(ch)`:** admitted for any holder of a send-capable channel (as `sgo.Channel.Close`); closing a closed channel has no effect (as SGO-PERMIT-002), unlike Go, which panics.
- **Send on a closed channel:** fails the sending task with a new failure kind `ClosedChannel` (SGO-DEF-003 amended), with no effect; Go panics instead. (The explicit-permit form keeps its error result.)
- **Receive from a closed, empty channel:** returns the zero value and `ok = false` (as Go and `Receive`).
- **Nil channels:** do not exist. Channel types join the no-nil rule of capability handles (SGO-TYPE-005): no nil literal of a channel type, variables must be initialized (SGO-BIND-003), and `make(chan T, n)` is the only constructor, so nil-channel blocking cannot occur. A `select` arm cannot be disabled by setting its channel to nil; programs restructure instead.
- **`len(ch)`, `cap(ch)`:** denied in 0.3.0. They only report instantaneous counts, which tempt check-then-act races and are nondeterministic.
- **`for v := range ch`:** admitted; it ends when the channel is closed and empty, and each wait is a cancellation point (D33).
- **Directional types** `<-chan T`, `chan<- T`: admitted, with Go's implicit conversion from bidirectional channels; a receive-only channel cannot be closed.

## D36 — Channel element types in 0.3.0 **[USER]** · APPROVED 2026-11-01

**Options.**
- **(a) Keep D29:** element types `int`, `bool`, `string`, `error`, for native channels and `sgo.Channel` alike.
- **(b) Add channels of those types** (`chan chan int`), so tasks can hand each other reply channels.
- **(c) Add capability handles as elements,** so authority can be passed through channels.

**Recommendation: (a) plus (b).** Reply channels are a central channel idiom and add no shared mutable state (a channel is synchronized). Handles as elements (c) stay denied in 0.3.0: authority would move between tasks outside spawn, which the ceiling model (D24) does not yet describe. Aggregate values (structs, slices) remain the separate decision D29 deferred, before W09.

## D37 — Shared-memory synchronization **[USER]** · APPROVED 2026-11-01

**Recommendation.** Keep SGO-SYNC-001 denying Go's `sync` and `sync/atomic` entirely in 0.3.0. S-Go subset 0–2 has no mutable state shared between tasks (D01: only immutable values and synchronized handles cross task boundaries), so mutexes and condition variables would have nothing to protect, and wait groups are replaced by nurseries. Coordination between tasks (waiting for an event, bounding concurrent use of a resource) is covered by the structured primitives of D39, not by Go's package. The plan's W07 audit then reduces to evidence that every `sync` route stays rejected, including through dependencies.

## D38 — Carry-overs from 0.2.0 **[USER]** · APPROVED 2026-11-01

**Recommendation.**
- **(a) Generator hand-off.** A `*sgo.Random` returned by `Split` is unowned until its first use; the first task that uses it becomes its owner, and any other task's later use is a `SpawnContractViolation`. A parent can then split one generator per child before spawning it, giving every task a deterministic stream (today only the root task can use a generator). The claim is one atomic step; the original generator stays with its owner.
- **(b) Module path diagnostic.** A module path equal to `sgo`, or to any other TCB package path or path prefix, is rejected with `SGO_E_USAGE` at `sgo.mod` instead of an import cycle (SGO-IMP-001 amended).
- **(c) Simulated network:** not in 0.3.0; it belongs with I/O in W09.

## D39 — Structured synchronization primitives **[USER]** · APPROVED 2026-11-01

**Why it matters.** Without shared memory, Go's mutexes have nothing to protect, but tasks still coordinate: waiting until something happened, and bounding how many tasks use a resource at once (Trio's `Event` and `CapacityLimiter`). Closed channels and buffered channels can imitate both, but only by convention, and a slot taken from a buffered channel is not tied to the task that took it.

**Recommendation.** Add two primitives to 0.3.0, built on the managed-channel runtime and the W06 obligation tracking, and checked against the executable model like channels (D30):
- **`ev := sgo.NewEvent()`** with `ev.Set()` (idempotent; every waiter wakes), `err := ev.Wait(task)` (returns nil once set, or a cancellation error; a cancellation point), and `ev.IsSet() bool`. `Set` happens-before every `Wait` that returns nil.
- **`lim := sgo.NewCapacityLimiter(n)`** (`n >= 1`) with `tok, err := lim.Acquire(task)` (waits in FIFO order for one of `n` slots; a cancellation point; returns a task-affine `*sgo.Token`) and `tok.Release()`. A token is an obligation like a send permit: an unreleased token is released at the owner's nursery-body exit and at task end, with a `TokenAbandoned` diagnostic, so a failing or cancelled task never leaks a slot. Releasing twice, or releasing another task's token, is a `SpawnContractViolation`.
- **Not added:** `Semaphore` (a limiter without borrower tracking, the property worth having), `Lock` (a limiter of 1 with no shared state to protect), and `Condition` (requires shared state).


## D40 — Spec version and scope of the lab **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** The lab is spec **0.4.0**, a strict superset of 0.3.0 (as D23, D31): no program changes meaning and no construct is added. It adds the **lab execution configuration** (plan §1.3: a configuration, not a dialect) with its own clauses: scheduler, virtual time, effect policy, trace and replay, outcomes. A program built for the lab and one built for production are the same source under the same admission; they differ only in the scheduling and effect adapters and in the recorded execution configuration (S14). Lab support is W08 only; simulated network and file fixtures belong with W09.

## D41 — How a lab run is started **[USER]** · APPROVED 2026-11-02

**Why it matters.** The lab must never activate by accident in production, and an environment variable or flag that changes production behaviour would be a mode the plan forbids (S15).

**Options.**
- **(a) Separate lab build.** `sgo lab run [-seed N] [-trace FILE] [-sched random|fifo] [-quota N]`, `sgo lab replay FILE`, and `sgo lab sweep -seeds A..B` build the program as a **lab binary**: the execution configuration `lab` is part of the build identity (objects and binaries of the two configurations refuse to mix, SGO-IMP-002), and the run's manifest is passed by the driver. A production binary contains no path that enables the lab scheduler.
- **(b) Runtime switch.** One binary; the lab is enabled by a manifest file or environment variable at start.

**Recommendation: (a).** It keeps production binaries free of any lab activation path and makes "this trace came from a lab binary of exactly this program" checkable from the identity (D04, S14).

## D42 — Scheduler: single managed user execution **[USER]** · APPROVED 2026-11-02

**Why it matters.** Replay needs every user-visible decision to come from the trace; Go's scheduler, `GOMAXPROCS`, and GC timing must not change them (plan §9.1, D10).

**Options.**
- **(a) Baton scheduler over the S-Go runtime.** Exactly one S-Go task holds the baton and runs; every other task is parked by the S-Go runtime. At each **scheduling boundary** (D43) the running task yields; the scheduler computes the canonical ready set (task IDs in creation order), chooses one (seeded generator or the trace), and hands it the baton. Runtime goroutines (GC, services) still run freely; they never hold the baton and never make user-visible decisions. Upstream `synctest` is a source pattern (durable blocking, fake time, idleness detection), not the mechanism, because it has no choice layer.
- **(b) Extend `testing/synctest` bubbles** with a recorded choice for every runnable goroutine.

**Recommendation: (a).** S-Go already funnels every blocking and wake-up through its own runtime (nurseries, channels, events, sleep, cancellation), so the ready set and every decision are in S-Go code with stable IDs, independent of Go's run queues. (b) would have to intercept Go's scheduler itself.

## D43 — Scheduling boundaries and the step quota **[USER]** · APPROVED 2026-11-02

**Recommendation.**
- **Choice points:** spawn, task end, nursery join, every channel operation and `select`, `Event`/`CapacityLimiter` operations, `Sleep`, `Checkpoint`, readiness, effect operations, and `Writer.WriteString`. At each, the ready set and the chosen task are recorded; a `select` also records its ready arms and the chosen arm (D34).
- **Steps:** a lab build instruments every loop back-edge and function entry of S-Go code with a step counter (a lab-only compiler instrumentation, recorded in the identity). Every `K` steps (manifest, default 1024) the running task reaches a deterministic yield, which is also a choice point, so a busy loop cannot starve others forever. The total steps are bounded by the **quota** (manifest); exhausting it ends the run with `LabQuotaExhausted` (D07, D11).
- Production builds have no such instrumentation.

## D44 — Generator and streams **[USER]** · APPROVED 2026-11-02

**Options.**
- **(a) PCG-DXSM 128** (the algorithm of `sgo.Random`, D26, already checked against `math/rand/v2`'s vectors), with unbiased bounded sampling, and **independent streams** derived from the manifest's master seed by a frozen SplitMix64 step: `sched`, `fault`, and `data`.
- **(b) PCG32** as named in plan §9.4, a second algorithm with its own vectors.

**Recommendation: (a).** One generator in the TCB instead of two, with reference vectors already in the evidence; stream isolation is tested (D01). The plan accepts PCG32; it does not require it.

## D45 — Effects in the lab **[USER]** · APPROVED 2026-11-02

**Recommendation.**
- **Clock:** virtual monotonic time, starting at 0; it advances only when no task is ready, to the next timer deadline (D06, D07). `Sleep` uses virtual time; timers with equal deadlines fire in `(deadline, timer ID)` order.
- **WallClock:** the manifest's fixed epoch plus virtual time.
- **Random:** unchanged (it is already seeded by the program).
- **Entropy:** `UnsupportedLabEffect` (a lab failure kind, status 2) in the default policy; it is unpredictable by definition, and silently substituting a seeded stream would hide that from the program's author. A later manifest option may supply recorded entropy.
- **SimStore:** the existing in-process store (deterministic), starting empty.
- **Stdout:** written and also recorded as `write` events in the trace.
- Anything else effectful is already denied at compile time in 0.3.0, so the lab subset is all of 0.3.0 except Entropy (D09).

## D46 — Trace and replay format **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** Versioned JSONL (plan §9.5): a header with the manifest (identity of binary, spec, admission, configuration, target, sources, seed and streams, scheduler and instrumentation versions, quota, epoch), one line per event (sequence, virtual time, task/scope/operation IDs, kind, ready set, choice, arm set, written text, outcome), and a footer with the event count, completion status, and SHA-256 of the canonical lines. No PIDs, host paths, addresses, or wall-clock durations in canonical lines; host data goes to a sidecar file. `replay` rejects an incompatible manifest before running, replays every choice from the trace (never from the generator), and stops at the first divergence, reporting the expected and actual ready sets and the last matching event (D03, D04, D05, D12).

## D47 — Lab outcomes **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** In addition to every production outcome (unchanged, S14): **`LabDeadlock`** when no task is ready and no timer is pending while a task is live, with the wait graph (each live task, the operation it waits in, the scope chain) in the record (D08); **`LabQuotaExhausted`** when the quota is used up (not a deadlock, D07); **`UnsupportedLabEffect`** (D45). Each is one diagnostic record, status 2. A run that hangs without reaching a boundary (uninstrumented TCB code) is ended by the driver's wall-clock watchdog and reported in the sidecar as a host hang, never as a managed outcome (D11).

## D48 — Sweeps and saved failures **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** `sgo lab sweep -seeds A..B` runs each seed in a fresh process and reports the distinct semantic outcomes (exit status, stdout, diagnostic record). A run **fails** if its exit status is not 0, or, with `-expect FILE`, if its stdout differs from the file. The first failure's trace is saved; `sgo lab replay` reproduces it in a fresh process (D13). A sweep passing means those runs passed, not that every schedule does (plan §9.6); the record says so.

## D49 — Scheduler policies **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** `random` (default): uniform choice from the ready set with the `sched` stream. `fifo`: the longest-ready task runs next (a diagnostic, fair round-robin scheduler). The scheduler policy and version are in the manifest; a random schedule is not assumed fair, and the quota bounds every run.

## D50 — Select arm index in the lab trace **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** The `arms` and `arm` fields of a lab `select` event are **source indices**: arm *i* is the *i*-th case of the `select` statement as written (default excluded). The compiler gives the runtime each case's source index next to the case list, and the lab maps the runtime's case order (sends first) back before recording. Traces then read against the program text and match the model (sgo/model, which uses script order) for any mix of send and receive arms. Cost: a small compiler table and one lowering test; no change outside lab builds' trace output.

## D51 — Spec version and scope of W09 **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** W09 is spec **0.5.0**, a strict superset of 0.4.0: no 0.4.0 program changes meaning. It admits a deliberately small production I/O surface in package `sgo` only: confined directory and file handles (SPEC-FILE), and TCP connections and listeners on granted IP endpoints (SPEC-NET). Go's `os`, `io`, `net`, `syscall`, and every other ambient I/O package stay denied (S13, S17); nothing is admitted by package name. Not in 0.5.0: DNS, TLS, HTTP, UDP, Unix sockets, pipes, stdin, file metadata beyond size and kind, permissions, process execution. Each needs its own contract later.

## D52 — How file and network authority reaches a program **[USER]** · APPROVED 2026-11-02

**Why it matters.** Root authority must come from a trusted policy, not from a constructor any dependency can call (plan §8.2), and production behaviour must not depend on an ambient switch (S15).

**Options.**
- **(a) Declared in the main `sgo.mod`, fixed at build time.** `grant dir NAME PATH ro|rw` (PATH relative to the process working directory at start, or absolute), `grant connect IP:PORT`, `grant listen IP:PORT`. The grants are part of the build identity (as the effect kinds are, SGO-AUTH-002); `root.Dir("NAME")`, `root.Dial("IP:PORT")`, `root.Listen("IP:PORT")` return root handles only for declared grants. The program opens each granted directory once when it starts and holds it, so later renames of the path do not move the grant.
- **(b) Declared names in `sgo.mod`, bound to paths and endpoints at launch** (a launcher flag or environment variable), so one binary can be deployed with different data directories.

**Recommendation: (a)** for 0.5.0. Every authority a binary can exercise is visible in its identity and its module file, nothing at launch can widen it, and auditing a binary needs no knowledge of its environment. (b) can come later as a narrowing-only binding of declared grants.

## D53 — Path confinement **[USER]** · APPROVED 2026-11-02

**Why it matters.** Plan §8.3 forbids string-prefix confinement; traversal, symlinks, and concurrent renames must not escape a directory grant (I01).

**Options.**
- **(a) Component-wise resolution from the held directory descriptor, no symlinks.** A path is a sequence of names (no empty name, `.`, `..`, NUL, or leading `/`); each component is opened with `openat(dirfd, name, O_NOFOLLOW | O_DIRECTORY | O_CLOEXEC)`, the last with `O_NOFOLLOW`. Any symlink in the path is an error (`PathRejected`). On linux, `openat2(RESOLVE_BENEATH | RESOLVE_NO_SYMLINKS)` is used when the kernel has it, with the same result. No path is ever resolved from the process working directory or as a string.
- **(b) Follow symlinks that stay inside the grant**, which needs re-validation against concurrent changes and is hard to make race-free on darwin.

**Recommendation: (a).** It is race-free on both targets with portable primitives, and the refusal of symlinks is visible and testable. Following symlinks can be added later under its own contract.

## D54 — Handle ownership, sharing, and close **[USER]** · APPROVED 2026-11-02

**Why it matters.** Closing a resource to cancel one operation must not silently break another owner's operation (I04), and stale completions must not act on a reused descriptor (I05).

**Recommendation.**
- A `*sgo.File`, `*sgo.Conn`, or `*sgo.Listener` is a capability handle (no nil, no comparison) owned by the task that opened or accepted it, like a token (SGO-SYNC-002): only the owner closes it; an unclosed handle is closed when its nursery body exits or its task ends, with a `HandleAbandoned` entry in the failure chain.
- `h.LendTo(n)` lends it to a nursery (as effect handles): children may read and write; the lend ends when the nursery closes. `Close` while the handle is lent to a live nursery fails with `HandleLent` and has no effect; the owner closes after the join.
- Cancellation never closes a handle: it ends only the cancelled task's own operation.
- Each handle has a generation; the runtime's descriptor table maps a handle to its descriptor only while the generation matches, so a completion or operation on a closed handle fails with `HandleClosed` and never reaches a reused descriptor.

## D55 — Cancellation and blocking host operations **[USER]** · APPROVED 2026-11-02

**Why it matters.** Plan W09: isolate blocking host work without pretending it is cancellable by dropping an awaiter (I07).

**Recommendation.**
- **Sockets** are non-blocking and use the runtime's netpoller: a waiting `Read`, `Write`, `Accept`, or `Dial` is a cancellation point (SGO-CANCEL-002); cancellation wakes it, and it returns the progress made so far with the cancellation error.
- **Regular files** are read and written with blocking system calls, each bounded to 1 MiB, on the calling thread (the runtime hands the processor off as for any blocking system call). A file operation is **not** a cancellation point while in the kernel: cancellation is observed before it starts and after it returns, and the contract says so; nothing reports a running file operation as stopped.
- Deadlines: `Conn.ReadFor(n, ms)` style per-operation timeouts are omitted in 0.5.0; a task bounds an operation by running it in a nursery and cancelling it (Clock/Sleep), which the netpoller honours.

## D56 — Partial progress and retry **[USER]** · APPROVED 2026-11-02

**Recommendation.** `Read(max int) (string, error)` returns what was read (possibly short; `""` with `nil` only for `max == 0`; end of file or connection is `sgo.EOF`). `Write(s string) (int, error)` writes all of `s` unless an error stops it and returns the bytes written. Every I/O error carries its progress: `sgo.Progress(err) int` (bytes transferred before the error) and `sgo.NoEffect(err) bool` (true only when nothing was transferred and the operation may be retried safely). A cancellation after partial progress returns both the bytes and the cancellation error (I03).

## D57 — Network policy **[USER]** · APPROVED 2026-11-02

**Options.**
- **(a) Exact IP endpoints.** `grant connect IP:PORT` and `grant listen IP:PORT` (IPv4 or IPv6 literal); `Dial` checks every call, so a reconnect is checked again (I02). No hostnames, no DNS. An accepted connection carries no authority beyond itself.
- **(b) Hostname grants with DNS**, which needs a resolver contract (caching, rebinding) first.

**Recommendation: (a)** for 0.5.0; (b) is a later work package.

## D58 — Files and network in the lab **[USER]** · APPROVED 2026-11-02

**Recommendation.** In a lab run, directory, connection, and listener grants are served by in-process **fake adapters** (an in-memory directory tree per grant, starting empty unless the manifest seeds it; a loopback network that connects granted dials to granted listeners of the same run), with the same contracts, progress, and errors as the real adapters, and every operation a choice point (SGO-LAB-003). The fault stream (D44) can inject short reads and writes. I06 runs the same bounded scenarios through the real adapters (production, against local fixtures) and the fakes (lab) and compares normalized outcomes. Real host I/O is never reached from a lab binary.

## D59 — File operations in 0.5.0 **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** `Dir`: `Open(path) (*File, error)` (read), `Create(path) (*File, error)` (write; creates or truncates; `rw` grants only), `ReadFile`/`WriteFile` (whole file, bounded size), `Remove(path)`, `Mkdir(path)`, `List(path) ([]string, error)` (sorted names; **amended 2026-11-02 by the user: `List(path) (*sgo.Listing, error)` with `Len() int` and `Name(i int) string`, since `[]string` is not an admitted type**), `Sub(path) (*Dir, error)` (attenuation to a subdirectory), `ReadOnly() *Dir`. `File`: `Read`, `Write`, `Close`, `Size`. No seek, rename, link, chmod, or metadata in 0.5.0.

## D60 — Unix domain sockets **[USER]** · APPROVED 2026-11-02

**Why it matters.** Unix domain sockets are the first choice for inter-process communication on one host; D57 admitted only IP endpoints, and the SGO-NET-002 draft denied Unix sockets.

**Decision (the recommendations, approved by the user).**
1. **Grant by exact path in the main `sgo.mod`:** `grant connect unix:PATH` and `grant listen unix:PATH`, PATH absolute or relative to the working directory at start; `root.Dial("unix:PATH")` and `root.Listen("unix:PATH")` are checked against the grants exactly, on every call, then the path is given to the kernel. The path names an endpoint, not a tree: the kernel follows symlinks in it, and the spec says so.
2. **Same machinery as TCP:** non-blocking stream sockets on the network poller, cancellable waits, the handle contract of D54-D56, `*sgo.Conn` and `*sgo.Listener`; in the lab, the run's loopback network keyed by the path.
3. **Stale socket files:** `Listen` removes an existing file at PATH only if it is a socket, then binds; any other file there is `AddressInUse`. Closing a listener removes its socket file if the path still names the socket it created.
4. **Scope:** stream sockets on file-system paths only, at most 103 bytes (the portable `sun_path` limit), checked by the driver. Denied: Linux abstract names, datagram and seqpacket sockets, and passing descriptors or credentials as messages (authority arriving outside the grants).
5. **Peer identity:** `c.PeerUID() (int, error)` on a Unix connection returns the peer process's user ID (SO_PEERCRED on linux, getpeereid on darwin); on a TCP connection it returns an `Unsupported` error. In lab runs the peer is the run itself and PeerUID returns the fixed value 1000.

## D61 — Spec version and scope of W10 **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** W10 is spec **0.6.0**, a strict superset of 0.5.0 (no 0.5.0 program changes meaning). It adds only tooling: tests written in S-Go (D62), a type-check command (D63), and the distribution (below). The language and the runtime contracts do not change; examples, reports, and benchmarks are evidence, not spec surface. Editor/LSP support stays explicitly unsupported (plan §11/W10) until a forked tool works.

**Distribution.** A release is an archive `sgo-<version>-<os>-<arch>.tar.gz` holding `bin/sgo`, the toolchain binaries it runs (`compile`, `link`, `asm`), the TCB sources and `sgo` package it compiles for targets, and the frozen registry (`spec/VERSION`, `SGO_SPEC_ADMISSION.json`, `TARGET_TCB.json`). It holds **no `go` command** and no stock standard library sources outside the TCB, so the installation is single-language. `sgo` finds its root from its own path; installing is extracting the archive and adding `bin` to `PATH`. A script builds the archive from a clean checkout and a fresh-machine check (CI) builds and runs the examples from it (W10 gate).

## D62 — Tests written in S-Go: `sgo test` and per-test roots **[USER]** · APPROVED 2026-11-02

**Why it matters.** T04 requires each test to own an audited lifetime root: a failed test drains or terminates, and no task or handle crosses into another test.

**Options.**
- **(a) One process per test.** Test files are `*_test.sgo` in the package; a test is `func TestName(t *sgo.T) error` with `t.Context() *sgo.Context` (the test's root, with the module's grants) and `t.TempDir() *sgo.Dir` (a fresh `rw` directory removed after the test). `sgo test [-run regexp] [packages]` builds one test binary per package and runs each selected test in its **own process** as that process's root task: the test passes if it returns nil and its root ends cleanly; a returned error, a failure, a leaked obligation, or a timeout fails it with the diagnostic record. Isolation is the process boundary, so nothing a test spawns or opens can reach another test.
- **(b) All tests in one process,** each under a fresh root-like scope. Faster, but isolation then rests on runtime bookkeeping (grants, timers, handles, lab state) that would need a new "test root" concept in every subsystem.

**Recommendation: (a).** Strongest isolation with no new runtime concept; per-test process start (a few ms) is acceptable. `sgo lab test` (the same tests under the lab, one seed per test, trace on failure) follows from the existing lab driver.

## D63 — A type-check command for editors and scripts **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** `sgo check [packages]` runs exactly the build front end (parse, admission, type checking, S-Go checks) without code generation or linking, and reports the same diagnostics, codes, and positions as `sgo build` (T03: one front end, so the tools cannot disagree). `-json` prints the diagnostics as JSON lines for editors. No LSP server ships; the limitation is documented.

## D64 — How the examples assert quiescence **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** Structured concurrency already makes post-scope quiescence a property of every nursery (no task outlives its scope; handles, tokens, and permits are resolved). The examples assert it as tests (D62): at the end of each test the harness checks the runtime's counters for the test process (live tasks other than the root, sleepers, socket waiters, open handles, unresolved permits and tokens) are all zero, and fails the test with `LeakedWork` otherwise. No user-visible counting API is added.

## D65 — Lab variables and production binaries (T02) **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** Keep the current rule and prove it: a production binary has no lab activation path, so lab environment variables (`GOLABSEED` and anything `SGO_LAB*`) and a manifest on fd 3 have no effect on it and it reports no replay guarantee; `sgo lab replay` and `sgo lab run` refuse a production binary and a production build ID (already enforced, D41). W10 adds the T02 test for both directions. No variable is ever read to refuse or alter a run (S15).

## D66 — The three examples **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** In `sgo/examples/`: **pipeline** (a producer, N workers, and a collector over channels with a capacity limiter; cancellation on the first bad record); **service** (a Unix-socket service started with `n.Start`, ready only after it listens; clients; orderly shutdown by cancellation with cleanup of every connection); **replay** (a seeded ordering bug found by `sgo lab sweep`, its saved trace committed, `sgo lab replay` reproducing it, and the fixed program passing the same sweep). Each has `sgo test` tests asserting outcomes and quiescence (D64), and each runs from the release archive in CI.

## D67 — Performance evidence **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** A benchmark harness (`sgo/bench/`) runs pairs of programs (S-Go, and equivalent Go built by the isolated upstream reference toolchain) and S-Go-only microbenchmarks: empty nursery, spawn/join, cancellation fan-out, channel throughput (rendezvous and buffered), peak live tasks, lab off versus lab on. Each is run 20 times in fresh processes; the harness records raw samples, wall and CPU time, peak RSS and virtual size, allocations where measurable, compile time, and host and toolchain identity, and reports P50/P95 with spread. Results are evidence only: no claim without samples, and no check is removed to improve a number.

## D68 — Adopted conformance versus the upstream suite (T05) **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** Two separate lanes, never mixed: the untouched upstream suite runs only with the reference toolchain (B03, as today), and an **intentional rejection report** runs `sgo check` over the upstream language tests (`test/*.go`, copied as `.sgo`), records each file's outcome (admitted, or the first rejection code and clause), and classifies them by clause. A file that `sgo check` admits and that has an expected output is built and run by S-Go and must match; an unexplained mismatch blocks the release. No upstream test is changed or added to a target lane to keep it green.

## D69 — sgo test details for spec 0.6.0 **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** (1) `sgo.StartTest` is admitted only in the main package `sgo test` generates: the compiler admits it only under `-sgotestmain`, which only the driver passes for that file, so user code calling it is rejected at compile time (`SGO_E_UNADMITTED`). (2) A test that returns an error and leaves obligations keeps its error as the primary outcome, with the abandoned obligations in the record's chain; `LeakedWork` only when the test returned nil (SGO-TEST-003). (3) Tests of package `main` are refused in 0.6.0 (`SGO_E_USAGE`: move the code under test to a library package). (4) The watchdog is a fixed 60 s with no flag. (5) Tests under the lab configuration are not in 0.6.0.

## D70 — W11 preregistration **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** Fixed before any measurement (F08). **Workloads:** the D67 set (spawn/join, rendezvous, buffered, cancellation fan-out, live tasks, empty nursery), deep recursion (1, 4, and 7 MiB of frames), large single frames (64 KiB to 1 MiB), idle tasks at 10k, 50k, and 100k, and the three D66 examples. **Metrics:** wall, CPU, peak RSS, virtual size, mapping count, time to exhaustion and its outcome. **Samples:** 20 fresh processes per row; on note2 (linux/amd64) pinned to fixed cores with `taskset`, kernel and load recorded. **Adoption threshold:** the candidate is adopted only if (a) every correctness case F01-F05 and F07 passes on all three targets, (b) P50 peak RSS or P50 wall improves by at least 20% on the idle-task and example workloads, and (c) no other row regresses by more than 5% at P50. Otherwise W11 ends `investigated_not_adopted` (D74).

## D71 — Experimental identity `vstack` **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** The prototype is a third execution configuration, as the lab is (D41): build-tagged runtime files, a separate `vstack` section in `TARGET_TCB.json` with its own digests, `config=vstack` in package identities and build IDs, selected only by an explicit driver flag (`sgo build -vstack`), never by environment. The linker refuses to mix `vstack` with `prod`, `lab`, or `test` packages. The default runtime and its TCB digests do not change.

## D72 — Prototype scope **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** One demand-paged reservation per task (8 MiB, tunable) with an OS guard region below it (PROT_NONE, at least 64 KiB, larger than the largest admitted frame probe); the function prologue's stack-bound check and every preemption check stay, the bound now only reporting overflow (`StackExhausted`, SGO-STACK) instead of growing; no stack copying or shrinking; a freed stack is released with `madvise(MADV_DONTNEED)` (linux) or `MADV_FREE_REUSABLE` (darwin) before reuse, so no data survives (F07); g0 and signal stacks keep the upstream implementation (F05). Large frames probe the guard page by page (F02).

## D73 — Platform limits are reported, not hidden **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** Idle-task density (F06) is measured at the host's defaults and recorded with them: linux at the stock `vm.max_map_count` 65530 (set with sudo on note2, restored afterwards) and at note2's raised 1048576; overcommit mode 0; darwin with its 16 KiB pages. Exhaustion must end the program with a defined failure (mapping or reservation refused: `StackExhausted` or `RuntimeDefect` with the cause), never a crash in unrelated memory. CI runners measure only at their defaults.

## D74 — Outcome rule **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** If D70's threshold is met with every correctness case passing, adoption is proposed to the user as a separate decision (it would change the default runtime and its TCB). If correctness holds but the benefit is not met, W11 closes `investigated_not_adopted`: the architecture note, prototype, correctness results, and measurements are kept behind the `vstack` identity, unused by default. A correctness failure that cannot be fixed within the prototype's scope closes W11 `investigated_not_adopted` with that failure as the evidence.

## D75 — Fresh-context review (Q01) **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** You start a **new reviewer job** that has never seen this work, with read-only use of the repository at a pinned commit. I write the brief `sgo/docs/w12-review-brief.md`: the constitution, the spec, the diff against the upstream pin restricted to S-Go changes, the plan's checklist (root paths, lock order, linearization points, failure propagation, exception edges, policy bypasses, replay scope, published claims), and how to build and run things. The reviewer reports findings to this job as messages. Each finding becomes a row in `sgo/evidence/w12/review-findings.json` (id, severity blocker/major/minor, location, claim, resolution, fix commit, verifying test). Every blocker and major finding is fixed with a regression test, or the release is withheld. Minor findings are fixed or listed as limitations, but never if they are compatibility escapes. The reviewer re-checks the fixes. **Fallback** if you can't start a new job: the note2 helper, which wrote no S-Go code, reviews after a context reset, and the evidence records that the reviewer had seen test runs.

## D76 — `GUARANTEES.json` and traceability (Q04, S20) **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** `sgo/GUARANTEES.json` (plan §10.1) is the single published claim set. Each guarantee has an id, a statement, the frozen clauses it rests on, its enforcement (compiler check, link check, runtime check, lab), the acceptance cases and evidence paths, the targets, and a **scope** taken from a closed list (S20): `admission` (statically rejected), `runtime_enforced` (checked at run time with a specified failure), `model_explored` (bounded model exploration, stating the bounds), `tested` (executed cases only). No guarantee claims proof of program correctness, termination, or all schedules. The file also states the stack implementation (upstream growable stacks; W11 `investigated_not_adopted`), the bootstrap and upstream identities, and the known limitations. `guarantee-check.py` (CI) fails if any of these holds: a guarantee cites a non-frozen clause, a case that isn't implemented and passed, or a missing evidence file; an admitted family in `SGO_SPEC_ADMISSION.json` has no guarantee; a limitation names a compatibility escape. `sgo/docs/GUARANTEES.md` is generated from the JSON and answers plan §15.2's six questions.

## D77 — No-legacy-path audit (S17, release blockers) **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** The audit covers what ships: the release archive (`sgo`, `compile`, `asm`, `link`, the TCB sources) and every flag, environment variable, and input those tools accept. A script, `legacy-audit.py`, enumerates each tool's flag table and environment reads from the source. Each one must be classified as S-Go-specified, refused (with a test), or inert (with a test showing no effect). Anything unclassified fails. Reachable inherited paths, such as a `.go` input or a stock build mode, need a rejection fixture. The source tree may keep unshipped upstream code (`cmd/go`, `gofmt`, …) because the archive excludes it (`make-release.py` refuses other executables). The archive's contents check therefore becomes part of the audit.

## D78 — Release qualification and reproduction (Q03) **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** A CI workflow, `sgo-release`, builds `sgo-<spec>-<os>-<arch>.tar.gz` on the three targets: `macos-14` (darwin/arm64), `ubuntu-24.04`, and `ubuntu-24.04-arm`. It also produces the source archive (`git archive` of the release commit), one combined diff against the upstream pin, and `SHA256SUMS`. A **second job, on fresh runners with no checkout**, downloads only those artifacts, verifies the checksums, follows the written install instructions, then builds, runs, and tests the three examples and replays a shipped trace. A clean bootstrap from the source archive (stage-0 build plus the conformance lane) runs on linux/amd64. Publishing a tag or GitHub release is **your decision**: the workflow uploads artifacts only.

## D79 — Negative-fixture honesty (Q02) **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** `test-negative.py` picks one representative per plan §12.3 category: orphan on closure, duplicate permit commit, capacity leak, lost wakeup, policy bypass (undeclared effect, capability forgery), replay divergence (edited trace or seed), unreported partial I/O, and incorrect return/defer. It checks each fails with its **specified** code and clause, not merely a non-zero exit. A harness self-test flips each expectation and requires the harness to report a failure, which proves it can't pass vacuously. Cases already in conformance are reused, and only missing categories get new fixtures.

## D80 — Upstream tests and stress (W12 re-run) **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** Re-run at the release commit: the full upstream suite on the reference toolchain (B03, both linux targets), the runtime lane including the race lane on all three targets, all conformance lanes, and stress. Lab stress is 1000 seeds per lab conformance case, plus a sweep over each example. Production stress is the native, cancellation, and I/O runtime tests at `-count=20` under `GOMAXPROCS` 1, 4, and 16. Reduced counts on a noisy host are recorded (plan §12.2), and no scenario is dropped.

## D81 — Licensing (W12) **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** The release carries Go's `LICENSE` and `PATENTS` unchanged. Every file added by S-Go carries the BSD-style "S-Go Authors" header, and `license-audit.py` checks this for each added path in the diff against the pin. Vendored modules stay as upstream ships them. The audit also confirms that no code was adapted from outside Go, including the model, scripts, and examples. If any is found, it is listed with its license, or removed.

## D82 — Remaining partial evidence **[DEFAULT]** · APPROVED 2026-11-02

**Recommendation.** T07's allocation counts stay unmeasured, and T07 stays **PARTIAL** as a stated limitation in `GUARANTEES.json`. S-Go programs can't read runtime statistics: `runtime` isn't admitted, and since W12 R2 (D84) the runtime ignores `GODEBUG` too (it did not before R2; the reviewer found it). Adding a hook only to count allocations would widen the TCB for a benchmark. This is a measurement gap, not a guarantee or a compatibility escape, so it doesn't block release. F03–F05 stay unrun for the W11 reason (no prototype). The open parts of S04 (drift in an adopted clause; a blanket same-as-Go rule) and S20 are completed by `adoption-check.py` mutation cases and D76's scope checks.


## D83 — Release registry version **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** Spec **0.7.0** is the release registry: the rows of `SGO_SPEC_ADMISSION.json` that W03–W10 implemented, enforced and tested become `admitted` (SR2), and the review's clause wording fixes (D-R1 for SGO-TCB-002, D-R5 for SGO-MEM-002, and any later ones) take new clause versions. No feature is added. The admission digest and every identity move once, at this version change, with a CHANGELOG entry.

## D84 — `GOMAXPROCS` in S-Go programs (W12 R2) **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** An S-Go program's runtime reads no environment setting at all: `GODEBUG`, `GOTRACEBACK`, `GOGC`, `GOMEMLIMIT`, `GOMAXPROCS`, and `GOROOT` are ignored, and the build's fixed defaults apply (`GOMAXPROCS` = the CPU count). No spec amendment is needed, since this is what SGO-PROC-004 already says. The S-Go linker marks every binary (`runtime.sgoLinked`) so this holds from the first instruction. Binaries linked by the upstream linker (the runtime test lane) keep upstream behavior. D80's stress runs at 1, 4 and 16 processors vary the CPU set (`taskset` on linux), not an environment variable. D82's rationale is corrected: GODEBUG was **not** ignored before R2; allocation counts stay unmeasured because S-Go programs cannot read runtime statistics and a benchmark-only hook would widen the TCB.

## D85 — Next work package: the core data model (W13) **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** W13 is the core data model, ahead of library modules and a standard library, because every later step is built on composite types and the sharing rule is the largest open safety question. Plan: `sgo/docs/w13-data-model-plan.md`. A standard library waits for W13 stages 1–3 and a library-module work package.

## D86 — Sharing rule for mutable aggregates **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** Option E of the W13 plan §5: mutable aggregates (slices, maps, pointers, structs containing them) are task-local and cannot be captured by a `Spawn` callback; crossing a task boundary is by copy, by a frozen (deeply immutable) value, or later by a narrow whole-variable move. Go-style sharing with run-time race detection (option D) is rejected because SGO-MEM-003 is a static guarantee.

## D87 — Stage order **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** Stage 1 values (structs, arrays of admitted types, named types with value-receiver methods), spec 0.8.0 → stage 2 task-local slices, `append`, `[]byte`, string indexing, maps, pointers and pointer receivers, copy-only crossing, spec 0.9.0 → stage 3 freeze → stage 4 move, only if needed. Interfaces follow stage 2 as a separate work package.

## D88 — Map iteration order **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** Insertion order, specified and identical in production and the lab, so runs and replays agree. Go's randomized order is rejected (nondeterminism outside the scheduler); sorted order is rejected (cost, needs ordered keys).

## D89 — Copy semantics **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** Values without internal references copy implicitly, as integers do. Values with internal references cross a task only through an explicit `sgo.Copy(x)` (a deep copy), and an uncopied send is `SGO_E_UNCOPIED_SEND`. No user-defined copy hooks.

## D90 — Representation of frozen values **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** A distinct type `sgo.Frozen[T]`, produced by `sgo.Freeze(x)` (which consumes or copies `x`), with read-only access checked at compile time. A run-time immutability flag is rejected: misuse would be a run-time failure and the check would grow the TCB.

## D91 — Bounds, nil and map failures **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** Index or slice out of range, nil dereference and writes to a nil map are defined failures with their own failure kinds (for example `BoundsViolation`), recorded and propagated like other task failures (SGO-ERR-*), with exit status and diagnostic format specified, not upstream panics with Go tracebacks.

## D92 — W13 process **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** As W04–W12: step 13.0 models the sharing rule (task-local, copy, freeze, move) and adds the plan's five example programs as pending conformance fixtures; each stage then gets drafts for approval, positive and negative fixtures, a spec version, CI on both linux runners, and reviewer verification. The five programs (P1 using copy) are the acceptance bar for W13.

## D93 — The byte type (W13 stage 2) **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** Admit `byte` (0..255) with checked arithmetic: a result outside the range is `IntegerOverflow`; `int(b)` always succeeds, `byte(i)` fails with `IntegerOverflow` outside 0..255. No `rune`; `range` over a string stays denied (iterate `[]byte(s)`). Draft SGO-TYPE-007 (`sgo/docs/w13-stage2-drafts.md`).

## D94 — Function literals that capture task-local variables **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** Such a literal is task-local: it may be called, deferred, or assigned to a local variable (then task-local too), but not passed as an argument, returned, or captured by a `Spawn` callback (`SGO_E_TASK_LOCAL_CAPTURE`). This closes the path by which a closure over a slice or map could reach another task through a helper that spawns it. Draft SGO-COMP-004 rule (2).

## D95 — Channels as struct fields and array elements **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** A channel type is shareable: structs and arrays of value and channel types are copied like values and share their channels, as channels already are. They are not comparable (channels are not). Needed for request/reply structs (P2).

## D96 — nil for pointers, slices, and maps **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** `nil` is a value of pointer, slice, and map types (not of channels, function types, or handles): nil literals and `== nil`. A nil slice or map reads as empty; writing a nil map is `NilMapWrite`; dereferencing nil is `NilDereference` (D91). Draft SGO-COMP-008.

## D97 — P1 in stage 2 **[NEEDS USER]** · APPROVED 2026-11-02

**Recommendation.** Capturing `sgo.Copy(in)` in a `Spawn` callback is a move into a closure (stage 4), so P1 is rewritten for stage 2 with value-array inputs (`[3]string`, copied on capture); results still return through a channel with `sgo.Copy`.

## D98 — What sgo.Freeze does to its argument (W13 stage 3) **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** `sgo.Freeze(x)` makes one deep copy (the `sgo.Copy` of SGO-COMP-004) and seals it; `x` stays the caller's mutable value. One copy per freeze, none per reader. With the narrow move of stage 4, `sgo.Freeze(move(x))` can skip the copy. Rejected: freezing in place, which needs the stage-4 "no other live reference" analysis now. Draft SGO-COMP-009 (`sgo/docs/w13-stage3-drafts.md`).

## D99 — How frozen contents are read **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Access paths: `f.Get()` appears only at the head of a selector/index chain, as the argument of `len`, or as a `range` operand; the chain must end in a value type, a `Frozen[U]`, or a channel, and `range` binds value-typed variables only. Any other use of a frozen-derived reference (assign, pass, return, send, capture, `sgo.Copy`, write, `append`, `delete`, slicing) is `SGO_E_FROZEN_WRITE`. Rejected: `Get` returning a deep copy (defeats sharing); flow-sensitive taint tracking (larger, worse diagnostics).

## D100 — Which types may be frozen **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Any admitted type holding no handle and no channel. Freezing a value type is allowed and shares it without copying on each capture.

## D101 — Frozen identity and zero value **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** `sgo.Frozen[T]` has no zero value of its own (must be initialized), no `nil`, and no `==`. It is shareable: captured by `Spawn` callbacks and sent or committed without `sgo.Copy`.
## D102 — W13 stage 4 (move) deferred; W13 closes after stage 3 · APPROVED 2026-11-03

**Decision.** The narrow `move` of D86/D87 (stage 4, optional) is not built in W13. Every W13 acceptance program passes with copy and freeze; move would only save a copy (P1's hand-over, `sgo.Freeze(move(x))`) at the cost of the "no other live reference" analysis. It is reconsidered when a real program needs zero-copy hand-over. W13 closes with stages 1–3 and the review (V1–V4 verified).

## D103 — Where does the standard library live, and is it trusted? **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** S-Go source shipped with the toolchain under the reserved import prefix `std/` (`std/strings`, …), compiled under exactly the same admission and sharing rules as user code. It is **not** part of the TCB: it gains no privilege, and its packages are identified by contract digests like any dependency. A user module path may not equal or lie under `std` (as for `sgo`, D38). (sgo/docs/stdlib-foundations-plan.md)

## D104 — How are other modules required? **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** `require <module path> <version>` lines in `sgo.mod`, resolved only from local directories in this version: a `replace <path> => <dir>` line, or a vendored copy under `vendor/`. No network fetch. `sgo.sum` records a content digest of every required module's files; a mismatch is an error. Versions are compared but no minimum-version selection is attempted: each module path appears once in the build, and two different required versions of one path are an error. (sgo/docs/stdlib-foundations-plan.md)

## D105 — What crosses a module boundary? **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Exported names only, checked by the existing SGO-IMP-002 contract digests extended to required modules; a dependency's effect grants are not inherited (the root module's grants bound the program, SGO-AUTH-*), and a dependency's own `sgo.mod` grants must be a subset of the root's or the build fails. (sgo/docs/stdlib-foundations-plan.md)

## D106 — Runes **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** `rune` as a checked integer type (0..0x10FFFF, overflow is `IntegerOverflow` like `byte`), rune literals, `range` over a string yielding (byte index, rune) with U+FFFD for invalid UTF-8, and `string(r)`, `[]rune(s)`, `string(rs)` conversions. `range` over a string stays an iteration *by rune*, never by byte (bytes are `[]byte(s)` or indexing). (sgo/docs/stdlib-foundations-plan.md)

## D107 — Interfaces and the sharing rule (W17, decided now so W16 avoids designs W17 would break) **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Interface values take the sharing class of what they may hold: an ordinary interface type is **task-local** (any admitted type converts to it; it crosses tasks only through `sgo.Copy`, which deep-copies the dynamic value); `error` stays **shareable** and only shareable types convert to it (as V2 fixed). Type assertions and type switches come with interfaces. (sgo/docs/stdlib-foundations-plan.md)

## D108 — Generics **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Deferred. The first standard library uses per-type functions (`sort.Ints`, `sort.Strings`) and function parameters (`sort.Slice`-style with an index `less`), which S-Go already admits. Generics get their own decision when containers are needed. (sgo/docs/stdlib-foundations-plan.md)

## D109 — Package-level state in the standard library **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** None. Every `std/` package is functions, types and constants only; there is no package-level variable to race on, no `init`, and no hidden global (e.g. no global random source: randomness stays a capability, `*sgo.Random`, SGO-AUTH-*). (sgo/docs/stdlib-foundations-plan.md)

## D110 — Process **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** As W13: drafts of each work package's clauses for approval, conformance cases with negative fixtures, a spec version per work package, mutation tests, and reviewer verification on linux. The standard library's packages get conformance cases *and* their own `sgo test` suites, run in CI. (sgo/docs/stdlib-foundations-plan.md)

## D111 — What does a `require` version mean without a network or VCS? **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** A label checked only through `sgo.sum`: `sgo.sum` binds `(path, version)` to the digest of the module's files, so the same label can never silently name different code. Versions have the form `vMAJOR.MINOR.PATCH`. Each module path appears at most once in a build; two modules requiring different versions of one path is `SGO_E_MODULE_VERSION`. No minimum-version selection. (sgo/docs/w14-drafts.md)

## D112 — Where is a required module's source? **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Only from the main module: a `replace <path> => <dir>` line (a directory relative to the main module root) or else `vendor/<path>/`. Dependencies' own `replace` lines are ignored (as in Go); their `require` lines count. (sgo/docs/w14-drafts.md)

## D113 — Who writes `sgo.sum`? **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Only `sgo mod sum`, an explicit command. Builds, checks, tests and lab runs never write it; a missing or mismatched entry fails the build with the command to run. `sgo mod verify` rechecks every entry. (sgo/docs/w14-drafts.md)

## D114 — Which grants may a dependency's `sgo.mod` contain? **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Effect-kind grants only, each also granted by the main module (D105). File and network grants (`grant dir/connect/listen`) are refused in a dependency: their handles come only from the root task and reach a library only as arguments. (sgo/docs/w14-drafts.md)

## D115 — Who may import `std/`? What may `std/` import? **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Any package may import `std/...`. A `std/` package imports only other `std/` packages and `sgo`, never a user module. `std` is one implicit module that is always present, never required, replaced, or vendored. (sgo/docs/w14-drafts.md)

## D116 — Are surrogates rune values? **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Yes, as values: the rune range check is 0..0x10FFFF only. A rune literal or `\u` escape naming a surrogate is a compile error, and encoding a surrogate (`string(r)`, `string(rs)`) yields U+FFFD, as in Go. (sgo/docs/w15-drafts.md)

## D117 — How is invalid UTF-8 decoded? **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** As Go: each invalid or truncated sequence, overlong form, or encoded surrogate decodes to U+FFFD and advances one byte; `range` over strings, `[]rune(s)`, and `std/utf8` share this rule. (sgo/docs/w15-drafts.md)

## D118 — Untyped rune constants **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** A rune literal is an untyped rune constant with default type `rune`, assignable to int, byte, or rune (and named types over them) when representable; typed operands are not mixed without a conversion. (sgo/docs/w15-drafts.md)

## D119 — Admitted escapes **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Admit `\u` and `\U` in string and rune literals and `\'` in rune literals; surrogates and values above 0x10FFFF are compile errors; `\a \b \f \v` and octal escapes stay denied; `\xHH` in a rune literal is the code point 0xHH. (sgo/docs/w15-drafts.md)

## D120 — Rune conversions **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** `rune(i)`, `int(r)`, `rune(b)`, `byte(r)` (checked), `string(r)`, `[]rune(s)`, `string(rs)`. `string(i)` for an int stays denied (use `string(rune(i))`); conversions between slice types stay denied. (sgo/docs/w15-drafts.md)

## D121 — Library errors without interfaces **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** `std/errors` v1 is `New` only; `Is`, `As`, `Unwrap`, `Join` come with W17. Library errors are distinguished by being non-nil and by their message, whose exact format each clause states (Go's formats). (sgo/docs/w16-drafts.md)

## D122 — Unicode scope of the first library **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** White_Space classification is complete (25 code points, no table), so `TrimSpace` and `Fields` behave as in Go; case mapping is ASCII-only and named so (`ToUpperASCII`, `ToLowerASCII`, `EqualFoldASCII`); full case tables wait for a table representation without package variables. (sgo/docs/w16-drafts.md)

## D123 — Sorting without generics or interfaces **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Typed in-place sorts (`Ints`, `Strings`, `Runes`, `Bytes`), `Search`/`SearchInts`/`SearchStrings`, and `Perm(n, less) []int` returning the sorted order as indices; every sort is stable and uses one fixed merge sort, so results are identical on every platform. (sgo/docs/w16-drafts.md)

## D124 — Library functions outside their domain **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Input-caused conditions that Go reports as errors are errors; preconditions Go enforces by panicking are `panic` with a message naming the function; arithmetic overflow is the checked `IntegerOverflow` at the library's position. (sgo/docs/w16-drafts.md)

## D125 — Checking "behaves like Go" **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** A function with a Go counterpart of the same name and parameter shape has its documented results; a differential test compares Go's package and `std/` on the same generated inputs (edge cases, seeded random strings including invalid UTF-8); deviations are listed in each clause. (sgo/docs/w16-drafts.md)

## D126 — The library API is specified **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Each package's exported API and behavior is a frozen clause (SGO-STD-001..007, `SPEC-STD.md`); an exported name not in a clause is refused by `std-api-check.py`; adding a function is a clause amendment and a spec version. (sgo/docs/w16-drafts.md)

## D127 — Admitted interface forms **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Named interface types with admitted method signatures, embedding of named interface types, and `any`; interface type literals outside a declaration and type sets/unions stay denied. (sgo/docs/w17-drafts.md)

## D128 — Comparing interface values **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Only with nil; `==` between interface values and interface map keys are denied (Go's comparison can panic at run time), as for `error`. (sgo/docs/w17-drafts.md)

## D129 — Type assertions **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** The comma-ok form and type switches; the single-value `x.(T)`, which panics on a mismatch, is denied; an impossible T is a compile error. (sgo/docs/w17-drafts.md)

## D130 — No typed nil **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** A nil pointer converted to an interface type is the nil interface; a method call on a nil interface is `NilDereference`; a deliberate, stated difference from Go. (sgo/docs/w17-drafts.md)

## D131 — Method sets and calls **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Go's method-set rules; dynamic dispatch to the dynamic type's method; method values and method expressions of interface types stay denied. (sgo/docs/w17-drafts.md)

## D132 — Interfaces under the sharing rule **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** Every interface type but `error` is task-local whatever its dynamic value: no send without `sgo.Copy` (which deep-copies the dynamic value), no spawn capture, no frozen or shareable storage; `error` stays shareable and only shareable types convert to it. (sgo/docs/w17-drafts.md)

## D133 — Library additions with interfaces **[NEEDS USER]** · APPROVED 2026-11-03

**Recommendation.** `errors.Unwrap` and `sort.Sort` with `sort.Interface` (stable); `std/io`, `fmt`, and `errors.Is`/`As` get their own drafts. (sgo/docs/w17-drafts.md)

## D134 — Shape of std/io **[NEEDS USER]** · PROPOSED

**Recommendation.** Go's Reader/Writer/EOF; task-local; sgo.File and sgo.Conn gain Go-shaped byte methods that io adapters wrap. (sgo/docs/stdlib-parity-plan.md)

## D135 — Capability handles inside interfaces **[NEEDS USER]** · PROPOSED

**Recommendation.** sgo.Copy of an interface that may hold a handle is refused at compile time (SGO_E_CAPABILITY_MISUSE); handles cross tasks only by LendTo. Today the runtime refuses the operation late (sgoHandleUser). (sgo/docs/stdlib-parity-plan.md)

## D136 — Package-level tables **[NEEDS USER]** · PROPOSED

**Recommendation.** Admit package-level var of type sgo.Frozen[T] with a constant composite-literal initializer; all other package state stays denied (D109). (sgo/docs/stdlib-parity-plan.md)

## D137 — Variadic parameters **[NEEDS USER]** · PROPOSED

**Recommendation.** Admit func f(xs ...T), f(a, b) and f(s...); xs is a slice owned by the callee's task. (sgo/docs/stdlib-parity-plan.md)

## D138 — fmt without reflection **[NEEDS USER]** · PROPOSED

**Recommendation.** Format basic types, error, []byte, nil, and fmt.Stringer; other values through generated String methods; Go's verbs and flags, checked by test-std-diff. (sgo/docs/stdlib-parity-plan.md)

## D139 — Generics **[NEEDS USER]** · PROPOSED

**Recommendation.** Type parameters with constraint interfaces (type-set unions only as constraints); a type parameter is task-local unless its constraint makes it shareable; model first. (sgo/docs/stdlib-parity-plan.md)

## D140 — Floating point **[NEEDS USER]** · PROPOSED

**Recommendation.** float64 only, IEEE 754 with Go's rounding; checked float-to-int conversion; no float map keys; FMA contraction disabled in S-Go code. (sgo/docs/stdlib-parity-plan.md)

## D141 — encoding/json **[NEEDS USER]** · PROPOSED

**Recommendation.** No reflection: sgo generate json writes marshalers from type declarations; output and errors match Go's for generated types. (sgo/docs/stdlib-parity-plan.md)

## D142 — Order and scope **[NEEDS USER]** · PROPOSED

**Recommendation.** W19 io/bufio, W20 frozen tables + unicode/hash/crypto/encoding, W21 variadics + fmt, W22 generics + slices/maps/containers, W23 floats, W24 json; each with drafts, freeze, review, Linux evidence. (sgo/docs/stdlib-parity-plan.md)

## D143 — Delegated dialer · APPROVED 2026-11-03 (owner delegated)

**Decision.** `root.Dialer(endpoints []string) (*sgo.Dialer, error)`: a capability handle limited to endpoints the main module grants (IP:PORT or HOST:PORT), created by the root task only, lendable to nurseries like any handle. A task that owns it or runs inside a nursery it is lent to may `Dial` (and `Resolve`, D145) within its endpoints; the connection belongs to the dialing task. `root.Dial` stays as it is. (Happy Eyeballs, RFC 8305; sgo/docs/happy-eyeballs-plan.md)

## D144 — Handing a connection up · APPROVED 2026-11-03 (owner delegated)

**Decision.** `sgo.ConnSlot`: `root.NewConnSlot()` / `ctx.NewConnSlot()` creates a slot owned by the calling task, lendable to its nurseries. `slot.Put(c)` by the connection's owner (a borrower of the slot) moves the connection into the slot; only the first Put succeeds (`AlreadySet` after it), which is the race winner. `slot.Take()` by the slot's owner returns the connection, now owned by that task. A connection left in the slot is closed with it, like any handle (HandleAbandoned). Handle lists stay touched only by their owner: Put unlinks from the putter's list, Take links into the taker's. (Happy Eyeballs, RFC 8305; sgo/docs/happy-eyeballs-plan.md)

## D145 — Name resolution · APPROVED 2026-11-03 (owner delegated)

**Decision.** `grant connect HOST:PORT` (host names now admitted) authorizes resolving HOST and connecting to its addresses at PORT. `d.Resolve(host string, family int) ([]string, error)` on a dialer whose endpoints include HOST:PORT returns IP literals (`sgo.IPv6` or `sgo.IPv4`), each of which that dialer may then dial at PORT. Production: /etc/hosts, then DNS over UDP to the nameservers of /etc/resolv.conf (A or AAAA, answers followed through CNAME records, no search domains), with cancellation. UDP stays denied to user code. Lab: a fixed table (D146). (Happy Eyeballs, RFC 8305; sgo/docs/happy-eyeballs-plan.md)

## D146 — Lab network for the race · APPROVED 2026-11-03 (owner delegated)

**Decision.** `sgo lab` runs read an optional `lab.net` file in the module: `resolve HOST IP...` entries for the fake resolver, and `blackhole IP:PORT` / `delay IP:PORT MS` entries for endpoints whose fake Dial waits until cancelled or for MS of virtual time. Happy Eyeballs then runs deterministically under replay: same seed, same winner, same trace. (Happy Eyeballs, RFC 8305; sgo/docs/happy-eyeballs-plan.md)

## D147 — `check`: checked propagation · APPROVED 2026-11-04

**Decision.** `check` becomes a reserved word. Two forms: the statement `check f(…)` and the whole right side of an assignment or short declaration, `x₁, …, xₙ := check f(…)`, where the call's last result is `error` and the left side binds the other results. It is defined by its desugaring: if the error is not nil, the enclosing function returns exactly as a written `return` with that error would (crossing nurseries and running deferred calls as any return does); otherwise the targets are assigned, and only then. Not admitted: `check` inside a larger expression, a condition, an argument list, a `return` operand list, or an `if`/`for`/`switch` init statement. The enclosing function must have a trailing `error` result. `check` counts as a use of the error (SGO-CTL-001); `_ = f()` stays the only explicit discard. No annotation or wrapping is added to the error. (sgo/docs/structured-resources-plan.md)

## D148 — Structured resources: a scope owns its resources · APPROVED 2026-11-04

**Decision.** Every resource is acquired into a scope, the way every task is spawned into a nursery. A scope is a nursery (`n`) or a task body (its context: `root`, or a child's `t`). The scope releases its resources when it ends, so no resource outlives its scope and a forgotten `Close` is not a bug. A nursery ends by (1) joining its children, (2) releasing its resources in reverse order of acquisition, (3) computing its outcome; a task body releases its resources before its outcome is published. `Close` stays as an optional early release that returns its error to the caller. Built-in resources: files, listeners, connections, dialers (and every later handle kind). A method on a scope (`*sgo.Context`, `*sgo.Nursery`) acquires into that scope; every other acquiring call takes the scope as its first argument (`l.Accept(t)`, `d.Dial(t, addr)`, `dir.Open(n, path)`).

## D149 — Where a release error goes · APPROVED 2026-11-04

**Decision.** A failed release is never dropped. In a nursery it is a new step of the outcome order (SGO-NUR-004): if the body and every child succeeded, the nursery unwinds with kind `ReleaseError` carrying the first failed release (in release order); otherwise each failed release is appended to the chain of the primary outcome. In a task body: a task returning nil ends `Errored` with the first failed release; otherwise the failed releases join its chain. For `main` this is exit status 1 (`MainError`).

## D150 — Sharing within a scope replaces lending · APPROVED 2026-11-04

**Decision.** A resource is usable by every task running inside its scope: the acquiring task and every task spawned, at any depth, into a nursery nested in that scope, since they all end before the scope releases it. `LendTo` is removed. Explicit `Close` is permitted to any task inside the scope while no other task is waiting in an operation on it (otherwise `ResourceBusy`, no effect). Concurrent use is per kind: sockets keep one waiting reader and one waiting writer at a time (others wait their turn); files serialize operations.

## D151 — A resource never outlives the name of its scope · APPROVED 2026-11-04

**Decision.** Static rule (`SGO_E_RESOURCE_ESCAPES`): a value whose type holds a resource (a resource handle type, or a struct, array, or slice holding one, or a function literal capturing one) is tied to the scope expression it was acquired into, and may not flow beyond the lexical extent of that scope's name: no assignment to a variable declared outside it, no return from inside it, no send on a channel, no capture by a closure that escapes it. Structs may now hold resources (copies are harmless: ownership is the scope's). The one explicit way out is `r.MoveTo(s)`, where `s` is a scope enclosing r's current scope and the caller runs inside r's scope; it re-ties r to `s` statically and moves it at run time.

## D152 — User resources: `s.Own(r)` and `Release(ok bool) error` · APPROVED 2026-11-04

**Decision.** `sgo.Scope` is the interface `*sgo.Context` and `*sgo.Nursery` implement. A user type becomes a resource by being registered with `s.Own(r)`, which requires `r` to have a method `Release(ok bool) error`; the type's values are then subject to D151 (a type with a `Release(bool) error` method is a resource type). At the scope's end `Release(true)` runs if the scope is ending normally and `Release(false)` if it is unwinding (failure or cancellation): commit or roll back. Release order is strict reverse acquisition across built-in and user resources: a user resource owned after the handles it holds were acquired is released before them, so its `Release` can still use them. Release errors follow D149. A panic in `Release` is a failure (SGO-DEF-004).

## D153 — Superseded: abandonment, lending, the connection slot · APPROVED 2026-11-04

**Decision.** D54's `HandleAbandoned` entry is removed (scope release is the normal path). D144's `sgo.ConnSlot` is superseded by `MoveTo` (D151); the slot is removed with `LendTo` (D150). Their clauses (SGO-FILE-003, SGO-NET-002..004) are rewritten in the 0.16.0 freeze.

## D154 — No scope finalizers, no `defer` of fallible calls · APPROVED 2026-11-04

**Decision.** `n.Defer` stays denied for good: cleanup is attached to the resource it releases (D148, D152), not registered separately with a scope. `defer` of a call returning `error` stays denied (SGO-DEF-001). `defer` of non-error calls is unchanged.

## D155 — Release during cancellation · APPROVED 2026-11-04

**Decision.** Releases run while a scope unwinds from cancellation or failure, like deferred calls. A release that waits (a socket close that flushes, a user `Release` that blocks) is not a cancellation point; it is bounded by the drain budget (SPEC-DRAIN): past the budget the process ends `DrainBudgetExceeded` as for any non-cooperative drain.

## D156 — Non-goals of W21 · APPROVED 2026-11-04

**Decision.** Unchanged: comma-ok results (`v, ok := …`), errors as data (inspected with `IsCancelled`, `Error()`, comparison), error values (sealed, no wrapping at `check`, no position recorded). Not added: a `handle` hook, `try`-style operators, exceptions, result types. Channels, events, limiters, and tokens are not resources in this sense (tokens keep their D39 rules) unless a later decision says so.

## D157 — Withdrawn: the connection slot in scope terms · WITHDRAWN 2026-11-05

**Record.** Briefly implemented on the W21 branch: `sgo.ConnSlot` as a resource of its scope, the first `Put` moving a connection into it. Withdrawn before any freeze in favour of D158: a slot is a mailbox, an unstructured construct specific to one resource kind, while the nursery already decides who owns what. D153 stands as written: the slot is removed.

## D158 — The deciding scope owns the candidates · APPROVED 2026-11-05

**Decision.** Hand-off of resources between tasks uses only scopes, `MoveTo`, and channels; there is no slot or lending construct. The idiom: acquire candidates into the scope that decides (for a race, the race nursery: `d.Dial(n, addr)`), report them to its body over a channel, `MoveTo` the chosen one outward, and let the scope's end release the rest.

1. **Channels may carry resources within their scope.** A channel type whose element type holds a resource is a resource type for SGO-RES-003: tied to the extent where it was made, never escaping it. A value tied to scope `s` may be sent only on a channel tied within `s`'s extent; a received value is tied to the channel's tie. `sgo.Copy` of a resource stays denied.
2. **(Gap A) `MoveTo` for user resources.** The runtime records, with each `Own` registration, the registered value itself; `sgo.MoveTo(r, s)` moves the registration of `r` (built-in or user resource) to the enclosing scope `s` under SGO-RES-003's rule. Built-in handles keep their `r.MoveTo(s)` method.
3. **(Gap B) Concurrent use of user resources.** Tasks sharing a scope share its user resources; a user resource serializes itself (with `sgo.Mutex`, a limiter, or its own built-in handles' per-kind rules). The runtime adds no lock around user methods; `Release` runs once, in the task ending the scope, after the join.
4. **(Gap C) Long-lived deciding scopes.** Not a language rule: the idiom is one nested nursery per decision or request, so candidates and per-request resources are released at its end. Documented with the examples.
5. **(Gap D) No inward moves.** A resource is never moved to a narrower scope; narrower use is by sharing (D150), by acquiring where used, or by a lease (a user resource of the narrow scope holding a reference to an outer resource).

Supersedes D144's slot and D157. SGO-NET-003/004 are rewritten: the dialer race is an ordinary nursery.

## D159 — Error exits cancel the nurseries they cross · APPROVED 2026-11-05

**Decision.** A `check` that fails, or a `return` whose error result is not nil, cancels every nursery it crosses before joining it; the cancellation is consumed at each crossed nursery's boundary (SGO-CANCEL-004), and the error propagates unchanged. A normal exit (end of body, `return` with a nil error, `break`, `continue`) still joins without cancelling. Motivation: the Happy Eyeballs review (HE-02) showed that the natural error path, `check` inside a nursery, turned "give up" into "wait for everything". Amends SGO-CTL-004 and SGO-ERR-004.

## D160 — Timers as `select` arms · APPROVED 2026-11-05

**Decision.** `t.After(ms)` is a receive-only timer usable as a `select` arm (`case <-t.After(ms):`), on virtual time in the lab and a scheduling point with the arm tie-break rules of SPEC-CHAN/SPEC-LAB. It needs no clock grant: it reveals elapsed time only by ordering (D30's clock grant still guards reading the time). A timer belongs to the task that made it and costs nothing after its `select` ends. Removes timer tasks, tick events, and their sequence numbers (HE-01, HE-03).

## D161 — Tasks that return values · APPROVED 2026-11-05

**Decision.** `n.Go(func(t *sgo.Context) (T, error)) *sgo.Task[T]` spawns a child whose result is read by the nursery's tasks, also as a `select` arm (`case v, err := <-h:`). A child's error delivered this way is data: it does not fail the nursery (unlike `Spawn`). A `Task[T]` is tied to its nursery (SGO-RES-003); a resource result is acquired into the nursery (D158). `sgo.NewGroup[T](n)` collects the results of many such children in completion order.

## D162 — Scope deadlines · APPROVED 2026-11-05

**Decision.** `n.CancelAfter(ms)` and `t.CancelAfter(ms)` cancel the scope after `ms`, with a cancellation that `sgo.IsDeadline` recognizes (kind `DeadlineExceeded`, still `IsCancelled`). Virtual time in the lab.

## D163 — Tagged unions with exhaustive switch · APPROVED 2026-11-05 (later work package)

**Decision.** A union type lists named cases, each with its own fields; a `switch` over a union must cover every case or have `default`, and each case binds only its fields. Designed after W22 shows how much of the event-loop pattern remains.

## D164 — Whole-message reads · APPROVED 2026-11-05

**Decision.** `ReadAll(max int) (string, error)` (until end of stream, bounded) and `ReadLine(max int) (string, error)` on `*sgo.Conn` and `*sgo.File`. Removes the short-read bug class (HE-04).

## D165 — Inherited deadlines · APPROVED 2026-11-05

**Decision.** A deadline set with `CancelAfter` applies to every nested scope and I/O wait under it; an inner scope may only shorten it. `t.Deadline() (int, bool)` reports the remaining milliseconds, so code can pass it on (an outgoing request, a driver). Like `context.WithTimeout`, but attached to scopes, so it cannot be forgotten.

## D166 — Bounded groups · APPROVED 2026-11-05

**Decision.** `sgo.NewGroup[T](n, limit)`: `g.Go(...)` waits while `limit` children are running. Built on the capacity limiter (SPEC-SYNC) and D161, as one construct, so no program hand-writes the acquire/release pair.

## D167 — Request/reply calls · APPROVED 2026-11-05

**Decision.** `sgo.Call[Req, Resp]`: `c.Call(t, req) (Resp, error)` sends a request and waits for its own reply; the serving side receives `req, reply := c.Accept(t)` and answers with `reply.Send(resp)`. Cancelling the caller withdraws its request; a reply to a withdrawn request is dropped. The server's scope must enclose the caller's (SGO-RES-003). For multiplexers, pool front ends, and services.

## D168 — Spelling of value-returning tasks · APPROVED 2026-11-05 (implementer, delegated)

**Decision.** D161's semantics stand; its surface changes because the trusted `sgo` package is Go code: Go has no generic methods (`n.Go` cannot infer `T`), and a two-value receive is `(value, ok bool)`, never `(value, error)`. The spelling is `h := sgo.Go(n, func(t *sgo.Context) (T, error) {...})` returning `*sgo.Task[T]`; `v, err := h.Wait(t)` waits for the result; and the select arm is `case r := <-h.Result():`, where `Result()` returns a receive-only native channel of `sgo.Result[T]` (fields `Value T`, `Err error`) that the runtime fills once when the child ends. The child's error is data: it does not fail the nursery. `sgo.NewGroup[T](n)` follows the same shape. No compiler special case is needed beyond the admission rules D161 already requires.

## D169 — Effect handles and `LendTo` · APPROVED 2026-11-05 (option a)

**Question.** SGO-RES-002 v2 removed `LendTo` from resources; effect handles (`Clock`, `WallClock`, `Random`, `Entropy`, `SimStore`, SPEC-AUTH) still have it: `h.LendTo(n)` returns a copy revoked when `n` closes (A03, fixture `auth-lend-revoked`). Since W21, structure already bounds what `LendTo` was for: nothing started in `n` runs after `n` closes, and ceilings (`SpawnWithout`) and attenuation (`ReadOnly`, `Prefix`) already narrow authority. Revocation only matters when a copy leaves `n`, which needs a channel to a task outside it.

**Options.**
- (a) **Effect handles are shareable authority tokens; `LendTo` is withdrawn.** Authority is bounded by grants, ceilings, and attenuation, not by scope. A handle may be captured and sent like today. Simplest; one less concept; matches how `Dialer` (a resource) and `Clock` (not one) already differ.
- (b) **Effect handles become scope resources** like `Timer`: `root.SimStore()` is a resource of the root body, `s.Attenuate(n)` (replacing `LendTo`) makes a copy tied to `n` by the flow rule and revoked when `n` ends. Gives compile-time containment and keeps revocation, at the cost of resource rules on every effect handle (no interface conversion, no storing outside the extent).
- (c) Keep `LendTo` as it is (status quo): a run-time revocation unlike every other scoping rule in the language.

**Recommendation.** (a). Revocation has no remaining use that structure doesn't cover, except handing a copy to an outside task over a channel, which (a) treats as deliberate delegation, as passing any authority is. `auth-lend-revoked` becomes a reject fixture (`LendTo` not admitted); SGO-AUTH-001/003 get new versions at 0.17.0.

**Decision.** (a), approved by the owner 2026-11-05. `LendTo` is removed from every effect handle and the revocation state with it (`src/sgo/effects.go`, `src/runtime/sgo_effects.go`); a call is refused as an unadmitted member. Effect handles stay outside the resource rules.

## D170 — Deadlines attach to nurseries · APPROVED 2026-11-05 (implementer, delegated)

**Decision.** D162's `t.CancelAfter(ms)` on a task context is not provided: a task body is not a cancellable scope (cancellation lives on nurseries and a task's linked scopes), and making it one would change every wait's wake path. A timeout for a task's work is written by opening a nursery around it: `nursery (n) { n.CancelAfter(ms); … }`. `n.CancelAfter`, `sgo.IsDeadline`, and `t.Deadline()` are as D162 and D165 decided; `CancelAfter` follows `Cancel`'s caller rule. In the lab, `CancelAfter` ends the run with `UnsupportedLabEffect` for now: a virtual fire happens while the blocked task may hold channel and scope locks, so lab deadlines need their own design (a later step), not a cancellation run inside the scheduler. **Follow-up (2026-11-05):** lab deadlines are implemented as option 1 of `sgo/docs/w22-drafts.md`: an advance that fires a deadline hands the baton to a driver goroutine (a declared runtime service, lab only) that cancels with no lock held.

## D171 — Scope-owned timers · APPROVED 2026-11-05

**Decision.** `tm := n.NewTimer(ms)` (or `t.NewTimer(ms)` for a task body) returns a `*sgo.Timer` owned by that scope: `case <-tm.C():` is a select arm usable in any number of selects, `tm.Reset(ms)` re-arms it from now, and `tm.Stop()` disarms it. A timer is a resource of its scope (it has `Release`), so the flow rule keeps it inside the scope and the scope stops it when it ends. It complements D160's one-select `t.After`: an RFC 8305 attempt delay is measured from the attempt's start, not from each select. No clock grant, as D160. Virtual time in the lab.

## D172 — Groups: the shape of D161's NewGroup and D166 · APPROVED 2026-11-05

**Decision.** `g := sgo.NewGroup[T](n, limit)` (`limit` 0: unbounded) groups value-returning children of `n`: `g.Go(t, f)` starts `f` as `sgo.Go` does, waiting while `limit` children of the group are running (a cancellation point); `case r := <-g.Next():` receives the results in completion order, each once. Results are queued without bound, so a child never blocks on publishing its result and an owner that stops reading cannot deadlock the nursery's join. Slice results are passed as `sgo.Frozen[[]T]` (user decision, 2026-11-05).

**Plan.** W22 (language core): D159, D160, D161, D162, D165, with a model of timers, task handles, and deadline inheritance first; acceptance is Happy Eyeballs rewritten on them. W23 (concurrency library): D164, D166, D167, `Reader`/`Writer` interfaces, a `Pool[T]` helper; acceptance is an HTTP/1.1 server example. Later: D163, TLS, JSON, HTTP/2. Plan: `sgo/docs/w22-w23-plan.md`.

## D173 — Whole-message reads: the read turn and the handle's read buffer (D164) · APPROVED 2026-11-06 (implementer, delegated)

**Decision.** D164's `ReadLine` must read past the line end, so a `*sgo.Conn` or `*sgo.File` keeps the bytes it read and has not returned in a buffer on the handle, and every read (`Read`, `ReadLine`, `ReadAll`) takes the handle's read turn for its duration: one read at a time per handle, another task's read waits for the turn (a cancellation point), so buffered bytes are never reordered between readers. `Read(max)` returns buffered bytes first, without a system call. `ReadLine(max)` returns the next line without its `\n` and one `\r` before it; the line with its terminator is at most `max` bytes, else the error is `TooLong` (`sgo.IsTooLong`) and nothing is consumed; a last line without `\n` is returned at end of stream, and `EOF` comes only when nothing is left. `ReadAll(max)` reads to end of stream, at most `max` bytes, else `TooLong` with nothing consumed. A cancelled or failed `ReadLine`/`ReadAll` consumes nothing: the bytes it read stay buffered for the next read. No read-ahead conflicts with writing: a file is opened for reading or for writing, never both. The buffer is part of the handle and goes with it on `MoveTo`; closing the handle discards it.

**Why.** Every bug class HE-04 named (a greeting split across reads, a line split across chunks) disappears without a user-written loop, and no read loses data to cancellation, which a user-level buffer could not promise when the reading task is cancelled between chunks.

## D174 — Request/reply calls: the shape of D167 · APPROVED 2026-11-06 (implementer, delegated)

**Decision.** `c := sgo.NewCall[Req, Resp](n)` makes a call point served inside nursery `n`; the handle is shareable like task handles (D169), and only tasks inside `n`'s scope may use it. `resp, err := c.Call(t, req)` hands `req` to a server and waits for its own reply. The server takes requests with `r := c.Accept(t)` and answers with `r.Reply(resp, err)`; `r.Value()` is the request. Requests are handed over by rendezvous: a caller cancelled before a server accepted its request withdraws it, and no server ever sees it. Both waits are cancellation points with the unwinding of a native receive (SGO-NATIVE, as `Task.Wait`). A reply to a caller that was cancelled after the request was accepted is dropped. An accepted request is a resource of the accepting task's body: if the body ends without `Reply`, the caller's `Call` returns the error `NoReply` (`sgo.IsNoReply`), so a server that fails, unwinds, or forgets never strands a caller. A second `Reply` is a `SpawnContractViolation`. `Req` and `Resp` are types a task may hand to another (the rule of `sgo.Go` results: no task-local types, no resources). Accepting through a `select` arm is not in this version: a request received by an arm could not be registered with its receiver, so it would bypass the `NoReply` guarantee; a server's shutdown is the cancellation of its scope, which ends `Accept`.

**Amended 2026-11-06 (review of `6db6fde457`).** `Accept(s sgo.Scope)` names the scope the request belongs to, as every acquisition does (SGO-RES-001), instead of hard-wiring the accepting task's body: a server that accepts into a nursery per iteration answers a forgotten request with `NoReply` at that iteration's end. Whether a request was answered is decided once under the reply record's lock (`Reply` loses to an earlier answer, which is the `SpawnContractViolation`). A call point is shareable and its use outside its nursery is a runtime `MissingCapability`, not a compile-time error; `Call` with no server waits until its scope is cancelled.

**Why.** The multiplexers, pool front ends, and services of W23 need a reply that is guaranteed to come back or fail, and withdrawal that cannot be raced; built on the rendezvous channel and the task-result record, the only new runtime state is the `NoReply` release.

## D175 — Pools with leases (W23 step 4) · APPROVED 2026-11-06 (implementer, delegated)

**Decision.** `p := sgo.NewPool[T](n)` makes a pool of items of type `T` that lives in nursery `n`; the handle is shareable like task handles and call points, and is tied to `n` when `T` holds resources (as `*sgo.Task[T]` is). `p.Put(v)` adds an idle item; it is checked as a send on the pool (D158): an item that holds a resource must be tied at least as long as the pool, so an item is never released before its pool. `l := p.Acquire(s)` waits for an idle item and returns a lease, a resource of scope `s` (SGO-RES-001), which must lie within the pool's extent; `l.Value()` is the item, tied as the lease. Waiters are served oldest first; the wait is a cancellation point with the unwinding of a native receive, and a waiter either gets an item or is cancelled, never both (an item is handed over by the channel protocol). When the lease is released (its scope ends, after the scope's tasks have ended), the item goes back to the pool: to the oldest waiter, or idle. `l.Return()` puts it back early; `l.Drop()` keeps it out of the pool for good (a connection found broken; the item is still released by its own owner). Using a lease after `Return` or `Drop` is a `SpawnContractViolation`. `T` is a type a task may hand to another (the rule of `sgo.Go` results). Commit or rollback of a transaction over a leased connection is the transaction's own business: a user resource in the lease's scope sees `Release(false)` when that scope unwinds or an error exit crosses it (D159), and the item still goes back.

**Why.** The W23 server example needs a database-style pool whose connections are used exclusively, come back on every exit path, and cannot outlive the nursery that owns them. Built on the unbounded native queue of groups (`sgo_group.go`) and on resource registration (`runtime_resOwn`), it adds no runtime state. Model: `sgo/model/poolmodel.go` (1 422 states; four mutants caught: a returned item put back twice, a dropped item put back, a cancelled waiter taking an item, a put-back that wakes no waiter).

## D176 — Resource interfaces: an interface may hold resources only by embedding `sgo.Resource` · APPROVED 2026-11-06 (user)

**Question.** W23 step 4 planned "Reader/Writer interfaces over the I/O types", but a resource never becomes an interface value (W21 review Z1, SGO-RES-003): an interface is not tied to a scope. Options were resource interfaces (a), generic functions over a method constraint (b), or no abstraction (c). The user rejected (b) (generics make code hard to read and reason about) and asked for (a) in a general form: any interface, not only readers and writers, ergonomic and readable. A second design, `.work/sgo-scoped-interfaces.md`, proposed letting every interface, `any` included, hold resources with the compiler inferring each value's provenance; it was reviewed and rejected, and is not a planned extension.

**Decision.** An interface is a *resource interface* when it embeds `sgo.Resource`, directly or through another resource interface. `sgo.Reader`, `sgo.LineReader`, and `sgo.Writer` are predeclared resource interfaces with no other privilege; users declare their own the same way (`type Store interface { sgo.Resource; Get(string) (string, error) }`). The rules:

1. **A resource interface is a resource type** (SGO-RES-001): every flow rule of resources applies to its values unchanged — tied to the scope of what they hold, no assignment, return, capture, send, or storage beyond that scope (SGO-RES-003), refused by `sgo.Copy`, `Freeze`, and as a map key, exactly where resource types are refused.
2. **Conversion.** A value converts to a resource interface whose methods it has; the interface value is tied as the source (a resource-free source gives an untied value). A resource, or a value of a resource interface, converts to no other interface: `any`, `error`, and every interface that does not embed the marker still never hold a resource (Z1 for them is unchanged). So the type alone tells whether a value may hold a resource; nothing is inferred.
3. **Assertions.** `x.(T)` and type switches from a resource interface give values tied as `x`; asserting to a non-resource interface (or to `any`) is refused.
4. **Ownership.** Conversion moves nothing: the resource stays in its scope's list and is released once by it; `Close`/`Release` through the interface is the dynamic value's own method. Moves stay explicit (`MoveTo` on the concrete handle, then convert).
5. **Sharing.** Resource interface values are task-local (D132): a dynamic value may be a plain mutable value or a user resource pointer. A concrete handle crosses into a child and is converted there.
6. **Wrappers (adopted from the reviewed alternative).** The result of a call whose resource results can only hold what its resource arguments hold — a constructor such as `NewBufferedReader(src sgo.Reader) *BufferedReader` — is tied as narrowly as those arguments jointly, not merely to the caller's scope; this applies to concrete handles too. A result that holds resources from several arguments must satisfy each tie: for nested scopes that is the innermost, and arguments whose scopes the compiler cannot order make the result unusable beyond the call, with a diagnostic.
7. **Calls through an interface.** A resource result of a method called through a resource interface is tied to the receiver and to the resource arguments. A method body that stores a resource parameter into longer-lived memory is already refused (SGO-RES-005) for every implementation, so no separate retention contract is needed.
8. **Conservatism.** When a relation cannot be established the program is rejected; diagnostics name the user's objects ("the buffered reader holds a connection of nursery n; this assignment lets it outlive n").

**Amended 2026-11-06 (review of `a27425d5c5`).** Rule 6 is made precise (C1), with no callee summary: for a call of a function or method outside package `sgo`, the result is tied to the innermost of the ties of its *authority arguments* — resource-typed arguments (handles, user resources, resource interfaces, and aggregates or pointers holding them), scope arguments (`*sgo.Nursery`, `*sgo.Context`, `sgo.Scope`), handles delivering resources (`*Task`/`*Group`/`*Pool`/`*Call` of a resource type), and the receiver. If the callee is a function value, or an argument is a function value (a closure can carry any scope it captured), or no authority argument exists, or two of their ties cannot be ordered, the result is tied to the caller's current extent, as before. Rule 7 applies the same with the interface receiver as an authority argument (C2). Package `sgo` calls keep their specified ties. Also (C3): a nil resource interface value is admitted and a method call on it is `NilDereference`; `==` on resource interfaces is refused (D128); an assertion from a resource interface to a concrete handle type gives that handle, shareable within its scope as before conversion; a conversion to a narrower resource interface keeps the tie. **Names (user, 2026-11-06):** `*sgo.Writer` is already the stdout handle (SGO-PROC-003), so the predeclared interfaces are in the standard package `std/io` (`io.Reader`, `io.LineReader`, `io.Writer`, `io.ReadWriter`, `io.LineReadWriter`, `io.WriteString`; SGO-STD-008), declared in S-Go source by embedding `sgo.Resource` like any user interface. Standard output has `Write` and converts to `io.Writer` untied.

**Why.** The marker keeps the analysis type-based: code that never touches resources (every `any`, `error`, and unmarked interface) keeps exactly today's rules, the checker reuses the existing flow pass instead of inferring provenance per value across packages, and a reader of the code sees from a declaration whether a value may hold a connection. Cost: one embedded line in interfaces that carry I/O, usually inherited from `sgo.Reader`/`sgo.Writer`. Amends SGO-RES-003 (Z1 holds for non-resource interfaces) and SGO-IFACE at 0.18.0; new draft clause for resource interfaces.

## D177 — `Listen` and `Dial` take the scope that owns the handle · PROPOSED

**Question.** `root.Listen(addr)` and `root.Dial(addr)` take no scope: the handle belongs to the scope of the calling task, so a function that listens for a nursery's lifetime must close the listener by hand after the nursery (`check l.Close()` in `sgo/examples/httpserver`). `l.MoveTo(n)` cannot hand it to the nursery, because ownership only moves outward (D158). Every other acquisition already names its scope: `l.Accept(s)`, `d.Dial(s, addr)`, `dir.Open(s, name)`, `n.NewTimer`.

**Recommendation.** `c.Listen(s sgo.Scope, addr string)` and `c.Dial(s sgo.Scope, addr string)`: `c` is the context holding the grant (as today), `s` is the scope that owns the handle, the caller's current scope or one enclosing it (as `Dir.Open`). The handle is released when `s` ends, so `Run`-style functions listen into their nursery and need no `Close`. No one-argument form is kept (no compatibility mode). Every call site changes (32 programs in the corpus); SGO-NET-002 and SGO-NET-003 get new versions.

**Why.** One rule for every acquisition: the code shows who owns a handle where it is made, and a lifetime is a block, not a call the reader must find. Cost: one more argument at each call; a mechanical migration.

## D178 — Command-line arguments · PROPOSED

**Question.** SGO-PROC-004 gives a program no access to its command line, so a program cannot choose what to do (`httpserver serve`, `httpserver get /hello`) and examples run fixed demos.

**Recommendation.** `root.Args() []string` returns a fresh copy of the arguments after the program name. Only the root context has it (it is input to the program, given by whoever started it; children receive what the root passes them). Arguments are not authority: grants stay in `sgo.mod`, and no argument can widen them. The driver passes them as `sgo run PKG -- ARGS...` and `sgo test` passes none. In the lab, the arguments are part of the run's manifest: `sgo lab run PKG -- ARGS` records them in the trace, and replay with different arguments is refused, as a different build is today. Environment variables stay ignored (SGO-PROC-004, which gets a new version).

**Why.** Arguments are the smallest input a program needs to be a tool rather than a demo. They are deterministic per run, so the lab keeps exact replay by recording them. A slice copy keeps the task-local rules unchanged.

## D179 — Interrupts cancel the root · PROPOSED

**Question.** OS signals are denied (SGO-PROC-001): SIGINT or SIGTERM ends a program outside S-Go's outcomes, with no release of its scopes. A server run until interrupted cannot shut down through its scopes.

**Recommendation.** The first SIGINT or SIGTERM cancels the root scope, with a cancellation that `sgo.IsInterrupted` recognizes (and `sgo.IsCancelled`); the program then unwinds through its nurseries, releasing every resource, and exits with the outcome of that unwind (status 1 if `main` returns the cancellation). A second signal during the unwind ends the process at once with a fixed status (130 / 143). Other signals keep their default actions. In the lab, no signal is delivered; a scenario may inject an interrupt at a virtual time, recorded in the trace.

**Why.** Graceful shutdown then needs no new construct: it is the cancellation every S-Go program already handles, and the release order is the one the scopes already guarantee. Cost: a runtime signal handler, which the runtime already installs for other purposes; a new cause of cancellation in SGO-CANCEL.

**Plan (W24, after approval).** D177 first (mechanical migration, simplest examples). Then D178 and D179 together: `httpserver` becomes `serve` / `get PATH`, with the demo kept as `demo`. Model first for D179 (an interrupt racing scope ends). Spec 0.19.0.

---

## Approval record

- 2026-10-30: user approved D01–D21 as recommended ("decision approved").
- 2026-10-30: user approved D22 (darwin/arm64 first; linux/amd64 real target; linux/arm64 first-class).
- Follow-up applied: execution plan amended (§1.2, §4.2, §4.5, §4.6, §15.1), start prompt reading order and pin updated, missing inventory families added to `SGO_SPEC_ADMISSION.json` as denied in subset 0, `SGO_SPEC_SUBSET0.md` drafted.
- 2026-10-31: user approved D23–D30 as recommended ("recommendations are good enough"). Follow-up: spec 0.2.0 clauses for capabilities, root grants, first effects, readiness, and sgo.Channel permits; W05 and W06 implementation starts after those clauses are frozen.
- 2026-11-01: user approved D31–D39 as recommended (D39, structured `Event` and `CapacityLimiter`, added at the user's request; D37 revised to point to it). Follow-up: spec 0.3.0 clauses for native channels, `select`, the structured primitives, and the D38 carry-overs; W07 implementation, model first.
- 2026-11-02: user approved D40–D49 as recommended ("looks good, approved!"). Follow-up: spec 0.4.0 clauses for the lab execution configuration (scheduler, boundaries and quota, streams, effects, trace and replay, outcomes); W08 implementation, model first (D30).
- 2026-11-02: user approved D50 as recommended (source-order select arm indices in the lab trace).
- 2026-11-02: user approved the SPEC-LAB 0.4.0 clauses SGO-LAB-002 to SGO-LAB-006 as drafted ("looks good, continue"); frozen in spec 0.4.0 with the lab conformance lane (17 lab cases).
- 2026-11-02: user approved D51–D59 as recommended ("looks good, continue"). Follow-up: spec 0.5.0 clauses for confined files and TCP endpoints (SPEC-FILE, SPEC-NET); W09 implementation, model first (D30).
- 2026-11-02: user approved D60 as recommended (Unix domain sockets: exact path grants, socket-only stale removal, stream sockets on paths only, `PeerUID` in 0.5.0) and the SPEC-FILE/SPEC-NET 0.5.0 drafts otherwise; D60 is folded into 0.5.0 before freezing.
- 2026-11-02: user approved D61-D68 as recommended ("approved"; D62 option (a), one process per test). Follow-up: spec 0.6.0 clauses for `sgo test` and `sgo check`; W10 implementation.
- 2026-11-02: user approved D69 as recommended ("the recommendations are good") together with the drafts SGO-TOOL-002, SGO-TEST-002, SGO-TEST-003. Follow-up: StartTest restricted to the generated main (`-sgotestmain`), test conformance cases, freeze in spec 0.6.0.
- 2026-11-02: user approved D70-D74 as recommended ("approved; go ahead"), including D70's adoption threshold (>= 20% P50 RSS or wall on the idle-task and example workloads, no other row worse than 5%, every correctness case passing on all three targets). Follow-up: W11 architecture note, stack-mapping probe (darwin locally, linux/amd64 on note2), `vstack` prototype.
- 2026-11-02: user chose option A after the probe stage ("ok, option A"). W11 closes `investigated_not_adopted` without building the D71/D72 prototype, because the probe measured the layout's lower bound at 2-6x the baseline's memory per idle task on all three targets, so D70's threshold can't be met. F03-F05 stay unrun (they need the prototype). Default runtime unchanged (growable stacks, SGO-STACK-001). Evidence: `sgo/evidence/w11/README.md`.
- 2026-11-02: user approved D75-D82 as recommended ("approved"). For D75 the reviewer is the existing job `sgolang-reviewer` on note2 (`job:note2/id:77bfab38c5a3`), which has its own clone and context and has not worked on S-Go code. Follow-up: review brief `sgo/docs/w12-review-brief.md`, findings ledger `sgo/evidence/w12/review-findings.json`, then D76-D81 in parallel with the review.
- 2026-11-02: user approved D83 (spec 0.7.0 as the release registry) and D84 (S-Go programs ignore GOMAXPROCS like every other runtime environment setting) as recommended ("ok, continue").
- 2026-11-02: user approved D85-D92 as recommended ("looks good; go ahead"). W13 starts with step 13.0 (sharing-rule model, pending fixtures for the five programs).
- 2026-11-02: user approved the W13 stage 1 drafts SGO-COMP-002, SGO-COMP-003, SGO-UTYPE-002 (sgo/docs/w13-stage1-drafts.md) as written ("approved; go ahead"). Stage 1 implementation starts; the drafts move into sgo/spec at the 0.8.0 freeze.
- 2026-11-02: user approved D93-D97 and the W13 stage 2 drafts (sgo/docs/w13-stage2-drafts.md: SGO-COMP-004..008, SGO-TYPE-007) as recommended ("approved; go ahead"). Stage 2 implementation starts; the drafts freeze at spec 0.9.0.
- 2026-11-03: user approved D98–D101 and the W13 stage 3 draft (sgo/docs/w13-stage3-drafts.md: SGO-COMP-009) as recommended ("approved; go ahead"). Stage 3 implementation starts; the draft freezes at spec 0.10.0.
- 2026-11-03: user directed "close W13" (D102: stage 4 deferred) and to prepare for the standard library.
- 2026-11-03: user approved D103–D110 and the standard-library foundations plan (sgo/docs/stdlib-foundations-plan.md) as recommended ("looks good; approved"). W14 clause drafts follow for approval before implementation.
- 2026-11-03: user approved D111–D115 and the W14 drafts (sgo/docs/w14-drafts.md: SGO-IMP-001 v2, SGO-IMP-002 v2, SGO-IMP-003, SGO-IMP-004) as recommended ("looks good; approved"). W14 implementation starts; the drafts freeze at spec 0.11.0.
- 2026-11-03: user approved D116–D120 and the W15 drafts (sgo/docs/w15-drafts.md: SGO-LEX-004 v2, SGO-TYPE-008, SGO-TYPE-001 v8, SGO-TYPE-007 v2, SGO-VAL-002 v3, SGO-COMP-005 v2) as recommended ("looks good; approved"). W15 implementation starts; the drafts freeze at spec 0.12.0.
- 2026-11-03: user approved D121–D126 and the W16 drafts (sgo/docs/w16-drafts.md: SGO-STD-001..007, SGO-IMP-004 v2) as recommended ("looks good; approved"). W16 implementation starts; the drafts freeze at spec 0.13.0.
- 2026-11-03: user approved D127–D133 and the W17 drafts (sgo/docs/w17-drafts.md: SGO-IFACE-001 v2, SGO-IFACE-002..004, amendments) as recommended ("looks good; approved"). W17 starts with the share model; the drafts freeze at spec 0.14.0.
- 2026-11-05: user approved D169 option (a): effect handles are shareable authority tokens, `LendTo` withdrawn.
- 2026-11-03: the owner delegated D143–D146 to the implementer ("own the decisions, handle it to deliver a working version of Happy Eyeballs algorithm (RFC 8305)"). IPv6 endpoints were already admitted (SGO-NET-002); a connect timeout is the cancellation of an attempt (a waiting Dial is a cancellation point), so no timeout API is added. D134–D142 stay proposed.
- 2026-11-04: user approved structured resources and `check` (D147–D156) after a design discussion ("this looks great! approved!"): resources are owned by scopes (nurseries and task bodies) and released when the scope ends; the earlier `defer check` draft (kept in a scratch file, not in the repo) was dropped. W21 starts with the scope–resource model.
- 2026-11-04: user approved the W21 clause drafts (sgo/docs/w21-drafts.md: SGO-ERR-004, SGO-RES-001..004, amendments) as written ("approved!"). W21 implementation starts with `check`; the drafts freeze at spec 0.16.0.
- 2026-11-05: user approved D158 after a design discussion ("approved; go ahead"): no slot construct; the deciding scope owns candidates, channels carry resources within their scope, `MoveTo` works for user resources, user resources serialize themselves. D157 (connection slot in scope terms) is withdrawn.
- 2026-11-05: user approved D159–D167 ("approved, do it") after a design discussion derived from the Happy Eyeballs review: error exits cancel crossed nurseries, timers as select arms, value-returning tasks and groups, scope and inherited deadlines, whole-message reads, bounded groups, request/reply calls; tagged unions later. Work packages W22 (language core) and W23 (concurrency library).
- 2026-11-05: implementer recorded D168 (spelling of D161, forced by the Go type system) under the delegated authority; semantics unchanged.
- 2026-11-05: implementer recorded D170 (deadlines on nurseries only; lab deadlines deferred) under the delegated authority.
- 2026-11-05: user approved D171 (scope-owned timers) and groups before the Happy Eyeballs rewrite, with frozen slice results ("yes, go ahead"; "slice results => frozen is better"); recorded with the group shape as D172.
- 2026-11-06: implementer recorded D173 (the read turn and read buffer behind D164) under the delegated authority.
- 2026-11-06: implementer recorded D174 (the shape of D167's request/reply calls) under the delegated authority.
- 2026-11-06: implementer recorded D175 (pools with leases) under the delegated authority.
- 2026-11-06: user approved D176 option (a) with the opt-in marker only ("I agree ... let's stick with the marker mechanism only"), after rejecting generic functions (b) and the inferred all-interfaces design of `.work/sgo-scoped-interfaces.md`; `any`, `error`, and unmarked interfaces never hold resources.
