// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// sgo check [-json] [packages] (D63): the front end of sgo build (parsing, admission, type
// checking, and the S-Go checks) over each package and its imports, with no code generation and
// no linking. Each package is compiled with -sgocheck, which writes only export data for its
// dependents, so check and build report the same diagnostics, codes, and positions (T03).
// Packages are directories, or "./..." for every package of the module; the default is ".".
func cmdCheck(args []string) error {
	jsonOut := false
	var dirs []string
	for _, a := range args {
		switch {
		case a == "-json":
			jsonOut = true
		case strings.HasPrefix(a, "-"):
			return usage("unknown flag %s for sgo check (sgo check [-json] [packages])", a)
		default:
			dirs = append(dirs, a)
		}
	}
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	err := checkPackages(dirs, jsonOut)
	var te *toolErr
	if jsonOut && errors.As(err, &te) {
		printJSONDiag(te.pos, te.code, te.msg)
		return buildFailed
	}
	return err
}

func checkPackages(dirs []string, jsonOut bool) error {
	t, err := findToolchain()
	if err != nil {
		return err
	}
	goos, goarch := runtime.GOOS, runtime.GOARCH
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if r, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = r
	}
	m, err := findModule(cwd, t)
	if err != nil {
		return err
	}
	if err := t.checkModulePath(m, goos, goarch); err != nil {
		return err
	}
	rels, err := m.resolveDirs(cwd, dirs)
	if err != nil {
		return err
	}
	archives, err := t.buildTCB(goos, goarch)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "sgo-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	env := t.toolEnv(goos, goarch)
	done := map[string]bool{}
	failed := false
	for _, rel := range rels {
		pkgs, err := m.loadGraph(rel)
		if err != nil {
			return err
		}
		for _, p := range pkgs {
			if done[p.path] {
				continue
			}
			done[p.path] = true
			out, ok, err := t.checkPkg(m, p, archives, work, env)
			if err != nil {
				return err
			}
			if jsonOut {
				printJSONDiags(out)
			} else {
				os.Stderr.Write(out)
			}
			if !ok {
				failed = true
				break // dependents of a package that does not check cannot be checked
			}
		}
	}
	if failed {
		return buildFailed
	}
	return nil
}

// checkPkg runs the front end on one package: the compiler with -sgocheck writes an archive with
// only the package's export data, which its dependents import.
func (t *toolchain) checkPkg(m *module, p *pkg, archives map[string]string, work string, env []string) ([]byte, bool, error) {
	name := fmt.Sprintf("c%03d", p.order)
	importcfg := filepath.Join(work, name+".importcfg")
	if err := writeImportcfg(importcfg, archives); err != nil {
		return nil, false, err
	}
	pkgpath := p.path
	if p.mod.main && packageName(filepath.Join(p.mod.root, p.files[0])) == "main" {
		pkgpath = "main"
	}
	archive := filepath.Join(work, name+".a")
	args := append([]string{"-sgocheck", "-o", archive, "-p", pkgpath, "-importcfg", importcfg, "-pack", "-complete",
		"-trimpath", p.mod.trim()}, p.files...)
	cmd := exec.Command(t.tool("compile"), args...)
	cmd.Dir, cmd.Env = p.mod.root, env
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return out.Bytes(), false, nil
		}
		return nil, false, err
	}
	archives[p.path] = archive
	return out.Bytes(), true, nil
}

// packageDirs returns every directory of the module holding .sgo files (excluding testdata and
// directories whose name starts with "." or "_"), relative to the module root, sorted.
// resolveDirs maps package arguments (directories, or ./... for every package of the module) to
// directories relative to the module root.
func (m *module) resolveDirs(cwd string, dirs []string) ([]string, error) {
	var rels []string
	for _, d := range dirs {
		if d == "./..." {
			all, err := m.packageDirs()
			if err != nil {
				return nil, err
			}
			rels = append(rels, all...)
			continue
		}
		abs := d
		if !filepath.IsAbs(d) {
			abs = filepath.Join(cwd, d)
		}
		if r, err := filepath.EvalSymlinks(abs); err == nil {
			abs = r
		}
		st, err := os.Stat(abs)
		if err != nil || !st.IsDir() {
			return nil, usage("no package %s", d)
		}
		rel, err := filepath.Rel(m.root, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil, usage("%s is outside the module %s", abs, m.root)
		}
		rels = append(rels, rel)
	}
	return rels, nil
}

func (m *module) packageDirs() ([]string, error) {
	seen := map[string]bool{}
	err := filepath.WalkDir(m.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() && path != m.root && (name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
			return filepath.SkipDir
		}
		if d.IsDir() && path != m.root {
			if _, err := os.Stat(filepath.Join(path, "sgo.mod")); err == nil {
				return filepath.SkipDir // another module
			}
		}
		if !d.IsDir() && strings.HasSuffix(name, ".sgo") && !strings.HasSuffix(name, "_test.sgo") {
			rel, _ := filepath.Rel(m.root, filepath.Dir(path))
			seen[rel] = true
		}
		return nil
	})
	var dirs []string
	for d := range seen {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs, err
}

var packageClause = regexp.MustCompile(`(?m)^package\s+([A-Za-z_][A-Za-z0-9_]*)`)

// packageName returns the package clause's name of an S-Go file ("" if it has none).
func packageName(file string) string {
	b, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	if m := packageClause.FindSubmatch(b); m != nil {
		return string(m[1])
	}
	return ""
}

var diagLine = regexp.MustCompile(`^(.+?):(\d+):(\d+): ([A-Z][A-Z0-9_]+): (.*)$`)

// printJSONDiags prints compiler output as JSON lines: diagnostics with file, line, column,
// code, and message; any other line as a message.
func printJSONDiags(out []byte) {
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		if m := diagLine.FindStringSubmatch(line); m != nil {
			l, _ := strconv.Atoi(m[2])
			c, _ := strconv.Atoi(m[3])
			emitJSON(map[string]any{"file": m[1], "line": l, "column": c, "code": m[4], "message": m[5]})
			continue
		}
		emitJSON(map[string]any{"message": line})
	}
}

func printJSONDiag(pos, code, msg string) {
	if m := diagLine.FindStringSubmatch(pos + ": " + code + ": " + msg); m != nil {
		printJSONDiags([]byte(m[0]))
		return
	}
	emitJSON(map[string]any{"position": pos, "code": code, "message": msg})
}

func emitJSON(v map[string]any) {
	b, _ := json.Marshal(v)
	os.Stdout.Write(append(b, '\n'))
}
