# SPEC-STD — The standard library

The packages of `std` (SGO-IMP-004): their exported API and behavior. Derived from `sgo/docs/stdlib-foundations-plan.md` and `sgo/docs/w16-drafts.md` (D103, D108–D110, D121–D126).

### SGO-STD-001 — `std/errors`: Errors
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IMP-004, SGO-TYPE-001, SGO-COMP-004, SGO-ERR-003; D103, D109, D125, D126, D121.
- **grammar_and_types:** The package `std/errors` exports exactly: `func New(text string) error`.
- **preconditions:** None.
- **evaluation_and_value_semantics:** `New(text)` returns an error value whose `Error()` is text, as `sgo.NewError`. Two calls give two values; error values are not comparable (SGO-TYPE-006). `Is`, `As`, `Unwrap`, and `Join` are not provided (D121): they need interfaces and type assertions.
- **ownership_and_lifecycle:** As user code: values returned are owned by the calling task; slices are task-local (SGO-COMP-004).
- **effect_and_capability_requirements:** None: the package holds no capability and has no package-level state (D109).
- **concurrency_and_memory_order:** n/a — ordinary functions of the calling task; no synchronization.
- **allowed_nondeterminism:** None: every result is a function of the arguments.
- **failure_and_partial_progress:** None.
- **cancellation_and_cleanup:** n/a — no cancellation points.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export; every admission diagnostic for calls as for user code.
- **runtime_guards:** None.
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code: the package is S-Go source compiled under the same admission and sharing rules as user code (SGO-IMP-004), not part of the TCB.
- **positive_tests:** fixture:std-errors-new; the `sgo test` suite of `std/errors`; test-std-diff.
- **negative_tests:** fixture:std-undefined-name.
- **implementation_coverage:** `src/sgostd/errors/`.
- **executed_evidence:** W16: its `sgo test` suite (`sgo/scripts/test-std.py`), `sgo/scripts/test-std-diff.py`, `sgo/scripts/std-api-check.py`, conformance (sgo/evidence/w16/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-STD-002 — `std/strings`: Strings
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IMP-004, SGO-TYPE-001, SGO-COMP-004, SGO-ERR-003; D103, D109, D125, D126, D122, D124.
- **grammar_and_types:** The package `std/strings` exports exactly: `func (*Builder) Len() int`; `func (*Builder) Reset()`; `func (*Builder) String() string`; `func (*Builder) WriteByte(c byte) error`; `func (*Builder) WriteRune(r rune) (int, error)`; `func (*Builder) WriteString(s string) (int, error)`; `func Compare(a, b string) int`; `func Contains(s, substr string) bool`; `func ContainsRune(s string, r rune) bool`; `func Count(s, substr string) int`; `func Cut(s, sep string) (string, string, bool)`; `func CutPrefix(s, prefix string) (string, bool)`; `func CutSuffix(s, suffix string) (string, bool)`; `func EqualFoldASCII(s, t string) bool`; `func Fields(s string) []string`; `func HasPrefix(s, prefix string) bool`; `func HasSuffix(s, suffix string) bool`; `func Index(s, substr string) int`; `func IndexByte(s string, c byte) int`; `func IndexRune(s string, r rune) int`; `func Join(elems []string, sep string) string`; `func LastIndex(s, substr string) int`; `func Repeat(s string, count int) string`; `func Replace(s, old, repl string, n int) string`; `func ReplaceAll(s, old, repl string) string`; `func Split(s, sep string) []string`; `func SplitN(s, sep string, n int) []string`; `func ToLowerASCII(s string) string`; `func ToUpperASCII(s string) string`; `func Trim(s, cutset string) string`; `func TrimLeft(s, cutset string) string`; `func TrimPrefix(s, prefix string) string`; `func TrimRight(s, cutset string) string`; `func TrimSpace(s string) string`; `func TrimSuffix(s, suffix string) string`; `type Builder`. `Builder` is a struct type whose zero value is ready to use; it is task-local.
- **preconditions:** For `Repeat`: count ≥ 0.
- **evaluation_and_value_semantics:** Each function has the documented result of Go's `strings` function of the same name, on the same arguments (D125), except: `ToUpperASCII`, `ToLowerASCII`, and `EqualFoldASCII` map only the ASCII letters A-Z and a-z (D122; Go's `ToUpper` and `EqualFold` use Unicode tables and are not provided); `TrimSpace` and `Fields` use Unicode's White_Space property, as Go's do. Invalid UTF-8 is decoded as SGO-VAL-002 (D117). `Builder.String` returns a copy; the write methods return Go's results (a nil error).
- **ownership_and_lifecycle:** As user code: values returned are owned by the calling task; slices are task-local (SGO-COMP-004).
- **effect_and_capability_requirements:** None: the package holds no capability and has no package-level state (D109).
- **concurrency_and_memory_order:** n/a — ordinary functions of the calling task; no synchronization.
- **allowed_nondeterminism:** None: every result is a function of the arguments.
- **failure_and_partial_progress:** `Repeat` with a negative count is a `panic` with the message `strings.Repeat: negative count` (D124); a result whose length overflows int is `IntegerOverflow`; failing to allocate is `ResourceExhausted`.
- **cancellation_and_cleanup:** n/a — no cancellation points.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export; every admission diagnostic for calls as for user code.
- **runtime_guards:** `IntegerOverflow` (checked arithmetic).
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code: the package is S-Go source compiled under the same admission and sharing rules as user code (SGO-IMP-004), not part of the TCB.
- **positive_tests:** fixture:std-strings-basic; the `sgo test` suite of `std/strings`; test-std-diff.
- **negative_tests:** fixture:std-strings-repeat-negative.
- **implementation_coverage:** `src/sgostd/strings/`.
- **executed_evidence:** W16: its `sgo test` suite (`sgo/scripts/test-std.py`), `sgo/scripts/test-std-diff.py`, `sgo/scripts/std-api-check.py`, conformance (sgo/evidence/w16/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-STD-003 — `std/bytes`: Byte slices
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IMP-004, SGO-TYPE-001, SGO-COMP-004, SGO-ERR-003; D103, D109, D125, D126, D124.
- **grammar_and_types:** The package `std/bytes` exports exactly: `func Clone(b []byte) []byte`; `func Compare(a, b []byte) int`; `func Contains(b, subslice []byte) bool`; `func Count(s, sep []byte) int`; `func Equal(a, b []byte) bool`; `func HasPrefix(s, prefix []byte) bool`; `func HasSuffix(s, suffix []byte) bool`; `func Index(s, sep []byte) int`; `func IndexByte(b []byte, c byte) int`; `func Join(s [][]byte, sep []byte) []byte`; `func LastIndex(s, sep []byte) int`; `func Split(s, sep []byte) [][]byte`; `func TrimSpace(s []byte) []byte`.
- **preconditions:** None.
- **evaluation_and_value_semantics:** Each function has the documented result of Go's `bytes` function of the same name (D125). Results that are slices of an argument share its array, with the lengths and capacities Go's `bytes` gives them (`Split` pieces have their own length as capacity; `TrimSpace` returns a subslice, or nil if nothing remains); `Clone` and `Join` return new arrays (`Clone(nil)` is nil; `Join` of no elements is an empty non-nil slice).
- **ownership_and_lifecycle:** As user code: values returned are owned by the calling task; slices are task-local (SGO-COMP-004).
- **effect_and_capability_requirements:** None: the package holds no capability and has no package-level state (D109).
- **concurrency_and_memory_order:** n/a — ordinary functions of the calling task; no synchronization.
- **allowed_nondeterminism:** None: every result is a function of the arguments.
- **failure_and_partial_progress:** Failing to allocate is `ResourceExhausted`.
- **cancellation_and_cleanup:** n/a — no cancellation points.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export; every admission diagnostic for calls as for user code.
- **runtime_guards:** None.
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code: the package is S-Go source compiled under the same admission and sharing rules as user code (SGO-IMP-004), not part of the TCB.
- **positive_tests:** fixture:std-bytes-basic; the `sgo test` suite of `std/bytes`; test-std-diff.
- **negative_tests:** fixture:std-undefined-name.
- **implementation_coverage:** `src/sgostd/bytes/`.
- **executed_evidence:** W16: its `sgo test` suite (`sgo/scripts/test-std.py`), `sgo/scripts/test-std-diff.py`, `sgo/scripts/std-api-check.py`, conformance (sgo/evidence/w16/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-STD-004 — `std/strconv`: Integer, boolean, and quoted text
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IMP-004, SGO-TYPE-001, SGO-COMP-004, SGO-ERR-003; D103, D109, D125, D126, D121, D124.
- **grammar_and_types:** The package `std/strconv` exports exactly: `func Atoi(s string) (int, error)`; `func FormatBool(b bool) string`; `func FormatInt(v int, base int) string`; `func Itoa(v int) string`; `func ParseBool(s string) (bool, error)`; `func ParseInt(s string, base int) (int, error)`; `func QuoteToASCII(s string) string`; `func Unquote(s string) (string, error)`.
- **preconditions:** For `FormatInt`: 2 ≤ base ≤ 36.
- **evaluation_and_value_semantics:** Each function has the documented result of Go's `strconv` function of the same name (D125), with these differences: `ParseInt` has no bitSize (int is the only integer width; its range is that of Go's ParseInt with bitSize 64, including returning the nearest bound with a range error); `Quote` is not provided (it needs Unicode printability tables, D122); `QuoteToASCII` and `Unquote` are Go's. Errors have Go's messages, `strconv.<Func>: parsing <input>: invalid syntax`, `... value out of range`, or `... invalid base <b>`, except that the input is quoted by `QuoteToASCII` rather than Go's `Quote` (they differ only for printable non-ASCII text); `Unquote` fails with `invalid syntax`.
- **ownership_and_lifecycle:** As user code: values returned are owned by the calling task; slices are task-local (SGO-COMP-004).
- **effect_and_capability_requirements:** None: the package holds no capability and has no package-level state (D109).
- **concurrency_and_memory_order:** n/a — ordinary functions of the calling task; no synchronization.
- **allowed_nondeterminism:** None: every result is a function of the arguments.
- **failure_and_partial_progress:** `FormatInt` with a base outside 2..36 is a `panic` with the message `strconv: illegal AppendInt/FormatInt base` (the message Go's `FormatInt` panics with).
- **cancellation_and_cleanup:** n/a — no cancellation points.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export; every admission diagnostic for calls as for user code.
- **runtime_guards:** None.
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code: the package is S-Go source compiled under the same admission and sharing rules as user code (SGO-IMP-004), not part of the TCB.
- **positive_tests:** fixture:std-import, fixture:std-strconv-parse; the `sgo test` suite of `std/strconv`; test-std-diff.
- **negative_tests:** fixture:std-strconv-parse.
- **implementation_coverage:** `src/sgostd/strconv/`.
- **executed_evidence:** W16: its `sgo test` suite (`sgo/scripts/test-std.py`), `sgo/scripts/test-std-diff.py`, `sgo/scripts/std-api-check.py`, conformance (sgo/evidence/w16/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-STD-005 — `std/utf8`: UTF-8
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IMP-004, SGO-TYPE-001, SGO-COMP-004, SGO-ERR-003; D103, D109, D125, D126, D117.
- **grammar_and_types:** The package `std/utf8` exports exactly: `const MaxRune`; `const RuneError`; `const RuneSelf`; `const UTFMax`; `func AppendRune(p []byte, r rune) []byte`; `func DecodeLastRune(p []byte) (rune, int)`; `func DecodeLastRuneInString(s string) (rune, int)`; `func DecodeRune(p []byte) (rune, int)`; `func DecodeRuneInString(s string) (rune, int)`; `func EncodeRune(p []byte, r rune) int`; `func FullRune(p []byte) bool`; `func FullRuneInString(s string) bool`; `func RuneCount(p []byte) int`; `func RuneCountInString(s string) int`; `func RuneLen(r rune) int`; `func RuneStart(b byte) bool`; `func Valid(p []byte) bool`; `func ValidRune(r rune) bool`; `func ValidString(s string) bool`. The constants are untyped (Go's `unicode/utf8` declares them so too): `RuneError` is the rune constant U+FFFD, `RuneSelf` is 0x80, `MaxRune` is the rune constant U+10FFFF, and `UTFMax` is 4.
- **preconditions:** For `EncodeRune`: p is long enough for the encoding (`BoundsViolation` otherwise, as Go panics).
- **evaluation_and_value_semantics:** Each function has the documented result of Go's `unicode/utf8` function of the same name (D125). Decoding follows D117: `DecodeRuneInString` and `range` over a string agree on every input.
- **ownership_and_lifecycle:** As user code: values returned are owned by the calling task; slices are task-local (SGO-COMP-004).
- **effect_and_capability_requirements:** None: the package holds no capability and has no package-level state (D109).
- **concurrency_and_memory_order:** n/a — ordinary functions of the calling task; no synchronization.
- **allowed_nondeterminism:** None: every result is a function of the arguments.
- **failure_and_partial_progress:** None (a too-short p in EncodeRune is `BoundsViolation`).
- **cancellation_and_cleanup:** n/a — no cancellation points.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export; every admission diagnostic for calls as for user code.
- **runtime_guards:** `BoundsViolation`.
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code: the package is S-Go source compiled under the same admission and sharing rules as user code (SGO-IMP-004), not part of the TCB.
- **positive_tests:** fixture:std-utf8-decode; the `sgo test` suite of `std/utf8`; test-std-diff.
- **negative_tests:** fixture:std-undefined-name.
- **implementation_coverage:** `src/sgostd/utf8/`.
- **executed_evidence:** W16: its `sgo test` suite (`sgo/scripts/test-std.py`), `sgo/scripts/test-std-diff.py`, `sgo/scripts/std-api-check.py`, conformance (sgo/evidence/w16/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-STD-006 — `std/sort`: Sorting and searching
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IMP-004, SGO-TYPE-001, SGO-COMP-004, SGO-ERR-003; D103, D109, D125, D126, D123, D124.
- **grammar_and_types:** The package `std/sort` exports exactly: `func Bytes(x []byte)`; `func Ints(x []int)`; `func IntsAreSorted(x []int) bool`; `func Perm(n int, less func(i, j int) bool) []int`; `func Runes(x []rune)`; `func Search(n int, f func(int) bool) int`; `func SearchInts(a []int, x int) int`; `func SearchStrings(a []string, x string) int`; `func Strings(x []string)`; `func StringsAreSorted(x []string) bool`.
- **preconditions:** For `Perm`: n ≥ 0; less is a strict weak order on 0..n-1 (if it is not, the result is still a permutation). A function value that captures task-local data cannot be passed as an argument (SGO-COMP-004, D94), so less and f compute from their arguments or from shareable data.
- **evaluation_and_value_semantics:** Every sort is stable and uses one fixed merge sort (D123), so results are the same on every platform and run: `Perm(n, less)` returns the indices 0..n-1 in stable sorted order; `Ints`, `Strings`, `Runes`, and `Bytes` sort in place in increasing order (strings by bytes); `Search` returns the smallest i in [0, n) with f(i) true, or n, as Go's; `SearchInts` and `SearchStrings` return the insertion index of x in a sorted slice.
- **ownership_and_lifecycle:** As user code: values returned are owned by the calling task; slices are task-local (SGO-COMP-004).
- **effect_and_capability_requirements:** None: the package holds no capability and has no package-level state (D109).
- **concurrency_and_memory_order:** n/a — ordinary functions of the calling task; no synchronization.
- **allowed_nondeterminism:** None: every result is a function of the arguments.
- **failure_and_partial_progress:** `Perm` with n < 0 is a `panic` with the message `sort.Perm: negative length`; a failure of less or f propagates.
- **cancellation_and_cleanup:** n/a — no cancellation points.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export; every admission diagnostic for calls as for user code.
- **runtime_guards:** None.
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code: the package is S-Go source compiled under the same admission and sharing rules as user code (SGO-IMP-004), not part of the TCB.
- **positive_tests:** fixture:std-sort-stable; the `sgo test` suite of `std/sort`; test-std-diff.
- **negative_tests:** fixture:std-sort-perm-negative, fixture:std-sort-capture-denied.
- **implementation_coverage:** `src/sgostd/sort/`.
- **executed_evidence:** W16: its `sgo test` suite (`sgo/scripts/test-std.py`), `sgo/scripts/test-std-diff.py`, `sgo/scripts/std-api-check.py`, conformance (sgo/evidence/w16/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-STD-007 — `std/math`: Integer math
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IMP-004, SGO-TYPE-001, SGO-COMP-004, SGO-ERR-003; D103, D109, D125, D126, D124.
- **grammar_and_types:** The package `std/math` exports exactly: `const MaxInt`; `const MinInt`; `func Abs(x int) int`; `func Max(a, b int) int`; `func Min(a, b int) int`; `func Pow(x, n int) int`. `MaxInt` and `MinInt` are the untyped constants 2^63-1 and -2^63.
- **preconditions:** For `Pow`: n ≥ 0.
- **evaluation_and_value_semantics:** Integer only (floating point stays denied, SGO-FLOAT-001): `Abs` is the absolute value, `Min` and `Max` the smaller and larger argument, `Pow(x, n)` x to the power n (`Pow(x, 0)` is 1). Go's `math` functions of these names take float64; these take int.
- **ownership_and_lifecycle:** As user code: values returned are owned by the calling task; slices are task-local (SGO-COMP-004).
- **effect_and_capability_requirements:** None: the package holds no capability and has no package-level state (D109).
- **concurrency_and_memory_order:** n/a — ordinary functions of the calling task; no synchronization.
- **allowed_nondeterminism:** None: every result is a function of the arguments.
- **failure_and_partial_progress:** `Abs(MinInt)` and a `Pow` outside the range of int are `IntegerOverflow` (checked arithmetic, D124); `Pow` with n < 0 is a `panic` with the message `math.Pow: negative exponent`.
- **cancellation_and_cleanup:** n/a — no cancellation points.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export; every admission diagnostic for calls as for user code.
- **runtime_guards:** `IntegerOverflow`.
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code: the package is S-Go source compiled under the same admission and sharing rules as user code (SGO-IMP-004), not part of the TCB.
- **positive_tests:** fixture:std-math-basic; the `sgo test` suite of `std/math`; test-std-diff.
- **negative_tests:** fixture:std-math-abs-overflow, fixture:std-math-pow-negative.
- **implementation_coverage:** `src/sgostd/math/`.
- **executed_evidence:** W16: its `sgo test` suite (`sgo/scripts/test-std.py`), `sgo/scripts/test-std-diff.py`, `sgo/scripts/std-api-check.py`, conformance (sgo/evidence/w16/README.md).
- **supported_execution_configurations:** production, lab.
