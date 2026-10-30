# W00 evidence — reproducible fork and baseline

Fields follow `SGO_TASKS.json` `evidence_required_fields`. Machine-readable records are the JSON files in this directory. Large raw logs (test events) stay in `$SGO_CACHE/logs` locally and in CI artifacts.

| Field | Value |
|---|---|
| work_package_id | W00 |
| state | **passed** on darwin/arm64, linux/amd64, linux/arm64 (S01/S15 at gate level; compiler/linker level is W03). |
| spec_version / spec_digest / admission_digest | `null` (no clause frozen; W01). The target gate reads `SGO_SPEC_ADMISSION.json` and admits nothing. |
| execution_configuration | Not applicable: no S-Go program executes at W00. |
| upstream_commit | `go1.27.3` = `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` |
| fork_commit | `bd5889a4c87f6aa9d209bd181bd5efc8a3fe3173` (code); B01/B02/B04/checks re-run at this commit; B03 and benchmarks ran at `807dd104` (same scripts except the PATH guard, which changes nothing on this host: no Go exists in `/usr/bin`, `/bin`, `/usr/sbin`, `/sbin`) |
| host_and_toolchain | darwin/arm64, Apple Silicon, 12 CPUs; bootstrap `go1.26.10` official archive, SHA-256 `0b518349…20b3` (UPSTREAM.lock.json) |
| host_target_build_identity | H0 bootstrap `$SGO_CACHE/bootstrap/go1.26.10-darwin-arm64/go`; R reference `$SGO_CACHE/reference/go1.27.3-b01` and `-b02`; H1 stage-0 `<fork>/pkg/tool/darwin_arm64` (see `sgo/docs/stage-map.md`) |
| next_dependency_ready_task | W01 — closed first-subset specification and executable oracle |

## 1. darwin/arm64 results (2026-10-30)

| Case | Command | Exit | Result | Record |
|---|---|---|---|---|
| B01 | `sgo/scripts/build-reference.sh` | 0 | **passed**: built in 54 s; `go version go1.27.3 darwin/arm64`; GOROOT and GOTOOLDIR inside the worktree | `reference-build-darwin-arm64-b01.json`, `logs/…-b01.log` |
| B02 | `sgo/scripts/build-reference.sh --label b02` | 0 | **passed**: independent worktree; `go`, `compile`, `link` binaries **bit-identical** to B01 (same SHA-256) | `reference-build-darwin-arm64-b02.json`, `checks-darwin-arm64.json` |
| B03 | `sgo/scripts/test-reference.sh` (`go tool dist test -k -json`) | 0 | **passed**: 459 packages passed, 252 had no tests; 129,325 tests passed, **0 failed**, 2,912 skipped; 7 min 31 s | `reference-test-darwin-arm64.json` |
| B04 | `sgo/scripts/build-stage0.sh` (`dist bootstrap -a -sgo-stage0`) | 0 | **passed**: toolchain1 built by the bootstrap only in 11 s; go_bootstrap/toolchain2/toolchain3/std skipped; no `bin/go` produced | `stage0-build-darwin-arm64.json`, `logs/stage0-build-darwin-arm64.log` |
| B05 | `sgo/scripts/check-w00.sh` | 0 | **passed**: with a fake `go` first on PATH and hostile `GOROOT/GOTOOLCHAIN/GOFLAGS/GOEXPERIMENT/GODEBUG`, scripts select the recorded tools; the fake `go` never ran; build environments are scrubbed | `checks-darwin-arm64.json` |
| B06 | `sgo/scripts/source-map-audit.py` | 0 | **passed**: 37 original claims resolved, 10 do not exist as written (recorded, not stubbed); all 41 plan §10.3 paths exist; 5 confirmed findings | `sgo/docs/source-map.md` |
| S01 | `check-w00.sh` | 0 | **passed at gate level**: the target entry `sgo/scripts/sgo-target` denies a valid subset-0 program, legacy Go with bare `go`, build-tag/pragma/linkname escapes, `-compat`, `--legacy`, `-lang`, `-gcflags`, override env vars, and no-argument use (all exit 2, `SGO_E_UNADMITTED`) | `checks-darwin-arm64.json` |
| S15 | `check-w00.sh` | 0 | **passed at gate level**: building host compiler sources, linking a stock object, and backend access through the target entry are denied. **Not enforced yet:** the host-built stage-0 compiler still accepts Go when invoked directly (W03) | `checks-darwin-arm64.json` |
| — | `sgo/scripts/bench-reference.sh` | 0 | baseline: 17 runtime benchmark groups × 5 (task creation, channels, select, stacks, scheduling, defer/panic) | `bench-reference-darwin-arm64.{txt,json}` |

**Skip reasons (B03).** The 2,912 skipped tests are upstream skips in `dist test`'s default short mode and for missing optional environment (e.g. network, other OSes/architectures, privileged operations). The 200-entry sample is in the JSON; the full list is in the raw event file. No test was skipped by S-Go.

## 2. Known limitations

1. **Compiler-level denial is W03.** S01/S15 pass only for the W00 target entry point. The stage-0 compiler is the unmodified upstream compiler; it is a host tool, not an S-Go target path, and it still accepts Go when run directly.
2. **Toolchain identity.** The stage-0 compiler reports `go1.27.3`. S-Go spec/ABI identity in export data and link metadata is W03 (plan §1.3).
3. **Runtime admission (T0)** has no contract form yet (stage map §3); W01 decides it.
4. **`nursery` keyword collision.** `hash("nursery") == hash("chan") == 28` in the upstream scanner's perfect-hash keyword table; W03 must change the table (source map §3).
5. **Benchmarks** are load-sensitive and only comparable on the same machine with benchstat.

## 3. Linux evidence (D22)

**First CI run (41077627683, cancelled):** on linux/arm64, B01, B02, and B04 passed, and the B05 environment check **failed**: GitHub's Ubuntu image ships `/usr/bin/go`, which was inside the scrubbed PATH. Fixed in `bd5889a4` with a PATH guard (`sgo/scripts/lib.sh`); the check now requires `go` on PATH to resolve to the guard. This is the kind of silent fallback B05 exists to catch.

GitHub Actions workflow `.github/workflows/sgo-w00.yml` runs B01, B02, B04, B05/S01/S15, and B03 on `ubuntu-24.04` (linux/amd64) and `ubuntu-24.04-arm` (linux/arm64). Results are recorded below from the run's artifacts.

**Second CI run (41079191705, at `de72f7fd94`): passed on both Linux targets.**

| Case | linux/amd64 (`ubuntu-24.04`) | linux/arm64 (`ubuntu-24.04-arm`) |
|---|---|---|
| B01 | passed: `go version go1.27.3 linux/amd64`, 2 min 48 s | passed: `go version go1.27.3 linux/arm64`, 1 min 55 s |
| B02 | passed: `go`, `compile`, `link` bit-identical to B01 | passed: bit-identical |
| B03 | passed: 529 packages, **186,663 tests passed, 0 failed**, 3,112 skipped; 20 min | passed: 466 packages, **129,697 tests passed, 0 failed**, 2,952 skipped; 11.5 min |
| B04 | passed: stage-0 toolchain1 only | passed |
| B05 | passed: PATH lookup of `go` resolves to the guard (the image's `/usr/bin/go` is shadowed) | passed |
| S01 / S15 | passed at gate level | passed at gate level |

Records: `*-linux-amd64*.json`, `*-linux-arm64*.json`, and `logs/*linux*`. Raw test events are in the run's artifacts (`w00-evidence-ubuntu-24.04*`, kept 90 days). linux/amd64 runs more test packages than linux/arm64; the reason was not investigated, and the per-package lists are in each JSON. Benchmarks were recorded on darwin/arm64 only; CI runners are shared hosts, so their timings would not be stable baselines.
