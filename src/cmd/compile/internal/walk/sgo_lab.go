// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/internal/src"
)

// sgoLabInstrument adds the lab's step counter to fn (D43): one step at the function's entry and
// one at the start of every iteration of each loop (its back-edge; S-Go has no goto). Only S-Go
// code of a lab build is instrumented (-sgolab); closures are separate functions and get their
// own entry step. Compiler-generated wrappers are not S-Go code and are left alone.
func sgoLabInstrument(fn *ir.Func) {
	if !base.SGo || !base.Flag.SGoLab || fn.Wrapper() {
		return
	}
	step := func(pos src.XPos) ir.Node {
		return typecheck.Stmt(typecheck.Call(pos, typecheck.LookupRuntime("sgoLabStep"), nil, false))
	}
	ir.VisitList(fn.Body, func(n ir.Node) {
		switch n := n.(type) {
		case *ir.ForStmt:
			n.Body.Prepend(step(n.Pos()))
		case *ir.RangeStmt:
			n.Body.Prepend(step(n.Pos()))
		}
	})
	fn.Body.Prepend(step(fn.Pos()))
}
