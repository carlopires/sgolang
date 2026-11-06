# SPEC-STD — The standard library

The packages of `std` (SGO-IMP-004): their exported API and behavior. Derived from `sgo/docs/stdlib-foundations-plan.md` and `sgo/docs/w16-drafts.md` (D103, D108–D110, D121–D126).

### SGO-STD-001 — `std/errors`: Errors
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-IMP-004, SGO-TYPE-001, SGO-COMP-004, SGO-ERR-003; D103, D109, D125, D126, D121.
- **grammar_and_types:** The package `std/errors` exports exactly: `func New(text string) error`; `func Unwrap(err error) error` (spec 0.14.0, D133).
- **preconditions:** None.
- **evaluation_and_value_semantics:** `New(text)` returns an error value whose `Error()` is text, as `sgo.NewError`. Two calls give two values; error values are not comparable (SGO-TYPE-006). `Unwrap(err)` returns the result of err's `Unwrap() error` method, or nil when err is nil or has no such method (spec 0.14.0, D133). `Is`, `As`, and `Join` are not provided (D121, D133): `Is` needs error comparison, which SGO-TYPE-006 denies, and `As` needs generics or reflection.
- **ownership_and_lifecycle:** As user code: values returned are owned by the calling task; slices are task-local (SGO-COMP-004).
- **effect_and_capability_requirements:** None: the package holds no capability and has no package-level state (D109).
- **concurrency_and_memory_order:** n/a — ordinary functions of the calling task; no synchronization.
- **allowed_nondeterminism:** None: every result is a function of the arguments.
- **failure_and_partial_progress:** None.
- **cancellation_and_cleanup:** n/a — no cancellation points.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export; every admission diagnostic for calls as for user code.
- **runtime_guards:** None.
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code: the package is S-Go source compiled under the same admission and sharing rules as user code (SGO-IMP-004), not part of the TCB.
- **positive_tests:** fixture:std-errors-new, fixture:iface-error-field; the `sgo test` suite of `std/errors`; test-std-diff.
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
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-IMP-004, SGO-TYPE-001, SGO-COMP-004, SGO-ERR-003, SGO-FLOAT-002; D103, D109, D125, D126, D121, D124, D193.
- **grammar_and_types:** The package `std/strconv` exports exactly: `func Atoi(s string) (int, error)`; `func FormatBool(b bool) string`; `func FormatFloat(f float64, fmt byte, prec int) string`; `func FormatInt(v int, base int) string`; `func Itoa(v int) string`; `func ParseBool(s string) (bool, error)`; `func ParseFloat(s string) (float64, error)`; `func ParseInt(s string, base int) (int, error)`; `func QuoteToASCII(s string) string`; `func Unquote(s string) (string, error)`.
- **preconditions:** For `FormatInt`: 2 ≤ base ≤ 36.
- **evaluation_and_value_semantics:** Each function has the documented result of Go's `strconv` function of the same name (D125), with these differences: `ParseInt` has no bitSize (int is the only integer width; its range is that of Go's ParseInt with bitSize 64, including returning the nearest bound with a range error); `ParseFloat` and `FormatFloat` (spec 0.20.0) have no bitSize (`float64` is the only float type): `ParseFloat` accepts a decimal number (digits with an optional `.` and exponent `e`/`E`), a hexadecimal number (`0x`, hex digits with an optional `.`, and a mandatory exponent `p`/`P`), an optional sign, `_` only between digits or after a base prefix, and `inf`, `infinity`, or `nan` in any case; it returns the nearest `float64` to the exact value (ties to even), ±Inf with `value out of range` if that overflows, and 0 with `invalid syntax` for any other text. `FormatFloat(f, fmt, prec)` formats with `fmt` one of `e` `E` (`-d.dddde±dd`), `f` (`-ddd.dddd`), `g` `G` (`e` form for an exponent below -4 or at least the precision, `f` form otherwise, trailing zeros removed), `b` (`-ddddp±ddd`, the integer mantissa and binary exponent), `x` `X` (`-0x1.hhhhp±dd`); `prec` is the digits after the point (`e` `f` `x`) or the significant digits (`g`), and -1 means the fewest digits that `ParseFloat` reads back as the same value; the decimal digits are correctly rounded (ties to even on the exact binary value); an unknown `fmt` gives `%` followed by it; infinities are `+Inf` and `-Inf` and NaN is `NaN`. Both use exact multiprecision decimal arithmetic. `Quote` is not provided (it needs Unicode printability tables, D122); `QuoteToASCII` and `Unquote` are Go's. Errors have Go's messages, `strconv.<Func>: parsing <input>: invalid syntax`, `... value out of range`, or `... invalid base <b>`, except that the input is quoted by `QuoteToASCII` rather than Go's `Quote` (they differ only for printable non-ASCII text); `Unquote` fails with `invalid syntax`.
- **ownership_and_lifecycle:** As user code: values returned are owned by the calling task; slices are task-local (SGO-COMP-004).
- **effect_and_capability_requirements:** None: the package holds no capability and has no package-level state (D109).
- **concurrency_and_memory_order:** n/a — ordinary functions of the calling task; no synchronization.
- **allowed_nondeterminism:** None: every result is a function of the arguments.
- **failure_and_partial_progress:** `FormatInt` with a base outside 2..36 is a `panic` with the message `strconv: illegal AppendInt/FormatInt base` (the message Go's `FormatInt` panics with).
- **cancellation_and_cleanup:** n/a — no cancellation points.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export; every admission diagnostic for calls as for user code.
- **runtime_guards:** None.
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code: the package is S-Go source compiled under the same admission and sharing rules as user code (SGO-IMP-004), not part of the TCB.
- **positive_tests:** fixture:std-import, fixture:std-strconv-parse, fixture:std-strconv-float; the `sgo test` suite of `std/strconv`; test-std-diff; W25 differential runs against Go's strconv: 45600 FormatFloat and 4073 ParseFloat cases identical (sgo/evidence/w25/README.md).
- **negative_tests:** fixture:std-strconv-parse, fixture:std-strconv-float (out of range and invalid syntax).
- **implementation_coverage:** `src/sgostd/strconv/` (W25: `decimal.sgo`, `atof.sgo`, `ftoa.sgo`).
- **executed_evidence:** W16: its `sgo test` suite (`sgo/scripts/test-std.py`), `sgo/scripts/test-std-diff.py`, `sgo/scripts/std-api-check.py`, conformance (sgo/evidence/w16/README.md); W25: conformance and the differential tests against Go (sgo/evidence/w25/README.md).
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
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-IMP-004, SGO-TYPE-001, SGO-COMP-004, SGO-ERR-003; D103, D109, D125, D126, D123, D124.
- **grammar_and_types:** The package `std/sort` exports exactly: `func Bytes(x []byte)`; `func Ints(x []int)`; `func IntsAreSorted(x []int) bool`; `type Interface` (spec 0.14.0, D133); `func Perm(n int, less func(i, j int) bool) []int`; `func Runes(x []rune)`; `func Search(n int, f func(int) bool) int`; `func SearchInts(a []int, x int) int`; `func SearchStrings(a []string, x string) int`; `func Sort(data Interface)` (spec 0.14.0, D133); `func Strings(x []string)`; `func StringsAreSorted(x []string) bool`.
- **preconditions:** For `Perm`: n ≥ 0; less is a strict weak order on 0..n-1 (if it is not, the result is still a permutation). A function value that captures task-local data cannot be passed as an argument (SGO-COMP-004, D94), so less and f compute from their arguments or from shareable data.
- **evaluation_and_value_semantics:** Every sort is stable and uses one fixed merge sort (D123), so results are the same on every platform and run: `Perm(n, less)` returns the indices 0..n-1 in stable sorted order; `Sort(data)` sorts data in place in increasing order of `data.Less`, stably, using only `Len`, `Less`, and `Swap` (insertion-sorted blocks merged in place, O(n·log n) calls of Less and O(n·log² n) calls of Swap; spec 0.14.0, D133); `Ints`, `Strings`, `Runes`, and `Bytes` sort in place in increasing order (strings by bytes); `Search` returns the smallest i in [0, n) with f(i) true, or n, as Go's; `SearchInts` and `SearchStrings` return the insertion index of x in a sorted slice.
- **ownership_and_lifecycle:** As user code: values returned are owned by the calling task; slices are task-local (SGO-COMP-004).
- **effect_and_capability_requirements:** None: the package holds no capability and has no package-level state (D109).
- **concurrency_and_memory_order:** n/a — ordinary functions of the calling task; no synchronization.
- **allowed_nondeterminism:** None: every result is a function of the arguments.
- **failure_and_partial_progress:** `Perm` with n < 0 is a `panic` with the message `sort.Perm: negative length`; a failure of less or f propagates.
- **cancellation_and_cleanup:** n/a — no cancellation points.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export; every admission diagnostic for calls as for user code.
- **runtime_guards:** None.
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code: the package is S-Go source compiled under the same admission and sharing rules as user code (SGO-IMP-004), not part of the TCB.
- **positive_tests:** fixture:std-sort-stable, fixture:std-sort-interface; the `sgo test` suite of `std/sort`; test-std-diff.
- **negative_tests:** fixture:std-sort-perm-negative, fixture:std-sort-capture-denied.
- **implementation_coverage:** `src/sgostd/sort/`.
- **executed_evidence:** W16: its `sgo test` suite (`sgo/scripts/test-std.py`), `sgo/scripts/test-std-diff.py`, `sgo/scripts/std-api-check.py`, conformance (sgo/evidence/w16/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-STD-007 — `std/math`: Floating-point and integer math
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-IMP-004, SGO-TYPE-001, SGO-COMP-004, SGO-ERR-003, SGO-FLOAT-002, SGO-FLOAT-003; D103, D109, D125, D126, D124, D188, D191, D192.
- **grammar_and_types:** The package `std/math` exports exactly: `const E`; `const Ln10`; `const Ln2`; `const Log10E`; `const Log2E`; `const MaxFloat64`; `const MaxInt`; `const MinInt`; `const Phi`; `const Pi`; `const SmallestNonzeroFloat64`; `const Sqrt2`; `const SqrtE`; `const SqrtPhi`; `const SqrtPi`; `func Abs(x float64) float64`; `func Acos(x float64) float64`; `func Acosh(x float64) float64`; `func Asin(x float64) float64`; `func Asinh(x float64) float64`; `func Atan(x float64) float64`; `func Atan2(y, x float64) float64`; `func Atanh(x float64) float64`; `func Cbrt(x float64) float64`; `func Ceil(x float64) float64`; `func Copysign(f, sign float64) float64`; `func Cos(x float64) float64`; `func Cosh(x float64) float64`; `func Exp(x float64) float64`; `func Exp2(x float64) float64`; `func Expm1(x float64) float64`; `func FMA(x, y, z float64) float64`; `func Float64bits(f float64) int`; `func Float64frombits(b int) float64`; `func Floor(x float64) float64`; `func Frexp(f float64) (float64, int)`; `func Hypot(p, q float64) float64`; `func Inf(sign int) float64`; `func IntAbs(x int) int`; `func IntMax(a, b int) int`; `func IntMin(a, b int) int`; `func IntPow(x, n int) int`; `func IsInf(f float64, sign int) bool`; `func Ldexp(frac float64, e2 int) float64`; `func Log(x float64) float64`; `func Log10(x float64) float64`; `func Log1p(x float64) float64`; `func Log2(x float64) float64`; `func Max(x, y float64) float64`; `func Min(x, y float64) float64`; `func Mod(x, y float64) float64`; `func Modf(f float64) (float64, float64)`; `func NaN() float64`; `func Nextafter(x, y float64) float64`; `func Pow(x, y float64) float64`; `func Round(x float64) float64`; `func RoundToEven(x float64) float64`; `func Signbit(x float64) bool`; `func Sin(x float64) float64`; `func Sincos(x float64) (sn, cs float64)`; `func Sinh(x float64) float64`; `func Sqrt(x float64) float64`; `func Tan(x float64) float64`; `func Tanh(x float64) float64`; `func Trunc(x float64) float64`. `MaxInt` and `MinInt` are the untyped integer constants 2^63-1 and -2^63; `MaxFloat64` is (2-2^-52)·2^1023 and `SmallestNonzeroFloat64` 2^-1074; `E`, `Pi`, `Phi`, `Sqrt2`, `SqrtE`, `SqrtPi`, `SqrtPhi`, `Ln2`, `Log2E`, `Ln10`, `Log10E` are untyped floating-point constants with at least 60 significant decimal digits (e, π, φ, √2, √e, √π, √φ, ln 2, 1/ln 2, ln 10, 1/ln 10). In spec 0.19 and earlier, `Abs`, `Max`, `Min`, and `Pow` took `int`: those are now `IntAbs`, `IntMax`, `IntMin`, `IntPow` (D191).
- **preconditions:** For `IntPow`: n ≥ 0. None for the float functions: every argument, including NaN and the infinities, has a defined result.
- **evaluation_and_value_semantics:** Integer functions: `IntAbs` is the absolute value, `IntMin` and `IntMax` the smaller and larger argument, `IntPow(x, n)` x to the power n (`IntPow(x, 0)` is 1). Exact float functions, whose result is the correctly rounded value: `Abs`, `Copysign`, `Signbit` (true for a negative number and for -0; false for NaN), `Floor`, `Ceil`, `Trunc`, `Round` (half away from zero), `RoundToEven`, `Mod` (the remainder of x/y with the sign of x), `Sqrt`, `FMA` (x·y+z with one rounding: the only fused operation, SGO-FLOAT-002), `Min` and `Max` (`Max` is +Inf if either argument is +Inf and `Min` is -Inf if either is -Inf, even with a NaN argument; otherwise NaN if either argument is NaN; -0 is smaller than +0), `Inf`, `NaN`, `IsNaN`, `IsInf`, `Nextafter`, `Frexp` (a fraction in [0.5, 1) and an exponent, with ±0, ±Inf, and NaN returned unchanged with exponent 0), `Ldexp` (frac·2^e2, rounded once), `Modf` (integer and fractional parts with the sign of the argument), and `Float64bits`/`Float64frombits` (the IEEE 754 representation as an int in two's complement; NaN gives 0x7FF8000000000000). Transcendental functions (`Exp`, `Exp2`, `Expm1`, `Log`, `Log2`, `Log10`, `Log1p`, `Pow`, `Sin`, `Cos`, `Tan`, `Sincos`, `Asin`, `Acos`, `Atan`, `Atan2`, `Sinh`, `Cosh`, `Tanh`, `Asinh`, `Acosh`, `Atanh`, `Hypot`, `Cbrt`) are computed in S-Go source by the portable algorithms of Go 1.26's `src/math` (Cephes and FreeBSD msun polynomials; Payne-Hanek reduction for trigonometric arguments of 2^29 and above), each operation rounded once and never fused: the result is within 1 ulp of the exact value and is the same bit pattern on every target, but is not always the correctly rounded one. Special values of the transcendental functions: a NaN argument gives NaN except `Pow(x, ±0)` = 1, `Pow(1, y)` = 1, and `Hypot(±Inf, y)` = +Inf; an argument outside a function's domain gives NaN (`Log(-1)`, `Sqrt(-1)`, `Asin(2)`, `Acosh(0)`); a pole gives a signed infinity (`Log(0)` = -Inf, `Pow(0, -1)` = +Inf, `Atanh(1)` = +Inf); an overflow gives a signed infinity (`Exp(1000)` = +Inf) and an underflow a subnormal or a signed zero. The exact value of every result is pinned by the differential tests (positive_tests).
- **ownership_and_lifecycle:** As user code: values returned are owned by the calling task; slices are task-local (SGO-COMP-004).
- **effect_and_capability_requirements:** None: the package holds no capability and has no package-level state (D109).
- **concurrency_and_memory_order:** n/a — ordinary functions of the calling task; no synchronization.
- **allowed_nondeterminism:** None: every result is a function of the arguments.
- **failure_and_partial_progress:** `IntAbs(MinInt)` and an `IntPow` outside the range of int are `IntegerOverflow` (checked arithmetic, D124); `IntPow` with n < 0 is a `panic` with the message `math.IntPow: negative exponent`. No float function fails.
- **cancellation_and_cleanup:** n/a — no cancellation points.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export; every admission diagnostic for calls as for user code.
- **runtime_guards:** `IntegerOverflow` (integer functions only).
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code: the package is S-Go source compiled under the same admission and sharing rules as user code (SGO-IMP-004), not part of the TCB.
- **positive_tests:** fixture:std-math-basic, fixture:std-math-float, fixture:float-special; the `sgo test` suite of `std/math`; `sgo/scripts/test-std-diff.py`; W25 differential runs against Go's portable math (amd64 v1, no fusion): 1433 arguments × 25 transcendental results and 419 × 16 exact functions bit-identical (sgo/evidence/w25/README.md).
- **negative_tests:** fixture:std-math-abs-overflow, fixture:std-math-pow-negative.
- **implementation_coverage:** `src/sgostd/math/` (W16; W25: `float.sgo`, `bits.sgo`, `fma.sgo`, and the `go_*.sgo` ports of Go's portable transcendentals, including `go_trig_reduce.sgo`).
- **executed_evidence:** W16: its `sgo test` suite (`sgo/scripts/test-std.py`), `sgo/scripts/test-std-diff.py`, `sgo/scripts/std-api-check.py`, conformance (sgo/evidence/w16/README.md); W25: conformance and the differential tests against Go (sgo/evidence/w25/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-STD-008 — `std/io`: Stream interfaces
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IMP-004, SGO-RES-006, SGO-FILE-001, SGO-NET-001, SGO-PROC-003; D134, D176.
- **grammar_and_types:** The package `std/io` exports exactly: `func WriteString(w Writer, s string) error`; `type LineReadWriter`; `type LineReader`; `type ReadWriter`; `type Reader`; `type Writer`. `Reader` is `sgo.Resource` with `Read(max int) (string, error)`; `LineReader` is `sgo.Resource` with `ReadLine(max int) (string, error)`; `Writer` is `sgo.Resource` with `Write(s string) (int, error)`; `ReadWriter` embeds `Reader` and `Writer`; `LineReadWriter` embeds `LineReader` and `Writer`. All are resource interfaces (SGO-RES-006), declared in S-Go source by embedding `sgo.Resource` exactly as user code may.
- **preconditions:** As the methods of the dynamic value.
- **evaluation_and_value_semantics:** `*sgo.File` and `*sgo.Conn` have the methods of `ReadWriter` and `LineReadWriter`; standard output (`*sgo.Writer`) has `Write` and converts to `Writer` as an untied value. `WriteString(w, s)` calls `w.Write(s)` once and returns its error, or an error `io.WriteString: short write` if fewer than `len(s)` bytes were written without an error.
- **ownership_and_lifecycle:** As SGO-RES-006: a value is tied as what it holds.
- **effect_and_capability_requirements:** None of its own: the package holds no capability and has no package-level state (D109).
- **concurrency_and_memory_order:** n/a: values are task-local (SGO-RES-006).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** As the dynamic value's methods.
- **cancellation_and_cleanup:** As the dynamic value's methods.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export; the checks of SGO-RES-006.
- **runtime_guards:** As SGO-RES-006.
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code: the package is S-Go source compiled under the same admission and sharing rules as user code (SGO-IMP-004), not part of the TCB.
- **positive_tests:** fixture:std-io-streams.
- **negative_tests:** fixture:std-io-escape.
- **implementation_coverage:** `src/sgostd/io/`.
- **executed_evidence:** conformance fixtures (darwin/arm64); Linux at the W23 merge.
- **supported_execution_configurations:** production, lab.


### SGO-STD-009 — `std/fmt`: Formatted text
- **version:** 2
- **status:** frozen
- **reference_closure:** SGO-IMP-004, SGO-STD-001, SGO-STD-004, SGO-STD-008, SGO-IFACE-004, SGO-TYPE-003, SGO-FLOAT-002; D125, D183, D193.
- **grammar_and_types:** The package `std/fmt` exports exactly: `func Errorf(format string, args []any) error`; `func Fprintf(w io.Writer, format string, args []any) error`; `func Sprintf(format string, args []any) string`; `func Wrapf(err error, format string, args []any) error`; `type Stringer`. `Stringer` is an interface with `String() string`. The arguments are an explicit `[]any`, since variadic functions are denied (SGO-TYPE-003).
- **preconditions:** None.
- **evaluation_and_value_semantics:** A verb is `%`, then any of the flags `-` (pad on the right), `+` (print a number's sign also when it is positive; no effect with `%v`), and `0` (pad with zeros, on the left only, after the sign of a number; ignored with `-`), then an optional decimal width counted in runes, then an optional precision `.` followed by decimal digits (none means 0), then the verb letter; there are no other flags (spec 0.20.0 adds `+`, the precision, and the float verbs). By type switch, never reflection: `string` takes `s v q x`; `int`, `byte`, `rune` take `d v x c q`; `float64` takes `v f F e E g G b x X` (spec 0.20.0); `bool` takes `t v`; a value with an `Error() string` method, then one with `String() string`, takes `s v q x` on that text; `nil` takes `v` (`<nil>`). `%q` of text quotes as `strconv.QuoteToASCII` (Go's `%+q`), and of an integer is a single-quoted ASCII rune literal (`'a'`, `'\u00e9'`, `'\ufffd'` for an invalid rune); `%x` of text is the lower-case hex of its bytes. A precision is the maximum runes of text for `s v q` (applied before quoting) and the maximum bytes for `x`; for `d v x` of an integer the minimum digits, padded with zeros (precision 0 prints nothing for the value 0), and then the `0` flag does not apply; without a precision, the `0` flag pads an integer's digits to the width after its sign; `c` and `q` of an integer and booleans ignore it. A `float64` is formatted by `strconv.FormatFloat`: `%v` as `g` with the shortest precision, `%b %g %G %x %X` with the shortest precision unless one is given, `%e %E %f` with precision 6 unless one is given, and `%F` as `%f`; `+Inf` keeps its `+` and NaN prints as `NaN` (`+NaN` with the `+` flag); infinities and NaN are padded with spaces even with the `0` flag. A format error is reported in the text as Go's fmt does: `%!d(string=x)` for a verb the type does not take (the value formatted as `%v` with the verb's flags and width), `%!d(MISSING)`, `%!(EXTRA int=1, ...)`, `%!(NOVERB)`; a value of another type prints its type as `?`. `Fprintf` writes the text with `io.WriteString`. `Errorf` returns an error with the text; `Wrapf(err, format, args)` returns an error whose text is the formatted text, `": "`, and `err.Error()`, and which wraps err (`errors.Unwrap`). Both are value types, so they may cross tasks.
- **ownership_and_lifecycle:** None: the results are values.
- **effect_and_capability_requirements:** None of its own; `Fprintf` writes through w.
- **concurrency_and_memory_order:** n/a: the package has no state (D109).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** `Fprintf` returns w's error; the formatting functions do not fail.
- **cancellation_and_cleanup:** As w's methods.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export. An argument in `[]any` never becomes an `error` again (SGO-IFACE-004), so there is no `%w` verb: `Wrapf` takes the wrapped error as a parameter.
- **runtime_guards:** As SGO-ERR-003.
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code: S-Go source under the same rules as user code (SGO-IMP-004).
- **positive_tests:** fixture:std-fmt-basic, fixture:std-fmt-float, fixture:float-usertype; `sgo/scripts/test-std-diff.py` (against Go's fmt, with float arguments, `+`, and precisions).
- **negative_tests:** fixture:std-fmt-basic, fixture:std-fmt-float (format errors in the text).
- **implementation_coverage:** `src/sgostd/fmt/`.
- **executed_evidence:** conformance fixture std-fmt-basic (darwin/arm64, linux/amd64, linux/arm64); `sgo/scripts/test-std-diff.py` (seeds 1 and 7, identical to Go's); `src/sgostd` package tests; `sgo/evidence/w24/README.md`; W25: `sgo/scripts/test-std-diff.py` seeds 1-6, 11698 checks identical to Go's (sgo/evidence/w25/README.md).
- **supported_execution_configurations:** production, lab.

### SGO-STD-010 — `std/time`: Durations
- **version:** 1
- **status:** frozen
- **reference_closure:** SGO-IMP-004, SGO-STD-001, SGO-STD-004, SGO-UTYPE-001, SGO-AUTH-002; D125, D183.
- **grammar_and_types:** The package `std/time` exports exactly: `const Hour`; `const Microsecond`; `const Millisecond`; `const Minute`; `const Nanosecond`; `const Second`; `func (Duration) Milliseconds() int`; `func (Duration) Nanoseconds() int`; `func (Duration) String() string`; `func ParseDuration(s string) (Duration, error)`; `type Duration`. `Duration` is `int`, nanoseconds; the constants are `Duration`.
- **preconditions:** None.
- **evaluation_and_value_semantics:** `ParseDuration` accepts an optional sign and a sequence of decimal numbers, each with an optional fraction and a unit (`ns`, `us`, `µs`, `μs`, `ms`, `s`, `m`, `h`), as Go's does, with its error texts (`time: invalid duration "x"`, `time: missing unit in duration "x"`, `time: unknown unit "u" in duration "x"`), quoting as `strconv.QuoteToASCII`. A fraction is applied exactly in integer arithmetic (Go multiplies in float64; the results agree except where float64 rounding changes the last nanosecond), and a magnitude above 1<<63-1 nanoseconds is invalid. `String` formats as Go's (`72h3m0.5s`, `1.5µs`, `0s`). The package has no clock: the current time comes only from the clock capabilities (SGO-AUTH-002).
- **ownership_and_lifecycle:** None: values.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** n/a: the package has no state (D109).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** `ParseDuration` returns an error and 0 for invalid text.
- **cancellation_and_cleanup:** n/a: no waits.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export.
- **runtime_guards:** As SGO-ERR-003.
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code (SGO-IMP-004).
- **positive_tests:** fixture:std-time-duration; `sgo/scripts/test-std-diff.py` (against Go's time).
- **negative_tests:** fixture:std-time-duration (invalid texts).
- **implementation_coverage:** `src/sgostd/time/`.
- **executed_evidence:** conformance fixture std-time-duration (darwin/arm64, linux/amd64, linux/arm64); `sgo/scripts/test-std-diff.py` (seeds 1 and 7, identical to Go's); `src/sgostd` package tests; `sgo/evidence/w24/README.md`.
- **supported_execution_configurations:** production, lab.

### SGO-STD-011 — `std/bits`: Modular arithmetic and bit counts
- **version:** 1
- **status:** draft
- **reference_closure:** SGO-IMP-004, SGO-TYPE-009, SGO-VAL-001; D196.
- **grammar_and_types:** The package `std/bits` exports exactly: `func Add32(x, y, carry uint32) (sum, carryOut uint32)`; `func Add64(x, y, carry uint64) (sum, carryOut uint64)`; `func Div32(hi, lo, y uint32) (quo, rem uint32)`; `func Div64(hi, lo, y uint64) (quo, rem uint64)`; `func LeadingZeros32(x uint32) int`; `func LeadingZeros64(x uint64) int`; `func Len32(x uint32) int`; `func Len64(x uint64) int`; `func Mul32(x, y uint32) (hi, lo uint32)`; `func Mul64(x, y uint64) (hi, lo uint64)`; `func OnesCount32(x uint32) int`; `func OnesCount64(x uint64) int`; `func ReverseBytes32(x uint32) uint32`; `func ReverseBytes64(x uint64) uint64`; `func RotateLeft32(x uint32, k int) uint32`; `func RotateLeft64(x uint64, k int) uint64`; `func Sub32(x, y, borrow uint32) (diff, borrowOut uint32)`; `func Sub64(x, y, borrow uint64) (diff, borrowOut uint64)`; `func TrailingZeros32(x uint32) int`; `func TrailingZeros64(x uint64) int`.
- **preconditions:** For `Add32`, `Add64`, `Sub32`, `Sub64`: the carry or borrow is 0 or 1 (other values give Go's results). For `Div32`, `Div64`: `y != 0` and `y > hi`.
- **evaluation_and_value_semantics:** Each function returns the result of Go's `math/bits` function of the same name for every input: sums, differences, and products modulo 2^32 or 2^64 with the carry, borrow, or high word returned; quotient and remainder of a double-width dividend; rotations by k modulo the width (a negative k rotates right); bit lengths and counts; byte reversal. These are the only wrapping operations in S-Go (SGO-TYPE-009).
- **ownership_and_lifecycle:** None: values.
- **effect_and_capability_requirements:** None.
- **concurrency_and_memory_order:** n/a: the package has no state (D109).
- **allowed_nondeterminism:** None.
- **failure_and_partial_progress:** `Div32` and `Div64` fail with `DivideByZero` for `y == 0` and with `IntegerOverflow` for `y <= hi`, where Go's panic; no other function fails.
- **cancellation_and_cleanup:** n/a: no waits.
- **compile_link_checks:** `SGO_E_UNDEFINED` for a name the package does not export.
- **runtime_guards:** As SGO-ERR-003.
- **tcb_assumptions:** None beyond the compiler and runtime used by all S-Go code (SGO-IMP-004).
- **positive_tests:** fixture:std-bits-basic; `.work/w26/bitsdiff.py` (against Go's math/bits; 28426 results identical, seeds 1-3), to move into `sgo/scripts/test-std-diff.py`.
- **negative_tests:** fixture:std-bits-div-zero, fixture:std-bits-div-overflow.
- **implementation_coverage:** `src/sgostd/bits/`.
- **executed_evidence:** PENDING: W26.
- **supported_execution_configurations:** production, lab.
