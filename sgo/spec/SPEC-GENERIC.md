# SPEC-GENERIC — Generics

Wholly denied in subset 0 (`SGO_SPEC_ADMISSION.json` row `SPEC-GENERIC`: `subset0` = `denied`).

### SGO-GEN-001 — User generics are denied
- **version:** 3
- **status:** frozen
- **reference_closure:** SGO-ERR-001, SGO-PERMIT-002; D05, D28.
- **grammar_and_types:** Denied: type parameters, constraints, generic functions or types declared in user code, and instantiation of anything except the parameterized members of package `sgo` admitted by SGO-PERMIT-002 (`sgo.Channel[T]`, `sgo.SendPermit[T]`, `sgo.NewChannel[T]`, with the type arguments that clause admits), `sgo.Copy[T]` (SGO-COMP-004), and `sgo.Frozen[T]` and `sgo.Freeze[T]` (SGO-COMP-009, spec 0.10.0). No other form of these constructs, packages, or APIs is admitted in any package compiled for the target, including dependencies and unreachable code.
- **preconditions:** n/a — denial clause; there is nothing to use.
- **evaluation_and_value_semantics:** n/a — denied constructs never execute.
- **ownership_and_lifecycle:** n/a — denied constructs never execute.
- **effect_and_capability_requirements:** n/a — denied constructs never execute.
- **concurrency_and_memory_order:** n/a — denied constructs never execute.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Any use is a compile error; no object file is produced.
- **cancellation_and_cleanup:** n/a — denied constructs never execute.
- **compile_link_checks:** `SGO_E_UNADMITTED`, naming the construct and this clause.
- **runtime_guards:** n/a — rejected before execution.
- **tcb_assumptions:** n/a — no runtime involvement.
- **positive_tests:** n/a — denial clause; admission is planned for later subset (instantiation closure, plan §1.4).
- **negative_tests:** S10; fixture:type-generic.
- **implementation_coverage:** gate level only — `sgo/scripts/sgo-target` denies every program (W00); compiler-level denial is W03.
- **executed_evidence:** W00 gate checks passed on darwin/arm64, linux/amd64, linux/arm64 (gate level; not construct-specific).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
