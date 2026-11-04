// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Compiler entry points of the nursery statement (W03) and its unwinding barrier.
//
// The compiler lowers nursery (n) { body } to
//
//	s := sgoNurseryEnter() // hidden temporary
//	n := &sgo.Nursery{s}
//	body
//	sgoNurseryExit(s)
//
// and inserts sgoNurseryExit for every return, break, or continue that leaves the statement
// (SGO-CTL-004). Unwinding (panics and checked failures) does not pass through that code: the
// panic frame walk calls sgoUnwindNurseries before it runs the deferred calls of each frame, so
// each frame's active nurseries complete before its deferred calls (SGO-DEF-005).

package runtime

import (
	"internal/runtime/sys"
	"unsafe"
)

// sgoFailure is a checked failure raised by the runtime on behalf of S-Go code (SGO-DEF-003). pos
// is "file:line:col" when the compiler supplied the position, else empty.
type sgoFailure struct{ kind, msg, pos string }

func (f *sgoFailure) Error() string { return f.kind + ": " + f.msg }

// sgoNurseryEnter opens a nursery owned by the calling task and returns its scope, which the
// compiler wraps in the *sgo.Nursery binder.
func sgoNurseryEnter() unsafe.Pointer {
	s, err := sgoNurseryOpen()
	if err != nil {
		panic(err)
	}
	s.frame = getg().stack.hi - sys.GetCallerSP()
	return unsafe.Pointer(s)
}

// sgoNurseryExit completes a nursery on a normal exit (end of block, or an exit crossing it):
// wait for its children, close it, and apply its outcome (SGO-NUR-003, SGO-NUR-004 rules 2-4).
func sgoNurseryExit(p unsafe.Pointer) {
	s := (*sgoScope)(p)
	r, bad, err := sgoNurseryJoin(s)
	if err != nil {
		panic(err)
	}
	// The nursery's resources are released after the join, before its outcome (SGO-RES-001).
	t := getg().sgo
	// ok: the nursery ends normally, neither failing nor cancelled (D152: a cancelled scope rolls back).
	failed := sgoResRelease(&s.res, r.Outcome == sgoSucceeded && sgoCancelledFrom(s) == nil)
	if r.Outcome == sgoSucceeded {
		if f := sgoReleaseOutcome(t, failed, false); f != nil {
			panic(f) // a failed release: ReleaseError (D149), or a Release that panicked
		}
		return // Rule 4: normal completion.
	}
	sgoReleaseOutcome(t, failed, true)
	// The primary keeps its origin; its chain and the other children's failures follow it in the
	// diagnostic chain (SGO-ERR-002).
	for _, c := range bad {
		if c.id == r.Primary {
			sgoPropagate(t, bad, c)
		}
	}
	if r.Outcome == sgoFailed {
		// Rule 2: unwind with the failure of the failed child with the smallest creation ID.
		panic(r.Panic)
	}
	// Rule 3: unwind with ChildError carrying the errored child's message.
	panic(error(&sgoFailure{"ChildError", r.Err.Error(), ""}))
}

// sgoUnwindNurseries completes, innermost first, the calling task's open nurseries that were
// opened in frames the unwinding is leaving: those at stack offset >= off (the frame whose
// deferred calls run next, or deeper), or all of them if all is set (no frame with deferred calls
// remains). Each is cancelled with cause Unwind before it is joined (SGO-DEF-005,
// SGO-CANCEL-001). The failure being unwound stays primary (SGO-NUR-004 rule 1); join results do
// not raise.
func sgoUnwindNurseries(off uintptr, all bool) {
	t := getg().sgo
	if t == nil {
		return
	}
	for n := len(t.nest); n > 0; n = len(t.nest) {
		s := t.nest[n-1]
		if !all && s.frame < off {
			return
		}
		sgoCancel(s, sgoCause{kind: sgoCauseUnwind})
		r, bad, err := sgoNurseryJoin(s)
		if err != nil {
			throw("sgo: unwinding join failed") // the innermost scope is the caller's own: a defect
		}
		// Released after the join, with ok false: the scope is unwinding (D152, D155).
		sgoReleaseOutcome(t, sgoResRelease(&s.res, false), true)
		// A cancellation unwind (D33) is not a failure: the owner behaves as if the nursery body had
		// returned, so a failed or errored child becomes the primary failure (Rules 2 and 3).
		if u := sgoUnwindingCancel(); u != nil && u.fail == nil && r.Outcome != sgoSucceeded && r.Outcome != sgoCancelled {
			for _, c := range bad {
				if c.id == r.Primary {
					sgoPropagate(t, bad, c)
				}
			}
			sgoUnwindRetarget(t) // the record names the child's origin, as a nursery exit would
			if r.Outcome == sgoFailed {
				u.fail = r.Panic
			} else {
				u.fail = error(&sgoFailure{"ChildError", r.Err.Error(), ""})
			}
			continue
		}
		// Rule 1: the owner's failure stays primary; every child failure joins its chain.
		sgoRecord(t, bad, nil)
	}
}
