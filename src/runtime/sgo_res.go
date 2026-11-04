// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Structured resources (W21, D148–D152, SGO-RES-001..004; model sgo/model/scoperes.go).
//
// Every resource is in the list of exactly one scope: a nursery (sgoScope.res) or a task body
// (sgoTask.res). A scope releases its list when it ends, after its join, in reverse order of
// acquisition: a nursery in sgoNurseryRelease (called by the nursery's exit paths after
// sgoNurseryJoin), a task body before its outcome is published (sgoTaskMain, sgoExitMain). Every
// task running inside a resource's scope may use it (D150): the scope, its nested nurseries, and
// the tasks spawned into those, transitively. No task outside can name it (D151, compiler).
//
// A new handle is created in the calling task's body list (sgoHandleNew) and moved by the API
// entry point into the scope the call names (sgoResAdopt). Lists are touched by every task inside
// their scope, so each has its own lock; nothing is allocated under it.

package runtime

import "unsafe"

// sgoResList is a scope's resources in acquisition order (doubly linked through sgoHandle.prev
// and next).
type sgoResList struct {
	lock        mutex
	first, last *sgoHandle
}

const sgoHUser = 7 // a user resource (s.Own, D152): released by calling its Release method

func (l *sgoResList) push(h *sgoHandle) {
	lock(&l.lock)
	h.prev, h.next = l.last, nil
	if l.last != nil {
		l.last.next = h
	} else {
		l.first = h
	}
	l.last = h
	h.list = l
	unlock(&l.lock)
}

// remove unlinks h from its list, if it is in one.
func sgoResUnlink(h *sgoHandle) {
	l := h.list
	if l == nil {
		return
	}
	lock(&l.lock)
	if h.prev != nil {
		h.prev.next = h.next
	} else if l.first == h {
		l.first = h.next
	}
	if h.next != nil {
		h.next.prev = h.prev
	} else if l.last == h {
		l.last = h.prev
	}
	h.prev, h.next, h.list = nil, nil, nil
	unlock(&l.lock)
}

// popLast removes and returns the most recently acquired resource of l, or nil.
func (l *sgoResList) popLast() *sgoHandle {
	lock(&l.lock)
	h := l.last
	if h != nil {
		l.last = h.prev
		if l.last != nil {
			l.last.next = nil
		} else {
			l.first = nil
		}
		h.prev, h.next, h.list = nil, nil, nil
	}
	unlock(&l.lock)
	return h
}

// sgoInside reports whether task t runs inside the scope (nurs, or the body of task body): the
// scope itself, a nursery nested in it, or a task spawned into one of those, transitively.
func sgoInside(t *sgoTask, nurs *sgoScope, body *sgoTask) bool {
	if t == nil {
		return false
	}
	if body != nil && t == body {
		return true
	}
	for c := t.currentScope(); c != nil; c = c.parent {
		if c == nurs || (body != nil && c.owner == body) {
			return true
		}
	}
	return false
}

// sgoEncloses reports whether scope (tn, tb) is the scope (hn, hb) or encloses it.
func sgoEncloses(tn *sgoScope, tb *sgoTask, hn *sgoScope, hb *sgoTask) bool {
	start := hn
	if hb != nil {
		if tb == hb {
			return true
		}
		start = hb.scope // a task body is inside the nursery the task was spawned into
	}
	for c := start; c != nil; c = c.parent {
		if c == tn || (tb != nil && c.owner == tb) {
			return true
		}
	}
	return false
}

// sgoResTarget returns the list of the scope (nurs, body) that the current task acquires into; a
// task outside the scope, or a nursery that is already releasing, fails it with MissingCapability.
func sgoResTarget(nurs *sgoScope, body *sgoTask, op string) *sgoResList {
	t := sgoCurrent()
	if !sgoInside(t, nurs, body) {
		panic(error(&sgoFailure{"MissingCapability", op + ": the task does not run inside the scope it names", ""}))
	}
	if body != nil {
		return &body.res
	}
	lock(&nurs.lock)
	ended := nurs.state >= sgoFinalizing
	unlock(&nurs.lock)
	if ended {
		panic(error(&sgoFailure{"MissingCapability", op + ": the nursery has ended", ""}))
	}
	return &nurs.res
}

// sgoResPlace moves a handle just created by the current task into the scope (nurs, body), whose
// list l the entry point obtained from sgoResTarget before the effect.
func sgoResPlace(h *sgoHandle, l *sgoResList, nurs *sgoScope, body *sgoTask) {
	sgoResUnlink(h)
	h.nurs, h.body = nurs, body
	l.push(h)
}

//go:linkname sgo_resMove sgo.runtime_resMove
func sgo_resMove(h, task, nurs unsafe.Pointer) error {
	n, b := sgoScopeOf(task, nurs)
	return sgoIOResult(sgoResMove((*sgoHandle)(h), n, b), "MoveTo")
}

// sgo_resOwn makes a user resource of scope (task, nurs): release runs when the scope ends (D152).
//
//go:linkname sgo_resOwn sgo.runtime_resOwn
func sgo_resOwn(task, nurs unsafe.Pointer, release func(bool) error) {
	n, b := sgoScopeOf(task, nurs)
	l := sgoResTarget(n, b, "Own")
	h := &sgoHandle{kind: sgoHUser, fd: -1, name: "user resource", release: release, nurs: n, body: b}
	l.push(h)
}

// sgoScopeOf decodes the scope argument of the sgo API: a *sgo.Context's task or a *sgo.Nursery's
// scope (exactly one is non-nil).
func sgoScopeOf(task, nurs unsafe.Pointer) (*sgoScope, *sgoTask) {
	return (*sgoScope)(nurs), (*sgoTask)(task)
}

// sgoResMove moves h to the enclosing scope (nurs, body) (D151): the caller runs inside h's scope
// and the target encloses it, else MissingCapability with no effect.
func sgoResMove(h *sgoHandle, nurs *sgoScope, body *sgoTask) *sgoIOErr {
	t := sgoCurrent()
	if !sgoInside(t, h.nurs, h.body) {
		panic(error(&sgoFailure{"MissingCapability", "MoveTo: the task does not run inside the resource's scope", ""}))
	}
	if h.closed {
		return &sgoIOErr{"HandleClosed", 0}
	}
	if !sgoEncloses(nurs, body, h.nurs, h.body) {
		panic(error(&sgoFailure{"MissingCapability", "MoveTo: the target scope does not enclose the resource's scope", ""}))
	}
	l := &nurs.res
	if body != nil {
		l = &body.res
	}
	sgoResUnlink(h)
	h.nurs, h.body = nurs, body
	l.push(h)
	return nil
}

// sgoReleased is one failed release: its record entry, and the panic value if Release panicked.
type sgoReleased struct {
	info  sgoFailureInfo
	panic any
}

// sgoResRelease releases every resource of l, most recent first; ok is false when the scope is
// unwinding (D152: Release(false)). It returns the failed releases in release order (D149). The
// releases continue after a failed one.
func sgoResRelease(l *sgoResList, ok bool) []sgoReleased {
	var failed []sgoReleased
	for h := l.popLast(); h != nil; h = l.popLast() {
		if h.closed {
			continue
		}
		if h.kind == sgoHUser {
			if r, bad := sgoCallRelease(h, ok); bad {
				failed = append(failed, r)
			}
			continue
		}
		if e := sgoHandleFree(h); e != nil {
			failed = append(failed, sgoReleased{info: sgoFailureInfo{"ReleaseError", "release of " + h.name + ": " + e.kind, ""}})
		}
	}
	return failed
}

// sgoCallRelease runs a user resource's Release (D152); a returned error or a panic is a failed
// release, and a panic does not stop the other releases.
func sgoCallRelease(h *sgoHandle, ok bool) (r sgoReleased, bad bool) {
	h.closed = true
	fn := h.release
	h.release = nil
	defer func() {
		if v := recover(); v != nil {
			r, bad = sgoReleased{info: sgoFailureOf(v), panic: v}, true
		}
	}()
	if err := fn(ok); err != nil {
		return sgoReleased{info: sgoFailureInfo{"ReleaseError", err.Error(), ""}}, true
	}
	return sgoReleased{}, false
}

// sgoReleaseOutcome applies failed releases to a scope ending without a primary failure: the first
// becomes the failure (a recovered Release panic is re-raised as itself, D152), the others join the
// task's chain. With a primary already in flight, every one joins the chain.
func sgoReleaseOutcome(t *sgoTask, failed []sgoReleased, primary bool) any {
	var first any
	for i, r := range failed {
		if i == 0 && !primary {
			if r.panic != nil {
				first = r.panic
			} else {
				first = error(&sgoFailure{"ReleaseError", r.info.msg, ""})
			}
			continue
		}
		t.seq++
		t.entries = append(t.entries, sgoEntry{t.seq, r.info})
	}
	return first
}
