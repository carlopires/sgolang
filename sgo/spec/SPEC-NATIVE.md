# SPEC-NATIVE — Native escape hatches

Wholly denied in subset 0 (`SGO_SPEC_ADMISSION.json` row `SPEC-NATIVE`: `subset0` = `denied`).

### SGO-NATIVE-001 — Native escape hatches are denied in subset 0
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-ERR-001; constitution §2; plan §8.1.
- **grammar_and_types:** Denied: `unsafe`, cgo (`import "C"`), assembly files, `//go:` directives (including `linkname`, `noescape`, `nosplit`, `build`), `//sgo:` pragmas, plugins, and direct system calls (`syscall`, `golang.org/x/sys`). No form of these constructs, packages, or APIs is admitted in any package compiled for the target, including dependencies and unreachable code.
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
- **positive_tests:** n/a — denial clause; admission is planned for never as an application facility (constitution §2); runtime use only via SGO-TCB-002.
- **negative_tests:** S11, S15; fixture:native-unsafe, fixture:native-cgo, fixture:native-linkname, fixture:native-asm.
- **implementation_coverage:** gate level only — `sgo/scripts/sgo-target` denies every program (W00); compiler-level denial is W03.
- **executed_evidence:** W00 gate checks passed on darwin/arm64, linux/amd64, linux/arm64 (gate level; not construct-specific).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
