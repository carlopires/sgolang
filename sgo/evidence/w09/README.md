# W09 evidence — production I/O adapters: confined files, TCP and Unix sockets (spec 0.5.0)

| Field | Value |
|---|---|
| work_package_id | W09 |
| state | passed on darwin/arm64 (local, all commands below); linux/amd64 and linux/arm64: `sgo-runtime` run 41354110725 at `0b68d667d9`, the last runtime change (S-Go runtime tests including `TestSGoResolve`, `TestSGoResolveRenameRace`, `TestSGoHandle`, `TestSGoNet`, `TestSGoNetUnix`, `TestSGoDirList`, with -cpu=1,4 and the race detector); `sgo-w00` (builds, compile-level and driver conformance with all 325 cases, identity, TCB, flags, mutations, `test-lab.py`) at `f78bb3249e`: run 41357268356 passed on both runners (every step, including the driver conformance with all 325 cases and the B03 upstream baseline) |
| spec_version | 0.5.0 frozen (101 clauses: 95 frozen, 6 superseded); W09 adds SGO-FILE-002, SGO-FILE-003, SGO-NET-002, supersedes SGO-FILE-001 and SGO-NET-001, and amends SGO-TYPE-001, SGO-TYPE-005, SGO-AUTH-002 (`sgo/spec/CHANGELOG.md`) |
| admission identity | registry content digest `f9a7bfd0b6abd1dc`; toolchain identity `6c8a3b09d1890c38` |
| decisions | D51–D59 approved 2026-11-02 (D59 amended by the user: `Dir.List` returns `*sgo.Listing`); D60 (Unix domain sockets) approved 2026-11-02; SPEC-FILE and SPEC-NET clauses approved 2026-11-02 |
| fork_commits | model `ece336b75c`, `893e5130f3`; runtime resolution `89901bcd53`, handles `458ed90c3f`, sockets `726b74ea16`; grants and API `75fc26c940`; driver, linker, admission `b17a0f1083`; `Dir.List` `5a00a99112`; lab fakes `3f534df489`; Unix sockets `0b68d667d9`; spec 0.5.0 freeze and 19 conformance cases `f78bb3249e` |
| upstream_commit | `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` (go1.27.3) |
| host_and_toolchain | macOS 15.3 arm64; stage-0 by bootstrap go1.26.10; `sgo version`: `devel-a6c637230f3b spec 0.5.0 admission 6c8a3b09d1890c38 upstream 2aae1000ea1d (host-built)` |
| artifact_digests (sha256 prefix) | `sgo` 1af958fdb86f1f7c, `compile` 38fa32d607f82968, `link` b0784a1e35a76b6d, `asm` 73c23712465ac0f9, `sgo/TARGET_TCB.json` 1b4a19ed96fdba27 |
| execution_configuration | production (host files and sockets) and lab (in-memory trees and the run's loopback network; no host I/O). A program with I/O grants records their digest in its build ID (`io=`) |
| design | `sgo/docs/w09-plan.md`; model `sgo/model/iopath*.go`, `iohandle*.go`; runtime `src/runtime/sgo_fs.go` (confined resolution), `sgo_io.go` (handle contract), `sgo_net.go` (sockets on the poller), `sgo_iodir.go`, `sgo_iogrant.go`, `sgo_ioapi.go`, `sgo_iofake.go`, `sgo_netfake.go`, system call layers `sgo_io_sys_*`, `sgo_net_sys_*`; API `src/sgo/io.go`; driver `src/cmd/sgo/iogrant.go`; linker `-sgoiogrants` (`ld/sgoid.go`); admission lists |
| next_dependency_ready_task | W10 |

## Acceptance cases

| Case | Evidence |
|---|---|
| I01 confined path resolution | model scenarios (`iopath_test.go`, 14 with renames explored; disabling the chain verification lets the moved-out scenario escape and the model catches it); `TestSGoResolve` (the model's static scenarios on a real file system), `TestSGoResolveRenameRace` (2000 resolutions against a real rename loop: outcomes within the model's set, no descriptor leak); fixture `file-confined-paths` (traversal, absolute path, prefix confusion, symlink escape, `..` inside a path) |
| I02 network scope | fixture `net-undeclared-endpoint` (every Dial checked: a granted endpoint is attempted, a later one elsewhere fails with `MissingCapability`), `net-dial-from-child`, `net-grant-hostname` (no host names), `TestSGoParseAddr` (endpoint parser against `net/netip`) |
| I03 partial reads and writes | `Read` returns at least one byte or an error with progress 0; `Write` returns its progress; fixture `net-cancel-progress` (a blocked write cancelled with 0 < `Progress` < len, not `NoEffect`, `IsCancelled`); model handle scenarios |
| I04 shared-handle cancellation | fixtures `file-handle-lent` (`HandleLent`, the borrower undisturbed), `net-cancel-progress` (the owner's connection intact after its borrower's operation is cancelled); `TestSGoHandle`, `TestSGoNet` (cancelled Accept, Read, Write; the handles work afterwards) |
| I05 close and reuse | fixture `file-handle-closed` (`HandleClosed` through a closed handle while a new one is open); `TestSGoHandle`; model generations |
| I06 real/fake adapter contract | lab twins `lab-file-dir-ops`, `lab-net-tcp-echo`, `lab-net-unix-echo` produce the same outputs as `file-dir-ops`, `net-tcp-echo`, `net-unix-echo` (16-seed sweeps for the network cases, repeat and replay for the files case) |
| I07 unsupported blocking host operation | file system calls are bounded and not reported as stopped (SGO-FILE-003); ambient host APIs stay rejected: `native-syscall-import`, `net-dial`, `file-open` |
| S13, S17 | `file-open`, `net-dial`, `native-syscall-import` stay rejected; undeclared grants, grants from children, bad grant lines fail closed |

## Commands (local, darwin/arm64, at `f78bb3249e`)

| Command | Exit | Result |
|---|---|---|
| `test-model.sh` | 0 | path and handle model scenarios; earlier lanes unchanged |
| `runtime-lane.sh -run 'TestSGo\|TestAPI\|TestRandom\|TestMulAdd' runtime sgo` | 0 | includes the I/O tests above |
| `runtime-lane.sh -race -run TestSGo runtime` | 0 | no race |
| `conformance.py run --sgo pkg/tool/darwin_arm64/sgo` | 0 | 325/325 (179 run, 117 reject, 29 lab); `validate`, `selftest` ok |
| `conformance-compile.py`, `test-identity.py`, `test-tcb.py`, `test-flags.py`, `test-lab.py`, `test-mutations.py`, `test-native-lowering.py`, `test-fmt.py`, `test-x10.py` | 0 | no regression |
| `spec-tool.py check`, `inventory-check.py`, `policy-report.py --check`, `generated-twin-check.py`, `adoption-check.py`, `source-map-audit.py` | 0 | 101 clauses (95 frozen, 6 superseded) |

## Findings during W09

- A handle lent to a nursery that outlives its owner could be closed under its borrowers: lending is limited to the owner's own nurseries, and a handle still lent to an open nursery is closed at the owner's end, not at the exit of the body that opened it (stated in SGO-FILE-003).
- The I/O system call wrappers passed stack addresses as `uintptr` through functions that could grow the stack; they are now `nosplit` and `uintptrkeepalive`, like the syscall package's.
- The endpoint parser accepted a second `::` in an IPv6 literal (found by the comparison with `net/netip`).
- A socket test passed alone but failed in the full suite because its writer and reader shared one direction; fixed in the test before commit.
- `D59`'s `List(path) ([]string, error)` was not expressible (no slices); the user chose `*sgo.Listing`.

## Open

- The chain verification bounds what another process's concurrent renames can cause; it is not atomic. Linux `openat2(RESOLVE_BENEATH)` would give the kernel's guarantee where available (not used yet).
- Lab fakes: no manifest seeding of the trees, no fault-stream short transfers yet; the lab wait graph names a socket wait as a channel receive.
- On linux, a Unix connect to a listener whose backlog (128) is full fails at once instead of waiting.
