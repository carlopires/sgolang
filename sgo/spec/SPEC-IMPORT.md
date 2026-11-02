# SPEC-IMPORT — Transitive package/body/initialization contracts and artifact identity

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §14, decision D15, plan §1.3.

### SGO-IMP-001 — User packages and the module
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-BIND-001, SGO-BIND-002, SGO-BIND-005, SGO-TOOL-001; D15.
- **grammar_and_types:** A program is built from one module: a directory tree whose root contains an `sgo.mod` file declaring `module <path>` and `sgo <spec-version>`. Package import paths are `<module path>/<relative directory>`. A module path may not equal, contain, or lie under the import path of a target TCB package (for example `sgo`, `runtime`, or `internal/...`), since its packages would shadow trusted ones (D38). Import cycles are a compile error, reported once, at the import declaration in the cycle's lexically first package (by import path) that names the next package of the cycle. Every package of the module is compiled under subset 0; there is no other mode, and no package outside the module except `sgo` can be imported.
- **preconditions:** `sgo.mod` names a spec version the toolchain implements; otherwise the build fails.
- **evaluation_and_value_semantics:** n/a — build structure; subset 0 has no package initialization.
- **ownership_and_lifecycle:** n/a — build structure.
- **effect_and_capability_requirements:** Packages grant no authority (SGO-CAP-001).
- **concurrency_and_memory_order:** n/a — build structure.
- **allowed_nondeterminism:** None: the build result does not depend on directory listing order.
- **failure_and_partial_progress:** Any package failing admission fails the whole build; no partial program is linked.
- **cancellation_and_cleanup:** n/a — build structure.
- **compile_link_checks:** `SGO_E_USAGE` at the module directive for a module path that collides with a TCB package path; `SGO_E_IMPORT_CYCLE`, `SGO_E_SPEC_VERSION`, `SGO_E_UNADMITTED` (out-of-module import).
- **runtime_guards:** n/a — build structure.
- **tcb_assumptions:** n/a — toolchain only.
- **positive_tests:** S09; fixture:imp-user-package.
- **negative_tests:** S09, S15; fixture:imp-cycle, fixture:imp-spec-version-mismatch, fixture:tcb-module-named-sgo.
- **implementation_coverage:** W03: `src/cmd/compile/internal/noder/sgo_identity.go`, `src/cmd/compile/internal/sgoadmit/identity.go`, `src/cmd/sgo/module.go`, `src/cmd/sgo/build.go`.
- **executed_evidence:** conformance fixtures imp-cycle, imp-spec-version-mismatch, imp-user-package, tcb-module-named-sgo (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-IMP-002 — Contract identity of compiled packages
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IMP-001, SGO-BIND-005, SGO-TOOL-001; plan §1.3; SPEC-1, SPEC-3.
- **grammar_and_types:** n/a — artifact rule.
- **preconditions:** n/a — artifact rule.
- **evaluation_and_value_semantics:** Each compiled package records in its export data and object file: the spec version, the admission digest, the package's contract digest (SHA-256 over its exported declarations and their types), the contract digests of every package it imports, and the S-Go ABI identity. The linker and the importing compiler verify all of them.
- **ownership_and_lifecycle:** n/a — artifact rule.
- **effect_and_capability_requirements:** n/a — artifact rule.
- **concurrency_and_memory_order:** n/a — artifact rule.
- **allowed_nondeterminism:** None: digests are deterministic functions of their inputs.
- **failure_and_partial_progress:** Any mismatch, or an object file without S-Go identity (including every stock Go object), fails the build.
- **cancellation_and_cleanup:** n/a — artifact rule.
- **compile_link_checks:** `SGO_E_STALE_CONTRACT` (dependency contract digest changed), `SGO_E_SPEC_MISMATCH` (spec or admission digest differs), `SGO_E_FOREIGN_OBJECT` (no S-Go identity). Build-cache keys include all of these identities.
- **runtime_guards:** n/a — link time.
- **tcb_assumptions:** n/a — toolchain only.
- **positive_tests:** S09; fixture:imp-user-package.
- **negative_tests:** S09, S15, S18; fixture:imp-stale-contract (a rebuild after a dependency's exported signature changes must type-check against the new contract, never reuse the stale one), fixture:imp-stock-object (a stock Go object as build input is rejected); link-level `SGO_E_STALE_CONTRACT` and `SGO_E_FOREIGN_OBJECT` are compiler/linker unit tests (W03).
- **implementation_coverage:** gate level only — `sgo-target` denies linking stock objects (W00); compiler/linker checks are W03.
- **executed_evidence:** W00 check `linking a stock object is denied` passed on all three targets (gate level).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
