# SPEC-TIMER — Production/virtual clocks, timers, deadlines and tie ordering

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §12.

### SGO-TIMER-001 — Clock
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-WAIT-001, SGO-DRAIN-002; D22.
- **grammar_and_types:** Subset 0 has no clock-reading API; time is used only by `Sleep` (SGO-WAIT-001) and the drain budget (SGO-DRAIN-002).
- **preconditions:** n/a — no user API.
- **evaluation_and_value_semantics:** Both use the target's monotonic clock with millisecond resolution or finer. Wall-clock time is never used. Timer expiry means at least the requested duration has elapsed on the monotonic clock; there is no upper bound in production (scheduling delay).
- **ownership_and_lifecycle:** Timers belong to the wait or scope that created them and are removed exactly once (CAN-2).
- **effect_and_capability_requirements:** n/a — no user-visible clock.
- **concurrency_and_memory_order:** Timer expiry and cancellation follow the one-winner rule of SGO-WAIT-001.
- **allowed_nondeterminism:** The amount by which a timer fires late. Among timers that are due together, wake-up order is unspecified in production; the lab (W08) orders them by (deadline, timer ID).
- **failure_and_partial_progress:** n/a — timers cannot fail except by resource exhaustion (SGO-ERR-003).
- **cancellation_and_cleanup:** A cancelled sleep's timer is removed (SGO-WAIT-001).
- **compile_link_checks:** `SGO_E_UNADMITTED` for any clock or timer API other than `Sleep`.
- **runtime_guards:** n/a — no user API.
- **tcb_assumptions:** Upstream runtime timers and the target monotonic clock: `clock_gettime(CLOCK_MONOTONIC)` on Linux, `mach_absolute_time`-based libSystem calls on darwin (SGO-TCB-001, D22).
- **positive_tests:** C09; fixture:wait-sleep.
- **negative_tests:** S17; fixture:timer-time-package.
- **implementation_coverage:** none yet — W04.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
