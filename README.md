# S-Go (sgolang)

S-Go is an independent, specification-defined language with structured concurrency, built from a fork of the Go compiler and runtime. It has no Go compatibility mode: code without a valid S-Go specification is not admitted.

**Status:** planning complete for the first slice; implementation not started. Nothing here builds yet.

- Design and execution bundle: [`sgo/`](sgo/README.md)
- Start with [`sgo/SGO_LANGUAGE_CONSTITUTION.md`](sgo/SGO_LANGUAGE_CONSTITUTION.md), then [`sgo/SGO_DECISIONS.md`](sgo/SGO_DECISIONS.md) and [`sgo/SGO_FRONTIER_EXECUTION_PLAN.md`](sgo/SGO_FRONTIER_EXECUTION_PLAN.md).
- First-slice language draft: [`sgo/SGO_SPEC_SUBSET0.md`](sgo/SGO_SPEC_SUBSET0.md)

The Go source fork (pinned `go1.27.3`) is imported by work package W00.
