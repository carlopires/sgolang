// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// sgo test (W10, D62, D64; spec 0.6.0 draft): the tests of a package are the functions
//
//	func TestX(t *sgo.T) error
//
// in its _test.sgo files (X empty or not starting with a lower-case letter), which are compiled
// into the package only here. sgo test builds one test binary per package: the package with its
// test files, and a generated main that asks sgo.StartTest which test it was started for. Each
// test then runs in its own process (D62 option a) with its selection on fd 3 (never the
// environment) and a fresh temporary directory, served to it as T.TempDir and removed afterwards.
// A test passes if it returns nil and leaves no obligation unresolved (D64); a test still running
// after testTimeout is killed and fails.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// testTimeout is the watchdog of one test process (D62).
const testTimeout = 60 * time.Second

// testsFailed reports that at least one test failed; the results were already printed.
var testsFailed = &toolErr{status: 1}

func cmdTest(args []string) error {
	verbose := false
	var run *regexp.Regexp
	var dirs []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-v":
			verbose = true
		case a == "-run" || strings.HasPrefix(a, "-run="):
			val, ok := strings.CutPrefix(a, "-run=")
			if !ok {
				if i+1 == len(args) {
					return usage("flag -run needs a value")
				}
				i++
				val = args[i]
			}
			re, err := regexp.Compile(val)
			if err != nil {
				return usage("-run: invalid regular expression %q: %v", val, err)
			}
			run = re
		case strings.HasPrefix(a, "-"):
			return usage("unknown flag %s for sgo test (sgo test [-v] [-run regexp] [packages])", a)
		default:
			dirs = append(dirs, a)
		}
	}
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	return testPackages(dirs, verbose, run)
}

func testPackages(dirs []string, verbose bool, run *regexp.Regexp) error {
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
	failed := false
	for _, rel := range rels {
		ok, err := t.testPackage(m, rel, goos, goarch, verbose, run)
		if err != nil {
			return err
		}
		failed = failed || !ok
	}
	if failed {
		return testsFailed
	}
	return nil
}

// testFunc is one discovered test.
type testFunc struct {
	name string
	pos  string // file:line:1 of the declaration
}

var (
	testDecl = regexp.MustCompile(`(?m)^func[ \t]+(Test[^a-z(\s][A-Za-z0-9_]*|Test)[ \t]*\(`)
	testSig  = regexp.MustCompile(`^func[ \t]+Test[A-Za-z0-9_]*[ \t]*\([ \t]*[A-Za-z_][A-Za-z0-9_]*[ \t]+\*[ \t]*sgo\.T[ \t]*\)[ \t]*error[ \t]*\{`)
)

// discoverTests returns the tests of the package's _test.sgo files in source order.
func discoverTests(m *module, p *pkg) ([]testFunc, error) {
	var tests []testFunc
	for _, f := range p.files {
		if !strings.HasSuffix(f, "_test.sgo") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(m.root, f))
		if err != nil {
			return nil, err
		}
		for _, loc := range testDecl.FindAllSubmatchIndex(src, -1) {
			line := 1 + bytes.Count(src[:loc[0]], []byte("\n"))
			pos := fmt.Sprintf("%s:%d:1", f, line)
			end := bytes.IndexByte(src[loc[0]:], '\n')
			decl := src[loc[0]:]
			if end >= 0 {
				decl = decl[:end]
			}
			if !testSig.Match(decl) {
				return nil, &toolErr{pos, "SGO_E_TEST_SIGNATURE", "a test must be declared func " + string(src[loc[2]:loc[3]]) + "(t *sgo.T) error (D62)", 1}
			}
			tests = append(tests, testFunc{string(src[loc[2]:loc[3]]), pos})
		}
	}
	return tests, nil
}

// testMain is the generated main of a test binary: it runs the test StartTest selects.
func testMain(pkgPath, pkgName string, tests []testFunc) string {
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n\t\"sgo\"\n\n\t\"" + pkgPath + "\"\n)\n\n") // named imports are not admitted (SGO-BIND-002)
	b.WriteString("func main(root *sgo.Context) error {\n\tt, i := sgo.StartTest(root)\n\tswitch i {\n")
	for i, tf := range tests {
		fmt.Fprintf(&b, "\tcase %d:\n\t\treturn %s.%s(t)\n", i, pkgName, tf.name)
	}
	b.WriteString("\t}\n\treturn sgo.NewError(\"sgo test: no such test\")\n}\n")
	return b.String()
}

// testPackage builds the test binary of the package at rel and runs its selected tests. It
// reports whether every test passed.
func (t *toolchain) testPackage(m *module, rel, goos, goarch string, verbose bool, run *regexp.Regexp) (bool, error) {
	start := time.Now()
	tm := *m
	tm.testDir = rel
	pkgs, err := tm.loadGraph(rel)
	if err != nil {
		return false, err
	}
	p := pkgs[len(pkgs)-1]
	tests, err := discoverTests(&tm, p)
	if err != nil {
		return false, err
	}
	if len(tests) == 0 {
		fmt.Printf("?   \t%s\t[no test files]\n", p.path)
		return true, nil
	}
	name := packageName(filepath.Join(m.root, p.files[0]))
	switch name {
	case "main":
		return false, &toolErr{tests[0].pos, "SGO_E_USAGE", "tests of package main are not supported; move the code under test to a library package (W10)", 2}
	case "sgo":
		return false, &toolErr{tests[0].pos, "SGO_E_USAGE", "a package named sgo cannot be tested: the generated test main imports package sgo (D62)", 2}
	}
	archives, err := t.buildTCB(goos, goarch)
	if err != nil {
		return false, err
	}
	work, err := os.MkdirTemp("", "sgo-test-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(work)
	env := t.toolEnv(goos, goarch)
	for _, q := range pkgs {
		a, err := t.compileUser(&tm, q, false, false, archives, work, env)
		if err != nil {
			return false, err
		}
		archives[q.path] = a
	}
	mainSrc := filepath.Join(work, "testmain.sgo")
	if err := os.WriteFile(mainSrc, []byte(testMain(p.path, name, tests)), 0o666); err != nil {
		return false, err
	}
	importcfg := filepath.Join(work, "testmain.importcfg")
	if err := writeImportcfg(importcfg, archives); err != nil {
		return false, err
	}
	mainArchive := filepath.Join(work, "testmain.a")
	// -sgotestmain: only this generated package may call sgo.StartTest (D69).
	if err := t.runTool(work, env, "compile", "-sgotestmain", "-o", mainArchive, "-p", "main", "-importcfg", importcfg,
		"-pack", "-complete", "-trimpath", work+"=>", "testmain.sgo"); err != nil {
		return false, err
	}
	exe := filepath.Join(work, "test")
	id := t.buildInfo(goos, goarch, 0, m.grants, ioDigest(m.ioGrants), false, true)
	if err := t.link(archives, mainArchive, exe, id, 0, m.grants, m.ioGrants, false, true, env); err != nil {
		return false, err
	}

	ok := true
	for i, tf := range tests {
		if run != nil && !run.MatchString(tf.name) {
			continue
		}
		if verbose {
			fmt.Printf("=== RUN   %s\n", tf.name)
		}
		r, err := runTest(exe, m.root, i)
		if err != nil {
			return false, err
		}
		status := "PASS"
		if !r.passed {
			status, ok = "FAIL", false
		}
		fmt.Printf("--- %s: %s (%.2fs)\n", status, tf.name, r.wall.Seconds())
		if !r.passed || verbose {
			for _, l := range strings.Split(strings.TrimRight(string(r.output), "\n"), "\n") {
				if l != "" {
					fmt.Printf("    %s\n", strings.TrimPrefix(l, "    "))
				}
			}
			if r.hang {
				fmt.Printf("    test killed after %v (D62 watchdog)\n", testTimeout)
			}
		}
	}
	result := "ok  "
	if !ok {
		result = "FAIL"
	}
	fmt.Printf("%s\t%s\t%.3fs\n", result, p.path, time.Since(start).Seconds())
	return ok, nil
}

type testResult struct {
	passed, hang bool
	output       []byte // stdout and stderr, interleaved as written
	wall         time.Duration
}

// runTest runs test index of the binary exe in its own process, in the module root, with a fresh
// temporary directory.
func runTest(exe, dir string, index int) (*testResult, error) {
	tmp, err := os.MkdirTemp("", "sgo-tmp-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if r, err := filepath.EvalSymlinks(tmp); err == nil {
		tmp = r
	}
	sel := filepath.Join(tmp, ".selection")
	sdir := filepath.Join(tmp, "t")
	if err := os.Mkdir(sdir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(sel, []byte(fmt.Sprintf("test %d\ntmp %s\n", index, sdir)), 0o600); err != nil {
		return nil, err
	}
	sf, err := os.Open(sel)
	if err != nil {
		return nil, err
	}
	defer sf.Close()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdin = nil
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.ExtraFiles = []*os.File{sf} // fd 3
	start := time.Now()
	err = cmd.Run()
	r := &testResult{output: out.Bytes(), wall: time.Since(start)}
	var ee *exec.ExitError
	switch {
	case ctx.Err() != nil:
		r.hang = true
	case err == nil:
		r.passed = true
	case errors.As(err, &ee):
	default:
		return nil, err
	}
	return r, nil
}
