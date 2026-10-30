# SPEC-STACK — GC roots, stack relocation, overflow and experimental stack contract

Subset 0. Derived from plan §5.3, §1.2 (growable stacks first), invariants GC-1, GC-2, decision D16.

### SGO-STACK-001 — Stacks and roots in subset 0
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-ERR-003, SGO-TCB-001; D16; GC-1, GC-2.
- **grammar_and_types:** n/a — runtime rule.
- **preconditions:** n/a — runtime rule.
- **evaluation_and_value_semantics:** Each task runs on a growable stack (upstream mechanism). Stack growth and movement are invisible to programs: subset 0 has no pointers, so no user value can refer to a stack address. Every runtime record for scopes, tasks, waits, and timers is a heap object reachable from an audited GC root while live (GC-1); none points into a movable stack (GC-2). The maximum stack size is recorded in build metadata.
- **ownership_and_lifecycle:** A task's stack is released after its outcome is published.
- **effect_and_capability_requirements:** n/a — runtime rule.
- **concurrency_and_memory_order:** n/a — per-task resource.
- **allowed_nondeterminism:** None visible to programs.
- **failure_and_partial_progress:** Exceeding the maximum stack size is resource exhaustion (exit status 4, SGO-ERR-003).
- **cancellation_and_cleanup:** n/a — runtime rule.
- **compile_link_checks:** Function prologues keep the upstream stack-check sequences (`stacksplit` on amd64 and arm64); removing them is part of the separate W11 experiment, not of subset 0.
- **runtime_guards:** Stack-bound checks in prologues.
- **tcb_assumptions:** Upstream stack growth, copying, and GC stack scanning (SGO-TCB-001).
- **positive_tests:** X12; fixture:stack-deep-recursion-ok.
- **negative_tests:** n/a — overflow is fail-stop; checked by fixture:err-stack-overflow-exit-4 under SGO-ERR-003.
- **implementation_coverage:** upstream mechanism unchanged; audit in W02.
- **executed_evidence:** none yet for S-Go records; upstream stack tests passed in W00 B03.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
