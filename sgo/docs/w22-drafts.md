# W22 clause drafts

Drafts for the W22 spec pass (freeze 0.17.0). Each one replaces a field of a frozen clause
and bumps its version. They are not in `sgo/spec/` yet, so the identity digest has not moved.

## SGO-CTL-004 v2 — Exits that cross nurseries (D159)

Implemented in `711ea7b88c`. Fixtures: `exit-check-cancels`, `exit-return-error-cancels`,
`exit-bare-return-cancels`, `exit-nil-return-joins`.

- **evaluation_and_value_semantics:** For `return e1, …, en`: evaluate the operands once, left
  to right, and hold the values. If the function's last result has type `error` and its held
  value (for a bare return, the named result) is not nil, cancel each crossed nursery from the
  innermost outward (SGO-CANCEL-001, cause `error exit by task N`). Then complete each crossed
  nursery from the innermost outward exactly as if its block had ended normally (SGO-NUR-003,
  SGO-NUR-004). Then run the function's deferred calls (SGO-DEF-001). Then return the held
  values. A failed `check` is such a return (SGO-ERR-004). For `break`/`continue` that leave
  nurseries: complete each crossed nursery innermost first, then transfer control. A `break` or
  `continue` whose target is inside the nursery does not complete it. Each execution of a
  nursery statement is a distinct scope with its own identity.
- **cancellation_and_cleanup:** An exit with a non-nil error cancels the nurseries it crosses
  before joining them. The cancellation is the owner's own, so it is consumed at each nursery's
  boundary (SGO-CANCEL-004): the nursery completes normally unless a child failed, and the
  returned error is unchanged. Every other exit (nil error, no error result, `break`,
  `continue`) joins without cancelling, and children still running keep the owner in JOINING.
- **positive_tests:** add fixture:exit-check-cancels, fixture:exit-return-error-cancels,
  fixture:exit-bare-return-cancels, fixture:exit-nil-return-joins.

## SGO-ERR-004 v2 — `check` (D159)

- **cancellation_and_cleanup:** add: "A failed `check` inside nurseries cancels the nurseries it
  crosses before joining them (SGO-CTL-004 v2)."
- **positive_tests:** add fixture:exit-check-cancels.
