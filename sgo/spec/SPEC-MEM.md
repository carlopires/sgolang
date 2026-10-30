# SPEC-MEM — References, aliasing, shared mutable memory, atomics and data-race contract

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §5 and decision D01.

### SGO-MEM-001 — Variables are task-local
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-BIND-003, SGO-TYPE-001; D01, D06, D07.
- **grammar_and_types:** The only variables are function parameters, results, and local variables. There are no package-level variables (SGO-BIND-003), pointers, or address-of operator (SGO-TYPE-001).
- **preconditions:** n/a — structural rule.
- **evaluation_and_value_semantics:** Assignment replaces a variable's value. Reading a variable yields the last value assigned to it by the same task.
- **ownership_and_lifecycle:** A variable belongs to the function activation (and therefore the task) that declared it; a captured variable lives as long as the function values that capture it (SGO-MEM-002).
- **effect_and_capability_requirements:** n/a — memory rule.
- **concurrency_and_memory_order:** Only the declaring task can assign a variable (with SGO-MEM-002, no other task can even observe a variable that is ever reassigned).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** n/a — reads and writes of variables cannot fail.
- **cancellation_and_cleanup:** n/a — memory rule.
- **compile_link_checks:** Enforced by SGO-BIND-003 and SGO-TYPE-001 denials.
- **runtime_guards:** n/a — structural rule.
- **tcb_assumptions:** The garbage collector keeps captured variables alive while reachable and never moves them observably (SGO-TCB-001).
- **positive_tests:** S16; fixture:mem-locals.
- **negative_tests:** S16; fixture:bind-package-var, fixture:type-pointer.
- **implementation_coverage:** none yet — W03.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-MEM-002 — Captured variables are effectively final
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-MEM-001, SGO-TYPE-003, SGO-TYPE-004; D01.
- **grammar_and_types:** A local variable (including a parameter or named result) that is referenced inside any function literal is *captured*. A captured variable must be effectively final: after its declaration, no assignment, `op=`, `++`, `--`, or `:=` re-assignment to it appears anywhere, neither in the declaring function nor in any function literal. The rule applies to every function literal, not only to `Spawn` callbacks, because a function value can reach another task.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** A function literal observes the single value its captured variables were initialized with.
- **ownership_and_lifecycle:** Captured variables live as long as the longest-lived function value capturing them.
- **effect_and_capability_requirements:** Capturing a capability handle passes its authority to wherever the function value goes (SGO-CAP-001).
- **concurrency_and_memory_order:** Because captured variables never change, any number of tasks may read them concurrently without a data race.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** A violation is a compile error.
- **cancellation_and_cleanup:** n/a — static rule.
- **compile_link_checks:** `SGO_E_CAPTURE_NOT_FINAL`, reported at the assignment and naming the capturing literal.
- **runtime_guards:** n/a — static rule.
- **tcb_assumptions:** n/a — compiler only.
- **positive_tests:** X11, S16; fixture:mem-capture-final.
- **negative_tests:** X11, S16; fixture:mem-capture-mutated, fixture:mem-capture-loop-var, fixture:mem-capture-assigned-in-literal.
- **implementation_coverage:** none yet — W03 (types2 check).
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-MEM-003 — No data races
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-MEM-001, SGO-MEM-002, SGO-TYPE-004, SGO-TCB-001; D01.
- **grammar_and_types:** n/a — theorem over the subset-0 rules.
- **preconditions:** n/a — holds for every admitted program.
- **evaluation_and_value_semantics:** In every admitted subset-0 program, no memory location is written by one task while another task can access it. Argument: user-visible mutable locations are only variables (SGO-MEM-001); a variable that is ever assigned after declaration cannot be captured (SGO-MEM-002); values passed between tasks are immutable or internally synchronized (SGO-TYPE-004). Tasks communicate only through child outcomes (SGO-SPAWN-004) and capability effects (SGO-PROC-003).
- **ownership_and_lifecycle:** n/a — theorem.
- **effect_and_capability_requirements:** n/a — theorem.
- **concurrency_and_memory_order:** Data races are statically impossible in subset 0. Runtime-internal synchronization is the TCB's obligation (SGO-TCB-001).
- **allowed_nondeterminism:** The only cross-task nondeterminism is scheduling order, which is observable only through the order of `WriteString` effects (SGO-PROC-003) and which child failure is cancelled versus completes first; outcomes are still selected deterministically by creation ID (SGO-NUR-004).
- **failure_and_partial_progress:** n/a — theorem.
- **cancellation_and_cleanup:** n/a — theorem.
- **compile_link_checks:** None of its own; it follows from the checks of the referenced clauses. The executable model (W01) and race-detector runs (W02/W03) provide evidence.
- **runtime_guards:** n/a — theorem.
- **tcb_assumptions:** The runtime itself is free of data races on the objects behind capability handles (SGO-TCB-001).
- **positive_tests:** S16; fixture:mem-share-immutable.
- **negative_tests:** S16; fixture:mem-capture-mutated.
- **implementation_coverage:** none yet — follows from W03 checks.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-MEM-004 — Happens-before
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-SPAWN-001, SGO-SPAWN-003, SGO-NUR-003, SGO-CANCEL-001, SGO-CANCEL-002, SGO-PROC-003.
- **grammar_and_types:** n/a — dynamic ordering rule.
- **preconditions:** n/a — dynamic ordering rule.
- **evaluation_and_value_semantics:** The following happens-before edges exist: (1) completing the evaluation of a `Spawn` call's receiver and argument happens-before the child callback starts (OWN-2); (2) a child's complete exit, including its deferred calls, happens-before the nursery statement that owns it completes (OWN-4); (3) a `Cancel` (or a child failure that cancels) happens-before any `Checkpoint` or `Sleep` that returns the resulting cancellation error; (4) `WriteString` calls on the same `Writer` are totally ordered, and each happens-before the next.
- **ownership_and_lifecycle:** n/a — ordering rule.
- **effect_and_capability_requirements:** n/a — ordering rule.
- **concurrency_and_memory_order:** These edges are the complete list of cross-task orderings in subset 0.
- **allowed_nondeterminism:** Any interleaving consistent with these edges and with each task's sequential order is allowed.
- **failure_and_partial_progress:** n/a — ordering rule.
- **cancellation_and_cleanup:** Edge (3) defines when cancellation becomes visible.
- **compile_link_checks:** n/a — dynamic rule.
- **runtime_guards:** n/a — provided by the runtime's synchronization (SGO-TCB-001).
- **tcb_assumptions:** Task creation, completion, cancellation, and the writer use synchronization that establishes these edges on every admitted target, including arm64's weaker memory model (D22).
- **positive_tests:** O01, O02; fixture:mem-join-visibility.
- **negative_tests:** n/a — ordering is checked by the executable model's history checks (W01 step 5) rather than by rejection fixtures.
- **implementation_coverage:** none yet — W02.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
