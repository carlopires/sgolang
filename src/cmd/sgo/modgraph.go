// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// Packages beyond the main module (SGO-IMP-001 v2, SGO-IMP-003, SGO-IMP-004; D103–D105,
// D111–D115): required modules from local directories, bound to their contents by sgo.sum, and the
// standard library, the implicit module std under $GOROOT/src/sgostd, checked against its MANIFEST.

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// modSrc is one module of a build: the main module, a required module, or std.
type modSrc struct {
	path    string
	root    string // absolute, symlink-free
	version string // required modules: the required version
	main    bool
	std     bool
}

// trim is the -trimpath replacement for the module's root: positions in the main module stay
// relative to its root; those of other modules are prefixed with their module path, so that a
// diagnostic or runtime record names the module it comes from.
// compileFiles names a package's files for the compiler: relative to the main module's root, absolute
// elsewhere, so that -trimpath (trim) rewrites a required module's or std's positions to
// <module path>/<file>.
func (s *modSrc) compileFiles(files []string) []string {
	if s.main {
		return files
	}
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = filepath.Join(s.root, f)
	}
	return out
}

func (s *modSrc) trim() string {
	if s.main {
		return s.root + "=>"
	}
	return s.root + "=>" + s.path
}

// modReq is a require line.
type modReq struct {
	path, version string
	at            string // position of the line
}

var versionRE = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// parseRequire parses "require <path> <version>" (SGO-IMP-003).
func parseRequire(fields []string, text, at string, seen map[string]bool) (modReq, error) {
	if len(fields) != 3 {
		return modReq{}, &toolErr{at, "SGO_E_USAGE", fmt.Sprintf("%q: want require <module path> <version> (SGO-IMP-003)", text), 2}
	}
	if !versionRE.MatchString(fields[2]) {
		return modReq{}, &toolErr{at, "SGO_E_MODULE_VERSION", fmt.Sprintf("version %q of %s is not of the form vMAJOR.MINOR.PATCH (SGO-IMP-003)", fields[2], fields[1]), 1}
	}
	if seen[fields[1]] {
		return modReq{}, &toolErr{at, "SGO_E_USAGE", fmt.Sprintf("%s is required twice (SGO-IMP-003)", fields[1]), 2}
	}
	seen[fields[1]] = true
	return modReq{fields[1], fields[2], at}, nil
}

// stdPath reports whether ip is the standard library or under it.
func stdPath(ip string) bool { return ip == "std" || strings.HasPrefix(ip, "std/") }

// resolveModules finds every module of the build: the main module, std, and the required modules,
// transitively, checking versions, grants, and sgo.sum before anything is compiled.
func (m *module) resolveModules(t *toolchain) error {
	if m.mods != nil {
		return nil
	}
	mods, versions, err := m.walkRequires(t)
	if err != nil {
		return err
	}
	if len(versions) > 0 {
		if err := m.checkSums(mods, versions); err != nil {
			return err
		}
	}
	m.mods = mods
	return nil
}

// walkRequires follows the require lines from the main module (SGO-IMP-003): each module path once
// at one version (D111), its source from the main module's replace line or vendor/ (D112), its
// sgo.mod read by readDependency. It returns every module of the build, std included, and the
// required versions.
func (m *module) walkRequires(t *toolchain) (map[string]*modSrc, map[string]string, error) {
	mods := map[string]*modSrc{m.path: {path: m.path, root: m.root, main: true}}
	stdRoot := filepath.Join(t.goroot, "src", "sgostd")
	if r, err := filepath.EvalSymlinks(stdRoot); err == nil {
		stdRoot = r
	}
	mods["std"] = &modSrc{path: "std", root: stdRoot, std: true}
	versions := map[string]string{}
	queue := slices.Clone(m.requires)
	for len(queue) > 0 {
		r := queue[0]
		queue = queue[1:]
		if r.path == m.path || stdPath(r.path) {
			return nil, nil, &toolErr{r.at, "SGO_E_USAGE", fmt.Sprintf("%s cannot be required: it is the main module or the standard library (SGO-IMP-003)", r.path), 2}
		}
		if v, ok := versions[r.path]; ok {
			if v != r.version {
				return nil, nil, &toolErr{r.at, "SGO_E_MODULE_VERSION", fmt.Sprintf("%s is required at %s and at %s; a module appears at most once in a build (SGO-IMP-003)", r.path, v, r.version), 1}
			}
			continue
		}
		versions[r.path] = r.version
		dir := filepath.Join(m.root, "vendor", filepath.FromSlash(r.path))
		if rp, ok := m.replaces[r.path]; ok {
			dir = filepath.Join(m.root, rp)
		}
		if d, err := filepath.EvalSymlinks(dir); err == nil {
			dir = d
		}
		reqs, err := m.readDependency(t, dir, r)
		if err != nil {
			return nil, nil, err
		}
		mods[r.path] = &modSrc{path: r.path, root: dir, version: r.version}
		queue = append(queue, reqs...)
	}
	return mods, versions, nil
}

// readDependency reads a required module's sgo.mod: its module path must be the required one, its
// spec version the toolchain's, and its grants effect kinds the main module also grants (D105,
// D114). Its replace lines are ignored (D112); its require lines are returned.
func (m *module) readDependency(t *toolchain, dir string, r modReq) ([]modReq, error) {
	shown := dir
	if rel, err := filepath.Rel(m.root, dir); err == nil {
		shown = filepath.ToSlash(rel) // diagnostics name directories relative to the main module
	}
	f, err := os.Open(filepath.Join(dir, "sgo.mod"))
	if err != nil {
		return nil, &toolErr{r.at, "SGO_E_MODULE_MISSING", fmt.Sprintf("%s %s: no sgo.mod in %s (add replace %s => <dir> or vendor it under vendor/%s) (SGO-IMP-003)", r.path, r.version, shown, r.path, r.path), 1}
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	path := ""
	var reqs []modReq
	seen := map[string]bool{}
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if i := strings.Index(text, "//"); i >= 0 {
			text = strings.TrimSpace(text[:i])
		}
		if text == "" {
			continue
		}
		at := fmt.Sprintf("%s/sgo.mod:%d:1", r.path, line)
		fields := strings.Fields(text)
		switch {
		case fields[0] == "module" && len(fields) == 2 && path == "":
			path = fields[1]
		case fields[0] == "sgo" && len(fields) == 2:
			if fields[1] != t.specVersion {
				return nil, &toolErr{at, "SGO_E_SPEC_VERSION", fmt.Sprintf("module %s requires S-Go spec %s; this toolchain implements %s", r.path, fields[1], t.specVersion), 1}
			}
		case fields[0] == "grant" && len(fields) >= 2 && slices.Contains([]string{"dir", "connect", "listen"}, fields[1]):
			return nil, &toolErr{at, "SGO_E_DEPENDENCY_GRANT", fmt.Sprintf("dependency %s declares grant %s; file and network handles reach a library only as arguments (SGO-IMP-003)", r.path, fields[1]), 1}
		case fields[0] == "grant" && len(fields) == 2:
			if !slices.Contains(m.grants, fields[1]) {
				return nil, &toolErr{at, "SGO_E_DEPENDENCY_GRANT", fmt.Sprintf("dependency %s needs grant %s, which the main module %s does not grant (SGO-IMP-003)", r.path, fields[1], m.path), 1}
			}
		case fields[0] == "require":
			q, err := parseRequire(fields, text, at, seen)
			if err != nil {
				return nil, err
			}
			reqs = append(reqs, q)
		case fields[0] == "replace":
			// Only the main module's replacements count (D112).
		default:
			return nil, &toolErr{at, "SGO_E_UNADMITTED", fmt.Sprintf("sgo.mod directive %q is not admitted", text), 1}
		}
	}
	if path != r.path {
		return nil, &toolErr{r.at, "SGO_E_MODULE_MISSING", fmt.Sprintf("%s/sgo.mod declares module %q, not the required %s (SGO-IMP-003)", shown, path, r.path), 1}
	}
	return reqs, sc.Err()
}

// modDigest is the content digest of a module's file tree (SGO-IMP-003): its sgo.mod and .sgo
// files, sorted by slash path relative to the root, each hashed as path NUL length NUL bytes.
// Directories holding another module (their own sgo.mod), vendor, and testdata are not part of it.
func modDigest(root string) (string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root {
				name := d.Name()
				if name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
					return filepath.SkipDir
				}
				if _, err := os.Stat(filepath.Join(p, "sgo.mod")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if rel, _ := filepath.Rel(root, p); rel == "sgo.mod" || filepath.Ext(p) == ".sgo" {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", f, len(b))
		h.Write(b)
	}
	return "h1:" + hex.EncodeToString(h.Sum(nil)), nil
}

// readSums reads sgo.sum: "<path> <version> h1:<hex>" per line.
func readSums(root string) (map[string]string, error) {
	sums := map[string]string{}
	b, err := os.ReadFile(filepath.Join(root, "sgo.sum"))
	if os.IsNotExist(err) {
		return sums, nil
	}
	if err != nil {
		return nil, err
	}
	for i, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 3 || !strings.HasPrefix(f[2], "h1:") {
			return nil, &toolErr{fmt.Sprintf("sgo.sum:%d:1", i+1), "SGO_E_MODULE_SUM", "malformed sgo.sum line; regenerate it with sgo mod sum (SGO-IMP-003)", 1}
		}
		sums[f[0]+" "+f[1]] = f[2]
	}
	return sums, nil
}

// checkSums compares every required module's digest with its sgo.sum entry (D113: never written here).
func (m *module) checkSums(mods map[string]*modSrc, versions map[string]string) error {
	sums, err := readSums(m.root)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(versions))
	for p := range versions {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		d, err := modDigest(mods[p].root)
		if err != nil {
			return err
		}
		want, ok := sums[p+" "+versions[p]]
		switch {
		case !ok:
			return &toolErr{"sgo.sum:1:1", "SGO_E_MODULE_SUM", fmt.Sprintf("no sgo.sum entry for %s %s; run sgo mod sum (SGO-IMP-003)", p, versions[p]), 1}
		case want != d:
			return &toolErr{"sgo.sum:1:1", "SGO_E_MODULE_SUM", fmt.Sprintf("%s %s: the module's files have digest %s, sgo.sum records %s; if the change is intended, run sgo mod sum (SGO-IMP-003)", p, versions[p], d, want), 1}
		}
	}
	return nil
}

// modOf returns the module an import path resolves to: the longest module path that is the
// import path or a prefix of it (SGO-IMP-001 v2), or nil.
func (m *module) modOf(ip string) *modSrc {
	var best *modSrc
	for p, s := range m.mods {
		if (ip == p || strings.HasPrefix(ip, p+"/")) && (best == nil || len(p) > len(best.path)) {
			best = s
		}
	}
	return best
}

// stdManifest is $GOROOT/src/sgostd/MANIFEST: "<sha256 hex>  <slash path>" per file (SGO-IMP-004).
func stdManifest(root string) (map[string]string, error) {
	b, err := os.ReadFile(filepath.Join(root, "MANIFEST"))
	if err != nil {
		return nil, &toolErr{"std:1:1", "SGO_E_STD_MANIFEST", fmt.Sprintf("the standard library has no MANIFEST in %s (SGO-IMP-004)", root), 1}
	}
	man := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) == 2 {
			man[f[1]] = f[0]
		}
	}
	return man, nil
}

// checkStdPackage checks the files of the std package in dir (relative to the std root) against
// the MANIFEST: every .sgo file present with its digest, and no manifest file missing.
func checkStdPackage(root, dir string, files []string) error {
	man, err := stdManifest(root)
	if err != nil {
		return err
	}
	pkgPath := "std/" + filepath.ToSlash(dir)
	for _, f := range files {
		rel := filepath.ToSlash(f)
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		if man[rel] != hex.EncodeToString(sum[:]) {
			return &toolErr{"std/" + rel + ":1:1", "SGO_E_STD_MANIFEST", fmt.Sprintf("%s is not the file the standard library's MANIFEST records; the installation was modified (SGO-IMP-004)", "std/"+rel), 1}
		}
	}
	for rel := range man {
		if filepath.ToSlash(filepath.Dir(filepath.FromSlash(rel))) == filepath.ToSlash(dir) && !slices.Contains(files, filepath.FromSlash(rel)) {
			return &toolErr{"std/" + rel + ":1:1", "SGO_E_STD_MANIFEST", fmt.Sprintf("%s is missing from package %s (SGO-IMP-004)", "std/"+rel, pkgPath), 1}
		}
	}
	return nil
}
