# SPEC-AUTH — Effect authority (subset 1)

Capabilities for effects other than stdout (D24, D25; plan §8). Part of spec 0.2.0.

### SGO-AUTH-001 — Capability handles and task ceilings
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-CAP-001, SGO-SPAWN-002, SGO-PROC-003, SGO-AUTH-002; D24; plan §8.2.
- **grammar_and_types:** Each effect kind `K` (SGO-AUTH-003) has an opaque handle type `*sgo.K` (for example `*sgo.Clock`). Handles have no exported fields and no public constructor; they are obtained only from the root context (`root.K(...)`, SGO-AUTH-002) or by attenuating another handle. They compare by identity only, like `*sgo.Nursery` (SGO-CAP-001).
- **preconditions:** Attenuation methods per kind are those of SGO-AUTH-003. A spawner narrows a child's ceiling with `n.SpawnWithout(kind string, f)`, where `kind` is a string naming an SGO-AUTH-003 kind (any other string is `InvalidArgument` at run time); the child's ceiling is the spawner's minus that kind (repeated narrowing removes several kinds). There is no operation that adds a kind to a ceiling.
- **evaluation_and_value_semantics:** Every task has a *ceiling*: the set of effect kinds it may exercise. The root task's ceiling is the program's root grants (SGO-AUTH-002). A spawned or started child inherits its spawner's ceiling unless the spawn narrows it; a ceiling is never widened. An effect operation succeeds only if both hold: the handle permits it, and the calling task's ceiling contains its kind. Attenuation returns a new handle that permits a subset of what the original permits; it never permits more.
- **ownership_and_lifecycle:** A handle may be passed to other tasks as a value; possession alone does not grant the effect (the ceiling check above).
- **effect_and_capability_requirements:** This clause defines them.
- **concurrency_and_memory_order:** Handles are immutable; attenuation creates a new handle. Ceilings are fixed when a task is created.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** An operation not permitted by the handle or the ceiling fails with `MissingCapability` (SGO-DEF-003) before any effect or resource creation.
- **cancellation_and_cleanup:** n/a — authority rule.
- **compile_link_checks:** `SGO_E_TYPE_MISMATCH` for a handle of the wrong kind; no other static check.
- **runtime_guards:** Handle and ceiling checks at every effect entry point.
- **tcb_assumptions:** Task ceilings are part of the task record, fixed at creation by `sgoAdmit` (TCB-P01; `src/runtime/sgo_auth.go`).
- **positive_tests:** A02, A10; fixture:auth-simstore-attenuation, fixture:auth-spawn-without.
- **negative_tests:** A01, A02, A09; fixture:auth-spawn-without, fixture:auth-spawn-without-unknown, fixture:auth-handle-compare-denied.
- **implementation_coverage:** W05: runtime `src/runtime/sgo_auth.go` (ceilings, `sgoRequire`, `SpawnWithout`); handles `src/sgo/effects.go`, `src/sgo/auth.go`; handle types in `types2/sgo.go` and `sgoadmit`.
- **executed_evidence:** `sgo/evidence/w05/README.md` (pending W05 closure).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-AUTH-002 — Root grants declared in sgo.mod
- **version:** 4
- **status:** frozen
- **reference_closure:** SGO-IMP-003, SGO-FILE-002, SGO-NET-002, SGO-IMP-001, SGO-TOOL-001, SGO-BUILD-001, SGO-AUTH-001; D25.
- **grammar_and_types:** `sgo.mod` may contain lines `grant <kind>` for the kinds of SGO-AUTH-003, each at most once. `root.K(...)` on the root context returns the root handle of kind `K`. Spec 0.5.0 adds the file and network grants `grant dir`, `grant connect`, and `grant listen` (SGO-FILE-002, SGO-NET-002); they are not effect kinds and do not enter ceilings: their handles come only from the root task's `root.Dir`, `root.Dial`, and `root.Listen`, and children use the handles the root passes to them. They are part of the build identity like the kinds.
- **preconditions:** Only the root context provides root handles, as `Stdout` (SGO-PROC-003).
- **evaluation_and_value_semantics:** The root task's ceiling is exactly the kinds granted in the main module's `sgo.mod`. `root.K(...)` for a kind that is not granted fails with `MissingCapability`. Dependencies cannot add grants: only the main module's `sgo.mod` counts. A required module's `grant` lines are a bound, not a source of authority: each must name a kind the main module also grants, and file and network grants are refused in dependencies (SGO-IMP-003, D114).
- **ownership_and_lifecycle:** Root handles live for the whole program.
- **effect_and_capability_requirements:** Defines the root of authority.
- **concurrency_and_memory_order:** n/a — fixed at build time.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** An unknown or duplicate `grant` line is `SGO_E_USAGE` at build time.
- **cancellation_and_cleanup:** n/a — grants are fixed at build time; nothing runs.
- **compile_link_checks:** The driver records the granted kinds in the build ID (`grants=<kind,kind|none>`); no flag, environment variable, or run-time setting changes them.
- **runtime_guards:** Root-context check; grant check.
- **tcb_assumptions:** The driver passes the grant set to the linker with `-sgogrants` (known kinds once each, canonical order), which sets `runtime.sgoGrants`, parsed when the program starts (as for the drain budget; `-X` stays refused).
- **positive_tests:** A01; fixture:auth-clock-granted, fixture:auth-wallclock.
- **negative_tests:** A01, A06; fixture:auth-missing-grant, fixture:auth-root-only, fixture:auth-grant-unknown, fixture:auth-grant-duplicate.
- **implementation_coverage:** W05: driver `src/cmd/sgo/module.go` (`grant` lines), `build.go` (build ID `grants=`), linker `ld/sgoid.go` (`-sgogrants`), runtime `sgo_auth.go`, `sgo_effects.go` (`sgo_rootGrant`).
- **executed_evidence:** `sgo/evidence/w05/README.md` (pending W05 closure).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-AUTH-003 — Effect kinds of subset 1
- **version:** 3
- **status:** frozen
- **reference_closure:** SGO-AUTH-001, SGO-AUTH-002, SGO-TIMER-001; D26; plan §8.3.
- **grammar_and_types:** Kinds and root accessors (each available only if granted, SGO-AUTH-002): `root.Clock() *sgo.Clock` with `c.Since() int` (ms of monotonic time since program start); `root.WallClock() *sgo.WallClock` with `w.Now() int` (UTC ms since 1970-01-01); `root.Random(seed int) *sgo.Random` with `r.Intn(n int) int`, `r.Split() *sgo.Random`; `root.Entropy() *sgo.Entropy` with `e.Bytes(k int) string`; `root.SimStore() *sgo.SimStore` with `s.Get(key string) (string, bool, error)`, `s.Put(key, value string) error`, `s.ReadOnly() *sgo.SimStore`, `s.Prefix(p string) *sgo.SimStore`. `simnet` (a loopback endpoint) is not part of spec 0.2.0 and stays denied (SGO-NET-001).
- **preconditions:** `Intn` requires `n > 0`; `Bytes` requires `1 <= k <= 1024`; `Prefix` requires a non-empty prefix.
- **evaluation_and_value_semantics:** `Since` and `Now` read the target's monotonic and wall clocks (SGO-TIMER-001); `Since` never decreases within a program. `Random(seed)` starts the PCG-DXSM generator with 128-bit state, the algorithm of Go's `math/rand/v2` `PCG`, seeded with `(seed, 0)`; `Intn` returns a uniform value in `[0, n)` by the unbiased method of that package, so a seed produces the same sequence on every target. `Split` returns a new generator seeded from the next two outputs of the current one, so derived streams are deterministic too. The seeded generator never draws from the OS; `Entropy` always does and is never replaced by a seeded generator (A08). `Get`/`Put` act on one in-memory string map per program; `ReadOnly` permits only `Get`; `Prefix(p)` permits only keys beginning with `p` (concatenated for nested prefixes).
- **ownership_and_lifecycle:** Root handles live for the program. Effect handles are shareable authority tokens: any task may hold one; authority is bounded by grants, ceilings, and attenuation, and is never revoked (D169). A `*sgo.Random` is task-affine: only its owner may call its methods. The owner of a generator returned by `Random` is the calling task; a generator returned by `Split` is unowned until its first use, and the first task that uses it becomes its owner in one atomic step, so a parent can split one generator per child before spawning it (D38); other handles may be used by any task whose ceiling permits the kind.
- **effect_and_capability_requirements:** One kind per handle type.
- **concurrency_and_memory_order:** Handles are immutable except the generator state of `*sgo.Random`, which only its owning task can touch (task affinity), so no handle is shared mutable state (D01). SimStore operations are linearizable; a `Put` happens-before every `Get` that returns its value.
- **allowed_nondeterminism:** `clock`, `wallclock`, `entropy` values; `random` is deterministic given its seed.
- **failure_and_partial_progress:** A precondition violation fails with `InvalidArgument`; a missing grant, a ceiling without the kind, or a handle that does not permit the operation (read-only, outside its prefix) fail with `MissingCapability`; use of a `*sgo.Random` by another task fails with `SpawnContractViolation`. Every failure happens before any effect. `Entropy` failure of the OS source is `ResourceExhausted` (SGO-ERR-003).
- **cancellation_and_cleanup:** n/a — none of these operations waits; cancellation is observed at Checkpoint/Sleep as before.
- **compile_link_checks:** `SGO_E_UNADMITTED` for `simnet` and for any clock, random, or I/O API outside this clause; `SGO_E_DISCARDED_ERROR` for unused `Get`/`Put` errors. Argument preconditions are checked at run time (`InvalidArgument`), as for `Sleep`.
- **runtime_guards:** As SGO-AUTH-001.
- **tcb_assumptions:** Monotonic clock (TCB-P05); TCB-P13 monotonic and wall-clock reads (`nanotime`, `time_now`), TCB-P14 OS cryptographic randomness (`readRandom`), TCB-P15 the per-program simulated store (copy-on-write map); callers only `src/sgo/effects.go` (`src/runtime/sgo_effects.go`).
- **positive_tests:** A04, A05, A08; fixture:auth-clock-granted, fixture:auth-wallclock, fixture:auth-random-deterministic, fixture:auth-entropy, fixture:auth-simstore-attenuation, fixture:auth-random-split-handoff.
- **negative_tests:** A04, A07; fixture:auth-lend-revoked (a reject: `LendTo` is not admitted, D169), fixture:auth-simstore-prefix-denied, fixture:auth-random-other-task, fixture:auth-entropy-invalid, fixture:auth-random-split-claimed.
- **implementation_coverage:** W05: handles `src/sgo/effects.go` (PCG-DXSM matches math/rand/v2, `TestRandomMatchesMathRandV2`); primitives `src/runtime/sgo_effects.go`.
- **executed_evidence:** `sgo/evidence/w05/README.md` (pending W05 closure).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
