# S-Go subset 0 — draft specification

**Version:** 0.1-draft · **Date:** 2026-10-30 · **Status:** DRAFT. No clause is frozen or admitted. W01 completes, freezes, and digests these clauses (plan §11 W01, decision D18).

## 0. Status and scope

Subset 0 is the closed language admitted by the first vertical slice (W00–W04, plan §15.1). It is derived from the user-approved decisions D01–D21 in `SGO_DECISIONS.md`. **Everything not specified here is denied** and must produce a compile-time diagnostic, never a fallback (constitution §2).

This draft states each clause's normative core, enforcement, and the related acceptance cases. Before freezing, W01 must expand every clause to the seven contract dimensions of constitution §3 (identity, static legality, value semantics, concurrency, failure, enforcement, evidence), or record why a dimension does not apply. Drafting choices that were open in the first draft are resolved in §18 (agent defaults the user may override).

Clause IDs follow D18: `SGO-<FAMILY>-<NNN>`, never reused. Families map to rows in `SGO_SPEC_ADMISSION.json`.

### 0.1 Example program

```go
package main

import "sgo"

func work(task *sgo.Context, out *sgo.Writer, id string) error {
    if err := task.Sleep(10); err != nil {
        return err // cancellation outcome, not a failure
    }
    return out.WriteString("done " + id + "\n")
}

func main(root *sgo.Context) error {
    out := root.Stdout()
    nursery (n) {
        n.Spawn(func(task *sgo.Context) error { return work(task, out, "a") })
        n.Spawn(func(task *sgo.Context) error { return work(task, out, "b") })
    }
    // Both children have terminated here.
    return out.WriteString("all done\n")
}
```

### 0.2 The `sgo` package surface in subset 0

The complete admitted API. Each item is compiler-known and individually specified; nothing else in package `sgo` is admitted.

| Item | Clause |
|---|---|
| `type Context` (opaque; used as `*sgo.Context`) | SGO-TYPE-005 |
| `type Nursery` (opaque; used as `*sgo.Nursery`) | SGO-TYPE-005 |
| `type Writer` (opaque; used as `*sgo.Writer`) | SGO-TYPE-005, SGO-PROC-003 |
| `func NewError(msg string) error` | SGO-TYPE-006 |
| `func IsCancelled(err error) bool` | SGO-CANCEL-003 |
| `func (*Nursery) Spawn(f func(*Context) error)` | SGO-SPAWN-001..004 |
| `func (*Nursery) Cancel()` | SGO-CANCEL-001 |
| `func (*Context) Checkpoint() error` | SGO-CANCEL-002 |
| `func (*Context) Sleep(ms int) error` | SGO-WAIT-001 |
| `func (*Context) Stdout() *Writer` | SGO-PROC-003 |
| `func (*Writer) WriteString(s string) error` | SGO-PROC-003 |

`Cancel`, `Checkpoint`, `Sleep`, and `IsCancelled` are the minimum API the first slice needs ("cancels supported waits"). Their spelling follows D11's style (resolution R1, §18).

---

## 1. Lexing — SPEC-LEX

### SGO-LEX-001 — Source text
Source files are UTF-8 with the `.sgo` extension; `.go` files are never S-Go input (R2). Invalid UTF-8, a byte-order mark other than at file start, and NUL bytes are compile errors.

### SGO-LEX-002 — Tokens and semicolons
Identifiers, operators, comments, and automatic semicolon insertion follow the rules adapted from the Go specification at `go1.27.3`, with attribution (D19). The adapted text is reproduced in the frozen clause; it does not track later Go changes.

### SGO-LEX-003 — Keywords
Reserved words: Go's 25 keywords plus `nursery`. Reserved words can never be identifiers. Keywords whose constructs are denied in subset 0 (`go`, `goto`, `select`, `chan`, `interface`, `map`, `struct`, `type`, `fallthrough`, `range`) are still recognized so that use of them produces a specific denial diagnostic (SGO-ERR-001). `go` produces `SGO_E_UNSTRUCTURED_SPAWN` (plan §4.2).

### SGO-LEX-004 — Literals
Admitted: decimal, hexadecimal (`0x`), octal (`0o`), and binary (`0b`) integer literals with optional `_` separators; interpreted string literals with escapes `\n \t \r \\ \" \xHH`; raw string literals. Denied: floating-point, imaginary, and rune literals, and `\u`/`\U`/octal escapes (R2).

Tests: X01, X03, S02, S05, S19.

## 2. Names and binding — SPEC-BIND

### SGO-BIND-001 — Packages and files
Every file starts with a package clause. Package `main` must declare the entry point of SGO-PROC-001. Other packages are user library packages (SGO-IMP-001).

### SGO-BIND-002 — Imports
Admitted: the canonical path `"sgo"` and user packages of the same module. Import aliases, dot imports, and blank imports are denied (D15). An unused import is a compile error.

### SGO-BIND-003 — Declarations
Package level: `const` and `func` declarations only. Package-level `var` and `func init` are denied (D07). Function level: `const`, `var x T = e`, `var x = e`, `x := e`, and multi-name forms with matching arity. A local variable must be initialized: `var x T` without an initializer is denied (R3), which removes the zero values of function and capability types.

### SGO-BIND-004 — Scope
Lexical block scoping as in the adapted Go rules, with these S-Go rules: declaring a name that is already declared in an enclosing block of the same function (including parameters and results), or that redefines a predeclared identifier (`len`, `true`, `error`, …), is a compile error (R3); an unused local variable is a compile error; the nursery binder `n` is block-scoped and immutable (SGO-NUR-001).

### SGO-BIND-005 — Exported names
An identifier starting with an upper-case letter is exported from a user package. Only `const` and `func` can be exported.

Tests: X02, S02, S03, S09.

## 3. Types — SPEC-TYPE

### SGO-TYPE-001 — Admitted types
`bool`; `int` (exactly 64-bit two's-complement signed); `string` (immutable byte sequence); function types whose parameter and result types are admitted; `error` (SGO-TYPE-006); `*sgo.Context`, `*sgo.Nursery`, `*sgo.Writer` (SGO-TYPE-005). No other type, type literal, or type declaration is admitted (D04–D06).

### SGO-TYPE-002 — Identity and assignability
Types are identical only if they are the same predeclared or opaque type, or function types with identical parameter and result lists. A value is assignable only to an identical type, except that an untyped constant is assignable to `int`, `bool`, or `string` when representable (SGO-VAL-004), and the predeclared `nil` is assignable only to `error`.

### SGO-TYPE-003 — Function values
Functions are first-class values. Function values have no equality, no `nil` value, and cannot be compared. Variadic functions are denied; any number of results is allowed (R3).

### SGO-TYPE-004 — Sendability
A type is *sendable* if its values can be observed by more than one task without a data race. In subset 0 every admitted type is sendable: `bool`, `int`, `string`, and `error` values are immutable; capability handles are internally synchronized; function values are sendable by SGO-MEM-002. The rule exists so later subsets can add non-sendable types (D01).

### SGO-TYPE-005 — Opaque capability types
`sgo.Context`, `sgo.Nursery`, and `sgo.Writer` can be used only through pointer types and the methods of §0.2. User code cannot construct, dereference, compare, convert, or write `nil` for them. Values come only from the runtime: `root` (SGO-PROC-001), the nursery binder (SGO-NUR-001), the `Spawn` callback parameter, and `Context.Stdout`.

### SGO-TYPE-006 — Sealed `error`
`error` values come only from `nil` and `sgo.NewError(msg)`, or from `sgo` functions. Admitted operations: `err == nil`, `err != nil`, and `err.Error() string`. Comparing two non-nil errors, user implementations of `error`, and wrapping are denied in subset 0.

Tests: S06, S10, S16, X11.

## 4. Values and evaluation — SPEC-VALUE

### SGO-VAL-001 — Integer arithmetic
Operators on `int`: `+ - * / %`, unary `-`, `& | ^ &^`, `<< >>`, and comparisons. Any result outside the 64-bit signed range is a checked runtime failure of kind `IntegerOverflow` (D02). This includes `MinInt / -1` and `-MinInt`. Division or remainder by zero is `DivideByZero`. A shift count outside `0..63` is `ShiftRange`. `/` truncates toward zero and `%` has the sign of the dividend. Checked failures unwind like `panic` (SGO-DEF-003).

### SGO-VAL-002 — Strings
Operators: `+` (concatenation), `== != < <= > >=` (bytewise lexicographic), and `len(s) int`. A concatenation whose length would exceed the implementation limit is resource exhaustion (SGO-ERR-003). Indexing, slicing, conversion, and `range` over strings are denied.

### SGO-VAL-003 — Booleans
`&& ||` short-circuit, `!`, `== !=`.

### SGO-VAL-004 — Constants
Constant expressions are evaluated exactly, at arbitrary precision, at compile time. Converting an untyped constant to `int` when it is not representable is a compile error. Constant division by zero is a compile error. `iota` is denied in subset 0 (R3).

### SGO-VAL-005 — Evaluation order
Because subset 0 has no pointers or shared mutable state, the order is fully specified and strict. Operands, call arguments, and the operands of a multi-value assignment are evaluated left to right, depth first. The function value of a call is evaluated before its arguments. In an assignment, all right-hand operands are evaluated before any variable is assigned. `&&` and `||` evaluate their right operand only when needed.

Tests: S16, X05.

## 5. Memory and data races — SPEC-MEM

### SGO-MEM-001 — Variables
Variables are local to a function invocation. There are no pointers, no package-level variables (D07), and no aliasing except through closure capture.

### SGO-MEM-002 — Captures are effectively final (D01)
A local variable captured by **any** function literal must be *effectively final*: it is never assigned after its declaration, by the declaring function or any function literal. Violations are compile errors (`SGO_E_CAPTURE_NOT_FINAL`). A function literal therefore closes over an immutable environment, so every function value is sendable (SGO-TYPE-004). This rule applies to all closures, not only `Spawn` callbacks, because a closure value can reach another task through capture or parameters.

### SGO-MEM-003 — No data races
Given SGO-MEM-001/002 and SGO-TYPE-004, no two tasks can access the same mutable memory location. Data races are statically impossible in subset 0. Tasks communicate only through child results (SGO-SPAWN-004) and capability effects (SGO-PROC-003).

### SGO-MEM-004 — Happens-before
- Evaluating a `Spawn` call (receiver and callback) happens-before the child callback starts (plan OWN-2).
- A child's complete exit, including its defers, happens-before the nursery statement completes (OWN-4).
- `n.Cancel()` happens-before the cancellation is observed by a `Checkpoint` or `Sleep` that returns a cancellation error.
- `WriteString` calls on one `Writer` are linearized (SGO-PROC-003).

Tests: X11, S16, M-cases of W01.

## 6. Control flow — SPEC-CONTROL

### SGO-CTL-001 — Statements
Admitted: blocks; `if`/`else if`/`else` with an optional init statement; `for` in three-clause, condition-only, and infinite forms; expression `switch` on `int`, `string`, or `bool` with an optional init statement, `default`, and no `fallthrough`; `return`; `break` and `continue`, labeled or unlabeled; labeled statements; expression statements consisting of calls; assignments `=` and `:=`; `x += e` and similar operator assignments for admitted operators; `defer` (SGO-DEF-001); `nursery` (SGO-NUR-001). Denied: `goto`, `fallthrough`, `select`, type switches, `for range`. `x++` and `x--` are admitted as statements meaning `x += 1` / `x -= 1`, with the overflow check of SGO-VAL-001 (R3). An expression statement whose call returns an `error` is a compile error (`SGO_E_DISCARDED_ERROR`); the explicit form `_ = f()` is required (R4).

### SGO-CTL-002 — Switch
Case expressions are evaluated top to bottom, left to right, and each is compared with `==` until one matches. Duplicate constant cases are a compile error. At most one `default`.

### SGO-CTL-003 — Terminating statements
A function with results must end in a terminating statement (adapted Go rule); otherwise it is a compile error. A statement that follows a terminating statement in the same block is a compile error (`SGO_E_UNREACHABLE`, R3).

### SGO-CTL-004 — Exits crossing nurseries
`return`, `break`, and `continue` that leave one or more nursery statements behave as in plan §4.4: return operands are evaluated once first, then the crossed nurseries are joined inside-out, each exactly once, then control transfers. A normal exit joins without cancelling. A `break`/`continue` that stays inside a nursery does not join it. Each iteration of a loop that contains a nursery creates a distinct scope identity.

Tests: X04–X08, X12.

## 7. Defer, panic, and failures — SPEC-DEFER

### SGO-DEF-001 — `defer`
`defer f(args)` where `f` is a named function or a function value. The function value and the arguments are evaluated when the `defer` statement executes. Deferred calls are function-scoped and run LIFO when the function returns or unwinds (D12). When a frame that contains active nursery statements unwinds, the nursery barriers run before that frame's defers (plan §4.5). Deferring a call that returns an `error` is a compile error (`SGO_E_DISCARDED_ERROR`); wrap it as `defer func() { _ = f() }()` to discard it explicitly (R4).

### SGO-DEF-002 — `panic`
`panic(msg)` where `msg` has type `string` (D13). It starts unwinding the current task with failure kind `Panic` and the message as payload. `recover` is denied in subset 0.

### SGO-DEF-003 — Checked runtime failures
`IntegerOverflow`, `DivideByZero`, `ShiftRange`, `SpawnContractViolation`, `InvalidArgument`, and `MissingCapability` unwind exactly like `panic`, with a distinct failure kind and a source position.

### SGO-DEF-004 — Panic during unwinding
If a deferred call panics while the task is unwinding, the first failure stays primary. Later failures are recorded in the diagnostic chain in occurrence order. Unwinding continues with the remaining deferred calls.

### SGO-DEF-005 — Unwinding to the task boundary
A failure that unwinds out of a child callback terminates that child with outcome `Failed` (SGO-SPAWN-004). A failure that unwinds out of `main` terminates the process with exit status 2 (SGO-PROC-002).

Tests: X09, X12, S08.

## 8. Nurseries — SPEC-NURSERY

### SGO-NUR-001 — Syntax and binding
`nursery (n) Block` is a statement. It declares `n` of type `*sgo.Nursery`, scoped to `Block`. `n` cannot be assigned, but may be passed as an argument and captured, under SGO-MEM-002. The anonymous form `nursery Block` is denied (D11).

### SGO-NUR-002 — Lifecycle
States: `OPEN → JOINING → FINALIZING → CLOSED`, and `OPEN|JOINING → CANCELLING → FINALIZING → CLOSED` (plan §4.3). In subset 0, FINALIZING has no user finalizers (`n.Defer` is denied).

### SGO-NUR-003 — Join
When `Block` completes, the owner waits in JOINING until every child has terminated. Children that are already live may still spawn into `n` during JOINING. The nursery reaches CLOSED only with zero live children (OWN-3).

### SGO-NUR-004 — Outcome
After CLOSED, the nursery statement completes with exactly one outcome, chosen in this order:
1. If the owner body itself unwound (panic or checked failure), that failure stays primary and keeps unwinding.
2. Otherwise, if any child ended `Failed`, the nursery statement unwinds with the failure of the earliest-created failed child (stable creation ID, plan §4.5). All other child failures are recorded in diagnostics.
3. Otherwise, if any child ended `Errored`, the nursery statement unwinds with failure kind `ChildError`, carrying the earliest-created child's error.
4. Otherwise, it completes normally. Children that ended `Cancelled` do not make the nursery fail.

A `ChildError` unwind reaching `main` maps to exit status 1 (SGO-PROC-002, R6). That refines D08, which only names a returned error. Subset 0 has no way to handle a child error locally (no `recover`); a later API wrapper may return it as a value.

### SGO-NUR-005 — Nesting
A nursery may be nested in another nursery's body or inside a child. Inner nurseries close before the enclosing block continues (lexical structure). An inner nursery's cancellation does not cancel its parent.

Tests: X04, O-cases, N-cases of W02/W03, S07.

## 9. Spawning — SPEC-SPAWN

### SGO-SPAWN-001 — Call form
`n.Spawn(f)` where `f` has type `func(*sgo.Context) error`. The receiver and argument are evaluated once, left to right, in the caller. `Spawn` has no result and does not wait for the child.

### SGO-SPAWN-002 — Admission checks
Before registering the child, the runtime checks that `n` is in state OPEN or JOINING, that the calling task is the owner of `n` or a live child of `n` (or a descendant executing in a nursery nested inside `n`), and that `n` is the caller's current innermost structured scope (plan §4.2). If any check fails, no child is created and the caller fails with `SpawnContractViolation` (SGO-DEF-003). If `n` is CANCELLING, no child is created and `Spawn` returns normally; the caller observes cancellation at its next `Checkpoint` or `Sleep` (R5). This is the plan §4.2 rule "during cancellation, admission creates no child": a child admitted then would only be cancelled immediately, so not creating it is observably equivalent and needs no new failure kind.

### SGO-SPAWN-003 — Registration and start
The child is registered in `n` before it becomes runnable (OWN-2). It runs `f(task)` with a fresh `*sgo.Context`. That context has no capabilities (`task.Stdout()` fails, SGO-PROC-003); its cancellation is linked to `n` and to all of `n`'s ancestors.

### SGO-SPAWN-004 — Child outcomes
| Callback ending | Outcome |
|---|---|
| returns `nil` | `Succeeded` |
| returns an error `e` where `sgo.IsCancelled(e)` | `Cancelled` |
| returns any other non-nil error | `Errored`: `n` is cancelled (SGO-CANCEL-001) |
| unwinds with a failure | `Failed`: `n` is cancelled |

The cancellation for `Errored` and `Failed` happens when the outcome is published, before the join completes. No outcome is discarded (plan §4.2).

### SGO-SPAWN-005 — Bare `go` is rejected
`go` statements are rejected everywhere, including unreachable code and dependencies, with `SGO_E_UNSTRUCTURED_SPAWN` (OWN-5).

Tests: S05, S06, S07, S08, O-cases.

## 10. Cancellation — SPEC-CANCEL

### SGO-CANCEL-001 — Request
`n.Cancel()` moves `n` from OPEN or JOINING to CANCELLING. A second call has no effect: cancellation is sticky and idempotent (CAN-1). Cancellation propagates to every task whose context is linked to `n`, including the owner while it executes inside `n`'s body, and to nurseries nested inside it. Child failure (SGO-SPAWN-004) calls the same transition. Only tasks that may spawn into `n` may call `Cancel` (same checks as SGO-SPAWN-002).

### SGO-CANCEL-002 — Observation
`task.Checkpoint()` returns `nil` if no linked scope is cancelled; otherwise it returns a cancellation error. A cancellation error stays observable: every later `Checkpoint` or `Sleep` returns it again. Code that never calls `Checkpoint` or `Sleep` is never interrupted by cancellation (cooperative model, plan §6.1).

### SGO-CANCEL-003 — Cancellation errors
`sgo.IsCancelled(err)` is `true` exactly for errors produced by SGO-CANCEL-002 and SGO-WAIT-001. `sgo.NewError` never produces one. The error message names the cancellation cause: an explicit `Cancel`, a sibling failure (with the sibling's creation ID), or ancestor cancellation.

### SGO-CANCEL-004 — Owner and ancestry
The owner's context observes cancellation of every nursery it is currently executing inside, as well as its own ancestry (plan §6.1). An inner nursery's cancellation never cancels its enclosing nurseries.

Tests: C-cases of W04, S07.

## 11. Drain and non-cooperation — SPEC-DRAIN

### SGO-DRAIN-001 — No false success
A nursery never completes while a registered child is live (CAN-5). A child that ignores cancellation keeps the nursery in CANCELLING indefinitely unless a drain budget applies.

### SGO-DRAIN-002 — Drain budget
The program may be built with a process-wide drain budget, `sgo build -drain-budget=<ms>`, recorded in the binary's build metadata and in run manifests; it is measured in milliseconds of monotonic time from a nursery's entry to CANCELLING. The default is none (wait indefinitely; plan §6.3: no surprise production deadline). No environment variable or runtime flag changes it (R7). A per-nursery API is later work. When the budget is exceeded, the runtime writes the scope tree and pending task IDs to the diagnostic stream and terminates the process with exit status 3 (plan §6.4). It never removes a live task and continues.

### SGO-DRAIN-003 — Host watchdog
Test runners also enforce an external wall-clock watchdog (plan §6.3). A watchdog kill is recorded as a runner result, distinct from exit status 3.

Tests: first-slice non-cooperative child scenario (plan §15.1), C-cases of W04.

## 12. Waiting and time — SPEC-WAIT, SPEC-TIMER

### SGO-WAIT-001 — `Sleep`
`task.Sleep(ms)` with `ms >= 0` blocks the task for at least `ms` milliseconds of monotonic time and then returns `nil`. If a linked scope is cancelled before or during the wait, it returns a cancellation error promptly and the wait registration is removed exactly once (CAN-2, plan §6.2). A negative `ms` is a checked runtime failure of kind `InvalidArgument`, which unwinds like `panic` (SGO-DEF-003, R8).

### SGO-WAIT-002 — Join is a managed wait
The owner waiting in JOINING or CANCELLING is a managed wait under the same one-winner registration protocol.

### SGO-TIMER-001 — Clock
Subset 0 exposes no clock reads. Sleep uses the production monotonic clock. Lab virtual time is out of scope for subset 0 (W08).

Tests: C-cases of W04.

## 13. Process, entry, and output — SPEC-PROC, SPEC-CAP

### SGO-PROC-001 — Entry point
Package `main` declares exactly `func main(root *sgo.Context) error` (D08). Any other signature, a missing `main`, or a Go-style `func main()` is a compile error (`SGO_E_MAIN_SIGNATURE`). `root` holds the stdout capability and the root cancellation. Subset 0 has no external way to cancel `root`; OS signals are denied and their mapping to root cancellation is later work.

### SGO-PROC-002 — Exit status
| Outcome | Status |
|---|---|
| `main` returns `nil` | 0 |
| `main` returns a non-nil error, or a `ChildError` unwinds out of `main` | 1 |
| A `Panic` or checked runtime failure unwinds out of `main` | 2 |
| Drain budget exceeded (SGO-DRAIN-002) | 3 |
| Resource exhaustion (SGO-ERR-003) | 4 |

Statuses 1–4 write a diagnostic record to stderr: failure kind, message, source position, and task creation ID. Its exact format is specified by SGO-ERR-002. Before returning status 0 or 1 from `main`, all output written through `Writer` is flushed.

### SGO-PROC-003 — Stdout capability (D09)
`root.Stdout()` returns the process's single `*sgo.Writer`. Called on any context other than `root`, it is a checked failure of kind `MissingCapability` (CAP-1: children start with no capabilities). The handle may be passed to children explicitly or by capture; holding the handle is the authority. `WriteString(s)` writes all of `s` atomically with respect to other `WriteString` calls and returns `nil`, or an error if the underlying stream fails (bytes written before the failure are reported in the message; CAN-3). `WriteString` is not a cancellation point. `print` and `println` are denied.

### SGO-PROC-004 — Arguments and environment
Not accessible in subset 0.

Tests: S06, S13, golden harness (SGO-TEST-001).

## 14. Packages and contracts — SPEC-IMPORT

### SGO-IMP-001 — User packages
A module contains package `main` and any number of user packages. Import cycles are compile errors. Every package is compiled under subset 0; there is no other mode.

### SGO-IMP-002 — Contract identity
Each compiled package records the spec version, the admission digest, and the digests of its dependencies' exported contracts in its export data. Linking a package compiled against a different spec or admission digest, or against a changed dependency contract, is an error (`SGO_E_STALE_CONTRACT`; plan §1.3). Stock Go objects are rejected.

Tests: S03, S09, S15.

## 15. Diagnostics and resources — SPEC-ERRORS

### SGO-ERR-001 — Compile-time diagnostics
Every rejection has a stable code, source position, and the clause or decision ID that denies it. Initial codes: `SGO_E_UNADMITTED` (construct denied in subset 0; names the construct), `SGO_E_UNSTRUCTURED_SPAWN`, `SGO_E_MAIN_SIGNATURE`, `SGO_E_CAPTURE_NOT_FINAL`, `SGO_E_CONST_OVERFLOW`, `SGO_E_STALE_CONTRACT`, `SGO_E_UNUSED`, plus type errors under SGO-TYPE. The compiler never downgrades a diagnostic to a warning.

### SGO-ERR-002 — Runtime diagnostic record
A single line of JSON on stderr per terminal failure (R9): `{"sgo_diagnostic":1,"kind":K,"message":M,"position":"file:line:col","task_id":N,"scope_id":N|null,"chain":[{"kind":K,"message":M,"position":P}...]}`, keys in that order, UTF-8, no trailing spaces. `kind` is one of the failure kinds of §7, `ChildError`, `DrainBudgetExceeded`, or `ResourceExhausted`; `task_id` is the stable creation ID (root = 0). Canonical fields exclude addresses and wall-clock times (plan §9.5).

### SGO-ERR-003 — Resource exhaustion (D16)
Stack overflow, heap exhaustion, and implementation limits (string length, task count) terminate the process with status 4 and a diagnostic record. They are not catchable and do not run deferred calls. Limits are recorded in the run manifest.

## 16. Infrastructure families (TCB, build, tooling, stacks, tests)

These rows are partly in subset 0 because the first slice needs them; their clauses are mainly W00/W01 deliverables.

- **SPEC-TCB:** every runtime primitive used by subset 0 (task creation, park/unpark, timers, stdout write, process exit) gets an individual contract and an audited caller list (plan §8.1). No package-name privilege.
- **SPEC-BUILD:** host-built compiler, stage map, and target identity per plan §10.2. Targets per D22: darwin/arm64 first; linux/amd64 (real target) and linux/arm64 first-class. No clause may assume a darwin-only mechanism.
- **SPEC-TOOL:** the S-Go driver is named **`sgo`** (user decision 2026-10-30), with `sgo build` and `sgo run`; subcommand set is drafted in W01. It accepts only subset-0 source and has no flag that widens admission.
- **SPEC-STACK:** inherited growable stacks, with GC-1/GC-2 obligations (plan §5.3). Fixed stacks are W11.
- **SPEC-TEST — SGO-TEST-001:** a conformance case is a directory with source files and a golden expectation: exit status, exact stdout bytes, and, for rejection cases, the diagnostic code and position. A host-side runner (external tooling) executes cases with a watchdog (D10).

## 17. Denied in subset 0

Each produces `SGO_E_UNADMITTED` naming the construct: floats, runes and other integer widths; pointers, arrays, slices, maps, structs; `type` declarations, methods, interfaces, generics; `chan`, `select`, `go`, `goto`, `fallthrough`, `range`; package-level `var`, `init`; `recover`; anonymous `nursery`; `n.Defer`; conversions; imports other than `sgo` and same-module packages; every standard-library package; `unsafe`, cgo, assembly, `//go:` directives; `print`/`println`. Families wholly denied: SPEC-CHAN, SPEC-PERMIT, SPEC-READY, SPEC-RANDOM, SPEC-FILE, SPEC-NET, SPEC-LAB, SPEC-FLOAT, SPEC-COMPOSITE, SPEC-USERTYPE, SPEC-IFACE, SPEC-GENERIC, SPEC-REFLECT, SPEC-NATIVE, SPEC-SYNC.

## 18. Drafting resolutions (W01 step 1, 2026-10-30)

Agent defaults under the approved decisions; the user may override any of them. None changes an approved decision.

| # | Item | Resolution | Reason |
|---|---|---|---|
| R1 | API names | Keep `n.Cancel()`, `task.Checkpoint() error`, `task.Sleep(ms int) error`, `sgo.IsCancelled(err) bool`. | Minimal surface for the first slice; consistent with `n.Spawn`. |
| R2 | File extension, escapes | `.sgo` only; escapes `\n \t \r \\ \" \xHH`; no `\u`/`\U`/octal. | A distinct extension keeps Go tooling from treating S-Go as Go and vice versa. Unicode escapes need rune rules, which subset 0 lacks. |
| R3 | Small language rules | Deny `var x T` without initializer, shadowing of enclosing locals and predeclared names, `iota`, variadics, unreachable statements. Admit `x++`/`x--` (checked) and any number of results. | Each removes an unspecified or confusing corner (zero values of capabilities, shadowed `err`, dead code) at little cost. |
| R4 | Discarded `error` results | Compile error everywhere, including `defer`; `_ = f()` is the explicit discard. | Plan §4.2: no outcome is discarded silently. |
| R5 | `Spawn` on a CANCELLING nursery | No child, no failure; cancellation is observed at the next `Checkpoint`/`Sleep`. | Matches plan §4.2; observably equivalent to admitting and immediately cancelling. |
| R6 | `ChildError` at `main` | Exit status 1. | It is an error outcome, not a panic. |
| R7 | Drain budget | Build-time `sgo build -drain-budget=<ms>`, default none, recorded in build metadata. | Explicit application setting (plan §6.3); no runtime override that could weaken it. |
| R8 | Negative `Sleep` | Checked failure `InvalidArgument`. | Specified failure instead of an arbitrary choice. |
| R9 | Diagnostic record | One JSON line, fixed key order (SGO-ERR-002). | Machine-checkable by the conformance runner; no addresses or times. |
| — | Driver name | `sgo` (user decision). | — |
