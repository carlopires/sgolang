# W16 drafts: standard library v1 (for approval)

Status: **approved 2026-11-03** (D121–D126 as recommended); freezes at spec 0.13.0. These clauses are kept here, outside `sgo/spec/`, so the
frozen 0.12.0 digest does not change. On approval they freeze at spec **0.13.0**. They implement
D103 and D108–D110 of `sgo/docs/stdlib-foundations-plan.md` (approved 2026-11-03) for the packages
the plan names: `errors`, `strings`, `bytes`, `strconv`, `utf8`, `sort`, `math`.

This first library is written without interfaces, generics, variadic parameters or floats. All
four stay denied until their own work packages. Every package is S-Go source under
`$GOROOT/src/sgostd/` with no package-level variables (D109). It is compiled under the same
admission and sharing rules as user code and listed in `MANIFEST` (SGO-IMP-004).

## Small decisions taken in these drafts (flagged for the owner)

| ID | Question | Draft choice |
|---|---|---|
| D121 | How do library errors work without interfaces or error comparison? | `std/errors` v1 is `New(msg) error` only (the same value as `sgo.NewError`). `Is`, `As`, `Unwrap` and `Join` need type assertions or comparison of interface values, so they come with W17. Until then a library error is distinguished only by being non-nil and by its message. Each clause states the exact message format (e.g. `strconv.Atoi: parsing "x": invalid syntax`, as in Go), so a program and a test can rely on it. |
| D122 | How much of Unicode is included? | Classification by the **White_Space** property is complete: it is 25 code points, written as a `switch` with no table. So `TrimSpace` and `Fields` behave as in Go. **Case mapping is ASCII-only**, and the names say so (`ToUpperASCII`, `ToLowerASCII`, `EqualFoldASCII`). Go's `ToUpper` uses Unicode tables, and a function with Go's name but ASCII behavior would be a trap. Full case mapping waits for a way to ship tables without package variables (constant strings are the likely route). |
| D123 | What does `sort` offer without generics, interfaces or element-writing callbacks? | Typed in-place sorts (`Ints`, `Strings`, `Runes`, `Bytes`) and their `IsSorted…` checks; `Search(n, f)` with `SearchInts` and `SearchStrings`; and `Perm(n, less) []int`, which returns the sorted order as indices so the caller can apply it. The callback `less(i, j int) bool` only reads; no `swap` callback has to write through a captured slice. **Every sort is stable** and uses one fixed algorithm (merge sort), so equal elements, and therefore results, are the same on every platform and run. That rules out Go's unstable `sort.Ints` behavior. |
| D124 | What happens when a library function is called outside its domain? | Conditions an input can cause and Go reports as errors are errors here too (parse errors, out-of-range parse results). Preconditions that Go enforces by panicking are `panic` here too (SGO-DEF-002, kind `Panic`, exit status 2), with a message naming the function (`strings.Repeat: negative count`). Arithmetic overflow inside a library function is the checked `IntegerOverflow` (SGO-VAL-001) at the library's source position, named `std/<file>`. The library never fails silently. |
| D125 | How is "behaves like Go" made checkable? | Every function whose name and parameter shape match a Go function has that function's documented results on S-Go's types. A **differential test** compares the two: a host program runs Go's package and the S-Go program runs `std/` on the same generated inputs (edge cases plus seeded random strings, including invalid UTF-8), and the outputs must be identical. Deliberate deviations are listed in each clause (for example `strconv.ParseInt` has no `bitSize` because `int` is the only integer width). |
| D126 | Is the library's API part of the specification? | Yes. Each package's exported API and behavior is a frozen clause (SGO-STD-001..007, new family `SPEC-STD.md`). Adding a function later is a clause amendment and a spec version, like any other surface. The admission registry and identity digests cover the library through these clauses and through `MANIFEST`. |

## Clauses (summary form; the frozen clauses carry every field of the clause format)

Each clause's grammar field lists the exported API; its semantics field says "as Go's function of
the same name" plus the listed deviations; and its tests field names conformance cases and the
package's `sgo test` suite.

### SGO-STD-001: `std/errors` (v1; new)
- **API:** `func New(text string) error`.
- **Semantics:** as `sgo.NewError`: `New(text).Error() == text`. Two calls give two values; error
  values are not comparable (SGO-TYPE-006).
- **Deferred to W17 (D121):** `Is`, `As`, `Unwrap`, `Join`.

### SGO-STD-002: `std/strings` (v1; new)
- **Search:** `Contains`, `ContainsRune`, `Count`, `HasPrefix`, `HasSuffix`, `Index`, `IndexByte`,
  `IndexRune`, `LastIndex`, `Compare`.
- **Split and join:** `Cut`, `CutPrefix`, `CutSuffix`, `Fields`, `Split`, `SplitN`, `Join`.
- **Transform:** `Repeat`, `Replace`, `ReplaceAll`, `Trim`, `TrimLeft`, `TrimRight`,
  `TrimPrefix`, `TrimSuffix`, `TrimSpace`, `ToUpperASCII`, `ToLowerASCII`, `EqualFoldASCII`.
- **`type Builder`:** a struct over a `[]byte`, with pointer-receiver methods `WriteString`,
  `WriteByte`, `WriteRune` (each returning `error`, always nil, as Go's), `Len`, `String`, `Reset`.
  A Builder is task-local (SGO-COMP-004); `String` copies.
- **Semantics:** Go's, with these deviations:
  - **No variadic functions:** none of Go's `strings` API is variadic, so nothing changes
    (`Join` takes `[]string`, as in Go).
  - **Case:** case functions are ASCII-only, as their names say (D122).
  - **`Fields`/`TrimSpace`:** they use the full White_Space set.
  - **Invalid UTF-8:** handled as in Go (each invalid byte is one U+FFFD where runes are
    involved, D117).
- **Failures:** `Repeat` with a negative count panics: `strings.Repeat: negative count`. A
  result too large is `IntegerOverflow` or `ResourceExhausted`.

### SGO-STD-003: `std/bytes` (v1; new)
- **API:** `Equal`, `Compare`, `Contains`, `Count`, `Index`, `IndexByte`, `LastIndex`,
  `HasPrefix`, `HasSuffix`, `Join`, `Split`, `TrimSpace`, `Clone`.
- **Semantics:** Go's, on `[]byte`.
  - Results that are slices of the argument alias it, as in Go (`TrimSpace`, `Split`).
  - `Clone` and `Join` return new arrays.
  - Aliasing is within one task (SGO-COMP-004).

### SGO-STD-004: `std/strconv` (v2 of the 0.11.0 package; new clause)
- **Kept:** `Itoa`, `Atoi`.
- **Added:** `ParseInt(s string, base int) (int, error)`, `FormatInt(v int, base int) string`,
  `ParseBool`, `FormatBool`, `QuoteToASCII`, `Unquote`.
- **Semantics:** Go's, with these deviations:
  - `ParseInt` has no `bitSize`: the result is always a 64-bit `int` (D125).
  - `Quote` is not offered, because it needs Unicode printability tables (D122). `QuoteToASCII`
    escapes every non-ASCII rune and gives the same result as Go's.
- **Error messages (D121):** exactly Go's:
  - `strconv.<Func>: parsing "<input>": invalid syntax`;
  - `... value out of range`;
  - `... invalid base <b>`.
  The input is quoted as `QuoteToASCII` would quote it.

### SGO-STD-005: `std/utf8` (v1; new)
- **Constants:** `RuneError rune = 0xFFFD`, `RuneSelf = 0x80`, `MaxRune rune = 0x10FFFF`,
  `UTFMax = 4`.
- **Functions:** `RuneLen`, `EncodeRune`, `AppendRune`, `DecodeRune`, `DecodeRuneInString`,
  `DecodeLastRune`, `DecodeLastRuneInString`, `FullRune`, `FullRuneInString`, `RuneCount`,
  `RuneCountInString`, `RuneStart`, `Valid`, `ValidString`, `ValidRune`.
- **Semantics:** as Go's `unicode/utf8`. Decoding follows D117, so `DecodeRuneInString` and `range`
  over a string agree byte for byte; a test checks this over every 1–4 byte prefix class.
- **Import path:** `std/utf8`, not `std/unicode/utf8`, because there is no `unicode` package yet.

### SGO-STD-006: `std/sort` (v1; new)
- **API:**
  - `Ints`, `Strings`, `Runes`, `Bytes` (in place);
  - `IntsAreSorted`, `StringsAreSorted`;
  - `Search(n int, f func(int) bool) int`, `SearchInts`, `SearchStrings`;
  - `Perm(n int, less func(i, j int) bool) []int`.
- **Semantics:**
  - Every sort is stable (D123), using one merge sort.
  - `Perm` returns the indices 0..n−1 in stable sorted order under `less`. `less` must be a strict
    weak order; if it is not, the result is still a permutation, determined by the algorithm, and
    the clause says so.
  - `Search` is Go's binary search.
- **Failures:**
  - `Perm` with n < 0 panics: `sort.Perm: negative length`.
  - A `less` that fails propagates its failure, as any call does.

### SGO-STD-007: `std/math` (v1; new)
- **Constants:** `MaxInt`, `MinInt`.
- **Functions:** `Abs(x int) int`, `Min(a, b int) int`, `Max(a, b int) int`, `Pow(x, n int) int`.
- **Semantics:** integer only (floats stay denied, SGO-FLOAT-001).
  - `Abs(MinInt)` and an overflowing `Pow` are `IntegerOverflow` (checked arithmetic, D124).
  - `Pow` with n < 0 panics: `math.Pow: negative exponent`.
- **Name collision:** Go's `math` functions of these names take `float64`. The package path and
  signatures differ, and the clause notes the difference.

### SGO-IMP-004: The standard library (v2; amends v1)
- **Package list:** the clause names the seven packages of SGO-STD-001..007 as the library of this
  spec version.
- **Source of truth:** each package's API is exactly its clause. An exported name that is not in a
  clause is refused by a check over `MANIFEST`'s files (`std-api-check.py`, new).
- **Other text:** unchanged.

## Implementation plan (after approval)

1. **Packages:** the S-Go sources under `src/sgostd/` (`errors`, `strings`, `bytes`, `strconv`,
   `utf8`, `sort`, `math`), written from Go's algorithms where they fit S-Go's subset, with no
   package-level variables. Any constant tables are `const` strings (D122).
2. **Tests:**
   - an `sgo test` suite per package (`*_test.sgo`, run by `test-std.py` in CI);
   - a differential harness `sgo/scripts/test-std-diff.py`, which builds a Go host program and an
     S-Go program over the same seeded inputs and compares their outputs line by line (D125).
3. **API check:** `sgo/scripts/std-api-check.py` compares every exported identifier and signature
   in `src/sgostd` with the clauses (D126). It runs in sgo-spec CI.
4. **Conformance:**
   - one or two run cases per package that import it from a user program;
   - negative cases for the D124 panics and overflows, and for a user module importing a
     `std/` name that does not exist (`SGO_E_UNDEFINED`).
5. **Mutation tests:**
   - a stable sort made unstable (caught by the stability case);
   - `DecodeRuneInString` advancing two bytes on invalid input (caught by the `range` agreement
     test);
   - an unexported helper exported (caught by `std-api-check`);
   - a changed error message (caught by the strconv cases).
6. **Freeze at 0.13.0:**
   - add SPEC-STD.md and amend SGO-IMP-004;
   - regenerate the registry, identity constants, MANIFEST, inventory and policy;
   - bump every `sgo.mod` and the module fixtures' `sgo.sum` files;
   - Linux CI evidence in `sgo/evidence/w16/README.md`, then reviewer verification.

## Not in W16

- `Is`/`As`/`Unwrap`/`Join` errors, `io`-style `Reader`/`Writer`, `fmt`, and sorting by interface
  (W17).
- Generic containers and slices helpers (D108, generics).
- Unicode case tables (D122).
- Floating-point math and `strconv.ParseFloat` (floats are denied).
- `bytes.Buffer` and `strings.Reader` (they fit better once `io` interfaces exist).
