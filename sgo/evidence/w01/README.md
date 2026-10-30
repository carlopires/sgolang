# W01 evidence — closed first-subset specification and executable oracle

| Field | Value |
|---|---|
| work_package_id | W01 |
| state | passed on darwin/arm64 (local) and linux/amd64, linux/arm64 (CI `sgo-spec` workflow); see §3 |
| spec_version | `0.1.0` (frozen 2026-10-30, user-approved) |
| admission_digest | `0cb2a1c95d357c93…` (full value in `SGO_SPEC_ADMISSION.json`) |
| upstream_commit | `2aae1000ea1d4b28923b96db28acf4e3b99d6aff` (go1.27.3) |
| host_and_toolchain | bootstrap go1.26.10 (all W01 tools run with it or with python3) |
| execution_configuration | none: no S-Go program executes in W01 |
| next_dependency_ready_task | W02 — runtime ownership, join, and GC-safe state |

## 1. Deliverables (plan §11 W01, `SGO_TASKS.json`)

| Deliverable | Artifact | Check |
|---|---|---|
| Frozen first-subset normative clauses | `sgo/spec/SPEC-*.md`: 84 clauses, 37 families, all 21 required fields each, all `frozen` | `spec-tool.py check` (format, references, no placeholders or "same as Go", frozen clauses cannot change without a version bump) |
| Explicit adoption ledger | `sgo/docs/clause-adoption-ledger.json`: 7 clauses adapting 14 Go-spec sections, digest-locked | `adoption-check.py` |
| Mechanically complete default-deny inventory | `sgo/spec/INVENTORY.map`, `sgo/docs/inventory.md`: 539 compiler/runtime surfaces → clauses (89 admit, 102 internal, 348 deny) | `inventory-check.py` (unmapped surface, dead rule, unknown clause, admit into denied family all fail) |
| TCB primitive contracts | SGO-TCB-001/002 (user-approved), `sgo/TARGET_TCB.json`: 1,020 runtime files pinned, 11 primitives, 11 enumerated runtime services | `tcb-manifest.py` |
| Checked dependency-summary rules | SGO-IMP-002 (contract digests per package), SGO-BIND-002/005 | conformance fixtures `imp-*` (run from W03) |
| Pure state-machine model with bounded enumeration, mutation, and specification-closure checks | `sgo/model/`; `sgo/docs/invariant-tests.md` | `test-model.sh`: 18 scenarios over all interleavings, 8 mutations detected, 5 corrupted states rejected, 7 golden histories |
| Conformance cases for every clause | `sgo/testdata/conformance/`: 170 cases (82 run, 88 reject) | `conformance.py validate`, `selftest` |

## 2. Acceptance cases

| Case | W01 result |
|---|---|
| M01 scope action enumeration | **passed** — every terminal state of every scenario satisfies the invariants; state counts are logged per scenario |
| M02 registration mutation | **passed** — `MutLateRegister` rejected (OWN-2) |
| M03 premature closure mutation | **passed** — `MutEarlyJoin` rejected (OWN-3/OWN-4) |
| M04 duplicate completion mutation | **passed** — `MutDuplicateCompletion` rejected; corrupted state with a double publication rejected |
| M05 join admission race | **passed** — `descendant-during-join`, `spawn-when-cancelling`: valid admissions counted before finalization, none lost |
| S02 closed inventory completeness | W01 part passed (inventory check, including a simulated new IR op); enforcement W03 |
| S03 missing or draft clause denial | W01 part passed (registry admits nothing; `sgo-target` denies; frozen-change protection); compiler enforcement W03 |
| S04 deliberate upstream clause adoption | W01 part passed (digest-locked ledger, no blanket inheritance); review W12 |
| S11 target TCB is specified | W01 part passed (manifest + negative self-tests); caller enforcement W02/W03 |
| S16 value and memory semantics closure | W01 part passed (SPEC-VALUE/MEM clauses with fixtures; static race freedom argument SGO-MEM-003); execution W03 |
| S20 no implementation-only proof claims | W01 part passed (§4 below) |

## 3. Platforms

All W01 tools are target-independent (Python and pure Go). The model suite ran on darwin/arm64 locally and on linux/amd64 and linux/arm64 in CI; the spec, inventory, TCB, adoption, and conformance checks run in CI on linux/amd64 and locally on darwin/arm64.

## 4. What W01 does and does not establish (S20)

- **Does:** the subset-0 language is fully specified as frozen clauses; every compiler/runtime surface that exists today is mapped to admit/internal/deny; the runtime is pinned by digest; the concurrency semantics of nurseries, spawn, cancellation, waits, and outcomes are consistent (the model explores every interleaving of 18 small programs and detects 8 kinds of semantic bugs).
- **Does not:** prove arbitrary programs correct or terminating; prove the model equal to the future implementation (W02–W04 must pass the same scenarios and the conformance cases); cover larger programs than the explored scenarios; implement or enforce anything in the compiler or runtime. No S-Go program has been compiled or run.

## 5. Known limitations

1. The inventory covers six surface kinds of the compiler and the runtime builtins it calls; public `go/*` packages, the linker, and the future `sgo` package API are added when they change (W03).
2. Conformance positions are defined by the clauses and SGO-ERR-001; if W03 finds a rule impractical, it is changed by a versioned clause amendment, not by editing fixtures to match.
3. `SGO_E_STALE_CONTRACT` and `SGO_E_FOREIGN_OBJECT` at link level are unit-test obligations for W03, not conformance cases.
