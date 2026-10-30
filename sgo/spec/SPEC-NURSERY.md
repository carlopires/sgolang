# SPEC-NURSERY — Nursery lifecycle, nesting, terminal snapshots, quiescence

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §8, decision D11, plan §4.3–§4.5, invariants OWN-1..4 and CAN-5.

### SGO-NUR-001 — Syntax and binding
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-LEX-003, SGO-TYPE-005, SGO-MEM-002, SGO-BIND-004; D11.
- **grammar_and_types:** `nursery ( identifier ) Block` is a statement. It declares `identifier` with type `*sgo.Nursery`, in scope only inside `Block`. The binder cannot be assigned. It may be passed as an argument and captured (it is effectively final). The binder may not be `_`. The anonymous form `nursery Block` is denied. A file that uses a nursery statement must import `"sgo"`.
- **preconditions:** The statement executes on a live task.
- **evaluation_and_value_semantics:** Executing the statement creates a new scope (state OPEN) owned by the current task, binds the handle, runs `Block`, then completes the scope (SGO-NUR-003, SGO-NUR-004).
- **ownership_and_lifecycle:** The scope's owner is the executing task (it is not a child of the scope). The scope ends when it reaches CLOSED. The handle may outlive the scope; later uses are run-time checked (SGO-SPAWN-002, SGO-CANCEL-001).
- **effect_and_capability_requirements:** The handle is the authority to admit children into and cancel this scope (SGO-CAP-001).
- **concurrency_and_memory_order:** Scope state is internally synchronized (SGO-TCB-001).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Creating a scope can fail only by resource exhaustion (SGO-ERR-003).
- **cancellation_and_cleanup:** If an enclosing scope of the owner is already cancelled, the new scope starts OPEN and is cancelled immediately (SGO-CANCEL-001 propagation).
- **compile_link_checks:** `SGO_E_UNADMITTED` (anonymous form), `SGO_E_ASSIGN_BINDER`, `SGO_E_RESERVED_WORD`.
- **runtime_guards:** n/a — creation cannot fail except by exhaustion.
- **tcb_assumptions:** Scope records are GC-rooted heap objects (plan §5.3, GC-1).
- **positive_tests:** X02, X04; fixture:nur-basic, fixture:nur-pass-handle.
- **negative_tests:** X02; fixture:nur-anonymous, fixture:nur-assign-binder.
- **implementation_coverage:** none yet — W02 (runtime), W03 (compiler).
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-NUR-002 — Lifecycle states
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-NUR-003, SGO-CANCEL-001; plan §4.3.
- **grammar_and_types:** n/a — dynamic rule.
- **preconditions:** n/a — dynamic rule.
- **evaluation_and_value_semantics:** A scope is in exactly one state: OPEN (owner body running), JOINING (body finished normally, waiting for children), CANCELLING (cancellation requested; body may still be running, children draining), FINALIZING (no live children; recording the outcome), CLOSED (terminal). Transitions: OPEN→JOINING (body completes normally); OPEN→CANCELLING and JOINING→CANCELLING (SGO-CANCEL-001); JOINING→FINALIZING and CANCELLING→FINALIZING (owner body done and zero live children); FINALIZING→CLOSED. No other transitions exist. In subset 0 FINALIZING runs no user code.
- **ownership_and_lifecycle:** Each execution of a nursery statement has a distinct scope ID; IDs are never reused.
- **effect_and_capability_requirements:** n/a — state rule.
- **concurrency_and_memory_order:** State transitions are linearizable; the transition to FINALIZING and any child admission are serialized (OWN-2).
- **allowed_nondeterminism:** None in the set of transitions; their timing depends on scheduling.
- **failure_and_partial_progress:** A drain-budget fail-stop (SGO-DRAIN-002) terminates the process while the scope is CANCELLING; the scope never reaches CLOSED.
- **cancellation_and_cleanup:** CANCELLING is the only state in which children are being asked to stop.
- **compile_link_checks:** n/a — dynamic rule.
- **runtime_guards:** The runtime asserts legal transitions; an illegal transition is a TCB defect (`RuntimeDefect`, SGO-ERR-003).
- **tcb_assumptions:** Scope state machine implemented as specified (SGO-TCB-001); checked by the executable model.
- **positive_tests:** O01, O03; fixture:nur-basic.
- **negative_tests:** n/a — illegal transitions are model and mutation tests (W01 step 5), not source fixtures.
- **implementation_coverage:** none yet — W01 model, W02 runtime.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-NUR-003 — Join
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-NUR-002, SGO-SPAWN-002, SGO-MEM-004; OWN-3, OWN-4.
- **grammar_and_types:** n/a — dynamic rule.
- **preconditions:** n/a — dynamic rule.
- **evaluation_and_value_semantics:** When the owner's body ends (normally, by an exit crossing the nursery, or by unwinding), the owner waits until the scope has zero live children. While it waits, children that are already registered may still admit new children into the scope (SGO-SPAWN-002). The scope reaches FINALIZING only when the body has ended and no registered child is live; then CLOSED.
- **ownership_and_lifecycle:** After CLOSED, no task belonging to the scope is live and no wait registration belonging to it remains (OWN-3).
- **effect_and_capability_requirements:** n/a — join has no external effect.
- **concurrency_and_memory_order:** Each child's complete exit happens-before the nursery statement completes (OWN-4, SGO-MEM-004).
- **allowed_nondeterminism:** The order in which children finish.
- **failure_and_partial_progress:** A child that never finishes keeps the owner waiting forever unless the scope is CANCELLING with a drain budget (SGO-DRAIN-002). Join never reports success while a child is live (CAN-5).
- **cancellation_and_cleanup:** Waiting in join is a managed wait (SGO-WAIT-002); it is not interrupted by cancellation of the scope itself, only by its children finishing.
- **compile_link_checks:** n/a — dynamic rule.
- **runtime_guards:** Live-child count checked at FINALIZING; a non-zero count there is `RuntimeDefect`.
- **tcb_assumptions:** Park/unpark of the owner and child completion publication (SGO-TCB-001).
- **positive_tests:** O01, O02, O04; fixture:nur-join-all, fixture:nur-child-spawns-during-join.
- **negative_tests:** n/a — behavioral; mutation tests in the model show a join that returns early is detected.
- **implementation_coverage:** none yet — W02.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-NUR-004 — Outcome
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-SPAWN-004, SGO-DEF-003, SGO-DEF-005, SGO-PROC-002; R6; plan §4.5.
- **grammar_and_types:** n/a — dynamic rule.
- **preconditions:** The scope is CLOSED.
- **evaluation_and_value_semantics:** The nursery statement completes with exactly one outcome, chosen in this order: (1) if the owner's body was unwinding, that failure stays primary and continues unwinding (child failures are appended to its chain); (2) else if any child ended `Failed`, unwind with the failure of the failed child with the smallest creation ID; (3) else if any child ended `Errored`, unwind with kind `ChildError`, whose message is that of the error returned by the errored child with the smallest creation ID; (4) else complete normally and continue after the statement. Children that ended `Cancelled` or `Succeeded` never cause a failure. All child failures and errors other than the primary are recorded in the chain (SGO-ERR-002).
- **ownership_and_lifecycle:** n/a — outcome rule.
- **effect_and_capability_requirements:** n/a — outcome rule.
- **concurrency_and_memory_order:** The outcome is computed after all children completed (OWN-4).
- **allowed_nondeterminism:** None: the primary is chosen by creation ID, never by timing.
- **failure_and_partial_progress:** Effects of all children remain; no rollback.
- **cancellation_and_cleanup:** Rules (2) and (3) only arise after the failing child already cancelled the scope (SGO-SPAWN-004).
- **compile_link_checks:** n/a — dynamic rule.
- **runtime_guards:** Outcome computation in the runtime.
- **tcb_assumptions:** Stable creation IDs assigned at registration (SGO-SPAWN-003).
- **positive_tests:** O05, S08; fixture:nur-child-error-exit-1, fixture:def-child-panic-cancels-siblings, fixture:nur-two-failures-lowest-id.
- **negative_tests:** n/a — behavioral; checked by positive fixtures of exact outcomes.
- **implementation_coverage:** none yet — W02/W03.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-NUR-005 — Nesting
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-NUR-001, SGO-CANCEL-004, SGO-SPAWN-002.
- **grammar_and_types:** A nursery statement may appear inside another nursery's block, inside a `Spawn` callback, or in any function called from them.
- **preconditions:** n/a — always permitted.
- **evaluation_and_value_semantics:** Nurseries nest lexically within a task: an inner nursery closes before the enclosing block continues. A task's *current scope* is its innermost open nursery, or, if it has none, the scope it was spawned into (for the root task: none).
- **ownership_and_lifecycle:** An inner scope is owned by the task executing it; it is not a child of the outer scope.
- **effect_and_capability_requirements:** n/a — structural rule.
- **concurrency_and_memory_order:** n/a — structural rule.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** An inner nursery's failure unwinds through the outer nursery's body like any other failure.
- **cancellation_and_cleanup:** Cancelling an outer scope cancels inner scopes of tasks linked to it (SGO-CANCEL-004); cancelling an inner scope never cancels an outer one.
- **compile_link_checks:** n/a — structural rule.
- **runtime_guards:** Current-scope tracking per task, used by SGO-SPAWN-002.
- **tcb_assumptions:** Per-task scope stack in runtime records (SGO-TCB-001).
- **positive_tests:** O06, S07; fixture:nur-nested, fixture:nur-inner-cancel-not-outer.
- **negative_tests:** S07; fixture:spawn-into-outer-from-inner.
- **implementation_coverage:** none yet — W02/W03.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
