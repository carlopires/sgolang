# SPEC-CONTROL — Branches, loops, returns, closures and all scope exits

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §6, decision D14, resolutions R3/R4, and plan §4.4.

### SGO-CTL-001 — Statements
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-BIND-003, SGO-VAL-001, SGO-DEF-001, SGO-NUR-001, SGO-CTL-002, SGO-CHAN-002, SGO-CHAN-004, SGO-COMP-002, SGO-COMP-005, SGO-COMP-006, SGO-VAL-002, SGO-IFACE-004; D14, D129; R3, R4.
- **grammar_and_types:** Admitted statements: blocks `{ … }`; declarations (SGO-BIND-003); assignments `=`, `:=`, `op=`; `x++`, `x--`; `_ = e` (explicit discard); expression statements that are calls; `if [init;] cond { … } [else if … | else { … }]`; `for { … }`, `for cond { … }`, `for [init]; [cond]; [post] { … }`; expression `switch` (SGO-CTL-002); type switches (SGO-IFACE-004, spec 0.14.0); `select` (SGO-CHAN-004); `for k, v := range x` over channels (SGO-CHAN-002), arrays (SGO-COMP-002), slices (SGO-COMP-005), maps (SGO-COMP-006), and strings (SGO-VAL-002); `return [e, …]`; `break [label]`; `continue [label]`; labeled statements `L: stmt` where stmt is `for`, `switch`, or a block; `defer` (SGO-DEF-001); `panic(msg)` as a call statement (SGO-DEF-002); `nursery (n) { … }` (SGO-NUR-001). Init and post statements are simple statements (assignment, `:=`, `++`/`--`, call). Denied: `goto`, `fallthrough`, `go`, `for … range` over any other operand (integers, functions), and empty statements other than an empty block. Version 1 also denied `select`, type switches, and every range form; the clauses named above admit them. An expression statement whose call returns one or more results including an `error` is denied; `_ = f()` (or `_, _ = f()`) is the explicit discard. A label must be used.
- **preconditions:** Conditions have type `bool` (SGO-VAL-003). `break` appears inside a `for` or `switch`; `continue` inside a `for`; a label names an enclosing labeled statement of the right kind.
- **evaluation_and_value_semantics:** `if` evaluates init, then cond, then runs one branch. `for` runs init once, then repeatedly: evaluate cond (absent means `true`), run the body, run post. `break` leaves the innermost (or labeled) `for`/`switch`; `continue` starts the next iteration of the innermost (or labeled) `for`, running its post statement. `return` ends the function (SGO-CTL-004 for nursery exits, SGO-DEF-001 for defers).
- **ownership_and_lifecycle:** Variables declared in init statements are scoped to the statement (SGO-BIND-004). In a three-clause `for`, the loop variable is one variable for the whole loop (it is reassigned by post, so it cannot be captured, SGO-MEM-002).
- **effect_and_capability_requirements:** n/a — control flow has no effects of its own.
- **concurrency_and_memory_order:** n/a — sequential within a task.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** A failure in any evaluated expression unwinds from that point (SGO-DEF-003).
- **cancellation_and_cleanup:** Loops are not cancellation points; a loop that never calls `Checkpoint` or `Sleep` is non-cooperative (SGO-DRAIN-001).
- **compile_link_checks:** `SGO_E_UNADMITTED` (denied statements); `SGO_E_DISCARDED_ERROR`; `SGO_E_UNUSED_LABEL`; `SGO_E_MISPLACED_BRANCH`; `SGO_E_UNSTRUCTURED_SPAWN` for `go`.
- **runtime_guards:** n/a — control flow itself cannot fail.
- **tcb_assumptions:** n/a — generated code.
- **positive_tests:** X04, X07; fixture:ctl-statements, fixture:ctl-labeled-loops, fixture:iface-type-switch.
- **negative_tests:** X08, S17; fixture:ctl-goto, fixture:ctl-range, fixture:ctl-discarded-error.
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/types2/sgo_flow.go`, `src/runtime/sgo_nursery.go`.
- **executed_evidence:** conformance fixtures ctl-discarded-error, ctl-goto, ctl-labeled-loops, ctl-range, ctl-select, ctl-statements (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production and lab.

### SGO-CTL-002 — Switch
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-CTL-001, SGO-VAL-005, SGO-TYPE-002.
- **grammar_and_types:** `switch [init;] [tag] { case e1, e2: … default: … }`. The tag has type `int`, `string`, or `bool`; a missing tag means `true`. Each case expression is assignable to the tag's type. At most one `default`. Duplicate constant case values are a compile error. `fallthrough` is denied.
- **preconditions:** n/a — static rules above.
- **evaluation_and_value_semantics:** Evaluate init, then the tag once. Then, in source order (top to bottom, left to right within a case), evaluate each case expression and compare it with `==` to the tag; run the first matching clause and stop. If none matches, run `default` if present. Case expressions after the match are not evaluated.
- **ownership_and_lifecycle:** Each clause is its own block (SGO-BIND-004).
- **effect_and_capability_requirements:** n/a — effects only from evaluated expressions.
- **concurrency_and_memory_order:** n/a — sequential.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** A failing case expression unwinds; earlier case expressions have been evaluated.
- **cancellation_and_cleanup:** n/a — not a cancellation point.
- **compile_link_checks:** `SGO_E_DUPLICATE_CASE`, `SGO_E_TYPE_MISMATCH`, `SGO_E_UNADMITTED` (`fallthrough`, type switch).
- **runtime_guards:** n/a — comparisons cannot fail.
- **tcb_assumptions:** n/a — generated code.
- **positive_tests:** S16; fixture:ctl-switch.
- **negative_tests:** S16; fixture:ctl-fallthrough, fixture:ctl-duplicate-case.
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/types2/sgo_flow.go`, `src/runtime/sgo_nursery.go`.
- **executed_evidence:** conformance fixtures ctl-duplicate-case, ctl-fallthrough, ctl-switch (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-CTL-003 — Terminating statements and unreachable code
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-CTL-001, SGO-DEF-002; R3.
- **grammar_and_types:** A terminating statement is: `return`; a call `panic(…)`; a block whose last statement is terminating; an `if` with an `else` where both branches are terminating; a `for` with no condition and no `break` referring to it; a `switch` with a `default`, no `break` referring to it, and every clause ending in a terminating statement; a labeled terminating statement; a `nursery` statement whose block is terminating. A function with results must end in a terminating statement. A statement that follows a terminating statement in the same block is a compile error.
- **preconditions:** n/a — static rule.
- **evaluation_and_value_semantics:** n/a — static rule.
- **ownership_and_lifecycle:** n/a — static rule.
- **effect_and_capability_requirements:** n/a — static rule.
- **concurrency_and_memory_order:** n/a — static rule.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Violations are compile errors.
- **cancellation_and_cleanup:** A `nursery` statement whose block ends in `return` still joins (SGO-CTL-004).
- **compile_link_checks:** `SGO_E_MISSING_RETURN`, `SGO_E_UNREACHABLE`.
- **runtime_guards:** n/a — static rule.
- **tcb_assumptions:** n/a — compiler only.
- **positive_tests:** X05; fixture:ctl-terminating.
- **negative_tests:** X05; fixture:ctl-missing-return, fixture:ctl-unreachable.
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/types2/sgo_flow.go`, `src/runtime/sgo_nursery.go`.
- **executed_evidence:** conformance fixtures ctl-missing-return, ctl-terminating, ctl-unreachable (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-CTL-004 — Exits that cross nurseries
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-NUR-003, SGO-NUR-004, SGO-VAL-005, SGO-DEF-001; plan §4.4.
- **grammar_and_types:** `return`, `break`, and `continue` may leave one or more enclosing `nursery` statements of the same function. Labels may not be used to jump into a nursery (no `goto` exists).
- **preconditions:** n/a — applies to every such exit.
- **evaluation_and_value_semantics:** For `return e1, …, en`: evaluate the operands once, left to right, and hold the values; then complete each crossed nursery from the innermost outward exactly as if its block had ended normally (SGO-NUR-003, SGO-NUR-004); then run the function's deferred calls (SGO-DEF-001); then return the held values. For `break`/`continue` that leave nurseries: complete each crossed nursery innermost first, then transfer control. A `break` or `continue` whose target is inside the nursery does not complete it. A normal exit joins; it never cancels. Each execution of a nursery statement (for example in each loop iteration) is a distinct scope with its own identity.
- **ownership_and_lifecycle:** The nursery is CLOSED before control reaches the target of the exit (OWN-3).
- **effect_and_capability_requirements:** n/a — no effects of its own.
- **concurrency_and_memory_order:** All children of each crossed nursery have completed before control transfers (OWN-4).
- **allowed_nondeterminism:** None in the ordering of joins; children's own completion order is scheduler-dependent but not observable here.
- **failure_and_partial_progress:** If completing a crossed nursery unwinds (SGO-NUR-004 rules 1–3), the exit is replaced by that unwind; held return values are discarded and outer crossed nurseries are completed by the unwind (SGO-DEF-005).
- **cancellation_and_cleanup:** Exits do not cancel. Children still running keep the owner waiting in JOINING.
- **compile_link_checks:** The compiler lowers each exit edge through the crossed nurseries' barriers; IR verification (W03) rejects code where an exit edge skips a barrier.
- **runtime_guards:** n/a — enforced by generated code and the runtime join.
- **tcb_assumptions:** Nursery join primitive (SGO-TCB-001).
- **positive_tests:** X04, X05, X06, X07, X12; fixture:ctl-return-from-nursery, fixture:ctl-break-out-of-nursery, fixture:ctl-nursery-in-loop.
- **negative_tests:** X08; fixture:ctl-goto.
- **implementation_coverage:** W03: `src/cmd/compile/internal/types2/sgo.go`, `src/cmd/compile/internal/types2/sgo_flow.go`, `src/runtime/sgo_nursery.go`.
- **executed_evidence:** conformance fixtures ctl-break-out-of-nursery, ctl-goto, ctl-nursery-in-loop, ctl-return-from-nursery (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
