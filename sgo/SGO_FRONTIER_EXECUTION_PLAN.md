# S-Go: Frontier-LLM Execution Plan

**Plan version:** 2.0 · **Prepared:** 2026-10-02 · **Language:** en-US  
**Input:** the seven Markdown documents in `sgo.zip`  
**Delivery:** an independent, specification-defined language built from a Go compiler/runtime fork; no Go compatibility path

## 0. Read this before making changes

**2026-10-30 amendment:** `SGO_DECISIONS.md` records user-approved decisions D01–D22. They are approved amendments: they take precedence over this plan where they differ, and they define subset 0, the closed first-slice language. `SGO_SPEC_SUBSET0.md` is its draft specification. The implementation repository is `github.com/carlopires/sgolang`.

This revision supersedes handoff v1.0. Read `SGO_LANGUAGE_CONSTITUTION.md` first: it records the user's non-negotiable no-compatibility and specification-admission requirements. It has precedence over this plan and the historical input. This plan supplies execution defaults; the implementing agent must freeze precise, versioned contracts before enabling a language feature. Preserve originals as historical input; do not apply their code mechanically.

**Correction:** v1.0 introduced compatibility profiles and inherited spawning for existing dependencies. Those were the assistant's design additions, not user requirements. They are removed. The explicit nursery-handle spawn surface below is a new recommended design default, not a claim that the user personally selected its exact spelling.

The supplied archive contains a conceptual whitepaper, a master blueprint, and five phase documents. It does **not** contain a Go checkout, implemented compiler changes, a runtime, a version pin, tests, or a build system. This plan is based on inspecting every supplied document and checking relevant upstream documentation/source pages. No S-Go compiler has been built or tested during preparation of this plan.

Use these evidence labels throughout implementation:

- **Observed:** present in the supplied archive or demonstrated by a recorded command.
- **Upstream:** behavior documented in the pinned Go sources or references in §16.
- **Decision:** an S-Go design choice made here, subject to a documented, evidence-backed amendment.
- **Hypothesis:** a claim that needs an experiment, not a fact or an acceptance result.

The work is complete only when an actual fork builds, executes the supported language, passes its conformance gates, and states its unsupported cases accurately. A parser demo, a callback-based library, a seeded random queue, or a collection of interfaces is not completion.

### Execution contract for the implementing agent

Read the constitution, this plan, `SGO_SPEC_ADMISSION.json`, `SGO_TASKS.json`, and `SGO_TEST_MATRIX.json`. Inspect the actual working tree before assuming the archive still describes it. Preserve user changes. Execute dependency-ready work packages in order. Each package must produce implementation, focused tests, and evidence, not another planning document alone.

Use small, coherent changes. Record actual source locations before editing compiler/runtime internals. Keep the separately built host implementation tools operational without adding any Go-language acceptance mode to the S-Go target compiler. Never turn a missing capability into a successful stub, weaken a test to match a bug, or label unrun tests as passing. Update the task ledger and handoff checkpoint before ending a session.

---

## 1. The product and its boundaries

### 1.1 Intended outcome

S-Go uses Go compiler/runtime source as implementation material, not as a compatibility contract. Its own specification defines:

1. **Structured task ownership:** explicit nursery scopes; transitive user-task ownership; join at every scope exit; failure propagation; no successful scope closure with live children.
2. **Cancel-correct effects:** cancellation-aware waiting, cleanup, and reserve/commit/abort boundaries with defined linearization points.
3. **Capability-mediated effects:** explicit authority for supported external operations, checked through the dependency graph rather than merely suggested by an API signature.
4. **A deterministic laboratory:** controlled scheduling, virtual time, simulated effects, replay traces, and bounded schedule exploration over a documented subset.
5. **A measured stack experiment:** investigate non-moving, virtually reserved stacks independently of the correctness features.

Keep the original ambition. Separate claims so that progress on one pillar cannot be mistaken for proof of the others.

### 1.2 Defaults to execute without another planning round

| Decision | Default |
|---|---|
| Upstream base | `go1.27.3` (D21; tag observed at commit `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` on 2026-10-30); W00 verifies and locks the commit and source/bootstrap checksums. Reference URLs in §16 still name `go1.27.1`; use the pinned tag's counterparts. |
| Why this tag | Carried forward as a source-selection default from v1.0, not a semantic dependency. W00 must verify availability and provenance and lock the exact source. No moving branch or fallback toolchain is permitted. [R1] |
| Execution targets (D22) | darwin/arm64 is the first development target (native host). linux/amd64 is the real production target and every decision must hold for it; linux/arm64 is first-class. Target-dependent work-package gates require execution evidence on all three. Initially use the inherited growable-stack machinery subject to S-Go runtime contracts. |
| Other targets | None admitted. Additional targets need their own TCB contracts and native execution evidence, not cross-compilation alone. |
| Core implementation language | Go and the existing Go assembly where necessary. Shell scripts for build orchestration. |
| Source layout | Reuse upstream layout where useful; remove/isolate unadmitted target code. Add project documents/tests/tools under `sgo/`; public fork packages under `src/sgo/`. |
| Root module | Do not run `go mod init` at the toolchain root or replace upstream module/workspace files. A standalone reference model may have a separate module under `sgo/model/`. |
| Runtime scope storage | Heap-backed, GC-reachable records first. No published pointers into movable parent stacks. |
| Cancellation guarantee | Correct outcomes at supported boundaries; bounded diagnostics/fail-stop escalation for non-cooperation, not arbitrary safe goroutine destruction. |
| Lab guarantee | Reproducible canonical managed-event traces under an identical manifest and controlled inputs. Not instruction-by-instruction replay of arbitrary Go. |
| Fixed stacks | A separate experimental track after semantic correctness. Adoption requires evidence. |

An existing implemented branch may justify another maintained Go release. In that case W00 must record the reason, exact pin, and refreshed source map before any changes. Do not silently rebase while implementing a work package.

### 1.3 One language; production and lab execution

There is exactly one public S-Go language. There is no `compat` mode, opt-in `strict` dialect, automatic legacy fallback, permissive importer, raw channel tier, or safety-disabling environment variable. Source that is valid Go is not automatically valid S-Go.

**Production execution:** every admitted construct, task, dependency, effect, and target-runtime entry point must satisfy the current S-Go specification and its enforcement policy. Unspecified or unimplemented constructs and imports are rejected, including constructs inherited from the fork's parser.

**Lab execution:** the same language and safety rules execute against controlled scheduling, virtual time, and versioned simulated adapters. Lab admission additionally checks replay coverage. Production nondeterminism must itself have a specified set of allowed outcomes; lab chooses/records outcomes within that contract. Lab is not a stricter safety dialect and production is not a fallback for failed lab checks.

Spec version/digest, ABI, admitted feature-set digest, dependency contracts, adapter identities, and execution configuration must be in export/link metadata and build-cache keys. Stock Go objects, uncontracted packages, stale contracts, and mismatched ABI/spec identities must fail before execution. A seed cannot upgrade an uninstrumented binary into a replayable one. No setting disables ownership, capability checks, or contract admission.

A candidate compiler may implement a small specified subset. Its manifest must describe that subset exactly; everything else remains rejected. `experimental` describes implementation maturity, not permission to execute an unspecified path.

### 1.4 Specification-first admission gate

`SGO_SPEC_ADMISSION.json` starts as a required inventory, not a completed language specification or an allowlist. W01 must expand it into a complete inventory of all parser/type-system constructs, primitive operations, builtins, package exports, compiler intrinsics, and target-runtime/effect boundaries that the first subset can reach. Inventory completeness must be checked against actual compiler dispatch tables and exported symbols, not just prose.

For each admitted entry record: stable clause ID/version, grammar and typing/preconditions, value and evaluation semantics, lifecycle/ownership/effects, concurrency and memory-order rules, failure/cancellation/panic/cleanup outcomes, allowed nondeterminism, static checks, runtime checks, positive and negative conformance cases, implementation coverage, and supported execution configurations. A non-applicable field needs an explicit rationale. A TODO, external link alone, prose annotation on an unchecked dependency, or “same as Go” is not a valid contract.

Specific upstream clauses may be deliberately adopted with an exact frozen revision, bounded scope, recorded content digest, all normative dependencies closed, conflict resolution against S-Go, and conformance tests. This is explicit specification reuse, not a default inheritance rule. There must be no blanket “Go semantics apply unless changed” clause.

The compiler/linker must reject missing, draft, conflicting, unsupported, or stale specification entries with source and clause diagnostics. Every indirect call, closure, interface target, generic instantiation, reflected operation, and generated source must remain inside the admitted set. Use a checked function/effect summary discipline for separate compilation; if its rules are not complete for a construct, reject that construct rather than trusting a declaration. Full effect/capability detail follows in W05, but default denial and transitive contract checking begin in W01/W03.

This is **not** a claim that a compiler proves arbitrary programs bug-free or terminating. It is a requirement that accepted operations have defined semantics and an enforceable legality contract. Runtime-dependent conditions may have specified checked failures; they may not become unspecified execution. W01 must explicitly settle integer/float edge cases, nil/bounds errors, aliasing/shared-memory rules, data races, atomics, map ordering, failure, and resource exhaustion before enabling the associated constructs. “Unspecified implementation behavior” is not an acceptable substitute for bounded, documented nondeterminism.

---

## 2. Audit of the existing proposal

The following corrections are prerequisites, not optional editorial changes.

| Supplied material | Problem | Required replacement |
|---|---|---|
| `SGO_MASTER_BLUEPRINT.md:6–11` | Fixed stacks are made a prerequisite for structured concurrency without evidence. | Bootstrap → semantic model → ownership runtime → language exits → cancellation/effects → channels → lab → optimization. |
| `PHASE1_FIXED_STACKS.md:4–22` | The proposed backend hooks are illustrative/unverified, and removing stack checks affects more than stack growth. Upstream x86 prologue generation has a `stacksplit` path; runtime stack guards also carry preemption requests. [R3, R4] | Keep upstream stacks/checks during the core implementation. Produce a pinned source map before any stack experiment. |
| `PHASE1_FIXED_STACKS.md:29–42` | The example treats `sysAlloc` as a `PROT_NONE` reservation. Its documented contract is immediately usable memory, and the pinned signature differs. [R5] | Use the actual OS/runtime memory-state API. Distinguish reserved virtual space, accessible pages, resident pages, and committed accounting. |
| Same stack example | Mapping only an initial accessible region does not explain how deeper stack accesses become valid. A bottom guard alone also does not solve large-frame guard skipping. | Define the entire accessible/guard layout and growth-or-overflow behavior. Do not invent a signal handler as an incidental fix. |
| `PHASE3_DESUGARING_IR.md:12–38` | A function `defer` is not a block-exit cleanup. The nursery could remain unjoined after its closing brace. [R6] | Native scope-exit lowering plus exceptional-unwind barriers. Cover every outward control-flow edge. |
| Same document | A `VARDEF`-style marker is not a complete GC lifetime, escape, and stack-relocation proof. | Use rooted heap state first. Audit runtime roots, write barriers, object lifetime, and unwind-marker relocation. [R7] |
| `PHASE3_DESUGARING_IR.md:5–61` | The proposed `gen.nurseryStmt` is not a verified patch against the selected compiler. Current construction passes through type checking and Unified IR writer/reader paths. [R2, R8] | Implement the actual compiler pipeline; do not add an unused function in a guessed file. |
| Whitepaper §3, pillar 2 | Passing `*Context` does not by itself prevent a dependency from calling another I/O entry point. | State a threat model and implement transitive enforcement plus runtime/adapter checks. Do not advertise hostile-code isolation without separate containment. |
| Whitepaper §3, pillar 3 | Stopping one arbitrary goroutine after a timeout can leave shared state or locks inconsistent. Virtual time also need not advance while work stays runnable. | Cooperative supported cancellation; step budgets and watchdogs; process-level fail-stop on contract failure. Never resume a program after abandoning a live task. |
| Whitepaper §3, pillar 4 | One seeded queue does not control map iteration, select, timers, system work, I/O, entropy, or all scheduler decisions. | A deterministic-execution contract, nondeterminism inventory, trace/replay protocol, and explicit exclusions. |
| `PHASE5_BLEND2_CHANNELS.md:3–36` | The sample does not specify abort, close races, unbuffered rendezvous, receiver wakeup, selection losers, or permit lifetime. Capacity zero never satisfies its reserve condition. | Model the protocol and its linearization points before changing native channels. |
| Whitepaper §4 | Some statements about Go and libraries are too absolute. Go is not unable to support useful structured-concurrency libraries; the fork's distinguishing aim is stronger transitive language/runtime enforcement. | Rewrite the rationale precisely. Preserve the architectural reason for a fork without relying on exaggerated impossibility claims. |

**Provenance correction (2026-10-30).** The archive names **BlenD2** (whitepaper §2.C, blueprint phase 5, `PHASE5_BLEND2_CHANNELS.md`) and supplies no URL, version, or specification. No concurrency reference by that name exists. The name is most likely a garbled **Bend 2** ([R18]), but Bend 2 is the source of none of the ideas attributed to it: it is a pure, linear-typed, fork/join-parallel language with compiler-enforced proved laws. It has no discrete-event simulation, reserve/commit channel protocol, or seeded scheduler. Those ideas match the asupersync runtime ([R13]): two-phase reserve/commit sends, request → drain → finalize cancellation, and a lab runtime with virtual time, deterministic scheduling, and trace replay. Treat "BlenD2" as a historical mislabel. Attribute reserve/commit and deterministic-lab design inspiration to R13; at most, Bend 2's law/proof gate is an analogue of this plan's specification-admission policy, not of the channel contract. Neither source is normative. Implement from the explicit contract in §7. The `original/` documents keep the old name byte-for-byte as historical input.

---

## 3. Normative invariants

Give these identifiers to assertions, tests, trace checks, and review comments.

| ID | Invariant |
|---|---|
| OWN-1 | Every admitted user task has exactly one owning nursery. A task may also temporarily have a different active nursery for new children. |
| OWN-2 | Child registration becomes visible before the child becomes runnable. Registration and final scope closure are serialized. |
| OWN-3 | Successful closure implies zero live children, no live nested scopes, no owned wait registrations/timers/permits, and completed scope finalizers. |
| OWN-4 | A child's complete exit, including its own defers, happens-before the parent's successful join returns. |
| OWN-5 | Bare `go` is rejected in all S-Go source. Spawn requires an explicit nursery capability and the same checked admission path in application code and dependencies. |
| OWN-6 | Runtime service tasks have explicit lifecycle/authority contracts and sealed creation paths. A package-name exemption must not convert arbitrary dependency tasks into services. |
| CAN-1 | Cancellation is sticky, idempotent, and propagates downward. A copied context cannot reset it. |
| CAN-2 | Each wait registration has exactly one terminal disposition: completion, cancellation, close/error, or a losing selection. |
| CAN-3 | A failed/cancelled operation reports its actual progress. Cancellation after a successful commit cannot retroactively turn that commit into a no-effect result. |
| CAN-4 | Scope cleanup does not erase the primary panic/error, reset the budget, or spawn detached work. |
| CAN-5 | Exceeding a drain contract does not permit the scope to return success while children remain live. |
| CH-1 | Every permit is committed or aborted once; all aliases share one terminal state. |
| CH-2 | For a buffered channel, queued items plus reserved send capacity never exceed capacity. Capacity-zero channels use a separate rendezvous protocol. |
| CH-3 | S-Go channel clauses explicitly define FIFO, close, selection atomicity, and happens-before edges; inherited Go behavior is not the authority. [R9 is reference material only.] |
| CAP-1 | Child authority is no greater than its inherited task ceiling, and explicit handles can only attenuate that authority. |
| CAP-2 | Revoked, expired, cross-domain, or insufficient handles fail before performing the forbidden effect. |
| LAB-1 | All scheduler-relevant user decisions in a supported lab run have a controlled or recorded source. |
| LAB-2 | Replay checks the manifest and each decision's eligible set; divergence is an error, never a fallback to live scheduling. |
| GC-1 | Every live scope/task/permit reference is visible through an audited GC-root path. |
| GC-2 | No scope record, context, permit, or unwind marker relies on a stale pointer into a relocated stack. |
| SPEC-1 | Every admitted construct/export/intrinsic has a frozen, closed S-Go contract and an enforced admission entry. |
| SPEC-2 | Missing or unsupported contracts cause rejection, never legacy execution. |
| SPEC-3 | Imports, indirect calls, generated code, optimizations, and target-runtime boundaries cannot bypass the contract set. |
| SPEC-4 | Production and lab preserve the same ownership, capability, failure, and cleanup rules. |
| SPEC-5 | Host bootstrap/reference tools are external build dependencies, never target-language acceptance modes. |

These invariants do not prove the absence of application data races. A join creates a completion ordering; it does not order concurrent sibling accesses. Keep race testing and production-mode concurrency stress in the validation strategy.

---

## 4. Language semantics to implement

### 4.1 Nursery syntax and binding

The recommended v2 surface is:

```go
nursery (n) {
    // n is an immutable binding to an opaque *sgo.Nursery capability.
}

nursery {
    // Anonymous structured scope. No ambient spawn authority is exposed.
}
```

The named form declares a new block-scoped binding, not an arbitrary expression or an existing context. `sgo.Nursery` and `sgo.Context` are distinct: a nursery handle authorizes child admission; a task context carries cancellation and bounded effect authority and cannot be converted into a nursery. Use an explicit canonical `sgo` import for the named form; canonical identity, not spelling, resolves the type. Freeze alias/unused-import handling in W01. Anonymous scopes may be used for structured lifetime/cancellation but do not create a hidden spawn API.

`nursery` is an S-Go keyword, with no Go-compatible identifier mode. Scanner tables, parser recovery, source positions, ASTs, type checkers, printers, and formatter must agree. All inherited syntax is subject to §1.4 admission, not implicitly enabled by parser reuse.

### 4.2 Explicit nursery-authorized spawning; no bare `go`

**`go f()` is not a valid S-Go statement.** Reject it even inside a nursery, inside unreachable source, in initialization, or in a dependency. Emit `SGO_E_UNSTRUCTURED_SPAWN` with a source location. Do not silently rewrite it into a structured spawn, and do not add a detached equivalent.

The recommended replacement is a compiler-known, runtime-enforced method on an opaque nursery handle:

```go
func helper(n *sgo.Nursery) {
    n.Spawn(func(task *sgo.Context) error {
        return worker(task)
    })
}

func main() {
    nursery (n) {
        helper(n)
    }
    // All admitted work in n has terminated before reaching this point.
}
```

This is proposed S-Go source, not code for an existing Go library. The exact method spelling can change through a recorded specification amendment; explicit authority and no detached task creation cannot.

Required contract for `(*sgo.Nursery).Spawn(func(*sgo.Context) error)`:

- The call is synchronous child admission, not a join. It returns no value after registration and runnable publication. Receiver and callback expression are evaluated once, left to right, in the caller; callback execution occurs only in the admitted child. Shared captured data retains the aliasing rules of the separately admitted S-Go memory model.
- Nursery handles are unforgeable. A call requires a live handle, a valid current-task membership, compatible domain, admitted callback/effect contract, and remaining authority/budget. Static violations are compile errors. State-dependent violations are checked before registration/publication and produce a specified `SpawnContractViolation` with no child created. A nil callback is such a violation. Argument-expression effects already performed in the caller are not rolled back.
- The target must be the caller's current structured scope (its owning scope when it has no inner scope). An explicit handle cannot bypass a currently active inner nursery, target an unrelated scope, or spawn into a closed/finalizing scope. This execution context validates the explicit capability; it never supplies missing authority.
- During normal join, already-live members holding the explicit capability may admit descendants under §4.3. During cancellation, admission creates no child and transfers through the specified sticky cancellation path. Cleanup contexts cannot spawn, even if a closure captured a nursery handle.
- Registration precedes runnable publication. Admission failure cannot leave a half-registered task. Returned `nil` from the child callback means success; a non-nil `error` is a child failure and participates in sibling cancellation and the nursery outcome. Escaping panic, task exit, and cancellation have distinct terminal outcomes. No child error is discarded to emulate `go f()`.
- A helper receives its nursery capability explicitly. A child context does not expose `CurrentNursery`, implicit `.Go`, or another ambient spawning route. Delegation by parameter or closure capture remains subject to membership and lifetime checks.
- Keeping a closed handle for immutable diagnostic inspection may be allowed by its contract; using it to spawn/effect is rejected. An optional future static lifetime system is not assumed to exist. Runtime-checked handle validity is a defined contract, not a claim of static proof.

Every task belongs to one nursery. A task may execute inside a nested nursery without changing its own owner; its current structured scope constrains new explicit admissions. Callback and interface typing must carry/check capability and effect requirements; merely attaching an unchecked contract file to a Go package cannot make it admissible.

A root task executes `main` or an individual test but does not supply an implicit process-lifetime nursery. Per D08, the entry point is `func main(root *sgo.Context) error`; `root` carries the initial capabilities (subset 0: stdout, D09) and the root cancellation, and process exit status is specified per outcome. Per D07, subset 0 has no package-level variables or `init` functions. Initializers may do only operations admitted by the initialization contract; any nursery they open must close before initialization completes. They cannot leave background work behind. Runtime services use sealed, individually specified service primitives, not an ordinary `go` exception or a public escape hatch.

### 4.3 Scope state machine and admission

Use a state machine equivalent to:

```text
OPEN -> JOINING -> FINALIZING -> CLOSED
  \        \
   +------> CANCELLING -> FINALIZING -> CLOSED
```

`CANCELLING` means cancellation requested and drain in progress, not that tasks have disappeared. A separate terminal diagnostic records a fail-stop contract violation; do not represent a poisoned live scope as successfully closed.

The nursery owner is not counted as its own child. It is suspended in join while its children finish.

**Normal join admission:** after the owner's body finishes, already-live child members may still spawn descendants into the same nursery. External/stale callers may not. The last child completion and admission of any new child must serialize; there is no transition to finalization while a registered member can still publish a child. This supports a child that starts its own helper after the owner has entered join.

**Cancellation admission:** new child work is rejected. Cleanup may use pre-existing capabilities through bounded cleanup operations, but may not prolong the scope by spawning replacement children.

Track live children, nested scopes, finalizers, permits, and pending waits with stable IDs and generations. Do not conflate “goroutine finished” with “all scope obligations resolved.”

### 4.4 Every exit path

| Exit | Behavior |
|---|---|
| Closing brace | Stop owner-body execution, join descendants, run scope finalizers, close, continue. |
| `return` crossing a nursery | Evaluate return operands once using the frozen S-Go evaluation order; preserve results; join crossed scopes inside-out; then complete the return and function defers. |
| Outward `break` / `continue` | Join exactly the scopes actually exited, inside-out. |
| Control transfer staying in the nursery | Do not join it. |
| `goto` into a nursery | Reject at compile time. |
| `goto` out of a nursery | Join all crossed scopes exactly once. A staging compiler may reject it explicitly until lowering exists. |
| Panic leaving the nursery owner's frame | Request cancellation; drain descendants; run scope finalizers; preserve propagation of the primary panic. |
| S-Go task-exit operation | `sgo.ExitTask()` drains active scopes before task completion; it is neither ordinary return nor application panic. Raw `runtime.Goexit` is an implementation bridge only, not an automatically available public API. |
| Fatal runtime failure / process termination | A specified fail-stop outcome is not successful scope closure. Raw `os.Exit` is rejected. Any public termination API requires a separately admitted authority and failure contract. |

**Normal nonlocal exits join; they do not implicitly cancel.** Service-style children must be cancelled explicitly before leaving normally. This is a deliberate semantic choice, not a hidden timeout.

For loops that re-enter the same source-level nursery, allocate distinct scope identities. A label jump that re-enters from outside must not revive an old scope.

### 4.5 Defer, finalization, and panic ordering

The v2 default deliberately specifies function-scoped `defer`: a direct `defer` in a nursery body belongs to the enclosing function and does not run merely at the nursery brace. Freeze argument evaluation, order, panic, recovery, and interaction with all scope exits in the S-Go clauses before admitting this construct. R6 is design/source reference, not an automatic fallback.

Define a distinct scope-finalizer contract before enabling this API:

```go
// Proposed API shape, not existing code.
n.Defer(func(cleanup *sgo.CleanupContext) error {
    return closeOwnedResource(cleanup)
})
```

Scope finalizers run LIFO after children have terminated. This keeps a parent's shared resource alive until its users finish. A producer whose shutdown requires closing a channel must close/cancel explicitly before join; a deferred close after join cannot solve that dependency cycle.

Cleanup contexts retain only necessary attenuated authority, mask ordinary cancellation delivery, cannot spawn, and share the remaining cleanup budget. They do not grant unlimited time or new authority. Finalizer failures are recorded without suppressing the primary failure.

When unwinding out of the function that contains active nursery scopes, nursery barriers run before that frame's ordinary defers. Defers in deeper called functions follow the frozen S-Go unwind order. Ordinary function defers retain their LIFO order relative to each other. Consequently, a function-level `recover` may observe a panic after affected nurseries have already drained.

A panic recovered inside a deeper helper before it crosses a nursery boundary is not a nursery failure. A panic that escapes a child task is captured after that task's normal defer/recover processing. Cancel siblings as soon as an escaping child failure is published, then join all children.

For a single escaping child panic, re-panic with the original payload at the parent's join boundary. For multiple child panics, choose the primary by stable task creation ID, retain all payloads/stacks in diagnostics, and do not select a primary based on wall-clock timing. An already-active owner panic remains primary. Explicit child errors produce a structured nursery failure unless handled by an API wrapper that returns that outcome as an error. Cancellation-only completion is distinguished from application error/panic.

Handle `panic(nil)`, panic during a defer, recover followed by a new panic, finalizer failure, and the specified task-exit operation explicitly. Never classify outcomes solely by testing `recover() != nil`.

### 4.6 Cancellation transfer from native syntax

Explicit cancellation-aware functions return typed cancellation results. Native channel syntax has no added error result, so it needs a different, specified mechanism.

**Decision:** a cancelled, managed native blocking operation unregisters its waiter and raises a runtime-recognized cancellation unwind signal from a safe user execution point, after releasing internal locks. The nursery/task boundary recognizes this as cancellation, not an application panic. Expose a predicate such as `sgo.IsCancellation(value)` for deliberate recovery.

Per D13, `recover` is denied in subset 0, so no code can intercept this signal there; the following applies once `recover` is admitted. Ordinary `recover` may intercept the signal. Cancellation remains sticky, subsequent cancellable operations observe it again, and a task cannot turn a cancellation request into new authority or new child admissions. Swallowing cancellation forever is non-cooperation, handled by the drain-contract escalation. This is a deliberate S-Go contract, not a constraint imposed by Go compatibility; a future non-interceptable cancellation mechanism would need its own complete semantics and implementation review.

Cancellation does not make a receive return a fabricated zero value, does not silently drop a send, and does not automatically close a shared channel. A native operation that already committed reports success; cancellation is observed at a later boundary.

Do not inject arbitrary cancellation panics at every instruction or loop backedge. Compiler-inserted lab checkpoints initially yield/account for execution only. Explicit checks and supported effect boundaries are where cancellation transfers control.

---

## 5. Runtime architecture and GC integration

### 5.1 Components

Keep public API types out of runtime dependency cycles.

| Component | Responsibility |
|---|---|
| Compiler integration | Closed specification admission, binding/type/effect checks, explicit spawn lowering, structured exit IR, task entry wrappers, spec/ABI metadata, safe lab checkpoints. |
| Runtime scope core | Task registration, nursery state, join, cancellation state, terminal outcomes, obligation accounting. |
| Public `sgo` package | Opaque contexts, explicit task/error APIs, cancellation inspection, deadlines, scope finalizers, readiness handshake. |
| Internal ABI package or builtin declarations | Minimal shared layouts/IDs and compiler-known runtime entry points. No import from runtime back into the public package. |
| Effect layer | Capability checks, cancellation-aware waits, simulated and real adapters. |
| Managed channel engine | Native channel integration and explicit typed permits using one semantic protocol. |
| Lab engine | Runnable decisions, virtual timers, deterministic IDs, trace/replay, fault injection. |
| Host runner | Builds/runs test subprocesses, captures traces, enforces wall-clock watchdogs, retains failure artifacts. |

Use existing runtime lock/park/wake mechanisms; do not add an unrelated user-space M:N runtime. Runtime hooks must obey scheduler constraints about allocation, stack splitting, write barriers, and lock ordering. [R7]

### 5.2 State and lifecycle

The design needs equivalents of `ScopeID`, `TaskID`, `OperationID`, `PermitID`, and an execution-domain ID. Use monotonic identities with overflow handling; do not expose addresses as identities.

A scope record needs its parent identity, owner task, state, children/obligations, cancellation cause, budget, finalizers, and terminal outcome collection. A task record distinguishes **owner scope** from **current structured scope**, and records its terminal state once. A permit references its channel, owner, generation, and terminal-state record.

A completed scope may leave behind an immutable diagnostic/result snapshot. Retaining such a snapshot must not retain live task stacks, pending timers, or channel values unintentionally. Context inspection after closure may read the snapshot; effect/spawn operations using a closed context fail.

Registration must precede publication to a runnable queue. The runtime central creation path is authoritative for accounting; compiler-generated entry trampolines handle user task outcomes where practical. Evaluate the nursery receiver and callback expression in the parent exactly once. Every admitted explicit spawn, including optimized/inlined and dependency calls, must use the same validation, trampoline, and registration. Bare `go` is rejected by the frontend, not retrofitted with dynamic ownership in the runtime.

Treat task completion as a multi-part transition: unwind user defers; close nested nurseries; resolve owned operation records; publish outcome and join synchronization; deregister; then allow goroutine reuse. Reset all S-Go fields before a recycled runtime goroutine can run again.

### 5.3 Rooting and stack movement

Initial scopes are ordinary heap-backed records. Anchor live records through an explicitly audited GC-visible registry/root path. Do not assume that adding a pointer field to a runtime `g` structure automatically supplies the required rooting.

If the chosen representation places references in persistent/non-scanned runtime storage, add and test the required explicit root scanning. Allocate bookkeeping while allocation is permitted, before entering scheduler critical sections. Publishing a child while holding a runtime lock must not unexpectedly allocate a map, grow a slice, invoke a user callback, or require an unavailable write barrier.

The implementation note for W02 must answer, for each pointer-bearing structure:

- What keeps it alive, and what code scans that root?
- Which writes need barriers, and in which execution context are they legal?
- When can it be removed, and what prevents a concurrent observer from using a recycled record?
- Can it contain a pointer into a goroutine stack? If so, why is that unavoidable, and how is relocation handled?

The default answer to the last question should be “no” for public/runtime scope state. Exception-unwind markers are more subtle: they identify live lexical scopes associated with frames. Their location metadata must remain valid through stack growth/shrink/copy. A rooted heap scope alone does not solve that problem.

Use upstream stack maps and escape/liveness machinery. Never mark an escaping parent object stack-only merely to meet a zero-allocation goal. Never use `uintptr` to hide live pointers from the collector. Add explicit join race annotations as appropriate; tests must validate the real synchronization, not merely silence the race detector.

### 5.4 Compiler lowering requirements

Implement nursery representation through the actual pinned pipeline: private syntax, `types2`, Unified IR encoding/decoding, IR walking/lowering, liveness/escape analysis, and generated runtime declarations. Native source tools need their corresponding public AST/parser/type-checker updates. [R2, R8]

Use a structured IR node or equivalent scope-aware control-flow representation that survives until all exit edges can be lowered correctly. Emitted cleanup must be idempotent across normal paths and exceptional fallback paths.

An ordinary hidden closure is **not** acceptable as the final implementation of arbitrary nursery bodies: it changes the targets of `return`, `break`, `continue`, labels, function-scoped `defer`, and sometimes `recover`. A callback API is useful for the reference model and early runtime experiments, but it is not the language implementation.

Preserve source positions for errors, panic stacks, traces, and checkpoint IDs. Add export/import round trips, cross-package inlining, generic functions containing nurseries, closures, and named-return tests. A compiler that works only with inlining disabled has not passed this gate.

---

## 6. Cancellation, budgets, and non-cooperation

### 6.1 Supported operations first

Start with explicit checkpoints, nursery join, managed sleep/timers, and managed channel waits. Add I/O only when each adapter has a documented cancellation boundary and progress result. Reject unsupported operations; they are not runnable S-Go with a warning or a reduced guarantee.

Cancellation causes include owner request, ancestor cancellation, deadline, escaping child failure, and contract violation. Preserve cause identity and ancestry; do not overwrite the original cause with a later convenience error.

Cancelling a nursery signals both its children and its still-executing owner body at supported boundaries. For the owner, effective cancellation includes its task's owning-scope ancestry **and** its currently active nested nursery. Do not check only `task.ownerScope` and miss a child failure while the owner is blocked inside the nursery body.

A cancellation unwind signal identifies its originating scope/cause generation. An inner nursery drains itself but must not consume cancellation targeted at an ancestor. Local cancellation-only completion may be consumed at its matching boundary; ancestor cancellation remains pending and propagates outward. These rules do not discard an independent application panic/error.

Normal completion racing with cancellation is decided by the operation's terminal transition. A completed operation remains completed. A task's final classification separately records whether cancellation was requested before it finished.

### 6.2 Waiter protocol

For every blocking operation, draw the state transition before coding it. The minimal protocol is:

```text
NEW -> REGISTERED -> READY -> CONSUMED
                 \-> CANCELLED
                 \-> CLOSED_OR_FAILED
                 \-> SELECT_LOSER
```

Actual implementations may combine states, but must preserve one terminal winner. Check cancellation and publish the wait registration under a synchronization protocol that prevents the “cancel after check, before park” lost-wakeup window. Use the runtime's atomic park/unlock handshake rather than manually unlocking and later parking.

Detach cancelled/losing waiters exactly once. Queue wakeups while locked as required, then make tasks runnable after releasing locks. Establish and document a lock order across scope, channel, timer, and selection records. Never call finalizers or arbitrary application code while holding these locks.

A final cancellation check after an effect committed must not erase that effect. Expose partial read/write counts for I/O. Retry only from an explicitly no-effect outcome.

### 6.3 Budget model

There are three different budgets:

| Budget | Measures | Purpose |
|---|---|---|
| Virtual deadline | Simulated monotonic time | Time-dependent test behavior and simulated cancellation deadlines. |
| Event/checkpoint quota | Managed progress opportunities | Bound schedules where runnable work prevents virtual time from advancing. |
| Host watchdog | Real elapsed time of a subprocess | Catch uninstrumented loops, runtime deadlocks, blocking host behavior, and broken diagnostics. |

Use explicit units and overflow-checked arithmetic. A nested scope's budget cannot exceed the remaining ancestor budget. Shielded cleanup consumes the remaining budget rather than starting a new full allocation. Do not choose a universal 500 ms production policy merely because the original whitepaper uses that example.

Default test quotas and watchdog values are runner configuration, recorded in evidence. Production drain policies are explicit application settings. A normal nursery may intentionally run for the lifetime of a service; normal operation has no surprise shutdown deadline.

### 6.4 Escalation

When cooperative drain exhausts its contract, capture the scope tree, pending task/operation IDs, last known checkpoints, budget/cause, and available stack diagnostics. The runner or process then terminates with a non-success result.

Do not remove a live goroutine from scheduling and continue the application as though cleanup succeeded. It may own locks, references, or external effects. Diagnostics in a corrupted/deadlocked runtime are best effort; an external watchdog is the final containment boundary and must not wait forever to produce a perfect dump.

Compiler-inserted deterministic yield checkpoints improve lab observability and step bounds for compiled code. They do not make arbitrary foreign code, assembly, syscalls, or unsafe cleanup cancellable. A host-watchdog failure is recorded separately from an exact replayable virtual-budget failure.

---

## 7. Reserve/commit/abort contract

*Historically labeled "BlenD2" in the input archive; see the provenance correction in §2. Design inspiration: [R13].*

### 7.1 Model before native-channel surgery

First implement an executable state-machine model with bounded exploration. Then implement a typed managed-channel surface against it. Only then integrate the protocol into native `chan`, `select`, channel range, and reflection paths.

This sequencing is not a retreat to a library: the typed layer is a conformance oracle and development surface. Native integration remains a required work package.

There is one admitted S-Go channel contract; no ordinary/unmanaged channel mode exists. Until native channel integration passes its gate, reject native channel use rather than falling back to inherited channel paths. Managed channels belong to an execution domain, not necessarily the nursery that happened to allocate them; legitimate communication between related scopes remains possible. Operation permits/waits belong to their owning tasks/scopes. Crossing incompatible lab/host domains fails rather than mixing schedulers.

### 7.2 Permit state and aliases

Use a shared operation record behind any public handle:

```text
RESERVED -> COMMITTING -> COMMITTED
    |            \----> FAILED_NO_EFFECT
    +-----------------> ABORTED
```

`COMMITTING` is a short, non-suspending arbitration state, not permission to hold a channel lock while running user code. Resolve the real operation under the channel's synchronization protocol. A commit failure reports the value as unpublished; success reports it as published once.

A public permit must be explicit about `Commit(value)` and `Abort()`. Copies and pointer aliases refer to the same state; they do not duplicate capacity. Add `noCopy`/vet support as a diagnostic aid, but runtime state enforces correctness. This S-Go design does not use garbage-collection finalizers to resolve abandoned permits; deterministic obligation resolution is specified below. This is a design choice, not a Go-compatibility constraint.

Track permits in task/scope obligations. At owner-body exit, abort that body's outstanding uncommitted scope-owned permits before joining children, so an unused reservation cannot itself prevent a waiting child from finishing. Likewise resolve a terminating child's reservations before publishing its completion. Initial permits are task-affine at runtime: aliases share state, but another task cannot commit or transfer the obligation without a separately specified transfer protocol. Task termination resolves unused reservations; abandoned permits generate a diagnostic where required. Do not silently leave capacity reserved until some future GC. Long-lived tasks can explicitly abort; configurable scope operation/permit quotas bound accidental accumulation.

### 7.3 Linearization decisions

| Race or event | Required result |
|---|---|
| Cancellation wins before reserve | No permit and no effect. |
| Cancellation wins after reserve but before commit arbitration | Reservation is aborted; capacity/value ownership is restored. |
| Commit wins before cancellation | Complete the short commit; report success if publication succeeds. Later cancellation is not retroactive. |
| Close wins before publication | Commit returns a closed/no-effect result for the explicit API; native syntax follows its specified send-on-closed panic behavior. |
| Publication wins before close | The value remains published; close cannot discard it. |
| Double commit / commit after abort | Deterministic misuse diagnostic; no second effect. |
| Abort after successful commit | No rollback of the published value; return a terminal-state error or documented idempotent status. |
| Task panics with a reserved permit | Abort and release the obligation before task completion is published. |

Do not claim commit “cannot fail” unless the chosen reservation contract actually prevents every competing close/disconnect event. The default contract above deliberately permits a no-effect closed result. Reserve/commit techniques in the cited runtime inspiration also distinguish a successful publication from a disconnected receiver outcome. [R13]

### 7.4 Buffered and unbuffered channels

Publication/value ownership in this protocol means responsibility for whether a send took effect; it is not a new affine type system for arbitrary S-Go values. Slices, pointers, and other values can still alias application memory. This protocol does not grant race-free access to those aliases.

For capacity `C > 0`, model queue length, reservations, blocked senders/receivers, and close state. Enforce `queued + send_reserved <= C`. Preserve visible FIFO order; do not expose uninitialized reserved slots. Keep pointer-containing values alive until delivery or abort, and clear references after ownership ends.

For `C == 0`, a send reservation requires a matching receive opportunity; the handshake owns both operation records until a single commit/abort decision. No `% capacity` arithmetic may execute for zero capacity. Preserve the send/receive synchronization required for rendezvous channels. [R9]

A receive reservation, if exposed publicly, must identify a specific message/rendezvous and define what other receivers may do while it is held. Do not add this public feature until its semantics are modeled. Internal receive-side state for rendezvous/select is required regardless.

### 7.5 Select, close, and admitted native surfaces

A `select` has one shared decision record. Candidate cases may register waiters, but only one case wins; all losers unregister/abort. Lock ordering must cover multiple channels without deadlock. For lab trace identity, use stable channel/case IDs, not memory addresses.

Specify channel operands and send values to be evaluated exactly once in source order before selection; freeze all remaining expression-order details in S-Go clauses. Handle nil channels, default, closed receive, closed send, duplicate channel cases, cancellation races, timers, and an empty select. A cancellation path cannot execute a user case or publish a value for a losing case.

Cover compiler fast paths, `reflect.Select`, channel range, channel direction conversions, generics, channel length/capacity, race annotations, and GC stack-pointer handling. Upstream channel/select sources are the implementation reference, not the short `hchan` replacement in the archive. [R10]

Scope cancellation wakes its owned waiters. It does not close a shared channel automatically. Shutdown by close remains an explicit channel-owner operation.

---

## 8. Capabilities and standard-library policy

### 8.1 Threat model

Enforcement is mandatory for all admitted S-Go code, including the complete dependency graph; there is no unsafe or legacy application dialect. It is not a claim that arbitrary hostile native code inside the same address space is safely sandboxed.

Treat the compiler, linker, target runtime, selected library internals, and audited adapters as the trusted computing base (TCB). Every target primitive must have an explicit specification, lifecycle/effect contract, call-site policy, and conformance evidence. Trust is an enforcement boundary, not an exemption from specification. List the set by exact build identity, source/symbol identity, and reason. Public package names, annotations, copied code, or calls originating in a trusted package cannot confer unrestricted privilege. Host bootstrap/runtime code is separately identified as external implementation tooling, not target S-Go.

Reject user cgo, assembly, `unsafe`, unsafe reflection bypasses, `go:linkname`, plugin loading, direct syscalls, arbitrary process execution, uncontrolled finalizers/cleanups, and alternative I/O entry points. Do not add an enable-unsafe switch; a future facility must have a new enforceable S-Go contract, not inherit these escape hatches. Package initialization and callbacks are part of the audit. Dynamic dispatch/function values do not make a forbidden target acceptable.

Use compile/link-time transitive checking for unsupported surfaces and runtime checks at supported effect entry points. An incomplete analyzer must reject unknown effectful cases rather than claiming to prove them safe. Never authorize by matching a function's printed name alone.

### 8.2 Authority representation

Separate cancellation/lifecycle context from the actual effect grant. A context may carry opaque references to grants, but passing a broad context does not itself demonstrate least privilege.

Effective authority is bounded by both the current task's inherited ceiling and the explicit handle. Child grants can narrow path, operation, address, byte, or time limits; they cannot enlarge them. Root grants come from a trusted runner/application policy, not a public constructor callable by dependencies.

Capabilities need domain/issuer identity, grant type, attenuation, expiry/revocation behavior, and a stable ID for traces. Public fields must not allow forging a stronger grant. The mutable shared-state model must not let a child edit the parent's authority.

### 8.3 First supported effects

Implement capability-mediated clock/timer and deterministic test-randomness APIs first, then simulated files/network, then bounded production adapters. Keep cryptographic entropy separate from a replayable PRNG; never silently replace production security randomness with a seeded generator.

For filesystem access, do not implement confinement as a string-prefix check. Resolve relative to an owned directory/root capability and test traversal, symlinks, races, and handle reuse using platform-supported safe primitives chosen during W09. For network access, define whether policy grants hostnames, resolved endpoints, listeners, or an already-open connection. Check redirects and subsequent connections through the same policy.

Opaque owned file/connection handles may carry authority for subsequent operations, so every method need not accept a redundant context solely for style. Cancellation/deadlines must still be explicit and enforceable. Document close ownership to avoid cancelling one task by destroying another task's shared resource.

### 8.4 Library admission, not ecosystem preservation

Existing Go packages are migration inputs, not supported dependencies. Recompilation alone does not make them legal. Admit a package only after its exposed and internal reachable operations satisfy S-Go clauses, its source and transitive dependencies are checked, and its code/contract identities are bound into export/link metadata. All package bodies must use admitted syntax, including otherwise-unused functions; dead code must not hide a legacy construct.

Fork, redesign, narrow, or omit standard-library APIs when required. A familiar package name or matching signature does not authorize an effect. Breaking source/API compatibility is acceptable and is never, by itself, a reason to retain ambient I/O or detached task behavior. Pure utilities also require an admitted behavioral/type contract; “pure” is not an exemption from language admission.

Long-lived library background tasks need explicit structured service ownership supplied through the API or an individually specified runtime-service primitive for genuine runtime work. Registering arbitrary dependency tasks with an immortal runtime root is forbidden. Omit APIs until their lifecycle, cancellation, failure, and effect contracts are complete.

Keep the host compiler's implementation dependencies in the separate bootstrap environment. Porting the target library must not be blocked on preserving the host tool's APIs: use an isolated host build/source boundary, not a target compatibility flag (§10.2).

A hostile-code deployment requiring stronger native containment additionally needs an OS/process boundary. That is a separate deployment contract, not something the presence of a context parameter proves.

---

## 9. Deterministic lab and replay

### 9.1 Reuse upstream mechanisms, extend the guarantee

Study `testing/synctest` and `runtime/synctest.go` before adding another virtual-time ownership mechanism. Upstream already implements descendant bubbles, fake time, durable blocking, and domain checks. Those are useful source patterns, not proof that a PCG-driven replay scheduler already exists. [R11]

Do not attempt to nest two independent lab/bubble schedulers without an explicit integration design. Initially reject unsupported nesting and run lab cases as isolated processes.

**Single managed user execution** is the requirement—not a false assertion that the Go runtime can have only one OS thread. Runtime/GC/service execution may still need its own machinery. Prove that it does not change eligible user decisions or recorded effects. `GOMAXPROCS=1` alone is not the implementation.

### 9.2 Nondeterminism inventory

| Source | Initial lab policy |
|---|---|
| User runnable selection | Stable eligible set plus versioned scheduler PRNG or recorded decision. |
| Spawn/wake ordering | Register IDs before publication; canonical ordering for simultaneous visibility. |
| Native select | Log eligible case IDs and selected winner; replay checks the set. |
| Equal-deadline timers | Stable `(deadline, timer ID)` ordering. |
| Time reads/sleep | Virtual monotonic clock and fixed recorded epoch. |
| User randomness | Explicit deterministic capability with a separate PRNG stream. |
| Map iteration | Initially reject user/dependency map-range paths in the supported replay subset, or replace with an audited deterministic collection operation. Do not claim a scheduler seed controls upstream map iteration. |
| Pointer formatting / address order | Exclude from canonical traces; reject reliance on it for managed decisions. |
| Filesystem enumeration / network / DNS | Simulated adapters with deterministic fixtures in the initial release. |
| Environment / arguments | Frozen in the run manifest; unexpected reads fail or use recorded policy values. |
| Finalizers / cleanups / signals | No uncontrolled user callbacks inside the guarantee. |
| Background runtime activity / GC | Audit safe points and wakeups; validate repeated traces under GC stress. |
| Shared-memory synchronization | Only audited operations enter the replay coverage contract. Production race/stress testing remains required. |
| cgo / assembly / raw syscalls | Rejected as application escape paths in every S-Go execution configuration. Only individually contracted TCB operations may use native mechanisms. |

Reject unsupported effects at the earliest reliable boundary and emit `UnsupportedLabEffect` with source/operation detail. Do not merely print a warning and continue claiming determinism. The map-iteration restriction applies transitively to relevant dependency code; an allowlisted helper must establish deterministic observable behavior itself.

### 9.3 Scheduler and time algorithm

At each supported scheduling boundary, produce the canonical list of runnable user task IDs. Select from that list with a versioned bounded random function, or consume the next replay decision. Execute the selected task until a managed block/yield/boundary.

When there are no runnable user tasks, inspect managed pending events. Advance virtual time to the next event deadline and publish eligible wakeups in canonical order. If no task and no pending event can make progress while a scope remains live, report a managed deadlock with its wait graph.

Runnable work can prevent time advancement. Count execution checkpoints/events independently. Instrument supported loops and selected call boundaries for deterministic yielding/accounting without arbitrary cancellation exceptions. Exempt runtime critical sections correctly. Store instrumentation version and compiler flags in the manifest.

Do not assume a random schedule is fair. Include a deterministic FIFO/round-robin diagnostic scheduler and a bounded-starvation policy for chosen stress configurations. A schedule quota exhausted while tasks remain live is a distinct result, not proof of deadlock.

### 9.4 PRNG contract

PCG32 is acceptable. Freeze its algorithm version, unsigned arithmetic, seed/stream initialization, odd stream increment, rotation behavior, and bounded-range sampling. Test against published/reference vectors. [R12]

Use separate reproducible streams for scheduling, user test data, and injected faults. Record the stream derivation algorithm; do not derive from process addresses, implementation-dependent hashes, or map order. Prefer unbiased bounded sampling rather than blindly using `rnd % n`.

A seed is a convenient generator input. A trace is the authoritative replay artifact.

### 9.5 Trace protocol

Start with a versioned JSONL format for inspectability. Buffer runtime events safely; avoid allocation or formatting while holding runtime locks. A host-side encoder can serialize preallocated records.

The header includes:

```text
trace_schema_version, sgo_abi_version, upstream_commit, fork_commit,
binary_digest, compiler_flags, spec_digest, admission_digest, execution_configuration, GOOS, GOARCH,
source_and_dependency_digests, workload_and_arguments,
capability_policy_digest, fixture_digests, environment_snapshot,
scheduler_version, instrumentation_version, seed_and_streams,
virtual_epoch, event_quota, deadline_policy
```

Events include scope/task/operation IDs, source/checkpoint identity, virtual time, event sequence, eligible choice set, selected ID, and terminal outcome/progress. Record spawn, wait, wake, choice, timer, cancel, permit, commit/abort, scope exit, and diagnostic transitions.

Canonical output must exclude PIDs, wall-clock durations, host paths, and addresses. Retain nondeterministic host diagnostics in a separate sidecar. Include a footer with event count, completion status, and digest so truncated traces cannot masquerade as complete ones.

Replay rejects an incompatible manifest before execution and stops at the first impossible choice or event mismatch. Report expected/actual eligible sets and the last matching event. Never fall back to PRNG choices or real I/O on divergence.

A different architecture, optimizer setting, binary, fixture, or adapter revision may invalidate a trace. Cross-version/cross-architecture replay is a later feature with its own equivalence rules.

### 9.6 Exploration and evidence limits

Implement bounded enumeration of the pure state model for small scenarios; this is distinct from seed sweeps of the runtime. Use both. Add fault injection at reserve/commit/abort, cancellation delivery, close, readiness, and simulated I/O completion boundaries.

A repeated seed sweep passing means those executions passed. It is not a proof of all schedules. A deterministic lab over effect boundaries is not a complete data-race detector and does not reproduce every production instruction interleaving. Publish that boundary clearly.

---

## 10. Repository, bootstrap, and source map

### 10.1 Proposed additions

Keep the upstream layout where it remains useful for source reuse, but remove or isolate inherited code that has no admitted target role. Layout reuse is not a requirement to preserve all upstream functionality. Add project material with clear boundaries:

```text
sgo/                              # Project support, not a replacement toolchain root.
  README.md
  UPSTREAM.lock.json
  GUARANTEES.json
  SPEC_ADMISSION.json               # Closed, versioned target feature/contract registry.
  TARGET_TCB.json                   # Individually contracted target runtime/adapter primitives.
  TASKS.json
  TEST_MATRIX.json
  docs/
    original/                     # Unmodified supplied design documents.
    decisions/                    # Numbered, focused design amendments.
    source-map.md
    language.md
    constitution.md
    clause-adoption-ledger.md       # Explicit adopted clauses, no implicit Go semantics.
    runtime-invariants.md
    capability-policy.md
    replay-format.md
  model/                          # Separate small Go module; pure state-machine oracle.
  scripts/
  testdata/                       # Positive/negative compiler and runtime fixtures.
  examples/
  evidence/                       # Indexes/manifests; large logs stored as CI artifacts.
  checkpoints/
src/sgo/                          # Proposed public standard-library facade in the fork.
src/runtime/sgo_*.go               # Proposed runtime additions; names confirmed in W00.
```

Do not introduce broad abstractions just to fill this tree. Add files as their work package requires them. Preserve upstream generated-file workflows, licenses, release notes, and test harnesses.

### 10.2 Host bootstrap is external tooling, not a target dialect

Separate three artifacts by source/build/cache identity:

1. **Pinned stock Go bootstrap/reference tools:** external tools used to build compiler implementation code and run the untouched upstream comparison suite. They are not shipped as an S-Go application path.
2. **S-Go compiler implementation:** initially written in host Go and built by the external host toolchain. Its implementation language does not determine the language it accepts. It must emit only specified S-Go target programs and reject bare `go`, uncontracted APIs, and missing target metadata.
3. **S-Go target runtime/libraries/programs:** governed by S-Go contracts and target admission. Target-runtime machine-level mechanisms require sealed, individually specified TCB intrinsics. Stock target objects or host libraries cannot be linked in merely to make a build succeed.

Do not assume the inherited self-hosting `make.bash` pipeline can recompile host-Go compiler sources with the new S-Go compiler. W00 must map those stages and provide a host-built stage-0 compiler/driver path. Later self-hosting requires porting the implementation to admitted S-Go, not adding a legacy parser flag, file extension, pragma, or package exemption. A release must state whether it is host-built or self-hosted.

Target runtime service-spawn sites must be ported to sealed service intrinsics with enumerated purposes and lifetimes; they cannot retain ordinary `go` as a language exception. Low-level runtime assembly must be represented in the target TCB manifest with its own contracts and inaccessible user entry points. This is a necessary trusted implementation boundary, not a second application language.

A baseline command sequence such as `make.bash`/`dist test` may run only in the untouched, isolated upstream reference worktree. The fork's build commands must be derived from its actual host/target stage map. Record `GOROOT_BOOTSTRAP`, host compiler/source checksums, target ABI/spec identities, and actual executable paths. Disable automatic stock-toolchain fallback or downloads in the S-Go driver. [R14 is background source material.]

Use distinct caches and manifests for host tools, untouched reference builds, and S-Go target artifacts. Negative tests must attempt cross-linking, stale export reuse, mode flags, environment overrides, copied package names, and direct backend invocation. Public compiler, linker, driver, and import entry points must agree on admission. An isolated compiler-unit test harness is not an additional supported target language.

Port/adapt upstream tests only after mapping their expected semantics to an admitted S-Go clause. Intentionally incompatible Go programs should become rejection fixtures; preserving their behavior is not a release gate. Race instrumentation and other native analysis components need their own contracted TCB build lanes and cannot re-enable user cgo or unsafe code.

### 10.3 Source map: starting points, not blind patch instructions

**Verified starting points** below were checked against primary source pages. Other entries are inspection targets whose exact files/symbols must be recorded from the checkout before editing.

| Area | Start here | Status and required inspection |
|---|---|---|
| Compiler overview | `src/cmd/compile/README.md` | Verified pipeline overview. [R2] |
| Private lexer | `src/cmd/compile/internal/syntax/tokens.go`, `scanner.go` | Verified. Scanner uses a keyword hash table; adding a keyword must account for collisions/generated token data. [R15] |
| Private syntax | `.../syntax/nodes.go`, `parser.go`, walkers/printers | Inspect dispatch, positions, recovery, branch checks, generators. |
| Type checking | `src/cmd/compile/internal/types2/` | Inspect statement handling, scope binding, branch validation, generated/shared code with `go/types`. |
| Unified IR | `src/cmd/compile/internal/noder/writer.go`, `reader.go` | Verified. Extend encoding/decoding and applicable format identities. [R8] |
| IR/lowering | `src/cmd/compile/internal/ir/`, `walk/`, `ssagen/`, `escape/`, `liveness/` | Inspect every transformation that can duplicate, move, or erase an exit barrier. |
| Runtime builtins | Compiler runtime declaration/generation files | Discover exact files/generator in checkout. Do not hand-edit generated output only. |
| Public language tools | `src/go/{token,scanner,ast,parser,types,printer,format}/`, `src/cmd/gofmt/` | Inspect complete S-Go propagation and admission; standalone source APIs must not expose an implicit legacy acceptance mode. |
| Task creation/scheduling | `src/runtime/proc.go`: `newproc`, `newproc1`, completion and runnable paths | Creation paths verified; audit every caller and goroutine-reuse path. [R16] |
| Runtime task layout | `src/runtime/runtime2.go` and runtime GC roots | Inspect `g`, wait records, ownership storage, root scanning. |
| Panic/defer/Goexit | `src/runtime/panic.go`, compiler defer lowering | Inspect normal and open-coded defers, recovery, stack unwind, task completion. |
| Stacks | `src/runtime/stack.go`, stack allocation/copy/shrink; arch assembly | Inspect only for correctness until W11. [R4] |
| Prologue stack checks | `src/cmd/internal/obj/x86/obj6.go`: `stacksplit` | Verified x86 location; architecture-specific changes require separate review. [R3] |
| Memory mapping | `src/runtime/mem.go`, OS-specific memory files | Generic contract verified; inspect actual OS reservation/mapping operations. [R5] |
| Channels/select | `src/runtime/chan.go`, `select.go`, compiler channel lowering | Verified runtime references; inspect fast, reflect, race, and stack-copy paths. [R10] |
| Lab starting point | `src/runtime/synctest.go`, `src/testing/synctest/`, associated timer/scheduler hooks | Verified references; extend rather than assume seeded replay already exists. [R11] |
| Timers/netpoll | Timer and netpoll runtime files; `internal/poll`, `net`, `os` | Discover exact files and ownership/cancellation contracts before adapters. |
| Build/link identity | `cmd/go`, compiler export data, build configuration, linker metadata | Inspect spec/ABI/admission propagation, host-target isolation, and rejection of stock or mismatched objects. |

For each edited subsystem, record file, symbol, pinned upstream commit, callers, invariants, and applicable regression tests. A function named in an old design but absent from the checkout is an audit finding—not a reason to create a disconnected replacement.

---

## 11. Dependency-ordered work packages

`SGO_TASKS.json` is the machine-readable ledger. The descriptions below define the work; the JSON test matrix supplies concrete scenarios. All initial statuses are `pending`. Passing the written plan's consistency checks is not passing implementation tests.

### W00 — Establish a reproducible fork and baseline

**Depends on:** nothing. **Risk:** medium.

Inspect the current tree; preserve local changes. Create or locate the real Go fork. Resolve `go1.27.3` (D21) to its exact commit; record bootstrap binary/source provenance and checksums. Store original document/archive hashes. Establish host/target build separation, an explicit no-legacy policy, the initial deny-by-default target gate, deterministic inputs, and isolated baseline scripts. Map self-hosting stages rather than assuming the new compiler can compile unported host-Go implementation sources.

Build the unmodified toolchain first. Record its applicable distribution tests, package tests, benchmark baselines, host configuration, and known environmental failures. Create the pinned source map and prohibit use of guessed symbols from the old phase documents.

**Deliverables:** `UPSTREAM.lock.json`; isolated host/reference build scripts; target stage map and deny-by-default gate; source map; baseline evidence; initial task ledger; a real buildable checkout.

**Gate:** another clean worktree can reproduce the recorded build procedure; reported compiler paths point into the fork, not the system installation. A source download/build failure is a blocker to runtime implementation, not permission to simulate a successful bootstrap.

### W01 — Freeze semantics and build an executable oracle

**Depends on:** W00. **Risk:** high.

Translate the constitution and §§1–9 into a closed, versioned specification for the first subset. Expand `SPEC_ADMISSION.json` against every relevant frontend/IR dispatch and export surface; freeze explicit adopted clauses and exclusions, initial package/effect-summary checking, and TCB contracts. Do not enable a feature with an unresolved normative question. Then translate the approved rules into focused language/runtime decisions. Resolve contradictions by recording the chosen rule and its tests; do not reopen settled defaults simply to produce more prose. Implement a small pure-Go state model for scope state, child admission/completion, cancellation, obligations, and terminal outcomes. It should support bounded action enumeration and invariant checks without OS goroutines.

Add golden action histories for normal join, live-child descendant creation during join, cancellation, nested scopes, competing failures, and invalid operations. Deliberately mutate the model to verify that the oracle rejects missing registration, duplicate completion, and closure with outstanding obligations.

**Deliverables:** closed first-subset spec and adoption ledger; complete deny-by-default inventory; TCB primitive contracts; executable state model; invariant-to-test map; mutation/admission checks.

**Gate:** each first-subset feature has complete normative clauses and both acceptance and rejection scenarios; all other features are denied. The oracle finds deliberately introduced invariant violations and rejects invalid histories. Inventory coverage is checked mechanically against the actual surfaces. The model is not an implemented language/runtime.

### W02 — Runtime ownership, join, and GC-safe state

**Depends on:** W01. **Risk:** very high.

Add runtime scope/task records and the specified explicit nursery admission primitive. Internal, non-distributed conformance hooks may test it before nursery syntax exists; no hook permits legacy target source. Implement owner/current-scope distinction, explicit-handle validity/membership checks, registration before runnable publication, join, stable IDs, and record cleanup. Port runtime-service task creation to contracted sealed primitives. Dependencies must call the checked S-Go spawn API; they cannot retain bare `go`.

Implement and document the GC-root scheme, allocation-safe registration protocol, lock order, goroutine-field reset, and join synchronization. Treat runtime services as a small audited class. Keep upstream stack growth/shrink and checks intact.

**Deliverables:** real runtime integration; temporary API tests; GC/root/lock design note; ownership diagnostics.

**Gate:** nested ownership, descendant admission races, goroutine reuse, and GC/stack-growth stress pass. Scope records and children are reclaimed after quiescence. The admitted S-Go subset passes its own gates, and negative tests demonstrate that neither legacy spawning nor an uncontracted service path can execute. The isolated upstream baseline remains a comparison artifact, not a target mode.

### W03 — Full nursery compiler path and exit semantics

**Depends on:** W02. **Risk:** very high.

Implement specification-admitted tokens/parser/AST, distinct nursery/context binding and type checks, explicit spawn lowering, private/public source surfaces, Unified IR round trips, native IR lowering, runtime declarations, and task entry outcomes. Reject every inherited construct not in the closed admission registry and validate transitive package contracts. Add proper scope-exit barriers for every path in §4. Integrate S-Go panic/recover/task-exit behavior (including the audited internal Goexit bridge) and function/open-coded defers. Implement scope finalizers and original panic-payload propagation.

Use explicit experimental checkpoints to test cancellation propagation until managed effects land in W04/W07. Incomplete syntax/exit cases must produce a clear unsupported diagnostic during intermediate commits, not compile into incorrect behavior. Completion of W03 requires the full exit matrix.

**Deliverables:** executable new syntax; positive/negative compiler fixtures; cross-package/generic/inlining tests; exit-order traces; panic/GC stress coverage.

**Gate:** normal brace exit, returns, named results, labeled branches, loops, nested scopes, defers, recovery, and the S-Go task-exit operation behave as specified. Disabling optimization is not the only passing configuration. The stage-0 compiler remains buildable with the isolated host toolchain; no target compatibility mode is introduced.

### W04 — Cancellation protocol, managed waiting, and budgets

**Depends on:** W03. **Risk:** very high.

Implement sticky cancellation, cause propagation, explicit checkpoints, cancellation-aware joins, timer/sleep waiting, finalizer cleanup contexts, budget inheritance, and terminal outcome classification. Implement the atomic register/check/park/unregister/wake protocol with fault hooks at each race window.

Add the host subprocess runner and watchdog. Distinguish virtual deadlines, step quotas, and host hangs. When drain fails, record obligations and terminate without claiming successful closure. The runner must not need the child process to cooperate in order to stop a hung test.

**Deliverables:** cancellation APIs/runtime integration; waiter protocol; budget diagnostics; watchdog runner; adversarial cancellation tests.

**Gate:** cancellation before/during/after registration, readiness, and completion has exactly one outcome, no lost wakeups, and no stale registrations. Cleanup cannot reset budgets or spawn. An infinite-loop fixture ends as a non-success subprocess result, not a silently abandoned task.

### W05 — Capability core, policy enforcement, and readiness

**Depends on:** W04. **Risk:** high.

Implement opaque authority handles, task ceilings, attenuation, revocation, domain checks, and a default-deny supported-package/effect policy. Begin with clocks, test randomness, and simulated effects. Include transitive dependency/import/initialization checks and runtime guardrails. Version the TCB allowlist.

Add a Trio-style readiness operation: a parent may start a child and wait for a one-shot ready notification. Failure/cancellation before readiness wakes the parent; duplicate readiness is a diagnostic. Cancellation cannot return success while leaving an unregistered started child behind.

**Deliverables:** capability facade/enforcement; policy report; fake adapters; readiness handshake; negative capability fixtures.

**Gate:** absence/narrowing/revocation tests deny the effect before it occurs, including hidden dependency entry points. Unsupported unsafe/cgo/assembly routes are rejected, not silently trusted. Ready/failure/cancel races preserve ownership.

### W06 — Modeled and typed reserve/commit/abort channels

**Depends on:** W04. **Risk:** high.

Extend the pure model for buffered channels, unbuffered rendezvous, close, permits, aliases, cancellation, and selection decisions. Enumerate bounded histories. Implement an explicit typed managed-channel API against the same semantic protocol, with runtime-owned obligation accounting.

Start with explicit send permits; model receive-side selection/rendezvous state before exposing public receive permits. Add debug assertions for counts, value ownership, waiter lifecycle, and zero-capacity paths. Add fault injection before/after each terminal transition.

**Deliverables:** channel state model; typed implementation; bounded exploration; permit/cancellation/close tests; linearization notes.

**Gate:** no duplicated/lost published values, capacity leak, unresolved permit after task completion, or successful closed-channel publication contrary to the defined ordering. The implementation and model agree on all bounded reference histories.

### W07 — Native channels, select, and cancellation transfer

**Depends on:** W03, W05, W06. **Risk:** very high.

Integrate the protocol into managed native `chan` operations and all relevant compiler/runtime entry paths. Implement cancellation unwind from safe user boundaries, selection winner/loser cleanup, rendezvous, close, nil-channel behavior, range, reflection, and value evaluation order. No channel path may fall back to raw Go behavior; keep not-yet-implemented native operations rejected until their S-Go contracts are implemented.

Add domain checks and audit interactions with standard synchronization primitives. Reject raw mutex/condition/waitgroup operations without a complete S-Go contract; distinguish specifically admitted production-only nondeterministic operations from operations that lack any valid contract. A supported wrapper must not use an untracked helper goroutine as its cancellation strategy.

**Deliverables:** native syntax integration; trace points; S-Go semantic and memory-order conformance; reflected/optimized/generic cases or explicit rejection where not admitted.

**Gate:** channel/select conformance matches §7; the race detector observes legitimate synchronization and still detects deliberate sibling data races. Forced GC/stack movement while channel operations hold stack-related runtime references does not corrupt values.

### W08 — Deterministic lab scheduler, virtual time, and replay

**Depends on:** W04, W05, W07. **Risk:** very high.

Audit/reuse upstream synctest integration points. Implement a single managed user scheduler, stable IDs/ready sets, PCG test vectors and streams, deterministic timers, safe yield instrumentation, step budgets, canonical trace serialization, manifest validation, replay divergence, and domain isolation.

Implement the nondeterminism inventory as enforceable policy. Include a schedule-independent semantic outcome comparison plus exact canonical event replay. Add a fixture with a deliberate concurrency bug that can be found, captured, and replayed. Do not claim exact replay until repeated independent processes prove it.

**Deliverables:** lab execution mode; trace/replay runner; deterministic fake adapters; seed sweeps; one bounded-exploration regression corpus.

**Gate:** identical manifests reproduce canonical traces across fresh processes; invalid manifests/choices fail clearly; tied timers/selects are controlled; busy loops consume step quota; uninstrumented hangs hit the host watchdog. Unsupported real effects never silently enter the run.

### W09 — Production I/O adapters and truthful progress contracts

**Depends on:** W05, W07. **Risk:** high.

Implement a deliberately small audited production surface: timers, owned connection/listener operations, and confined file operations. Specify close ownership, deadlines, partial progress, cancellation registration, and cleanup. Integrate netpoll where appropriate; isolate unavoidable blocking host work without pretending it is cancellable by dropping an awaiter.

Test against local, controlled fixtures. Check path traversal/symlink cases, network policy changes, redirects or reconnects where supported, shared-handle cancellation, short reads/writes, and cancellation after partial progress.

**Deliverables:** supported adapter matrix; real/fake interface correspondence; partial-progress tests; policy bypass tests.

**Gate:** external effects require valid authority; no-effect failures can be retried safely, partial effects are reported, and closing a resource to cancel one operation does not silently violate another owner's contract. Unsupported filesystem/network operations are rejected in all S-Go programs; documentation alone is not enforcement.

### W10 — Tooling, admission conformance, examples, and performance gates

**Depends on:** W08, W09. **Risk:** high.

Finish mandatory spec/admission/ABI propagation; source formatting and type/tool surfaces; host/target/build-cache separation; S-Go distribution and rejection tests; per-test roots; diagnostic rendering; examples; and supported-target matrix. Ensure generic export/import and source tools do not diverge. Keep editor/LSP support explicitly limited until a corresponding forked tool actually works.

Deliver at least three examples: a scoped worker pipeline; a readiness-gated service with cancellation/cleanup; and a deterministic injected-failure scenario with a saved replay. Each example must assert post-scope quiescence, not just print plausible output.

Benchmark the specified S-Go semantics against comparable workloads on the isolated upstream reference, plus empty nursery cost, task spawn/join, cancellation fanout, channel throughput, peak live tasks, and trace-disabled/enabled cost. Record allocations, RSS versus virtual size, compiler time, and P50/P95 distributions. Separate throughput from semantic correctness; do not remove checks to win a microbenchmark.

**Deliverables:** usable single-language toolchain; examples; source-tool tests; feature/admission report; intentional Go rejection report; benchmark report; guarantee manifest.

**Gate:** a clean documented installation can build/run the examples and reproduce a supplied trace; host/target, spec, ABI, or execution-configuration contamination is rejected; the shipped compiler has no compatibility flags, uncontracted surfaces, or warning-only admission paths.

### W11 — Fixed virtual-stack experiment and adoption decision

**Depends on:** W10. **Risk:** very high. **Merge policy:** experimental until all gates pass.

Investigate the original 8 MiB virtual-stack proposal without changing the core default. Compare growable stacks with a non-moving stack allocation under a separate feature/build identity.

Begin with an accessible demand-paged range and explicit guard layout supported by the OS; retain overflow/preemption checks until a separately validated replacement exists. Do not conflate page residency with accessibility. Verify large-frame behavior, stack clash/guard skipping, panic/unwind/traceback, GC scanning, scheduler preemption, signal/system stacks, and goroutine stack reuse. Keep g0/signal stacks on their upstream implementation initially.

Measure many idle tasks, deep recursion, large frames, cancellation fanout, and real examples. Account for address-space reservation, RSS, mapping count, page-table/commit overhead, and exhaustion behavior. The proposed 8 MiB size is a tunable experiment, not a universal proof of sufficient stack space.

**Deliverables:** architecture note; isolated prototype; correctness/stress results; performance/resource comparison; accept/reject/defer decision with evidence.

**Gate:** no regression in memory safety, preemption, GC, unwinding, or supported behavior. Adoption additionally requires a meaningful measured benefit on representative workloads. A correct negative experiment completes the investigation; it does not justify making a harmful stack design the default.

### W12 — Independent review and release qualification

**Depends on:** W10 and a recorded W11 decision. **Risk:** high.

Run an adversarial review from a fresh context using the specification and actual diff. Verify root paths, lock order, linearization points, failure propagation, exception edges, policy bypasses, replay scope, and all published claims. Re-run a clean bootstrap, mandatory conformance lanes, selected upstream distribution tests, and seeded/production stress. Check failure fixtures actually fail.

Remove every target legacy/permissive path. A remaining compatibility escape is a release blocker and cannot be waived by listing it as a limitation. Audit the TCB and verify the complete spec-to-syntax/API/intrinsic-to-implementation-to-test mapping. Review licensing for any adapted external code. Package the toolchain/source patch series, checksums, build instructions, guarantee manifest, and known limitations.

**Deliverables:** review findings and resolutions; reproducible release evidence; implementation summary; supported/unsupported feature matrix.

**Gate:** no unresolved correctness/security-claim blocker; every claimed capability has traceable evidence. Any remaining limitation is visible in the release's guarantee manifest and user documentation.

---

## 12. Validation strategy and concrete acceptance rules

### 12.1 Test layers

Use the test matrix's stable case IDs. Each scenario becomes an executable test/fixture with a named invariant and expected result. Related edge cases should be table-driven where that improves clarity; do not inflate the suite with duplicated setup or meaningless coverage assertions.

Run pure state-model tests without runtime nondeterminism. Run compiler positive/negative fixtures through the fork. Run runtime tests in isolated processes when panic, Goexit, fatal termination, or watchdog behavior is expected. Use ordinary production scheduling and supported race instrumentation in addition to lab mode.

Use a separately built, untouched upstream reference lane only for engineering comparisons. S-Go conformance runs in production and lab configurations with identical mandatory safety/admission rules; both include legacy-rejection and unknown-contract tests. A test passed only under lab scheduling does not establish production concurrency correctness.

### 12.2 Initial workload counts

These are proposed acceptance workloads, not measured results or a completeness proof:

- For critical lab fixtures, run seeds `0..999`; retain every failure trace. Run a smaller deterministic subset in routine edits and the full set at integration gates.
- Re-run a representative set of at least ten recorded manifests ten times each in fresh processes; compare canonical traces and semantic outcomes.
- Enumerate small channel/ownership models with capacities `0, 1, 2`, up to three tasks, and a recorded action-depth bound. Report explored/pruned histories and bound, not “all interleavings” without qualification.
- Run GC/stack-movement/production-scheduling stress with enough repetitions to exercise reuse and race windows; retain the exact configuration and counts.

W00/W01 may tune execution counts for the available machine, but reductions must be recorded. Required scenarios and invariants cannot be dropped merely to get a green run.

### 12.3 Release blockers

Any orphan on purported successful closure, missing GC root, use-after-recycle, duplicate publication, capacity leak, lost wakeup, unreported partial I/O, incorrect return/defer behavior, silent policy bypass, or replay fallback is a blocker.

Expected rejection is a passing outcome only for features explicitly not admitted by that S-Go specification/build or by the lab replay contract. An unsupported error for a required nursery exit, native channel operation, or promised adapter is not a passing implementation of that feature.

Non-cooperative cancellation passes only when the test records a contract violation and terminates the isolated process. It must not resume useful application execution with the abandoned task hidden from accounting.

### 12.4 Performance decisions

Do not invent benchmark results or set a success target based on the old whitepaper's rhetoric. W00 records a baseline; W10 compares equivalent workloads and repeated samples. Evaluate costs against the S-Go workload goals and a comparable isolated reference. There is no Go-compatibility overhead target or requirement to preserve behavior merely to keep an upstream benchmark green. Never weaken a contract to improve a benchmark, and report semantic differences that make a comparison non-equivalent.

For fixed-stack adoption, predeclare the representative workloads and minimum worthwhile gain before running the comparison. A speedup that costs unacceptable task density, virtual mappings, or correctness is a rejection. Keep the default growable-stack implementation until the experiment earns a change.

---

## 13. Agent workflow, evidence, and resumption

### 13.1 One work package at a time

At the start of a package, read the actual relevant source and run the closest baseline test. State the small set of invariants touched. Add a failing test or an executable model case for the behavior, implement the change, then run focused tests plus relevant regression lanes.

Commit or produce a coherent diff only after inspecting it for unintended files/generated changes. Do not create many unrelated half-implemented subsystems in one session. A designated reviewer may examine a finished patch/model in parallel, but overlapping edits to runtime scheduling, compiler IR, or ABI files need a single integration owner.

Correctness reviews for GC roots, unwind semantics, channel linearization, and capability enforcement are explicit gates. A model agreeing with its own generated implementation is insufficient; include independent mutation/adversarial cases.

### 13.2 Evidence record

For each work package, store an evidence index with:

```text
work_package_id, state, spec_version, fork_commit, upstream_commit,
host_and_toolchain, spec_digest, admission_digest, execution_configuration, exact_commands, exit_codes,
test_case_ids, passed_failed_skipped_unrun, skip_reasons,
trace_and_log_paths, artifact_digests, benchmark_configuration,
changed_invariants, known_limitations, next_dependency_ready_task
```

Test output must come from execution. A proposed command belongs in `planned_commands`, never in `executed_commands`. An environmental failure and an implementation failure must remain distinguishable. Store large logs outside version control when appropriate, but keep a durable path/hash index.

Use statuses `pending`, `in_progress`, `blocked`, `failed`, and `passed`. W11 may also end `investigated_not_adopted` with evidence. There is no status that translates “code was written” into “tests passed.”

### 13.3 Session checkpoint

At the end of a session, write a compact checkpoint containing current branch/commit, dirty files, completed tests, failed/unrun tests, current blocker, relevant architectural decisions, and the next exact command or edit. The next agent reads this checkpoint before regenerating a plan.

When blocked by unavailable tools, network, or compute, complete independent specification/model work that remains valid, report the blocker precisely, and do not substitute invented build evidence. Do not ask the user to decide implementation details already resolved by this plan.

### 13.4 Changes to this plan

A justified amendment needs: the conflicting source/evidence, the invariant affected, a replacement rule, effects on existing S-Go guarantees, updated tests/admission entries, and S-Go version-transition notes. Go ecosystem breakage is not a veto. Changing the constitution requires an explicit user decision. Small implementation details do not require a new architecture memo. Major changes—such as implicit process-lifetime ownership, uncatchable cancellation, or stack-only scope records—cannot be slipped into a patch without an explicit decision.

---

## 14. Explicit prohibitions against implementation drift

Do not:

- Add `compat`, an opt-in safety dialect, an unsafe switch, a legacy parser/import/link path, or automatic stock-Go fallback.
- Accept any syntax/export/intrinsic solely because the inherited Go compiler accepts it, or attach unchecked contract annotations to an otherwise unported dependency.
- Retain bare `go`, infer a missing spawn capability from ambient state, or silently translate legacy spawning.
- Exempt target runtime/library code from specification because it is trusted or historically came from Go.
- Replace the fork deliverable with an `errgroup` wrapper, callback nursery, parser-only demo, or disconnected custom scheduler.
- Apply the old fixed-stack/IR snippets without verifying the actual pinned source.
- Use an enclosing-function defer as the only nursery-exit mechanism, or a hidden closure as the final general lowering.
- Hide live pointers in `uintptr`, publish movable stack records to children, or suppress required write barriers.
- Remove stack/preemption checks or stop a goroutine arbitrarily to satisfy a timeout test.
- Fix ownership tests by moving application tasks into an immortal root or by ignoring hidden dependency goroutines.
- Treat a seed, one OS thread, or `GOMAXPROCS=1` as sufficient evidence of deterministic replay.
- Let finalizers silently resolve the correctness of abandoned permits, or report cancelled operations as no-effect after publication.
- Whitelist broad effectful packages merely to get the demo to compile, or advertise capability passing as complete hostile-code isolation.
- Mark tests passed from static inspection, suppress failures with broad skips, or publish stronger guarantees than `GUARANTEES.json` and evidence support.

---

## 15. Definition of done and first execution target

### 15.1 First vertical slice

The admitted language for this slice is subset 0, defined by `SGO_DECISIONS.md` and specified in `SGO_SPEC_SUBSET0.md`. The first meaningful delivery is a small, closed, specification-admitted language subset in W00–W04: a reproducibly bootstrapped fork that executes nursery syntax, accounts for transitive children, joins on all required exits, survives GC/stack movement, propagates child failures, and cancels supported waits without orphaning tasks.

This slice must run at least: normal nested join; a helper-spawned child using an explicit nursery capability; rejection of the same helper using bare `go`; rejection of missing/dependency-stale contracts; parent return with a named result; outward labeled control flow; child panic with sibling cleanup; and a non-cooperative child that yields a failed subprocess instead of false successful closure.

Do not start fixed-stack work to postpone those semantics. Do not stop after the first vertical slice and claim the four pillars are complete; continue through dependency-ready packages.

### 15.2 Core release

The core release requires the first slice plus native managed channels/select, capability enforcement for the advertised surface, deterministic simulated-effect replay, production adapters for the advertised surface, tooling/build identity, executable examples, and independent review.

The fixed-stack investigation must have a recorded decision. Adoption is not required when measurements or correctness reject it; the release must clearly state which stack implementation it uses.

A user should be able to answer from the release artifacts:

**What can I compile? Which operations are cancellation-aware? Which effects are mediated? What exactly can I replay? What is rejected? What happens on a stuck task? Which tests demonstrated each answer?**

---

## 16. Primary references and provenance

References are carried forward from the v1 handoff as upstream implementation/design material. They are not normative S-Go clauses and do not certify the implementation. This v2 correction did not re-verify those external sources. W00 must verify source availability, exact revisions, build requirements, and actual locations; W01 must separately freeze any clauses intentionally adopted into S-Go.

**R1 — Go release history.** Selected release and support context.  
`https://go.dev/doc/devel/release`

**R2 — Go compiler overview, go1.27.1.** Pipeline and Unified IR.  
`https://raw.githubusercontent.com/golang/go/go1.27.1/src/cmd/compile/README.md`

**R3 — x86 object/prologue generation, go1.27.1.** `stacksplit` implementation location.  
`https://raw.githubusercontent.com/golang/go/go1.27.1/src/cmd/internal/obj/x86/obj6.go`

**R4 — Go runtime stack source.** Stack guard/preemption relationship; verify the pinned counterpart in W00.  
`https://go.dev/src/runtime/stack.go`

**R5 — Runtime memory-state API, go1.27.1.** `sysAlloc`, reservation/mapping contracts.  
`https://raw.githubusercontent.com/golang/go/go1.27.1/src/runtime/mem.go`

**R6 — Go language specification.** Defer, control flow, evaluation, panic/recover, channels/select.  
`https://go.dev/ref/spec`

**R7 — Runtime contribution/invariant documentation, go1.27.1.** Runtime execution, allocation, write-barrier, stack, and trace constraints.  
`https://raw.githubusercontent.com/golang/go/go1.27.1/src/runtime/HACKING.md`

**R8 — Unified IR implementation, go1.27.1.** Writer and reader.  
`https://raw.githubusercontent.com/golang/go/go1.27.1/src/cmd/compile/internal/noder/writer.go`  
`https://raw.githubusercontent.com/golang/go/go1.27.1/src/cmd/compile/internal/noder/reader.go`

**R9 — Go memory model.** Goroutine creation/completion and channel synchronization.  
`https://go.dev/ref/mem`

**R10 — Native channels and select, go1.27.1.** Implementation reference, not a compatibility obligation.  
`https://raw.githubusercontent.com/golang/go/go1.27.1/src/runtime/chan.go`  
`https://raw.githubusercontent.com/golang/go/go1.27.1/src/runtime/select.go`

**R11 — Upstream virtual-time testing.** Bubble ownership, durable blocking, isolation, and runtime integration.  
`https://pkg.go.dev/testing/synctest`  
`https://raw.githubusercontent.com/golang/go/go1.27.1/src/runtime/synctest.go`

**R12 — PCG reference documentation.** Generator behavior and reference implementation.  
`https://www.pcg-random.org/using-pcg-c-basic.html`

**R13 — asupersync project source/documentation.** Inspiration for owned scopes, explicit capability contexts, cancellation protocols, and reserve/commit outcomes. Also the actual origin of the reserve/commit and deterministic-lab ideas the input archive attributed to "BlenD2". Its README states that production cleanup budgets are advisory and a non-cooperative task is never forcibly stopped; S-Go's stronger fail-stop/budget contract is this plan's own design, not inherited from R13. Review individual files and licenses before copying code; do not assume Rust RAII semantics transfer to Go.  
`https://github.com/Dicklesworthstone/asupersync`

**R14 — Building Go from source.** Bootstrap compiler and build procedure.  
`https://go.dev/doc/install/source`

**R15 — Private scanner/tokens, go1.27.1.** Actual keyword table/generation context.  
`https://raw.githubusercontent.com/golang/go/go1.27.1/src/cmd/compile/internal/syntax/scanner.go`  
`https://raw.githubusercontent.com/golang/go/go1.27.1/src/cmd/compile/internal/syntax/tokens.go`

**R16 — Runtime task creation, go1.27.1.** Central `newproc`/`newproc1` integration and classification reference.  
`https://raw.githubusercontent.com/golang/go/go1.27.1/src/runtime/proc.go`

**R17 — Structured concurrency rationale, Nathaniel J. Smith.** Nursery-style lifetime design inspiration; not an S-Go implementation.  
`https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/`

**R18 — Bend 2 language.** Most likely origin of the input archive's "BlenD2" name (see §2 provenance correction). Pure, strongly typed, fork/join-parallel language whose compiler demands proofs of declared laws (`LAWS.bend`) before accepting code. Relevant only as an analogue of S-Go's default-deny specification admission; it is not a source for the reserve/commit contract, the lab scheduler, or any S-Go normative clause.  
`https://github.com/bendlang/bend`

**Input archive SHA-256:**  
`b6672dbf5fef2839c6bb4779f92c9b0fd0d6151a47cfe08aad18ed040d8996bf`

**Important verification limit:** v1 reported primary-source web inspection without building a full pinned checkout. This v2 revision changes and validates planning artifacts only; it did not re-verify those external references, implement a compiler, or execute conformance tests. W00 must resolve the exact commit, audit the checkout, and execute the baseline. No source commit SHA or implementation test result is supplied as if verified.
