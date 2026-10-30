# S-Go open design decisions

**Version:** 1 · **Date:** 2026-10-30 · **Status:** D01–D21 APPROVED by the user on 2026-10-30, as recommended. D22 OPEN.

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
| D22 | First execution target vs. darwin/arm64 dev host | Keep Linux/amd64 (plan default) until decided | USER | OPEN |

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

## D22 — First execution target **[USER]** · OPEN

The plan's first target is Linux/amd64 (§1.2). The user's development host is darwin/arm64 (host Go 1.26.2). Options: (a) keep Linux/amd64 and develop on a Linux/amd64 VM or remote host (emulated amd64 on Apple Silicon is slow); (b) make Linux/arm64 the first target, which runs natively in a VM/container on the host; (c) make darwin/arm64 first, pulling macOS-specific runtime work into the first slice. **Until decided, the plan default (a) stands.** W00 must not lock a target before this decision is recorded.

---

## Approval record

- 2026-10-30: user approved D01–D21 as recommended ("decision approved").
- Follow-up applied: execution plan amended (§1.2, §4.2, §4.5, §4.6, §15.1), start prompt reading order and pin updated, missing inventory families added to `SGO_SPEC_ADMISSION.json` as denied in subset 0, `SGO_SPEC_SUBSET0.md` drafted.
