# W03 evidence — nursery compiler and all exit paths

| Field | Value |
|---|---|
| work_package_id | W03 |
| state | passed on darwin/arm64 (local, all commands below) and linux/amd64, linux/arm64 (CI `sgo-w00` run 41158745121 at `cba9a572dd`: driver conformance, identity, fmt, TCB identity, X10, flags, mutations; `sgo-runtime` run at `c6132c40b2`, the last runtime change) |
| spec_version | 0.1.0 (84 frozen clauses; no clause changed in W03) |
| spec_digest / admission_digest | `sgo/SGO_SPEC_ADMISSION.json` sha256 prefix `5558fdec248c016e` (recorded in every package, binary, and the driver) |
| fork_commit | `cba9a572dd` |
| upstream_commit | `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` (go1.27.3) |
| host_and_toolchain | macOS 15.3 arm64; stage-0 tools built by bootstrap go1.26.10 darwin/arm64 (`sgo/scripts/build-stage0.sh`); `sgo version`: `devel-821263cc2783 spec 0.1.0 admission 5558fdec248c016e upstream 2aae1000ea1d (host-built)` |
| artifact_digests (sha256 prefix) | `sgo` 1d8a5210f7b3c28b, `compile` fe5849468ac0260c, `link` 552a20d95aad17ee, `asm` 73c23712465ac0f9, `sgo/TARGET_TCB.json` a8f738183f9ce1a0 |
| execution_configuration | production (lab mode denied until W08); targets darwin/arm64, linux/amd64, linux/arm64; drain budget not yet implemented (W04) |
| host_target_build_identity | binaries carry build ID `sgo1/spec=0.1.0/admission=5558fdec248c016e/target=<os>-<arch>/toolchain=<id>/upstream=<commit>` (Mach-O text marker; ELF `.note.go.buildid`) |
| benchmark_configuration | none (no performance claims in W03) |
| next_dependency_ready_task | W04 — cancellation, Sleep/Checkpoint, drain budget (clears the 17 pending conformance cases) |

## Commands (local, at `cba9a572dd`; logs and exit codes in `logs/`)

| Command | Exit | Result |
|---|---|---|
| `sgo/scripts/spec-tool.py check` | 0 | 84 frozen clauses |
| `sgo/scripts/inventory-check.py` | 0 | 546 surfaces, 104 rules (admit 91, internal 107, deny 348) |
| `sgo/scripts/tcb-manifest.py` | 0 | 31 packages, 1028 files, 12 changed from upstream, 5 active / 5 dormant / 1 not-built services |
| `sgo/scripts/generated-twin-check.py` | 0 | 59 generator inputs/outputs equal upstream |
| `sgo/scripts/conformance.py validate` / `selftest` | 0 / 0 | 191 cases (98 run, 93 reject) |
| `sgo/scripts/conformance.py run --sgo pkg/tool/darwin_arm64/sgo --pending sgo/testdata/conformance-pending.json` | 0 | 174 passed, 17 pending (W04), 0 unexpected |
| `sgo/scripts/conformance-compile.py` | 0 | 79/79 eligible rejections at the expected positions; 94 run cases free of diagnostics |
| `sgo/scripts/test-identity.py` | 0 | build metadata; stale contract (exported, unexported, recorded-only); foreign export data and object; spec mismatch |
| `sgo/scripts/test-tcb.py` | 0 | edited/added runtime file and edited `sgo` file: one `SGO_E_TCB_DIGEST` each |
| `sgo/scripts/test-x10.py` | 0 | cross-package nurseries identical with default optimization, `-l`, and the driver |
| `sgo/scripts/test-flags.py` | 0 | compiler flags/environment do not change admission; driver/linker refuse permissive flags and modes |
| `sgo/scripts/test-mutations.py` | 0 | 2 inventory, 4 spec, 3 amendment mutations rejected |
| `sgo/scripts/test-fmt.py` | 0 | 198 files formatted, 2 refused, 79 formatted run cases rerun unchanged |
| `sgo/scripts/runtime-lane.sh -count=3 -cpu=1,4 -run 'TestSGo\|TestAPI' runtime sgo` | 0 | includes TestSGoGoexitIsDefect |
| `sgo/scripts/runtime-lane.sh -race -count=2 -run TestSGo runtime` | 0 | |
| `sgo/scripts/test-host-pkg.sh cmd/compile/internal/syntax -count=1` | 0 | nursery parse/print/position, keyword tests |

Passed/failed/skipped/unrun: every command above passed; 17 conformance cases are unrun-by-design pending W04 (listed in `sgo/testdata/conformance-pending.json`, which may only shrink); nothing skipped.

## Acceptance cases

"Subset-0 scope" means the frozen spec denies the construct the case exercises, so the W03 obligation is its rejection; the positive part arrives with the clause that admits it.

| Case | Evidence | Result |
|---|---|---|
| X01 lexer and rejection gate | lex-* fixtures (keywords, `nursery` reserved, bare go, Go extensions, legacy octal, UTF-8); syntax `TestNurseryParse/Branches/Reserved`, `TestKeyword*`; tool-compat-flag | passed |
| X02 nursery capability binder | nur-anonymous, nur-assign-binder, nur-pass-handle, cap-compare-nursery, cap-nil-context, cap-context-as-nursery, bind-import-alias/sgo, spawn-helper-with-handle | passed |
| X03 all source surfaces | syntax print/position round trip; `sgo fmt` over every fixture (test-fmt); Unified IR export/import (imp-user-package, test-x10, test-identity); one parser/type checker for build and fmt | passed |
| X04 normal block exit | nur-basic, nur-nested, def-after-nursery, spawn-into-outer-from-inner | passed (scope finalizers: subset-0 scope, def-scope-finalizer rejects `n.Defer`) |
| X05 return operands, named results | ctl-return-operands-nursery, ctl-return-from-nursery, val-eval-order; ctl-defer-assigns-named-result | passed; defers cannot assign named results in S-Go (captures final, SGO-MEM-002) |
| X06 nested return | test-x10 (`Nested`), ctl-return-from-nursery; join of a non-innermost scope is a runtime violation | passed |
| X07 labeled break/continue | ctl-labeled-nursery-exits, ctl-break-out-of-nursery, ctl-nursery-in-loop, ctl-labeled-loops | passed |
| X08 goto boundaries | ctl-goto | passed for subset 0 (`goto` denied, so no inward or outward edge exists) |
| X09 defer vs scope finalizer | def-lifo, def-after-nursery, def-scope-finalizer | passed for subset 0 (finalizers denied; defers stay function-scoped, LIFO) |
| X10 export, generics, inlining | test-x10 (default and `-l` agree; nursery functions not inlined, spawned closures inlinable) | passed; generics half subset-0 scope (type-generic rejects) |
| X11 closures and captures | mem-capture-escapes-frame, mem-capture-final/-loop-var/-mutated/-assigned-in-literal, mem-locals | passed |
| X12 exceptional frame movement | def-unwind-deep-nurseries(-child): 3001 open nurseries on a growing stack, root and child | passed; task-exit operation not in spec 0.1.0 |
| P01 single escaping child panic | nur-child-error-exit-1, err-chain-*, def-child-panic-cancels-siblings | W03 part passed (original payload re-raised after siblings complete); sibling cancellation W04 (pending fixture) |
| P02 recovered child panic | def-recover | W03 part passed: `recover` denied in subset 0; the runtime's own recovery in task entry publishes no false failure (TestSGo* lane) |
| P03 multiple escaping panics | nur-two-failures-lowest-id, nur-primary-not-by-time, err-chain-nested-child | W03 part passed; drain under cancellation W04 |
| P04 owner panic precedence | err-chain-owner-and-children | W03 part passed; finalizer failures subset-0 scope |
| P05 nil and replacement panics | def-panic-in-defer, def-panic-int, def-recover | W03 part passed: `panic` takes a string (no `panic(nil)`), `recover` denied |
| P06 task exit, raw Goexit | proc-raw-goexit, TestSGoGoexitIsDefect | W03 part passed; `sgo.ExitTask` and the test runner are not in spec 0.1.0 |
| P07 owner blocked during child failure | cancel-owner-woken-by-child-failure | W04 (pending; needs cancellation) |
| P08 recovery after nursery unwind | def-recover | W03 part passed: `recover` denied; recovery of runtime-internal unwinding drops its chain entries (sgo_record.go) |
| S01 no legacy flags | test-flags, err-no-suppression-flag, tool-compat-flag, lab-flag, proc-godebug-ignored | W03 part passed (W10: release tooling) |
| S02 inventory completeness | inventory-check; test-mutations (2 mutations) | W03 part passed |
| S03 missing/draft clause | spec-tool check; test-mutations (4 mutations) | W03 part passed |
| S05 bare go transitively | lex-bare-go, spawn-go-unreachable, spawn-go-in-dependency, spawn-go-generated-file, bind-init-func | W03 part passed (W05: ported packages) |
| S06 no implicit authority | cap-context-as-nursery, cap-nil-context, spawn-unrelated-task, spawn-helper-with-handle | passed (runtime part in W02) |
| S08 spawn callback failures | nur-child-error-exit-1, err-chain-child-errors, spawn-nil-callback, val-eval-order | W03 part passed; drain of siblings W04 |
| S09 dependency contracts | test-identity, test-tcb, imp-stale-contract, imp-stock-object | W03 part passed; effect annotations do not exist in subset 0 (W05/W10) |
| S10 indirect dispatch | admission is syntactic over all target source, so function values, closures, and interfaces reach only admitted code; type-generic, reflect-import, spawn-helper-with-handle | W03 part passed (W05: summaries for ported packages) |
| S12 unadmitted channels | chan-type, chan-select, reflect-import, sync-mutex | W03 part passed (W07: channel admission) |
| S15 no target build escape | imp-stock-object, test-identity (foreign objects), test-tcb, test-flags, tool-go-file-input | W03 part passed (W10) |
| S16 value/memory semantics | val-* (overflow, division, shifts, conversions, strings), err-stack-overflow-exit-4, mem-* | W03 part passed; shared mutation and atomics do not exist in subset 0 (W07) |
| S17 documented unsupported rejected | every subset-0 denial has a reject fixture; admitted-but-unimplemented W04 operations fail closed with SpawnContractViolation | W03 part passed (W09/W12 release qualification) |
| S18 amendment propagation | test-mutations (frozen clause change; amended registry refused by the toolchain); identity in packages, binaries, driver | W03 part passed (W10/W12) |
| S19 source tools | `sgo fmt` refuses unparsable S-Go and builds nothing; the driver accepts only `.sgo` through the one admission path (tool-go-file-input, test-flags) | W03 part passed (W10) |

## Changed invariants (TCB and toolchain)

- TCB files changed from upstream: 12 (`sgo/TARGET_TCB.json`). W03 added `runtime/sgo_main.go`, `sgo_nursery.go`, `sgo_record.go`, `sgo_api.go`, the `sgo` package (`src/sgo/sgo.go`, `sgo.s`), and hooks in `panic.go`, `runtime2.go`, `stack.go`, `proc.go` (W02 added `sgo_scope.go`, `sgo_service.go`). Every change is pinned by digest and reviewed in its commit message.
- The compiler admits a TCB package only from its pinned files (`SGO_E_TCB_DIGEST`); any other package goes through subset-0 admission.
- The linker links only S-Go executables with complete, consistent identities (`SGO_E_FOREIGN_OBJECT`, `SGO_E_SPEC_MISMATCH`, `SGO_E_STALE_CONTRACT`, `SGO_E_USAGE`).
- The driver refuses a GOROOT whose spec or admission registry differs from the one it was built for.
- `runtime.Goexit` on an S-Go task is a RuntimeDefect (exit 4).

## Known limitations

- Cancellation, `Sleep`, `Checkpoint`, and the drain budget are W04: until then `Nursery.Cancel` and `Context.Sleep` fail closed, and unwinding waits for children instead of cancelling them.
- A file deleted from a TCB package is not detected by the compiler's digest check (only edited and added files are); the manifest check (`tcb-manifest.py`, CI) does detect it.
- The scope lock is still unranked (W02 note); W04 adds a static rank.
- Generics, `goto`, `recover`, scope finalizers, channels, and shared mutation are denied by spec 0.1.0; the cases above that exercise them pass only their rejection part.
