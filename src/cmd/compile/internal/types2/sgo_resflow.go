// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// The resource flow rule (SGO-RES-003, D151): a value of a resource type is tied to the name of
// the scope it was acquired into and may not flow beyond that name's lexical extent. An extent
// is a function declaration (root and scope parameters), a function literal (a callback's task
// body), or a nursery statement. The analysis is conservative: a variable's value is tied to the
// extent declaring the variable (every store into it was checked against that), unless an earlier
// x.MoveTo(s) re-tied it to s's extent.

// sgoResKind reports whether T is a built-in resource handle (SGO-RES-001).
func sgoResKind(T Type) bool {
	if !isSgoHandle(T) {
		return false
	}
	switch Unalias(Unalias(T).(*Pointer).base).(*Named).obj.name {
	case "File", "Conn", "Listener", "Dialer", "ConnSlot":
		return true
	}
	return false
}

// sgoResType reports whether T is a resource type: a resource handle, a type with a method
// Release(bool) error (a user resource, SGO-RES-004), or a struct, array, or slice holding one.
func sgoResType(T Type) bool { return sgoResTypeRec(T, map[Type]bool{}) }

func sgoResTypeRec(T Type, seen map[Type]bool) bool {
	if T == nil {
		return false
	}
	T = Unalias(T)
	if seen[T] {
		return false
	}
	seen[T] = true
	if sgoResKind(T) || sgoHasRelease(T) {
		return true
	}
	switch u := T.Underlying().(type) {
	case *Struct:
		for _, f := range u.fields {
			if sgoResTypeRec(f.typ, seen) {
				return true
			}
		}
	case *Array:
		return sgoResTypeRec(u.elem, seen)
	case *Slice:
		return sgoResTypeRec(u.elem, seen)
	}
	return false
}

func sgoHasRelease(T Type) bool {
	for _, t := range []Type{T, NewPointer(T)} {
		obj, _, _ := LookupFieldOrMethod(t, false, nil, "Release")
		f, _ := obj.(*Func)
		if f == nil {
			continue
		}
		sig := f.Signature()
		if sig.params.Len() == 1 && sig.results.Len() == 1 && isBoolean(sig.params.vars[0].typ) &&
			Identical(sig.results.vars[0].typ, universeError) {
			return true
		}
	}
	return false
}

type sgoResFlow struct {
	check  *Checker
	parent map[syntax.Node]syntax.Node // extent -> enclosing extent
	home   map[Object]syntax.Node      // variable or scope name -> extent declaring it
	scope  map[Object]syntax.Node      // scope name (nursery binder, parameter) -> its extent
	moved  map[Object]syntax.Node      // variable -> extent of its last MoveTo target
	ext    []syntax.Node               // current extent chain
}

// within reports whether extent a lies within (or is) extent b.
func (f *sgoResFlow) within(a, b syntax.Node) bool {
	for ; a != nil; a = f.parent[a] {
		if a == b {
			return true
		}
	}
	return false
}

func (f *sgoResFlow) cur() syntax.Node { return f.ext[len(f.ext)-1] }

// inner returns the innermost of two extents on one chain.
func (f *sgoResFlow) inner(a, b syntax.Node) syntax.Node {
	if f.within(a, b) {
		return a
	}
	return b
}

// scopeExtent returns the extent a scope expression names, or nil.
func (f *sgoResFlow) scopeExtent(e syntax.Expr) syntax.Node {
	if n, _ := syntax.Unparen(e).(*syntax.Name); n != nil {
		if obj := f.check.Uses[n]; obj != nil {
			if x := f.scope[obj]; x != nil {
				return x
			}
			if f.isScopeType(obj.Type()) {
				return f.ext[0] // root or another scope value: the function
			}
		}
	}
	return nil
}

func (f *sgoResFlow) isScopeType(T Type) bool {
	if isSgoHandle(T) {
		switch Unalias(Unalias(T).(*Pointer).base).(*Named).obj.name {
		case "Context", "Nursery":
			return true
		}
	}
	if n, _ := Unalias(T).(*Named); n != nil && n.obj != nil && n.obj.pkg != nil && n.obj.pkg.path == "sgo" && n.obj.name == "Scope" {
		return true
	}
	return false
}

// tie returns the extent the value of e is tied to.
func (f *sgoResFlow) tie(e syntax.Expr) syntax.Node {
	switch e := syntax.Unparen(e).(type) {
	case *syntax.Name:
		if obj := f.check.Uses[e]; obj != nil {
			if x := f.moved[obj]; x != nil {
				return x
			}
			if x := f.home[obj]; x != nil {
				return x
			}
		}
		return f.ext[0] // a package-level name or a constant: no resource can be stored there
	case *syntax.SelectorExpr:
		if _, isPkg := f.check.Uses[e.Sel].(*Var); isPkg {
			if n, _ := e.X.(*syntax.Name); n != nil {
				if _, pkg := f.check.Uses[n].(*PkgName); pkg {
					return f.ext[0]
				}
			}
		}
		return f.tie(e.X)
	case *syntax.IndexExpr:
		return f.tie(e.X)
	case *syntax.SliceExpr:
		return f.tie(e.X)
	case *syntax.Operation:
		if e.Y == nil {
			return f.tie(e.X)
		}
	case *syntax.CompositeLit:
		t := f.ext[0]
		for _, el := range e.ElemList {
			if kv, _ := el.(*syntax.KeyValueExpr); kv != nil {
				el = kv.Value
			}
			if f.isRes(el) {
				t = f.inner(f.tie(el), t)
			}
		}
		return t
	case *syntax.CallExpr:
		// An acquisition is tied to its scope argument or receiver; another call's resource
		// results are tied to the caller's scope, as narrowly as the extent of the call.
		var args []syntax.Expr
		if sel, _ := syntax.Unparen(e.Fun).(*syntax.SelectorExpr); sel != nil {
			args = append(args, sel.X)
		}
		for _, a := range append(args, e.ArgList...) {
			if x := f.scopeExtent(a); x != nil {
				return x
			}
		}
	}
	return f.cur()
}

func (f *sgoResFlow) isRes(e syntax.Expr) bool {
	tv, ok := f.check.Types[e]
	return ok && tv.Type != nil && sgoResTypeRec(tv.Type, map[Type]bool{})
}

func (f *sgoResFlow) errorf(at syntax.Node, format string, args ...any) {
	f.check.errorf(at, UnsupportedFeature, "SGO_E_RESOURCE_ESCAPES: "+format+" (SGO-RES-003)", args...)
}

// sgoResourceFlow checks fd against the resource flow rule.
func (check *Checker) sgoResourceFlow(fd *syntax.FuncDecl) {
	if fd.Body == nil {
		return
	}
	f := &sgoResFlow{check: check, parent: map[syntax.Node]syntax.Node{}, home: map[Object]syntax.Node{},
		scope: map[Object]syntax.Node{}, moved: map[Object]syntax.Node{}}
	f.ext = []syntax.Node{fd}
	f.params(fd.Type, fd)
	var stack []syntax.Node
	syntax.Inspect(fd.Body, func(n syntax.Node) bool {
		if n == nil {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if top == f.cur() {
				f.ext = f.ext[:len(f.ext)-1]
			}
			return true
		}
		stack = append(stack, n)
		switch n := n.(type) {
		case *syntax.FuncLit:
			f.parent[n] = f.cur()
			f.ext = append(f.ext, n)
			f.params(n.Type, n)
		case *syntax.NurseryStmt:
			f.parent[n] = f.cur()
			f.ext = append(f.ext, n)
			if n.Name != nil {
				if obj := check.Defs[n.Name]; obj != nil {
					f.scope[obj], f.home[obj] = n, n
				}
			}
		case *syntax.Name:
			if obj := check.Defs[n]; obj != nil {
				f.home[obj] = f.cur()
			} else if obj, _ := check.Uses[n].(*Var); obj != nil && f.home[obj] != nil && sgoResType(obj.Type()) {
				f.capture(n, obj, stack)
			}
		case *syntax.AssignStmt:
			f.assign(n)
		case *syntax.ReturnStmt:
			for _, r := range sgoList(n.Results) {
				if f.isRes(r) && !f.within(f.funcExtent(), f.tie(r)) {
					f.errorf(r, "%s is tied to a scope that ends before this function returns; return it only from the scope's own function, or MoveTo an enclosing scope", syntax.String(r))
				}
			}
		case *syntax.SendStmt:
			if f.isRes(n.Value) {
				f.errorf(n.Value, "%s holds a resource and cannot be sent on a channel; resources are shared by scope, not by message", syntax.String(n.Value))
			}
		case *syntax.CallExpr:
			f.call(n)
		}
		return true
	})
}

// params records a function's parameters: scope parameters name the function's extent.
func (f *sgoResFlow) params(t *syntax.FuncType, ext syntax.Node) {
	for _, p := range t.ParamList {
		if p.Name == nil {
			continue
		}
		if obj := f.check.Defs[p.Name]; obj != nil {
			f.home[obj] = ext
			if f.isScopeType(obj.Type()) {
				f.scope[obj] = ext
			}
		}
	}
}

func (f *sgoResFlow) funcExtent() syntax.Node {
	for i := len(f.ext) - 1; i >= 0; i-- {
		switch f.ext[i].(type) {
		case *syntax.FuncLit, *syntax.FuncDecl:
			return f.ext[i]
		}
	}
	return f.ext[0]
}

// rootVar returns the variable an assignment target stores into, or nil.
func (f *sgoResFlow) rootVar(e syntax.Expr) Object {
	for {
		switch x := syntax.Unparen(e).(type) {
		case *syntax.Name:
			if obj := f.check.Defs[x]; obj != nil {
				return obj
			}
			return f.check.Uses[x]
		case *syntax.SelectorExpr:
			e = x.X
		case *syntax.IndexExpr:
			e = x.X
		case *syntax.Operation:
			e = x.X
		default:
			return nil
		}
	}
}

func (f *sgoResFlow) assign(a *syntax.AssignStmt) {
	lhs, rhs := sgoList(a.Lhs), sgoList(a.Rhs)
	for i, l := range lhs {
		var r syntax.Expr
		switch {
		case len(rhs) == len(lhs):
			r = rhs[i]
		case len(rhs) == 1:
			r = rhs[0] // a multi-value call: its results share the call's tie
		default:
			continue
		}
		tv, ok := f.check.Types[l]
		var T Type
		if ok {
			T = tv.Type
		} else if n, _ := l.(*syntax.Name); n != nil {
			if obj := f.check.Defs[n]; obj != nil {
				T = obj.Type()
			}
		}
		if T == nil || !sgoResType(T) {
			continue
		}
		obj := f.rootVar(l)
		if obj == nil {
			continue
		}
		home := f.home[obj]
		if a.Op == syntax.Def && f.check.Defs[l.(*syntax.Name)] != nil {
			home = f.cur()
		}
		if home == nil {
			f.errorf(l, "%s is not a local variable; a resource lives only in its scope", syntax.String(l))
			continue
		}
		if !f.within(home, f.tie(r)) {
			f.errorf(r, "%s is tied to a scope that ends before %s does; acquire it into the outer scope, or MoveTo that scope first", syntax.String(r), syntax.String(l))
		}
		delete(f.moved, obj)
	}
}

// call checks sgo.Copy of a resource and records x.MoveTo(s).
func (f *sgoResFlow) call(c *syntax.CallExpr) {
	sel, _ := syntax.Unparen(c.Fun).(*syntax.SelectorExpr)
	if sel == nil {
		return
	}
	if fn, _ := f.check.Uses[sel.Sel].(*Func); fn != nil && fn.pkg != nil && fn.pkg.path == "sgo" {
		switch {
		case fn.name == "Copy" && fn.Signature().recv == nil:
			for _, a := range c.ArgList {
				if f.isRes(a) {
					f.errorf(a, "%s holds a resource and cannot be copied with sgo.Copy", syntax.String(a))
				}
			}
		case fn.name == "MoveTo" && len(c.ArgList) == 1:
			if n, _ := syntax.Unparen(sel.X).(*syntax.Name); n != nil {
				if obj := f.check.Uses[n]; obj != nil {
					if x := f.scopeExtent(c.ArgList[0]); x != nil && f.within(f.cur(), x) {
						f.moved[obj] = x
					}
				}
			}
		}
	}
}

// capture checks a use of resource variable obj: every function literal between the use and the
// variable's extent must be a spawned callback (an argument of a call), so it ends inside the
// scope that releases the resource.
func (f *sgoResFlow) capture(n *syntax.Name, obj *Var, stack []syntax.Node) {
	home := f.home[obj]
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == home {
			return
		}
		lit, _ := stack[i].(*syntax.FuncLit)
		if lit == nil {
			continue
		}
		p, _ := sgoParent(stack, i)
		if call, _ := p.(*syntax.CallExpr); call == nil || syntax.Unparen(call.Fun) == lit {
			f.errorf(n, "the function literal captures resource %s and may outlive its scope; only a callback spawned inside the scope may capture it", n.Value)
			return
		}
	}
}
