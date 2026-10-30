# SPEC-TOOL — Driver/compiler/linker/export/cache and source-tool admission

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §16, the user's naming decision (driver `sgo`), resolution R7, plan §10.2.

### SGO-TOOL-001 — The sgo driver
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-IMP-001, SGO-IMP-002, SGO-DRAIN-002, SGO-ERR-001, SGO-BUILD-001; R7.
- **grammar_and_types:** The S-Go driver is the executable `sgo`. Subset-0 commands: `sgo build [-o out] [-drain-budget=<ms>] [-target=<os>/<arch>] [packages]`, `sgo run [-drain-budget=<ms>] <main package>`, `sgo version`, `sgo fmt [files]`. Any other command or flag is rejected. In particular there is no flag, environment variable, file name, build tag, or pragma that changes the language, disables a check, selects a different compiler, or enables Go compatibility (S01).
- **preconditions:** The working directory is inside a module (SGO-IMP-001).
- **evaluation_and_value_semantics:** `build` compiles every package of the requested main package's import graph under the current spec, then links a binary whose build metadata records spec version, admission digest, drain budget, target, and toolchain identity. `run` builds into a temporary location and executes the binary, propagating its exit status. `version` prints the S-Go version, spec version, admission digest, and upstream base commit. `fmt` formats `.sgo` files and refuses any file it cannot parse as S-Go.
- **ownership_and_lifecycle:** Build caches are keyed by every identity of SGO-IMP-002 and kept separate from any Go build cache.
- **effect_and_capability_requirements:** The driver reads the module and writes outputs; it never downloads toolchains or modules (no network access) and never executes a `go` binary.
- **concurrency_and_memory_order:** n/a — toolchain.
- **allowed_nondeterminism:** None in outputs: builds are reproducible for identical inputs (W00 B02 shows the base toolchain is).
- **failure_and_partial_progress:** Any error leaves no output binary.
- **cancellation_and_cleanup:** n/a — toolchain.
- **compile_link_checks:** Unknown commands and flags: `SGO_E_USAGE`. `-target` accepts only `darwin/arm64`, `linux/amd64`, `linux/arm64` (default: the host); any other value is `SGO_E_TARGET` (SGO-BUILD-001). Inputs that are not `.sgo` files of the module: `SGO_E_UNADMITTED`.
- **runtime_guards:** n/a — toolchain.
- **tcb_assumptions:** The driver is a host tool built by the pinned bootstrap (stage H1).
- **positive_tests:** S01, S15; fixture:proc-hello.
- **negative_tests:** S01, S15, S19; fixture:tool-compat-flag, fixture:tool-go-file-input.
- **implementation_coverage:** gate level only — `sgo/scripts/sgo-target` denies every request (W00); the driver is W03.
- **executed_evidence:** W00 gate checks passed on all three targets.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
