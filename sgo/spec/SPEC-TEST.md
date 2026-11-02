# SPEC-TEST — Test harness: golden program tests (subset 0) and later a testing package

Subset 0. Derived from decision D10 and `SGO_SPEC_SUBSET0.md` §16.

### SGO-TEST-001 — Conformance cases
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-ERR-001, SGO-ERR-002, SGO-PROC-002, SGO-DRAIN-003; D10.
- **grammar_and_types:** A conformance case is a directory `sgo/testdata/conformance/<name>/` containing an `sgo.mod`, one or more `.sgo` packages, and `expect.json` with: `"kind"` (`"run"` or `"reject"`); for `run`: `"exit"` (integer), `"stdout"` (exact string) or `"stdout_any_of"` (list of exact strings, for cases with allowed nondeterminism), and optionally `"stderr_kind"` (the diagnostic `kind`); for `reject`: `"diagnostics"`, a list of `{"code", "position"}`; plus `"clauses"` (clause IDs the case checks) and optionally `"timeout_ms"` and `"expect_watchdog"`.
- **preconditions:** n/a — harness rule.
- **evaluation_and_value_semantics:** The runner builds the case with `sgo build`, runs it (for `run` cases) with an empty environment and no arguments, and compares exit status, stdout, and the diagnostic kind exactly; for `reject` cases it compares diagnostic codes and positions exactly. A case passes only if every expectation matches.
- **ownership_and_lifecycle:** n/a — harness rule.
- **effect_and_capability_requirements:** n/a — harness rule.
- **concurrency_and_memory_order:** n/a — harness rule.
- **allowed_nondeterminism:** Only as listed in `stdout_any_of`; each listed output must be justified by the clause's allowed_nondeterminism field.
- **failure_and_partial_progress:** A mismatch, build failure of a `run` case, or watchdog kill fails the case (SGO-DRAIN-003).
- **cancellation_and_cleanup:** n/a — harness rule.
- **compile_link_checks:** The spec tool checks that every `fixture:<name>` referenced by a frozen clause exists; the runner checks that every case names at least one existing clause.
- **runtime_guards:** n/a — harness rule.
- **tcb_assumptions:** n/a — host tooling.
- **positive_tests:** S03; fixture:proc-hello.
- **negative_tests:** S03; fixture:lex-bare-go.
- **implementation_coverage:** W01, W03: `sgo/scripts/conformance.py`.
- **executed_evidence:** conformance fixtures lex-bare-go, proc-hello (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w01/README.md`; `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-TEST-002 — sgo test
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-TOOL-001, SGO-TOOL-002, SGO-PROC-001, SGO-PROC-002, SGO-AUTH-002, SGO-FILE-002, SGO-ERR-002, SGO-IMP-002, SGO-TEST-003; D62, D69.
- **grammar_and_types:** `sgo test [-v] [-run regexp] [packages]` (packages as in SGO-TOOL-002). A test file is a file of the package directory whose name ends in `_test.sgo`; it has the package's own package clause and is part of the package only under `sgo test` of that package. A test is a top-level function `func TestX(t *sgo.T) error` in a test file, where `X` is empty or does not start with a lower-case letter; `sgo.T` is a capability handle (SGO-TYPE-005) with methods `Context() *sgo.Context`, `TempDir() *sgo.Dir`, and `Log(s string)`. A function whose name has that form but another signature is `SGO_E_TEST_SIGNATURE` at its declaration. The package must not be `main` or `sgo` (`SGO_E_USAGE`).
- **preconditions:** As SGO-TOOL-002.
- **evaluation_and_value_semantics:** For each package, `sgo test` builds one test binary: the package with its test files and its module dependencies (without their test files), and a generated main package that calls `sgo.StartTest(root)` and runs the selected test with the root task's `T`. Its build ID names the configuration `test`. Each test selected by `-run` (all by default), in source order, runs in a fresh process of that binary started in the module root, with the test's index and a fresh empty temporary directory passed on descriptor 3 (never through the environment or arguments). The process has the module's grants (SGO-AUTH-002, SGO-FILE-002) plus the temporary directory as an extra read-write directory grant reachable only through `T.TempDir`. The test passes iff its process exits with status 0: the test returned nil and left no obligation unresolved (SGO-TEST-003). `sgo test` prints `--- PASS: TestX (seconds)` or `--- FAIL: TestX (seconds)` per test, followed for a failed test (or for every test with `-v`) by the process's output indented by four spaces; then `ok` or `FAIL` with the import path and total time, or `?` and `[no test files]`. `T.Log` writes its argument and a newline to the test's output.
- **ownership_and_lifecycle:** `T` belongs to the root task; its methods are root-only (SGO-CAP-001). The temporary directory is removed by `sgo test` after the test's process ends, however it ends. Test binaries and intermediate files are removed at exit.
- **effect_and_capability_requirements:** A test has the root task's ceiling of the module's grants; `T.TempDir` needs no grant. `sgo.StartTest` is admitted only in the generated main package (compiled with `-sgotestmain`, D69); anywhere else it is `SGO_E_UNADMITTED`, and called other than by the root task of a test binary it is a `MissingCapability` failure (SGO-DEF-003). A production binary never reads descriptor 3 as a test selection.
- **concurrency_and_memory_order:** Tests of a package run one at a time, each in its own process; no state is shared between tests except the module's granted directories.
- **allowed_nondeterminism:** The reported times; a test's own allowed nondeterminism.
- **failure_and_partial_progress:** A test that fails does not stop the others. A test whose process has not ended 60 seconds after it started is killed and fails (`test killed after 1m0s`). The exit status of `sgo test` is 1 if any test failed or any package did not build, 2 for usage errors.
- **cancellation_and_cleanup:** The watchdog kill is the only cancellation; the temporary directory is removed after it.
- **compile_link_checks:** The test files are compiled under the same admission and checks as the rest of the package; `SGO_E_TEST_SIGNATURE` as above. The generated main package records the configuration `testmain` in its identity (SGO-IMP-002); the linker accepts a `testmain` package only as the main package of a test binary (`-sgotest`), and a test binary only with one (`SGO_E_SPEC_MISMATCH`).
- **runtime_guards:** The runtime of a test binary refuses to start without a test selection on descriptor 3 (`Defect`).
- **tcb_assumptions:** `runtime/sgo_test_boot.go` and `src/sgo/test.go` (TARGET_TCB.json).
- **positive_tests:** T04; fixture:test-pass-fail, fixture:test-tempdir, fixture:test-run-filter, fixture:test-files-not-in-build; `sgo/scripts/test-test.py` (pass, -run, -v, TempDir fresh per test and removed, module grants).
- **negative_tests:** T04; fixture:test-starttest-denied, fixture:test-signature, fixture:test-main-refused, fixture:test-pass-fail; `sgo/scripts/test-test.py` (returned error, checked failure, wrong signature, unknown flag, test files absent from build and check, watchdog); `sgo/scripts/test-flags.py` (`-sgotestmain` and `-sgotest` refusals).
- **implementation_coverage:** `src/cmd/sgo/test.go`, `src/cmd/sgo/module.go` (test files), linker `-sgotest`, `src/runtime/sgo_test_boot.go`, `src/sgo/test.go`.
- **executed_evidence:** `sgo/evidence/w10/README.md` (the scripts and conformance cases above on darwin/arm64, linux/amd64, linux/arm64).
- **supported_execution_configurations:** production; tests under the lab configuration are not in 0.6.0.

### SGO-TEST-003 — Test quiescence
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-TEST-002, SGO-PERMIT-002, SGO-SYNC-002, SGO-FILE-003, SGO-ERR-002; D64.
- **grammar_and_types:** n/a — runtime rule of test binaries.
- **preconditions:** A test binary's root task has returned from the test.
- **evaluation_and_value_semantics:** When the test returns, every obligation the root task still holds — send permits neither committed nor aborted, limiter tokens not released, and I/O handles not closed — is resolved by the runtime as at any task end (each recorded as abandoned) and counted. If the test returned nil and the count is not zero, the test fails with the diagnostic record of kind `LeakedWork`, message `the test left N obligation(s) unresolved (D64)`, whose chain lists the abandoned obligations in order; the process exits with status 1. If the test returned an error or failed, that outcome stays primary and the abandoned obligations appear in its chain.
- **ownership_and_lifecycle:** Obligations of child tasks are resolved when those tasks end (nursery join), so at the end of a test only the root's remain.
- **effect_and_capability_requirements:** n/a — runtime rule.
- **concurrency_and_memory_order:** The check runs on the root task after every nursery of the test has joined.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** As above; production binaries are not affected.
- **cancellation_and_cleanup:** The resolution closes leaked handles and aborts leaked permits before the process exits.
- **compile_link_checks:** n/a — runtime rule.
- **runtime_guards:** `LeakedWork` as above.
- **tcb_assumptions:** `runtime/sgo_test_boot.go`.
- **positive_tests:** T04; fixture:test-leaked-work; `sgo/scripts/test-test.py` (a test that resolves its obligations passes).
- **negative_tests:** T04; fixture:test-leaked-work; `sgo/scripts/test-test.py` (a test leaving a permit fails with LeakedWork and a PermitAbandoned chain).
- **implementation_coverage:** `sgoTestEnd` in `src/runtime/sgo_test_boot.go`.
- **executed_evidence:** `sgo/evidence/w10/README.md` (the scripts and conformance cases above on darwin/arm64, linux/amd64, linux/arm64).
- **supported_execution_configurations:** production test binaries.
