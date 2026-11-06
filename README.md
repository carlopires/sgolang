# S-Go (sgolang)

S-Go is an independent, specification-defined language with structured concurrency, built from a
fork of the Go compiler and runtime (pinned to `go1.27.3`). It has no Go compatibility mode: code
without a valid S-Go specification is not admitted.

## Why S-Go

- **No detached work.** Every task belongs to a `nursery`; leaving the nursery waits for all of its
  tasks, on every exit path, and a failing task cancels its siblings.
- **Resources belong to scopes.** Files, connections, timers, and user resources are released when
  their scope ends; the compiler refuses code that lets one outlive its scope.
- **No data races between tasks.** Mutable values stay in one task; they cross only as copies,
  frozen values, or through synchronized handles.
- **Explicit authority.** Files, network endpoints, clocks, and randomness are capabilities granted in
  `sgo.mod`; there is no ambient access.
- **One result on every target.** `int` is 64 bits everywhere, `float64` arithmetic is IEEE 754
  rounded once per operation and never fused, and maps iterate in insertion order, so a program
  prints the same output on darwin/arm64, linux/amd64, and linux/arm64.
- **Deterministic testing.** The lab configuration runs a program under a seeded scheduler and
  virtual time, records a trace, and replays it exactly.

```go
package main

import (
	"sgo"
	"std/strconv"
)

func main(root *sgo.Context) error {
	nursery (n) {
		a := sgo.Go(n, func(t *sgo.Context) (int, error) { return 20, nil })
		b := sgo.Go(n, func(t *sgo.Context) (int, error) { return 22, nil })
		x := check a.Wait(root)
		y := check b.Wait(root)
		check root.Stdout().WriteString("sum: " + strconv.Itoa(x+y) + "\n")
	} // every task started in n has ended here
	return nil
}
```

This program is the conformance case
[`readme-example`](sgo/testdata/conformance/readme-example/main.sgo); it prints `sum: 42`.

## Status

Spec **0.21.0** (the current version is in [`sgo/spec/VERSION`](sgo/spec/VERSION)). The toolchain
(`sgo build`, `run`, `check`, `test`, `fmt`, `mod`, `lab`) works on darwin/arm64, linux/amd64, and
linux/arm64. Work packages W00–W25 are complete (W11, fixed stacks, was investigated and not
adopted); W25 admitted `float64` with `std/math`, `strconv`, and `fmt` support. The language is still
evolving: each spec version is frozen, and changes are recorded in
[`sgo/spec/CHANGELOG.md`](sgo/spec/CHANGELOG.md).

## Getting started

- Build from source (needs only the pinned bootstrap, fetched by the script):
  `sgo/scripts/build-stage0.sh`, which installs `pkg/tool/<os>_<arch>/sgo`.
- Install a release archive: [`sgo/docs/INSTALL.md`](sgo/docs/INSTALL.md).
- Examples: [`sgo/examples/`](sgo/examples/) (Happy Eyeballs, an HTTP/1.1 server, a CLI, pipeline,
  service, lab replay).

## Documentation

- Specification: [`sgo/spec/`](sgo/spec/README.md) (normative clauses) and the published guarantees,
  [`sgo/docs/GUARANTEES.md`](sgo/docs/GUARANTEES.md).
- Design: [`sgo/SGO_LANGUAGE_CONSTITUTION.md`](sgo/SGO_LANGUAGE_CONSTITUTION.md),
  [`sgo/SGO_DECISIONS.md`](sgo/SGO_DECISIONS.md), and the full bundle in [`sgo/`](sgo/README.md).

Licensed under Go's BSD-style license; see [`LICENSE`](LICENSE) and [`PATENTS`](PATENTS).
