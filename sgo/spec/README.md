# S-Go specification

Normative clauses of S-Go, one file per surface family (`SPEC-<FAMILY>.md`, matching the rows of `sgo/SGO_SPEC_ADMISSION.json`). Format and identity follow decision D18. The version of the whole specification is in `VERSION`.

Precedence (constitution §1): user requirements → constitution → **frozen clauses here** and approved decisions → execution plan. A clause never relies on "Go behavior" (constitution §3). Where wording is adapted from the Go specification, the clause says so and is self-contained (D19).

## Clause format

```text
### SGO-<CODE>-<NNN> — Title
- **version:** 1
- **status:** draft | frozen | superseded
- **<field>:** text (continuation lines are indented by two spaces)
```

Every clause has all of the following fields. `n/a — <reason>` is allowed when a dimension does not apply; a bare `n/a`, `TODO`, or "same as Go" is rejected by `sgo/scripts/spec-tool.py`.

| Field | Content |
|---|---|
| reference_closure | Clauses and decisions this clause depends on. The tool checks that every `SGO-…` reference exists. |
| grammar_and_types | Syntax and static typing rules. |
| preconditions | What must hold before the construct is used, statically or dynamically. |
| evaluation_and_value_semantics | Order of evaluation, results, values. |
| ownership_and_lifecycle | Who owns what is created; when it ends. |
| effect_and_capability_requirements | External effects and the capability each needs. |
| concurrency_and_memory_order | Sharing, synchronization, happens-before. |
| allowed_nondeterminism | The complete set of permitted outcomes where more than one exists. |
| failure_and_partial_progress | Failure outcomes and what has already happened when they occur. |
| cancellation_and_cleanup | Interaction with cancellation, defers, and nursery exits. |
| compile_link_checks | Checks the compiler/linker performs, with diagnostic codes. |
| runtime_guards | Checks performed at run time, with failure kinds. |
| tcb_assumptions | Trusted runtime primitives relied on (`SPEC-TCB`). |
| positive_tests / negative_tests | Acceptance-case IDs from `SGO_TEST_MATRIX.json` and `fixture:<name>` conformance programs under `sgo/testdata/conformance/`. |
| implementation_coverage | Where it is implemented (none until W03 for most clauses). |
| executed_evidence | Recorded runs (none until implemented). |
| supported_execution_configurations | `production`; `lab` is denied until W08 (SPEC-LAB). |

## Status rules

- **draft**: may still change. Fields may say `PENDING: …`.
- **frozen**: content fixed; its digest is recorded in the registry. No `PENDING` content; every `fixture:` it names must exist. Changing the text requires a version bump and a recorded amendment (`spec-tool.py update` refuses silent changes).
- **superseded**: kept for history; replaced by a newer clause ID.

A frozen clause is **specified**, not **admitted**. A registry row becomes `admitted` only when its clauses are frozen, implemented, enforced, and have executed evidence (constitution §6). Until then the target entry point denies it.

## Commands

```sh
sgo/scripts/spec-tool.py check    # validate; fail if the registry is stale (CI)
sgo/scripts/spec-tool.py update   # write clause IDs, digests, spec_version, admission_digest
sgo/scripts/spec-tool.py list     # one line per clause
```
