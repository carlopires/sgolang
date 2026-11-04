// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

// check c: checked propagation (SGO-ERR-004, D147).
//
// The value of check c is the results of the call c except the last, which is an error: no value,
// one value, or a tuple. Where check may stand (an expression statement, or the whole right side of
// an assignment or short declaration, not in an init or post statement) is checked over the
// function bodies by sgoCheckPlacement. The noder lowers it to the comparison and return it stands
// for (noder: sgoCheckStmt).

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

func (check *Checker) sgoCheckExpr(x *operand, e *syntax.CheckExpr) exprKind {
	call, _ := syntax.Unparen(e.X).(*syntax.CallExpr)
	if call == nil {
		check.rawExpr(nil, x, e.X, nil, false)
		check.errorf(e, InvalidCall, "SGO_E_CHECK_NOT_ERROR: check applies to a call whose last result is an error (SGO-ERR-004)")
		x.invalidate()
		x.expr = e
		return expression
	}
	check.rawExpr(nil, x, call, nil, false)
	if !x.isValid() {
		x.expr = e
		return expression
	}
	var results []*Var
	switch t := x.typ().(type) {
	case *Tuple:
		results = t.vars
	default:
		if x.mode() != novalue {
			results = []*Var{NewParam(call.Pos(), nil, "", t)}
		}
	}
	if len(results) == 0 || !Identical(results[len(results)-1].typ, universeError) {
		check.errorf(e, InvalidCall, "SGO_E_CHECK_NOT_ERROR: check applies to a call whose last result is an error (SGO-ERR-004)")
		x.invalidate()
		x.expr = e
		return expression
	}
	if sig := check.sig; sig == nil || sig.results.Len() == 0 || !Identical(sig.results.At(sig.results.Len()-1).typ, universeError) {
		check.errorf(e, InvalidCall, "SGO_E_CHECK_NO_RESULT: check needs an enclosing function whose last result is an error (SGO-ERR-004)")
		x.invalidate()
		x.expr = e
		return expression
	}
	rest := results[:len(results)-1]
	x.expr = e
	switch len(rest) {
	case 0:
		x.mode_ = novalue
		x.typ_ = nil
		return statement
	case 1:
		x.mode_ = value
		x.typ_ = rest[0].typ
	default:
		x.mode_ = value
		x.typ_ = NewTuple(rest...)
	}
	return expression
}

// sgoCheckPlacement reports every check expression of fd (function literals included) that is not
// an expression statement or the whole right side of a plain assignment or short declaration in a
// statement list. Init and post statements are not in statement lists, so check is denied there.
func (check *Checker) sgoCheckPlacement(fd *syntax.FuncDecl) {
	ok := make(map[*syntax.CheckExpr]bool)
	mark := func(list []syntax.Stmt) {
		for _, s := range list {
			for {
				l, _ := s.(*syntax.LabeledStmt)
				if l == nil {
					break
				}
				s = l.Stmt
			}
			switch s := s.(type) {
			case *syntax.ExprStmt:
				if ce, _ := syntax.Unparen(s.X).(*syntax.CheckExpr); ce != nil {
					ok[ce] = true
				}
			case *syntax.AssignStmt:
				if s.Rhs != nil && (s.Op == 0 || s.Op == syntax.Def) {
					if ce, _ := syntax.Unparen(s.Rhs).(*syntax.CheckExpr); ce != nil {
						ok[ce] = true
					}
				}
			}
		}
	}
	syntax.Inspect(fd, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.BlockStmt:
			mark(n.List)
		case *syntax.CaseClause:
			mark(n.Body)
		case *syntax.CommClause:
			mark(n.Body)
		}
		return true
	})
	syntax.Inspect(fd, func(n syntax.Node) bool {
		if ce, _ := n.(*syntax.CheckExpr); ce != nil && !ok[ce] {
			check.errorf(ce, UnsupportedFeature, "SGO_E_UNADMITTED: check is admitted only as a statement or as the whole right side of an assignment, not in an expression, a condition, an argument, a return, or an init or post statement (SGO-ERR-004)")
		}
		return true
	})
}

// sgoAfterPlacement reports every call of (*sgo.Context).After in fd that is not the operand of a
// select receive arm, case <-t.After(ms): or case v := <-t.After(ms): (D160, SGO-WAIT-003).
func (check *Checker) sgoAfterPlacement(fd *syntax.FuncDecl) {
	ok := make(map[*syntax.CallExpr]bool)
	syntax.Inspect(fd, func(n syntax.Node) bool {
		cc, _ := n.(*syntax.CommClause)
		if cc == nil || cc.Comm == nil {
			return true
		}
		var x syntax.Expr
		switch s := cc.Comm.(type) {
		case *syntax.ExprStmt:
			x = s.X
		case *syntax.AssignStmt:
			x = s.Rhs
		}
		if op, _ := syntax.Unparen(x).(*syntax.Operation); op != nil && op.Op == syntax.Recv && op.Y == nil {
			if c, _ := syntax.Unparen(op.X).(*syntax.CallExpr); c != nil {
				ok[c] = true
			}
		}
		return true
	})
	syntax.Inspect(fd, func(n syntax.Node) bool {
		c, _ := n.(*syntax.CallExpr)
		if c == nil || ok[c] || !check.sgoIsAfter(c) {
			return true
		}
		check.errorf(c, UnsupportedFeature, "SGO_E_UNADMITTED: t.After is admitted only as the operand of a select receive arm, case <-t.After(ms): (SGO-WAIT-003)")
		return true
	})
}

// sgoIsAfter reports whether c calls the method After of sgo.Context.
func (check *Checker) sgoIsAfter(c *syntax.CallExpr) bool {
	sel, _ := syntax.Unparen(c.Fun).(*syntax.SelectorExpr)
	if sel == nil || sel.Sel.Value != "After" {
		return false
	}
	var obj Object
	if s := check.Selections[sel]; s != nil {
		obj = s.obj
	} else if check.Uses != nil {
		obj = check.Uses[sel.Sel]
	}
	f, _ := obj.(*Func)
	if f == nil || f.pkg == nil || f.pkg.path != "sgo" {
		return false
	}
	sig, _ := f.typ.(*Signature)
	return sig != nil && sig.recv != nil
}

// sgoGoCalls checks the result type T of every sgo.Go call in fd, inferred or written: the result
// is delivered to the nursery's tasks over a channel, so T is a channel element (D161).
func (check *Checker) sgoGoCalls(fd *syntax.FuncDecl) {
	syntax.Inspect(fd, func(n syntax.Node) bool {
		c, _ := n.(*syntax.CallExpr)
		if c == nil {
			return true
		}
		fun := syntax.Unparen(c.Fun)
		if ix, _ := fun.(*syntax.IndexExpr); ix != nil {
			fun = ix.X
		}
		sel, _ := fun.(*syntax.SelectorExpr)
		if sel == nil {
			return true
		}
		fn, _ := check.Uses[sel.Sel].(*Func)
		if fn == nil || fn.pkg == nil || fn.pkg.path != "sgo" || fn.name != "Go" {
			return true
		}
		tv, ok := check.Types[c]
		if !ok || tv.Type == nil {
			return true
		}
		p, _ := tv.Type.(*Pointer)
		if p == nil {
			return true
		}
		if nt, _ := p.base.(*Named); nt != nil && nt.TypeArgs().Len() == 1 {
			check.sgoChanElemOK(c, nt.TypeArgs().At(0), false)
		}
		return true
	})
}
