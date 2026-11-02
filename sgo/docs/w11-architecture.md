# W11 architecture note — fixed virtual stacks (`vstack`), investigation only

Decisions D70–D74 (approved 2026-11-02). Plan §11/W11; matrix F01–F08. Nothing here changes
the default runtime: `vstack` is a separate execution configuration (D71), and adopting it
would be a separate decision (D74) with a spec amendment.

## 1. The baseline: growable, moving stacks

| Mechanism | Where | What `vstack` must do with it |
|---|---|---|
| A new task starts on `startingStackSize` (2 KiB, then the GC-tracked average) from the stack pools | `malg`, `newproc1`, `stackalloc` (`stack.go:344`) | Allocate one fixed reservation instead (§2) |
| The function prologue compares SP with `g.stackguard0`; on failure `morestack` → `newstack` | `newstack` (`stack.go:1055`) | Keep the check. Growth becomes a defined exhaustion; a preemption request (`stackPreempt`) keeps its path unchanged |
| Growth doubles the stack and copies it, adjusting stack pointers | `copystack` (`stack.go:929`) | Never called for a `vstack` task |
| The GC shrinks stacks that use under a quarter | `shrinkstack` (`stack.go:1290`) | Never shrinks a `vstack` task (`isShrinkStackSafe` false for it) |
| Dead tasks' stacks are cached with their `g` if they have the starting size | `gfput`/`gfget` (`proc.go:5520`) | Keep the reservation with the `g`, released to the OS first (§4, F07) |
| The pacer counts stack bytes as scannable work: `hi - lo` per stack | `addScannableStack` (`mgcpacer.go:974`, `proc.go:4523`, `5423`) | **Must not** count the reservation: 10,000 tasks × 8 MiB would add 80 GB of "scannable" stack, raising the heap goal until the GC effectively stops. Count the touched size instead (§5) |
| Maximum stack 1 GB on 64-bit (`maxstacksize`, `proc.go:164`); exceeding it is `ResourceExhausted`, exit 4 (`err-stack-overflow-exit-4`) | `newstack` | The maximum becomes the reservation (8 MiB, tunable). Any program needing between 8 MiB and 1 GB of stack regresses; W11 records this as a supported-behaviour difference (gate) |

SGO-STACK-001 states "each task runs on a growable stack". `vstack` deviates from it. That is
acceptable for an experimental configuration, but it must be recorded, and adoption would need
an amendment to the clause.

## 2. Layout of one reservation (F01)

```
 lo - G                 lo                                   hi
  | guard (PROT_NONE) |  usable stack, demand-paged (RW)  ...  |
  +-------------------+-------------------------------------------+
                       ^ g.stack.lo          frames grow down ^ g.stack.hi
```

- One `mmap` per task of `size + G` bytes, `MAP_PRIVATE|MAP_ANON` (`MAP_NORESERVE` on linux),
  then `mprotect(PROT_NONE)` on the low `G` bytes. Pages become resident only when touched;
  **accessible is not the same as resident** (the plan's warning), and the probe measures both.
- `G` is at least 64 KiB, larger than any prologue's maximum frame plus `stackGuard`. The
  prologue check still guards every frame. The guard page is defence in depth, for NOSPLIT
  chains and for frames the compiler probes (F02).
- The reservation is outside the heap arenas, like g0 stacks on some OSes. `findObject` and
  `spanOf` must treat it as non-heap. The probe and F07 check that no heap metadata refers to it.

## 3. Overflow and preemption (F02, F03)

- `stackguard0 = lo + stackGuard`, as today. Exceeding it calls `newstack`. For a `vstack` task,
  `newstack` either serves a preemption request exactly as today, or fails the task with
  `ResourceExhausted` (exit 4), the same record as the baseline's overflow.
- Preemption (async signals, and `stackPreempt` in `stackguard0`) is unchanged. Loops keep their
  safepoints, because the compiler is identical; only the runtime differs.

## 4. Reuse (F07)

A freed `vstack` stays mapped with its `g` for reuse, but its pages are released first:
`madvise(MADV_DONTNEED)` on linux (the next touch reads zero pages) and `madvise(MADV_FREE_REUSABLE)`
on darwin. A recycled task therefore never sees an earlier task's data.

## 5. GC and accounting

- Stack scanning already walks from SP to `hi`, so its cost is proportional to the touched depth,
  not the reservation.
- `scannableStackSize` counts `hi - lowest SP observed`, rounded to pages, updated at GC. It is
  never the reservation. `memstats.stacks_inuse` reports resident bytes where the OS gives them
  (`mincore` is too slow per GC; the probe measures RSS instead).
- `runtime.MemStats` is not visible to S-Go programs. These numbers matter only for the pacer
  and for the measurements.

## 6. Untouched (F05)

g0 and signal stacks (`mstart`, `minit`, `sigaltstack`), and the lab, keep the upstream paths.
`vstack` changes only user task stacks.

## 7. Platform limits (F06, D73)

| Host | Default | Consequence for one 8 MiB + guard reservation per task |
|---|---|---|
| linux, stock `vm.max_map_count` 65530 | 2 mappings per task (guard + usable) | about 32,000 live tasks, minus the program's own mappings, before `mmap` fails |
| linux, note2 1048576 | 2 per task | about 500,000 tasks; virtual space (8 MiB × N) and overcommit then bind |
| linux overcommit 0 (heuristic) | `MAP_NORESERVE` reservations are not charged | large N admitted; RSS grows only with touched pages |
| darwin arm64, 16 KiB pages | no tunable map count; the kernel's VM map entries and the 64 GB default user VA limit | measured, not assumed |

Exhaustion must end the program with a defined failure (`ResourceExhausted` naming the refused
reservation), never a fault in unrelated memory.

## 8. Order of work

1. **Probe (no runtime change).** A host program builds the §2 layout N times and records virtual
   size, RSS, mappings, time, and the failure at exhaustion, on darwin and on note2 at both map
   limits. It answers F01 and most of F06 before any runtime work.
2. **Prototype** behind `vstack` (D71): `malg`/`stackalloc`, `newstack`, `shrinkstack`,
   `gfput`/`gfget`, scannable accounting; identity, TCB section, linker refusal, driver flag.
3. **Correctness:** F01–F05 and F07 as runtime-lane tests and conformance cases built with
   `-vstack`, on all three targets.
4. **Measurement** per D70, with the baseline on the same hosts and the same pinning.
5. **Decision** per D74.
