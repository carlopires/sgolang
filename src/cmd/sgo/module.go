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
	"slices"
	"sort"
	"strconv"
	"strings"
)

// module is the module containing the working directory (SGO-IMP-001).
type module struct {
	root     string   // absolute directory containing sgo.mod
	path     string   // module path
	pathAt   string   // position of the module directive in sgo.mod
	grants   []string // granted effect kinds, in canonical order (SGO-AUTH-002)
	ioGrants []string // file and network grants, canonical and sorted (iogrant.go)
	testDir  string   // sgo test: the package whose _test.sgo files are loaded ("" otherwise)

	requires []modReq           // require lines of sgo.mod (SGO-IMP-003)
	replaces map[string]string  // replace lines: module path => directory relative to the root
	mods     map[string]*modSrc // every module of the build, by path (resolveModules)
	tc       *toolchain         // the toolchain that read sgo.mod
}

// grantKinds are the effect kinds a grant line may name (SGO-AUTH-003), in canonical order.
var grantKinds = []string{"clock", "wallclock", "random", "entropy", "simstore"}

// pkg is one package of the module: the .sgo files of one directory.
type pkg struct {
	path    string      // import path
	mod     *modSrc     // the module the package belongs to
	dir     string      // directory relative to its module root ("." for the root)
	files   []string    // relative to its module root
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
			m := &module{root: d, tc: t}
			return m, m.parseMod(t)
		}
		if filepath.Dir(d) == d {
			return nil, usage("no sgo.mod in %s or any parent directory (SGO-IMP-001)", dir)
		}
	}
}

// parseMod reads sgo.mod: the directives "module <path>" and "sgo <spec-version>", once each,
// and "grant <kind>" lines naming effect kinds once each (SGO-AUTH-002).
func (m *module) parseMod(t *toolchain) error {
	f, err := os.Open(filepath.Join(m.root, "sgo.mod"))
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sawSgo := false
	granted := map[string]bool{}
	replaceAt := map[string]string{} // replace lines by module path: their positions
	required := map[string]bool{}
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
			m.path, m.pathAt = fields[1], at
		case fields[0] == "sgo" && len(fields) == 2 && !sawSgo:
			sawSgo = true
			if fields[1] != t.specVersion {
				return &toolErr{at, "SGO_E_SPEC_VERSION", fmt.Sprintf("module requires S-Go spec %s; this toolchain implements %s", fields[1], t.specVersion), 1}
			}
		case fields[0] == "grant" && len(fields) >= 2 && slices.Contains([]string{"dir", "connect", "listen"}, fields[1]):
			if _, err := m.parseIOGrant(fields, text, at); err != nil {
				return err
			}
		case fields[0] == "grant":
			if len(fields) != 2 || !slices.Contains(grantKinds, fields[1]) {
				return &toolErr{at, "SGO_E_USAGE", fmt.Sprintf("%q does not grant an effect kind (grant one of %s) (SGO-AUTH-002)", text, strings.Join(grantKinds, ", ")), 2}
			}
			if granted[fields[1]] {
				return &toolErr{at, "SGO_E_USAGE", fmt.Sprintf("%s is granted twice (SGO-AUTH-002)", fields[1]), 2}
			}
			granted[fields[1]] = true
		case fields[0] == "require":
			r, err := parseRequire(fields, text, at, required)
			if err != nil {
				return err
			}
			m.requires = append(m.requires, r)
		case fields[0] == "replace":
			if len(fields) != 4 || fields[2] != "=>" || filepath.IsAbs(fields[3]) {
				return &toolErr{at, "SGO_E_USAGE", fmt.Sprintf("%q: want replace <module path> => <relative dir> (SGO-IMP-003)", text), 2}
			}
			if _, dup := m.replaces[fields[1]]; dup {
				return &toolErr{at, "SGO_E_USAGE", fmt.Sprintf("%s is replaced twice (SGO-IMP-003)", fields[1]), 2}
			}
			if stdPath(fields[1]) {
				return &toolErr{at, "SGO_E_USAGE", fmt.Sprintf("%s cannot be replaced: std is never required, replaced, or vendored (SGO-IMP-003, SGO-IMP-004)", fields[1]), 2}
			}
			replaceAt[fields[1]] = at
			if m.replaces == nil {
				m.replaces = map[string]string{}
			}
			m.replaces[fields[1]] = filepath.FromSlash(fields[3])
		default:
			return &toolErr{at, "SGO_E_UNADMITTED", fmt.Sprintf("sgo.mod directive %q is not admitted (module, sgo, grant, require, and replace only)", text), 1}
		}
	}
	for _, k := range grantKinds {
		if granted[k] {
			m.grants = append(m.grants, k)
		}
	}
	if m.path == "" {
		return &toolErr{"sgo.mod:1:1", "SGO_E_UNADMITTED", "sgo.mod has no module directive", 1}
	}
	if at, ok := replaceAt[m.path]; ok {
		return &toolErr{at, "SGO_E_USAGE", fmt.Sprintf("%s is the main module and cannot be replaced (SGO-IMP-003)", m.path), 2}
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

// importPath maps a directory relative to the module root to its import path.
func (s *modSrc) importPath(dir string) string {
	if dir == "." {
		return s.path
	}
	return s.path + "/" + filepath.ToSlash(dir)
}

// loadGraph loads the main package in dir (relative to the main module root) and every package it
// imports from the main module, the required modules, and std (SGO-IMP-001 v2), checks inputs and
// package clauses, rejects import cycles, and returns the packages in dependency order (imports
// first). The modules are resolved and their sgo.sum entries checked first (SGO-IMP-003).
func (m *module) loadGraph(dir string) ([]*pkg, error) {
	if err := m.resolveModules(m.tc); err != nil {
		return nil, err
	}
	pkgs := map[string]*pkg{}
	var load func(s *modSrc, dir string, from *importRef) (*pkg, error)
	load = func(s *modSrc, dir string, from *importRef) (*pkg, error) {
		ip := s.importPath(dir)
		if p := pkgs[ip]; p != nil {
			return p, nil
		}
		p, err := m.loadPkg(s, dir, from)
		if err != nil {
			return nil, err
		}
		pkgs[ip] = p
		for _, imp := range p.imports {
			t := m.modOf(imp.path)
			rel := "."
			if imp.path != t.path {
				rel = filepath.FromSlash(strings.TrimPrefix(imp.path, t.path+"/"))
			}
			if _, err := load(t, rel, &imp); err != nil {
				return nil, err
			}
		}
		return p, nil
	}
	if _, err := load(m.mods[m.path], dir, nil); err != nil {
		return nil, err
	}
	if err := checkCycles(pkgs); err != nil {
		return nil, err
	}
	return depOrder(pkgs), nil
}

// loadPkg reads the package clauses and imports of one directory's .sgo files in module s.
func (m *module) loadPkg(s *modSrc, dir string, from *importRef) (*pkg, error) {
	abs := filepath.Join(s.root, dir)
	missing := func(why string) error {
		if from != nil {
			return &toolErr{from.pos, "SGO_E_UNDEFINED", fmt.Sprintf("package %s %s", from.path, why), 1}
		}
		return usage("no .sgo files in %s", dir)
	}
	ents, err := os.ReadDir(abs)
	if err != nil || len(ents) == 0 {
		if from != nil {
			if s.main {
				return nil, missing("is not in the module")
			}
			return nil, missing(fmt.Sprintf("does not exist in module %s", s.path))
		}
		return nil, usage("cannot read package directory %s", dir)
	}
	p := &pkg{path: s.importPath(dir), mod: s, dir: dir}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := filepath.Join(dir, e.Name())
		if name == "sgo.mod" || name == "sgo.sum" || s.std && name == "MANIFEST" {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext == ".sgo" {
			if strings.HasSuffix(e.Name(), "_test.sgo") && (!s.main || m.testDir == "" || filepath.Clean(dir) != filepath.Clean(m.testDir)) {
				continue // test files belong only to sgo test of their own package (D62)
			}
			p.files = append(p.files, filepath.Clean(name))
		} else if sourceExts[ext] {
			return nil, unadmittedInput(filepath.Clean(name))
		}
	}
	if len(p.files) == 0 {
		return nil, missing("has no .sgo files")
	}
	sort.Strings(p.files)
	if s.std {
		if err := checkStdPackage(s.root, dir, p.files); err != nil {
			return nil, err
		}
	}

	fset := token.NewFileSet()
	var pkgName string
	for _, f := range p.files {
		src, err := os.ReadFile(filepath.Join(s.root, f))
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
			if err != nil || ip == "sgo" || path.Clean(ip) != ip {
				continue // sgo is the TCB; the compiler rejects a malformed path
			}
			pos := fset.Position(is.Path.Pos()).String()
			if !s.main {
				pos = s.path + "/" + pos // positions name the module they are in
			}
			t := m.modOf(ip)
			switch {
			case s.std && (t == nil || !t.std):
				// std imports only std and sgo (SGO-IMP-004, D115), whatever the main module requires.
				return nil, &toolErr{pos, "SGO_E_UNADMITTED", fmt.Sprintf("import of %q by the standard library (std imports only std and sgo) is not admitted (SGO-IMP-004)", ip), 1}
			case t == nil:
				if first, _, _ := strings.Cut(ip, "/"); strings.Contains(first, ".") {
					return nil, &toolErr{pos, "SGO_E_UNDEFINED", fmt.Sprintf("package %s is in no module of the build; require its module in sgo.mod (SGO-IMP-001, SGO-IMP-003)", ip), 1}
				}
				// Go standard-library, C, and unsafe imports are refused by the compiler.
			case s.std && !t.std:
				// A std package importing a user module is refused by the compiler (D115).
			default:
				p.imports = append(p.imports, importRef{ip, pos})
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
