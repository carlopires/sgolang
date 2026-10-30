# S-Go: A Paradigm Shift in Systems Concurrency
## Conceptual Whitepaper, Rationale, and References

---

## 1. The Problem: Go's Concurrency is the `goto` of our Era

In 1968, Edsger W. Dijkstra published "Go To Statement Considered Harmful," arguing that unconstrained jumps make it impossible to reason about program state. Today, unconstrained concurrency is the modern equivalent.

Standard Go relies heavily on the naked `go f()` statement. This design has fundamental theoretical flaws for high-reliability systems:
1. **Violates Lexical Scoping:** A function can return while its background goroutines continue running, breaking the fundamental contract of procedural programming (that a function's side effects are contained within its lifetime).
2. **Silent Leaks:** Orphaned goroutines that block on I/O or channels leak memory and stack frames silently. The runtime has no concept of "ownership."
3. **Cooperative-Only Cancellation:** `ctx.Done()` is a polite suggestion. If a deeply nested library ignores the context, it cannot be interrupted.
4. **Non-Deterministic Testing:** Go's M:N scheduler introduces arbitrary interleaving. Concurrency bugs (heisenbugs) are notoriously difficult to reproduce in CI/CD environments.

---

## 2. Core Inspirations & References

S-Go does not invent structured concurrency from scratch. It is a synthesis of three major theoretical breakthroughs in concurrency design over the last decade:

### A. Trio & "Notes on Structured Concurrency" (Nathaniel J. Smith)
**Reference:** *[https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/](https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/)*
- **Idea Borrowed:** The **Nursery** (or Task Group). Concurrency must be strictly bound to a lexical block. You cannot exit a block until all concurrent tasks spawned within it have terminated.

### B. `asupersync` (Rust Async Runtime)
**Reference:** *[https://github.com/Dicklesworthstone/asupersync](https://github.com/Dicklesworthstone/asupersync)*
- **Idea Borrowed:** **Cancel-Correctness and Explicit Capabilities**. In `asupersync`, operations explicitly declare their capability context (`&Cx`). Cancellation is not a boolean flag, but a formal protocol: `Request -> Drain -> Finalize`. Operations cannot be left in a half-finished state.

### C. BlenD2 (Discrete-Event Boundary Verification)
- **Idea Borrowed:** **Deterministic Boundaries and Two-Phase Contracts**. Concurrency should be treated as a discrete-event simulation. By forcing all inter-task communication through a `Reserve -> Commit` boundary, and using a seeded PRNG for scheduling, we guarantee that any race condition can be reproduced with 100% fidelity.

---

## 3. The Four Pillars of S-Go

### Pillar 1: Zero-Orphan Lexical Lifetimes
S-Go introduces the `nursery` control-flow block. When execution reaches the end of a `nursery`, the parent task is structurally suspended until all children terminate.
If a child crashes (panics), the nursery catches it, cancels all sibling tasks, waits for them to drain, and then re-raises the panic in the parent. **Control flow is mathematically guaranteed to rejoin.**

### Pillar 2: Capability-Gated Authority
In standard Go, any function can call `http.Get()` or `os.Open()`, using "ambient authority." This makes sandboxing impossible.
In S-Go, standard library I/O requires a capability context (`*runtime.Context`). A spawned task inherits capabilities from its nursery. If you don't pass the network capability down the call stack, the function physically cannot perform network I/O. This provides military-grade dependency sandboxing.

### Pillar 3: Cancel-Correctness & Bounded Drains
When a nursery is cancelled, it enters a `DRAINING` state.
Tasks are not immediately killed (which corrupts memory), but they are given a **Bounded Drain Budget** (e.g., 500ms of virtual time). During this time, they can close files and rollback transactions. If they exceed the budget, the runtime halts them and emits a `ContractViolationDiagnostic`. There are no infinite hangs.

### Pillar 4: The Deterministic Lab Engine
By compiling with `GOLABSEED=12345`, the S-Go runtime switches off multi-threading. It becomes a single-threaded Discrete Event Simulator (DES).
- The OS clock is mocked. Time only advances when all tasks are blocked.
- Preemption is disabled. Context switches happen exactly and only at I/O or channel boundaries.
- Scheduling decisions are driven by a PCG32 random number generator initialized by the seed.
**Result:** If a test fails in CI, you can download the seed, run it locally, and observe the exact same bug, byte-for-byte, instruction-for-instruction.

---

## 4. Why Fork? Why not a Library?

The Go community has tried to fix this in userland (e.g., `golang.org/x/sync/errgroup`). Libraries fail because:
1. **The Contagion Problem:** If you use an `errgroup`, but import a 3rd-party library that calls `go f()` internally, your application still leaks goroutines.
2. **The Stack Problem:** Go's dynamic stack-splitting (morestack) requires runtime support that prevents implementing true zero-allocation scope boundaries in user space.
3. **The I/O Problem:** You cannot forcefully interrupt a goroutine blocked on a standard `net.Conn.Read()` without runtime scheduler integration.

S-Go is a hard fork because structured concurrency must be enforced by the compiler (syntax), the GC (stack allocation), and the lowest levels of the runtime scheduler.