// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The rules that keep task-local values in one task (SGO-COMP-004, D86, D94), as checked by the
// sharing-rule model (sgo/model/sharemodel.go):
//
//  1. A function literal that captures a variable of a task-local type, or a variable holding a
//     task-local function value, is task-local itself, and so is a local variable assigned one.
//     A task-local function value is only called, deferred, or assigned to a local variable; it
//     is not passed as an argument (in particular to Spawn), returned, or stored elsewhere.
//  2. A value of a task-local type is sent, committed through a permit, or sent in a select arm
//     only as a call sgo.Copy(...).
//  3. The address of a captured variable is not taken, explicitly or by calling a pointer method
//     on it (SGO-MEM-002: it would allow writing the variable).

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// sgoTaskLocalRules applies the rules to one function declaration with its function literals and
// captured variables (sgoCaptured).
func (check *Checker) sgoTaskLocalRules(fd *syntax.FuncDecl, lits []*syntax.FuncLit, captured map[*Var]*syntax.FuncLit) {
	// Variables each literal captures (declared outside it).
	caps := make(map[*syntax.FuncLit][]*Var, len(lits))
	for _, lit := range lits {
		start, end := syntax.StartPos(lit), syntax.EndPos(lit)
		seen := map[*Var]bool{}
		syntax.Inspect(lit.Body, func(n syntax.Node) bool {
			if name, _ := n.(*syntax.Name); name != nil {
				if v := check.sgoLocalVar(check.Uses[name]); v != nil && !seen[v] && (v.pos.Cmp(start) < 0 || v.pos.Cmp(end) > 0) {
					seen[v] = true
					caps[lit] = append(caps[lit], v)
				}
			}
			return true
		})
	}
	// Function-valued assignments: variable := expression.
	type funcAssign struct {
		v   *Var
		rhs syntax.Expr
	}
	var assigns []funcAssign
	addPairs := func(lhs []syntax.Expr, rhs []syntax.Expr, defs bool) {
		if len(lhs) != len(rhs) {
			return
		}
		for i, l := range lhs {
			name, _ := syntax.Unparen(l).(*syntax.Name)
			if name == nil {
				continue
			}
			obj := check.Uses[name]
			if defs {
				if d := check.Defs[name]; d != nil {
					obj = d
				}
			}
			if v := check.sgoLocalVar(obj); v != nil && isFuncType(v.typ) {
				assigns = append(assigns, funcAssign{v, rhs[i]})
			}
		}
	}
	syntax.Inspect(fd, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.AssignStmt:
			if n.Rhs != nil {
				addPairs(syntax.UnpackListExpr(n.Lhs), syntax.UnpackListExpr(n.Rhs), n.Op == syntax.Def)
			}
		case *syntax.VarDecl:
			lhs := make([]syntax.Expr, len(n.NameList))
			for i, nm := range n.NameList {
				lhs[i] = nm
			}
			addPairs(lhs, syntax.UnpackListExpr(n.Values), true)
		}
		return true
	})

	// Fixed point: task-local variables (by type, or holding task-local function values) and
	// task-local literals.
	localVar := func(v *Var) bool { return sgoTaskLocal(v.typ) }
	fnVars := map[*Var]bool{}
	localLit := map[*syntax.FuncLit]bool{}
	isLocalFunc := func(e syntax.Expr) bool {
		switch x := syntax.Unparen(e).(type) {
		case *syntax.FuncLit:
			return localLit[x]
		case *syntax.Name:
			v := check.sgoLocalVar(check.Uses[x])
			return v != nil && fnVars[v]
		}
		return false
	}
	for changed := true; changed; {
		changed = false
		for _, lit := range lits {
			if localLit[lit] {
				continue
			}
			for _, v := range caps[lit] {
				if localVar(v) || fnVars[v] {
					localLit[lit], changed = true, true
					break
				}
			}
		}
		for _, a := range assigns {
			if !fnVars[a.v] && isLocalFunc(a.rhs) {
				fnVars[a.v], changed = true, true
			}
		}
	}

	// Uses of task-local function values, the sends of rule 2, and the addresses of rule 3.
	var stack []syntax.Node
	syntax.Inspect(fd.Body, func(n syntax.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		parent := syntax.Node(nil)
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		stack = append(stack, n)
		switch n := n.(type) {
		case *syntax.FuncLit:
			if localLit[n] {
				check.sgoLocalFuncUse(n, parent, stack, sgoFirstLocalCapture(caps[n], localVar, fnVars))
			}
		case *syntax.Name:
			if v := check.sgoLocalVar(check.Uses[n]); v != nil && fnVars[v] {
				check.sgoLocalFuncUse(n, parent, stack, v.name)
			}
		case *syntax.SendStmt:
			check.sgoCopied(n.Value, "a send")
		case *syntax.CallExpr:
			if sel, _ := syntax.Unparen(n.Fun).(*syntax.SelectorExpr); sel != nil {
				if s := check.Selections[sel]; s != nil && s.kind == MethodVal {
					if s.obj.Name() == "Commit" && sgoIsPermit(s.recv) && len(n.ArgList) == 1 {
						check.sgoCopied(n.ArgList[0], "SendPermit.Commit")
					}
					check.sgoPointerMethodOnCaptured(sel, s, captured)
				}
			}
		case *syntax.Operation:
			if n.Op == syntax.And && n.Y == nil {
				if root := check.sgoAssignedRoot(n.X); root != nil {
					if v := check.sgoLocalVar(check.Uses[root]); v != nil && captured[v] != nil {
						check.errorf(n, UnassignableOperand, "SGO_E_CAPTURE_NOT_FINAL: the address of %s is taken, and it is captured by the function literal at %s (SGO-MEM-002, SGO-COMP-007)", root.Value, syntax.StartPos(captured[v]))
					}
				}
			}
		}
		return true
	})
}

// sgoFirstLocalCapture names the first captured variable that makes a literal task-local.
func sgoFirstLocalCapture(vs []*Var, localVar func(*Var) bool, fnVars map[*Var]bool) string {
	for _, v := range vs {
		if localVar(v) || fnVars[v] {
			return v.name
		}
	}
	return "?"
}

// sgoLocalFuncUse reports a task-local function value e used other than as the function of a
// call, the call of a defer, or the right-hand side of an assignment to a local variable.
func (check *Checker) sgoLocalFuncUse(e syntax.Node, parent syntax.Node, stack []syntax.Node, what string) {
	// Skip parentheses.
	i := len(stack) - 2
	for i >= 0 {
		if _, ok := stack[i].(*syntax.ParenExpr); !ok {
			break
		}
		i--
	}
	if i >= 0 {
		parent = stack[i]
	}
	switch p := parent.(type) {
	case *syntax.CallExpr:
		if syntax.Unparen(p.Fun) == e {
			return // a call
		}
		if sel, _ := syntax.Unparen(p.Fun).(*syntax.SelectorExpr); sel != nil {
			if s := check.Selections[sel]; s != nil {
				switch s.obj.Name() {
				case "Spawn", "SpawnWithout", "Start":
					if sgoIsNursery(s.recv) {
						check.errorf(e, UnsupportedFeature, "SGO_E_TASK_LOCAL_CAPTURE: the function passed to %s captures the task-local variable %s; pass a copy through a channel (sgo.Copy) or capture only value types (SGO-COMP-004)", s.obj.Name(), what)
						return
					}
				}
			}
		}
	case *syntax.AssignStmt:
		if p.Rhs != nil && sgoInList(p.Rhs, e) {
			return // stored in a local variable, which is task-local in turn
		}
	case *syntax.ListExpr:
		if i >= 1 {
			if a, ok := stack[i-1].(*syntax.AssignStmt); ok && a.Rhs == p {
				return
			}
			if d, ok := stack[i-1].(*syntax.VarDecl); ok && d.Values == p {
				return
			}
		}
	case *syntax.VarDecl:
		if p.Values != nil && sgoInList(p.Values, e) {
			return
		}
	}
	check.errorf(e, UnsupportedFeature, "SGO_E_TASK_LOCAL_CAPTURE: this function value captures the task-local variable %s; it can only be called, deferred, or kept in a local variable (SGO-COMP-004, D94)", what)
}

func sgoInList(list syntax.Expr, e syntax.Node) bool {
	for _, x := range syntax.UnpackListExpr(list) {
		if syntax.Unparen(x) == e || x == e {
			return true
		}
	}
	return false
}

// sgoCopied reports a value of a task-local type crossing tasks without sgo.Copy (rule 2).
func (check *Checker) sgoCopied(v syntax.Expr, what string) {
	tv, ok := check.Types[v]
	if !ok || tv.Type == nil || !sgoTaskLocal(tv.Type) {
		return
	}
	if call, _ := syntax.Unparen(v).(*syntax.CallExpr); call != nil {
		fun := syntax.Unparen(call.Fun)
		if ix, _ := fun.(*syntax.IndexExpr); ix != nil {
			fun = ix.X // sgo.Copy[T](x)
		}
		if isSgoSelector(fun, "Copy") {
			return
		}
	}
	check.errorf(v, UnsupportedFeature, "SGO_E_UNCOPIED_SEND: %s of a task-local value of type %s crosses to another task; send sgo.Copy(...) (SGO-COMP-004)", what, tv.Type)
}

// sgoPointerMethodOnCaptured reports a pointer-receiver method called on a captured variable
// (an implicit &x, rule 3).
func (check *Checker) sgoPointerMethodOnCaptured(sel *syntax.SelectorExpr, s *Selection, captured map[*Var]*syntax.FuncLit) {
	sig, _ := s.obj.Type().(*Signature)
	if sig == nil || sig.recv == nil {
		return
	}
	if _, ptrRecv := sig.recv.typ.(*Pointer); !ptrRecv {
		return
	}
	if _, recvIsPtr := s.recv.Underlying().(*Pointer); recvIsPtr {
		return // called through a pointer: no address is taken
	}
	root := check.sgoAssignedRoot(sel.X)
	if root == nil {
		return
	}
	if v := check.sgoLocalVar(check.Uses[root]); v != nil && captured[v] != nil {
		check.errorf(sel, UnassignableOperand, "SGO_E_CAPTURE_NOT_FINAL: %s takes the address of %s, which is captured by the function literal at %s (SGO-MEM-002, SGO-COMP-007)", syntax.String(sel), root.Value, syntax.StartPos(captured[v]))
	}
}

// sgoIsNamedSgo reports whether T is *sgo.<name> (or an instance of it).
func sgoIsNamedSgo(T Type, name string) bool {
	p, ok := Unalias(T).(*Pointer)
	if !ok {
		return false
	}
	n, ok := Unalias(p.base).(*Named)
	return ok && n.obj != nil && n.obj.pkg != nil && n.obj.pkg.path == "sgo" && n.obj.name == name
}

func sgoIsNursery(T Type) bool { return sgoIsNamedSgo(T, "Nursery") }
func sgoIsPermit(T Type) bool  { return sgoIsNamedSgo(T, "SendPermit") }

// sgoIfaceConversion reports a value of a task-local type converted to an interface type (error,
// the only admitted one): error values are shareable (captured by Spawn callbacks, sent without
// sgo.Copy), so only shareable types may become errors (SGO-COMP-004, W13 review V2).
func (check *Checker) sgoIfaceConversion(x *operand, T Type) {
	if !IsInterface(T) || x.typ() == nil || IsInterface(x.typ()) || x.isNil() {
		return
	}
	if sgoTaskLocal(x.typ()) {
		check.errorf(x, UnsupportedFeature, "SGO_E_TASK_LOCAL_CAPTURE: a value of the task-local type %s cannot become %s: error values are shared between tasks; only shareable types may implement error (SGO-COMP-004)", x.typ(), T)
	}
}
