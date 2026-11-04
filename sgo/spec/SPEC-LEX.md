# SPEC-LEX — Lexing, grammar, reserved words, and source recovery

Subset 0. Derived from `SGO_SPEC_SUBSET0.md` §1, decisions D14/D19, and resolutions R2.

### SGO-LEX-001 — Source text
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-ERR-001; D19; R2.
- **grammar_and_types:** A source file is a sequence of Unicode code points encoded as UTF-8, stored in a file whose name ends in `.sgo`. A byte-order mark (U+FEFF) is permitted only as the first code point and is ignored. Files ending in `.go` or any other extension are never S-Go source.
- **preconditions:** The file is readable and its bytes are valid UTF-8.
- **evaluation_and_value_semantics:** n/a — lexical rule; produces no run-time value.
- **ownership_and_lifecycle:** n/a — no run-time objects.
- **effect_and_capability_requirements:** n/a — compile-time only.
- **concurrency_and_memory_order:** n/a — compile-time only.
- **allowed_nondeterminism:** None. The token sequence is a function of the file bytes.
- **failure_and_partial_progress:** Invalid UTF-8, a NUL byte, or a misplaced byte-order mark is a compile error at the first offending byte; no object file is produced for the package.
- **cancellation_and_cleanup:** n/a — compile-time only.
- **compile_link_checks:** `SGO_E_SOURCE_ENCODING` with file, line, column. The driver rejects non-`.sgo` inputs with `SGO_E_UNADMITTED` naming the file.
- **runtime_guards:** n/a — no run-time behavior.
- **tcb_assumptions:** n/a — compiler only.
- **positive_tests:** S02; fixture:lex-utf8-ok.
- **negative_tests:** S01, S19; fixture:lex-bad-utf8, fixture:lex-go-extension.
- **implementation_coverage:** W03: `src/cmd/compile/internal/syntax/scanner.go`, `src/cmd/compile/internal/syntax/parser.go`, `src/cmd/sgo/build.go`.
- **executed_evidence:** conformance fixtures lex-bad-utf8, lex-go-extension, lex-utf8-ok (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-LEX-002 — Tokens, comments, and semicolons
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-LEX-001, SGO-LEX-003; D19.
- **grammar_and_types:** Tokens are identifiers, keywords, operators and punctuation, and literals. Identifiers are a letter or `_` followed by letters, digits, or `_`, where letters are Unicode categories Lu, Ll, Lt, Lm, Lo and digits are category Nd. Comments are `//` to end of line and `/* … */` (not nested); a comment containing a newline acts like a newline. Admitted operators: `+ - * / % & | ^ &^ << >> && || ! == != < <= > >= = := += -= *= /= %= &= |= ^= &^= <<= >>= ++ -- ( ) { } , ; . :`, and from spec 0.9.0 `[ ]` and `...` (SGO-COMP-005). The scanner recognizes all other Go operators (e.g. `~`) so that their use is reported by SGO-ERR-001 rather than misparsed. Automatic semicolon insertion: a semicolon is inserted after a line's final token if that token is an identifier, an integer or string literal, one of the keywords `break`, `continue`, `return`, or one of `++ -- ) }`. These rules are adapted from the Go specification at go1.27.3 (D19) and are complete as stated here.
- **preconditions:** SGO-LEX-001 holds.
- **evaluation_and_value_semantics:** n/a — lexical rule.
- **ownership_and_lifecycle:** n/a — no run-time objects.
- **effect_and_capability_requirements:** n/a — compile-time only.
- **concurrency_and_memory_order:** n/a — compile-time only.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** An invalid character or unterminated comment is a compile error; the compiler may continue scanning to report more errors but produces no object file.
- **cancellation_and_cleanup:** n/a — compile-time only.
- **compile_link_checks:** `SGO_E_SYNTAX` for malformed tokens; `SGO_E_UNADMITTED` for recognized but denied operators (SGO-ERR-001).
- **runtime_guards:** n/a — no run-time behavior.
- **tcb_assumptions:** n/a — compiler only.
- **positive_tests:** X01, X03; fixture:lex-semicolons.
- **negative_tests:** X01; fixture:lex-denied-operators.
- **implementation_coverage:** W03: `src/cmd/compile/internal/syntax/scanner.go`, `src/cmd/compile/internal/syntax/parser.go`, `src/cmd/sgo/build.go`.
- **executed_evidence:** conformance fixtures lex-denied-operators, lex-semicolons (darwin/arm64, linux/amd64, linux/arm64); `sgo/evidence/w03/README.md`.
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-LEX-003 — Reserved words
- **version:** 2
- **status:** draft
- **reference_closure:** SGO-LEX-002, SGO-SPAWN-005, SGO-ERR-001, SGO-ERR-004; D11, D14, D147.
- **grammar_and_types:** The reserved words are `break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var nursery check`. A reserved word is never an identifier, in any position. Of these, `chan`, `fallthrough`, `go`, `goto`, `interface`, `map`, `range`, `select`, `struct`, and `type` begin constructs denied in subset 0.
- **preconditions:** n/a — applies to every token.
- **evaluation_and_value_semantics:** n/a — lexical rule.
- **ownership_and_lifecycle:** n/a — no run-time objects.
- **effect_and_capability_requirements:** n/a — compile-time only.
- **concurrency_and_memory_order:** n/a — compile-time only.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** Using `nursery` (or any reserved word) as an identifier is a compile error. Using a denied construct's keyword produces that construct's denial diagnostic.
- **cancellation_and_cleanup:** n/a — compile-time only.
- **compile_link_checks:** `SGO_E_RESERVED_WORD` for a reserved word used as a name; `SGO_E_UNSTRUCTURED_SPAWN` for `go`; `SGO_E_UNADMITTED` for the other denied keywords. The compiler's keyword table, `go/token`, and the formatter agree on the list (checked by test).
- **runtime_guards:** n/a — no run-time behavior.
- **tcb_assumptions:** n/a — compiler only.
- **positive_tests:** X01, X02; fixture:lex-keywords.
- **negative_tests:** S05, S19; fixture:lex-nursery-as-name, fixture:lex-bare-go, fixture:err-check-reserved-word.
- **implementation_coverage:** partial — keyword table has room for `nursery` (fork commit 77100c67, TestKeywordHashRoom); the `nursery` token itself is W03.
- **executed_evidence:** TestKeywordHashRoom and TestKeywordsStillScan passed on darwin/arm64 (host test lane) and on linux/amd64 and linux/arm64 (CI run 41085990334).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).

### SGO-LEX-004 — Literals
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-VAL-004, SGO-ERR-001, SGO-TYPE-008; D03, D04, D119; R2.
- **grammar_and_types:** Integer literals: decimal (`0` or a non-zero digit followed by digits), hexadecimal (`0x`/`0X`), octal (`0o`/`0O`), binary (`0b`/`0B`), with optional single `_` between digits or after the base prefix. A leading-zero form such as `012` is a compile error (no legacy octal). String literals: interpreted (`"…"`, escapes exactly `\n \t \r \\ \" \xHH \uHHHH \UHHHHHHHH`, no raw newline) and raw (backquoted, no escapes, carriage returns removed). Rune literals (spec 0.12.0): one character other than `'`, newline, or `\` between single quotes, or one escape `\n \t \r \\ \' \xHH \uHHHH \UHHHHHHHH`. Floating-point and imaginary literals and the escapes `\a \b \f \v \NNN` are denied; `\'` is not an escape of string literals nor `\"` of rune literals.
- **preconditions:** SGO-LEX-001 holds.
- **evaluation_and_value_semantics:** An integer literal denotes an untyped integer constant of unbounded size (SGO-VAL-004). A string literal denotes an untyped string constant whose bytes are the UTF-8 encoding of its characters, with each `\xHH` contributing exactly one byte and each `\u`/`\U` escape the UTF-8 encoding of its code point. A rune literal denotes an untyped rune constant (default type `rune`, SGO-TYPE-008) whose value is the code point of its character or escape; `\xHH` in a rune literal is the code point 0xHH (D119).
- **ownership_and_lifecycle:** n/a — constants have no lifecycle.
- **effect_and_capability_requirements:** n/a — compile-time only.
- **concurrency_and_memory_order:** n/a — immutable constants.
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** A malformed or denied literal is a compile error, including a rune literal with zero or several characters and a `\u`/`\U` escape naming a surrogate (U+D800..U+DFFF) or a value above 0x10FFFF.
- **cancellation_and_cleanup:** n/a — compile-time only.
- **compile_link_checks:** `SGO_E_SYNTAX` for malformed literals (an empty or multi-character rune literal, an escape not of its literal kind); `SGO_E_UNADMITTED` for denied literal kinds and escapes; `SGO_E_CONST_RANGE` for a `\u`/`\U` escape naming a surrogate or a value above 0x10FFFF.
- **runtime_guards:** n/a — no run-time behavior.
- **tcb_assumptions:** n/a — compiler only.
- **positive_tests:** S16; fixture:lex-literals, fixture:lex-rune-literals, fixture:lex-unicode-escape.
- **negative_tests:** S16; fixture:lex-float-literal, fixture:lex-legacy-octal, fixture:lex-rune-literal-empty, fixture:lex-rune-literal-multi, fixture:lex-escape-surrogate, fixture:lex-escape-above-max, fixture:lex-escape-octal, fixture:lex-escape-bell.
- **implementation_coverage:** W03: `src/cmd/compile/internal/syntax/scanner.go`, `src/cmd/compile/internal/syntax/parser.go`, `src/cmd/sgo/build.go`; W15: `sgoadmit/admit_expr.go` (`basicLit`, `escapes`), `noder/noder.go` (`sgoSyntaxMsg`).
- **executed_evidence:** W03 (sgo/evidence/w03/README.md); W15 conformance (sgo/evidence/w15/README.md).
- **supported_execution_configurations:** production; lab denied until W08 (SPEC-LAB).
