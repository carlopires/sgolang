// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// module is the module containing the working directory (SGO-IMP-001).
type module struct {
	root string // absolute directory containing sgo.mod
	path string // module path
}

// pkg is one package of the module: the .sgo files of one directory.
type pkg struct {
	path    string // import path
	dir     string // directory relative to the module root ("." for the root)
	files   []string
	imports []importRef // imports of module packages, in source order
	order   int         // position in dependency order
}

type importRef struct {
	path string
	pos  string // file:line:col of the import path literal
}

// sourceExts are file kinds a Go toolchain would compile or link. In an S-Go module they are not
// admitted inputs (SGO-TOOL-001, SGO-NATIVE-001); other files (data, notes) are not inputs.
var sourceExts = map[string]bool{
	".go": true, ".s": true, ".S": true, ".sx": true, ".c": true, ".h": true, ".cc": true, ".cpp": true,
	".cxx": true, ".hh": true, ".hpp": true, ".hxx": true, ".m": true, ".f": true, ".F": true, ".for": true,
	".f90": true, ".syso": true, ".a": true, ".o": true, ".so": true, ".dylib": true, ".swig": true, ".swigcxx": true,
}

func unadmittedInput(name string) *toolErr {
	return &toolErr{name + ":1:1", "SGO_E_UNADMITTED", "only .sgo files are S-Go inputs (SGO-TOOL-001)", 1}
}

// findModule finds the module root above dir and checks sgo.mod (SGO-IMP-001).
func findModule(dir string, t *toolchain) (*module, error) {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "sgo.mod")); err == nil {
			m := &module{root: d}
			return m, m.parseMod(t)
		}
		if filepath.Dir(d) == d {
			return nil, usage("no sgo.mod in %s or any parent directory (SGO-IMP-001)", dir)
		}
	}
}

// parseMod reads sgo.mod: exactly the directives "module <path>" and "sgo <spec-version>".
func (m *module) parseMod(t *toolchain) error {
	f, err := os.Open(filepath.Join(m.root, "sgo.mod"))
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sawSgo := false
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if i := strings.Index(text, "//"); i >= 0 {
			text = strings.TrimSpace(text[:i])
		}
		if text == "" {
			continue
		}
		at := fmt.Sprintf("sgo.mod:%d:1", line)
		fields := strings.Fields(text)
		switch {
		case fields[0] == "module" && len(fields) == 2 && m.path == "":
			m.path = fields[1]
		case fields[0] == "sgo" && len(fields) == 2 && !sawSgo:
			sawSgo = true
			if fields[1] != t.specVersion {
				return &toolErr{at, "SGO_E_SPEC_VERSION", fmt.Sprintf("module requires S-Go spec %s; this toolchain implements %s", fields[1], t.specVersion), 1}
			}
		default:
			return &toolErr{at, "SGO_E_UNADMITTED", fmt.Sprintf("sgo.mod directive %q is not admitted (module and sgo only)", text), 1}
		}
	}
	if m.path == "" {
		return &toolErr{"sgo.mod:1:1", "SGO_E_UNADMITTED", "sgo.mod has no module directive", 1}
	}
	if !sawSgo {
		return &toolErr{"sgo.mod:1:1", "SGO_E_SPEC_VERSION", "sgo.mod has no sgo directive naming the spec version", 1}
	}
	return sc.Err()
}

// importPath maps a directory relative to the root to its import path.
func (m *module) importPath(dir string) string {
	if dir == "." {
		return m.path
	}
	return m.path + "/" + filepath.ToSlash(dir)
}

// loadGraph loads the main package in dir (relative to the root) and every module package it
// imports, checks inputs and package clauses, rejects import cycles, and returns the packages in
// dependency order (imports first).
func (m *module) loadGraph(dir string) ([]*pkg, error) {
	pkgs := map[string]*pkg{}
	var load func(dir string, from *importRef) (*pkg, error)
	load = func(dir string, from *importRef) (*pkg, error) {
		ip := m.importPath(dir)
		if p := pkgs[ip]; p != nil {
			return p, nil
		}
		p, err := m.loadPkg(dir, from)
		if err != nil {
			return nil, err
		}
		pkgs[ip] = p
		for _, imp := range p.imports {
			rel := "."
			if imp.path != m.path {
				rel = filepath.FromSlash(strings.TrimPrefix(imp.path, m.path+"/"))
			}
			if _, err := load(rel, &imp); err != nil {
				return nil, err
			}
		}
		return p, nil
	}
	if _, err := load(dir, nil); err != nil {
		return nil, err
	}
	if err := checkCycles(pkgs); err != nil {
		return nil, err
	}
	return depOrder(pkgs), nil
}

// loadPkg reads the package clauses and imports of one directory's .sgo files.
func (m *module) loadPkg(dir string, from *importRef) (*pkg, error) {
	abs := filepath.Join(m.root, dir)
	ents, err := os.ReadDir(abs)
	if err != nil || len(ents) == 0 {
		if from != nil {
			return nil, &toolErr{from.pos, "SGO_E_UNDEFINED", fmt.Sprintf("package %s is not in the module", from.path), 1}
		}
		return nil, usage("cannot read package directory %s", dir)
	}
	p := &pkg{path: m.importPath(dir), dir: dir}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := filepath.Join(dir, e.Name())
		if name == "sgo.mod" {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext == ".sgo" {
			p.files = append(p.files, filepath.Clean(name))
		} else if sourceExts[ext] {
			return nil, unadmittedInput(filepath.Clean(name))
		}
	}
	if len(p.files) == 0 {
		if from != nil {
			return nil, &toolErr{from.pos, "SGO_E_UNDEFINED", fmt.Sprintf("package %s has no .sgo files", from.path), 1}
		}
		return nil, usage("no .sgo files in %s", dir)
	}
	sort.Strings(p.files)

	fset := token.NewFileSet()
	var pkgName string
	for _, f := range p.files {
		src, err := os.ReadFile(filepath.Join(m.root, f))
		if err != nil {
			return nil, err
		}
		af, err := parser.ParseFile(fset, f, src, parser.ImportsOnly|parser.SkipObjectResolution)
		if af == nil || err != nil && af.Name == nil {
			continue // the compiler reports the syntax error
		}
		if pkgName == "" {
			pkgName = af.Name.Name
		} else if af.Name.Name != pkgName {
			return nil, &toolErr{fset.Position(af.Package).String(), "SGO_E_PACKAGE_CLAUSE",
				fmt.Sprintf("package %s; the other files of %s are package %s (SGO-BIND-001)", af.Name.Name, p.path, pkgName), 1}
		}
		for _, is := range af.Imports {
			ip, err := strconv.Unquote(is.Path.Value)
			if err != nil {
				continue
			}
			if ip == m.path || strings.HasPrefix(ip, m.path+"/") {
				if path.Clean(ip) != ip {
					continue // the compiler rejects the malformed path
				}
				p.imports = append(p.imports, importRef{ip, fset.Position(is.Path.Pos()).String()})
			}
		}
	}
	return p, nil
}

// checkCycles reports one import cycle, at the import in the cycle's lexically first package
// (by import path) that names the next package of the cycle (SGO-IMP-001).
func checkCycles(pkgs map[string]*pkg) error {
	const (
		unvisited = iota
		active
		done
	)
	state := map[string]int{}
	var stack []string
	var cycle []string
	var visit func(ip string) bool
	visit = func(ip string) bool {
		state[ip] = active
		stack = append(stack, ip)
		for _, imp := range pkgs[ip].imports {
			switch state[imp.path] {
			case active:
				for i := len(stack) - 1; i >= 0; i-- {
					if stack[i] == imp.path {
						cycle = append([]string(nil), stack[i:]...)
						return true
					}
				}
			case unvisited:
				if visit(imp.path) {
					return true
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[ip] = done
		return false
	}
	paths := make([]string, 0, len(pkgs))
	for ip := range pkgs {
		paths = append(paths, ip)
	}
	sort.Strings(paths)
	for _, ip := range paths {
		if state[ip] == unvisited && visit(ip) {
			break
		}
	}
	if cycle == nil {
		return nil
	}
	first := 0
	for i, ip := range cycle {
		if ip < cycle[first] {
			first = i
		}
	}
	from, next := cycle[first], cycle[(first+1)%len(cycle)]
	for _, imp := range pkgs[from].imports {
		if imp.path == next {
			rotated := append(append([]string(nil), cycle[first:]...), cycle[:first]...)
			return &toolErr{imp.pos, "SGO_E_IMPORT_CYCLE", "import cycle: " + strings.Join(append(rotated, from), " -> "), 1}
		}
	}
	return fmt.Errorf("sgo: internal error locating import cycle %v", cycle)
}

// depOrder returns the packages with every import before its importer, ties by import path.
func depOrder(pkgs map[string]*pkg) []*pkg {
	paths := make([]string, 0, len(pkgs))
	for ip := range pkgs {
		paths = append(paths, ip)
	}
	sort.Strings(paths)
	var out []*pkg
	seen := map[string]bool{}
	var visit func(ip string)
	visit = func(ip string) {
		if seen[ip] {
			return
		}
		seen[ip] = true
		for _, imp := range pkgs[ip].imports {
			visit(imp.path)
		}
		pkgs[ip].order = len(out)
		out = append(out, pkgs[ip])
	}
	for _, ip := range paths {
		visit(ip)
	}
	return out
}
