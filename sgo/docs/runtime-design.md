# Runtime design: scope core and sealed services (W02)

Implements trusted primitives TCB-P01 (task creation), TCB-P02 (park/unpark), TCB-P03 (scope state and join), and TCB-P11 (runtime services) of SGO-TCB-001. Code: `src/runtime/sgo_scope.go`, `src/runtime/sgo_service.go`; small hooks in `proc.go` (`newproc`, `gdestroy`) and `runtime2.go` (`g.sgo`, `g.sgoService`, `waitReasonSGoJoin`). Tests: `src/runtime/sgo_test.go`, hooks in `export_sgo_test.go` (compiled only into runtime tests), run in the runtime test lane (`sgo/scripts/runtime-lane.sh`).

## 1. Records

| Record | Fields that matter | Created | Removed |
|---|---|---|---|
| `sgoTask` | `id` (creation ID, root = 0), `scope` (owner scope), `nest` (nurseries it is executing inside), `sibling`, `fn` (callback until it starts), outcome | root: `sgoEnterRoot` (W03: runtime start-up); child: `sgoSpawn` before the child exists | after its outcome is published and its scope closes and is unreferenced |
| `sgoScope` | `id`, `state` (SGO-NUR-002), `owner`, `parent`, `live`, `first/last` child list, `joiner`, `snapshot` | `sgoNurseryOpen` | after CLOSED and once no handle refers to it |

## 2. GC roots and stack movement (plan §5.3)

For each pointer-bearing structure:

1. **What keeps it alive, and what scans it?** A task record is reachable from `g.sgo` of its goroutine (every `g` is a heap object reachable from `allgs`, scanned as an ordinary typed object) and from its scope's child list. A scope is reachable from its owner's `nest` while open and from each child's `scope`. Callbacks and captured values are reachable from `sgoTask.fn` until the child starts, then from the child goroutine's stack. No new GC root, root-scanning code, or non-scanned storage was added: every reference is an ordinary heap pointer in a typed object.
2. **Which writes need barriers, and where?** All writes are ordinary Go assignments in non-`nowritebarrier` runtime code, so the compiler emits write barriers. The only write on the system stack is `newg.sgo = c` inside `systemstack` in `sgoSpawn`, the same context in which upstream `newproc1` writes other pointer fields of `newg`; write barriers are permitted there.
3. **When are they removed, and what prevents use after recycling?** A child stops being live when it publishes its outcome (`sgoTaskExit`), which also clears `getg().sgo`; `gdestroy` clears `g.sgo` and `g.sgoService` again before any goroutine reuse, so a reused `g` never carries a stale record (O06). At CLOSED a scope drops its child list (`first`, `last`) and keeps only an immutable `snapshot`, so a retained handle does not retain children, their callbacks, or their payloads (O07).
4. **Can it point into a goroutine stack?** No. Records are heap-allocated and hold only heap pointers; nothing stores the address of a stack slot. Stack growth, copying, and shrinking therefore need no S-Go handling (O08). Exception-unwind markers for nursery frames are compiler work (W03) and must follow the same rule.

## 3. Registration before publication (OWN-2) and allocation safety

`sgoSpawn` performs, in order: argument and membership checks (no lock); allocation of the child record (no lock held); under the scope lock, the state check, ID assignment, `live++`, and linking into the child list (no allocation: intrusive list); unlock; then on the system stack `newproc1`, `newg.sgo = c`, `runqput`. The child therefore counts as live before it can run, and no allocation happens while a runtime lock is held. Children start at the static function `sgoTaskStart` (static funcval; the runtime forbids heap closures) and find their callback through `getg().sgo`.

## 4. Join synchronization

The owner sets `s.joiner` and parks with `goparkunlock(&s.lock, waitReasonSGoJoin, …)`, which releases the lock only after the goroutine is parked; the last child decrements `live` under the same lock and, if it sees a parked joiner, readies it **after** unlocking. A lost wakeup is impossible because the joiner registration and the decrement are serialized by the lock and the park/unlock handshake is atomic (plan §6.2). The owner rechecks `live` after waking. For the race detector, each child does `racereleasemerge(scope)` before publishing and the owner does `raceacquire(scope)` after the last wakeup; without these annotations `-race` reports a false join race (checked).

## 5. Lock order

The scope lock is a leaf: it is never held while calling user code, allocating, acquiring another scope lock, or parking (except through `goparkunlock`, which releases it). W04 kept that property instead of taking scope locks along a scope chain (§9 explains how propagation avoids it), so the complete order is: scope lock → stdout lock (only in the drain-budget decision, which never releases the stdout lock); `sgoSleepLock` alone (runtime timer locks are taken only after it is released, and timer callbacks run with no timer lock held, `unlockAndRun`). The graph is acyclic and no static rank is needed; the locks use the runtime's default rank.

## 6. Sealed runtime services (OWN-6, TCB-P11)

Every goroutine created through `newproc` (the `go`-statement path) is classified by its creating function: the innermost inlined function at `callerpc - 1` (the call instruction; `callerpc` itself can belong to an inlined call that follows, which the census test caught). Only the eleven creation sites enumerated in `sgo/TARGET_TCB.json` map to service kinds; the kind is stored in the unexported `g.sgoService` with per-kind started/live counters. With `sgoEnforce` on, any other creation is a fail-stop `RuntimeDefect` naming the creator. Names include the package path (`runtime.…`), and only digest-pinned runtime sources can define runtime functions, so user or dependency code cannot spoof a service (tested with a `gcenable` in another package). S-Go tasks are created only by `sgoSpawn` through `newproc1` directly and are never services. W03 turns enforcement on at start-up of every S-Go program; the runtime test lane leaves it off because its tests are Go code.

## 7. Ownership diagnostics

Violations return `SpawnContractViolation: <reason>` naming the failed check (nil handle, nil callback, non-task caller, not the caller's current scope, state). Internal invariant violations throw with a specific `sgo:` message (closed scope with a running child, negative live count, task exited with an open nursery, service created outside the enumeration with its creator), which W03 maps to the `RuntimeDefect` diagnostic record (SGO-ERR-002/003).

## 8. Nursery statement and unwinding barrier (W03)

Code: `src/runtime/sgo_nursery.go`, one hook in `panic.go` (`(*_panic).nextFrame`); tests: `src/runtime/sgo_nursery_test.go`. The compiler lowers `nursery (n) { body }` to `s := sgoNurseryEnter(); n := &sgo.Nursery{s}; body; sgoNurseryExit(s)`, and emits `sgoNurseryExit` for every `return`/`break`/`continue` that leaves the statement, innermost first, after the return operands are held (SGO-CTL-004). `sgoNurseryExit` joins and applies SGO-NUR-004 rules 2–4 (a failed child's payload is unwound as is; an errored child unwinds as `ChildError`).

Unwinding does not run that code. `sgoNurseryEnter` records `stack.hi − SP` of the executing frame in `sgoScope.frame` (an offset, so stack copying does not invalidate it, §2.4). After `nextFrame` finds the next frame with deferred calls, and only for real unwinding (not `deferreturn`), `sgoUnwindNurseries` joins the task's open nurseries whose offset is at least that frame's offset (that frame and the frames already left), innermost first; when no frame is left it joins all of them. So in every frame the active nurseries complete before the frame's deferred calls run (SGO-DEF-005), and outer frames' nurseries stay open until unwinding reaches them. The join cannot raise during unwinding (rule 1: the unwinding failure stays primary). Not yet: cancelling the scope before the unwinding join (W04), and recording non-primary child failures in the diagnostic chain (SGO-ERR-002, W03 diagnostics).

## 9. Cancellation, Sleep, and the drain budget (W04)

Code: `src/runtime/sgo_cancel.go` (TCB-P04, TCB-P05); hooks in `sgoTaskExit` (ChildFailed), `sgoUnwindNurseries` (Unwind), `sgoSpawn` (R5), the join (drain timer stop); tests: `src/runtime/sgo_cancel_test.go`.

*Own transitions.* `n.Cancel()`, a child ending Errored/Failed (before its outcome is published), and the owner's body unwinding through the nursery move the scope OPEN/JOINING → CANCELLING under its lock and record the cause; later requests have no effect. The transition stores the next value of a process-wide sequence in `s.cancelSeq` (atomic; read without the lock).

*Propagation is computed.* A scope's `parent` is the owner's current scope when it was opened, so a task's linked scopes (its current scope up to its spawn scope) and everything that transitively cancels them lie on one parent chain. A scope is CANCELLING in effect if it or a scope above it on the chain cancelled on its own (cause Ancestor otherwise). Nothing is written into inner scopes, so no second scope lock is ever taken; inner cancellation never reaches outer scopes and stops affecting a task once its nursery closes and leaves the chain; nurseries opened inside a cancelled scope are cancelling in effect and admit no child (R5). The cause a task reports is that of the outermost linked scope cancelled in effect: its own cause if its own transition has the smallest sequence number on the chain from it upward, else "ancestor scope <id> cancelled" for the scope that has.

*Sleep.* A sleeping task registers a fresh record in `sgoSleepers` under `sgoSleepLock`, after re-checking cancellation under that lock, and parks with an unlock function that releases the lock and only then arms the record's timer (so the timer can never ready a goroutine that is not parked). A canceller makes its transition visible before scanning the registry under the same lock, so a wake-up cannot be lost. The timer and a canceller race on the record's state word; exactly one wins and readies the goroutine. A stale timer of a finished record fails that CAS and does nothing; records are never reused.

*Drain budget.* `sgo build -drain-budget=<ms>` reaches the runtime through the linker's dedicated `-sgodrainbudget` flag (it sets `runtime.sgoDrainBudget`; `-X` stays refused) and is recorded in the build ID. Each own transition starts a monotonic timer; if the scope has not reached FINALIZING when it fires, the runtime takes the stdout lock (a write in progress completes, none starts), writes one `DrainBudgetExceeded` record without allocating, and exits with status 3. Scopes cancelled only in effect need no timer: their ancestor cannot finalize before they close.

## 10. Not yet

Lab scheduling (W08). Cleanup contexts with quotas, scope finalizers, readiness handshakes, permits, `recover`, and `sgo.ExitTask` are denied by spec 0.1.0.
