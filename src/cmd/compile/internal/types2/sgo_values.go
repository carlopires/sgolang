// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// S-Go value structs, arrays, and named types (W13 stage 1: SGO-COMP-002, SGO-COMP-003,
// SGO-UTYPE-002). Admission (sgoadmit) passes the syntax of these constructs; the type-directed
// parts of the rules are checked here, after type checking.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// sgoValueType reports whether T is a value type (SGO-COMP-002): int, bool, string, a struct of
// value types, an array of a value type, or a named type of package other than sgo whose
// underlying type is one of these. why names the first offending part when it is not.
func sgoValueType(T Type) (ok bool, why string) {
	return sgoValueTypeRec(T, map[Type]bool{})
}

func sgoValueTypeRec(T Type, seen map[Type]bool) (bool, string) {
	T = Unalias(T)
	if seen[T] {
		return true, "" // a cycle is reported by the type checker (invalid recursive type)
	}
	seen[T] = true
	if n, ok := T.(*Named); ok && n.obj != nil && n.obj.pkg != nil && n.obj.pkg.path == "sgo" {
		return false, "type " + n.obj.name + " of package sgo"
	}
	switch u := T.Underlying().(type) {
	case *Basic:
		switch u.kind {
		case Int, Bool, String:
			return true, ""
		}
		return false, "type " + u.name
	case *Struct:
		for _, f := range u.fields {
			if ok, why := sgoValueTypeRec(f.typ, seen); !ok {
				return false, "field " + f.name + ": " + why
			}
		}
		return true, ""
	case *Array:
		if ok, why := sgoValueTypeRec(u.elem, seen); !ok {
			return false, "element: " + why
		}
		return true, ""
	}
	return false, "type " + TypeString(T, nil)
}

// sgoTypeDecls checks the package's type definitions: each defines a named value type
// (SGO-UTYPE-002, SGO-COMP-002).
func (check *Checker) sgoTypeDecls(files []*syntax.File) {
	for _, f := range files {
		for _, d := range f.DeclList {
			td, _ := d.(*syntax.TypeDecl)
			if td == nil {
				continue
			}
			tn, _ := check.Defs[td.Name].(*TypeName)
			if tn == nil {
				continue
			}
			if ok, why := sgoValueType(tn.typ); !ok {
				check.errorf(td.Type, UnsupportedFeature, "SGO_E_UNADMITTED: type %s is not a value type (%s); only value types can be defined (SGO-COMP-002)", td.Name.Value, why)
			}
		}
	}
}

// sgoArrayType checks an array type expression: its element type is a value type (SGO-COMP-002).
func (check *Checker) sgoArrayType(e *syntax.ArrayType) {
	tv, ok := check.Types[e]
	if !ok || tv.Type == nil {
		return
	}
	a, _ := tv.Type.(*Array)
	if a == nil {
		return
	}
	if ok, why := sgoValueType(a.elem); !ok {
		check.errorf(e.Elem, UnsupportedFeature, "SGO_E_UNADMITTED: array element %s is not a value type (SGO-COMP-002)", why)
	}
}

// sgoChanElemOK checks a channel element (or sgo.Channel[T], sgo.SendPermit[T] argument) type:
// int, bool, string, error, a value type, or for native channels a channel of these
// (SGO-CHAN-002, SGO-PERMIT-002, amended by SGO-COMP-002).
func (check *Checker) sgoChanElemOK(at syntax.Expr, T Type, native bool) {
	if T == nil {
		return
	}
	if Identical(T, universeError) {
		return
	}
	if c, ok := T.Underlying().(*Chan); ok && native {
		check.sgoChanElemOK(at, c.elem, true)
		return
	}
	if ok, why := sgoValueType(T); !ok {
		check.errorf(at, UnsupportedFeature, "SGO_E_UNADMITTED: channel element %s is not admitted: elements are int, bool, string, error, and value types (SGO-CHAN-002, SGO-COMP-002)", why)
	}
}

// sgoChanType checks a native channel type expression's element.
func (check *Checker) sgoChanType(e *syntax.ChanType) {
	if tv, ok := check.Types[e]; ok && tv.Type != nil {
		if c, _ := tv.Type.(*Chan); c != nil {
			check.sgoChanElemOK(e.Elem, c.elem, true)
		}
	}
}

// sgoIndex checks an index expression: only arrays are indexed (SGO-COMP-003); instantiations of
// package sgo's generic members are admitted by sgoadmit.
func (check *Checker) sgoIndex(e *syntax.IndexExpr) {
	if tv, ok := check.Types[e]; ok && tv.IsType() {
		check.sgoInstanceElem(e)
		return
	}
	tv, ok := check.Types[e.X]
	if !ok || tv.Type == nil {
		return
	}
	switch tv.Type.Underlying().(type) {
	case *Array:
		return
	case *Signature:
		check.sgoInstanceElem(e) // sgo.NewChannel[T]
		return
	}
	check.errorf(atPos(e.Pos()), UnsupportedFeature, "SGO_E_UNADMITTED: index expression on %s is not admitted; only arrays are indexed (SGO-COMP-001, SGO-COMP-003)", tv.Type)
}

// sgoInstanceElem checks the type argument of sgo.Channel[T], sgo.SendPermit[T], sgo.NewChannel[T].
func (check *Checker) sgoInstanceElem(e *syntax.IndexExpr) {
	if tv, ok := check.Types[e.Index]; ok && tv.IsType() {
		check.sgoChanElemOK(e.Index, tv.Type, false)
	}
}

// sgoCompositeLit checks a composite literal's type: a named struct type or an array type, never a
// type of package sgo (SGO-COMP-002).
func (check *Checker) sgoCompositeLit(e *syntax.CompositeLit) {
	tv, ok := check.Types[e]
	if !ok || tv.Type == nil {
		return
	}
	if ok, why := sgoValueType(tv.Type); !ok {
		check.errorf(e, UnsupportedFeature, "SGO_E_UNADMITTED: composite literal of %s is not admitted (SGO-COMP-002)", why)
	}
}

// sgoMethodValue reports a method value (a selector of a method that is not called) or a method
// expression (SGO-UTYPE-002). called holds the selectors that are the function of a call.
func (check *Checker) sgoMethodValue(e *syntax.SelectorExpr, called map[*syntax.SelectorExpr]bool) {
	sel := check.Selections[e]
	if sel == nil {
		return
	}
	// Methods of package sgo's handles are function values by their own clauses (SGO-AUTH-*);
	// SGO-UTYPE-002 denies method values of user-declared types.
	if n, _ := Unalias(sel.recv).(*Named); n == nil || n.obj == nil || n.obj.pkg == nil || n.obj.pkg.path == "sgo" {
		return
	}
	switch sel.kind {
	case MethodExpr:
		check.errorf(e, UnsupportedFeature, "SGO_E_UNADMITTED: method expression %s is not admitted (SGO-UTYPE-001)", syntax.String(e))
	case MethodVal:
		if !called[e] {
			check.errorf(e, UnsupportedFeature, "SGO_E_UNADMITTED: method value %s is not admitted; call the method (SGO-UTYPE-001)", syntax.String(e))
		}
	}
}

// sgoBasicConversions are the predeclared types whose conversions SGO-TYPE-002 denies, except from
// a named type with that underlying type (SGO-UTYPE-002).
var sgoBasicConversions = map[string]bool{"int": true, "string": true, "bool": true, "error": true}

// sgoConversion checks a conversion T(x) (SGO-TYPE-002, SGO-UTYPE-002): to int, bool, string, or
// error only from a named type with an identical underlying type; to any other type only from a
// value with an identical underlying type or from an untyped constant of the same basic kind.
func (check *Checker) sgoConversion(call *syntax.CallExpr) {
	ft, ok := check.Types[call.Fun]
	if !ok || !ft.IsType() || ft.Type == nil || len(call.ArgList) != 1 {
		return
	}
	T := ft.Type
	at, ok := check.Types[call.ArgList[0]]
	if !ok || at.Type == nil {
		return
	}
	A := at.Type
	if name, _ := syntax.Unparen(call.Fun).(*syntax.Name); name != nil && sgoBasicConversions[name.Value] {
		if sgoIsBasic(A) || !Identical(A.Underlying(), T.Underlying()) {
			check.errorf(name, UnsupportedFeature, "SGO_E_UNADMITTED: conversion to %s is not admitted in S-Go subset 0 (SGO-TYPE-002)", name.Value)
		}
		return
	}
	if Identical(A.Underlying(), T.Underlying()) {
		return
	}
	if b, ok := A.(*Basic); ok && b.info&IsUntyped != 0 {
		const kinds = IsInteger | IsBoolean | IsString
		if u, ok := T.Underlying().(*Basic); ok && b.info&kinds == u.info&kinds {
			return
		}
	}
	check.errorf(call, UnsupportedFeature, "SGO_E_UNADMITTED: conversion of %s to %s is not admitted; only conversions between identical underlying types are (SGO-UTYPE-002)", A, T)
}

// sgoIsBasic reports whether T is a predeclared basic type (not a named type over one).
func sgoIsBasic(T Type) bool {
	_, ok := Unalias(T).(*Basic)
	return ok
}

// sgoAssignedRoot returns the variable a field or element assignment target writes: the name at
// the root of x.F, a[i], and their combinations (SGO-MEM-002 as amended by SGO-COMP-002).
func sgoAssignedRoot(e syntax.Expr) *syntax.Name {
	for {
		switch x := syntax.Unparen(e).(type) {
		case *syntax.Name:
			return x
		case *syntax.SelectorExpr:
			e = x.X
		case *syntax.IndexExpr:
			e = x.X
		default:
			return nil
		}
	}
}
