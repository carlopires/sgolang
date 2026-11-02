# W12 evidence — independent review and release qualification (spec 0.7.0)

| Field | Value |
|---|---|
| work_package_id | W12 |
| state | in progress. Every review finding (R1–R12, R6b) is fixed and verified by the reviewer on linux/amd64; R13 is fixed (`17054fecb6`) and awaiting the reviewer's re-check. D77, D78, D79 are done; D80 passed on darwin/arm64 and runs on linux in `sgo-stress` run 41523619092 (PENDING). Closing evidence is recorded below as it lands |
| spec_version | 0.7.0 frozen (104 clauses: 98 frozen, 6 superseded): the release registry (D83) with 30 families admitted and 8 denied; amended SGO-TCB-002 v2, SGO-MEM-002 v2, SGO-PROC-004 v2, SGO-FILE-002 v3 (`sgo/spec/CHANGELOG.md`) |
| admission identity | admission digest `3eb9134114d04921` (SHA-256 of `sgo/SGO_SPEC_ADMISSION.json`, recorded by the toolchain and every binary); registry `rows_digest` `f7289798757c5538` (R12) |
| decisions | D75–D84 approved 2026-11-02 |
| reviewer | `sgolang-reviewer` (`job:note2/id:77bfab38c5a3`), fresh context, linux/amd64 (kernel 7.2); review brief `sgo/docs/w12-review-brief.md`; every finding, fix, test, mutation, and verification: `review-findings.json` |
| upstream_commit | `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` (go1.27.3) |
| next_dependency_ready_task | none (W12 is the last work package); publishing a release is the owner's decision (D78) |

## Review findings (Q01)

| ID | Severity | Fix | Status |
|---|---|---|---|
| R1 | blocker | trust keyed on (import path, file name, digest) in compiler, importer, linker `f71e01af75` | verified |
| R2 | blocker | S-Go binaries read no runtime environment setting (linker mark) `bc8027270a` | verified |
| R3 | blocker | compile/link flag allowlists, required consistent `-buildid` `098959aa24` | verified |
| R4 | blocker | regular files only (`NotRegularFile`), file syscalls off the scheduler `426276e16c` | verified |
| R5 | blocker | `return` operands into captured named results rejected `f35be2e975` | verified |
| R6 | major | `openat2` beneath the grant on linux, `PathRaced`, truncate after verify `876daac0a8`, reopened and fixed `f80338d74e` | verified |
| R6b | minor | `openat2` availability decided once by a probe `085ce6e4df` | verified |
| R7 | minor | evidence written only with `SGO_RECORD=1`; CI clean-tree steps `a8e9d7feab`, `f247f1219a` | verified |
| R8 | minor | TCB cache entries verified against a digest manifest on reuse `a8e9d7feab` | verified |
| R9 | minor | no allocation under an S-Go runtime lock; `lock-alloc-check.py` in CI `d329fa71b7`, `2bb64c63b6`, `b59858ca16` | verified |
| R10 | minor (doc) | lock order in runtime-design §5 `2bb64c63b6` | verified |
| R11 | minor | a raced walk-mode `Create` that created a file reports `PathRaced` `b8176fd831` | verified |
| R12 | minor (doc) | one meaning of admission digest; `spec-tool.py check` asserts it `b8176fd831` | verified |
| R13 | minor | `make-release.py` refuses unknown arguments, never writes into the tree `17054fecb6` | fixed, re-check pending |

The reviewer's closing summary lists what was not reviewed or only partly: the deep socket error and partial-write paths, the darwin syscall layer (no darwin host), the lab scheduler internals, the model as an independent model, the compiler lowering beyond conformance, and linux/arm64 (CI only). These are stated limits of this review, not claims.

## Acceptance cases

| Case | Evidence |
|---|---|
| Q01 independent review | the table above; `review-findings.json` (findings, self-findings SR1–SR4, doc items, the reviewer's commands and results) |
| Q02 negative-fixture honesty | `sgo/scripts/test-negative.py` (D79, `d13f44aef4`): 15 representatives of the plan §12.3 categories fail with their specified code and clause; a self-test changes each pinned code (lab refusals: swaps the runner's codes) and requires a failure. In `sgo-w00` CI |
| Q03 release reproduction | `sgo-release` (D78) run 41506985922: archives on macos-14, ubuntu-24.04, ubuntu-24.04-arm; source archive, combined diff against the pin, `SHA256SUMS`; `verify-release.sh` on fresh runners with no checkout follows `INSTALL.md` (install, examples run/test/sweep, shipped trace replays and diverges after the fix, Go input refused) on all three; bootstrap from the source archive (stage-0 and the conformance lane) on linux/amd64. Verified locally by the reviewer on linux/amd64 |
| Q04 explicit registry | `sgo/SGO_SPEC_ADMISSION.json` 0.7.0; `sgo/GUARANTEES.json` and `guarantee-check.py` (in `sgo-spec` CI): each published guarantee rests on admitted rows, frozen clauses, and passed acceptance cases (SR2) |
| S04 adopted clauses | `adoption-check.py` with its mutation cases (`test-mutations.py`) |
| S11 TCB | `test-tcb.py`: edited or added TCB inputs fail with `SGO_E_TCB_DIGEST`; byte copies under user paths go through admission (R1); an altered cached archive is rebuilt (R8); a pinned archive under a user import path is refused (D-R1b) |
| S17 no legacy path | `legacy-audit.py` (D77): every flag of compile, asm, link classified and refused unless the driver writes it (121 refused); every environment input classified, refused ones refused, inert ones byte-identical; the driver's 16 flags specified and unknown flags refused by 8 commands; the release archive holds only the four tools and the TCB sources; `make-release.py` refuses unknown arguments (R13) |
| S18 amendments | `spec-tool.py` refuses a frozen clause changed without a version bump (it did, for SGO-FILE-002, hence v3); `test-mutations.py` amendment cases |
| S20 scope of claims | `guarantee-check.py` refuses proof claims and checks each guarantee's scope vocabulary and model bounds |

## Stress (D80)

| Platform | Lab (30 cases × 1000 seeds, twice, + examples) | Production `-count=20 -cpu=1,4,16` | Race `-count=4 -cpu=1,4,16` |
|---|---|---|---|
| darwin/arm64 (local) | ok, 2m50s | ok, 129 s | ok, 767 s, no `DATA RACE` |
| linux/amd64, linux/arm64 | `sgo-stress` run 41523619092: PENDING | PENDING | PENDING |

## Commands

    sgo/scripts/legacy-audit.py
    sgo/scripts/test-negative.py
    sgo/scripts/guarantee-check.py
    sgo/scripts/stress.py --seeds 1000 --count 20 --cpus 1,4,16
    sgo/scripts/make-release.py --out DIR && (cd DIR && shasum -a 256 sgo-*.tar.gz > SHA256SUMS) && sgo/scripts/verify-release.sh DIR

## Known gaps

- T07 allocation counts unmeasured (D82); F03–F05 unrun (W11 closed investigated, not adopted).
- The walk-mode resolution (darwin, linux < 5.6, seccomp) admits the raced outcomes stated in SGO-FILE-002 v3; linux 5.6+ resolves in the kernel.
- The TCB cache check is integrity against corruption, not tamper resistance (SGO-TCB-002 v2, D-R8).
