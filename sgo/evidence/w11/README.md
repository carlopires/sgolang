# W11 evidence: fixed virtual stacks (vstack)

**Outcome: `investigated_not_adopted`** (D74; user decision 2026-11-02, "ok, option A").
W11 closed at the probe stage. No `vstack` runtime prototype was built. The default
runtime keeps upstream growable stacks (SGO-STACK-001), and no runtime, compiler, spec,
or TCB file changed.

Decision rule: D70 (preregistered). Design: `sgo/docs/w11-architecture.md`.
Probe: `sgo/w11/probe/main.go` (host tooling built with the pinned Go, no runtime change).
CI probe workflow: `.github/workflows/sgo-w11-probe.yml`, run 41444689371 (both jobs success).

## Why the experiment existed

The original proposal (`/Users/carlopires/ycode/sgo/original/`) made fixed stacks
**phase 1, a prerequisite** for everything else:

- `SGO_MASTER_BLUEPRINT.md`: "Strip Relocatable Stacks. Change `src/runtime` to use fixed
  8MB virtual stacks and remove `LoweredStackCheck` from the SSA backend."
- `SGO_CONCEPTUAL_WHITEPAPER.md` §4: "Go's dynamic stack-splitting (morestack) requires
  runtime support that prevents implementing true zero-allocation scope boundaries."
- `docs/PHASE3_DESUGARING_IR.md`: the nursery record (`NurseryScope`) was a local on the
  parent's stack, and children would hold pointers to it. Moving stacks would break those
  pointers.
- `docs/PHASE1_FIXED_STACKS.md`: the sketch removed the prologue checks and treated
  `sysAlloc` as a `PROT_NONE` reservation.

The execution plan (§2) audited this. It rejected fixed stacks as a prerequisite ("without
evidence"). It kept the stack checks, because they also carry preemption requests, and it
put scope records on the heap ("no published pointers into movable parent stacks"). W03–W10
then delivered nurseries, cancellation, channels, the lab, and I/O on growable stacks.
That removed the correctness motivation and left W11 as a performance experiment
(plan §12.4: "Keep the default growable-stack implementation until the experiment earns
a change").

## Probe results

The probe builds the proposed layout directly: one `MAP_NORESERVE` 8 MiB reservation per
task, a 64 KiB `PROT_NONE` guard below `lo`, and one touched page. These numbers are
the **floor** of the layout's cost. A runtime implementation can only add to them.

| Check | darwin/arm64 (local, 16 KiB pages) | linux/amd64 (note2) | linux/arm64 (CI) | linux/amd64 (CI) |
|---|---|---|---|---|
| F02 guard: write below `lo` faults | ✅ | ✅ `SEGV_ACCERR` | ✅ | ✅ |
| F07 reuse via madvise zeroes pages | ❌ `MADV_FREE_REUSABLE`: 512/512 pages keep old data | ✅ `MADV_DONTNEED`, 0 stale | ✅ 0 stale | ✅ 0 stale |
| F06 RSS per idle task | ~16.6 KiB (n=20000) | 4290 B (n=512000) | ~4.3 KiB | ~4.3 KiB |
| F06 page tables per idle task | not measured | ~4.03 KiB (VmPTE), not in RSS | not measured | not measured |
| Reserve+guard+touch per task | ~6 µs | ~5.1 µs | ~6.1 µs | ~11.8 µs |
| F06 capacity / failure | 2,000,000 admitted, no exhaustion | 32,749 @ 65530; 524,268 @ 1048576 | 131,057 @ 262144 | 131,057 @ 262144 |
| Failure mode | — | `mprotect: cannot allocate memory` | same | same |

Files: `probe-darwin-arm64.jsonl`, `probe-linux-amd64.jsonl` (note2, both map limits),
`probe-linux-arm64.jsonl` and `probe-linux-amd64-ci.jsonl` (GitHub runners).

Notes:
- The darwin density curve isn't monotonic at large n (5.10 GB RSS at 1,024,000, then
  2.49 GB at 2,000,000), because macOS compresses or purges idle pages under pressure.
  Only the n ≤ 20000 reading is used.
- On linux the map limit fails in the guard `mprotect` (the second VMA), and it is
  process-wide: any later runtime `mmap` would also fail. A real runtime would need a
  reservation cap well below `max_map_count/2`.
- `"signal":"signal -1"` in `guard` output is harmless: the child exited with Go's
  fault report instead of being killed by the signal.

## Comparison with the W10 baseline (`sgo/evidence/w10/bench-*.json`)

| Platform | Baseline per live task | vstack floor per idle task | Baseline spawn+join | vstack reserve alone |
|---|---|---|---|---|
| darwin/arm64 | ~2.9 KB | ≥16.6 KiB (**~5.8x**) | 0.50 µs | ~6 µs |
| linux/amd64 | ~1.9 KB | ~8.2 KiB incl. page tables (**~4.4x**) | 0.64 µs | ~5.1 µs |
| linux/arm64 | ~2.0 KB | ≥4.3 KiB RSS, plus page tables (**≥2.1x**) | 0.86 µs | ~6.1 µs |

(Baseline per task = (`livetasks` − `emptynursery`) P50 maxrss / 10000.)

## Decision against D70 (F08)

D70 adopts only if P50 RSS or wall time on idle/example workloads improves by ≥20%, with
no other row more than 5% worse, and every correctness case passes on all three targets.

- **Idle-task RSS:** the layout's floor is 2–6x *worse* than the baseline on every
  target. The RSS criterion can't be met.
- **Wall time:** creating a fresh stack costs ~6–12x the baseline's whole spawn+join.
  Reuse could amortize this, but on darwin reuse needs explicit zeroing or a remap
  because madvise leaves old data. The remaining upside, avoiding `copystack` in deep
  recursion, isn't one of D70's idle/example workloads.
- **Capacity:** at linux's stock map limit, tasks are capped near 32k (65530) or 131k
  (262144). Today's capacity is bounded only by memory. Plan §12.4 rejects "a speedup
  that costs unacceptable task density [or] virtual mappings".

Building the D71/D72 prototype couldn't change these numbers, because they are the layout's
lower bound. W11 therefore closes `investigated_not_adopted` with this evidence.

## Acceptance cases

| Case | Status | Basis |
|---|---|---|
| F01 memory-state layout | passed (probe lane) | reserve, guard, and touched regions built and measured with the real OS APIs on 3 targets; no `sysAlloc`-as-`PROT_NONE` assumption |
| F02 large frame and overflow | passed for the guard (probe lane) | guard faults on all targets; large-frame guard-skipping in compiled code wasn't tested (no prototype) |
| F03 preemption and GC progress | not run | needs the runtime prototype; not built |
| F04 unwind and traceback | not run | needs the runtime prototype; not built |
| F05 system and signal stacks | not run | needs the runtime prototype; not built |
| F06 idle-task density | passed (probe lane) | virtual size, RSS, mappings, page tables (note2), and exhaustion at stock and raised limits |
| F07 stack reuse | passed (probe lane) | linux zeroes via `MADV_DONTNEED`; darwin doesn't, which is recorded as a cost |
| F08 adoption decision | passed | `investigated_not_adopted`, above |

Nothing in the shipped toolchain depends on W11. The guarantee manifest (W12) states that
S-Go uses upstream growable stacks.
