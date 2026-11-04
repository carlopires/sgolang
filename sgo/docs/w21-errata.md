# Spec 0.16.1 errata: W21 review Z1–Z4

The review at `1444dd8440` (note2, linux/amd64) found that the resource flow rule could be
evaded through pointers, interfaces, parameter and receiver stores, `Own` of outer values, and
`defer` inside nursery bodies. It also found that a nil resource result with a nil error was
misreported. The implementation is fixed; these are the clause amendments. Each bumps the
clause version, and all of them freeze together as 0.16.1.

## SGO-RES-001 v2: resource types

- **grammar_and_types:** add: "A pointer to, or a map whose keys or values are, a value of a
  resource type is itself a resource type. A value of a resource type is never converted to an
  interface type (`SGO_E_RESOURCE_ESCAPES` at the conversion), except as an argument of package
  `sgo` (`Own`, `MoveTo`): an interface value is not tied to a scope."

## SGO-RES-003 v2: the flow rule

- **evaluation_and_value_semantics:** add: "A store whose target is reached through a pointer, a
  slice, or a map (`p.f = r`, `s[i] = r`, `*p = r`, `x.p.f = r`) is checked against the extent
  from which that memory may be reached, not against the declaration of the root variable. For
  a single step through the root, this is the extent of the root's top-level pointee. For a
  deeper path, it is the extent of any memory reachable through the root. An allocation (`&T{}`,
  `new`, `make`, a slice or map literal) is reachable from the extent that makes it; an
  assignment of a reference value makes the target's extent reach the source's memory, and
  aliases share it; parameters and receivers reach the caller's memory, so a store of one
  parameter's resource into another parameter's or the receiver's memory is an escape. Memory
  of unknown origin (a call result, a package-level variable) may be reached from anywhere."
- Rule (3): "…a function literal called in place, or deferred when the deferring function's own
  extent is the scope (a task body, or a scope parameter)". A deferred call inside a nursery
  statement may not mention a resource tied to that nursery or to a scope nested inside the
  function (`SGO_E_RESOURCE_ESCAPES` at the use): it runs when the function returns, after the
  nursery released the resource.
- **negative_tests:** add fixture:res-escape-alias, fixture:res-escape-any, fixture:res-escape-addr,
  fixture:res-escape-param-store, fixture:res-escape-receiver-store,
  fixture:res-defer-nursery-capture.
- **positive_tests:** add fixture:res-local-holder-ok, fixture:res-defer-task-scope.

## SGO-RES-004 v2: user resources

- **preconditions:** add: "The operand of `s.Own(x)` was made inside `s`'s extent (an allocation
  there, or a local assigned only such values); `Own` of a value named outside the scope, or
  reachable from outer memory, is `SGO_E_RESOURCE_ESCAPES` at the operand."
- **cancellation_and_cleanup:** say explicitly: "`ok` is false whenever the scope is cancelled,
  including by an ancestor's cancellation, even if its body and all children end normally."
- **negative_tests:** add fixture:res-own-outer-var, fixture:res-own-outer-field.
- **positive_tests:** add fixture:res-own-fresh.

## SGO-ERR-004 v2: `check`

- **runtime_guards:** replace "resource results are nil and are never observed by a `check` in
  the caller" with: "A `return` that gives the literal `nil` for a resource result checks at run
  time that its error operand is not nil; otherwise the task fails with `InvalidArgument` at the
  `return` ('a function returned a nil resource with a nil error'), so a resource result is nil
  only together with an error."
- **positive_tests:** add fixture:res-return-nil-nil-runtime.

## SGO-RES-002 v2: `LendTo` (Q4)

- Erratum: "removed from every resource handle type and `*sgo.Dir`". Effect handles keep `LendTo`
  for now. Its withdrawal (D169, planned for 0.17.0) needs SPEC-AUTH to say whether effect
  handles are tied like resources or are shareable authority tokens gated by ceilings.

## SGO-COMP-007 v2: nil handles (Z4.2)

- **runtime_guards:** add: "A method call on a handle of package `sgo` checks its receiver: the
  zero value of a struct field of handle type is nil, and the call fails with `NilDereference`
  at the call (not as a nil error inside package `sgo`)." The check is inserted when the call is
  read, before inlining.
- **negative_tests:** add fixture:res-nil-handle-field.

## Not in this erratum

- `res-defer-nursery-close` (`defer c.Close()` inside a nursery) is covered by the same rule as
  `res-defer-nursery-capture`; a file-based case is a follow-up.
