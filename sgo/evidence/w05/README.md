# W05 evidence — capabilities and readiness

| Field | Value |
|---|---|
| work_package_id | W05 |
| state | passed on darwin/arm64 (local, all commands below) and linux/amd64, linux/arm64: `sgo-runtime` run 41251584929 at `984ab21da0`, the last runtime change (S-Go runtime and sgo package tests with -cpu=1,4, race detector, upstream runtime -short); `sgo-spec` run 41251861849 and `sgo-w00` run 41251861844 at `2365302e5e` (builds, compile-level and driver conformance with every W05 case, identity, fmt, TCB, X10, flag and mutation gates, policy report) |
| spec_version | **0.2.0** (frozen in W05: 87 frozen clauses, 2 superseded; `sgo/spec/CHANGELOG.md`) |
| admission identity | registry content digest `d1ecf78b6f825e3f`; toolchain identity (SHA-256 of `sgo/SGO_SPEC_ADMISSION.json`) `40c3952dc2ada3d7` |
| fork_commit | `2365302e5e` |
| upstream_commit | `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` (go1.27.3) |
| host_and_toolchain | macOS 15.3 arm64; stage-0 by bootstrap go1.26.10; `sgo version`: `devel-6a79e8c87f88 spec 0.2.0 admission 40c3952dc2ada3d7 upstream 2aae1000ea1d (host-built)` |
| artifact_digests (sha256 prefix) | `sgo` b6b064b55759ebe0, `compile` 6bee8eea09444df1, `link` b8670ea32d4fcc64, `asm` 73c23712465ac0f9, `sgo/TARGET_TCB.json` 928c60b1ca02d9e8 (allowlist digest `65635bcf5fdb76b0`) |
| execution_configuration | production (lab denied until W08); targets darwin/arm64, linux/amd64, linux/arm64; root grants per binary (`grant` lines, recorded as `grants=` in the build ID) |
| host_target_build_identity | `sgo1/spec=0.2.0/admission=<id>/drain=<ms\|none>/grants=<kinds\|none>/target=<os>-<arch>/toolchain=<id>/upstream=<commit>` |
| design | `sgo/docs/runtime-design.md` §11 (readiness), §12 (authority and effects); `sgo/docs/policy.md` (generated policy report) |
| next_dependency_ready_task | W07 (native channels and select; depends on W03, W05, W06) |

## Deliverables

- **Capability facade and enforcement** (SGO-AUTH-001..003): task ceilings fixed at creation and narrowed by `n.SpawnWithout`; root grants from `sgo.mod`, through the driver, linker (`-sgogrants`), and runtime; effect handles `*sgo.Clock`, `*sgo.WallClock`, `*sgo.Random`, `*sgo.Entropy`, `*sgo.SimStore` with attenuation (`ReadOnly`, `Prefix`) and revocable `LendTo`; every operation checks ceiling, revocation, and permission before any effect.
- **TCB/policy manifest**: `sgo/TARGET_TCB.json` is versioned (spec version, allowlist digest) and lists TCB-P12..P16; `sgo/scripts/policy-report.py` generates `sgo/docs/policy.md` and fails CI when the effect kinds (runtime, driver, linker, package sgo, spec), the handle types (sgoadmit, types2), the 28 denied routes and their cases, or the allowlist's spec version disagree.
- **Simulated effects** ("fake adapters"): the seeded generator (PCG-DXSM, identical to math/rand/v2) and the in-memory store; clock reads are real but only through handles.
- **Readiness handshake** (SGO-READY-002): `n.Start`, `*sgo.Ready`.
- **Capability and readiness fixtures**: 30 new cases — 19 `auth-*`, 6 `ready-*`, `native-syscall-import`, `native-plugin-import`, `native-runtime-finalizer`, and 2 `tcb-*`.
- **Spec 0.2.0** frozen, with the subset-0 clauses it amends (TYPE-001, TYPE-005, CAP-001, GEN-001) and supersedes (PERMIT-001, READY-001).

## Commands (local, at `2365302e5e`; logs and exit codes in `logs/`)

| Command | Exit | Result |
|---|---|---|
| `runtime-lane.sh -count=5 -cpu=1,4,16 -run 'TestSGo\|TestAPI\|TestRandom\|TestMulAdd' runtime sgo` | 0 | |
| `runtime-lane.sh -race -count=3 -run TestSGo runtime` | 0 | |
| `runtime-lane.sh -v -run TestSGoReady runtime` | 0 | 5 tests; start/cancel race: ready won 77, cancellation won 323 of 400, no other outcome, nothing left registered |
| `runtime-lane.sh -v sgo` | 0 | PCG-DXSM and bounded draws equal math/rand/v2 for 6 seeds × 1000 outputs and 11 bounds; Split; mul64/add64 equal math/bits |
| `runtime-lane.sh -short runtime` (full upstream runtime suite) | 0 | |
| `conformance.py validate`, `selftest` | 0 | 240 cases (133 run, 107 reject) |
| `conformance.py run --sgo … --pending …` | 0 | 240 passed; pending list empty |
| the 30 new W05 cases, 25 times | 0 | identical every time |
| `conformance-compile.py` | 0 | 88/88 eligible rejections; 128 run cases clean |
| `test-identity.py`, `test-flags.py`, `test-mutations.py`, `test-tcb.py`, `test-x10.py`, `test-fmt.py` | 0 | build ID now carries `grants=`; scripts read the spec version from `sgo/spec/VERSION` |
| `test-model.sh` | 0 | |
| `spec-tool.py check`, `inventory-check.py`, `tcb-manifest.py`, `generated-twin-check.py`, `adoption-check.py`, `policy-report.py` | 0 | 87 frozen + 2 superseded clauses at 0.2.0; 31 packages, 1037 files, 21 changed from upstream |

CI on the way: `sgo-spec` failed at `52e39f5035`, `1e308bb225`, `984ab21da0`, and `0084f33c4e` because the runner self-test's fake driver did not reproduce expected stderr, which the new `auth-*` cases (sorted first) require; fixed in `2365302e5e`. Passed/failed/skipped/unrun: all passed; nothing skipped or unrun.

## Acceptance cases

"Production" marks where the case's lab-domain part belongs to W08 (lab execution is denied until then, SPEC-LAB).

| Case | Result | Evidence |
|---|---|---|
| A01 Missing authority | passed | `auth-missing-grant` (no grant: MissingCapability before any effect), `auth-root-only`, `auth-grant-unknown`, `auth-grant-duplicate`; `sgoRequire` precedes every primitive |
| A02 Attenuation | passed | `auth-spawn-without` (narrowed child and grandchild), `auth-simstore-attenuation` (ReadOnly, nested Prefix), `auth-simstore-prefix-denied`; no operation widens a ceiling or handle |
| A03 Expired and revoked handle | passed | `auth-lend-revoked` (lent handle works while its nursery is open, revoked when it closes; the original keeps working) |
| A04 Cross-domain handle | passed for production | one execution domain per program: handles are opaque in-process objects that cannot be converted, serialized, compared, or constructed (SGO-TYPE-005, `auth-handle-compare-denied`); lab/host domains are W08 |
| A05 Hidden dependency effect | passed | `auth-dependency-effect` (dependency only through a given handle, within the caller's ceiling), `auth-function-value` (indirect call), `auth-dependency-ambient` (os in a dependency), `bind-init-func` (no init path) |
| A06 Unsafe escape hatches | passed | `native-unsafe`, `native-cgo`, `native-asm`, `native-linkname`, `native-syscall-import`, `native-plugin-import`, `tool-go-file-input`, `imp-stock-object`; `test-flags.py` (no flag, mode, or environment reopens them) |
| A07 Uncontrolled cleanup callback | passed | `native-runtime-finalizer` (runtime cannot be imported), `def-scope-finalizer` |
| A08 Separate entropy sources | passed | `auth-random-deterministic` (seed 42 gives the math/rand/v2 sequence on every target), `auth-entropy` (OS source), `TestRandomMatchesMathRandV2`; Entropy never uses the seeded generator |
| A09 TCB confusion | passed | `tcb-user-package-named-runtime` (no privilege by name), `tcb-module-named-sgo` (cannot shadow package sgo), `test-tcb.py` (TCB admitted only by exact file digests) |
| A10 Mutable authority alias | passed | `auth-spawn-without` (a captured handle does not carry authority), handles immutable except a task-affine generator (`auth-random-other-task`); ceilings written only at task creation |
| R01 Readiness success | passed | `TestSGoReadySuccess`, `ready-signal` |
| R02 Failure before readiness | passed | `TestSGoReadyChildEndsFirst` (error, panic, nil), `TestSGoReadyStarterCancelled`, `ready-child-fails-first`, `ready-cancelled-before-signal` |
| R03 Duplicate readiness | passed | `TestSGoReadyDuplicateSignal`, `ready-double-signal`, `ready-signal-other-task` |
| R04 Start cancel race | passed | `TestSGoReadyCancelRace` (both outcomes exercised; a successful Start always names a running, joined child; nothing left registered), `ready-cancelled-before-signal` |
| S05 Bare go rejected transitively (W05 part) | passed | `spawn-go-in-dependency`, `spawn-go-generated-file`, `spawn-go-unreachable` (W03), unchanged at 0.2.0 |
| S09 Dependency contract integrity (W05 part) | passed | `imp-stale-contract`, `imp-stock-object`, `test-identity.py` (digest-only change caught at compile time), `test-tcb.py` |
| S10 Indirect dispatch contract closure (W05 part) | passed | `auth-function-value`; interfaces, user generics, reflection denied (`type-generic`, `reflect-import`) |
| S11 Target TCB is specified (W05 part) | passed | TCB-P12..P16 in the versioned manifest with clauses and callers; `test-tcb.py`; `native-linkname` (sealed primitives unreachable from application code) |
| S13 No ambient library preservation (W05 part) | passed | `bind-import-fmt`, `auth-dependency-ambient`, `timer-time-package`, `random-import`, `net-dial`, `sync-mutex`; `sgo/docs/policy.md` |

## Gate (plan W05)

- Absence, narrowing, and revocation deny the effect before it occurs, including hidden dependency entry points: A01–A03, A05 above; every primitive is reached only after `sgo_require` and the handle checks.
- Unsupported unsafe/cgo/assembly routes are rejected, not trusted: A06, A07, A09.
- Ready/failure/cancel races preserve ownership: R01–R04.

## Decisions and open items

- `Start` returns `child <id> ended before ready` when the child returns `nil` without signalling (SGO-READY-002).
- `SpawnWithout`'s kind and argument preconditions are run-time checks (`InvalidArgument`); there is no compile-time negative-constant check.
- Entropy's OS-source failure ends the program as ResourceExhausted (status 4), as SGO-ERR-003 requires.
- Open for a later spec version: a `*sgo.Random` can be obtained only by the root task (task affinity has no hand-off); a module named `sgo` is rejected as an import cycle rather than with a dedicated diagnostic; `simnet` is not in 0.2.0.
