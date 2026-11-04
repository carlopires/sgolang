// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// S-Go subset-0 type-level rules (enabled by Config.SGo). Diagnostics start with their SGO_E_ code
// (SGO-ERR-001). Admission (cmd/compile/internal/sgoadmit) has already denied every construct
// outside subset 0 before these checks run.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
	"strings"
)

// isSgoSelector reports whether e is sgo.<name>.
func isSgoSelector(e syntax.Expr, name string) bool {
	sel, ok := e.(*syntax.SelectorExpr)
	if !ok || sel.Sel.Value != name {
		return false
	}
	x, ok := sel.X.(*syntax.Name)
	return ok && x.Value == "sgo"
}

// sgoMainDecl checks SGO-PROC-001: package main declares exactly func main(root *sgo.Context) error.
func (check *Checker) sgoMainDecl(s *syntax.FuncDecl) {
	t := s.Type
	ok := len(s.TParamList) == 0 && len(t.ParamList) == 1 && len(t.ResultList) == 1
	if ok {
		p, isOp := t.ParamList[0].Type.(*syntax.Operation)
		ok = isOp && p.Op == syntax.Mul && p.Y == nil && isSgoSelector(p.X, "Context")
	}
	if ok {
		r, isName := t.ResultList[0].Type.(*syntax.Name)
		ok = isName && r.Value == "error"
	}
	if !ok {
		check.softErrorf(s, InvalidMainDecl, "SGO_E_MAIN_SIGNATURE: package main must declare func main(root *sgo.Context) error (SGO-PROC-001)")
	}
}

// sgoPackageChecks applies package-level rules once objects are collected.
func (check *Checker) sgoPackageChecks(files []*syntax.File) {
	if check.pkg.name == "main" && len(files) > 0 && check.pkg.scope.Lookup("main") == nil {
		// At the start of the package clause's line (the package keyword) of the first file.
		name := files[0].PkgName.Pos()
		check.softErrorf(syntax.MakePos(name.Base(), name.Line(), 1), InvalidMainDecl, "SGO_E_MAIN_SIGNATURE: package main has no func main(root *sgo.Context) error (SGO-PROC-001)")
	}
}

// sgoCodes maps type-checker error codes to S-Go diagnostic codes (SGO-ERR-001). Codes not listed
// are type errors of SGO-TYPE-002 and map to SGO_E_TYPE_MISMATCH.
var sgoCodes = map[Code]string{
	UnusedVar:              "SGO_E_UNUSED",
	UnusedImport:           "SGO_E_UNUSED",
	UnusedLabel:            "SGO_E_UNUSED_LABEL",
	UndeclaredName:         "SGO_E_UNDEFINED",
	UndeclaredImportedName: "SGO_E_UNDEFINED",
	UnexportedName:         "SGO_E_UNEXPORTED",
	DuplicateDecl:          "SGO_E_REDECLARED",
	NoNewVar:               "SGO_E_NO_NEW_VARIABLES",
	WrongAssignCount:       "SGO_E_ASSIGN_COUNT",
	WrongArgCount:          "SGO_E_ARG_COUNT",
	WrongResultCount:       "SGO_E_ARG_COUNT",
	MissingReturn:          "SGO_E_MISSING_RETURN",
	DuplicateCase:          "SGO_E_DUPLICATE_CASE",
	DivByZero:              "SGO_E_CONST_DIV_ZERO",
	NumericOverflow:        "SGO_E_CONST_OVERFLOW",
	InvalidShiftCount:      "SGO_E_CONST_SHIFT",
	MisplacedBreak:         "SGO_E_MISPLACED_BRANCH",
	MisplacedContinue:      "SGO_E_MISPLACED_BRANCH",
	MismatchedPkgName:      "SGO_E_PACKAGE_CLAUSE",
	InvalidMainDecl:        "SGO_E_MAIN_SIGNATURE",
}

func sgoCoded(code Code, msg string) string {
	if len(msg) >= 6 && msg[:6] == "SGO_E_" {
		return msg
	}
	c, ok := sgoCodes[code]
	if code == SwappedSliceIndices || code == InvalidIndex && (strings.Contains(msg, "out of bounds") || strings.Contains(msg, "must not be negative")) {
		c, ok = "SGO_E_CONST_RANGE", true // a constant index outside the array (SGO-COMP-003)
	}
	if !ok {
		c = "SGO_E_TYPE_MISMATCH"
	}
	return c + ": " + msg
}

// isChanType reports whether T's underlying type is a channel type.
func isChanType(T Type) bool {
	_, ok := T.Underlying().(*Chan)
	return ok
}

// isFuncType reports whether T's underlying type is a function type.
func isFuncType(T Type) bool {
	_, ok := T.Underlying().(*Signature)
	return ok
}

// isSgoHandle reports whether T is *sgo.Context, *sgo.Nursery, *sgo.Writer, or an instance of
// *sgo.Channel[T] or *sgo.SendPermit[T] (SGO-TYPE-005, SGO-PERMIT-002).
func isSgoHandle(T Type) bool {
	p, ok := Unalias(T).(*Pointer)
	if !ok {
		return false
	}
	n, ok := Unalias(p.base).(*Named)
	if !ok || n.obj == nil || n.obj.pkg == nil || n.obj.pkg.path != "sgo" {
		return false
	}
	switch n.obj.name {
	case "Context", "Nursery", "Writer", "Channel", "SendPermit", "Ready",
		"Clock", "WallClock", "Random", "Entropy", "SimStore", "Event", "CapacityLimiter", "Token",
		"Dir", "File", "Conn", "Listener", "Listing", "T", "Dialer":
		return true
	}
	return false
}

// sgoErrorPath reports whether a return statement with operands results takes the error path of a
// function with results res: there are at least two results, the last is error, and its operand
// is not the literal nil (SGO-ERR-004).
func sgoErrorPath(res *Tuple, results []syntax.Expr) bool {
	n := res.Len()
	if n < 2 || len(results) != n || !Identical(res.vars[n-1].typ, universeError) {
		return false
	}
	if name, _ := syntax.Unparen(results[n-1]).(*syntax.Name); name != nil && name.Value == "nil" {
		return false
	}
	return true
}

// sgoNilOK checks an assignment of nil to T: nil is only a value of type error (SGO-TYPE-002), and
// never a capability handle (SGO-TYPE-005).
func (check *Checker) sgoNilOK(x *operand, T Type) bool {
	switch {
	case T == nil:
		return true
	case check.sgoErrReturn && sgoResKind(T):
		return true // the error path of a resource result (SGO-ERR-004): never observed by check
	case isSgoHandle(T):
		check.errorf(x, IncompatibleAssign, "SGO_E_CAPABILITY_MISUSE: nil is not a value of capability type %s (SGO-TYPE-005)", T)
		return false
	case isChanType(T):
		check.errorf(x, IncompatibleAssign, "SGO_E_TYPE_MISMATCH: nil is not a value of channel type %s; make(chan T) is the only constructor (SGO-CHAN-002)", T)
		return false
	case sgoNilable(T):
		return true // pointers, slices, and maps (SGO-COMP-008)
	case !Identical(T, universeError):
		check.errorf(x, IncompatibleAssign, "SGO_E_TYPE_MISMATCH: nil is only a value of type error, not %s (SGO-TYPE-002)", T)
		return false
	}
	return true
}

// sgoComparisonOK applies the S-Go comparison rules: function values and capability handles are
// never compared (SGO-TYPE-003, SGO-TYPE-005), and two error values are compared only against nil
// (SGO-TYPE-006).
func (check *Checker) sgoComparisonOK(x, y *operand) bool {
	for _, o := range []*operand{x, y} {
		if o.mode() == nilvalue {
			continue
		}
		switch {
		case isSgoHandle(o.typ()):
			check.errorf(x, MismatchedTypes, "SGO_E_CAPABILITY_MISUSE: capability handles cannot be compared (SGO-TYPE-005)")
			return false
		case isChanType(o.typ()):
			check.errorf(x, MismatchedTypes, "SGO_E_UNADMITTED: comparison of channel values is not admitted (SGO-CHAN-002)")
			return false
		case isFuncType(o.typ()):
			check.errorf(x, MismatchedTypes, "SGO_E_UNADMITTED: comparison of function values is not admitted in S-Go subset 0 (SGO-TYPE-003)")
			return false
		case sgoHasChan(o.typ()):
			check.errorf(x, MismatchedTypes, "SGO_E_UNADMITTED: comparison of values holding channels or frozen values is not admitted (D95, D101)")
			return false
		}
	}
	if x.mode() == nilvalue || y.mode() == nilvalue {
		return true
	}
	if Identical(x.typ(), universeError) && Identical(y.typ(), universeError) {
		check.errorf(x, MismatchedTypes, "SGO_E_UNADMITTED: comparison of two error values is not admitted in S-Go subset 0 (SGO-TYPE-006)")
		return false
	}
	if sgoHoldsIface(x.typ()) || sgoHoldsIface(y.typ()) {
		check.errorf(x, MismatchedTypes, "SGO_E_UNADMITTED: comparison of interface values is not admitted; compare with nil or use a type switch (SGO-IFACE-001, D128)")
		return false
	}
	return true
}

// sgoNilable reports whether nil is a value of T: pointers (not handles), slices, maps (SGO-COMP-008,
// D96), and interfaces (SGO-IFACE-002).
func sgoNilable(T Type) bool {
	switch T.Underlying().(type) {
	case *Pointer:
		return !isSgoHandle(T)
	case *Slice, *Map, *Interface:
		return true
	}
	return false
}
