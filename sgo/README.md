# S-Go project bundle

This directory holds S-Go's specification, decisions, plans, tools, tests, and evidence. The
compiler and runtime are in `src/` (a fork of Go 1.27.3); the top-level [`README.md`](../README.md)
introduces the language.

## Current status

Spec **0.20.0** (current version: [`spec/VERSION`](spec/VERSION)). The `sgo` toolchain (`build`,
`run`, `check`, `test`, `fmt`, `mod`, `lab`) is implemented and tested on darwin/arm64,
linux/amd64, and linux/arm64. Work packages W00–W25 are complete (W11, fixed virtual stacks, was
investigated and not adopted). Each spec version is frozen, every clause names its conformance
fixtures, and each work package has an evidence record.

| Where | What |
|---|---|
| [`spec/`](spec/README.md) | The normative clauses (`SPEC-*.md`), the version, and [`CHANGELOG.md`](spec/CHANGELOG.md). |
| [`SGO_SPEC_ADMISSION.json`](SGO_SPEC_ADMISSION.json) | The admission registry: each surface family, its clauses, digests, and status. |
| [`docs/GUARANTEES.md`](docs/GUARANTEES.md) | The published guarantees and the clauses and tests behind each. |
| [`SGO_DECISIONS.md`](SGO_DECISIONS.md) | Design decisions D01 onward, with the user's approvals. |
| [`SGO_LANGUAGE_CONSTITUTION.md`](SGO_LANGUAGE_CONSTITUTION.md) | The constraints every decision and clause must satisfy. |
| [`SGO_TASKS.json`](SGO_TASKS.json), [`SGO_TEST_MATRIX.json`](SGO_TEST_MATRIX.json) | The work-package ledger and the acceptance matrix. |
| [`testdata/conformance/`](testdata/conformance/) | Conformance cases (`sgo/scripts/conformance.py`). |
| [`examples/`](examples/) | Example programs, built and swept in the lab by `sgo/scripts/test-examples.py`. |
| [`evidence/`](evidence/) | Per-work-package evidence records (CI runs, measurements, review findings). |
| [`docs/INSTALL.md`](docs/INSTALL.md) | Installing a release archive. |
| [`scripts/`](scripts/) | The build, gate, and test scripts. |

## Historical: the v2 planning bundle

The text below is the handoff of the v2 planning bundle (2026-10-30), kept for history (only its heading levels changed). It
describes the project before any implementation; its status statements no longer hold, and the
hashes in [`MANIFEST.json`](MANIFEST.json) describe the bundle as it was delivered.

**This bundle replaces v1.0.** S-Go is an independent specification-defined language built from a Go compiler/runtime fork, not a compatible Go dialect. No compatibility mode, optional safety tier, uncontracted dependency, or unspecified fallback is permitted.

### Start

Give the coding agent this bundle and the actual repository. Paste `SGO_AGENT_START_PROMPT.md`. The agent must first read `SGO_LANGUAGE_CONSTITUTION.md`, then inspect existing work and execute the dependency-ready packages. No compiler/runtime implementation is included.

### Contents

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

### Interpretation

Bare `go` is rejected in the recommended v2 design. The proposed `n.Spawn` API requires explicit nursery authority; runtime context only checks that authority. Production and lab enforce the same safety rules. Go sources and packages can be reused only after their relevant semantics and enforcement are deliberately specified and ported. Host Go tools may build the compiler without becoming an S-Go target language path.

The core still proceeds from isolated bootstrap and a closed specification to GC-safe task ownership, nursery exits, cancellation, capabilities, channel contracts, replay, and production adapters. Fixed virtual stacks remain a separate measured investigation. These are sequencing choices, not ecosystem compromises.

### Status and source provenance (v2, superseded by the status above)

All implementation tasks are pending and all acceptance scenarios unrun. The specification registry is a work inventory, not proof that the whole language has already been specified. No S-Go compiler was built or tested for this revision. External references are carried forward from v1, not freshly verified; W00 must validate exact source and toolchain identities. Document validation is not language conformance.
