// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

// S-Go maps (SGO-COMP-006): map operations of functions compiled from .sgo files call the runtime's
// insertion-ordered map (src/runtime/sgo_map.go) through the generic entry points, never Go's
// maps. The decision is made per operation, by the source file of its position: S-Go code
// inlined into a generated Go function (the main wrapper) keeps its .sgo positions, and Go code
// inlined into an S-Go function keeps its .go positions (package sgo and the runtime use no maps).

import (
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
)

// sgoMaps reports whether the operation being walked (at base.Pos) comes from an S-Go source file.
func sgoMaps() bool {
	return base.Pos.IsKnown() && strings.HasSuffix(base.Ctxt.PosTable.Pos(base.Pos).Filename(), ".sgo")
}

// sgoMakeMap lowers make(map[K]V, hint) for S-Go code: the map is always allocated by the runtime.
func sgoMakeMap(n *ir.MakeExpr, init *ir.Nodes) ir.Node {
	t := n.Type()
	hint := n.Len
	if hint == nil {
		hint = ir.NewInt(base.Pos, 0)
	}
	fn := typecheck.LookupRuntime("sgomakemap", t.Key(), t.Elem())
	return mkcall1(fn, n.Type(), init, reflectdata.MakeMapRType(base.Pos, n), typecheck.Conv(hint, types.Types[types.TINT]))
}

// sgoPosString is the position of the current node as a string constant, for failures that name
// it (SGO-COMP-008).
func sgoPosString() ir.Node {
	return typecheck.Expr(ir.NewString(base.Pos, base.SGoPos(base.Pos)))
}
