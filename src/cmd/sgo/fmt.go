// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// sgo fmt [files] formats .sgo files in place (SGO-TOOL-001), with gofmt's layout. Without
// arguments it formats the .sgo files of the current directory. A file that does not parse is
// refused (SGO_E_SYNTAX) and left unchanged.
//
// S-Go adds one statement to Go's syntax, nursery (n) { ... }. It is masked as the call
// statement __sgo_nursery(n) followed by the block, which go/format lays out as two lines, and is
// restored afterwards.

const sgoNurseryMask = "__sgo_nursery"

// check c (SGO-ERR-004) is masked as the statement __sgo_check followed by c, so go/format lays out
// c exactly as it would without check (no change of expression depth); go/format puts the mask on
// its own line (or keeps "; " in an init position), and the restore joins it back to check c.
const sgoCheckMask = "__sgo_check"

var checkRestore = regexp.MustCompile(sgoCheckMask + `(?:; *|\n[ \t]*)`)

var nurseryRestore = regexp.MustCompile(sgoNurseryMask + `\(([A-Za-z_][A-Za-z0-9_]*)?\)\n\t*\{`)

func cmdFmt(args []string) error {
	files := args
	if len(files) == 0 {
		var err error
		if files, err = filepath.Glob("*.sgo"); err != nil {
			return err
		}
		sort.Strings(files)
	}
	failed := false
	for _, f := range files {
		if strings.HasPrefix(f, "-") {
			return usage("unknown flag %s for sgo fmt", f)
		}
		if filepath.Ext(f) != ".sgo" {
			return unadmittedInput(f)
		}
		src, err := os.ReadFile(f)
		if err != nil {
			return usage("cannot read %s", f)
		}
		out, err := formatSgo(f, src)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			failed = true
			continue
		}
		if !bytes.Equal(out, src) {
			if err := os.WriteFile(f, out, 0o666); err != nil {
				return err
			}
		}
	}
	if failed {
		return buildFailed
	}
	return nil
}

// formatSgo returns the formatted source of one .sgo file.
func formatSgo(name string, src []byte) ([]byte, error) {
	for _, m := range []string{sgoNurseryMask, sgoCheckMask} {
		if bytes.Contains(src, []byte(m)) {
			return nil, &toolErr{name + ":1:1", "SGO_E_SYNTAX", "the identifier " + m + " is reserved by sgo fmt", 1}
		}
	}
	masked, err := maskNurseries(name, src)
	if err != nil {
		return nil, err
	}
	out, err := format.Source(masked)
	if err != nil {
		if list, ok := err.(scanner.ErrorList); ok && len(list) > 0 {
			e := list[0]
			return nil, &toolErr{fmt.Sprintf("%s:%d:%d", name, e.Pos.Line, e.Pos.Column), "SGO_E_SYNTAX", e.Msg, 1}
		}
		return nil, &toolErr{name + ":1:1", "SGO_E_SYNTAX", err.Error(), 1}
	}
	out = nurseryRestore.ReplaceAllFunc(out, func(m []byte) []byte {
		name := nurseryRestore.FindSubmatch(m)[1]
		if len(name) == 0 {
			return []byte("nursery {")
		}
		return []byte("nursery (" + string(name) + ") {")
	})
	out = checkRestore.ReplaceAll(out, []byte("check "))
	if bytes.Contains(out, []byte(sgoNurseryMask)) || bytes.Contains(out, []byte(sgoCheckMask)) {
		return nil, &toolErr{name + ":1:1", "SGO_E_SYNTAX", "a nursery statement or check could not be formatted", 1}
	}
	return out, nil
}

// maskNurseries rewrites every "nursery ( name ) {" and "nursery {" (tokens, so strings and comments
// are untouched) as "__sgo_nursery(name); {".
func maskNurseries(name string, src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file := fset.AddFile(name, -1, len(src))
	var s scanner.Scanner
	var errs scanner.ErrorList
	s.Init(file, src, func(pos token.Position, msg string) { errs.Add(pos, msg) }, 0)
	type tok struct {
		pos token.Pos
		tok token.Token
		lit string
	}
	var toks []tok
	for {
		pos, t, lit := s.Scan()
		if t == token.EOF {
			break
		}
		if t == token.SEMICOLON && lit == "\n" {
			continue // automatic semicolon
		}
		toks = append(toks, tok{pos, t, lit})
	}
	if len(errs) > 0 {
		e := errs[0]
		return nil, &toolErr{fmt.Sprintf("%s:%d:%d", name, e.Pos.Line, e.Pos.Column), "SGO_E_SYNTAX", e.Msg, 1}
	}
	var out bytes.Buffer
	last := 0
	for i := 0; i < len(toks); i++ {
		if toks[i].tok == token.IDENT && toks[i].lit == "check" && i+1 < len(toks) &&
			(toks[i+1].tok == token.IDENT || toks[i+1].tok == token.LPAREN) {
			start := file.Offset(toks[i].pos)
			out.Write(src[last:start])
			out.WriteString(sgoCheckMask + ";")
			last = start + len("check")
			continue
		}
		if toks[i].tok != token.IDENT || toks[i].lit != "nursery" {
			continue
		}
		start := file.Offset(toks[i].pos)
		switch {
		case i+4 < len(toks) && toks[i+1].tok == token.LPAREN && toks[i+2].tok == token.IDENT &&
			toks[i+3].tok == token.RPAREN && toks[i+4].tok == token.LBRACE:
			out.Write(src[last:start])
			out.WriteString(sgoNurseryMask + "(" + toks[i+2].lit + "); ")
			last = file.Offset(toks[i+4].pos)
			i += 4
		case i+1 < len(toks) && toks[i+1].tok == token.LBRACE:
			out.Write(src[last:start])
			out.WriteString(sgoNurseryMask + "(); ")
			last = file.Offset(toks[i+1].pos)
			i++
		default:
			p := fset.Position(toks[i].pos)
			return nil, &toolErr{fmt.Sprintf("%s:%d:%d", name, p.Line, p.Column), "SGO_E_SYNTAX", "nursery is a reserved word; expected nursery (name) { ... }", 1}
		}
	}
	out.Write(src[last:])
	return out.Bytes(), nil
}
