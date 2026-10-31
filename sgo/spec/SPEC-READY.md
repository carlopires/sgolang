# SPEC-READY — Readiness handshake

Denied in subset 0 (spec 0.1.0, `SGO_SPEC_ADMISSION.json` row `SPEC-READY`: `subset0` = `denied`). Spec 0.2.0 admits `n.Start` with a readiness handshake (SGO-READY-002), which supersedes SGO-READY-001.

### SGO-READY-001 — Readiness handshakes are denied in subset 0
- **version:** 2
- **status:** superseded
- **reference_closure:** SGO-ERR-001, SGO-READY-002; plan §4.3.
- **grammar_and_types:** Superseded in spec 0.2.0 by SGO-READY-002, which admits `n.Start`. Still denied: every other nursery-authorized readiness API (`n.SpawnReady` or similar). No form of these constructs, packages, or APIs is admitted in any package compiled for the target, including dependencies and unreachable code.
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
- **positive_tests:** n/a — denial clause; admission is planned for W05 (plan §8).
- **negative_tests:** S07; fixture:ready-api.
- **implementation_coverage:** gate level only — `sgo/scripts/sgo-target` denies every program (W00); compiler-level denial is W03.
- **executed_evidence:** W00 gate checks passed on darwin/arm64, linux/amd64, linux/arm64 (gate level; not construct-specific).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-READY-002 — Start with a readiness handshake (subset 1)
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-SPAWN-002, SGO-SPAWN-003, SGO-SPAWN-004, SGO-CANCEL-001, SGO-CANCEL-002, SGO-WAIT-002, SGO-NUR-004; D27; plan W05.
- **grammar_and_types:** `err := n.Start(f)` where `n` has type `*sgo.Nursery` and `f` has type `func(task *sgo.Context, ready *sgo.Ready) error`; `Start` returns `error`. `ready.Signal()` has no result. `*sgo.Ready` is an opaque capability (comparable only as SGO-CAP-001 allows; no public constructor).
- **preconditions:** As `Spawn` (SGO-SPAWN-002): the caller can spawn into `n`. `ready.Signal()` may only be called by the task started with it.
- **evaluation_and_value_semantics:** `n`, then `f`, are evaluated once. Admission and registration happen exactly as for `Spawn` (SGO-SPAWN-002/003); if the scope is CANCELLING in effect, no child is created and `Start` returns a cancellation error (SGO-CANCEL-003). Otherwise the caller waits until the first of: (1) the child calls `ready.Signal()` → `Start` returns `nil` and the child keeps running in `n`; (2) the child ends before signalling → `Start` returns the child's returned error if it returned one (including a cancellation error), and otherwise (it failed, or returned `nil` without signalling) an error whose message is `child <id> ended before ready`; the child's outcome is published to `n` as usual (SGO-SPAWN-004, so a failure or an error also cancels `n`); (3) a scope linked to the caller becomes CANCELLING → `Start` returns a cancellation error; the child stays registered in `n` and is joined with it. In every case the child is a registered child of `n`; `Start` never reports success for a child that is not running in `n`, and never leaves an unregistered child.
- **ownership_and_lifecycle:** The child is owned by `n` like a spawned child; the `Ready` capability belongs to the child.
- **effect_and_capability_requirements:** Requires the nursery handle (SGO-CAP-001); `Ready` grants only signalling.
- **concurrency_and_memory_order:** `Signal` happens-before `Start`'s return of `nil`. Signal, child completion, and cancellation are arbitrated by one terminal decision of the waiter; exactly one of (1)–(3) is reported.
- **allowed_nondeterminism:** Which of (1)–(3) wins when they race; each outcome is specified.
- **failure_and_partial_progress:** A second `Signal` fails the child with `SpawnContractViolation` (diagnostic; the accepted readiness is unchanged). `Signal` by another task fails that task with `SpawnContractViolation`. `Signal` after the waiter returned by (3) is accepted and has no effect.
- **cancellation_and_cleanup:** `Start`'s wait is a cancellation point for the caller (case 3). The child's cancellation follows SGO-CANCEL-001 as for any child of `n`.
- **compile_link_checks:** `SGO_E_DISCARDED_ERROR` if the result of `Start` is unused; `SGO_E_TYPE_MISMATCH` for a callback of another type.
- **runtime_guards:** Admission as SGO-SPAWN-002; one terminal decision per start; Ready ownership check.
- **tcb_assumptions:** Task creation, park/unpark, cancellation query (SGO-TCB-001 TCB-P01, P02, P04); the readiness decision is part of the scope primitive (`src/runtime/sgo_ready.go`, `sgo/docs/runtime-design.md` §11).
- **positive_tests:** R01, R02, R04; fixture:ready-signal, fixture:ready-child-fails-first, fixture:ready-cancelled-before-signal.
- **negative_tests:** R03; fixture:ready-double-signal, fixture:ready-signal-other-task, fixture:ready-discarded-error.
- **implementation_coverage:** W05: runtime `src/runtime/sgo_ready.go` (`sgoStart`, one CAS terminal decision under the cancellation registry lock), admission through `sgoAdmit`; API `src/sgo/ready.go`; `*sgo.Ready` is a capability handle (`types2/sgo.go`, `sgoadmit`).
- **executed_evidence:** `sgo/evidence/w05/README.md` (pending W05 closure); runtime tests `TestSGoReady*`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
