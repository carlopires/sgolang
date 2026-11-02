# W10 plan — tooling, admission conformance, examples, performance (spec 0.6.0)

Decisions D61–D68 (approved 2026-11-02). Acceptance: T01–T08, S01, S09, S14, S15, S18, S19.

## Surface

- `sgo check [-json] [packages]` (D63): the build front end only (parse, admission, type checking, S-Go checks); the same diagnostics, codes, and positions as `sgo build`; no object files, no binary.
- `sgo test [-run regexp] [-v] [packages]` (D62): test files `*_test.sgo` in the package; tests `func TestName(t *sgo.T) error`; `t.Context() *sgo.Context` (the test's root, with the module's grants), `t.TempDir() *sgo.Dir` (fresh `rw`, removed after the test), `t.Log(s string)`. One test binary per package; each selected test runs in its own process as that process's root task. A test passes iff it returns nil, its root ends cleanly, and nothing is left (D64); otherwise it fails with its diagnostic record. Output: one line per test (`ok`/`FAIL`, name, duration) and a summary; exit 1 if any test fails.
- `sgo lab test` (later in W10): the same tests under the lab, one seed per test, trace saved on failure.
- Release archive (D61): `sgo/scripts/release.sh` builds `sgo-<version>-<os>-<arch>.tar.gz`; CI extracts it on a fresh runner and builds and tests the examples from it.

## `sgo test` design

1. **Discovery.** The driver parses the package's `*_test.sgo` files (same parser), and collects top-level `func TestX(t *sgo.T) error` with `X` starting with an upper-case letter. A test file is in the package's own namespace (no external test packages in 0.6.0).
2. **Test main.** The driver generates the package's test main in S-Go source: `func main(root *sgo.Context) error` that calls the test named by the runtime's test selector (`sgo.T` holds the root). The generated file goes through the same admission as user code; it is not a trusted path.
3. **Selection.** The test binary is linked with the list of tests; the driver runs it once per selected test with the test's index on a dedicated file descriptor (as the lab manifest, D41), never through the environment (S15). A test binary started without a selection exits as a usage error.
4. **TempDir.** For each test the driver creates a temporary directory and the test binary receives it as an extra `rw` directory grant named by the runtime (not by `sgo.mod`), so the grant set of a test is the module's grants plus its own directory; the driver removes it after the test.
5. **Quiescence (D64).** After the test function returns and the root nursery ends, the runtime checks its counters (tasks other than the root, sleepers, socket waiters, open handles, unresolved permits and tokens); a non-zero counter fails the test with `LeakedWork` and the counts. By construction these are zero at a clean root end, so the check guards the runtime's own obligations.
6. **Timeout.** Each test has a watchdog (default 60 s, `-timeout`); a test that exceeds it is killed and reported as `TimedOut`.

## Steps

1. `sgo check` and its T03 test (check and build agree on every conformance case).
2. `sgo test`: `sgo.T`, discovery, generated main, selection on a descriptor, per-test process, TempDir, quiescence, timeout; T04 tests (failure, leak attempt, cross-test isolation).
3. Spec 0.6.0 drafts (SPEC-TOOL amendment, SPEC-TEST addition) → user approval → freeze; fixtures.
4. Examples (D66) with tests; release archive (D61) and the clean-install CI job.
5. T01, T02, S01, S09, S15, S18, S19 isolation and rejection tests (extend test-identity, test-flags, test-lab).
6. Benchmark harness and report (D67); intentional rejection report over the upstream language tests (D68).
7. Evidence `sgo/evidence/w10/README.md`, ledgers, CI.
