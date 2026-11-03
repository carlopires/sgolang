# W15 drafts: runes and text iteration (for approval)

Status: **draft, awaiting owner approval.** These clauses are kept here, outside `sgo/spec/`, so the
frozen 0.11.0 digest does not change. On approval they freeze at spec **0.12.0**. They implement
D106 (runes, approved 2026-11-03) and follow the process of D110
(`sgo/docs/stdlib-foundations-plan.md`).

Summary: `rune` becomes an admitted, checked integer type (0..0x10FFFF), with rune literals, the
`\u`/`\U` escapes, `range` over a string by rune, and the conversions `string(r)`, `[]rune(s)` and
`string(rs)`. Nothing else changes. Strings stay sequences of bytes, `len(s)` stays a byte count,
and `s[i]` stays a byte.

## Small decisions taken in these drafts (flagged for the owner)

| ID | Question | Draft choice |
|---|---|---|
| D116 | Surrogates (U+D800..U+DFFF) are not valid Unicode scalar values. Are they rune values? | Yes, as values: the range check is only 0..0x10FFFF, so arithmetic on runes stays a plain checked range (as for `byte`). They are refused where text is *made*: a rune literal or `\u` escape naming a surrogate is a compile error (as in Go). Encoding one (`string(r)`, `string(rs)`) yields U+FFFD, which is Go's behavior. Making surrogates an `IntegerOverflow` would give the type a hole in its range that no other integer type has. |
| D117 | How is invalid UTF-8 decoded? | Exactly as Go does: each maximal invalid prefix (an invalid or truncated sequence, an overlong form, or an encoded surrogate) decodes to U+FFFD and advances **one byte**. `range` over a string, `[]rune(s)`, and W16's `std/utf8` all use this one rule. |
| D118 | What type does an untyped rune constant take, and where does it fit? | A rune literal is an untyped rune constant. Its default type is `rune`, and it is assignable to `int`, `byte`, or `rune` (and named types over them) when representable, so `b == 'a'` works for a byte b. Typed operands are not mixed: `rune + int` needs a conversion (`SGO_E_TYPE_MISMATCH`), as for `byte`. |
| D119 | Which escapes are admitted? | `\u` and `\U` become admitted in both string and rune literals (non-ASCII text without raw UTF-8 in the source). `\'` is admitted in rune literals only. Escapes naming a surrogate or a value above 0x10FFFF are a compile error. `\a \b \f \v` and octal `\NNN` stay denied. In a rune literal, `\xHH` is the code point 0xHH. In a string it is still one byte. |
| D120 | Which conversions involve `rune`? | `rune(i)` and `int(r)`; `rune(b)` (always in range) and `byte(r)` (checked, 0..255); `string(r)`; `[]rune(s)`; `string(rs)`. `string(i)` for an `int` i stays denied: it is a known source of bugs (`string(65)` is `"A"`, not `"65"`), and `string(rune(i))` says what it means. `[]byte(rs)`, `[]rune(b)` and other slice-to-slice conversions are denied; go through a string. |

## Clauses

### SGO-LEX-004: Literals (v2; amends v1)
- **grammar_and_types:** As v1, plus:
  - **Rune literals:** a single character other than `'`, newline or `\` between single quotes,
    or one escape: `\n \t \r \\ \' \xHH \uHHHH \UHHHHHHHH`.
  - **String literals** also admit `\uHHHH` and `\UHHHHHHHH`. `\'` is not admitted in strings, and
    `\"` is not admitted in rune literals.
  - Still denied: floating-point and imaginary literals, the escapes `\a \b \f \v`, octal `\NNN`.
- **evaluation_and_value_semantics:**
  - A rune literal denotes an untyped rune constant whose value is the code point of its character
    or escape. `\xHH` gives 0xHH (D119).
  - In a string literal, `\u` and `\U` contribute the UTF-8 encoding of their code point, while
    `\xHH` still contributes exactly one byte.
- **failure_and_partial_progress:** A malformed or denied literal is a compile error. This
  includes a rune literal with zero or several characters, and a `\u`/`\U` escape naming a
  surrogate or a value above 0x10FFFF.
- **compile_link_checks:**
  - `SGO_E_SYNTAX` for malformed literals (an empty or multi-character rune literal, a bad escape).
  - `SGO_E_UNADMITTED` for denied literal kinds and escapes.
  - `SGO_E_CONST_RANGE` for a `\u`/`\U` escape naming a surrogate or a value above 0x10FFFF.
- **positive_tests:** fixture:lex-literals, fixture:lex-rune-literals, fixture:lex-unicode-escape
  (now accepted: `\u00e9` in a string).
- **negative_tests:** fixture:lex-float-literal, fixture:lex-legacy-octal,
  fixture:lex-rune-literal-empty, fixture:lex-rune-literal-multi, fixture:lex-escape-surrogate,
  fixture:lex-escape-above-max, fixture:lex-escape-octal.

### SGO-TYPE-008: The rune type (v1; new)
- **reference_closure:** SGO-TYPE-001, SGO-TYPE-002, SGO-TYPE-007, SGO-VAL-001, SGO-VAL-002,
  SGO-VAL-004, SGO-LEX-004, SGO-COMP-005; D106, D116, D118, D120.
- **grammar_and_types:**
  - `rune` is admitted: an integer type with values 0..0x10FFFF.
  - Rune operands combine with rune operands under the operators of SGO-VAL-001 (shifts take an
    int count).
  - Untyped constants (integer or rune) representable in 0..0x10FFFF are assignable to rune.
  - Conversions (D120): `rune(i)` for an int i, `int(r)`, `rune(b)` for a byte b, `byte(r)`.
  - Runes are comparable and ordered, so they can be map keys and `switch` cases.
  - Named types over rune (`type Letter rune`) are admitted like named types over int.
  - `rune` is a distinct type, not an alias: `int32` stays denied (SGO-TYPE-001), and no other
    integer width is introduced.
- **preconditions:** For `rune(i)`: `0 ≤ i ≤ 0x10FFFF`. For `byte(r)`: `r ≤ 255`.
- **evaluation_and_value_semantics:** Rune arithmetic is exact. The mathematical result of `+`,
  `-`, `*`, `/`, `%` or `<<` on runes is the result if it lies in 0..0x10FFFF; `&`, `|` and `>>`
  cannot leave the range. Surrogate values are ordinary rune values (D116).
- **ownership_and_lifecycle:** n/a; a value type.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** A value type: shareable, sendable, and allowed in `error`
  values and `sgo.Frozen` (SGO-TYPE-004).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:**
  - A result outside 0..0x10FFFF, `rune(i)` with i outside that range, and `byte(r)` with r > 255
    are `IntegerOverflow` (SGO-VAL-001).
  - Division by zero is `DivideByZero`.
  - A shift count outside 0..63 is `ShiftRange`.
- **cancellation_and_cleanup:** n/a; rune operations are not cancellation points.
- **compile_link_checks:**
  - `SGO_E_CONST_OVERFLOW` for constants outside 0..0x10FFFF, or outside 0..255 when converted
    to byte.
  - `SGO_E_TYPE_MISMATCH` for mixing rune with int or byte without a conversion.
  - `SGO_E_UNADMITTED` for `string(i)` with an int i (D120).
- **runtime_guards:** `IntegerOverflow`, `DivideByZero`, `ShiftRange`.
- **tcb_assumptions:** The checked-arithmetic code generation of SGO-VAL-001 and SGO-TYPE-007,
  extended to a 32-bit operand with the upper bound 0x10FFFF.
- **positive_tests:** fixture:type-rune-arith, fixture:type-rune-convert,
  fixture:type-rune-untyped-const, fixture:type-rune-map-switch, fixture:type-rune-named.
- **negative_tests:** fixture:type-rune-overflow (0x10FFFF + 1), fixture:type-rune-underflow,
  fixture:type-rune-convert-overflow (`rune(0x110000)`), fixture:type-rune-to-byte-overflow,
  fixture:type-rune-const-overflow, fixture:type-rune-int-mismatch,
  fixture:type-string-of-int-denied, fixture:type-int32-denied.

### SGO-TYPE-001: Admitted types (v8; amends v7)
- **grammar_and_types:** As v7, adding `rune` (SGO-TYPE-008, spec 0.12.0) to the admitted types.
  The final sentence becomes: "Other integer widths (including `int32` and `uint8`),
  floating-point, complex, `uintptr` and `any` are denied." This also removes v7's outdated
  mention of `byte`, which has been admitted since 0.9.0.

### SGO-TYPE-007: The byte type (v2; amends v1)
- **grammar_and_types:** As v1, plus `rune(b)` and `byte(r)` (SGO-TYPE-008). Byte constants may
  be written as rune literals when representable (`b == 'a'`). The v1 remark that rune literals
  are denied is removed.
- **preconditions:** As v1, plus for `byte(r)`: `r ≤ 255`.
- **failure_and_partial_progress:** As v1, plus `byte(r)` with r > 255 is `IntegerOverflow`.

### SGO-VAL-002: Strings (v3; amends v2)
- **grammar_and_types:** As v2, plus:
  - `for i := range s`, `for i, r := range s` and `for range s` over a string s. i is an int, r
    is a rune.
  - `string(r)` for a rune r.
  - `[]rune(s)` and `string(rs)` (SGO-COMP-005 v2).
  - `string(i)` for an int i stays denied (D120).
- **evaluation_and_value_semantics:** As v2, plus:
  - **`range`** evaluates its operand once. It then visits the string's runes in order: i is the
    byte offset where each rune starts, and r is the decoded rune. Invalid UTF-8 decodes to
    U+FFFD and advances one byte (D117). The loop body runs once per decoded rune, so a string of
    n bytes gives at most n iterations.
  - **`string(r)`** is the UTF-8 encoding of r: 1 to 4 bytes, or U+FFFD's 3 bytes for a
    surrogate (D116).
  - `len` is still the number of bytes.
- **failure_and_partial_progress:** As v2. Decoding never fails: invalid input is a value
  (U+FFFD), not an error.
- **cancellation_and_cleanup:** `range` over a string is not a cancellation point (as `range` over
  a slice, SGO-COMP-005).
- **compile_link_checks:** `SGO_E_UNADMITTED` for `string(i)` with an int i, and for conversions
  between strings and types other than `[]byte`, `[]rune` and rune.
- **positive_tests:** fixture:val-strings, fixture:val-string-index,
  fixture:val-string-range (now accepted), fixture:val-string-range-invalid-utf8,
  fixture:val-string-range-index-only, fixture:val-string-of-rune.
- **negative_tests:** fixture:type-string-of-int-denied,
  fixture:val-string-range-assign-denied (assigning to a range variable of the wrong type).

### SGO-COMP-005: Slices, `[]byte`, `[]rune`, and string indexing (v2; amends v1)
- **grammar_and_types:** As v1, with these changes:
  - `[]rune(s)` for a string s and `string(rs)` for a `[]rune` rs are added.
  - The sentence "`range` over a string, `[]rune`, and conversions between strings and anything but
    `[]byte` stay denied" becomes "Conversions between strings and slice types other than `[]byte`
    and `[]rune` are denied, as are conversions between slice types; `range` over a string is
    SGO-VAL-002 v3".
- **evaluation_and_value_semantics:** As v1, plus:
  - `[]rune(s)` decodes s as `range` does (D117) into a new slice whose length and capacity equal
    the number of runes.
  - `string(rs)` concatenates the UTF-8 encodings of the elements; a surrogate encodes as U+FFFD
    (D116).
  - Both conversions copy; the result shares no memory with the operand.
- **failure_and_partial_progress:** As v1. Allocation failure is `ResourceExhausted`.
- **compile_link_checks:** `SGO_E_UNADMITTED` for conversions between strings and other slice
  types, and between slice types (`[]byte(rs)`, `[]rune(b)`). The v1 code for `range` over strings
  and `[]rune` is removed.
- **positive_tests:** As v1, plus fixture:comp-rune-slice (round trip, surrogate → U+FFFD,
  invalid UTF-8, mutating the copy leaves the string unchanged).
- **negative_tests:** As v1, without fixture:comp-string-range-denied (which flips to accepted and
  is renamed comp-string-range), plus fixture:comp-slice-convert-denied.

## Implementation plan (after approval)

1. **Lexer and parser** (`syntax/scanner.go`, the S-Go literal checks): rune literals, `\u`/`\U`
   in strings, and the D119 escape set; surrogate and range rejection at compile time.
2. **Types** (`types2/sgo*.go`, not the generated twins):
   - admit `rune` and untyped rune constants (D118);
   - the D120 conversion table;
   - `range` over strings;
   - keep `int32` denied even though Go's `rune` aliases it. The admission check must look at the
     name the source wrote, not only at the underlying type.
3. **Checked arithmetic** (`noder/writer.go` `sgoChecked`, `ssagen/sgo.go`): an upper-bound check
   at 0x10FFFF for rune results and conversions, alongside the existing int and byte paths.
4. **Lowering**: string `range` and the conversions reuse the runtime's `decoderune`,
   `intstring`, `stringtoslicerune` and `slicerunetostring`, which already follow D116/D117.
   They are part of the TCB, so the TCB manifest is regenerated and the inventory is extended by
   their entries.
5. **Model**: no new concurrency, so no state-model work is needed. A table-driven test compares
   the S-Go decoding of every 1–4 byte prefix class with Go's `unicode/utf8`.
6. **Conformance**: the fixtures named above, about 30 new and 3 flipped.
7. **Freeze at 0.12.0**:
   - regenerate the registry, identity constants, inventory and policy;
   - bump every `sgo.mod` (except `imp-spec-version-mismatch`);
   - add a CHANGELOG entry;
   - mutation tests (drop the 0x10FFFF bound, allow `string(i)`, decode invalid bytes as
     themselves) must each be caught;
   - Linux CI evidence in `sgo/evidence/w15/README.md`;
   - reviewer verification.

## Not in W15

The `std/utf8` and `std/strings` functions (W16). `fmt`-style formatting of runes (later).
`int32`/`uint8` aliases (stay denied). `range` over integers or functions (separate decisions).
SGO-CTL-001 still says `for … range` is denied in general. Every later clause that admits a
`range` form already overrides it, so this draft amends nothing there; a cleanup of that sentence
can ride along with the 0.12.0 freeze if you want.
