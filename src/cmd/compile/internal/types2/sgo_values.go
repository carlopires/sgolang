// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// S-Go structs, arrays, slices, maps, pointers, and named types (W13: SGO-COMP-002..008,
// SGO-UTYPE-002, SGO-TYPE-007). Admission (sgoadmit) passes the syntax of these constructs; the
// type-directed parts of the rules are checked here, after type checking.
//
// Three classes of types appear as fields, elements, and pointer targets:
//
//	value       int, bool, string, byte, and structs and arrays of value types (SGO-COMP-002)
//	shareable   value types, channel types, and structs and arrays of shareable types (D95)
//	task-local  slices, maps, pointers, and structs and arrays containing them (SGO-COMP-004)
//
// Functions, error values, and handles are never fields or elements.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// sgoValueType reports whether T is a value type: map keys and the stage-1 aggregates. why names
// the first offending part when it is not.
func sgoValueType(T Type) (ok bool, why string) {
	return sgoTypeRec(T, map[Type]bool{}, false)
}

// sgoFieldType reports whether T may be a field, element, map value, pointer target, or channel
// element: a value, shareable, or task-local type built from admitted types.
func sgoFieldType(T Type) (ok bool, why string) {
	return sgoTypeRec(T, map[Type]bool{}, true)
}

func sgoTypeRec(T Type, seen map[Type]bool, refs bool) (bool, string) {
	T = Unalias(T)
	if seen[T] {
		return true, "" // a recursive type: its parts are checked where they are first reached
	}
	seen[T] = true
	if _, ok := sgoFrozenElem(T); ok {
		if refs {
			return true, "" // shareable (SGO-COMP-009); its element is checked where it is formed
		}
		return false, "type " + TypeString(T, nil) + " (frozen values have no ==)"
	}
	if n, ok := T.(*Named); ok && n.obj != nil && n.obj.pkg != nil && n.obj.pkg.path == "sgo" {
		return false, "type " + n.obj.name + " of package sgo"
	}
	switch u := T.Underlying().(type) {
	case *Basic:
		switch u.kind {
		case Int, Bool, String, Uint8, Int32:
			// Int32 is only reachable as rune: the name int32 is denied (SGO-TYPE-008).
			return true, ""
		}
		return false, "type " + u.name
	case *Struct:
		for _, f := range u.fields {
			if ok, why := sgoTypeRec(f.typ, seen, refs); !ok {
				return false, "field " + f.name + ": " + why
			}
		}
		return true, ""
	case *Array:
		if ok, why := sgoTypeRec(u.elem, seen, refs); !ok {
			return false, "element: " + why
		}
		return true, ""
	}
	if !refs {
		return false, "type " + TypeString(T, nil)
	}
	switch u := T.Underlying().(type) {
	case *Chan:
		if ok, why := sgoTypeRec(u.elem, seen, refs); !ok && !Identical(u.elem, universeError) {
			return false, "channel element: " + why
		}
		return true, ""
	case *Slice:
		if ok, why := sgoTypeRec(u.elem, seen, refs); !ok {
			return false, "slice element: " + why
		}
		return true, ""
	case *Map:
		if ok, why := sgoTypeRec(u.key, map[Type]bool{}, false); !ok {
			return false, "map key (keys are value types): " + why
		}
		if ok, why := sgoTypeRec(u.elem, seen, refs); !ok {
			return false, "map value: " + why
		}
		return true, ""
	case *Pointer:
		if sgoResKind(T) {
			return true, "" // a resource held by a struct, array, or slice (D151)
		}
		if isSgoHandle(T) {
			return false, "handle type " + TypeString(T, nil)
		}
		if ok, why := sgoTypeRec(u.base, seen, refs); !ok {
			return false, "pointer target: " + why
		}
		return true, ""
	case *Interface:
		// An interface field or element: task-local, or shareable for error (SGO-IFACE-002, D132).
		// Interfaces are never value types (no == beyond nil, D128), so never map keys.
		return true, ""
	}
	return false, "type " + TypeString(T, nil)
}

// sgoTaskLocal reports whether T is a task-local type (SGO-COMP-004): a slice, map, or pointer
// (other than a handle), or a struct or array containing one. Channels are shareable.
func sgoTaskLocal(T Type) bool {
	return sgoTaskLocalRec(T, map[Type]bool{})
}

func sgoTaskLocalRec(T Type, seen map[Type]bool) bool {
	T = Unalias(T)
	if seen[T] {
		return false
	}
	seen[T] = true
	if isSgoHandle(T) {
		return false
	}
	if _, ok := sgoFrozenElem(T); ok {
		return false // shareable: nobody writes it (SGO-COMP-009)
	}
	switch u := T.Underlying().(type) {
	case *Slice, *Map, *Pointer:
		return true
	case *Interface:
		// Every interface type but error is task-local, whatever its dynamic value (D132).
		return !Identical(T, universeError)
	case *Struct:
		for _, f := range u.fields {
			if sgoTaskLocalRec(f.typ, seen) {
				return true
			}
		}
	case *Array:
		return sgoTaskLocalRec(u.elem, seen)
	}
	return false
}

// sgoHasChan reports whether a value of T holds a channel (such values have no ==, D95).
func sgoHasChan(T Type) bool {
	if _, ok := sgoFrozenElem(T); ok {
		return true // frozen values have no == either (D101)
	}
	switch u := T.Underlying().(type) {
	case *Chan:
		return true
	case *Struct:
		for _, f := range u.fields {
			if sgoHasChan(f.typ) {
				return true
			}
		}
	case *Array:
		return sgoHasChan(u.elem)
	}
	return false
}

// sgoHoldsIface reports whether a value of T holds an interface value, at the top or in a field or
// element (D128: interface values are compared only with nil).
func sgoHoldsIface(T Type) bool {
	switch u := T.Underlying().(type) {
	case *Interface:
		return true
	case *Struct:
		for _, f := range u.fields {
			if sgoHoldsIface(f.typ) {
				return true
			}
		}
	case *Array:
		return sgoHoldsIface(u.elem)
	}
	return false
}

// sgoTypeDecls checks the package's type definitions (SGO-UTYPE-002, SGO-COMP-002).
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
			if it, _ := tn.typ.Underlying().(*Interface); it != nil && !it.IsMethodSet() {
				// An embedded non-interface type makes a constraint interface (generics, D108).
				check.errorf(td.Type, UnsupportedFeature, "SGO_E_UNADMITTED: interface %s has a type set element; constraint interfaces are generics (SGO-IFACE-001)", td.Name.Value)
				continue
			}
			if ok, why := sgoFieldType(tn.typ); !ok {
				check.errorf(td.Type, UnsupportedFeature, "SGO_E_UNADMITTED: type %s is not admitted (%s); fields and elements are value, channel, slice, map, and pointer types (SGO-COMP-002, SGO-COMP-004)", td.Name.Value, why)
			}
		}
	}
}

// sgoTypeExpr checks an array, slice, map, or pointer type expression in a function body or
// signature (SGO-COMP-002, SGO-COMP-005..007).
func (check *Checker) sgoTypeExpr(e syntax.Expr) {
	tv, ok := check.Types[e]
	if !ok || tv.Type == nil || !tv.IsType() || isSgoHandle(tv.Type) {
		return
	}
	if ok, why := sgoFieldType(tv.Type); !ok {
		check.errorf(e, UnsupportedFeature, "SGO_E_UNADMITTED: type %s is not admitted (%s) (SGO-COMP-002, SGO-COMP-004)", tv.Type, why)
	}
}

// sgoChanElemOK checks a channel element (or sgo.Channel[T], sgo.SendPermit[T] argument) type:
// error or a value, shareable, or task-local type (SGO-CHAN-002, SGO-PERMIT-002, SGO-COMP-004).
func (check *Checker) sgoChanElemOK(at syntax.Expr, T Type, native bool) {
	if T == nil || Identical(T, universeError) {
		return
	}
	if _, isChan := T.Underlying().(*Chan); isChan && !native {
		check.errorf(at, UnsupportedFeature, "SGO_E_UNADMITTED: a channel is not an sgo.Channel payload (SGO-PERMIT-002)")
		return
	}
	if ok, why := sgoFieldType(T); !ok {
		check.errorf(at, UnsupportedFeature, "SGO_E_UNADMITTED: channel element %s is not admitted (SGO-CHAN-002, SGO-COMP-004)", why)
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

// sgoIndex checks an index expression: arrays, slices, strings, maps, and pointers to arrays are
// indexed (SGO-COMP-003, SGO-COMP-005, SGO-COMP-006); instantiations of package sgo's generic
// members are admitted by sgoadmit.
func (check *Checker) sgoIndex(e *syntax.IndexExpr) {
	if tv, ok := check.Types[e]; ok && tv.IsType() {
		check.sgoInstanceElem(e)
		return
	}
	tv, ok := check.Types[e.X]
	if !ok || tv.Type == nil {
		return
	}
	switch u := tv.Type.Underlying().(type) {
	case *Array, *Slice, *Map:
		return
	case *Basic:
		if u.info&IsString != 0 {
			return
		}
	case *Pointer:
		if _, ok := u.base.Underlying().(*Array); ok {
			return
		}
	case *Signature:
		check.sgoInstanceElem(e) // sgo.NewChannel[T], sgo.Copy[T]
		return
	}
	check.errorf(atPos(e.Pos()), UnsupportedFeature, "SGO_E_UNADMITTED: index expression on %s is not admitted (SGO-COMP-001)", tv.Type)
}

// sgoInstanceElem checks the type argument of sgo.Channel[T], sgo.SendPermit[T], sgo.NewChannel[T].
func (check *Checker) sgoInstanceElem(e *syntax.IndexExpr) {
	sel, _ := syntax.Unparen(e.X).(*syntax.SelectorExpr)
	if sel != nil && sel.Sel.Value == "Copy" {
		return // sgo.Copy[T]: T is checked where the value's type is
	}
	if sel != nil && (sel.Sel.Value == "Frozen" || sel.Sel.Value == "Freeze") {
		if tv, ok := check.Types[e.Index]; ok && tv.IsType() {
			check.sgoFrozenElemOK(e.Index, tv.Type)
		}
		return
	}
	if tv, ok := check.Types[e.Index]; ok && tv.IsType() {
		check.sgoChanElemOK(e.Index, tv.Type, false)
	}
}

// sgoCompositeLit checks a composite literal's type: never a type of package sgo, and only
// admitted field and element types (SGO-COMP-002).
func (check *Checker) sgoCompositeLit(e *syntax.CompositeLit) {
	switch e.Type.(type) {
	case *syntax.ArrayType, *syntax.SliceType, *syntax.MapType:
		return // the type expression is checked itself (sgoTypeExpr)
	}
	tv, ok := check.Types[e]
	if !ok || tv.Type == nil {
		return
	}
	if ok, why := sgoFieldType(tv.Type); !ok {
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
	recv := Unalias(sel.recv)
	if p, _ := recv.(*Pointer); p != nil {
		recv = Unalias(p.base) // a method of *T (SGO-COMP-007)
	}
	if n, _ := recv.(*Named); n == nil || n.obj == nil || n.obj.pkg == nil || n.obj.pkg.path == "sgo" {
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

// sgoConversion checks a conversion T(x) (SGO-TYPE-002, SGO-TYPE-007, SGO-UTYPE-002,
// SGO-COMP-005). Admitted: between types with identical underlying types; int(b) and byte(i)
// between int and byte (byte(i) is checked at run time); string(b) of a []byte and []byte(s) of a
// string; and an untyped constant to a named type of the same basic kind. A conversion to a
// predeclared type is never of an untyped constant or of a value of that same predeclared type.
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
	tu, au := T.Underlying(), A.Underlying()
	untyped := false
	if b, ok := A.(*Basic); ok && b.info&IsUntyped != 0 {
		untyped = true
	}
	isKind := func(u Type, k BasicKind) bool { b, ok := u.(*Basic); return ok && b.kind == k }
	isBytes := func(u Type) bool { s, ok := u.(*Slice); return ok && isKind(s.elem.Underlying(), Uint8) }
	isRunes := func(u Type) bool { s, ok := u.(*Slice); return ok && isKind(s.elem.Underlying(), Int32) }
	bad := func() {
		if name, _ := syntax.Unparen(call.Fun).(*syntax.Name); name != nil && sgoIsBasic(T) {
			check.errorf(name, UnsupportedFeature, "SGO_E_UNADMITTED: conversion to %s is not admitted in S-Go subset 0 (SGO-TYPE-002)", name.Value)
			return
		}
		check.errorf(call, UnsupportedFeature, "SGO_E_UNADMITTED: conversion of %s to %s is not admitted; only conversions between identical underlying types are (SGO-UTYPE-002)", A, T)
	}
	switch {
	case untyped:
		const kinds = IsInteger | IsBoolean | IsString
		b := A.(*Basic)
		if u, ok := tu.(*Basic); ok && !sgoIsBasic(T) && b.info&kinds == u.info&kinds {
			return // a constant to a named type of its kind
		}
		bad()
	case isKind(tu, Int) && isKind(au, Uint8), isKind(tu, Uint8) && isKind(au, Int):
		return // int(b), byte(i) (SGO-TYPE-007)
	case isKind(tu, Int32) && (isKind(au, Int) || isKind(au, Uint8)), (isKind(tu, Int) || isKind(tu, Uint8)) && isKind(au, Int32):
		return // rune(i), rune(b), int(r), byte(r) (SGO-TYPE-008, D120)
	case isKind(tu, String) && isKind(au, Int32):
		return // string(r) (SGO-VAL-002 v3); string(i) of an int stays denied (D120)
	case isKind(tu, String) && isBytes(au), isBytes(tu) && isKind(au, String):
		return // string(b), []byte(s) (SGO-COMP-005)
	case isKind(tu, String) && isRunes(au), isRunes(tu) && isKind(au, String):
		return // string(rs), []rune(s) (SGO-COMP-005 v2)
	case Identical(tu, au):
		if sgoIsBasic(T) && sgoIsBasic(A) {
			bad() // int(x) of an int, ...: not a conversion S-Go has
			return
		}
		return
	default:
		bad()
	}
}

// sgoIsBasic reports whether T is a predeclared basic type (not a named type over one).
func sgoIsBasic(T Type) bool {
	_, ok := Unalias(T).(*Basic)
	return ok
}

// sgoAssignedRoot returns the variable a field or element assignment target writes: the name at
// the root of x.F and a[i] where x is a struct value and a an array value, and their combinations
// (SGO-MEM-002 as amended by SGO-COMP-002). A write through a pointer, a slice, or a map changes
// the referenced object, not the variable: the result is nil.
func (check *Checker) sgoAssignedRoot(e syntax.Expr) *syntax.Name {
	for {
		switch x := syntax.Unparen(e).(type) {
		case *syntax.Name:
			return x
		case *syntax.SelectorExpr:
			if !check.sgoValuePath(x.X) {
				return nil
			}
			e = x.X
		case *syntax.IndexExpr:
			if !check.sgoValuePath(x.X) {
				return nil
			}
			e = x.X
		default:
			return nil
		}
	}
}

// sgoValuePath reports whether e is a struct or array value (so that a field or element of it is
// part of the variable that holds it).
func (check *Checker) sgoValuePath(e syntax.Expr) bool {
	tv, ok := check.Types[e]
	if !ok || tv.Type == nil {
		return true
	}
	switch tv.Type.Underlying().(type) {
	case *Struct, *Array:
		return true
	}
	return false
}
