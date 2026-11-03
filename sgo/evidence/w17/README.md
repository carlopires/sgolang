# W17 evidence: interfaces (spec 0.14.0)

These decisions were approved on 2026-11-03:
- D107 (interfaces under the sharing rule);
- D127–D133 (`sgo/docs/w17-drafts.md`).

Clauses frozen at 0.14.0:
- family SPEC-IFACE admitted: SGO-IFACE-002, SGO-IFACE-003, SGO-IFACE-004 (new), SGO-IFACE-001 v2;
- amended: SGO-TYPE-001 v9, SGO-TYPE-006 v2, SGO-COMP-004 v4, SGO-CTL-001 v2, SGO-STD-001 v2,
  SGO-STD-006 v2.

Guarantees: SPEC-IFACE moves from G-DENY to G-LANG; G-STD names `errors.Unwrap` and `sort.Sort`.

## Model first

The W13 share model (`sgo/model/share*.go`) gained the kind `SIface` and the operations
`SToIface` (boxing: a value is copied into the box, a pointer is shared) and `SAssert` (an
assertion returns an alias of the boxed object), a channel of interface values, and the D132
checker rules. It was committed (`e5a94b9d59`) before any compiler change.

- **Scenarios:** 11 new (3 accepted, 8 refused with their diagnostics), all explored without an
  ownership violation.
- **Soundness:** `GenShareIface` generates programs with boxing, assertions, method calls through
  interfaces, and the interface channel: 17,577 of 20,000 accepted, every interleaving of each
  explored, no violation.
- **Mutants**, each found by the generated programs alone (TestShareIfaceGeneratorFindsMutations):
  plain send of an interface, spawn capture of an interface, `error(i)` of an interface, and
  boxing or asserting without marking the boxed object aliased.
- The earlier generators' streams are unchanged: 17,362 and 17,235 programs accepted, as at W13.

## What was built

- **Admission** (`sgoadmit`): interface declarations (methods and embedded named interfaces),
  `any`, comma-ok assertions, type switches. Interface literals, type-set elements, and the
  single-value assertion are refused (SGO-IFACE-001).
- **Types** (`types2/sgo*.go`; the generated twins are untouched):
  - interface types other than `error` are task-local;
  - interface fields and elements are admitted; an `error` field keeps a struct shareable;
  - interfaces are never map keys, and compare only with `nil` (including inside structs);
  - constraint interfaces are refused, and `nil` is an interface value.
- **No typed nil** (`walk/convert.go`): converting a pointer sets the type word only when the
  pointer is non-nil, so a nil pointer becomes the nil interface (D130).
- **Nil calls** (`ssagen`): in S-Go code an interface method call checks the type word and calls
  `runtime.sgoNilInterfaceCall` with the position: `NilDereference`, "method call on a nil
  interface value". A call on a nil `error` keeps `NilError` (SGO-TYPE-006).
- **`sgo.Copy`:** deep-copies an interface's dynamic value (also inside structs); `Copy` of a nil
  interface was a run-time defect (the generic `.(T)` on nil) and now returns nil.
- **Library:** `errors.Unwrap` (through a named interface in `std/errors`) and `sort.Interface`
  with a stable, in-place `sort.Sort` (Go's insertion-sorted blocks and SymMerge, Len/Less/Swap
  only). `IsSorted` was left unexported: D133 did not approve it.

## Checks

- **Conformance:** 30 new cases (29 `iface-*` and `std-sort-interface`); `lex-denied-operators`
  now points at the `~int` element. Totals and platforms: see "Results".
- **Mutants**, each applied, rebuilt, caught by the named case, and restored (byte comparison):

  | Mutant | Caught by |
  |---|---|
  | single-value assertion admitted | iface-assert-single-denied |
  | typed nil kept (D130 removed) | iface-nil-pointer-is-nil |
  | interface types classified shareable | iface-send-denied, iface-capture-denied, iface-to-error-denied |
  | `==` between interface values admitted | iface-compare-denied |
  | nil interface call left to Go's nil check | iface-nil-call |
  | `sgo.Copy` shares an interface field's dynamic value | iface-copy-send |
  | constraint interface admitted | iface-constraint-denied |

  The `sgo.Copy` mutant was first missed: a top-level interface reaches `sgo.Copy` as `any`, so
  only an interface inside a struct goes through `copyIface`. `iface-copy-send` was extended with
  a struct holding an interface field, and the mutant is now caught.
- **Library:** `sort.Sort` is tested against a reference stable sort over 20 sizes (block
  boundaries and several merge levels) and 4 key ranges, with a swap bound; a planted stability
  bug fails `TestSortStable`. `std/errors` tests `Unwrap` on chains, nil, and non-wrappers.

## Results

- darwin/arm64 (local, on the freeze tree): conformance 561/561 (307 run, 217 reject, 31 lab, 6
  test); spec-tool (127 clauses), std-api-check (96 declarations), guarantee-check, inventory,
  policy, generated-twin-check, legacy-audit, license-audit, lock-alloc-check, adoption-check;
  test-identity, test-tcb, test-modules, test-std (20 tests), test-negative, test-flags,
  test-check (544 cases, check and build agree), test-test, test-std-diff (9,784 checks identical
  to Go), test-map-lowering; the share model tests.
- linux/amd64, linux/arm64: sgo-w00 run 41709938064 on the freeze commit `c44d0975f0` passed on both
  runners: stage-0 builds, conformance (561 cases), test-std, test-std-diff, the module tests,
  legacy-audit, the negative fixtures, and the upstream baseline (B03). sgo-spec run 41709938000
  (spec checks and the share model on both Linux architectures) passed on the same commit.
