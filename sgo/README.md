# S-Go execution handoff v2

**This bundle replaces v1.0.** S-Go is an independent specification-defined language built from a Go compiler/runtime fork, not a compatible Go dialect. No compatibility mode, optional safety tier, uncontracted dependency, or unspecified fallback is permitted.

## Start

Give the coding agent this bundle and the actual repository. Paste `SGO_AGENT_START_PROMPT.md`. The agent must first read `SGO_LANGUAGE_CONSTITUTION.md`, then inspect existing work and execute the dependency-ready packages. No compiler/runtime implementation is included.

## Contents

| File | Purpose |
|---|---|
| `SGO_LANGUAGE_CONSTITUTION.md` | User constraints, specification validity/admission requirements, explicit-spawn default, and bootstrap boundary. |
| `SGO_DECISIONS.md` | User-approved design decisions D01–D21 (2026-10-30) and open decision D22 (first target); defines subset 0 scope. Approved amendments to the plan. |
| `SGO_FRONTIER_EXECUTION_PLAN.md` | Fully revised architecture, contracts, 13 work packages, source map, evidence and release requirements. |
| `SGO_AGENT_START_PROMPT.md` | Replacement paste-ready execution prompt. |
| `SGO_SPEC_SUBSET0.md` | Draft specification of subset 0, the closed first-slice language derived from the approved decisions. Not frozen. |
| `SGO_SPEC_ADMISSION.json` | 37 required surface-family inventory seeds (10 added 2026-10-30), each tagged with its subset-0 status; all unadmitted until expanded and specified. Not a complete spec. |
| `SGO_TASKS.json` | Dependency graph and pending execution ledger. |
| `SGO_TEST_MATRIX.json` | 147 acceptance specifications, all unrun, including 20 added no-legacy/specification checks. |
| `CHANGELOG_V2.md` | Exact architectural correction and remaining evidence limits. |
| `original/` | Seven unchanged historical input files; not active authority over v2. |
| `MANIFEST.json` | Provenance, hashes, and document-validation results. |

## Interpretation

Bare `go` is rejected in the recommended v2 design. The proposed `n.Spawn` API requires explicit nursery authority; runtime context only checks that authority. Production and lab enforce the same safety rules. Go sources and packages can be reused only after their relevant semantics and enforcement are deliberately specified and ported. Host Go tools may build the compiler without becoming an S-Go target language path.

The core still proceeds from isolated bootstrap and a closed specification to GC-safe task ownership, nursery exits, cancellation, capabilities, channel contracts, replay, and production adapters. Fixed virtual stacks remain a separate measured investigation. These are sequencing choices, not ecosystem compromises.

## Status and source provenance

All implementation tasks are pending and all acceptance scenarios unrun. The specification registry is a work inventory, not proof that the whole language has already been specified. No S-Go compiler was built or tested for this revision. External references are carried forward from v1, not freshly verified; W00 must validate exact source and toolchain identities. Document validation is not language conformance.
