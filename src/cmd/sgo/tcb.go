// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/build"
	"internal/buildcfg"
	"internal/goversion"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// The target TCB (SGO-TCB-002) is the package closure of runtime and sgo. It is compiled from the
// fork's sources by the fork's compiler and assembler; the compiler itself admits each package as
// trusted Go only if every file digest is pinned in sgo/TARGET_TCB.json, so the driver selects
// files but grants nothing. Archives are cached by the identity of the tools, the target, and
// every source file.

type tcbPkg struct {
	path   string
	dir    string
	gofile []string // absolute
	sfile  []string // absolute
	deps   []string
}

// buildContext selects files for a target exactly as the go command would for a CGO_ENABLED=0
// build with the toolchain's default experiments.
func (t *toolchain) buildContext(goos, goarch string) (*build.Context, error) {
	exp, err := buildcfg.ParseGOEXPERIMENT(goos, goarch, "")
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, e := range exp.Enabled() {
		tags = append(tags, "goexperiment."+e)
	}
	switch goarch {
	case "amd64":
		tags = append(tags, "amd64.v1")
	case "arm64":
		tags = append(tags, "arm64.v8.0")
	}
	var release []string
	for i := 1; i <= goversion.Version; i++ {
		release = append(release, fmt.Sprintf("go1.%d", i))
	}
	ctx := build.Default
	ctx.GOROOT, ctx.GOPATH, ctx.GOOS, ctx.GOARCH = t.goroot, "", goos, goarch
	ctx.CgoEnabled, ctx.Compiler, ctx.BuildTags, ctx.ToolTags, ctx.ReleaseTags = false, "gc", nil, tags, release
	return &ctx, nil
}

// loadTCB returns the closure of runtime and sgo in dependency order.
func (t *toolchain) loadTCB(ctx *build.Context) ([]*tcbPkg, error) {
	pkgs := map[string]*tcbPkg{}
	var order []*tcbPkg
	var visit func(path string) error
	visit = func(path string) error {
		if path == "unsafe" || pkgs[path] != nil {
			return nil
		}
		dir := filepath.Join(t.goroot, "src", filepath.FromSlash(path))
		bp, err := ctx.ImportDir(dir, 0)
		if err != nil {
			return fmt.Errorf("sgo: target TCB package %s: %v", path, err)
		}
		if len(bp.CgoFiles)+len(bp.CFiles)+len(bp.SysoFiles) > 0 {
			return fmt.Errorf("sgo: target TCB package %s needs cgo or C files", path)
		}
		p := &tcbPkg{path: path, dir: dir, deps: bp.Imports}
		pkgs[path] = p
		for _, f := range bp.GoFiles {
			p.gofile = append(p.gofile, filepath.Join(dir, f))
		}
		for _, f := range bp.SFiles {
			p.sfile = append(p.sfile, filepath.Join(dir, f))
		}
		for _, d := range bp.Imports {
			if err := visit(d); err != nil {
				return err
			}
		}
		order = append(order, p)
		return nil
	}
	for _, root := range []string{"runtime", "sgo"} {
		if err := visit(root); err != nil {
			return nil, err
		}
	}
	return order, nil
}

func cacheRoot() (string, error) {
	if d := os.Getenv("SGO_CACHE"); d != "" {
		return filepath.Join(d, "sgo-driver"), nil
	}
	d, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "sgo"), nil
}

// buildTCB returns the archive of every target TCB package, building them if needed.
// checkModulePath refuses a module path that equals, contains, or lies under the import path of a
// TCB package (SGO-IMP-001, D38): its packages would shadow trusted ones or import themselves.
func (t *toolchain) checkModulePath(m *module, goos, goarch string) error {
	ctx, err := t.buildContext(goos, goarch)
	if err != nil {
		return err
	}
	pkgs, err := t.loadTCB(ctx)
	if err != nil {
		return err
	}
	paths := []string{"unsafe"}
	for _, p := range pkgs {
		paths = append(paths, p.path)
	}
	for _, p := range paths {
		if m.path == p || strings.HasPrefix(p, m.path+"/") || strings.HasPrefix(m.path, p+"/") {
			return &toolErr{m.pathAt, "SGO_E_USAGE", fmt.Sprintf("module path %s collides with the trusted package path %s; a module path cannot equal, contain, or lie under a TCB package path (SGO-IMP-001)", m.path, p), 2}
		}
	}
	return nil
}

func (t *toolchain) buildTCB(goos, goarch string) (map[string]string, error) {
	ctx, err := t.buildContext(goos, goarch)
	if err != nil {
		return nil, err
	}
	pkgs, err := t.loadTCB(ctx)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s %s/%s %v\n", t.version, goos, goarch, ctx.ToolTags)
	for _, p := range pkgs {
		fmt.Fprintf(h, "pkg %s\n", p.path)
		// The assembler inputs are verified here, the compiler's check covering only Go files; their
		// headers enter the cache key, so an edited header cannot reuse a stale build.
		asm, err := verifyAsm(t.goroot, p.sfile)
		if err != nil {
			return nil, err
		}
		for _, l := range asm {
			fmt.Fprintf(h, "asm %s\n", l)
		}
		for _, f := range append(append([]string(nil), p.gofile...), p.sfile...) {
			data, err := os.ReadFile(f)
			if err != nil {
				return nil, err
			}
			sum := sha256.Sum256(data)
			fmt.Fprintf(h, "%s %x\n", filepath.Base(f), sum)
		}
	}
	root, err := cacheRoot()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, "tcb", hex.EncodeToString(h.Sum(nil))[:24])
	archives := map[string]string{}
	for _, p := range pkgs {
		archives[p.path] = filepath.Join(dir, strings.ReplaceAll(p.path, "/", "%")+".a")
	}
	if tcbCacheValid(dir, archives) {
		return archives, nil
	}
	// A missing, incomplete, or altered entry is rebuilt (W12 R8): the manifest written last lists
	// the sha256 of every archive, and a reused entry must match it byte for byte.
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}

	// Build in a sibling directory and rename it into place, so a cache entry is complete or absent.
	if err := os.MkdirAll(filepath.Dir(dir), 0o777); err != nil {
		return nil, err
	}
	work, err := os.MkdirTemp(filepath.Dir(dir), "tcb-build-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	built := map[string]string{}
	for _, p := range pkgs {
		a, err := t.buildTCBPkg(p, goos, goarch, work, built)
		if err != nil {
			return nil, err
		}
		built[p.path] = a
	}
	var man strings.Builder
	for _, p := range pkgs {
		name := strings.ReplaceAll(p.path, "/", "%") + ".a"
		data, err := os.ReadFile(filepath.Join(work, name))
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&man, "%x  %s\n", sha256.Sum256(data), name)
	}
	if err := os.WriteFile(filepath.Join(work, tcbManifestName), []byte(man.String()), 0o666); err != nil {
		return nil, err
	}
	if err := os.Rename(work, dir); err != nil {
		if !tcbCacheValid(dir, archives) {
			return nil, err // not a concurrent build that finished first
		}
	}
	return archives, nil
}

// tcbManifestName is the last file written into a TCB cache entry: one "sha256  archive" line per
// archive. An entry is reused only if it lists exactly the expected archives and each one's
// contents match (W12 R8); the old empty "complete" marker proved only that a build had finished.
const tcbManifestName = "manifest.sha256"

func tcbCacheValid(dir string, archives map[string]string) bool {
	data, err := os.ReadFile(filepath.Join(dir, tcbManifestName))
	if err != nil {
		return false
	}
	want := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		sum, name, ok := strings.Cut(line, "  ")
		if !ok || len(sum) != 64 {
			return false
		}
		want[name] = sum
	}
	if len(want) != len(archives) {
		return false
	}
	for _, a := range archives {
		sum, ok := want[filepath.Base(a)]
		if !ok {
			return false
		}
		data, err := os.ReadFile(a)
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != sum {
			return false
		}
	}
	return true
}

// buildTCBPkg compiles one package (with assembly when it has .s files) into work.
func (t *toolchain) buildTCBPkg(p *tcbPkg, goos, goarch, work string, built map[string]string) (string, error) {
	name := strings.ReplaceAll(p.path, "/", "%")
	obj := filepath.Join(work, name+".obj")
	if err := os.MkdirAll(obj, 0o777); err != nil {
		return "", err
	}
	archive := filepath.Join(work, name+".a")
	var cfg bytes.Buffer
	for _, d := range p.deps {
		if a, ok := built[d]; ok {
			fmt.Fprintf(&cfg, "packagefile %s=%s\n", d, a)
		}
	}
	importcfg := filepath.Join(obj, "importcfg")
	if err := os.WriteFile(importcfg, cfg.Bytes(), 0o666); err != nil {
		return "", err
	}
	env := t.toolEnv(goos, goarch)
	asmArgs := []string{"-p", p.path, "-std", "-I", obj, "-I", filepath.Join(t.goroot, "src", "runtime"),
		"-D", "GOOS_" + goos, "-D", "GOARCH_" + goarch}
	if goarch == "amd64" {
		asmArgs = append(asmArgs, "-D", "GOAMD64_v1")
	}
	args := []string{"-o", archive, "-p", p.path, "-std", "-lang=go1." + fmt.Sprint(goversion.Version),
		"-importcfg", importcfg, "-pack", "-trimpath", t.goroot + "=>GOROOT"}
	if len(p.sfile) > 0 {
		// As the go command does: -gensymabis runs before compile writes go_asm.h, so an empty
		// go_asm.h satisfies the assembly's #include for that pass.
		if err := os.WriteFile(filepath.Join(obj, "go_asm.h"), nil, 0o666); err != nil {
			return "", err
		}
		symabis := filepath.Join(obj, "symabis")
		gen := append(append([]string{"-gensymabis", "-o", symabis}, asmArgs...), p.sfile...)
		if err := t.runTool(p.dir, env, "asm", gen...); err != nil {
			return "", err
		}
		args = append(args, "-symabis", symabis, "-asmhdr", filepath.Join(obj, "go_asm.h"))
	} else {
		args = append(args, "-complete")
	}
	if err := t.runTool(p.dir, env, "compile", append(args, p.gofile...)...); err != nil {
		return "", err
	}
	var ofiles []string
	for _, s := range p.sfile {
		o := filepath.Join(obj, strings.TrimSuffix(filepath.Base(s), ".s")+".o")
		if err := t.runTool(p.dir, env, "asm", append(append([]string{"-o", o}, asmArgs...), s)...); err != nil {
			return "", err
		}
		ofiles = append(ofiles, o)
	}
	sort.Strings(ofiles)
	return archive, appendArchive(archive, ofiles)
}

// runTool runs a fork tool with the driver's fixed environment; its output goes to stderr.
func (t *toolchain) runTool(dir string, env []string, tool string, args ...string) error {
	cmd := exec.Command(t.tool(tool), args...)
	cmd.Dir, cmd.Env = dir, env
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	os.Stderr.Write(out.Bytes())
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return buildFailed
		}
		return err
	}
	return nil
}

// appendArchive appends object files to a Go archive written by compile -pack (the go command's
// packInternal: ar headers with zero ownership and mode 0644).
func appendArchive(archive string, ofiles []string) error {
	if len(ofiles) == 0 {
		return nil
	}
	f, err := os.OpenFile(archive, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	for _, o := range ofiles {
		data, err := os.ReadFile(o)
		if err != nil {
			f.Close()
			return err
		}
		name := filepath.Base(o)
		if len(name) > 16 {
			name = name[:16]
		}
		fmt.Fprintf(f, "%-16s%-12d%-6d%-6d%-8o%-10d`\n", name, 0, 0, 0, 0o644, len(data))
		f.Write(data)
		if len(data)&1 != 0 {
			f.Write([]byte{0})
		}
	}
	return f.Close()
}
