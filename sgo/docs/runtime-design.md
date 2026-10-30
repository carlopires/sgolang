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

The scope lock is a leaf: it is never held while calling user code, allocating, acquiring another scope lock, or parking (except through `goparkunlock`, which releases it). It currently uses the runtime's default (unranked) lock rank; when cancellation (W04) needs to take scope locks along a scope chain, the order will be parent before child and a static lock rank will be added to `mklockrank.go`.

## 6. Sealed runtime services (OWN-6, TCB-P11)

Every goroutine created through `newproc` (the `go`-statement path) is classified by its creating function: the innermost inlined function at `callerpc - 1` (the call instruction; `callerpc` itself can belong to an inlined call that follows, which the census test caught). Only the eleven creation sites enumerated in `sgo/TARGET_TCB.json` map to service kinds; the kind is stored in the unexported `g.sgoService` with per-kind started/live counters. With `sgoEnforce` on, any other creation is a fail-stop `RuntimeDefect` naming the creator. Names include the package path (`runtime.…`), and only digest-pinned runtime sources can define runtime functions, so user or dependency code cannot spoof a service (tested with a `gcenable` in another package). S-Go tasks are created only by `sgoSpawn` through `newproc1` directly and are never services. W03 turns enforcement on at start-up of every S-Go program; the runtime test lane leaves it off because its tests are Go code.

## 7. Ownership diagnostics

Violations return `SpawnContractViolation: <reason>` naming the failed check (nil handle, nil callback, non-task caller, not the caller's current scope, state). Internal invariant violations throw with a specific `sgo:` message (closed scope with a running child, negative live count, task exited with an open nursery, service created outside the enumeration with its creator), which W03 maps to the `RuntimeDefect` diagnostic record (SGO-ERR-002/003).

## 8. Nursery statement and unwinding barrier (W03)

Code: `src/runtime/sgo_nursery.go`, one hook in `panic.go` (`(*_panic).nextFrame`); tests: `src/runtime/sgo_nursery_test.go`. The compiler lowers `nursery (n) { body }` to `s := sgoNurseryEnter(); n := &sgo.Nursery{s}; body; sgoNurseryExit(s)`, and emits `sgoNurseryExit` for every `return`/`break`/`continue` that leaves the statement, innermost first, after the return operands are held (SGO-CTL-004). `sgoNurseryExit` joins and applies SGO-NUR-004 rules 2–4 (a failed child's payload is unwound as is; an errored child unwinds as `ChildError`).

Unwinding does not run that code. `sgoNurseryEnter` records `stack.hi − SP` of the executing frame in `sgoScope.frame` (an offset, so stack copying does not invalidate it, §2.4). After `nextFrame` finds the next frame with deferred calls, and only for real unwinding (not `deferreturn`), `sgoUnwindNurseries` joins the task's open nurseries whose offset is at least that frame's offset (that frame and the frames already left), innermost first; when no frame is left it joins all of them. So in every frame the active nurseries complete before the frame's deferred calls run (SGO-DEF-005), and outer frames' nurseries stay open until unwinding reaches them. The join cannot raise during unwinding (rule 1: the unwinding failure stays primary). Not yet: cancelling the scope before the unwinding join (W04), and recording non-primary child failures in the diagnostic chain (SGO-ERR-002, W03 diagnostics).

## 9. Not in W02

Cancellation (W04), owner-unwinding barriers and nursery syntax (W03), the public `sgo` package (W03), lab scheduling (W08). `sgoIsCancelled` returns false until W04 creates cancellation errors.
