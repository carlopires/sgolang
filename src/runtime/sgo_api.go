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

// sgo_write writes all of s to fd 1, atomically with respect to other calls (SGO-PROC-003). It
// returns the number of bytes written and whether the whole string was written.
//
//go:linkname sgo_write sgo.runtime_write
func sgo_write(s string) (int, bool) {
	if sgoLab != nil {
		sgoLabPoint()
		sgoLabWriteText(s)
	}
	p := unsafe.Pointer(unsafe.StringData(s))
	done := 0
	lock(&sgoStdoutLock)
	for done < len(s) {
		chunk := len(s) - done
		if chunk > 1<<30 {
			chunk = 1 << 30
		}
		n := write1(1, add(p, uintptr(done)), int32(chunk))
		if n == -_EINTR {
			continue
		}
		if n <= 0 {
			break
		}
		done += int(n)
	}
	unlock(&sgoStdoutLock)
	return done, done == len(s)
}
