# W10 evidence — tooling, examples, distribution, performance (spec 0.6.0)

| Field | Value |
|---|---|
| work_package_id | W10 |
| state | passed (T07 partial: allocation counts not measured). darwin/arm64: local, all commands below. linux/amd64 and linux/arm64: `sgo-w00` at `3dff7368f0` (the 0.6.0 freeze: driver conformance, `test-test`, identity, TCB, flags, lab, B03) run 41385728231 passed on both runners; `sgo-runtime` at `98d1d5a6e5` (the socket-turn fix) passed; `sgo-w00` at `5c696c766e` passed on both runners every step before B03, including `test-test`, `test-examples`, and the release clean install (run 41388746201, B03 in progress when this record was written). The closing run over the final tree, with the linux benchmark rows (D67), `sgo-w00` at `bc62b90f4e` (run 41392026352), passed on both runners, every step: driver conformance (336 cases), `test-check`, `test-test`, `test-examples`, the release clean install with `test-examples` and `test-isolation`, identity, TCB (including the assembler inputs), flags, lab, mutations, the benchmarks, and B03 |
| spec_version | 0.6.0 frozen (104 clauses: 98 frozen, 6 superseded); W10 adds SGO-TOOL-002, SGO-TEST-002, SGO-TEST-003 and amends SGO-TOOL-001 v2, SGO-TYPE-001 v5, SGO-TYPE-005 v5 (`sgo/spec/CHANGELOG.md`) |
| admission identity | registry admission digest `b25d4765ab53b004`; toolchain identity `a1bd4853d4c541d8` |
| decisions | D61–D68 approved 2026-11-02 (D62 option a, one process per test); D69 approved 2026-11-02 with the drafts |
| fork_commits | `sgo check` `4849095ae4`, `66484f16e3`; test binaries `2137cfce0f`, `5895b16948`; `sgo test` `7e8594004a`; drafts `34d2173f5c`; D69 `89f0c23016`; test conformance kind `1a8ffe527d`; freeze `3dff7368f0`; lab rendezvous fix `4a3beef346`; examples `05ae8dbe1f`, `0b792b7a41`, `330be5cd9b`, `137ae01a7d`; socket-turn fix `98d1d5a6e5`; release `5c696c766e`; TCB assembler inputs `be4c870951`; isolation tests `6915797095`; benchmarks `85119d8109`; rejection report `f60b91b59d` |
| upstream_commit | `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` (go1.27.3) |
| host_and_toolchain | macOS 15.3 arm64, 12 CPUs; stage-0 by bootstrap go1.26.10; `sgo version`: `devel-2a7044fd2d23 spec 0.6.0 admission a1bd4853d4c541d8 upstream 2aae1000ea1d (host-built)` |
| artifact_digests (sha256 prefix) | `sgo` 7ba2b1f42e6880e8, `compile` f6615f6b7f4ef1e9, `link` 63ff6b695ddc9634, `asm` 73c23712465ac0f9, `sgo/TARGET_TCB.json` 9dd771e5467d3ef8, `sgo-0.6.0-darwin-arm64.tar.gz` a374dd4f4592ca5d (local build, not published) |
| execution_configuration | production; lab (the examples' sweeps, the replay example); test binaries (configuration `test` in the build ID, generated main `testmain` in its identity) |
| design | `sgo/docs/w10-plan.md`; driver `src/cmd/sgo/check.go`, `test.go`, `tcbasm.go`; compiler `-sgocheck`, `-sgotestmain`; linker `-sgotest`; runtime `src/runtime/sgo_test_boot.go`; API `src/sgo/test.go`; examples `sgo/examples/`; release `sgo/scripts/make-release.py`; benchmarks `sgo/bench/`, `sgo/scripts/bench.py`; rejection report `sgo/scripts/rejection-report.py` |
| next_dependency_ready_task | W11 |

## Acceptance cases

| Case | Evidence |
|---|---|
| T01 artifact isolation | `test-isolation.py` (a lab-compiled package in a production main and the reverse: `SGO_E_SPEC_MISMATCH` at the import); `test-flags.py` (a `testmain` package outside a test binary and a production main in a test binary: `SGO_E_SPEC_MISMATCH` at link); `test-identity.py` (spec, admission, ABI, stale and foreign objects); `test-tcb.py` (edited Go, assembly, and header inputs of the TCB with a warm cache: `SGO_E_TCB_DIGEST`, nothing reused) |
| T02 a seed cannot upgrade a production artifact | `test-isolation.py` (a production binary with `GOLABSEED`, `SGO_LAB_SEED`, `SGOLAB`, `GODEBUG=sgolab=1` behaves as without them and writes no trace; `sgo lab run` does not take a binary); `test-lab.py` (a production binary given a lab manifest on fd 3 ignores it) |
| T03 format and typecheck integration | `test-fmt.py` (`sgo fmt` over every fixture: idempotent, keeps nurseries, refuses unparsable S-Go; formatted run cases rebuilt and rerun); `test-check.py` (`sgo check` and `sgo build` agree byte for byte on the 318 applicable cases, `-json`, a dependency that does not check stops its dependents). Editor/LSP support is documented as unsupported (D61, known gaps) |
| T04 per-test root ownership | one process per test with the root task as its lifetime root (SGO-TEST-002); `test-test.py` (a leaked permit fails the test with `LeakedWork`, an error after a leak keeps the error primary, a file written by one test is invisible to the next, a test that does not end is killed by the watchdog and its directory removed; disabling the leak check fails it); conformance cases `test-leaked-work`, `test-tempdir`, `test-pass-fail` |
| T05 adopted conformance versus the upstream suite | `rejection-report.md`: 356 top-level `test/*.go` under `sgo check`, 344 rejected with a code and clause, 12 admitted (10 `package ignored` stubs of directory tests, 2 with no code to admit), none with an expected output, 0 mismatches; B03 (untouched suite, reference toolchain) in `sgo-w00`; no target legacy lane |
| T06 three executable examples | `sgo/examples/{pipeline,service,replay}`; `test-examples.py` from the extracted release with env -i and no `go` (CI step "Release archive installs and runs the examples"): each checks, tests with quiescence (D64), runs against `expected.txt`, passes a 64-seed lab sweep; replay: found by a sweep, reproduced from the same seed and by replaying its trace, diverging after the fix |
| T07 performance evidence | `bench-<os>-<arch>.{json,md}`: raw samples of 20 fresh processes per row, wall, CPU, peak RSS, compile time, P50/P95/min–max, host and toolchain identity; S-Go vs Go pairs and lab off vs on. PARTIAL: allocation counts are not measured (S-Go programs have no memory statistics, and `GODEBUG` is ignored by design), and there is no speedup claim |
| T08 target qualification | the conformance cases are built and executed natively on each target: darwin/arm64 locally (336/336), linux/amd64 and linux/arm64 in `sgo-w00` (driver conformance step, native runners); cross-compilation is not used as runtime evidence |
| S14 production and lab share safety | W08 `lab-pair-*` cases and `lab-rejects-like-production`; W10: every example's production run and its 64-seed lab sweep give the same output; `test-isolation.py` T02 |
| S01, S15, S19 (W10 parts) | `test-isolation.py`: `sgo check` and `sgo test` refuse permissive flags and `.go` inputs and reject a denied construct identically under `GOFLAGS`/`GOEXPERIMENT`/`GODEBUG`; `sgo fmt` refuses `.go`; the release holds no `go` command or `gofmt` (`make-release.py`, CI step) |
| S09, S18 | `test-identity.py`, `test-mutations.py` (amended registry refused by the toolchain; the 0.6.0 freeze moved every identity) |
| D61 distribution | `make-release.py` + clean install (env -i, no `go`, empty HOME, outside the checkout) running `test-examples.py` and `test-isolation.py` from the archive |

## Defects found and fixed in W10

| Found by | Defect | Fix |
|---|---|---|
| pipeline example's lab sweep (55/64 schedules) | lab: a C == 0 `sgo.Channel` receiver's claimed senders were readied after the decision before its park, so the run could end `LabDeadlock` with a sender about to wake (production unaffected, 300/300) | `4a3beef346`; case `lab-rendezvous-claimed-senders` (fails on the previous runtime) |
| service example's tests | production: two tasks waiting on one socket handle (workers accepting on one listener) ended the program with `runtime: double wait` | `98d1d5a6e5` (a per-direction turn in the poller); case `net-shared-listener-accept` |
| release clean install | TCB: the assembler's inputs were never digest-checked (only the compiler's Go files were), the TCB cache key omitted headers, and 5 included headers (`runtime/cgo/abi_*.h`) were not in the manifest | `be4c870951`: the driver verifies every `.s` file and included header before assembling; headers pinned in `TARGET_TCB.json`; `test-tcb.py` cases |
| first `sgo test` link | `-sgotest` changed the configuration the linker requires of packages | `5895b16948` |
| D69 review | `sgo.StartTest` was callable by user code | `89f0c23016`: admitted only under `-sgotestmain`; `testmain` identity confined to test binaries |

## Commands (local, darwin/arm64, at `f60b91b59d`)

| Command | Exit | Result |
|---|---|---|
| `conformance.py run --sgo pkg/tool/darwin_arm64/sgo` | 0 | 336/336 (181 run, 119 reject, 30 lab, 6 test); `validate`, `selftest`, `conformance-compile.py` ok |
| `test-test.py`, `test-examples.py`, `test-isolation.py`, `test-check.py` | 0 | as above |
| `test-identity.py`, `test-tcb.py`, `test-flags.py`, `test-lab.py`, `test-mutations.py`, `test-x10.py`, `test-native-lowering.py`, `test-fmt.py`, `test-model.sh` | 0 | no regression |
| `runtime-lane.sh -run TestSGo runtime`; `go vet runtime sgo` for linux/amd64 and linux/arm64 | 0 | after each runtime change |
| `make-release.py` then the clean-install `test-examples.py` and `test-isolation.py` | 0 | 60 MB, 1064 TCB files, 5 pinned headers, no `go` |
| `bench.py`, `rejection-report.py` (`--check` ok) | 0 | reports above |
| `spec-tool.py check`, `inventory-check.py`, `policy-report.py --check`, `tcb-manifest.py`, `generated-twin-check.py`, `adoption-check.py` | 0 | 104 clauses (98 frozen, 6 superseded) |

## Known gaps

- Tests of package `main` and tests under the lab configuration are not in 0.6.0 (D69).
- The committed replay trace replays only with the build that recorded it (D46); `test-examples.py` checks it through a fresh trace of the same seed.
- T07 is partial: allocation counts and virtual size are not measured; compile times are not like for like; linux rows come from the closing run's artifacts.
- The rejection report covers top-level `test/*.go` only, not directory tests or subdirectories.
- Socket waits on a shared handle: tasks other than the turn holder retry after 1 ms (latency, not correctness).
- Editor/LSP support stays unsupported (D61).
