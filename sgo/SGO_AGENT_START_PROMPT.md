# S-Go v2 implementation handoff prompt

Paste the text below into a frontier coding agent with this bundle and access to the real repository. This replaces the v1 prompt.

---

You are implementing **S-Go**, an independent, specification-defined language built from a Go compiler/runtime fork. There is **no compatibility path and no commitment to the Go ecosystem**. The user's requirement is that code without a valid S-Go specification must not be admitted into the language.

Read in order:

1. `SGO_LANGUAGE_CONSTITUTION.md` — authoritative constraints and labeled design defaults.
2. `SGO_DECISIONS.md` — user-approved decisions D01–D21 (approved amendments; they override the plan where they differ) and open decision D22.
3. `SGO_FRONTIER_EXECUTION_PLAN.md` — v2 architecture, semantic decisions, work packages, and gates.
4. `SGO_SPEC_SUBSET0.md` — draft specification of subset 0, the closed first-slice language.
5. `SGO_SPEC_ADMISSION.json` — required inventory; initially unadmitted, not a finished specification.
6. `SGO_TASKS.json` and `SGO_TEST_MATRIX.json` — execution ledger and unrun acceptance scenarios.
7. `CHANGELOG_V2.md` and `original/` — correction history and unchanged historical design input.

The implementation repository is `github.com/carlopires/sgolang` (local clone `/Users/carlopires/ycode/sgolang`). Do not lock the first execution target in W00 until decision D22 is recorded.

Inspect the actual tree and checkpoint. Preserve user changes. Identify any v1 work that implemented compatibility profiles, dynamic implicit spawning, preserved ambient APIs, or permissive build/import paths. Do not silently preserve those decisions; remove or quarantine the affected implementation before target admission.

Implement the fork, not a callback library or parser demonstration. Start with W00 and continue through dependency-ready work packages. The first slice is W00–W04, but it must be a closed, specified subset. Do not accept additional inherited Go constructs merely because the parser/runtime handles them.

Non-negotiable rules:

- Exactly one S-Go language. No `compat`, optional safety dialect, raw-channel path, unsafe flag, legacy importer, or automatic stock-Go fallback.
- Missing/draft/conflicting/stale/unsupported contract means rejection. Cover syntax, values, types, callables, effects, dependencies, intrinsics, and target-runtime boundaries. An unchecked annotation is not a valid enforcement mechanism.
- Explicitly adopted upstream clauses need frozen content/revision, closed normative references, conflict resolution, and tests. Never use a blanket “same as Go unless changed” rule.
- Bare `go f()` is rejected everywhere in target source. The v2 default uses `n.Spawn(func(*sgo.Context) error)` with an explicit opaque nursery capability. Missing authority is not inferred from runtime context. Runtime state validates the passed capability; it does not replace one.
- Dependencies are ported and contract-checked or rejected. Existing names, APIs, standard-library provenance, or successful recompilation do not confer admission.
- Target-runtime trust does not waive specification. Every sealed primitive/service has a lifecycle/authority contract and audited callers; no broad trusted-package escape route.
- Production and lab share all safety rules. Lab additionally controls/records allowed nondeterminism and rejects unsupported replay effects without falling back to production.
- Bootstrap with a separately pinned host Go compiler if needed. Host implementation code is not S-Go target input. Map/rework inherited self-hosting stages instead of adding a legacy frontend mode. Report host-built versus self-hosted truthfully.
- Every nursery exit joins owned work and resolves obligations. Keep real scope-exit lowering, GC-safe records, specified failure/cancellation ordering, and one-winner permit/wait protocols. Never abandon live tasks and report successful closure.

W00 must verify and lock the `go1.27.3` source default (D21), exact commit, and host bootstrap/source checksums. An already established justified base can be retained with an explicit source-map decision. Do not invent source identities or let a build tool substitute a different compiler.

Keep growable-stack machinery for the core correctness work; W11 investigates fixed stacks. That is an implementation experiment, not compatibility policy. Do not apply the historical stack/IR snippets mechanically.

W01 must expand the admission inventory against actual parser/type/IR dispatch and exported/intrinsic surfaces, freeze a complete first-subset specification and TCB contracts, create explicit adoption/rejection entries, and implement the executable state model. W03 must enforce the registry through all build entry points and transitive imports. Later work packages enlarge the admitted subset only after specification and conformance gates.

For each package: inspect real source, identify clauses/invariants, add the focused positive/negative/model tests, implement a coherent change, run it, review the diff, and record actual evidence. Do not mark planned or unrun tests passed. Do not weaken an expected outcome to match the implementation. Compare upstream behavior only in a separate reference build, never an S-Go compatibility lane.

Maintain task/admission/guarantee ledgers and a resumable checkpoint. If a contract is incomplete, leave that feature rejected while completing independent valid work. Resolve implementation details through recorded amendments; do not reopen the user's settled constraints.

Before ending, record completed changes, actual commands/test outcomes, failed/unrun cases, admitted/rejected features, blockers, commit/dirty files, and the next dependency-ready action. No invented builds, benchmark numbers, formal proofs, or successful guarantees.

**Start by inspecting the repository and executing W00.**
