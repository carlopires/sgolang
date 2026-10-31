# SPEC-AUTH — Effect authority (subset 1)

Capabilities for effects other than stdout (D24, D25; plan §8). Draft for spec 0.2.0; denied until frozen, implemented, enforced, and evidenced (README, status rules).

### SGO-AUTH-001 — Capability handles and task ceilings
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-CAP-001, SGO-SPAWN-002, SGO-PROC-003, SGO-AUTH-002; D24; plan §8.2.
- **grammar_and_types:** Each effect kind `K` (SGO-AUTH-003) has an opaque handle type `*sgo.K` (for example `*sgo.Clock`). Handles have no exported fields and no public constructor; they are obtained only from the root context (`root.K(...)`, SGO-AUTH-002) or by attenuating another handle. They compare by identity only, like `*sgo.Nursery` (SGO-CAP-001).
- **preconditions:** PENDING: the exact attenuation methods per kind (SGO-AUTH-003).
- **evaluation_and_value_semantics:** Every task has a *ceiling*: the set of effect kinds it may exercise. The root task's ceiling is the program's root grants (SGO-AUTH-002). A spawned or started child inherits its spawner's ceiling unless the spawn narrows it; a ceiling is never widened. An effect operation succeeds only if both hold: the handle permits it, and the calling task's ceiling contains its kind. Attenuation returns a new handle that permits a subset of what the original permits; it never permits more.
- **ownership_and_lifecycle:** A handle may be passed to other tasks as a value; possession alone does not grant the effect (the ceiling check above).
- **effect_and_capability_requirements:** This clause defines them.
- **concurrency_and_memory_order:** Handles are immutable; attenuation creates a new handle. Ceilings are fixed when a task is created.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** An operation not permitted by the handle or the ceiling fails with `MissingCapability` (SGO-DEF-003) before any effect or resource creation.
- **cancellation_and_cleanup:** n/a — authority rule.
- **compile_link_checks:** `SGO_E_TYPE_MISMATCH` for a handle of the wrong kind; no other static check.
- **runtime_guards:** Handle and ceiling checks at every effect entry point.
- **tcb_assumptions:** PENDING: the runtime primitive holding task ceilings (TCB-P10 or new).
- **positive_tests:** A02, A10; PENDING: fixtures.
- **negative_tests:** A01, A02, A09; PENDING: fixtures.
- **implementation_coverage:** none yet — W05.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-AUTH-002 — Root grants declared in sgo.mod
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-IMP-001, SGO-TOOL-001, SGO-BUILD-001, SGO-AUTH-001; D25.
- **grammar_and_types:** `sgo.mod` may contain lines `grant <kind>` for the kinds of SGO-AUTH-003, each at most once. `root.K(...)` on the root context returns the root handle of kind `K`.
- **preconditions:** Only the root context provides root handles, as `Stdout` (SGO-PROC-003).
- **evaluation_and_value_semantics:** The root task's ceiling is exactly the kinds granted in the main module's `sgo.mod`. `root.K(...)` for a kind that is not granted fails with `MissingCapability`. Dependencies cannot add grants: only the main module's `sgo.mod` counts.
- **ownership_and_lifecycle:** Root handles live for the whole program.
- **effect_and_capability_requirements:** Defines the root of authority.
- **concurrency_and_memory_order:** n/a — fixed at build time.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** An unknown or duplicate `grant` line is `SGO_E_USAGE` at build time.
- **cancellation_and_cleanup:** n/a — grants are fixed at build time; nothing runs.
- **compile_link_checks:** The driver records the granted kinds in the build ID (`grants=<kind,kind|none>`); no flag, environment variable, or run-time setting changes them.
- **runtime_guards:** Root-context check; grant check.
- **tcb_assumptions:** PENDING: how the linker passes the grant set to the runtime (as for the drain budget).
- **positive_tests:** A01; PENDING: fixtures.
- **negative_tests:** A01, A06; PENDING: fixtures.
- **implementation_coverage:** none yet — W05.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-AUTH-003 — Effect kinds of subset 1
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-AUTH-001, SGO-AUTH-002, SGO-TIMER-001; D26; plan §8.3.
- **grammar_and_types:** Kinds: `clock` (monotonic time since program start, ms), `wallclock` (UTC ms since the epoch), `random` (seeded PRNG with a fixed algorithm, reproducible on every target), `entropy` (the OS CSPRNG), `simstore` and `simnet` (in-memory store and loopback endpoint used to test authority). PENDING: method signatures; PRNG algorithm choice.
- **preconditions:** PENDING.
- **evaluation_and_value_semantics:** PENDING: per kind. The seeded generator never draws from `entropy`, and `entropy` is never replaced by a seeded generator (A08).
- **ownership_and_lifecycle:** PENDING: revocation by lending a handle for the lifetime of a nursery (D24).
- **effect_and_capability_requirements:** One kind per handle type.
- **concurrency_and_memory_order:** PENDING.
- **allowed_nondeterminism:** `clock`, `wallclock`, `entropy` values; `random` is deterministic given its seed.
- **failure_and_partial_progress:** PENDING.
- **cancellation_and_cleanup:** PENDING.
- **compile_link_checks:** PENDING.
- **runtime_guards:** As SGO-AUTH-001.
- **tcb_assumptions:** Monotonic clock (TCB-P05); PENDING: wall clock and CSPRNG primitives.
- **positive_tests:** A03, A04, A05, A08; PENDING: fixtures.
- **negative_tests:** A03, A04, A07; PENDING: fixtures.
- **implementation_coverage:** none yet — W05.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
