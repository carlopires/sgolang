// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgoadmit

import (
	"cmd/compile/internal/syntax"
	"regexp"
	"strings"
)

// sgoMembers are the admitted members of package sgo (SGO_SPEC_SUBSET0.md §0.2).
var sgoMembers = map[string]bool{"Context": true, "Nursery": true, "Writer": true, "NewError": true, "IsCancelled": true,
	"Channel": true, "SendPermit": true, "NewChannel": true, "Ready": true,
	"Clock": true, "WallClock": true, "Random": true, "Entropy": true, "SimStore": true}

// sgoGeneric are the parameterized members of package sgo, admitted only instantiated with one
// channel element type (SGO-PERMIT-002); user-declared type parameters stay denied (SGO-GEN-001).
var sgoGeneric = map[string]bool{"Channel": true, "SendPermit": true, "NewChannel": true}

// chanElemTypes are the admitted channel element types (D29).
var chanElemTypes = map[string]bool{"int": true, "bool": true, "string": true, "error": true}

// sgoHandleTypes may appear behind a pointer in types (SGO-TYPE-005).
var sgoHandleTypes = map[string]bool{"Context": true, "Nursery": true, "Writer": true, "Ready": true,
	"Clock": true, "WallClock": true, "Random": true, "Entropy": true, "SimStore": true}

// deniedNames are predeclared identifiers denied in subset 0 (sgo/spec/INVENTORY.map "universe").
// Predeclared names cannot be redeclared (SGO-BIND-004), so every use refers to the predeclared one.
var deniedNames = map[string]string{
	"int8": "SGO-TYPE-001", "int16": "SGO-TYPE-001", "int32": "SGO-TYPE-001", "int64": "SGO-TYPE-001",
	"uint": "SGO-TYPE-001", "uint8": "SGO-TYPE-001", "uint16": "SGO-TYPE-001", "uint32": "SGO-TYPE-001",
	"uint64": "SGO-TYPE-001", "uintptr": "SGO-TYPE-001", "byte": "SGO-TYPE-001", "rune": "SGO-TYPE-001",
	"float32": "SGO-FLOAT-001", "float64": "SGO-FLOAT-001", "complex64": "SGO-FLOAT-001",
	"complex128": "SGO-FLOAT-001", "real": "SGO-FLOAT-001", "imag": "SGO-FLOAT-001", "complex": "SGO-FLOAT-001",
	"any": "SGO-IFACE-001", "comparable": "SGO-GEN-001", "iota": "SGO-VAL-004",
	"append": "SGO-COMP-001", "cap": "SGO-COMP-001", "clear": "SGO-COMP-001", "copy": "SGO-COMP-001",
	"delete": "SGO-COMP-001", "make": "SGO-COMP-001", "new": "SGO-COMP-001", "close": "SGO-CHAN-001",
	"max": "SGO-VAL-001", "min": "SGO-VAL-001", "print": "SGO-PROC-003", "println": "SGO-PROC-003",
	"recover": "SGO-DEF-002",
}

// conversionTypes are type names whose call form would be a conversion (SGO-TYPE-002).
var conversionTypes = map[string]bool{"int": true, "string": true, "bool": true, "error": true}

var legacyOctal = regexp.MustCompile(`^0[0-9_]+$`)

func (a *admitter) stmtList(list []syntax.Stmt) {
	for _, s := range list {
		a.stmt(s)
	}
}

func (a *admitter) stmt(s syntax.Stmt) {
	switch s := s.(type) {
	case nil:
	case *syntax.EmptyStmt:
		a.deny(s.Pos(), "empty statement", "SGO-CTL-001")
	case *syntax.LabeledStmt:
		a.stmt(s.Stmt)
	case *syntax.BlockStmt:
		a.stmtList(s.List)
	case *syntax.ExprStmt:
		a.expr(s.X)
	case *syntax.SendStmt:
		a.deny(s.Pos(), "channel send", "SGO-CHAN-001")
	case *syntax.DeclStmt:
		for _, d := range s.DeclList {
			switch d := d.(type) {
			case *syntax.ConstDecl:
				if d.Type != nil {
					a.typ(d.Type)
				}
				a.expr(d.Values)
			case *syntax.VarDecl:
				if d.Type != nil {
					a.typ(d.Type)
				}
				a.expr(d.Values)
			case *syntax.TypeDecl:
				a.deny(a.tokenBefore(d.Pos(), "type"), "type declaration", "SGO-UTYPE-001")
			}
		}
	case *syntax.AssignStmt:
		a.expr(s.Lhs)
		a.expr(s.Rhs)
	case *syntax.BranchStmt:
		switch s.Tok {
		case syntax.Goto:
			a.deny(s.Pos(), "goto", "SGO-CTL-001")
		case syntax.Fallthrough:
			a.deny(s.Pos(), "fallthrough", "SGO-CTL-002")
		}
	case *syntax.CallStmt:
		if s.Tok == syntax.Go {
			a.report(s.Pos(), "SGO_E_UNSTRUCTURED_SPAWN", "the go statement is not part of S-Go; spawn through a nursery handle (SGO-SPAWN-005)")
			return
		}
		a.expr(s.Call)
	case *syntax.ReturnStmt:
		a.expr(s.Results)
	case *syntax.IfStmt:
		a.stmt(s.Init)
		a.expr(s.Cond)
		a.stmt(s.Then)
		a.stmt(s.Else)
	case *syntax.ForStmt:
		if r, ok := s.Init.(*syntax.RangeClause); ok {
			a.deny(a.tokenAfter(s.Pos(), "range"), "range clause", "SGO-COMP-001")
			_ = r
			return
		}
		a.stmt(s.Init)
		a.expr(s.Cond)
		a.stmt(s.Post)
		a.stmt(s.Body)
	case *syntax.SwitchStmt:
		if g, ok := s.Tag.(*syntax.TypeSwitchGuard); ok {
			a.deny(g.Pos(), "type switch", "SGO-IFACE-001")
			return
		}
		a.stmt(s.Init)
		a.expr(s.Tag)
		for _, c := range s.Body {
			a.expr(c.Cases)
			a.stmtList(c.Body)
		}
	case *syntax.SelectStmt:
		a.deny(s.Pos(), "select", "SGO-CHAN-001")
	case *syntax.NurseryStmt:
		if s.Name == nil {
			a.deny(s.Pos(), "anonymous nursery", "SGO-NUR-001")
			return
		}
		a.stmt(s.Body)
	}
}

func (a *admitter) funcType(t *syntax.FuncType) {
	for _, f := range t.ParamList {
		if _, ok := f.Type.(*syntax.DotsType); ok {
			a.deny(f.Type.Pos(), "variadic parameter", "SGO-TYPE-003")
			continue
		}
		a.typ(f.Type)
	}
	for _, f := range t.ResultList {
		a.typ(f.Type)
	}
}

// typ checks an expression in type position.
func (a *admitter) typ(e syntax.Expr) {
	switch e := e.(type) {
	case nil:
	case *syntax.Name:
		a.name(e)
	case *syntax.SelectorExpr:
		a.sgoSelector(e)
	case *syntax.Operation: // *T
		if e.Op == syntax.Mul && e.Y == nil {
			if sel, ok := e.X.(*syntax.SelectorExpr); ok && isSgo(sel) && sgoHandleTypes[sel.Sel.Value] {
				return
			}
			if ix, ok := e.X.(*syntax.IndexExpr); ok && a.sgoInstance(ix, "Channel", "SendPermit") {
				return
			}
			a.deny(e.Pos(), "pointer type", "SGO-COMP-001")
			return
		}
		a.deny(e.Pos(), "type expression", "SGO-TYPE-001")
	case *syntax.FuncType:
		a.funcType(e)
	case *syntax.ParenExpr:
		a.typ(e.X)
	case *syntax.ArrayType, *syntax.SliceType:
		a.deny(e.Pos(), "array or slice type", "SGO-COMP-001")
	case *syntax.MapType:
		a.deny(e.Pos(), "map type", "SGO-COMP-001")
	case *syntax.ChanType:
		a.deny(e.Pos(), "channel type", "SGO-CHAN-001")
	case *syntax.StructType:
		a.deny(e.Pos(), "struct type", "SGO-COMP-001")
	case *syntax.InterfaceType:
		a.deny(e.Pos(), "interface type", "SGO-IFACE-001")
	case *syntax.IndexExpr:
		if sel, ok := e.X.(*syntax.SelectorExpr); ok && isSgo(sel) && sgoGeneric[sel.Sel.Value] {
			a.deny(e.Pos(), "sgo."+sel.Sel.Value+"[T] as a non-pointer type", "SGO-TYPE-005")
			return
		}
		a.deny(e.Pos(), "generic type instantiation", "SGO-GEN-001")
	default:
		a.deny(e.Pos(), "type expression", "SGO-TYPE-001")
	}
}

// sgoInstance reports whether e is sgo.M[T] for one of the members ms; it reports a denial and
// returns true (the instance is handled) when T is not an admitted channel element type.
func (a *admitter) sgoInstance(e *syntax.IndexExpr, ms ...string) bool {
	sel, ok := e.X.(*syntax.SelectorExpr)
	if !ok || !isSgo(sel) {
		return false
	}
	found := false
	for _, m := range ms {
		found = found || sel.Sel.Value == m
	}
	if !found {
		return false
	}
	if n, ok := e.Index.(*syntax.Name); ok && chanElemTypes[n.Value] {
		return true
	}
	a.deny(e.Index.Pos(), "channel element type "+syntax.String(e.Index)+" (admitted: int, bool, string, error)", "SGO-PERMIT-002")
	return true
}

func isSgo(sel *syntax.SelectorExpr) bool {
	x, ok := sel.X.(*syntax.Name)
	return ok && x.Value == "sgo"
}

// sgoSelector checks pkg.Member: members of package sgo must be admitted. Other selectors (user
// packages, method calls) are checked by the type checker.
func (a *admitter) sgoSelector(e *syntax.SelectorExpr) {
	if isSgo(e) {
		if !sgoMembers[e.Sel.Value] {
			a.deny(e.X.Pos(), "sgo."+e.Sel.Value, "SGO-ERR-001")
		}
		return
	}
	a.expr(e.X)
}

func (a *admitter) name(n *syntax.Name) {
	if clause, ok := deniedNames[n.Value]; ok {
		a.deny(n.Pos(), "predeclared "+n.Value, clause)
	}
}

func (a *admitter) expr(e syntax.Expr) {
	switch e := e.(type) {
	case nil:
	case *syntax.Name:
		a.name(e)
	case *syntax.BasicLit:
		a.basicLit(e)
	case *syntax.CompositeLit:
		a.deny(e.Pos(), "composite literal", "SGO-COMP-001")
	case *syntax.KeyValueExpr:
		a.deny(e.Pos(), "key-value expression", "SGO-COMP-001")
	case *syntax.FuncLit:
		a.funcType(e.Type)
		a.stmt(e.Body)
	case *syntax.ParenExpr:
		a.expr(e.X)
	case *syntax.SelectorExpr:
		a.sgoSelector(e)
	case *syntax.IndexExpr:
		if a.sgoInstance(e, "NewChannel") {
			return
		}
		a.deny(e.Pos(), "index expression", "SGO-COMP-001")
	case *syntax.SliceExpr:
		a.deny(e.Pos(), "slice expression", "SGO-COMP-001")
	case *syntax.AssertExpr:
		a.deny(e.Pos(), "type assertion", "SGO-IFACE-001")
	case *syntax.Operation:
		switch {
		case e.Y == nil && e.Op == syntax.And:
			a.deny(e.Pos(), "address-of operator &", "SGO-COMP-001")
		case e.Y == nil && e.Op == syntax.Recv:
			a.deny(e.Pos(), "channel receive", "SGO-CHAN-001")
		case e.Y == nil && e.Op == syntax.Mul:
			a.typ(e)
		default:
			a.expr(e.X)
			a.expr(e.Y)
		}
	case *syntax.CallExpr:
		if n, ok := e.Fun.(*syntax.Name); ok && conversionTypes[n.Value] {
			a.deny(n.Pos(), "conversion to "+n.Value, "SGO-TYPE-002")
			return
		}
		if e.HasDots {
			a.deny(e.Pos(), "variadic call", "SGO-TYPE-003")
			return
		}
		a.expr(e.Fun)
		for _, x := range e.ArgList {
			a.expr(x)
		}
	case *syntax.ListExpr:
		for _, x := range e.ElemList {
			a.expr(x)
		}
	default:
		a.typ(e)
	}
}

func (a *admitter) basicLit(l *syntax.BasicLit) {
	switch l.Kind {
	case syntax.FloatLit, syntax.ImagLit:
		a.deny(l.Pos(), "floating-point or imaginary literal", "SGO-FLOAT-001")
	case syntax.RuneLit:
		a.deny(l.Pos(), "rune literal", "SGO-LEX-004")
	case syntax.IntLit:
		if legacyOctal.MatchString(l.Value) {
			a.report(l.Pos(), "SGO_E_SYNTAX", "legacy octal literal "+l.Value+"; write 0o"+strings.TrimLeft(l.Value, "0")+" (SGO-LEX-004)")
		}
	case syntax.StringLit:
		if strings.HasPrefix(l.Value, "`") {
			return
		}
		v := l.Value
		for i := 1; i < len(v)-1; i++ {
			if v[i] != '\\' {
				continue
			}
			switch v[i+1] {
			case 'n', 't', 'r', '\\', '"':
				i++
			case 'x':
				i += 3
			default:
				a.deny(a.posAt(l.Pos(), a.offset(l.Pos())+i), "escape \\"+string(v[i+1]), "SGO-LEX-004")
				return
			}
		}
	}
}
