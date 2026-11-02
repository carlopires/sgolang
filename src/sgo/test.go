// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Tests (W10, D62; spec 0.6.0 draft). A test is a function TestX(t *sgo.T) error in a _test.sgo
// file. sgo test runs each test in its own process: the generated main asks StartTest which test
// it was started for, runs it with the root task, and returns its result. A nil result passes; an
// error or a failure fails the test, and so does an obligation the test leaves unresolved (D64).

package sgo

import "unsafe"

func runtime_testStart(task unsafe.Pointer) (index, grant int, ok bool)
func runtime_testLog(s string)

// T is the state of the running test. It belongs to the root task, which runs the test.
type T struct {
	c   *Context
	dir *Dir
}

// StartTest returns the test state and the index of the selected test. It is called by the main
// function sgo test generates; outside a test binary it is a MissingCapability failure.
func StartTest(root *Context) (*T, int) {
	root.own()
	i, g, ok := runtime_testStart(root.t)
	if !ok {
		runtime_fail("MissingCapability", "StartTest: not the root task of a test binary")
	}
	return &T{c: root, dir: &Dir{g: g}}, i
}

// Context returns the root context of the test.
func (t *T) Context() *Context { return t.c }

// TempDir returns the test's temporary directory, a read-write directory grant that exists only
// for this test and is removed by sgo test when the test ends.
func (t *T) TempDir() *Dir {
	t.c.own()
	return t.dir
}

// Log writes s to the test's log, shown when the test fails or with sgo test -v.
func (t *T) Log(s string) {
	t.c.own()
	runtime_testLog(s)
}
