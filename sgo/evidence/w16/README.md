# W16 evidence: standard library v1 (spec 0.13.0)

These decisions were approved on 2026-11-03:
- D103, D108–D110 (`sgo/docs/stdlib-foundations-plan.md`);
- D121–D126 (`sgo/docs/w16-drafts.md`).

Clauses frozen at 0.13.0:
- new family SPEC-STD: SGO-STD-001 to SGO-STD-007;
- amended: SGO-IMP-004 v2.

Guarantee G-STD is added to `sgo/GUARANTEES.json`.

## What was built

Seven packages under `src/sgostd/`, written in S-Go with no package-level variables (D109):

| Package | Contents |
|---|---|
| `std/errors` | `New` |
| `std/strings` | 34 functions and `Builder` |
| `std/bytes` | 13 functions |
| `std/strconv` | 8 functions |
| `std/utf8` | 4 constants and 15 functions |
| `std/sort` | 10 functions |
| `std/math` | 2 constants and 4 functions |

That is 93 exported declarations, each listed in its clause.

Implementation notes:
- **`utf8` without tables:** it decodes by range comparison, because Go's lookup tables would be
  package variables. Its decoding agrees with `range` over every 1–4 byte boundary prefix.
- **`strconv` ranges:** it reproduces the range behavior of Go's `ParseUint`/`ParseInt` without
  `uint64` by keeping the magnitude in two 32-bit halves. Its error messages are Go's (D121).
- **`sort`:**
  - Every sort is stable, through one merge sort (D123).
  - A function value that captures task-local data cannot be passed as an argument (D94). So the
    typed sorts compare through key slices internally, and the `Perm` and `Search` callbacks must
    compute from their arguments or from shareable data (fixture `std-sort-capture-denied`).

## Checks

| Check | What it verifies | Where |
|---|---|---|
| `sgo/scripts/test-std.py` | each package's `sgo test` suite (17 tests) | sgo-w00 CI |
| `sgo/scripts/test-std-diff.py` (new, D125) | a Go program calling Go's packages and an S-Go program calling `std/` run the same snippets on seeded inputs (edge cases plus random strings with invalid UTF-8) and must print identical lines | sgo-w00 CI |
| `sgo/scripts/std-api-check.py` (new, D126) | every exported declaration equals its clause's list | sgo-spec CI |

## Results (darwin/arm64, local)

- **Conformance:** 532 cases, all pass. 13 are new `std-*` cases, and `std-import` now expects Go's
  `strconv` messages.
- **Differential test:** 9,784 checks (seed 1) and 13,944 checks each (seeds 2 and 3) are identical
  to Go 1.26.10's packages.
- **Mutants**, each caught by the named check:

  | Mutant | Caught by |
  |---|---|
  | `DecodeRuneInString` advancing two bytes on an invalid lead byte | test-std-diff: 1,413 of 9,784 lines differ |
  | `Perm` unstable on equal keys | std-sort-stable |
  | an unexported `strconv` helper exported | std-api-check |
  | a changed `strconv` error message | std-strconv-parse |
  | `strings.Repeat`'s negative-count check removed | std-strings-repeat-negative |

- **Other checks**, all ok:
  - spec-tool (124 clauses), std-manifest, guarantee-check, license-audit;
  - generated-twin-check, test-identity, test-tcb.

## Linux

sgo-w00 run 41682646631 on `5b0fbace8b` (the 0.13.0 freeze `c877824b07` plus the guarantee and conformance-compile fixes): both jobs passed on linux/amd64 and
linux/arm64. That covers the stage-0 builds, conformance (532 cases), test-std, test-std-diff,
the module tests, legacy-audit, the negative fixtures and the upstream baseline (B03).
