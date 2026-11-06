// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// Package-level tables (D136, SGO-BIND-003 v2). The one package-level variable S-Go admits is a
// frozen table: var name = sgo.Freeze(lit), where lit is a composite literal whose elements and
// keys, at every depth, are constants or nested composite literals. Its value is built once
// before main, no initializer can read another variable, and no task can write it: the name is
// never assigned and its address is never taken.

// sgoPackageTables checks the initializer of every package-level variable declaration. The
// admitter (sgoadmit) has already refused every other form of package-level var.
func (check *Checker) sgoPackageTables(files []*syntax.File) {
	for _, f := range files {
		for _, d := range f.DeclList {
			vd, _ := d.(*syntax.VarDecl)
			if vd == nil {
				continue
			}
			if len(vd.NameList) != 1 {
				check.errorf(vd.NameList[1], UnsupportedFeature, "SGO_E_UNADMITTED: a package-level table declares one name (SGO-BIND-003, D136)")
				continue
			}
			lit := sgoFreezeArg(vd.Values)
			if lit == nil {
				check.errorf(vd.NameList[0], UnsupportedFeature, "SGO_E_UNADMITTED: a package-level variable must be a table, var %s = sgo.Freeze(composite literal) (SGO-BIND-003, D136)", vd.NameList[0].Value)
				continue
			}
			check.sgoConstLit(lit)
		}
	}
}

// sgoFreezeArg returns the composite literal of sgo.Freeze(lit), or nil if e is not that call.
func sgoFreezeArg(e syntax.Expr) *syntax.CompositeLit {
	call, _ := syntax.Unparen(e).(*syntax.CallExpr)
	if call == nil || len(call.ArgList) != 1 || call.HasDots {
		return nil
	}
	fun := syntax.Unparen(call.Fun)
	if ix, _ := fun.(*syntax.IndexExpr); ix != nil {
		fun = syntax.Unparen(ix.X) // sgo.Freeze[T](lit)
	}
	sel, _ := fun.(*syntax.SelectorExpr)
	if sel == nil || sel.Sel.Value != "Freeze" {
		return nil
	}
	if pkg, _ := syntax.Unparen(sel.X).(*syntax.Name); pkg == nil || pkg.Value != "sgo" {
		return nil
	}
	lit, _ := syntax.Unparen(call.ArgList[0]).(*syntax.CompositeLit)
	return lit
}

// sgoConstLit reports every element or key of lit, at any depth, that is neither a constant nor
// a composite literal.
func (check *Checker) sgoConstLit(lit *syntax.CompositeLit) {
	for _, e := range lit.ElemList {
		if kv, _ := e.(*syntax.KeyValueExpr); kv != nil {
			if _, typed := check.Types[kv.Key]; typed {
				check.sgoConstElem(kv.Key) // an array, slice, or map key; a struct field name records no type
			}
			check.sgoConstElem(kv.Value)
			continue
		}
		check.sgoConstElem(e)
	}
}

func (check *Checker) sgoConstElem(e syntax.Expr) {
	if lit, _ := syntax.Unparen(e).(*syntax.CompositeLit); lit != nil {
		check.sgoConstLit(lit)
		return
	}
	if tv, ok := check.Types[e]; ok && tv.Value != nil {
		return
	}
	check.errorf(e, UnsupportedFeature, "SGO_E_UNADMITTED: a package-level table holds only constants and composite literals; %s is not a constant (SGO-BIND-003, D136)", syntax.String(e))
}

// sgoTableWrites reports, in a function body, every assignment to a package-level variable (of
// this package or an imported one), every increment or range assignment of one, and every
// address taken of one: package-level tables are written by no task (D136).
func (check *Checker) sgoTableWrites(fd *syntax.FuncDecl) {
	report := func(at syntax.Expr, what string) {
		if name := check.sgoPackageVarAt(at); name != "" {
			check.errorf(at, UnsupportedFeature, "SGO_E_UNADMITTED: %s %s: a package-level table cannot be written (SGO-BIND-003, D136)", what, name)
		}
	}
	syntax.Inspect(fd.Body, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.AssignStmt:
			lhs := []syntax.Expr{n.Lhs}
			if l, _ := n.Lhs.(*syntax.ListExpr); l != nil {
				lhs = l.ElemList
			}
			for _, e := range lhs {
				report(e, "assignment to")
			}
		case *syntax.RangeClause:
			if n.Lhs != nil && !n.Def {
				lhs := []syntax.Expr{n.Lhs}
				if l, _ := n.Lhs.(*syntax.ListExpr); l != nil {
					lhs = l.ElemList
				}
				for _, e := range lhs {
					report(e, "range assignment to")
				}
			}
		case *syntax.Operation:
			if n.Op == syntax.And && n.Y == nil {
				report(n.X, "the address of")
			}
		}
		return true
	})
}

// sgoPackageVarAt returns the name of the package-level variable that e is, or that x.F and x[i]
// forms of e are rooted in, or "".
func (check *Checker) sgoPackageVarAt(e syntax.Expr) string {
	for {
		switch x := syntax.Unparen(e).(type) {
		case *syntax.Name:
			if v, _ := check.Uses[x].(*Var); v != nil && v.pkg != nil && v.parent == v.pkg.scope {
				return x.Value
			}
			return ""
		case *syntax.SelectorExpr:
			if pn, _ := syntax.Unparen(x.X).(*syntax.Name); pn != nil {
				if _, isPkg := check.Uses[pn].(*PkgName); isPkg {
					if v, _ := check.Uses[x.Sel].(*Var); v != nil {
						return pn.Value + "." + x.Sel.Value
					}
					return ""
				}
			}
			e = x.X
		case *syntax.IndexExpr:
			e = x.X
		default:
			return ""
		}
	}
}
