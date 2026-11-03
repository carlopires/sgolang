// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sgoadmit is the S-Go admission pass (SGO-ERR-001): it runs on parsed files before type
// checking and reports one diagnostic per outermost construct that subset 0 denies, at the first
// token that belongs only to that construct. A package is exempt only if its import path is a TCB
// package and every one of its files is that package's file pinned by digest in
// sgo/TARGET_TCB.json (SGO-TCB-002); no flag, name, pragma, or copied content widens admission.
package sgoadmit

import (
	"cmd/compile/internal/syntax"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Diag is one admission diagnostic.
type Diag struct {
	Pos  syntax.Pos
	Code string
	Msg  string
}

// Check runs admission on the parsed files of one package. directives are the positions of
// //go: compiler directives found while parsing. Diagnostics are returned in source order.
func Check(files []*syntax.File, filenames []string, directives []syntax.Pos) []Diag {
	a := &admitter{src: map[string][]byte{}}
	for i, f := range files {
		if i < len(filenames) {
			a.src[f.Pos().RelFilename()] = mustRead(filenames[i])
		}
		a.file(f)
	}
	for _, p := range directives {
		a.deny(a.tokenBefore(p, "//"), "compiler directive", "SGO-NATIVE-001")
	}
	order := map[string]int{}
	for i, f := range files {
		order[f.Pos().RelFilename()] = i
	}
	sort.SliceStable(a.diags, func(i, j int) bool {
		pi, pj := a.diags[i].Pos, a.diags[j].Pos
		if fi, fj := order[pi.RelFilename()], order[pj.RelFilename()]; fi != fj {
			return fi < fj
		}
		if pi.Line() != pj.Line() {
			return pi.Line() < pj.Line()
		}
		return pi.Col() < pj.Col()
	})
	return a.diags
}

func mustRead(name string) []byte {
	data, _ := os.ReadFile(name)
	return data
}

// Pkgpath is the import path of the package being compiled (set by the noder): a std/ package
// imports only std/ packages and sgo (SGO-IMP-004, D115).
var Pkgpath string

type admitter struct {
	diags []Diag
	src   map[string][]byte
}

func (a *admitter) report(pos syntax.Pos, code, msg string) {
	a.diags = append(a.diags, Diag{pos, code, msg})
}

func (a *admitter) deny(pos syntax.Pos, what, clause string) {
	a.report(pos, "SGO_E_UNADMITTED", what+" is not admitted in S-Go subset 0 ("+clause+")")
}

// offset returns the byte offset of pos in its file, or -1.
func (a *admitter) offset(pos syntax.Pos) int {
	src := a.src[pos.RelFilename()]
	line, col, off := uint(1), int(pos.Col()), 0
	for line < pos.Line() {
		i := strings.IndexByte(string(src[off:]), '\n')
		if i < 0 {
			return -1
		}
		off += i + 1
		line++
	}
	return off + col - 1
}

// posAt converts a byte offset in the file of ref to a position.
func (a *admitter) posAt(ref syntax.Pos, off int) syntax.Pos {
	src := a.src[ref.RelFilename()]
	line, col := uint(1), uint(1)
	for i := 0; i < off && i < len(src); i++ {
		if src[i] == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return syntax.MakePos(ref.Base(), line, col)
}

// tokenBefore finds the last occurrence of tok before pos (skipping only whitespace and other
// source between them on the way back) and returns its position; pos itself if not found.
func (a *admitter) tokenBefore(pos syntax.Pos, tok string) syntax.Pos {
	off := a.offset(pos)
	src := a.src[pos.RelFilename()]
	if off < 0 || off > len(src) {
		return pos
	}
	if i := strings.LastIndex(string(src[:off]), tok); i >= 0 {
		return a.posAt(pos, i)
	}
	return pos
}

// tokenAfter finds the first occurrence of tok at or after pos.
func (a *admitter) tokenAfter(pos syntax.Pos, tok string) syntax.Pos {
	off := a.offset(pos)
	src := a.src[pos.RelFilename()]
	if off < 0 || off > len(src) {
		return pos
	}
	if i := strings.Index(string(src[off:]), tok); i >= 0 {
		return a.posAt(pos, off+i)
	}
	return pos
}

func (a *admitter) file(f *syntax.File) {
	for _, d := range f.DeclList {
		switch d := d.(type) {
		case *syntax.ImportDecl:
			a.importDecl(d)
		case *syntax.ConstDecl:
			if d.Type != nil {
				a.typ(d.Type)
			}
			a.expr(d.Values)
		case *syntax.VarDecl:
			a.deny(a.tokenBefore(d.Pos(), "var"), "package-level variable", "SGO-BIND-003")
		case *syntax.TypeDecl:
			a.typeDecl(d)
		case *syntax.FuncDecl:
			a.funcDecl(d)
		}
	}
}

func (a *admitter) importDecl(d *syntax.ImportDecl) {
	if d.LocalPkgName != nil {
		a.deny(d.LocalPkgName.Pos(), "named, dot, or blank import", "SGO-BIND-002")
		return
	}
	if d.Path == nil {
		return
	}
	path, err := strconv.Unquote(d.Path.Value)
	if err != nil {
		return
	}
	first, _, _ := strings.Cut(path, "/")
	switch {
	case path == "sgo":
	case path == "C" || path == "unsafe":
		a.deny(d.Path.Pos(), "import of "+strconv.Quote(path), "SGO-NATIVE-001")
	case first == "std" && path != "std":
		// The S-Go standard library (SGO-IMP-004).
	case strings.HasPrefix(Pkgpath, "std/"):
		// A std package imports only std packages and sgo (SGO-IMP-004, D115).
		a.deny(d.Path.Pos(), "import of "+strconv.Quote(path)+" by the standard library (std imports only std and sgo)", "SGO-IMP-004")
	case !strings.Contains(first, "."):
		a.deny(d.Path.Pos(), "import of Go standard-library package "+strconv.Quote(path), "SGO-BIND-002")
	}
}

// typeDecl checks a package-level type declaration: a definition `type T U` of a named value type
// (SGO-UTYPE-002). Aliases and type parameters stay denied (SGO-UTYPE-001, SGO-GEN-001); a struct
// type literal is admitted only here, as the definition (SGO-COMP-002). types2 checks that U is a
// value type (sgoValueTypes).
func (a *admitter) typeDecl(d *syntax.TypeDecl) {
	switch {
	case d.Alias:
		a.deny(a.tokenBefore(d.Pos(), "type"), "type alias", "SGO-UTYPE-001")
		return
	case len(d.TParamList) > 0:
		a.deny(a.tokenBefore(d.TParamList[0].Pos(), "["), "type parameters", "SGO-GEN-001")
		return
	}
	if st, ok := d.Type.(*syntax.StructType); ok {
		a.structType(st)
		return
	}
	a.typ(d.Type)
}

// structType checks the fields of a struct type definition: named fields only, no tags, no blank
// fields (SGO-COMP-002).
func (a *admitter) structType(st *syntax.StructType) {
	var last syntax.Expr
	for i, f := range st.FieldList {
		switch {
		case f.Name == nil:
			a.deny(f.Type.Pos(), "embedded field", "SGO-COMP-001")
			continue
		case f.Name.Value == "_":
			a.deny(f.Name.Pos(), "blank field", "SGO-COMP-001")
		case i < len(st.TagList) && st.TagList[i] != nil:
			a.deny(st.TagList[i].Pos(), "struct tag", "SGO-COMP-001")
		}
		if f.Type != last { // fields declared in one list share their type expression
			a.typ(f.Type)
			last = f.Type
		}
	}
}

// recvOK checks a method receiver: a value receiver whose type is a name (SGO-UTYPE-002). Pointer
// receivers wait for stage 2; generic receivers are denied with type parameters.
func (a *admitter) recvOK(r *syntax.Field) bool {
	switch t := syntax.Unparen(r.Type).(type) {
	case *syntax.Name:
		return true
	case *syntax.Operation:
		if t.Op == syntax.Mul && t.Y == nil {
			if _, ok := syntax.Unparen(t.X).(*syntax.Name); ok {
				return true // a pointer receiver of a named type (SGO-COMP-007)
			}
		}
	case *syntax.IndexExpr:
		a.deny(a.tokenBefore(r.Pos(), "("), "method with a generic receiver", "SGO-GEN-001")
		return false
	}
	a.deny(a.tokenBefore(r.Pos(), "("), "method receiver type", "SGO-UTYPE-001")
	return false
}

func (a *admitter) funcDecl(d *syntax.FuncDecl) {
	switch {
	case d.Recv != nil && !a.recvOK(d.Recv):
		return
	case len(d.TParamList) > 0:
		a.deny(a.tokenBefore(d.TParamList[0].Pos(), "["), "type parameters", "SGO-GEN-001")
		return
	case d.Recv == nil && d.Name.Value == "init":
		a.deny(a.tokenBefore(d.Name.Pos(), "func"), "func init", "SGO-BIND-003")
		return
	case d.Body == nil:
		a.deny(a.tokenBefore(d.Name.Pos(), "func"), "function declaration without a body", "SGO-NATIVE-001")
		return
	}
	a.funcType(d.Type)
	a.stmtList(d.Body.List)
}
