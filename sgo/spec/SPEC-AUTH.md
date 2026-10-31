# SPEC-AUTH — Effect authority (subset 1)

Capabilities for effects other than stdout (D24, D25; plan §8). Draft for spec 0.2.0; denied until frozen, implemented, enforced, and evidenced (README, status rules).

### SGO-AUTH-001 — Capability handles and task ceilings
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-CAP-001, SGO-SPAWN-002, SGO-PROC-003, SGO-AUTH-002; D24; plan §8.2.
- **grammar_and_types:** Each effect kind `K` (SGO-AUTH-003) has an opaque handle type `*sgo.K` (for example `*sgo.Clock`). Handles have no exported fields and no public constructor; they are obtained only from the root context (`root.K(...)`, SGO-AUTH-002) or by attenuating another handle. They compare by identity only, like `*sgo.Nursery` (SGO-CAP-001).
- **preconditions:** Attenuation methods per kind are those of SGO-AUTH-003. A spawner narrows a child's ceiling with `n.SpawnWithout(kind string, f)`, where `kind` is a constant naming an SGO-AUTH-003 kind; the child's ceiling is the spawner's minus that kind (repeated narrowing removes several kinds). There is no operation that adds a kind to a ceiling.
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
- **grammar_and_types:** Kinds and root accessors (each available only if granted, SGO-AUTH-002): `root.Clock() *sgo.Clock` with `c.Since() int` (ms of monotonic time since program start); `root.WallClock() *sgo.WallClock` with `w.Now() int` (UTC ms since 1970-01-01); `root.Random(seed int) *sgo.Random` with `r.Intn(n int) int`, `r.Split() *sgo.Random`; `root.Entropy() *sgo.Entropy` with `e.Bytes(k int) string`; `root.SimStore() *sgo.SimStore` with `s.Get(key string) (string, bool, error)`, `s.Put(key, value string) error`, `s.ReadOnly() *sgo.SimStore`, `s.Prefix(p string) *sgo.SimStore`. Every handle type `*sgo.K` has `h.LendTo(n *sgo.Nursery) *sgo.K` (revocable copy). `simnet` (loopback endpoint) is PENDING and stays denied until specified.
- **preconditions:** `Intn` requires `n > 0`; `Bytes` requires `1 <= k <= 1024`; `Prefix` requires a non-empty prefix. `LendTo(n)` requires that the caller can spawn into `n` (SGO-SPAWN-002).
- **evaluation_and_value_semantics:** `Since` and `Now` read the target's monotonic and wall clocks (SGO-TIMER-001); `Since` never decreases within a program. `Random(seed)` starts the PCG-DXSM generator with 128-bit state, the algorithm of Go's `math/rand/v2` `PCG`, seeded with `(seed, 0)`; `Intn` returns a uniform value in `[0, n)` by the unbiased method of that package, so a seed produces the same sequence on every target. `Split` returns a new generator seeded from the next two outputs of the current one, so derived streams are deterministic too. The seeded generator never draws from the OS; `Entropy` always does and is never replaced by a seeded generator (A08). `Get`/`Put` act on one in-memory string map per program; `ReadOnly` permits only `Get`; `Prefix(p)` permits only keys beginning with `p` (concatenated for nested prefixes). A lent handle permits what the original permits while `n` is not CLOSED, and nothing afterwards.
- **ownership_and_lifecycle:** Root handles live for the program. A lent handle is revoked when its nursery closes (A03). A `*sgo.Random` is task-affine: only the task that obtained it (by `Random` or `Split`) may call its methods; other handles may be used by any task whose ceiling permits the kind.
- **effect_and_capability_requirements:** One kind per handle type.
- **concurrency_and_memory_order:** Handles are immutable except the generator state of `*sgo.Random`, which only its owning task can touch (task affinity), so no handle is shared mutable state (D01). SimStore operations are linearizable; a `Put` happens-before every `Get` that returns its value.
- **allowed_nondeterminism:** `clock`, `wallclock`, `entropy` values; `random` is deterministic given its seed.
- **failure_and_partial_progress:** A precondition violation fails with `InvalidArgument`; a missing grant, a ceiling without the kind, a handle that does not permit the operation (read-only, outside its prefix), or a revoked handle fail with `MissingCapability`; use of a `*sgo.Random` by another task fails with `SpawnContractViolation`. Every failure happens before any effect. `Entropy` failure of the OS source is `ResourceExhausted` (SGO-ERR-003).
- **cancellation_and_cleanup:** n/a — none of these operations waits; cancellation is observed at Checkpoint/Sleep as before.
- **compile_link_checks:** `SGO_E_UNADMITTED` for `simnet` and for any clock, random, or I/O API outside this clause; `SGO_E_DISCARDED_ERROR` for unused `Get`/`Put` errors; a negative constant `Intn`/`Bytes` argument is `SGO_E_INVALID_ARGUMENT`.
- **runtime_guards:** As SGO-AUTH-001.
- **tcb_assumptions:** Monotonic clock (TCB-P05); PENDING: wall-clock read and OS CSPRNG primitives (new TCB entries with callers).
- **positive_tests:** A03, A04, A05, A08; PENDING: fixtures.
- **negative_tests:** A03, A04, A07; PENDING: fixtures.
- **implementation_coverage:** none yet — W05.
- **executed_evidence:** none yet.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
