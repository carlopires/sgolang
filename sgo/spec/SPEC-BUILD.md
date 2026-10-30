# SPEC-BUILD — Host/reference/target separation and truthful self-hosting status

Subset 0. Derived from plan §10.2, stage map, decision D22, invariant SPEC-5.

### SGO-BUILD-001 — Build stages, targets, and identity
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-TOOL-001, SGO-TCB-002, SGO-IMP-002; D21, D22; SPEC-5.
- **grammar_and_types:** n/a — build rule.
- **preconditions:** n/a — build rule.
- **evaluation_and_value_semantics:** The S-Go toolchain (driver, compiler, linker, assembler) is host code built only by the pinned bootstrap recorded in `sgo/UPSTREAM.lock.json` (stage H1); it is never compiled by the S-Go compiler, so releases are reported as host-built. Target code is the runtime (admitted per SGO-TCB-002), the `sgo` package, and user programs. Admitted targets: darwin/arm64, linux/amd64, linux/arm64. Every built binary records its target, spec version, admission digest, toolchain identity, and upstream base commit.
- **ownership_and_lifecycle:** Host bootstrap, upstream reference builds, host tools, and target artifacts use separate directories and caches (stage map §2).
- **effect_and_capability_requirements:** n/a — build rule.
- **concurrency_and_memory_order:** n/a — build rule.
- **allowed_nondeterminism:** None: builds are reproducible for identical inputs.
- **failure_and_partial_progress:** A build for a non-admitted target fails. A missing or mismatched bootstrap fails the toolchain build; no other toolchain is substituted.
- **cancellation_and_cleanup:** n/a — build rule.
- **compile_link_checks:** `SGO_E_TARGET` for a non-admitted target. A target-dependent behavior difference among the three targets is a specification defect, not an allowed variation.
- **runtime_guards:** n/a — build rule.
- **tcb_assumptions:** n/a — host tooling.
- **positive_tests:** B01, B02, B04, S15; W00 evidence.
- **negative_tests:** B05, S15; fixture:tool-unadmitted-target.
- **implementation_coverage:** W00 — bootstrap pin, reference and stage-0 builds, separate caches, PATH guard (scripts under `sgo/scripts/`).
- **executed_evidence:** W00 B01–B05 passed on darwin/arm64, linux/amd64, linux/arm64 (`sgo/evidence/w00/`).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
