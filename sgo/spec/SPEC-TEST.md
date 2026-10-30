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
- **implementation_coverage:** none yet — runner in W01 step 6, execution from W03.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
