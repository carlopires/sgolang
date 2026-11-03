// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

// S-Go maps (SGO-COMP-006): every map operation of a package compiled in S-Go mode calls the
// runtime's insertion-ordered map (src/runtime/sgo_map.go) through the generic entry points, never
// Go's maps. The decision is the package's mode (sgoMaps), so no operation of an S-Go package can
// reach Go's maps, whatever its position; package sgo uses no maps at all (a compile error), so no
// Go map is ever inlined into S-Go code (W13 review V3).

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
)

// sgoMaps reports whether map operations of the package being compiled use the ordered map: every
// map of an S-Go package is an *sgoMap, so the choice is the package's compilation mode, not the
// position of the operation (W13 review V3: a position-less node must not fall back to Go's map).
// S-Go packages are compiled from .sgo files only and inline nothing that uses maps: package sgo
// uses none (refused below), and S-Go packages import no other Go package.
func sgoMaps() bool {
	if base.SGo {
		return true
	}
	if base.Ctxt.Pkgpath == "sgo" {
		base.Fatalf("sgo: package sgo must not use maps (they would be Go maps; S-Go maps are the runtime's ordered map, SGO-COMP-006)")
	}
	return false
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
