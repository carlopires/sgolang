# SPEC-TOOL — Driver/compiler/linker/export/cache and source-tool admission

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §16, the user's naming decision (driver `sgo`), resolution R7, plan §10.2.

### SGO-TOOL-001 — The sgo driver
- **version:** 3
- **status:** frozen
- **reference_closure:** SGO-IMP-001, SGO-IMP-002, SGO-DRAIN-002, SGO-ERR-001, SGO-BUILD-001; R7.
- **grammar_and_types:** The S-Go driver is the executable `sgo`. Subset-0 commands: `sgo build [-o out] [-drain-budget=<ms>] [-target=<os>/<arch>] [packages]`, `sgo run [-drain-budget=<ms>] <main package>`, `sgo version [-m binaries]`, `sgo fmt [files]`; spec 0.4.0 adds `sgo lab run|replay|sweep` (SGO-LAB-002) and spec 0.6.0 adds `sgo check` (SGO-TOOL-002) and `sgo test` (SGO-TEST-002). Any other command or flag is rejected. In particular there is no flag, environment variable, file name, build tag, or pragma that changes the language, disables a check, selects a different compiler, or enables Go compatibility (S01).
- **preconditions:** The working directory is inside a module (SGO-IMP-001).
- **evaluation_and_value_semantics:** `build` compiles every package of the requested main package's import graph under the current spec, then links a binary whose build metadata records spec version, admission digest, drain budget, target, and toolchain identity. `run` builds into a temporary location and executes the binary, propagating its exit status. `version` prints the S-Go version, spec version, admission digest, and upstream base commit; `version -m` prints the build metadata each named binary records (SGO-BUILD-001), one field per line, and refuses a file that has no S-Go build ID. `fmt` formats `.sgo` files and refuses any file it cannot parse as S-Go.
- **ownership_and_lifecycle:** Build caches are keyed by every identity of SGO-IMP-002 and kept separate from any Go build cache.
- **effect_and_capability_requirements:** The driver reads the module and writes outputs; it never downloads toolchains or modules (no network access) and never executes a `go` binary.
- **concurrency_and_memory_order:** n/a — toolchain.
- **allowed_nondeterminism:** None in outputs: builds are reproducible for identical inputs (W00 B02 shows the base toolchain is).
- **failure_and_partial_progress:** Any error leaves no output binary.
- **cancellation_and_cleanup:** n/a — toolchain.
- **compile_link_checks:** Unknown commands and flags: `SGO_E_USAGE`. `-target` accepts only `darwin/arm64`, `linux/amd64`, `linux/arm64` (default: the host); any other value is `SGO_E_TARGET` (SGO-BUILD-001). Inputs that are not `.sgo` files of the module: `SGO_E_UNADMITTED`.
- **runtime_guards:** n/a — toolchain.
- **tcb_assumptions:** The driver is a host tool built by the pinned bootstrap (stage H1).
- **positive_tests:** S01, S15; fixture:proc-hello; `sgo/scripts/test-identity.py` (`version -m` on programs with and without a trust record, and on a file with no build ID).
- **negative_tests:** S01, S15, S19; fixture:tool-compat-flag, fixture:tool-go-file-input.
- **implementation_coverage:** gate level only — `sgo/scripts/sgo-target` denies every request (W00); the driver is W03.
- **executed_evidence:** W00 gate checks passed on all three targets.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-TOOL-002 — sgo check
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-TOOL-001, SGO-IMP-001, SGO-IMP-002, SGO-ERR-001; D63.
- **grammar_and_types:** `sgo check [-json] [packages]`, where a package is a directory of the module or `./...` (every package of the module, excluding `testdata`, `_`- and `.`-prefixed directories, and nested modules); the default is the current directory. Any other flag is `SGO_E_USAGE`.
- **preconditions:** The working directory is inside a module (SGO-IMP-001).
- **evaluation_and_value_semantics:** `check` runs the same front end as `build` — parsing, admission, type checking, and every compile-time S-Go check — on each requested package and its module dependencies, in dependency order, and stops before code generation. It reports exactly the diagnostics `build` would report for the same package (codes, positions, messages; SGO-ERR-001), and exits 0 iff `build` of that package would compile. `_test.sgo` files are not part of any package for `check` (SGO-TEST-002). With `-json`, each diagnostic is one JSON object per line with `file`, `line`, `column`, `code`, `message`.
- **ownership_and_lifecycle:** `check` writes nothing in the module and produces no binary; intermediate export data stays in a temporary directory removed at exit.
- **effect_and_capability_requirements:** As SGO-TOOL-001: no network access, no `go` binary.
- **concurrency_and_memory_order:** n/a — toolchain.
- **allowed_nondeterminism:** None: the output for identical inputs is identical.
- **failure_and_partial_progress:** A package whose dependency does not check is not checked; its dependents report nothing further. The exit status is 1 if any package has a diagnostic, 2 for usage errors.
- **cancellation_and_cleanup:** n/a — toolchain.
- **compile_link_checks:** All of the compile-time checks of the packages checked; no link-time check (SGO-IMP-002 contract and TCB digest checks of the link are not run).
- **runtime_guards:** n/a — toolchain.
- **tcb_assumptions:** The driver and compiler as in SGO-TOOL-001.
- **positive_tests:** T03; fixture:test-files-not-in-build; `sgo/scripts/test-check.py` (every conformance case: check and build agree).
- **negative_tests:** T03; `sgo/scripts/test-check.py` (rejection cases report identical diagnostics; a dependency that does not check stops its dependents).
- **implementation_coverage:** `src/cmd/sgo/check.go`; compiler flag `-sgocheck` (stops after type checking, writing export data only).
- **executed_evidence:** `sgo/evidence/w10/README.md` (the scripts and conformance cases above on darwin/arm64, linux/amd64, linux/arm64).
- **supported_execution_configurations:** production (lab binaries are produced only by `sgo lab`).
