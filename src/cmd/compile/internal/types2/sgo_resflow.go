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
	case "File", "Conn", "Listener", "Dialer":
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
	if isSgoHandle(T) {
		// *sgo.Channel[E] and *sgo.SendPermit[E] holding resources (D158).
		n := Unalias(Unalias(T).(*Pointer).base).(*Named)
		if n.obj.name == "Channel" || n.obj.name == "SendPermit" {
			for _, a := range n.TypeArgs().list() {
				if sgoResTypeRec(a, seen) {
					return true
				}
			}
		}
		return false
	}
	switch u := T.Underlying().(type) {
	case *Chan:
		return sgoResTypeRec(u.elem, seen)
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

// sgoParamExt is the extent a resource parameter is tied to: some scope of the caller, enclosing
// the function, but not comparable with another parameter's.
type sgoParamExt struct{ syntax.Node }

type sgoResFlow struct {
	check  *Checker
	parent map[syntax.Node]syntax.Node // extent -> enclosing extent
	home   map[Object]syntax.Node      // variable or scope name -> extent declaring it
	scope  map[Object]syntax.Node      // scope name (nursery binder, parameter) -> its extent
	moved  map[Object]syntax.Node      // variable -> extent of its last MoveTo target
	ext    []syntax.Node               // current extent chain
	// Fixpoint state, kept across passes: a variable's value tie (the narrowest tie of what is
	// stored in it: how long its contents surely live) and its destination tie (the widest tie
	// of a channel or permit stored in it: how long it may live as a send destination).
	val, dst map[Object]syntax.Node
	pseudo   map[Object]*sgoParamExt // each resource parameter's extent, stable across passes
	top      syntax.Node             // the tie of a value holding no resource: it may flow anywhere
	unknown  syntax.Node             // a destination of unknown lifetime: nothing holding a resource is sent on it
	changed  bool
	report   bool
}

// within reports whether extent a lies within (or is) extent b.
func (f *sgoResFlow) within(a, b syntax.Node) bool {
	if b == f.top {
		return true
	}
	if a == f.top {
		return false
	}
	if _, pa := a.(*sgoParamExt); pa {
		return a == b
	}
	if _, pb := b.(*sgoParamExt); pb {
		return f.within(a, f.ext[0])
	}
	for ; a != nil; a = f.parent[a] {
		if a == b {
			return true
		}
	}
	return false
}

// join returns the widest of two extents, or unknown if they are incomparable.
func (f *sgoResFlow) join(a, b syntax.Node) syntax.Node {
	switch {
	case a == f.unknown || b == f.unknown:
		return f.unknown
	case f.within(a, b):
		return b
	case f.within(b, a):
		return a
	}
	return f.unknown
}

func (f *sgoResFlow) setVal(obj Object, t syntax.Node) {
	if old, ok := f.val[obj]; ok {
		t = f.inner(old, t)
		if t == old {
			return
		}
	}
	f.val[obj], f.changed = t, true
}

func (f *sgoResFlow) setDst(obj Object, t syntax.Node) {
	if old, ok := f.dst[obj]; ok {
		t = f.join(old, t)
		if t == old {
			return
		}
	}
	f.dst[obj], f.changed = t, true
}

// dest returns an upper bound on how long the channel or permit e lives.
func (f *sgoResFlow) dest(e syntax.Expr) syntax.Node {
	switch e := syntax.Unparen(e).(type) {
	case *syntax.CheckExpr:
		return f.dest(e.X)
	case *syntax.Name:
		if obj := f.check.Uses[e]; obj != nil {
			if d, ok := f.dst[obj]; ok {
				return d
			}
			if h, param := f.home[obj].(*sgoParamExt); param {
				return h
			}
		}
	case *syntax.CallExpr:
		fun := syntax.Unparen(e.Fun)
		if ix, _ := fun.(*syntax.IndexExpr); ix != nil {
			fun = ix.X // sgo.NewChannel[T]
		}
		var id *syntax.Name
		switch x := fun.(type) {
		case *syntax.Name:
			id = x
		case *syntax.SelectorExpr:
			id = x.Sel
			if fn, _ := f.check.Uses[id].(*Func); fn != nil && fn.pkg != nil && fn.pkg.path == "sgo" && fn.name == "Reserve" {
				return f.dest(x.X) // a permit lives as long as its channel
			}
		}
		if id != nil {
			switch obj := f.check.Uses[id].(type) {
			case *Builtin:
				if obj.id == _Make {
					return f.cur()
				}
			case *Func:
				if obj.pkg != nil && obj.pkg.path == "sgo" && obj.name == "NewChannel" {
					return f.cur()
				}
			}
		}
	}
	return f.unknown
}

func (f *sgoResFlow) cur() syntax.Node { return f.ext[len(f.ext)-1] }

// inner returns the innermost of two extents on one chain.
func (f *sgoResFlow) inner(a, b syntax.Node) syntax.Node {
	switch {
	case f.within(a, b):
		return a
	case f.within(b, a):
		return b
	}
	return f.ext[0] // two parameters' scopes: the function lies within both
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
	case *syntax.CheckExpr:
		return f.tie(e.X)
	case *syntax.Name:
		if obj := f.check.Uses[e]; obj != nil {
			if x := f.moved[obj]; x != nil {
				return x
			}
			if x, ok := f.val[obj]; ok {
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
		t := f.top // no resource element: the value holds none
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
		sel, _ := syntax.Unparen(e.Fun).(*syntax.SelectorExpr)
		if sel == nil {
			break
		}
		fn, _ := f.check.Uses[sel.Sel].(*Func)
		if fn == nil || fn.pkg == nil || fn.pkg.path != "sgo" {
			break
		}
		if fn.name == "Receive" || fn.name == "Reserve" {
			return f.tie(sel.X) // a received value or a permit: tied as the channel is
		}
		for _, a := range append([]syntax.Expr{sel.X}, e.ArgList...) {
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
	if !f.report {
		return
	}
	f.check.errorf(at, UnsupportedFeature, "SGO_E_RESOURCE_ESCAPES: "+format+" (SGO-RES-003)", args...)
}

// sgoResourceFlow checks fd against the resource flow rule.
func (check *Checker) sgoResourceFlow(fd *syntax.FuncDecl) {
	if fd.Body == nil {
		return
	}
	f := &sgoResFlow{check: check, val: map[Object]syntax.Node{}, dst: map[Object]syntax.Node{},
		unknown: &sgoParamExt{}, top: &sgoParamExt{}, pseudo: map[Object]*sgoParamExt{}}
	// Ties only narrow and destinations only widen, along finite chains: iterate to the fixpoint,
	// then report in one last pass.
	for i := 0; i < 20; i++ {
		f.changed = false
		f.pass(fd)
		if !f.changed {
			break
		}
	}
	f.report = true
	f.pass(fd)
}

func (f *sgoResFlow) pass(fd *syntax.FuncDecl) {
	check := f.check
	f.parent, f.home = map[syntax.Node]syntax.Node{}, map[Object]syntax.Node{}
	f.scope, f.moved = map[Object]syntax.Node{}, map[Object]syntax.Node{}
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
			f.send(n.Chan, n.Value)
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
			} else if _, decl := ext.(*syntax.FuncDecl); decl && sgoResType(obj.Type()) {
				if f.pseudo[obj] == nil {
					f.pseudo[obj] = &sgoParamExt{p.Name}
				}
				f.home[obj] = f.pseudo[obj]
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
		if _, param := f.home[obj].(*sgoParamExt); !param {
			f.setVal(obj, f.tie(r))
			f.setDst(obj, f.dest(r))
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
			f.move(sel.X, c.ArgList[0])
		case fn.name == "Own" && len(c.ArgList) == 1:
			f.move(c.ArgList[0], sel.X) // a registered user resource is tied to its scope (D152)
		case fn.name == "Commit" && len(c.ArgList) == 1:
			f.send(sel.X, c.ArgList[0])
		}
	}
	// sgo.MoveTo(r, s): a user resource (or any resource) moves to s (D158).
	if name, _ := syntax.Unparen(c.Fun).(*syntax.SelectorExpr); name != nil && len(c.ArgList) == 2 {
		if fn, _ := f.check.Uses[name.Sel].(*Func); fn != nil && fn.pkg != nil && fn.pkg.path == "sgo" && fn.name == "MoveTo" && fn.Signature().recv == nil {
			f.move(c.ArgList[0], c.ArgList[1])
		}
	}
}

// move records r.MoveTo(s) of a variable r: it is tied to s from here on.
func (f *sgoResFlow) move(r, s syntax.Expr) {
	if n, _ := syntax.Unparen(r).(*syntax.Name); n != nil {
		if obj := f.check.Uses[n]; obj != nil {
			if x := f.scopeExtent(s); x != nil && f.within(f.cur(), x) {
				f.moved[obj] = x
			}
		}
	}
}

// send checks that a value holding a resource is sent only on a channel that cannot outlive the
// value's scope (D158): the channel's tie lies within the value's.
func (f *sgoResFlow) send(ch, v syntax.Expr) {
	if !f.isRes(v) {
		return
	}
	if !f.within(f.dest(ch), f.tie(v)) {
		f.errorf(v, "%s is tied to a scope that may end before channel %s does; make the channel in that scope, or acquire the resource into the channel's scope", syntax.String(v), syntax.String(ch))
	}
}

// ends reports whether function literal lit, an operand of call, surely ends inside the current
// scope: it is called in place (also by defer), or it is the callback of a Spawn, SpawnWithout,
// or Start, which the nursery joins before it ends.
func (f *sgoResFlow) ends(call *syntax.CallExpr, lit *syntax.FuncLit) bool {
	if syntax.Unparen(call.Fun) == lit {
		return true
	}
	sel, _ := syntax.Unparen(call.Fun).(*syntax.SelectorExpr)
	if sel == nil {
		return false
	}
	fn, _ := f.check.Uses[sel.Sel].(*Func)
	if fn == nil || fn.pkg == nil || fn.pkg.path != "sgo" {
		return false
	}
	switch fn.name {
	case "Spawn", "SpawnWithout", "Start": // the nursery's children (SGO-SPAWN, SGO-AUTH, SGO-READY)
		return true
	}
	return false
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
		if call, _ := p.(*syntax.CallExpr); call == nil || !f.ends(call, lit) {
			f.errorf(n, "the function literal captures resource %s and may outlive its scope; only a literal called in place or spawned inside the scope may capture it", n.Value)
			return
		}
	}
}
