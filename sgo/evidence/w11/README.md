# W11 evidence: fixed virtual stacks (vstack), probe stage

State: **probe complete on darwin/arm64 and linux/amd64; prototype not started.**
Decision rule: D70 (preregistered). Probe source: `sgo/w11/probe/main.go`,
design: `sgo/docs/w11-architecture.md`.

The probe builds the proposed layout directly with the reference/bootstrap
toolchain, without changing the runtime: one `MAP_NORESERVE` 8 MiB reservation
per task, a 64 KiB `PROT_NONE` guard below `lo`, and one touched page. These
numbers are therefore the **floor** cost of the layout. A runtime prototype can
only add to them.

## Results

| Check | darwin/arm64 (16 KiB pages) | linux/amd64 (4 KiB pages, note2) |
|---|---|---|
| F02 guard fault | write below `lo` faults (caught) | faults, `SEGV_ACCERR` (code 0x2) on the guard page |
| F07 reuse via madvise | `MADV_FREE_REUSABLE`: **512/512 pages keep old data**, RSS unchanged | `MADV_DONTNEED`: 0 pages with old data, RSS -8 MiB |
| RSS per idle task | ~16.6 KiB (n=20000) | 4290 B (n=512000) |
| Page tables per idle task | not measured | **~4.03 KiB** (VmPTE 2.11 GB at n=524268), not in RSS |
| Kernel cost per idle task | ≥16.6 KiB | **~8.2 KiB** |
| Reserve+guard+touch | ~6 µs/task (n=20000) | ~5.06 µs/task |
| Capacity | 2,000,000 reservations, no exhaustion | **32,749 tasks at stock `max_map_count` 65530**; 524,268 at 1048576; fails in `mprotect` (ENOMEM), process-wide |

Notes:
- The darwin density curve isn't monotonic at large n: RSS was 5.10 GB at n=1,024,000
  and 2.49 GB at n=2,000,000. Under that pressure macOS compresses or purges idle
  pages, so darwin RSS is **not** a valid measure past a few hundred thousand
  tasks. Only the n ≤ 20000 reading (16.6 KiB/task, linear) is used above.
- On linux, exhausting the map limit is process-wide: any later runtime `mmap`
  (heap growth, threads) would fail too. A real runtime would need a reservation
  cap well below `max_map_count/2`.
- The `"signal":"signal -1"` field in `guard` output is harmless; it is printed
  when the child exits normally with Go's fault report.

## Comparison with the W10 baseline (measured, `sgo/evidence/w10/bench-*.json`)

| Platform | Baseline per live task | vstack floor per idle task | Baseline spawn+join | vstack reserve alone |
|---|---|---|---|---|
| darwin/arm64 | ~2.9 KB | ≥16.6 KiB (**~5.8x**) | 0.50 µs | ~6 µs |
| linux/amd64 | ~1.9 KB | ~8.2 KiB (**~4.4x**) | 0.64 µs | ~5.1 µs |
| linux/arm64 | ~2.0 KB | not yet probed | 0.86 µs | not yet probed |

(Baseline per-task = (`livetasks` − `emptynursery`) P50 maxrss / 10000.)

## Reading against D70

D70 adopts vstack only if P50 RSS or wall time on idle/example workloads improves
by ≥20%, with ≤5% regression elsewhere. Even before any runtime work, the layout's
floor is 4–6x **worse** than the baseline per idle task, and creating a fresh
stack costs ~8x more (reuse can amortize that). On linux at the stock map limit,
the baseline's task capacity, which today is bounded only by memory, drops to
~32k. The only remaining upside is wall time on deep-recursion workloads, where
copystack is avoided. Those are not idle/example workloads, and the stack-growth
copies they save are rare.
