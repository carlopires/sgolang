# W15 evidence: runes and text iteration (spec 0.12.0)

These decisions were approved on 2026-11-03:
- D106 (`sgo/docs/stdlib-foundations-plan.md`);
- D116–D120 (`sgo/docs/w15-drafts.md`).

Clauses frozen at 0.12.0:
- new: SGO-TYPE-008 v1;
- amended: SGO-LEX-004 v2, SGO-TYPE-001 v8, SGO-TYPE-007 v2, SGO-VAL-002 v3, SGO-COMP-005 v2,
  and SGO-COMP-001 v5. The SGO-COMP-001 change is consequential: string range and rune
  conversions leave its list of denied forms.

## What was built

- **Admission** (`sgoadmit/admit_expr.go`):
  - `rune` is admitted by name. `int32` stays denied by name, so the only way to reach Go's `Int32`
    kind is to write `rune`.
  - Rune literals and the D119 escape set are admitted: `\u`, `\U`, and `\'` in rune literals.
    `\a \b \f \v` and octal escapes stay denied.
  - The scanner's surrogate and above-max escape errors become `SGO_E_CONST_RANGE`
    (`noder/noder.go`).
- **Types** (`types2/sgo_values.go`, `types2/sgo_flow.go`):
  - rune values are admitted;
  - the D120 conversion table applies, and `string(i)` of an int stays refused;
  - `range` over strings is admitted, including untyped constant operands, whose type is still
    pending when the S-Go range check runs;
  - typed rune constants are checked against 0..0x10FFFF (`sgoRuneConsts`), since Go would only
    check the int32 range.
- **Code generation** (`ssagen/sgo.go`, `ssagen/ssa.go`):
  - checked rune arithmetic, negation and shifts, sharing the byte path with the bound 0x10FFFF and
    a maximum shift count of 20;
  - `rune(i)` and `byte(r)` are checked through a single narrowing-conversion check
    (`sgoConvCheck`).
- **Lowering** (`walk/sgo_text.go`):
  - In S-Go packages, `[]rune(s)` is resliced to `cap == len`. The runtime would otherwise return
    a 32-rune stack buffer or a capacity rounded up to a size class, so the capacity would depend
    on escape analysis and the allocator.
  - The runtime's `decoderune`, `intstring`, `stringtoslicerune` and `slicerunetostring` are now
    internal surfaces (`INVENTORY.map`).

## Results (darwin/arm64, local)

- **Conformance:** 519 cases, all pass. 29 are new, and 3 earlier rejections now run:
  `comp-string-range-denied` (renamed `comp-string-range`), `val-string-range` and
  `lex-unicode-escape`. `validate` passes.
- **Mutants**, each caught by the named case:

  | Mutant | Caught by |
  |---|---|
  | rune bound in the SSA checks widened to the int32 maximum | type-rune-overflow, type-rune-convert-overflow |
  | constant rune bound widened | type-rune-const-overflow |
  | `string(i)` of an int admitted | type-string-of-int-denied |
  | `[]rune` capacity taken from the runtime | comp-rune-slice |
  | invalid UTF-8 decoded as the byte itself (runtime) | val-string-range-invalid-utf8 |

  One more mutant (skipping the reslice outright) did not build and was replaced by the capacity
  one in the table.
- **Other checks**, all ok:
  - spec-tool (117 clauses), inventory, policy, std-manifest;
  - generated-twin-check, guarantee-check, license-audit, legacy-audit;
  - test-identity, test-tcb, test-modules, test-std, test-negative.

## Linux

sgo-w00 run 41663260839 on the freeze commit `0a832a9bc6`: both jobs passed on linux/amd64 and
linux/arm64. That covers the stage-0 builds, conformance (519 cases), the module and
standard-library tests, legacy-audit, the negative fixtures and the upstream baseline (B03).
sgo-spec run 41663261033 also passed on that commit.
