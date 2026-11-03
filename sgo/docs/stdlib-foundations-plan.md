# Standard-library foundations — plan (draft for decisions D103–D110)

Status: **draft, awaiting owner approval.** W13 closed at spec 0.10.1 (D102). The owner asked to
prepare for a standard library. This plan says what a standard library needs that S-Go does not
have yet, proposes the order of the work packages that provide it, and lists the decisions the owner
must take. Nothing here changes the spec until the drafts of each work package are approved, as in
W13.

## 1. What a standard library needs, and what S-Go has today

The first standard library is the set of packages most programs reach for: errors, strings and
bytes, number conversion, UTF-8, sorting, a string builder, small integer math, and later
formatting. Measured against spec 0.10.1:

| Need | Today (0.10.1) | Clause | Gap |
|---|---|---|---|
| Import packages outside the program's module | only the module's own packages and `sgo` | SGO-IMP-001 | **blocking**: there is nowhere for a standard library to live |
| Share code between projects | no `require` in `sgo.mod` | SGO-IMP-001 | blocking for third-party libraries; not for a standard library shipped with the toolchain |
| Text by character (UTF-8) | `string`, `[]byte`, `byte`; no `rune`, no rune literals, no `range` over strings | SGO-TYPE-001, SGO-LEX-004, SGO-COMP-005 | **blocking** for `strings`/`unicode/utf8`-style APIs |
| Abstraction over types (readers, writers, sorting) | interfaces denied except `error` | SGO-IFACE-001 | **blocking** for I/O-style libraries; avoidable for a first text/number set |
| Generic containers and algorithms | user type parameters denied | SGO-GEN-001 | avoidable at first (per-type functions, function parameters) |
| Variadic functions (`Join`, formatting) | variadic parameters denied | SGO-TYPE-003 | avoidable (slices as parameters) |
| Package-level state | package-level variables denied | SGO-BIND-003 | avoidable and desirable: a standard library without package state is simpler to make race-free |
| Floating point | denied | SGO-FLOAT-001 | out of scope for the first set |
| Errors with context | `sgo.NewError(msg)`; no wrapping, no inspection | SGO-TYPE-006 | needed by an `errors` package |

So three things block a first standard library: **a place to put it** (packages outside the
module), **runes** (text), and, for anything I/O-shaped, **interfaces**. Generics, variadics and
floats can wait.

## 2. Proposed work packages, in order

| WP | Content | Unblocks | Size |
|---|---|---|---|
| **W14** | Packages beyond the module: the toolchain's own S-Go packages under the reserved prefix `std/`, and `require` of other modules from local directories, with a lock file of content digests and SGO-IMP-002 contract checks across modules | a place for the standard library; sharing code between projects | medium |
| **W15** | Text: the `rune` type (checked, 0..0x10FFFF), rune literals, `range` over a string yielding byte index and rune (invalid UTF-8 yields U+FFFD), `string(r)` and `[]rune` conversions | `std/strings`, `std/utf8`, `std/strconv` quoting | small |
| **W16** | Standard library v1, written in S-Go under `std/`: `errors`, `strings`, `bytes`, `strconv` (integers), `utf8`, `sort` (by a `less` function), `math` (integer helpers); every package without package-level state | most programs' everyday needs | medium |
| **W17** | Interfaces: user-declared method-set interfaces, under the sharing rule (D107) | I/O-style libraries (`Reader`, `Writer`), `sort` over an interface, `fmt` | large |
| later | user generics; variadic parameters; formatting (`fmt`); floats | containers, printf-style output | each its own decision |

W14 comes first because without it there is nowhere to put a standard library. W15 is small and
unblocks text. W16 can then ship a useful first standard library *without* interfaces or generics
(Go's own library before 1.18 shows how far function parameters and per-type functions go).
W17, interfaces, is the largest item and touches the sharing rule again (the W13 review's V2 was an
interface conversion), so it gets its own model-first work package rather than being rushed into
the first library.

## 3. Decisions for the owner (D103–D110)

| ID | Question | Recommendation |
|---|---|---|
| D103 | Where does the standard library live, and is it trusted? | S-Go source shipped with the toolchain under the reserved import prefix `std/` (`std/strings`, …), compiled under exactly the same admission and sharing rules as user code. It is **not** part of the TCB: it gains no privilege, and its packages are identified by contract digests like any dependency. A user module path may not equal or lie under `std` (as for `sgo`, D38). |
| D104 | How are other modules required? | `require <module path> <version>` lines in `sgo.mod`, resolved only from local directories in this version: a `replace <path> => <dir>` line, or a vendored copy under `vendor/`. No network fetch. `sgo.sum` records a content digest of every required module's files; a mismatch is an error. Versions are compared but no minimum-version selection is attempted: each module path appears once in the build, and two different required versions of one path are an error. |
| D105 | What crosses a module boundary? | Exported names only, checked by the existing SGO-IMP-002 contract digests extended to required modules; a dependency's effect grants are not inherited (the root module's grants bound the program, SGO-AUTH-*), and a dependency's own `sgo.mod` grants must be a subset of the root's or the build fails. |
| D106 | Runes | `rune` as a checked integer type (0..0x10FFFF, overflow is `IntegerOverflow` like `byte`), rune literals, `range` over a string yielding (byte index, rune) with U+FFFD for invalid UTF-8, and `string(r)`, `[]rune(s)`, `string(rs)` conversions. `range` over a string stays an iteration *by rune*, never by byte (bytes are `[]byte(s)` or indexing). |
| D107 | Interfaces and the sharing rule (W17, decided now so W16 avoids designs W17 would break) | Interface values take the sharing class of what they may hold: an ordinary interface type is **task-local** (any admitted type converts to it; it crosses tasks only through `sgo.Copy`, which deep-copies the dynamic value); `error` stays **shareable** and only shareable types convert to it (as V2 fixed). Type assertions and type switches come with interfaces. |
| D108 | Generics | Deferred. The first standard library uses per-type functions (`sort.Ints`, `sort.Strings`) and function parameters (`sort.Slice`-style with an index `less`), which S-Go already admits. Generics get their own decision when containers are needed. |
| D109 | Package-level state in the standard library | None. Every `std/` package is functions, types and constants only; there is no package-level variable to race on, no `init`, and no hidden global (e.g. no global random source: randomness stays a capability, `*sgo.Random`, SGO-AUTH-*). |
| D110 | Process | As W13: drafts of each work package's clauses for approval, conformance cases with negative fixtures, a spec version per work package, mutation tests, and reviewer verification on linux. The standard library's packages get conformance cases *and* their own `sgo test` suites, run in CI. |

## 4. What W14 would contain (for orientation; its drafts come after approval)

- `sgo.mod` grammar: `require <path> <version>`, `replace <path> => <relative dir>`; `sgo.sum`.
- The driver resolves `std/...` from `$GOROOT/src/sgostd/` (S-Go sources shipped in the release
  archive; not `src/std/`, because `std` is a reserved pattern of the go command and the fork's
  upstream tooling must keep working) and required modules from their directories; each package is compiled once, in
  dependency order, with its own module's spec version checked against the toolchain's.
- SGO-IMP-001 v2 (packages beyond the module), SGO-IMP-002 v2 (contract digests across modules),
  a new SGO-IMP-003 (`require`, `replace`, `sgo.sum`), SGO-AUTH (grants of dependencies).
- Conformance: a two-module program, a digest mismatch, a cycle across modules, a version conflict,
  a dependency asking for more grants than the root, a module path under `std`, and a `std/`
  package compiled under the same admission rules (a `std/` file using a denied construct is
  refused like user code).
- A first `std/` package (`std/errors` or `std/strconv` with `Itoa`/`Atoi`) as the proof that the
  path works end to end; the rest of the library is W16.

## 5. What the owner needs to do now

Approve, amend, or reject D103–D110. On approval, W14's clause drafts are written and brought back
for approval before implementation, exactly as W13's stages were.
