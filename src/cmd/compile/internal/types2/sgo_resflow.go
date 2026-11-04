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
	case "File", "Conn", "Listener", "Dialer", "Timer", "Request", "Lease":
		return true
	}
	return false
}

// sgoResIface reports whether T is a resource interface: an interface type that embeds
// sgo.Resource, directly or through embedded interfaces (D176, SGO-RES-006).
func sgoResIface(T Type) bool { return sgoResIfaceRec(T, map[Type]bool{}) }

func sgoResIfaceRec(T Type, seen map[Type]bool) bool {
	if T == nil {
		return false
	}
	T = Unalias(T)
	if n, _ := T.(*Named); n != nil && n.obj != nil && n.obj.pkg != nil && n.obj.pkg.path == "sgo" && n.obj.name == "Resource" {
		return true
	}
	if seen[T] {
		return false
	}
	seen[T] = true
	it, _ := T.Underlying().(*Interface)
	if it == nil {
		return false
	}
	for _, e := range it.embeddeds {
		if sgoResIfaceRec(e, seen) {
			return true
		}
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
	if sgoResKind(T) || sgoHasRelease(T) || sgoResIface(T) {
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
	if sgoSharedGeneric(T) {
		// *sgo.Task[E] and *sgo.Group[E] delivering resources: tied to the nursery the children
		// were started in, where sgo.Go moves their resource results (D158, D161, D172).
		for _, a := range Unalias(Unalias(T).(*Pointer).base).(*Named).TypeArgs().list() {
			if sgoResTypeRec(a, seen) {
				return true
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
	case *Pointer: // a pointer to a value holding a resource (W21 review Z1)
		return sgoResTypeRec(u.base, seen)
	case *Map:
		return sgoResTypeRec(u.key, seen) || sgoResTypeRec(u.elem, seen)
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
	// pext is a variable's pointee extent (W21 review Z1): the widest extent from which the memory
	// reachable through the variable (by a pointer, slice, or map) may be reached. Aliases share
	// it: an assignment of a reference value joins the extents of both sides.
	pext    map[Object]syntax.Node
	psh     map[Object]syntax.Node  // the shallow pointee extent: of the top-level pointee only
	org     map[Object]syntax.Node  // where the values assigned to a variable were made (for Own, Z2)
	pseudo  map[Object]*sgoParamExt // each resource parameter's extent, stable across passes
	top     syntax.Node             // the tie of a value holding no resource: it may flow anywhere
	unknown syntax.Node             // a destination of unknown lifetime: nothing holding a resource is sent on it
	changed bool
	report  bool
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
				if obj.pkg != nil && obj.pkg.path == "sgo" && obj.name == "NewPool" && len(e.ArgList) == 1 {
					if x := f.scopeExtent(e.ArgList[0]); x != nil {
						return x // a pool lives in the nursery it names (D175)
					}
					return f.cur()
				}
			}
		}
	}
	return f.unknown
}

func (f *sgoResFlow) cur() syntax.Node { return f.ext[len(f.ext)-1] }

// joinP joins pointee extents; nil is "reaches no memory".
func (f *sgoResFlow) joinP(a, b syntax.Node) syntax.Node {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	}
	return f.join(a, b)
}

func (f *sgoResFlow) setPext(obj Object, t syntax.Node) { f.setP(f.pext, obj, t) }
func (f *sgoResFlow) setPsh(obj Object, t syntax.Node)  { f.setP(f.psh, obj, t) }

func (f *sgoResFlow) setP(m map[Object]syntax.Node, obj Object, t syntax.Node) {
	if obj == nil || t == nil {
		return
	}
	if old, ok := m[obj]; ok {
		t = f.joinP(old, t)
		if t == old {
			return
		}
	}
	m[obj], f.changed = t, true
}

// pextOf returns the deep pointee extent of e's value (nil: it reaches no memory).
func (f *sgoResFlow) pextOf(e syntax.Expr) syntax.Node { return f.ptr(e, true) }

// ptr returns where the memory e's value points to may be reached from: with deep, any memory
// reachable through it; otherwise its top-level pointee only (W21 review Z1, Z2).
func (f *sgoResFlow) ptr(e syntax.Expr, deep bool) syntax.Node {
	switch e := syntax.Unparen(e).(type) {
	case *syntax.CheckExpr:
		return f.ptr(e.X, deep)
	case *syntax.BasicLit:
		return nil
	case *syntax.Name:
		obj := f.check.Uses[e]
		if obj == nil {
			obj = f.check.Defs[e]
		}
		v, _ := obj.(*Var)
		if v == nil {
			return nil // nil, a constant, a function
		}
		if v.parent == v.pkg.scope {
			return f.unknown // a package-level variable
		}
		m := f.psh
		if deep {
			m = f.pext
		}
		if x, ok := m[obj]; ok {
			return x
		}
		if ps, ok := f.home[obj].(*sgoParamExt); ok {
			return ps // memory of the caller
		}
		if !sgoRefType(v.typ) {
			return nil
		}
		return f.unknown
	case *syntax.Operation:
		if e.Y == nil && e.Op == syntax.And {
			if _, lit := syntax.Unparen(e.X).(*syntax.CompositeLit); lit {
				if deep {
					return f.joinP(f.cur(), f.ptr(e.X, true))
				}
				return f.cur()
			}
			if n, _ := syntax.Unparen(e.X).(*syntax.Name); n != nil {
				if obj := f.check.Uses[n]; obj != nil && f.home[obj] != nil {
					t := f.joinP(f.home[obj], f.psh[obj]) // &x: x's own storage
					if deep {
						t = f.joinP(t, f.ptr(n, true))
					}
					return t
				}
			}
			return f.ptr(e.X, true)
		}
		if e.Y == nil {
			return f.ptr(e.X, true) // *p: a pointer loaded from memory reachable through p
		}
		return nil
	case *syntax.CompositeLit:
		t := syntax.Node(nil)
		if T := f.check.typeOf(e); T != nil {
			switch Unalias(T).Underlying().(type) {
			case *Slice, *Map:
				t = f.cur() // allocated here
				if !deep {
					return t
				}
			}
		}
		for _, el := range e.ElemList {
			if kv, _ := el.(*syntax.KeyValueExpr); kv != nil {
				el = kv.Value
			}
			t = f.joinP(t, f.ptr(el, true))
		}
		return t
	case *syntax.SelectorExpr:
		if n, _ := e.X.(*syntax.Name); n != nil {
			if _, pkg := f.check.Uses[n].(*PkgName); pkg {
				return f.ptr(e.Sel, deep)
			}
		}
		return f.ptr(e.X, true) // a pointer loaded from memory reachable through e.X
	case *syntax.IndexExpr:
		return f.ptr(e.X, true)
	case *syntax.SliceExpr:
		return f.ptr(e.X, deep) // the same backing memory
	case *syntax.CallExpr:
		if T := f.check.typeOf(e); T == nil || !sgoRefType(T) {
			return nil
		}
		if tv, ok := f.check.Types[e.Fun]; ok && tv.IsType() {
			if len(e.ArgList) == 1 {
				return f.ptr(e.ArgList[0], deep) // a conversion
			}
		}
		if id, _ := syntax.Unparen(e.Fun).(*syntax.Name); id != nil {
			if b, _ := f.check.Uses[id].(*Builtin); b != nil {
				switch b.id {
				case _New, _Make:
					return f.cur()
				case _Append:
					t := f.cur()
					for i, a := range e.ArgList {
						if i == 0 || deep {
							t = f.joinP(t, f.ptr(a, deep || i > 0))
						}
					}
					return t
				}
			}
		}
		return f.unknown // a call may return memory reachable from anywhere
	}
	return f.unknown
}

// storeTarget returns where the memory written by a store to l (an indirect target) may be
// reached from: the root's top-level pointee for p.f, p[i], *p; any memory reachable through the
// root for a deeper path.
func (f *sgoResFlow) storeTarget(l syntax.Expr) syntax.Node {
	steps, atRoot := 0, false
	e := l
	for {
		ind, next := false, syntax.Expr(nil)
		switch x := syntax.Unparen(e).(type) {
		case *syntax.SelectorExpr:
			if T := f.check.typeOf(x.X); T != nil {
				_, ind = Unalias(T).Underlying().(*Pointer)
			}
			next = x.X
		case *syntax.IndexExpr:
			if T := f.check.typeOf(x.X); T != nil {
				switch Unalias(T).Underlying().(type) {
				case *Slice, *Map, *Pointer:
					ind = true
				}
			}
			next = x.X
		case *syntax.Operation:
			ind, next = x.Y == nil && x.Op == syntax.Mul, x.X
		case *syntax.Name:
			if steps == 1 && atRoot {
				return f.ptr(x, false)
			}
			return f.ptr(x, true)
		default:
			return f.unknown
		}
		if ind {
			steps++
			_, atRoot = syntax.Unparen(next).(*syntax.Name)
		}
		e = next
	}
}

// sgoRefType reports whether a store through a value of type T can reach shared memory: T is, or
// holds, a pointer, slice, or map (not a handle of package sgo, which is opaque).
func sgoRefType(T Type) bool { return sgoRefTypeRec(T, map[Type]bool{}) }

func sgoRefTypeRec(T Type, seen map[Type]bool) bool {
	if T == nil || seen[T] {
		return false
	}
	seen[T] = true
	if isSgoHandle(T) {
		return false
	}
	switch u := Unalias(T).Underlying().(type) {
	case *Pointer, *Slice, *Map:
		return true
	case *Struct:
		for _, fl := range u.fields {
			if sgoRefTypeRec(fl.typ, seen) {
				return true
			}
		}
	case *Array:
		return sgoRefTypeRec(u.elem, seen)
	}
	return false
}

// indirect reports whether storing to e writes memory reached through a pointer, slice, or map.
func (f *sgoResFlow) indirect(e syntax.Expr) bool {
	for {
		switch x := syntax.Unparen(e).(type) {
		case *syntax.SelectorExpr:
			if T := f.check.typeOf(x.X); T != nil {
				if _, ptr := Unalias(T).Underlying().(*Pointer); ptr {
					return true
				}
			}
			e = x.X
		case *syntax.IndexExpr:
			if T := f.check.typeOf(x.X); T != nil {
				switch Unalias(T).Underlying().(type) {
				case *Slice, *Map, *Pointer:
					return true
				}
			}
			e = x.X
		case *syntax.Operation:
			if x.Y == nil && x.Op == syntax.Mul {
				return true
			}
			e = x.X
		default:
			return false
		}
	}
}

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
	case *syntax.AssertExpr:
		return f.tie(e.X) // the dynamic value: tied as the interface value is (D176)
	case *syntax.CallExpr:
		if tv, ok := f.check.Types[e.Fun]; ok && tv.IsType() && len(e.ArgList) == 1 {
			return f.tie(e.ArgList[0]) // a conversion keeps the value's tie (D176)
		}
		if t, ok := f.callTie(e); ok {
			return t
		}
		// The result of append holds its arguments: tied as narrowly as they are.
		if id, _ := syntax.Unparen(e.Fun).(*syntax.Name); id != nil {
			if b, _ := f.check.Uses[id].(*Builtin); b != nil && b.id == _Append {
				t := f.top
				for _, a := range e.ArgList {
					if f.isRes(a) {
						t = f.inner(f.tie(a), t)
					}
				}
				return t
			}
		}
		// An acquisition is tied to its scope argument or receiver; another call's resource
		// results are tied to the caller's scope, as narrowly as the extent of the call.
		fun := syntax.Unparen(e.Fun)
		if ix, _ := fun.(*syntax.IndexExpr); ix != nil {
			fun = ix.X // sgo.Go[T](n, f), sgo.NewGroup[T](n, limit)
		}
		sel, _ := fun.(*syntax.SelectorExpr)
		if sel == nil {
			break
		}
		fn, _ := f.check.Uses[sel.Sel].(*Func)
		if fn == nil || fn.pkg == nil || fn.pkg.path != "sgo" {
			break
		}
		switch fn.name {
		case "Receive", "Reserve":
			return f.tie(sel.X) // a received value or a permit: tied as the channel is
		case "Wait", "Result", "Next":
			if sig, _ := fn.typ.(*Signature); sig != nil && sig.recv != nil && sgoSharedGeneric(sig.recv.typ) {
				return f.tie(sel.X) // a task's or group's result: tied as the handle is
			}
		case "Value":
			if sgoRecvNamed(fn, "Lease") {
				return f.tie(sel.X) // a leased item: tied as the lease is (D175)
			}
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
	f := &sgoResFlow{check: check, val: map[Object]syntax.Node{}, dst: map[Object]syntax.Node{}, pext: map[Object]syntax.Node{}, psh: map[Object]syntax.Node{}, org: map[Object]syntax.Node{},
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
	if fd.Recv != nil {
		f.params(&syntax.FuncType{ParamList: []*syntax.Field{fd.Recv}}, fd) // the receiver is a parameter
	}
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
		case *syntax.VarDecl:
			vals := sgoList(n.Values)
			if len(vals) == len(n.NameList) {
				for i, nm := range n.NameList {
					f.setPext(check.Defs[nm], f.pextOf(vals[i]))
					f.setPsh(check.Defs[nm], f.ptr(vals[i], false))
					f.setP(f.org, check.Defs[nm], f.origin(vals[i]))
				}
			}
		case *syntax.RangeClause:
			if n.Def {
				for _, l := range sgoList(n.Lhs) {
					if nm, _ := l.(*syntax.Name); nm != nil {
						f.setPext(check.Defs[nm], f.pextOf(n.X))
					}
				}
			}
		case *syntax.CallStmt:
			if n.Tok == syntax.Defer {
				f.deferUses(n)
			}
		}
		return true
	})
}

// deferUses reports a deferred call inside a nursery statement that mentions a resource tied to a
// scope of this function narrower than the function itself: deferred calls run when the function
// returns, after that scope has released the resource (W21 review Z3).
func (f *sgoResFlow) deferUses(d *syntax.CallStmt) {
	fn := f.funcExtent()
	if f.cur() == fn {
		return
	}
	syntax.Inspect(d.Call, func(n syntax.Node) bool {
		nm, _ := n.(*syntax.Name)
		if nm == nil {
			return true
		}
		obj, _ := f.check.Uses[nm].(*Var)
		if obj == nil || !sgoResType(obj.Type()) {
			return true
		}
		if t := f.tie(nm); t != nil && t != fn && f.within(t, fn) {
			f.errorf(nm, "%s is tied to a scope that releases it before this function's deferred calls run; use it in that scope, or in a spawned task's own defer", nm.Value)
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
			// A store through a call result (get(p).f, p.self().f, mk()[0].f): the memory may be
			// reached from anywhere, so only a value tied to no scope may go there (W21 review Z1).
			if f.indirect(l) {
				if target := f.storeTarget(l); target != nil && !f.within(target, f.tie(r)) {
					f.errorf(r, "%s is tied to a scope, and %s is memory reached through a call result, which may outlive it; store through a variable whose memory is made inside the scope", syntax.String(r), syntax.String(l))
				}
			}
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
		target := home
		if f.indirect(l) {
			target = f.storeTarget(l) // the store writes memory reached through a pointer, slice, or map
		}
		if target != nil && !f.within(target, f.tie(r)) {
			f.errorf(r, "%s is tied to a scope that ends before %s does; acquire it into the outer scope, or MoveTo that scope first", syntax.String(r), syntax.String(l))
		}
		if sgoRefType(T) {
			if !f.indirect(l) {
				f.setP(f.org, obj, f.origin(r))
				f.setPsh(obj, f.joinP(f.ptr(r, false), home))
				f.setPext(obj, f.joinP(f.ptr(r, true), home))
			} else {
				f.setPext(obj, f.ptr(r, true)) // obj now reaches r's memory
			}
			// The source's memory is now reachable from the target too: aliases share extents.
			if rv, _ := f.rootVar(r).(*Var); rv != nil && rv.parent != rv.pkg.scope {
				shallow, deep := target, target
				if !f.indirect(l) {
					shallow, deep = f.psh[obj], f.pext[obj]
				}
				if sgoBareAlias(r) {
					f.setPsh(rv, shallow)
				}
				f.setPext(rv, deep)
			}
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
			if x := f.scopeExtent(sel.X); x != nil {
				if pt := f.origin(c.ArgList[0]); pt != nil && !f.within(pt, x) {
					f.errorf(c.ArgList[0], "Own of a value named outside the scope: %s is reachable after the scope releases it; make the value inside the scope (W21 review Z2)", syntax.String(c.ArgList[0]))
				}
			}
			f.move(c.ArgList[0], sel.X) // a registered user resource is tied to its scope (D152)
		case fn.name == "Commit" && len(c.ArgList) == 1:
			f.send(sel.X, c.ArgList[0])
		case fn.name == "Put" && len(c.ArgList) == 1 && sgoRecvNamed(fn, "Pool"):
			f.send(sel.X, c.ArgList[0]) // an item must be tied at least as long as its pool (D175)
		case fn.name == "Acquire" && len(c.ArgList) == 1 && sgoRecvNamed(fn, "Pool") && f.isRes(sel.X):
			// A lease, and the item read from it, live in the lease's scope: it must lie within the
			// pool's extent, where its resource items are (D175).
			if x := f.scopeExtent(c.ArgList[0]); x != nil && !f.within(x, f.tie(sel.X)) {
				f.errorf(c.ArgList[0], "the lease's scope %s may outlive pool %s, whose items it would hold; acquire into the pool's nursery or a scope inside it", syntax.String(c.ArgList[0]), syntax.String(sel.X))
			}
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
	case "Spawn", "SpawnWithout", "Start", "Go": // the nursery's children (SGO-SPAWN, SGO-AUTH, SGO-READY, D161)
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

// sgoBareAlias reports whether e's value is (the top-level pointee of) a variable itself: x, &x,
// or x[a:b], not a value loaded from memory reachable through it.
func sgoBareAlias(e syntax.Expr) bool {
	switch x := syntax.Unparen(e).(type) {
	case *syntax.Name:
		return true
	case *syntax.SliceExpr:
		return sgoBareAlias(x.X)
	case *syntax.Operation:
		return x.Y == nil && x.Op == syntax.And && sgoBareAlias(x.X)
	}
	return false
}

// origin returns where the top-level pointee of e's value was made: the extent of an allocation,
// or what the variable was assigned (not where it flowed later; those flows are checked as stores
// of a tied value). Own requires the origin within the owning scope (W21 review Z2).
func (f *sgoResFlow) origin(e syntax.Expr) syntax.Node {
	switch x := syntax.Unparen(e).(type) {
	case *syntax.CheckExpr:
		return f.origin(x.X)
	case *syntax.Name:
		obj := f.check.Uses[x]
		v, _ := obj.(*Var)
		if v == nil {
			return nil
		}
		if v.parent == v.pkg.scope {
			return f.unknown
		}
		if o, ok := f.org[obj]; ok {
			return o
		}
		if ps, ok := f.home[obj].(*sgoParamExt); ok {
			return ps
		}
		if !sgoRefType(v.typ) {
			return nil
		}
		return f.unknown
	case *syntax.Operation:
		if x.Y == nil && x.Op == syntax.And {
			if _, lit := syntax.Unparen(x.X).(*syntax.CompositeLit); lit {
				return f.cur()
			}
			if n, _ := syntax.Unparen(x.X).(*syntax.Name); n != nil {
				if obj := f.check.Uses[n]; obj != nil && f.home[obj] != nil {
					return f.home[obj]
				}
			}
		}
	case *syntax.CallExpr:
		if tv, ok := f.check.Types[x.Fun]; ok && tv.IsType() && len(x.ArgList) == 1 {
			return f.origin(x.ArgList[0])
		}
		if id, _ := syntax.Unparen(x.Fun).(*syntax.Name); id != nil {
			if b, _ := f.check.Uses[id].(*Builtin); b != nil && (b.id == _New || b.id == _Make) {
				return f.cur()
			}
		}
	}
	return f.ptr(e, false)
}

// sgoRecvNamed reports whether fn is a method of *sgo.<name>[...].
func sgoRecvNamed(fn *Func, name string) bool {
	sig, _ := fn.typ.(*Signature)
	if sig == nil || sig.recv == nil {
		return false
	}
	p, _ := Unalias(sig.recv.typ).(*Pointer)
	if p == nil {
		return false
	}
	n, _ := Unalias(p.base).(*Named)
	return n != nil && n.obj != nil && n.obj.name == name
}

// callTie is the tie of the result of a call of a function or method outside package sgo (D176,
// rule 6 as amended, C1 and C2): the innermost of the ties of its authority arguments — scope
// arguments, arguments holding resources (handles, user resources, resource interfaces, handles
// delivering resources, and aggregates or pointers of them), and the receiver. A callee can hold
// and return only resources it was handed or acquired into a scope it was handed. A callee that is
// a function value, an argument that is a function value (a closure carries any scope it
// captured), no authority argument, or two ties that cannot be ordered: the caller's current
// extent. ok is false for package sgo, whose calls keep their specified ties, and for builtins.
func (f *sgoResFlow) callTie(e *syntax.CallExpr) (syntax.Node, bool) {
	fun := syntax.Unparen(e.Fun)
	if ix, _ := fun.(*syntax.IndexExpr); ix != nil {
		fun = ix.X
	}
	var fn *Func
	var recv syntax.Expr
	switch x := fun.(type) {
	case *syntax.Name:
		switch obj := f.check.Uses[x].(type) {
		case *Builtin:
			return nil, false
		case *Func:
			fn = obj
		}
	case *syntax.SelectorExpr:
		fn, _ = f.check.Uses[x.Sel].(*Func)
		if fn != nil {
			if sig, _ := fn.typ.(*Signature); sig != nil && sig.recv != nil {
				recv = x.X
			}
		}
	}
	if fn == nil {
		return f.cur(), true // a function value
	}
	if fn.pkg != nil && fn.pkg.path == "sgo" {
		return nil, false
	}
	args := e.ArgList
	if recv != nil {
		args = append([]syntax.Expr{recv}, args...)
	}
	var t syntax.Node
	for _, a := range args {
		if tv, ok := f.check.Types[a]; ok && tv.Type != nil {
			if _, isFunc := tv.Type.Underlying().(*Signature); isFunc {
				return f.cur(), true
			}
		}
		var x syntax.Node
		if s := f.scopeExtent(a); s != nil {
			x = s
		} else if f.isRes(a) {
			x = f.tie(a)
		} else {
			continue
		}
		switch {
		case t == nil || f.within(x, t):
			t = x
		case f.within(t, x):
		default:
			return f.cur(), true // ties that cannot be ordered
		}
	}
	if t == nil || t == f.top {
		return f.cur(), true
	}
	return t, true
}
