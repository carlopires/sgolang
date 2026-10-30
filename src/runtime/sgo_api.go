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

//go:linkname sgo_isCurrent sgo.runtime_isCurrent
func sgo_isCurrent(task unsafe.Pointer) bool {
	return task != nil && getg().sgo == (*sgoTask)(task)
}

//go:linkname sgo_violation sgo.runtime_violation
func sgo_violation(msg string) {
	panic(error(&sgoViolation{msg}))
}

var sgoStdoutLock mutex

// sgo_write writes all of s to fd 1, atomically with respect to other calls (SGO-PROC-003). It
// returns the number of bytes written and whether the whole string was written.
//
//go:linkname sgo_write sgo.runtime_write
func sgo_write(s string) (int, bool) {
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
