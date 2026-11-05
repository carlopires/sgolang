// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Runtime side of package sgo (W03): definitions pushed to the sgo package through linkname.
// Only package sgo (digest-pinned trusted code, SGO-TCB-002) declares these functions.

package runtime

import "unsafe"

//go:linkname sgo_spawn sgo.runtime_spawn
func sgo_spawn(scope unsafe.Pointer, fn func(task unsafe.Pointer) error) error {
	// func(unsafe.Pointer) error and func(*sgoTask) error have the same ABI (one pointer argument);
	// reinterpreting the func value avoids a heap closure, which the runtime forbids. TCB-reviewed.
	return sgoSpawn((*sgoScope)(scope), *(*func(*sgoTask) error)(unsafe.Pointer(&fn)))
}

// sgo_spawnResult is sgo_spawn for a child that publishes a result (sgo.Go, Group.Go): when the
// nursery or an ancestor is cancelling and no child is admitted, it returns the scope's
// cancellation error, which the caller publishes as the result, so no returned handle stays
// unresolved (SGO17-01). The admission decision is sgoAdmit's, under the scope lock.
//
//go:linkname sgo_spawnResult sgo.runtime_spawnResult
func sgo_spawnResult(scope unsafe.Pointer, fn func(task unsafe.Pointer) error) (notAdmitted, err error) {
	s := (*sgoScope)(scope)
	c, err := sgoAdmit(s, *(*func(*sgoTask) error)(unsafe.Pointer(&fn)), nil, 0)
	if err != nil || c != nil {
		return nil, err
	}
	return sgoScopeCancellation(s), nil
}

//go:linkname sgo_isRoot sgo.runtime_isRoot
func sgo_isRoot(task unsafe.Pointer) bool {
	t := (*sgoTask)(task)
	return t != nil && t.id == 0 && t.scope == nil
}

// sgo_currentIsRoot reports whether the calling goroutine is the root task (root-only calls on a
// nursery's scope, SGO-RES-001).
//
//go:linkname sgo_currentIsRoot sgo.runtime_currentIsRoot
func sgo_currentIsRoot() bool {
	t := getg().sgo
	return t != nil && t.id == 0 && t.scope == nil
}

//go:linkname sgo_isCurrent sgo.runtime_isCurrent
func sgo_isCurrent(task unsafe.Pointer) bool {
	return task != nil && getg().sgo == (*sgoTask)(task)
}

// sgo_cancel is n.Cancel() (SGO-CANCEL-001).
//
//go:linkname sgo_cancel sgo.runtime_cancel
func sgo_cancel(scope unsafe.Pointer) {
	if sgoLab != nil {
		sgoLabPoint()
	}
	if err := sgoNurseryCancel((*sgoScope)(scope)); err != nil {
		panic(err)
	}
}

// sgo_checkpoint is task.Checkpoint() after the context-ownership check (SGO-CANCEL-002).
//
//go:linkname sgo_checkpoint sgo.runtime_checkpoint
func sgo_checkpoint(task unsafe.Pointer) error {
	if sgoLab != nil {
		sgoLabPoint()
	}
	return sgoTaskCancellation((*sgoTask)(task))
}

// sgo_sleep is task.Sleep(ms) after the argument and ownership checks (SGO-WAIT-001).
//
//go:linkname sgo_sleep sgo.runtime_sleep
func sgo_sleep(task unsafe.Pointer, ms int) error {
	return sgoSleepFor((*sgoTask)(task), int64(ms))
}

// sgo_isCancelled is sgo.IsCancelled (SGO-CANCEL-003).
//
//go:linkname sgo_isCancelled sgo.runtime_isCancelled
func sgo_isCancelled(err error) bool {
	return sgoIsCancelled(err)
}

//go:linkname sgo_violation sgo.runtime_violation
func sgo_violation(msg string) {
	panic(error(&sgoViolation{msg}))
}

// sgo_fail unwinds with a checked failure of the given kind (SGO-DEF-003).
//
//go:linkname sgo_fail sgo.runtime_fail
func sgo_fail(kind, msg string) {
	panic(error(&sgoFailure{kind, msg, ""}))
}

// sgoPanicValue is the payload of panic(msg) in S-Go code: a Panic failure with the position of
// the call (SGO-DEF-002, SGO-ERR-002). The compiler passes the position.
func sgoPanicValue(msg, pos string) any {
	return error(&sgoFailure{"Panic", msg, pos})
}

// Checked int arithmetic failures (SGO-VAL-001), called by compiled S-Go code with the position of
// the operation (cmd/compile/internal/ssagen/sgo.go).

func sgoOverflow(pos string) {
	panic(error(&sgoFailure{"IntegerOverflow", "integer overflow", pos}))
}

func sgoDivideByZero(pos string) {
	panic(error(&sgoFailure{"DivideByZero", "integer divide by zero", pos}))
}

func sgoShiftRange(pos string) {
	panic(error(&sgoFailure{"ShiftRange", "shift count outside 0..63", pos}))
}

// sgoBoundsViolation is an array index outside 0..n-1 in S-Go code (SGO-COMP-003, D91), called by
// compiled S-Go code with the position of the index expression, the index, and the length.
func sgoBoundsViolation(pos string, i, n int) {
	panic(error(&sgoFailure{"BoundsViolation", "index " + sgoIntString(i) + " out of range for length " + sgoIntString(n), pos}))
}

// sgoSliceBoundsViolation is a slice or string bound outside its range in S-Go code
// (SGO-COMP-005): bound i against limit n (the length, capacity, or the next bound).
func sgoSliceBoundsViolation(pos string, i, n int) {
	panic(error(&sgoFailure{"BoundsViolation", "slice bound " + sgoIntString(i) + " out of range for " + sgoIntString(n), pos}))
}

// sgoNilDereference is a dereference of a nil pointer in S-Go code (SGO-COMP-007).
func sgoNilDereference(pos string) {
	panic(error(&sgoFailure{"NilDereference", "dereference of a nil pointer", pos}))
}

// sgoNilResource is a return of a nil resource result with a nil error (SGO-ERR-004, W21 review Z4).
func sgoNilResource(pos string) {
	panic(error(&sgoFailure{"InvalidArgument", "a function returned a nil resource with a nil error", pos}))
}

// sgoNilInterfaceCall is a method call on a nil interface value in S-Go code (SGO-IFACE-003, D130).
func sgoNilInterfaceCall(pos string) {
	panic(error(&sgoFailure{"NilDereference", "method call on a nil interface value", pos}))
}

// sgoIntString formats v in decimal.
func sgoIntString(v int) string {
	var buf [24]byte
	if v < 0 {
		return "-" + string(itoa(buf[:], uint64(-v)))
	}
	return string(itoa(buf[:], uint64(v)))
}

var sgoStdoutLock mutex

// sgo_write writes all of s to fd 1 or fd 2 (standard output or error), atomically with respect to
// other calls (SGO-PROC-003, D178). It returns the number of bytes written and nil, or an I/O error:
// BrokenPipe when the reader has gone (SIGPIPE does not end the process), IOError otherwise.
//
//go:linkname sgo_write sgo.runtime_write
func sgo_write(fd int, s string) (int, error) {
	if sgoLab != nil {
		sgoLabPoint()
		if fd == 1 {
			sgoLabWriteText(s)
		}
	}
	p := unsafe.Pointer(unsafe.StringData(s))
	done := 0
	errno := int32(0)
	lock(&sgoStdoutLock)
	for done < len(s) {
		chunk := len(s) - done
		if chunk > 1<<30 {
			chunk = 1 << 30
		}
		n := write1(uintptr(fd), add(p, uintptr(done)), int32(chunk))
		if n == -_EINTR {
			continue
		}
		if n <= 0 {
			errno = -n
			break
		}
		done += int(n)
	}
	unlock(&sgoStdoutLock)
	if done == len(s) {
		return done, nil
	}
	op := "Stdout.Write"
	if fd == 2 {
		op = "Stderr.Write"
	}
	if errno == sgoEPIPE {
		return done, &sgoIOError{kind: "BrokenPipe", op: op, msg: "the reader has gone", progress: done}
	}
	return done, &sgoIOError{kind: "IOError", op: op, progress: done}
}

// sgo_isBrokenPipe reports whether err is a write to standard output or error whose reader has
// gone (D178).
//
//go:linkname sgo_isBrokenPipe sgo.runtime_isBrokenPipe
func sgo_isBrokenPipe(err error) bool {
	e, ok := any(err).(*sgoIOError)
	return ok && e.kind == "BrokenPipe"
}

// sgo_isTerminal reports whether fd (0, 1, or 2) is a terminal; never in the lab (D178).
//
//go:linkname sgo_isTerminal sgo.runtime_isTerminal
func sgo_isTerminal(fd int) bool {
	if sgoLab != nil {
		return false
	}
	var buf [128]byte // a termios
	return sgoIoctl(int32(fd), sgoTIOCGETA, unsafe.Pointer(&buf[0])) == 0
}

// sgo_terminalWidth is the width in columns of the terminal on fd, or 0 when fd is not a terminal
// or reports none; always 0 in the lab (D178).
//
//go:linkname sgo_terminalWidth sgo.runtime_terminalWidth
func sgo_terminalWidth(fd int) int {
	if sgoLab != nil {
		return 0
	}
	var ws [4]uint16 // rows, cols, xpixel, ypixel
	if sgoIoctl(int32(fd), sgoTIOCGWINSZ, unsafe.Pointer(&ws[0])) != 0 {
		return 0
	}
	return int(ws[1])
}
