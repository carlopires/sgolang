// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sgo is the S-Go subset-0 API (SGO_SPEC_SUBSET0.md §0.2). It is trusted target code
// (SGO-TCB-002): its sources are pinned in sgo/TARGET_TCB.json, and it reaches the runtime scope core
// (src/runtime/sgo_scope.go) through linknames pushed from the runtime. S-Go programs cannot construct,
// compare, or dereference its types (SGO-TYPE-005); values come only from the runtime.
package sgo

import "unsafe"

// Context is a task's context (SGO-TYPE-005). The root task's context also holds the stdout capability.
type Context struct{ t unsafe.Pointer }

// Nursery is the handle of one nursery statement (SGO-NUR-001).
type Nursery struct{ s unsafe.Pointer }

// Writer is the stdout capability (SGO-PROC-003).
type Writer struct{ _ [0]func() }

var stdout Writer

// Implemented in the runtime (sgo_scope.go).
func runtime_spawn(scope unsafe.Pointer, fn func(task unsafe.Pointer) error) error
func runtime_isRoot(task unsafe.Pointer) bool
func runtime_isCurrent(task unsafe.Pointer) bool
func runtime_write(s string) (int, bool)
func runtime_violation(msg string)
func runtime_fail(kind, msg string) // unwinds with a checked failure of the kind (SGO-DEF-003)

// Implemented in the runtime (sgo_cancel.go).
func runtime_cancel(scope unsafe.Pointer)
func runtime_checkpoint(task unsafe.Pointer) error
func runtime_sleep(task unsafe.Pointer, ms int) error
func runtime_isCancelled(err error) bool

// Spawn admits f as a child of n (SGO-SPAWN-001..004). A refused admission is a
// SpawnContractViolation that unwinds the caller (SGO-SPAWN-002).
func (n *Nursery) Spawn(f func(*Context) error) {
	if f == nil {
		runtime_violation("spawn with a nil callback")
	}
	if err := runtime_spawn(n.s, func(t unsafe.Pointer) error { return f(&Context{t: t}) }); err != nil {
		panic(err)
	}
}

// Cancel requests cancellation of n (SGO-CANCEL-001). The caller must be able to spawn into n;
// otherwise the call is a SpawnContractViolation and has no effect.
func (n *Nursery) Cancel() {
	if n == nil {
		runtime_violation("cancel with a nil nursery handle")
	}
	runtime_cancel(n.s)
}

// Checkpoint returns nil, or a cancellation error if a scope linked to the calling task is
// CANCELLING (SGO-CANCEL-002).
func (c *Context) Checkpoint() error {
	c.own()
	return runtime_checkpoint(c.t)
}

// Sleep blocks for at least ms milliseconds of monotonic time, or returns a cancellation error as
// soon as a scope linked to the calling task is CANCELLING (SGO-WAIT-001).
func (c *Context) Sleep(ms int) error {
	c.own()
	if ms < 0 {
		runtime_fail("InvalidArgument", "Sleep with a negative duration")
	}
	return runtime_sleep(c.t, ms)
}

// Stdout returns the stdout capability; only the root context holds it (SGO-PROC-003).
func (c *Context) Stdout() *Writer {
	c.own()
	if !runtime_isRoot(c.t) {
		runtime_fail("MissingCapability", "Stdout called on a context that does not hold the stdout capability")
	}
	return &stdout
}

// WriteString writes all of s to standard output, unbuffered and atomically (SGO-PROC-003).
func (w *Writer) WriteString(s string) error {
	n, ok := runtime_write(s)
	if !ok {
		return NewError("stdout write failed after " + itoa(n) + " bytes")
	}
	return nil
}

func (c *Context) own() {
	if c == nil || !runtime_isCurrent(c.t) {
		runtime_violation("context used by a task it does not belong to")
	}
}

// NewError returns a new non-nil error whose Error is msg (SGO-TYPE-006).
func NewError(msg string) error { return &errorString{msg} }

type errorString struct{ s string }

func (e *errorString) Error() string { return e.s }

// IsCancelled reports whether err is a cancellation error produced by Checkpoint or Sleep
// (SGO-CANCEL-003).
func IsCancelled(err error) bool { return runtime_isCancelled(err) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
