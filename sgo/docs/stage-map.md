# Build stage map (W00, test case B04)

Verified against `src/cmd/dist/build.go` (`cmdbootstrap`) and `src/cmd/dist/buildtool.go` (`bootstrapBuildTools`) at upstream `go1.27.3` (`2aae1000ea1d4b28923b96db28acf4e3b99d6aff`), and against the B01 build log, which prints each stage.

## 1. Inherited upstream pipeline (`make.bash` → `dist bootstrap`)

| # | Stage | Built from | Compiled by | Input language | S-Go classification |
|---|---|---|---|---|---|
| 0 | Bootstrap Go | official go.dev archive `go1.26.10` (UPSTREAM.lock.json) | — (binary) | — | **External host tool.** Pinned by SHA-256. Never a target. |
| 1 | `cmd/dist` | fork `src/cmd/dist` | bootstrap Go | Go | **Host tool**, host-built. |
| 2 | toolchain1 (`compile`, `link`, `asm`, `cgo`, …) | fork sources copied to `pkg/bootstrap` with rewritten imports (`bootstrapDirs`) | bootstrap Go | Go | **Host tool, host-built. This is the stage-0 S-Go compiler** once the fork's frontend changes. |
| 3 | `go_bootstrap` (+ `runtime`, `time/tzdata`) | fork `src/cmd/go`, `src/runtime` | **toolchain1** (fork compiler) | Go | **Self-hosted.** Breaks when the fork compiler rejects Go source. |
| 4 | toolchain2 | fork `cmd/*` tools | **toolchain1** via `go_bootstrap` | Go | **Self-hosted.** Same. |
| 5 | toolchain3 | fork `cmd/*` tools | **toolchain2** | Go | **Self-hosted.** Same. |
| 6 | `std` + `cmd` install | fork `src/...` | **toolchain3** | Go | **Self-hosted.** Std is Go source, not admitted S-Go (plan §8.4). |

Stages 3–6 use the fork's own compiler to compile Go source. Under the constitution (§5) and plan §10.2 the S-Go target compiler must not accept Go source, even to rebuild itself. As soon as the frontend starts rejecting Go (W03), stages 3–6 cannot work as they are. The fix is **not** a bootstrap parser mode, a file-extension or pragma exemption, or a package-name allowlist (all forbidden; test case S15).

## 2. S-Go stage design

| Stage | Built by | Purpose | Status |
|---|---|---|---|
| **H0** host bootstrap | — | Pinned go.dev binary. | Implemented (`fetch-bootstrap.sh`). |
| **H1** host tools: `dist`, `compile`, `link`, `asm`, and later the S-Go driver | H0 only | Host-built implementation of the S-Go toolchain. Never compiled by the S-Go compiler. | toolchain1 path implemented as `dist bootstrap -sgo-stage0` (`build-stage0.sh`), which also builds the S-Go driver `cmd/sgo` into `pkg/tool/<host>/sgo` (W03). The driver compiles the target TCB (closure of `runtime` and `sgo`, files selected with `go/build` for the target and the toolchain's default experiments) and the module's packages with the fork's `compile`/`asm`/`link`, links, and caches TCB archives by tool, target, and source digests (`$SGO_CACHE/sgo-driver` or the user cache directory). It never runs a `go` binary. |
| **T0** target runtime and TCB | H1 compiler, **admission lane for TCB sources** | `runtime` and its internal dependencies are Go source today. They enter the target only as TCB code admitted by exact source digest and individual contracts (`TARGET_TCB.json`, plan §8.1), not by package name. | Open: W01 specifies the TCB lane; W02/W03 implement it. |
| **T1** `sgo` package and user programs | H1 compiler | S-Go subset-0 programs, admitted by `SGO_SPEC_ADMISSION.json`. | W03. Until then the target gate denies everything. |
| **H1-test** host test lane | H0 only | Unit tests of host packages (compiler code) copied into a temporary module and run with the bootstrap (`test-host-pkg.sh`). Upstream-internal test helpers: `internal/testenv` is shimmed (`MustHaveGoBuild` skips, visibly); other internal imports exclude the test file. | Implemented for std-only packages (e.g. `cmd/compile/internal/syntax`). Packages importing `cmd/...` need import rewriting like `dist` (later). |
| **R** reference | H0 → upstream `make.bash` in an untouched worktree | Upstream comparison only (B01–B03). | Implemented (`build-reference.sh`, `test-reference.sh`). |

Consequences:

1. The fork stops using `make.bash` and stages 3–6 for its own toolchain. H1 builds every host tool, including the future S-Go driver, with H0, the way toolchain1 is built today. The upstream `go` command is a host tool for building host code at most; it is never the S-Go target driver (no stock fallback, test case B05).
2. Self-hosting means porting the implementation to admitted S-Go. Until then, releases must say "host-built" (plan §10.2).
3. Separate identities and caches (enforced by the scripts):

| Identity | Location | Cache |
|---|---|---|
| H0 | `$SGO_CACHE/bootstrap/go1.26.10-<platform>/go` | none (binary) |
| R | `$SGO_CACHE/reference/go1.27.3-<label>` (detached worktree at the locked commit) | `<worktree>/pkg/obj/go-build` (dist sets `GOCACHE`) |
| H1 | the fork checkout, `pkg/tool/<os>_<arch>/` | `<fork>/pkg/obj/go-build` |
| T0/T1 | W03 | W03: separate cache keyed by spec/admission digest (plan §1.3) |

## 3. Open items carried forward

- **T0 runtime admission** is the hardest boundary: the runtime is large Go source using `unsafe`, assembly, and linkname. W01 must decide its TCB contract form. This does not license user-level `unsafe` (SPEC-NATIVE stays denied).
- **Cross targets (D22).** H1 is host-built for the host, but its compiler can emit code for all three targets. Linux execution evidence (linux/amd64, linux/arm64) needs Linux hosts or CI runners; cross-compilation alone is not evidence.
