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


## D40 — Spec version and scope of the lab **[DEFAULT]** · PROPOSED 2026-11-02

**Recommendation.** The lab is spec **0.4.0**, a strict superset of 0.3.0 (as D23, D31): no program changes meaning and no construct is added. It adds the **lab execution configuration** (plan §1.3: a configuration, not a dialect) with its own clauses: scheduler, virtual time, effect policy, trace and replay, outcomes. A program built for the lab and one built for production are the same source under the same admission; they differ only in the scheduling and effect adapters and in the recorded execution configuration (S14). Lab support is W08 only; simulated network and file fixtures belong with W09.

## D41 — How a lab run is started **[USER]** · PROPOSED 2026-11-02

**Why it matters.** The lab must never activate by accident in production, and an environment variable or flag that changes production behaviour would be a mode the plan forbids (S15).

**Options.**
- **(a) Separate lab build.** `sgo lab run [-seed N] [-trace FILE] [-sched random|fifo] [-quota N]`, `sgo lab replay FILE`, and `sgo lab sweep -seeds A..B` build the program as a **lab binary**: the execution configuration `lab` is part of the build identity (objects and binaries of the two configurations refuse to mix, SGO-IMP-002), and the run's manifest is passed by the driver. A production binary contains no path that enables the lab scheduler.
- **(b) Runtime switch.** One binary; the lab is enabled by a manifest file or environment variable at start.

**Recommendation: (a).** It keeps production binaries free of any lab activation path and makes "this trace came from a lab binary of exactly this program" checkable from the identity (D04, S14).

## D42 — Scheduler: single managed user execution **[USER]** · PROPOSED 2026-11-02

**Why it matters.** Replay needs every user-visible decision to come from the trace; Go's scheduler, `GOMAXPROCS`, and GC timing must not change them (plan §9.1, D10).

**Options.**
- **(a) Baton scheduler over the S-Go runtime.** Exactly one S-Go task holds the baton and runs; every other task is parked by the S-Go runtime. At each **scheduling boundary** (D43) the running task yields; the scheduler computes the canonical ready set (task IDs in creation order), chooses one (seeded generator or the trace), and hands it the baton. Runtime goroutines (GC, services) still run freely; they never hold the baton and never make user-visible decisions. Upstream `synctest` is a source pattern (durable blocking, fake time, idleness detection), not the mechanism, because it has no choice layer.
- **(b) Extend `testing/synctest` bubbles** with a recorded choice for every runnable goroutine.

**Recommendation: (a).** S-Go already funnels every blocking and wake-up through its own runtime (nurseries, channels, events, sleep, cancellation), so the ready set and every decision are in S-Go code with stable IDs, independent of Go's run queues. (b) would have to intercept Go's scheduler itself.

## D43 — Scheduling boundaries and the step quota **[USER]** · PROPOSED 2026-11-02

**Recommendation.**
- **Choice points:** spawn, task end, nursery join, every channel operation and `select`, `Event`/`CapacityLimiter` operations, `Sleep`, `Checkpoint`, readiness, effect operations, and `Writer.WriteString`. At each, the ready set and the chosen task are recorded; a `select` also records its ready arms and the chosen arm (D34).
- **Steps:** a lab build instruments every loop back-edge and function entry of S-Go code with a step counter (a lab-only compiler instrumentation, recorded in the identity). Every `K` steps (manifest, default 1024) the running task reaches a deterministic yield, which is also a choice point, so a busy loop cannot starve others forever. The total steps are bounded by the **quota** (manifest); exhausting it ends the run with `LabQuotaExhausted` (D07, D11).
- Production builds have no such instrumentation.

## D44 — Generator and streams **[USER]** · PROPOSED 2026-11-02

**Options.**
- **(a) PCG-DXSM 128** (the algorithm of `sgo.Random`, D26, already checked against `math/rand/v2`'s vectors), with unbiased bounded sampling, and **independent streams** derived from the manifest's master seed by a frozen SplitMix64 step: `sched`, `fault`, and `data`.
- **(b) PCG32** as named in plan §9.4, a second algorithm with its own vectors.

**Recommendation: (a).** One generator in the TCB instead of two, with reference vectors already in the evidence; stream isolation is tested (D01). The plan accepts PCG32; it does not require it.

## D45 — Effects in the lab **[USER]** · PROPOSED 2026-11-02

**Recommendation.**
- **Clock:** virtual monotonic time, starting at 0; it advances only when no task is ready, to the next timer deadline (D06, D07). `Sleep` uses virtual time; timers with equal deadlines fire in `(deadline, timer ID)` order.
- **WallClock:** the manifest's fixed epoch plus virtual time.
- **Random:** unchanged (it is already seeded by the program).
- **Entropy:** `UnsupportedLabEffect` (a lab failure kind, status 2) in the default policy; it is unpredictable by definition, and silently substituting a seeded stream would hide that from the program's author. A later manifest option may supply recorded entropy.
- **SimStore:** the existing in-process store (deterministic), starting empty.
- **Stdout:** written and also recorded as `write` events in the trace.
- Anything else effectful is already denied at compile time in 0.3.0, so the lab subset is all of 0.3.0 except Entropy (D09).

## D46 — Trace and replay format **[DEFAULT]** · PROPOSED 2026-11-02

**Recommendation.** Versioned JSONL (plan §9.5): a header with the manifest (identity of binary, spec, admission, configuration, target, sources, seed and streams, scheduler and instrumentation versions, quota, epoch), one line per event (sequence, virtual time, task/scope/operation IDs, kind, ready set, choice, arm set, written text, outcome), and a footer with the event count, completion status, and SHA-256 of the canonical lines. No PIDs, host paths, addresses, or wall-clock durations in canonical lines; host data goes to a sidecar file. `replay` rejects an incompatible manifest before running, replays every choice from the trace (never from the generator), and stops at the first divergence, reporting the expected and actual ready sets and the last matching event (D03, D04, D05, D12).

## D47 — Lab outcomes **[DEFAULT]** · PROPOSED 2026-11-02

**Recommendation.** In addition to every production outcome (unchanged, S14): **`LabDeadlock`** when no task is ready and no timer is pending while a task is live, with the wait graph (each live task, the operation it waits in, the scope chain) in the record (D08); **`LabQuotaExhausted`** when the quota is used up (not a deadlock, D07); **`UnsupportedLabEffect`** (D45). Each is one diagnostic record, status 2. A run that hangs without reaching a boundary (uninstrumented TCB code) is ended by the driver's wall-clock watchdog and reported in the sidecar as a host hang, never as a managed outcome (D11).

## D48 — Sweeps and saved failures **[DEFAULT]** · PROPOSED 2026-11-02

**Recommendation.** `sgo lab sweep -seeds A..B` runs each seed in a fresh process and reports the distinct semantic outcomes (exit status, stdout, diagnostic record). A run **fails** if its exit status is not 0, or, with `-expect FILE`, if its stdout differs from the file. The first failure's trace is saved; `sgo lab replay` reproduces it in a fresh process (D13). A sweep passing means those runs passed, not that every schedule does (plan §9.6); the record says so.

## D49 — Scheduler policies **[DEFAULT]** · PROPOSED 2026-11-02

**Recommendation.** `random` (default): uniform choice from the ready set with the `sched` stream. `fifo`: the longest-ready task runs next (a diagnostic, fair round-robin scheduler). The scheduler policy and version are in the manifest; a random schedule is not assumed fair, and the quota bounds every run.

---

## Approval record

- 2026-10-30: user approved D01–D21 as recommended ("decision approved").
- 2026-10-30: user approved D22 (darwin/arm64 first; linux/amd64 real target; linux/arm64 first-class).
- Follow-up applied: execution plan amended (§1.2, §4.2, §4.5, §4.6, §15.1), start prompt reading order and pin updated, missing inventory families added to `SGO_SPEC_ADMISSION.json` as denied in subset 0, `SGO_SPEC_SUBSET0.md` drafted.
- 2026-10-31: user approved D23–D30 as recommended ("recommendations are good enough"). Follow-up: spec 0.2.0 clauses for capabilities, root grants, first effects, readiness, and sgo.Channel permits; W05 and W06 implementation starts after those clauses are frozen.
- 2026-11-01: user approved D31–D39 as recommended (D39, structured `Event` and `CapacityLimiter`, added at the user's request; D37 revised to point to it). Follow-up: spec 0.3.0 clauses for native channels, `select`, the structured primitives, and the D38 carry-overs; W07 implementation, model first.
