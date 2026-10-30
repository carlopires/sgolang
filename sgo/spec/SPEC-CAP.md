# SPEC-CAP — Capabilities, task ceilings, attenuation, revocation and opaque handles

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §13, decision D09, invariants CAP-1, CAP-2.

### SGO-CAP-001 — Capability model in subset 0
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-TYPE-005, SGO-PROC-003, SGO-SPAWN-003, SGO-NUR-001, SGO-MEM-002; CAP-1, CAP-2.
- **grammar_and_types:** The capabilities of subset 0 are the three handle kinds: `*sgo.Nursery` (admit children into and cancel one scope), `*sgo.Context` (cancellation queries and sleeping for one task; on `root` also obtaining the writer), and `*sgo.Writer` (write to stdout). There is no ambient authority: no package, global, or runtime state grants any of these operations.
- **preconditions:** n/a — model clause.
- **evaluation_and_value_semantics:** Authority is exercised only by calling a handle's method. Passing or capturing a handle delegates its full authority; subset 0 has no attenuation or revocation operations. A child's own context carries no capability beyond cancellation and sleeping (CAP-1).
- **ownership_and_lifecycle:** Handles are runtime objects; their validity periods are defined by their clauses (nursery: SGO-SPAWN-002; context: SGO-CANCEL-002; writer: process lifetime).
- **effect_and_capability_requirements:** This clause defines them.
- **concurrency_and_memory_order:** Handles are safe for concurrent use (SGO-TCB-001).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** An invalid use fails before the effect (CAP-2): `SpawnContractViolation` or `MissingCapability`.
- **cancellation_and_cleanup:** Holding a handle confers no right to outlive a scope.
- **compile_link_checks:** SGO-TYPE-005 static rules; denial of every other effectful API (SGO-PROC-004, SGO-IMP-001).
- **runtime_guards:** Per method clause.
- **tcb_assumptions:** Handles cannot be forged (no unsafe code, reflection, or conversions are admitted) (SGO-TCB-001).
- **positive_tests:** S06; fixture:cap-pass-handles, fixture:proc-writer-passed-to-child.
- **negative_tests:** S06, S13; fixture:proc-child-stdout, fixture:spawn-unrelated-task.
- **implementation_coverage:** none yet — W03/W05.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
