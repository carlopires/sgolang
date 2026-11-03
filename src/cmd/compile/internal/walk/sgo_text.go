// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

// S-Go text conversions (SGO-COMP-005 v2): []rune(s) has length and capacity equal to the
// number of runes. The runtime's stringtoslicerune returns a stack buffer of 32 runes for a
// non-escaping result, or an allocation rounded up to a size class, so its capacity depends on
// escape analysis and the allocator; S-Go reslices the result to r[:len(r):len(r)], so aliasing
// after append is a function of the program alone.

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
)

// sgoExactCap returns call resliced to its own length as capacity when the package is compiled
// in S-Go mode, and call unchanged otherwise.
func sgoExactCap(call ir.Node, init *ir.Nodes) ir.Node {
	if !base.SGo {
		return call
	}
	pos := call.Pos()
	tmp := typecheck.TempAt(pos, ir.CurFunc, call.Type())
	appendWalkStmt(init, typecheck.Stmt(ir.NewAssignStmt(pos, tmp, call)))
	len1 := typecheck.Expr(ir.NewUnaryExpr(pos, ir.OLEN, tmp))
	len2 := typecheck.Expr(ir.NewUnaryExpr(pos, ir.OLEN, tmp))
	s := ir.NewSliceExpr(pos, ir.OSLICE3, tmp, nil, len1, len2)
	s.SetType(call.Type())
	s.SetTypecheck(1)
	s.SetBounded(true) // 0 ≤ len ≤ len ≤ cap always holds
	return walkExpr(s, init)
}
