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

// Writer is the stdout or the stderr capability (SGO-PROC-003, D178).
type Writer struct {
	_  [0]func()
	fd int
}

var (
	stdout = Writer{fd: 1}
	stderr = Writer{fd: 2}
)

// Implemented in the runtime (sgo_scope.go).
func runtime_spawn(scope unsafe.Pointer, fn func(task unsafe.Pointer) error) error
func runtime_isRoot(task unsafe.Pointer) bool
func runtime_isCurrent(task unsafe.Pointer) bool
func runtime_write(fd int, s string) (int, error)
func runtime_stdin() unsafe.Pointer                                // sgo_stdin.go
func runtime_readSecret(h unsafe.Pointer, max int) (string, error) // sgo_stdin.go
func runtime_stdinIsTerminal() bool                                // sgo_stdin.go
func runtime_isBrokenPipe(err error) bool
func runtime_isTerminal(fd int) bool
func runtime_terminalWidth(fd int) int
func runtime_args() []string // sgo_proc.go
func runtime_violation(msg string)
func runtime_fail(kind, msg string) // unwinds with a checked failure of the kind (SGO-DEF-003)

// Implemented in the runtime (sgo_cancel.go).
func runtime_cancel(scope unsafe.Pointer)
func runtime_checkpoint(task unsafe.Pointer) error
func runtime_sleep(task unsafe.Pointer, ms int) error
func runtime_after(task unsafe.Pointer, ms int) <-chan int // sgo_after.go
func runtime_isCancelled(err error) bool
func runtime_cancelAfter(scope unsafe.Pointer, ms int) // sgo_deadline.go
func runtime_deadline(task unsafe.Pointer) (int, bool)
func runtime_isDeadline(err error) bool

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

// After returns a timer for one select arm, case <-t.After(ms): (D160, SGO-WAIT-003). The arm can
// proceed once at least ms milliseconds of monotonic time (virtual time in the lab) have passed;
// it receives ms. The compiler admits After only as the operand of a select receive arm; the timer
// belongs to the task that made it and is dropped when that select ends. It needs no clock grant:
// it reveals elapsed time only by the order in which arms proceed. Each evaluation of the select
// makes a new timer, so After(ms) in a loop restarts the interval whenever another arm wins; a
// deadline that persists across select evaluations is a scope timer (Context.NewTimer, D171).
func (c *Context) After(ms int) <-chan int {
	c.own()
	if ms < 0 {
		runtime_fail("InvalidArgument", "After with a negative duration")
	}
	return runtime_after(c.t, ms)
}

// Stdout returns the stdout capability; only the root context holds it (SGO-PROC-003).
func (c *Context) Stdout() *Writer {
	c.own()
	if !runtime_isRoot(c.t) {
		runtime_fail("MissingCapability", "Stdout called on a context that does not hold the stdout capability")
	}
	return &stdout
}

// Args returns the arguments the program was started with, after the program name: a fresh copy
// on each call (D178, SGO-PROC-004). Only the root context holds them; a child receives what the
// root passes it. Arguments carry no authority of their own.
func (c *Context) Args() []string {
	c.own()
	if !runtime_isRoot(c.t) {
		runtime_fail("MissingCapability", "Args called on a context that is not the root's: children receive what the root passes them")
	}
	return runtime_args()
}

// Stderr returns the stderr capability; only the root context holds it (D178).
func (c *Context) Stderr() *Writer {
	c.own()
	if !runtime_isRoot(c.t) {
		runtime_fail("MissingCapability", "Stderr called on a context that does not hold the stderr capability")
	}
	return &stderr
}

// WriteString writes all of s to the stream, unbuffered and atomically (SGO-PROC-003). A write
// whose reader has gone fails with an error for which IsBrokenPipe holds (D178).
func (w *Writer) WriteString(s string) error {
	_, err := runtime_write(w.fd, s)
	return err
}

// Write writes all of s to the stream, as WriteString does, and returns len(s), or the number of
// bytes written and an error. With it, the stream converts to io.Writer, as an untied value: it
// holds no scope resource (D176).
func (w *Writer) Write(s string) (int, error) {
	return runtime_write(w.fd, s)
}

// IsTerminal reports whether the stream is a terminal; never in the lab (D178).
func (w *Writer) IsTerminal() bool { return runtime_isTerminal(w.fd) }

// TerminalWidth is the width of the terminal in columns, or 0 when the stream is not a terminal
// or reports no width; always 0 in the lab (D178).
func (w *Writer) TerminalWidth() int { return runtime_terminalWidth(w.fd) }

// Input is the stdin capability (D178): the program's standard input, read through the read
// turn and read buffer of every handle (SGO-FILE-004). Every read is a cancellation point. It
// converts to io.Reader and io.LineReader as an untied value: it is never released or closed.
type Input struct{ h unsafe.Pointer }

// Stdin returns the stdin capability; only the root context holds it (D178). A child given the
// *Input may read it.
func (c *Context) Stdin() *Input {
	c.own()
	if !runtime_isRoot(c.t) {
		runtime_fail("MissingCapability", "Stdin called on a context that does not hold the stdin capability")
	}
	return &Input{runtime_stdin()}
}

// Read reads at most max bytes: at least one, or an error (IsEOF at the end of input).
func (in *Input) Read(max int) (string, error) { return runtime_read(in.h, max) }

// ReadLine reads the next line, without its "\n" and a "\r" before it (SGO-FILE-004).
func (in *Input) ReadLine(max int) (string, error) { return runtime_readLine(in.h, max) }

// ReadAll reads the rest of the input, at most max bytes (SGO-FILE-004).
func (in *Input) ReadAll(max int) (string, error) { return runtime_readAll(in.h, max) }

// ReadSecret reads a line as ReadLine does, with the terminal's echo off while it reads when
// standard input is a terminal (a password prompt). The terminal's settings are restored on every
// return, cancellation included.
func (in *Input) ReadSecret(max int) (string, error) { return runtime_readSecret(in.h, max) }

// IsTerminal reports whether standard input is a terminal; never in the lab.
func (in *Input) IsTerminal() bool { return runtime_stdinIsTerminal() }

// IsBrokenPipe reports whether err is a write to standard output or error whose reader has gone,
// as when the program's output is piped into a command that has exited (D178). A program may end
// quietly on it.
func IsBrokenPipe(err error) bool { return runtime_isBrokenPipe(err) }

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

// IsDeadline reports whether err is a cancellation by a deadline (D162): one set with CancelAfter
// on the scope or on an enclosing one. Such an error is also a cancellation (IsCancelled).
func IsDeadline(err error) bool { return runtime_isDeadline(err) }

// CancelAfter sets a deadline on n (D162, D170): ms milliseconds from now, n is cancelled with a
// cancellation that IsDeadline recognizes. Every nested scope, task, and I/O wait under n observes
// it (D165). A later call may only shorten the deadline. The caller's current scope must be n,
// as for Cancel.
func (n *Nursery) CancelAfter(ms int) {
	if n == nil {
		runtime_violation("CancelAfter with a nil nursery handle")
	}
	if ms < 0 {
		runtime_fail("InvalidArgument", "CancelAfter with a negative duration")
	}
	runtime_cancelAfter(n.s, ms)
}

// Deadline returns the milliseconds left before the earliest deadline that applies to the calling
// task (D165), never negative, and whether there is one, so code can pass it on.
func (c *Context) Deadline() (int, bool) {
	c.own()
	return runtime_deadline(c.t)
}

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
