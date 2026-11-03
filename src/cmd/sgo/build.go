// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func cmdBuild(args []string) error {
	f, err := parseBuildFlags("build", args)
	if err != nil {
		return err
	}
	if len(f.args) > 1 {
		return usage("sgo build takes at most one package")
	}
	var dir string
	if len(f.args) == 1 {
		dir = f.args[0]
	}
	out := f.out
	_, err = buildProgram(dir, f.target, f.drain, false, func(m *module, main *pkg) string {
		if out == "" {
			out = path_base(main.path)
		}
		return out
	})
	return err
}

func cmdRun(args []string) error {
	f, err := parseBuildFlags("run", args)
	if err != nil {
		return err
	}
	if len(f.args) != 1 {
		return usage("sgo run takes exactly one main package")
	}
	tmp, err := os.MkdirTemp("", "sgo-run-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	exe, err := buildProgram(f.args[0], "", f.drain, false, func(*module, *pkg) string { return filepath.Join(tmp, "prog") })
	if err != nil {
		return err
	}
	cmd := exec.Command(exe)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			os.RemoveAll(tmp)
			os.Exit(ee.ExitCode()) // SGO-TOOL-001: run propagates the exit status
		}
		return err
	}
	return nil
}

func path_base(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// buildProgram compiles the main package named by arg (a directory, default ".") and every module
// package it imports, links the program, and returns the output path. Any error leaves no output.
// lab builds a lab binary (sgo lab, D41): S-Go code is compiled and linked with -sgolab.
func buildProgram(arg, target string, drain int64, lab bool, output func(*module, *pkg) string) (string, error) {
	t, err := findToolchain()
	if err != nil {
		return "", err
	}
	goos, goarch, err := checkTarget(target)
	if err != nil {
		return "", err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// The compiler records file positions under its own resolved working directory, so the module
	// root used for -trimpath must be symlink-free too (e.g. /tmp -> /private/tmp on darwin);
	// otherwise runtime-record positions would be absolute (SGO-ERR-002).
	if r, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = r
	}
	dir := cwd
	if arg != "" {
		st, err := os.Stat(arg)
		switch {
		case err != nil:
			return "", usage("no package %s", arg)
		case !st.IsDir() && filepath.Ext(arg) != ".sgo":
			return "", unadmittedInput(filepath.Clean(arg))
		case !st.IsDir():
			arg = filepath.Dir(arg)
		}
		if filepath.IsAbs(arg) {
			dir = filepath.Clean(arg)
		} else {
			dir = filepath.Join(cwd, arg)
		}
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			dir = r
		}
	}
	m, err := findModule(dir, t)
	if err != nil {
		return "", err
	}
	if err := t.checkModulePath(m, goos, goarch); err != nil {
		return "", err
	}
	rel, err := filepath.Rel(m.root, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", usage("%s is outside the module %s", dir, m.root)
	}
	pkgs, err := m.loadGraph(rel)
	if err != nil {
		return "", err
	}
	main := pkgs[len(pkgs)-1] // dependency order: the requested package is last

	archives, err := t.buildTCB(goos, goarch)
	if err != nil {
		return "", err
	}
	work, err := os.MkdirTemp("", "sgo-build-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)
	env := t.toolEnv(goos, goarch)
	for _, p := range pkgs {
		a, err := t.compileUser(m, p, p == main, lab, archives, work, env)
		if err != nil {
			return "", err
		}
		archives[p.path] = a
	}
	exe := filepath.Join(work, "prog")
	lastBuildID = t.buildInfo(goos, goarch, drain, m.grants, ioDigest(m.ioGrants), lab, false)
	if err := t.link(archives, archives[main.path], exe, lastBuildID, drain, m.grants, m.ioGrants, lab, false, env); err != nil {
		return "", err
	}
	out := output(m, main)
	if !filepath.IsAbs(out) {
		out = filepath.Join(cwd, out)
	}
	return out, moveFile(exe, out)
}

// lastBuildID is the build ID of the program buildProgram built last (the lab trace header, D46).
var lastBuildID string

func writeImportcfg(path string, archives map[string]string) error {
	var keys []string
	for k := range archives {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b bytes.Buffer
	for _, k := range keys {
		fmt.Fprintf(&b, "packagefile %s=%s\n", k, archives[k])
	}
	return os.WriteFile(path, b.Bytes(), 0o666)
}

// compileUser compiles one module package under subset 0. Files are passed relative to the module
// root, so diagnostics and runtime-record positions are module-relative (SGO-ERR-001/002).
func (t *toolchain) compileUser(m *module, p *pkg, isMain, lab bool, archives map[string]string, work string, env []string) (string, error) {
	name := fmt.Sprintf("p%03d", p.order)
	importcfg := filepath.Join(work, name+".importcfg")
	if err := writeImportcfg(importcfg, archives); err != nil {
		return "", err
	}
	pkgpath := p.path
	if isMain {
		pkgpath = "main"
	}
	archive := filepath.Join(work, name+".a")
	args := append([]string{"-o", archive, "-p", pkgpath, "-importcfg", importcfg, "-pack", "-complete",
		"-trimpath", p.mod.trim()}, p.files...)
	if lab {
		args = append([]string{"-sgolab"}, args...)
	}
	return archive, t.runTool(p.mod.root, env, "compile", args...)
}

// buildInfo is the build metadata recorded in every binary (SGO-BUILD-001, SGO-TOOL-001): spec
// version, admission digest, drain budget, root grants (SGO-AUTH-002), target, toolchain
// identity, and upstream base commit. It is the binary's build ID, which the linker writes
// verbatim in every executable format. A lab binary's ID also names its configuration (D41).
func (t *toolchain) buildInfo(goos, goarch string, drain int64, grants []string, io string, lab, test bool) string {
	budget := "none"
	if drain > 0 {
		budget = fmt.Sprint(drain)
	}
	g := "none"
	if len(grants) > 0 {
		g = strings.Join(grants, ",")
	}
	config := ""
	switch {
	case lab && test:
		config = "/config=lab-test"
	case lab:
		config = "/config=lab"
	case test:
		config = "/config=test" // a test binary (D62): never mistaken for a production program
	}
	if io != "" {
		g += "/io=" + io // digest of the file and network grants (D52)
	}
	return fmt.Sprintf("sgo1/spec=%s/admission=%s%s/drain=%s/grants=%s/target=%s-%s/toolchain=%s/upstream=%s",
		t.specVersion, t.admission, config, budget, g, goos, goarch, t.version, t.upstream)
}

// link links the program; a drain budget is passed with the linker's dedicated -sgodrainbudget
// flag, which sets the runtime's budget (-X is refused for S-Go programs).
func (t *toolchain) link(archives map[string]string, main, exe, buildInfo string, drain int64, grants, ioGrants []string, lab, test bool, env []string) error {
	importcfg := exe + ".importcfg"
	if err := writeImportcfg(importcfg, archives); err != nil {
		return err
	}
	args := []string{"-o", exe, "-importcfg", importcfg, "-buildmode=exe", "-buildid=" + buildInfo}
	if drain > 0 {
		args = append(args, fmt.Sprintf("-sgodrainbudget=%d", drain))
	}
	if len(grants) > 0 {
		args = append(args, "-sgogrants="+strings.Join(grants, ","))
	}
	if len(ioGrants) > 0 {
		args = append(args, "-sgoiogrants="+strings.Join(ioGrants, "\n"))
	}
	if lab {
		args = append(args, "-sgolab")
	}
	if test {
		args = append(args, "-sgotest")
	}
	return t.runTool(filepath.Dir(exe), env, "link", append(args, main)...)
}

// moveFile moves src to dst, copying across file systems; dst appears only when complete.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp := dst + ".sgo-tmp"
	if err := os.WriteFile(tmp, data, 0o777); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
