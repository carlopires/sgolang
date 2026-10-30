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

// Cancel requests cancellation of n (SGO-CANCEL-001). Implemented in W04.
func (n *Nursery) Cancel() {
	runtime_violation("Nursery.Cancel is not implemented yet (S-Go W04)")
}

// Checkpoint reports cancellation (SGO-CANCEL-002). Until W04 implements cancellation no scope can
// be cancelled, so it only checks that the context belongs to the calling task.
func (c *Context) Checkpoint() error {
	c.own()
	return nil
}

// Sleep blocks for at least ms milliseconds (SGO-WAIT-001). Cancellation-aware waiting is W04; until
// then Sleep is refused rather than implemented without its cancellation contract.
func (c *Context) Sleep(ms int) error {
	c.own()
	if ms < 0 {
		runtime_fail("InvalidArgument", "Sleep with a negative duration")
	}
	runtime_violation("Context.Sleep is not implemented yet (S-Go W04)")
	return nil
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

// IsCancelled reports whether err is a cancellation error (SGO-CANCEL-003). Cancellation errors are
// produced from W04; until then none exist.
func IsCancelled(err error) bool { return false }

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
