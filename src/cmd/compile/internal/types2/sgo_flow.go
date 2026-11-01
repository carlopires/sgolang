// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// S-Go flow rules (SGO_SPEC_SUBSET0.md). They run once all function bodies are checked, over the
// syntax of each function declaration, using the objects types2 recorded in Defs and Uses, so a
// name is resolved exactly as the type checker resolved it. Every walk is in source order, which
// keeps the diagnostics deterministic.

// sgoPredeclared are the predeclared identifiers of S-Go subset 0 (SGO-BIND-004). Other universe
// names of Go are not identifiers of the S-Go universe and may be declared.
var sgoPredeclared = map[string]bool{
	"bool": true, "int": true, "string": true, "error": true,
	"true": true, "false": true, "nil": true, "len": true, "panic": true,
}

// sgoEnsureInfo makes sure the maps the flow rules read are recorded.
func (check *Checker) sgoEnsureInfo() {
	if check.Types == nil {
		check.Types = make(map[syntax.Expr]TypeAndValue)
	}
	if check.Defs == nil {
		check.Defs = make(map[*syntax.Name]Object)
	}
	if check.Uses == nil {
		check.Uses = make(map[*syntax.Name]Object)
	}
}

// sgoLocalScope reports whether s is a function or block scope (not the universe, a package, or
// a file scope).
func (check *Checker) sgoLocalScope(s *Scope) bool {
	return s != nil && s != Universe && s != check.pkg.scope && s.parent != check.pkg.scope
}

// sgoLocalVar returns obj as a local variable (parameters, results, and nursery binders included).
func (check *Checker) sgoLocalVar(obj Object) *Var {
	v, _ := obj.(*Var)
	if v == nil || !check.sgoLocalScope(v.parent) {
		return nil
	}
	return v
}

// sgoFlowChecks applies the flow rules to every function declaration.
func (check *Checker) sgoFlowChecks(files []*syntax.File) {
	for _, f := range files {
		for _, d := range f.DeclList {
			if fd, _ := d.(*syntax.FuncDecl); fd != nil && fd.Body != nil {
				check.sgoFuncFlow(fd)
			}
		}
	}
}

func (check *Checker) sgoFuncFlow(fd *syntax.FuncDecl) {
	var lits []*syntax.FuncLit
	binders := make(map[*Var]bool)
	syntax.Inspect(fd, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.Name:
			check.sgoShadow(n)
		case *syntax.DeclStmt:
			check.sgoUninitialized(n)
		case *syntax.ExprStmt:
			if call, _ := syntax.Unparen(n.X).(*syntax.CallExpr); call != nil {
				check.sgoDiscarded(call, syntax.StartPos(n.X))
			}
		case *syntax.CallStmt:
			if call, _ := syntax.Unparen(n.Call).(*syntax.CallExpr); call != nil && n.Tok == syntax.Defer {
				check.sgoDiscarded(call, n)
			}
		case *syntax.BlockStmt:
			check.sgoUnreachable(n.List)
		case *syntax.CaseClause:
			check.sgoUnreachable(n.Body)
		case *syntax.FuncLit:
			lits = append(lits, n)
		case *syntax.CallExpr:
			check.sgoPanicArg(n)
			check.sgoChanLen(n)
		case *syntax.ForStmt:
			check.sgoRange(n)
		case *syntax.FuncType:
			check.sgoChanResults(n)
		case *syntax.NurseryStmt:
			if n.Name != nil {
				if v := check.sgoLocalVar(check.Defs[n.Name]); v != nil {
					binders[v] = true
				}
			}
		}
		return true
	})
	check.sgoAssignments(fd, check.sgoCaptured(lits), binders)
}

// sgoShadow reports a declaration that hides a name of an enclosing block of the same function,
// a predeclared identifier, an imported package name, or a package-level declaration
// (SGO-BIND-004). A function literal is a function of its own: its declarations may reuse names
// of the enclosing function.
func (check *Checker) sgoShadow(name *syntax.Name) {
	obj := check.Defs[name]
	if name.Value == "_" || obj == nil {
		return
	}
	switch obj.(type) {
	case *Var, *Const:
	default:
		return
	}
	if !check.sgoLocalScope(obj.Parent()) {
		return
	}
	inFunc := !obj.Parent().isFunc // still inside the declaring function's scopes
	for s := obj.Parent().parent; s != nil; s = s.parent {
		local := check.sgoLocalScope(s)
		if local && !inFunc {
			continue // an enclosing function: not "within one function"
		}
		if local && s.isFunc {
			inFunc = false // s is the declaring function's own scope; outer local scopes are excluded
		}
		alt := s.Lookup(name.Value)
		if alt == nil {
			continue
		}
		switch {
		case s == Universe:
			if sgoPredeclared[name.Value] {
				check.errorf(name, DuplicateDecl, "SGO_E_SHADOW: %s shadows the predeclared identifier %s (SGO-BIND-004)", name.Value, name.Value)
			}
		case local:
			if alt.Pos().Cmp(obj.Pos()) > 0 {
				continue // declared later: not in scope here
			}
			check.errorf(name, DuplicateDecl, "SGO_E_SHADOW: %s shadows the declaration at %s (SGO-BIND-004)", name.Value, alt.Pos())
		default:
			check.errorf(name, DuplicateDecl, "SGO_E_SHADOW: %s shadows the package-level or imported name declared at %s (SGO-BIND-004)", name.Value, alt.Pos())
		}
		return
	}
}

// sgoUninitialized reports var x T without an initializer (SGO-BIND-003).
func (check *Checker) sgoUninitialized(s *syntax.DeclStmt) {
	for _, d := range s.DeclList {
		if v, _ := d.(*syntax.VarDecl); v != nil && v.Values == nil {
			check.errorf(s, UnsupportedFeature, "SGO_E_UNINITIALIZED: a variable declaration must have an initializer (SGO-BIND-003)")
			return
		}
	}
}

// sgoDiscarded reports a call statement (or deferred call) whose results include an error
// (SGO-CTL-001, SGO-DEF-001); _ = f() is the explicit discard.
func (check *Checker) sgoDiscarded(call *syntax.CallExpr, at poser) {
	tv, ok := check.Types[call]
	if !ok || tv.Type == nil {
		return
	}
	has := Identical(tv.Type, universeError)
	if t, _ := tv.Type.(*Tuple); t != nil {
		for _, v := range t.vars {
			has = has || Identical(v.typ, universeError)
		}
	}
	if has {
		check.errorf(at, UnusedResults, "SGO_E_DISCARDED_ERROR: the error result of %s is not used; write _ = ... to discard it (SGO-CTL-001)", call.Fun)
	}
}

// sgoPanicArg reports a call of the predeclared panic whose argument is not a string
// (SGO-DEF-002).
func (check *Checker) sgoPanicArg(call *syntax.CallExpr) {
	name, _ := syntax.Unparen(call.Fun).(*syntax.Name)
	if name == nil || len(call.ArgList) != 1 {
		return
	}
	if b, _ := check.Uses[name].(*Builtin); b == nil || b.id != _Panic {
		return
	}
	arg := call.ArgList[0]
	if tv, ok := check.Types[arg]; ok && tv.Type != nil && !isString(tv.Type) {
		check.errorf(arg, InvalidCall, "SGO_E_TYPE_MISMATCH: panic takes a string argument, not %s (SGO-DEF-002)", tv.Type)
	}
}

// sgoUnreachable reports the first statement that follows a terminating statement (SGO-CTL-003).
func (check *Checker) sgoUnreachable(list []syntax.Stmt) {
	for i, s := range list {
		if i+1 < len(list) && check.isTerminating(s, "") {
			check.errorf(syntax.StartPos(list[i+1]), UnsupportedFeature, "SGO_E_UNREACHABLE: statement follows a terminating statement (SGO-CTL-003)")
			return
		}
	}
}

// sgoCaptured maps each local variable referenced inside a function literal, but declared
// outside it, to the first literal capturing it (SGO-MEM-002).
func (check *Checker) sgoCaptured(lits []*syntax.FuncLit) map[*Var]*syntax.FuncLit {
	captured := make(map[*Var]*syntax.FuncLit)
	for _, lit := range lits {
		start, end := syntax.StartPos(lit), syntax.EndPos(lit)
		syntax.Inspect(lit.Body, func(n syntax.Node) bool {
			if name, _ := n.(*syntax.Name); name != nil {
				v := check.sgoLocalVar(check.Uses[name])
				if v != nil && (v.pos.Cmp(start) < 0 || v.pos.Cmp(end) > 0) && captured[v] == nil {
					captured[v] = lit
				}
			}
			return true
		})
	}
	return captured
}

// sgoAssignments reports assignments to nursery binders (SGO-NUR-001) and to captured variables
// (SGO-MEM-002), at the assigned name.
func (check *Checker) sgoAssignments(fd *syntax.FuncDecl, captured map[*Var]*syntax.FuncLit, binders map[*Var]bool) {
	syntax.Inspect(fd.Body, func(n syntax.Node) bool {
		a, _ := n.(*syntax.AssignStmt)
		if a == nil {
			return true
		}
		lhs := []syntax.Expr{a.Lhs}
		if l, _ := a.Lhs.(*syntax.ListExpr); l != nil {
			lhs = l.ElemList
		}
		for _, e := range lhs {
			name, _ := syntax.Unparen(e).(*syntax.Name)
			if name == nil {
				continue
			}
			v := check.sgoLocalVar(check.Uses[name])
			switch {
			case v == nil:
			case binders[v]:
				check.errorf(name, UnassignableOperand, "SGO_E_ASSIGN_BINDER: the nursery binder %s cannot be assigned (SGO-NUR-001)", name.Value)
			case captured[v] != nil:
				check.errorf(name, UnassignableOperand, "SGO_E_CAPTURE_NOT_FINAL: %s is captured by the function literal at %s and must not be assigned (SGO-MEM-002)", name.Value, syntax.StartPos(captured[v]))
			}
		}
		return true
	})
}

// sgoChanLen reports len of a channel: instantaneous counts are denied in 0.3.0 (SGO-CHAN-002).
func (check *Checker) sgoChanLen(call *syntax.CallExpr) {
	name, _ := syntax.Unparen(call.Fun).(*syntax.Name)
	if name == nil || len(call.ArgList) != 1 {
		return
	}
	if b, _ := check.Uses[name].(*Builtin); b == nil || b.id != _Len {
		return
	}
	if tv, ok := check.Types[call.ArgList[0]]; ok && tv.Type != nil && isChanType(tv.Type) {
		check.errorf(name, UnsupportedFeature, "SGO_E_UNADMITTED: len of a channel is not admitted (SGO-CHAN-002)")
	}
}

// sgoRange reports a range clause whose operand is not a channel: only channel range is admitted
// (SGO-CHAN-002; other range forms stay denied by SGO-COMP-001).
func (check *Checker) sgoRange(s *syntax.ForStmt) {
	r, _ := s.Init.(*syntax.RangeClause)
	if r == nil {
		return
	}
	if tv, ok := check.Types[r.X]; ok && tv.Type != nil && isChanType(tv.Type) {
		return
	}
	check.errorf(r.Pos(), UnsupportedFeature, "SGO_E_UNADMITTED: range clause is not admitted in S-Go subset 0 (SGO-COMP-001)")
}

// sgoChanResults reports a named result of channel type: it would start as a nil channel, and
// channels have no nil value (SGO-CHAN-002).
func (check *Checker) sgoChanResults(t *syntax.FuncType) {
	for _, f := range t.ResultList {
		if f.Name == nil {
			continue
		}
		if tv, ok := check.Types[f.Type]; ok && tv.Type != nil && isChanType(tv.Type) {
			check.errorf(f.Name, UnsupportedFeature, "SGO_E_UNINITIALIZED: a named result of channel type would start as a nil channel (SGO-CHAN-002)")
		}
	}
}
