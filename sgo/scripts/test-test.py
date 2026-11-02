#!/usr/bin/env python3
# Copyright 2026 The S-Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
"""test-test.py — sgo test runs each test in its own process (W10, D62, D64, T04).

On scratch modules:
  - a passing test, a test returning an error (its MainError record and T.Log lines shown), a test
    that fails (a checked failure), and a test leaving a permit unresolved (LeakedWork, D64);
  - T.TempDir is a fresh read-write directory per test, removed afterwards; a module directory
    grant still works in a test; root.Stdout output is shown under -v;
  - -run selects tests, -v prints every test; exit status 1 iff a test failed;
  - a package without test files, a Test function with a wrong signature (SGO_E_TEST_SIGNATURE),
    an unknown flag (SGO_E_USAGE, status 2);
  - _test.sgo files are not part of sgo build or sgo check, even when they do not compile;
  - a test that does not end is killed by the 60 s watchdog and fails.

    test-test.py [--sgo PATH] [--no-hang]
"""
import os
import re
import subprocess
import sys
import tempfile

ROOT = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
SPEC_VERSION = open(os.path.join(ROOT, "sgo", "spec", "VERSION")).read().strip()

CALC = """package calc

func Add(a, b int) int {
	return a + b
}
"""

CALC_TEST = """package calc

import "sgo"

func TestAdd(t *sgo.T) error {
	if Add(2, 3) != 5 {
		return sgo.NewError("Add(2, 3) != 5")
	}
	return nil
}

func TestReturnsError(t *sgo.T) error {
	t.Log("about to fail")
	return sgo.NewError("deliberate failure")
}

func TestTempDir(t *sgo.T) error {
	d := t.TempDir()
	if _, err := d.ReadFile("x.txt"); err == nil {
		return sgo.NewError("the temporary directory is not fresh")
	}
	if err := d.WriteFile("x.txt", "hello"); err != nil {
		return err
	}
	s, err := d.ReadFile("x.txt")
	if err != nil {
		return err
	}
	if s != "hello" {
		return sgo.NewError("read back " + s)
	}
	return nil
}

func TestTempDirAgain(t *sgo.T) error {
	if _, err := t.TempDir().ReadFile("x.txt"); err == nil {
		return sgo.NewError("another test's file is visible")
	}
	return nil
}

func TestLeak(t *sgo.T) error {
	ch := sgo.NewChannel[int](1)
	_, err := ch.Reserve(t.Context())
	return err
}

func TestLeakAndError(t *sgo.T) error {
	ch := sgo.NewChannel[int](1)
	if _, err := ch.Reserve(t.Context()); err != nil {
		return err
	}
	return sgo.NewError("error after a leak")
}

func TestGrant(t *sgo.T) error {
	s, err := t.Context().Dir("data").ReadFile("seed.txt")
	if err != nil {
		return err
	}
	if s != "seed\\n" {
		return sgo.NewError("seed.txt: " + s)
	}
	return nil
}

func TestStdout(t *sgo.T) error {
	return t.Context().Stdout().WriteString("written to stdout\\n")
}

func TestDivZero(t *sgo.T) error {
	zero := Add(0, 0)
	if 10/zero == 0 {
		return nil
	}
	return nil
}

func Testing() {}
"""

APP = """package main

import (
	"sgo"

	"example.com/tm/calc"
)

func main(root *sgo.Context) error {
	if calc.Add(1, 1) != 2 {
		return sgo.NewError("bad")
	}
	return nil
}
"""


def write(d, rel, s):
    p = os.path.join(d, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, "w") as f:
        f.write(s)


def main():
    args = sys.argv[1:]
    sgo = os.path.join(ROOT, "pkg", "tool", "darwin_arm64", "sgo")
    if args[:1] == ["--sgo"]:
        sgo, args = args[1], args[2:]
    hang = "--no-hang" not in args
    problems = []

    def check(cond, msg):
        if not cond:
            problems.append(msg)

    with tempfile.TemporaryDirectory() as tmp:
        tmp = os.path.realpath(tmp)
        mod, tdir = os.path.join(tmp, "tm"), os.path.join(tmp, "tmpdir")
        os.makedirs(tdir)
        env = {"PATH": "/usr/bin:/bin", "HOME": os.environ.get("HOME", "/tmp"), "TMPDIR": tdir}
        write(mod, "sgo.mod", "module example.com/tm\n\nsgo " + SPEC_VERSION + "\ngrant dir data data ro\n")
        write(mod, "data/seed.txt", "seed\n")
        write(mod, "calc/calc.sgo", CALC)
        write(mod, "calc/calc_test.sgo", CALC_TEST)
        write(mod, "app/main.sgo", APP)
        write(mod, "other/other.sgo", "package other\n\nfunc One() int {\n\treturn 1\n}\n")

        def sgo_run(*a):
            return subprocess.run([sgo, *a], cwd=mod, env=env, capture_output=True, text=True, timeout=600)

        r = sgo_run("test", "./calc")
        out = r.stdout
        check(r.returncode == 1, f"sgo test ./calc: exit {r.returncode}, want 1: {r.stderr.strip()[:300]!r}")
        want = {"TestAdd": "PASS", "TestReturnsError": "FAIL", "TestTempDir": "PASS", "TestTempDirAgain": "PASS",
                "TestLeak": "FAIL", "TestLeakAndError": "FAIL", "TestGrant": "PASS", "TestStdout": "PASS", "TestDivZero": "FAIL"}
        got = dict((m.group(2), m.group(1)) for m in re.finditer(r"^--- (PASS|FAIL): (\S+) \(", out, re.M))
        check(got == want, f"results {got}, want {want}")
        check(list(got) == list(want), f"tests ran in order {list(got)}, want source order")
        check("Testing" not in got, "Testing (lower case after Test) was run as a test")
        check("    about to fail" in out, "T.Log line of a failing test not shown")
        check('"kind":"MainError","message":"deliberate failure"' in out, "MainError record of a failing test not shown")
        check('"kind":"LeakedWork"' in out and '"kind":"PermitAbandoned"' in out, "leaked permit not reported as LeakedWork with its chain")
        check(re.search(r'"kind":"MainError","message":"error after a leak".*"chain":\[\{"kind":"PermitAbandoned"', out),
              "an error after a leak is not the primary outcome with the abandoned permit in its chain (SGO-TEST-003)")
        check(out.count('"kind":"LeakedWork"') == 1, "LeakedWork reported for a test that returned an error")
        check('"kind":"DivideByZero"' in out or "DivideByZero" in out, "checked failure of a test not shown")
        check("written to stdout" not in out, "output of a passing test shown without -v")
        check(re.search(r"^FAIL\texample\.com/tm/calc\t", out, re.M), "package FAIL line missing")
        check(os.listdir(tdir) == [], f"temporary directories left behind: {os.listdir(tdir)}")

        r = sgo_run("test", "-v", "-run", "Add|Stdout", "./calc")
        check(r.returncode == 0, f"sgo test -v -run: exit {r.returncode}: {r.stdout[-300:]!r} {r.stderr[-300:]!r}")
        check(r.stdout.count("=== RUN") == 2 and "--- PASS: TestAdd" in r.stdout and "--- PASS: TestStdout" in r.stdout,
              f"-v -run output: {r.stdout!r}")
        check("    written to stdout" in r.stdout, "-v does not show a passing test's output")
        check(re.search(r"^ok  \texample\.com/tm/calc\t", r.stdout, re.M), "package ok line missing")

        r = sgo_run("test", "./other")
        check(r.returncode == 0 and "[no test files]" in r.stdout, f"package without tests: {r.returncode} {r.stdout!r}")

        r = sgo_run("test", "-x")
        check(r.returncode == 2 and "SGO_E_USAGE: unknown flag -x" in r.stderr, f"unknown flag: {r.returncode} {r.stderr!r}")

        # Test files are not part of the package outside sgo test, even when they do not compile.
        write(mod, "calc/broken_test.sgo", "package calc\n\nfunc broken() int {\n\treturn \"no\"\n}\n")
        r = sgo_run("build", "-o", os.path.join(tmp, "app"), "./app")
        check(r.returncode == 0, f"sgo build with a broken _test.sgo: {r.returncode} {r.stderr!r}")
        r = sgo_run("check", "./calc", "./app")
        check(r.returncode == 0, f"sgo check with a broken _test.sgo: {r.returncode} {r.stderr!r}")
        r = sgo_run("test", "./calc")
        check(r.returncode == 1 and "broken_test.sgo:4:" in r.stderr, f"sgo test with a broken _test.sgo: {r.returncode} {r.stderr!r}")
        os.remove(os.path.join(mod, "calc", "broken_test.sgo"))

        write(mod, "other/other_test.sgo", "package other\n\nimport \"sgo\"\n\nfunc TestBad(t *sgo.T) {\n}\n")
        r = sgo_run("test", "./other")
        check(r.returncode == 1 and "other/other_test.sgo:5:1: SGO_E_TEST_SIGNATURE:" in r.stderr,
              f"wrong test signature: {r.returncode} {r.stderr!r}")

        if hang:
            write(mod, "other/other_test.sgo", "package other\n\nimport \"sgo\"\n\nfunc TestHang(t *sgo.T) error {\n\treturn t.Context().Sleep(600000)\n}\n")
            r = sgo_run("test", "./other")
            check(r.returncode == 1 and "--- FAIL: TestHang" in r.stdout and "D62 watchdog" in r.stdout,
                  f"hanging test: {r.returncode} {r.stdout!r}")
            check(os.listdir(tdir) == [], f"temporary directories left behind by a killed test: {os.listdir(tdir)}")

    for p in problems:
        print("test-test: " + p)
    if problems:
        print(f"test-test: {len(problems)} problem(s)")
        sys.exit(1)
    print("test-test: ok: pass, error, failure, LeakedWork, TempDir fresh and removed, grants, -v, -run, "
          "no test files, signature, flags, test files outside sgo test" + (", watchdog" if hang else ""))


if __name__ == "__main__":
    main()
