// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// Frozen values (SGO-COMP-009, D98–D101). sgo.Freeze(x) returns an sgo.Frozen[T] that any number
// of tasks read without copying. The guarantee that nobody writes it is static: the contents are
// reached only through access paths, chains that start with f.Get(), and a reference into a
// frozen value (a slice, map, or pointer, or a value holding one) never leaves its path.

// sgoFrozenElem reports whether T is sgo.Frozen[E], and returns E.
func sgoFrozenElem(T Type) (Type, bool) {
	n, ok := Unalias(T).(*Named)
	if !ok || n.obj == nil || n.obj.pkg == nil || n.obj.pkg.path != "sgo" || n.obj.name != "Frozen" {
		return nil, false
	}
	if n.TypeArgs().Len() != 1 {
		return nil, false
	}
	return n.TypeArgs().At(0), true
}

// sgoFrozenElemOK checks the type argument of sgo.Frozen[T] or sgo.Freeze[T]: T holds no handle
// and no channel (D100).
func (check *Checker) sgoFrozenElemOK(at syntax.Expr, T Type) {
	if why := sgoFrozenRefusal(T, map[Type]bool{}); why != "" {
		check.errorf(at, UnsupportedFeature, "SGO_E_UNADMITTED: sgo.Frozen[%s]: a frozen value holds no %s (SGO-COMP-009)", T, why)
	}
}

func sgoFrozenRefusal(T Type, seen map[Type]bool) string {
	T = Unalias(T)
	if seen[T] {
		return ""
	}
	seen[T] = true
	if isSgoHandle(T) {
		return "handle"
	}
	if sgoResIface(T) || sgoHasRelease(T) {
		return "resource" // a resource interface or a user resource: tied to a scope (D176, SGO-RES-006)
	}
	if _, ok := sgoFrozenElem(T); ok {
		return "" // its element was checked where that Frozen type was formed
	}
	switch u := T.Underlying().(type) {
	case *Chan:
		return "channel"
	case *Struct:
		for _, f := range u.fields {
			if why := sgoFrozenRefusal(f.typ, seen); why != "" {
				return why
			}
		}
	case *Array:
		return sgoFrozenRefusal(u.elem, seen)
	case *Slice:
		return sgoFrozenRefusal(u.elem, seen)
	case *Map:
		if why := sgoFrozenRefusal(u.key, seen); why != "" {
			return why
		}
		return sgoFrozenRefusal(u.elem, seen)
	case *Pointer:
		return sgoFrozenRefusal(u.base, seen)
	case *Signature:
		return "function value"
	}
	return ""
}

// sgoFreezeCall checks the element type of sgo.Freeze(x) when T is inferred (an explicit
// sgo.Freeze[T] is checked by sgoInstanceElem).
func (check *Checker) sgoFreezeCall(call *syntax.CallExpr) {
	if !isSgoSelector(syntax.Unparen(call.Fun), "Freeze") {
		return
	}
	if tv, ok := check.Types[call]; ok && tv.Type != nil {
		if elem, frozen := sgoFrozenElem(tv.Type); frozen {
			check.sgoFrozenElemOK(call, elem)
		}
	}
}

// sgoFrozenPaths checks every f.Get() in fd against the access-path rule (D99).
func (check *Checker) sgoFrozenPaths(fd *syntax.FuncDecl) {
	var stack []syntax.Node
	syntax.Inspect(fd, func(n syntax.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		if sel, _ := n.(*syntax.SelectorExpr); sel != nil && sel.Sel.Value == "Get" {
			if tv, ok := check.Types[sel.X]; ok && tv.Type != nil {
				if _, frozen := sgoFrozenElem(tv.Type); frozen {
					check.sgoFrozenPath(stack)
				}
			}
		}
		return true
	})
}

// sgoParent returns the nearest proper ancestor of stack[i] that is not a parenthesized
// expression, and its index (-1 if none).
func sgoParent(stack []syntax.Node, i int) (syntax.Node, int) {
	for j := i - 1; j >= 0; j-- {
		if _, paren := stack[j].(*syntax.ParenExpr); !paren {
			return stack[j], j
		}
	}
	return nil, -1
}

// sgoFrozenPath checks the access path headed by the f.Get selector at the top of stack.
func (check *Checker) sgoFrozenPath(stack []syntax.Node) {
	i := len(stack) - 1
	sel := stack[i].(*syntax.SelectorExpr)
	p, j := sgoParent(stack, i)
	call, _ := p.(*syntax.CallExpr)
	if call == nil || syntax.Unparen(call.Fun) != sel {
		check.errorf(sel, UnsupportedFeature, "SGO_E_UNADMITTED: %s is a method value; frozen contents are read only by calling Get (SGO-COMP-009)", syntax.String(sel))
		return
	}
	var cur syntax.Expr = call
	i = j
	for {
		p, j = sgoParent(stack, i)
		switch p := p.(type) {
		case *syntax.SelectorExpr:
			if syntax.Unparen(p.X) == cur {
				if s := check.Selections[p]; s != nil && s.kind != FieldVal {
					// A method call on a frozen-derived value: its receiver must hold no reference
					// (a value receiver copies the value; a pointer receiver takes its address).
					if s.kind == MethodVal && !check.sgoFrozenTaskLocal(cur) {
						if _, ptr := Unalias(s.obj.(*Func).Signature().Recv().Type()).(*Pointer); !ptr {
							return
						}
					}
					check.errorf(p, UnsupportedFeature, "SGO_E_FROZEN_WRITE: the method %s would receive a reference into a frozen value (SGO-COMP-009)", syntax.String(p))
					return
				}
				cur, i = p, j
				continue
			}
		case *syntax.IndexExpr:
			if syntax.Unparen(p.X) == cur {
				cur, i = p, j
				continue
			}
		}
		break
	}
	// cur is the end of the path; p is its parent.
	switch p := p.(type) {
	case *syntax.AssignStmt:
		if sgoListHas(p.Lhs, cur) {
			check.errorf(cur, UnsupportedFeature, "SGO_E_FROZEN_WRITE: %s is part of a frozen value and cannot be assigned (SGO-COMP-009)", syntax.String(cur))
			return
		}
	case *syntax.Operation:
		if p.Op == syntax.And && p.Y == nil {
			check.errorf(p, UnsupportedFeature, "SGO_E_FROZEN_WRITE: the address of part of a frozen value cannot be taken (SGO-COMP-009)")
			return
		}
	}
	if !check.sgoFrozenTaskLocal(cur) {
		return // a value (copied out), a Frozen[U], or a channel-free value: nothing escapes
	}
	switch p := p.(type) {
	case *syntax.CallExpr:
		if name, _ := syntax.Unparen(p.Fun).(*syntax.Name); name != nil && len(p.ArgList) == 1 {
			if b, _ := check.Uses[name].(*Builtin); b != nil && b.id == _Len {
				return
			}
		}
	case *syntax.RangeClause:
		if syntax.Unparen(p.X) == cur {
			for _, e := range sgoList(p.Lhs) {
				if tv, ok := check.Types[e]; ok && tv.Type != nil && sgoTaskLocal(tv.Type) {
					check.errorf(e, UnsupportedFeature, "SGO_E_FROZEN_WRITE: range over a frozen value binds %s to a reference into it; range over values only (SGO-COMP-009)", syntax.String(e))
					return
				}
				if name, _ := e.(*syntax.Name); name != nil {
					if obj := check.Defs[name]; obj != nil && sgoTaskLocal(obj.Type()) {
						check.errorf(e, UnsupportedFeature, "SGO_E_FROZEN_WRITE: range over a frozen value binds %s to a reference into it; range over values only (SGO-COMP-009)", name.Value)
						return
					}
				}
			}
			return
		}
	}
	tv := check.Types[cur]
	check.errorf(cur, UnsupportedFeature, "SGO_E_FROZEN_WRITE: %s (type %s) is a reference into a frozen value; it may only be selected, indexed, passed to len, or ranged over (SGO-COMP-009)", syntax.String(cur), tv.Type)
}

func (check *Checker) sgoFrozenTaskLocal(e syntax.Expr) bool {
	tv, ok := check.Types[e]
	return ok && tv.Type != nil && sgoTaskLocal(tv.Type)
}

func sgoList(e syntax.Expr) []syntax.Expr {
	if e == nil {
		return nil
	}
	if l, _ := e.(*syntax.ListExpr); l != nil {
		return l.ElemList
	}
	return []syntax.Expr{e}
}

func sgoListHas(e syntax.Expr, x syntax.Expr) bool {
	for _, y := range sgoList(e) {
		if syntax.Unparen(y) == x {
			return true
		}
	}
	return false
}
