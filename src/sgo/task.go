// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgo

import "unsafe"

// Value-returning tasks (D161, spelling D168; SGO-SPAWN-006 draft).

// Implemented in the runtime (sgo_taskres.go).
func runtime_taskResNew(elem unsafe.Pointer) unsafe.Pointer
func runtime_taskResSet(r, v unsafe.Pointer) bool
func runtime_resPromote(v any, builtin *[5]any, nurs unsafe.Pointer) error
func runtime_spawnResult(scope unsafe.Pointer, fn func(task unsafe.Pointer) error) (notAdmitted, err error)
func runtime_taskResChan(r unsafe.Pointer) unsafe.Pointer
func runtime_nativeRecv(c, dst unsafe.Pointer)
func runtime_taskEndErr(task unsafe.Pointer) error

// Result is the outcome of a task started by Go: its value and its error. The error is data: a
// child that returns an error through Go does not fail its nursery.
type Result[T any] struct {
	Value T
	Err   error
}

// Task is the handle of a child started by Go. Its result can be read any number of times, by any
// task of the nursery, with Wait or as a select arm on Result().
type Task[T any] struct{ r unsafe.Pointer }

// builtinHandles are the built-in resource types, each a pointer to a struct whose first word is
// its runtime handle: runtime_resPromote recognizes them by type (SGO17-02).
var builtinHandles = [5]any{(*File)(nil), (*Conn)(nil), (*Dialer)(nil), (*Listener)(nil), (*Timer)(nil)}

// Go starts f as a child of n, like n.Spawn, and returns a handle to its result (D161). If f
// returns a resource with a nil error, the resource is moved into n (D158): it is released when n
// ends unless a task of n moves it on. So is each resource of the child's body that a struct or
// array result holds, at any depth of fields and elements, once and in acquisition order; a
// resource already in n or in an enclosing scope stays where it is (SGO17-02). If the child ends without returning (it is cancelled or
// fails), its result's error is the cancellation or failure.
//
// Unlike n.Spawn, an error f returns is data: it is the result's Err and fails neither n nor a
// sibling, and f's body ends normally (its own resources see Release(true); own a resource in an
// inner nursery to have a failing check roll it back, D159). A failure of the child still fails n.
// A result whose Err is a cancellation describes the child; a reader that is itself cancelled
// while it waits unwinds instead (SGO-NATIVE).
func Go[T any](n *Nursery, f func(*Context) (T, error)) *Task[T] {
	if n == nil {
		runtime_violation("Go with a nil nursery handle")
	}
	if f == nil {
		runtime_violation("Go with a nil callback")
	}
	h := &Task[T]{r: runtime_taskResNew(resultType[T]())}
	start(n, f, func(r *Result[T]) { runtime_taskResSet(h.r, unsafe.Pointer(r)) })
	return h
}

// resultType returns the runtime type of Result[T].
func resultType[T any]() unsafe.Pointer {
	var zero Result[T]
	var e any = zero
	return (*[2]unsafe.Pointer)(unsafe.Pointer(&e))[0]
}

// start starts f as a child of n and publishes its Result once: the value and error it returns,
// with a resource moved into n (D158), or the cancellation or failure it ends with. When n or an
// ancestor is cancelling, no child is admitted and f never runs: the result is the scope's
// cancellation, published at once (SGO17-01), so no handle stays unresolved and a group's slot is
// released by the publication.
func start[T any](n *Nursery, f func(*Context) (T, error), publish func(*Result[T])) {
	notAdmitted, err := runtime_spawnResult(n.s, func(t unsafe.Pointer) error {
		set := false
		defer func() {
			if !set { // the child unwinds: publish why, so no reader waits forever
				res := Result[T]{Err: runtime_taskEndErr(t)}
				publish(&res)
			}
		}()
		v, err := f(&Context{t: t})
		if err == nil {
			err = runtime_resPromote(v, &builtinHandles, n.s)
		}
		res := Result[T]{Value: v, Err: err}
		publish(&res)
		set = true
		return nil
	})
	if err != nil {
		panic(err)
	}
	if notAdmitted != nil {
		res := Result[T]{Err: notAdmitted}
		publish(&res)
	}
}

// Wait waits for the task's result and returns it: the child's value and error. It is a channel
// receive (SGO-NATIVE): if a scope linked to c is cancelled first, the waiting task unwinds.
func (h *Task[T]) Wait(c *Context) (T, error) {
	c.own()
	var r Result[T]
	runtime_nativeRecv(runtime_taskResChan(h.r), unsafe.Pointer(&r))
	return r.Value, r.Err
}

// Result returns a new channel that delivers the task's result once, when it ends, for a select
// arm: case r := <-h.Result():. Each call returns its own channel, and each delivers the result
// once (edge-triggered): a loop that keeps one channel takes the arm once, then waits on its other
// arms. A call registers nothing; a channel that is never received from is garbage (W22 review
// G2, H1). A select loop that should handle the completion once keeps one channel:
//
//	done := h.Result()
//	for { select { case r := <-done: …; case … } }
func (h *Task[T]) Result() <-chan Result[T] {
	p := runtime_taskResChan(h.r)
	return *(*<-chan Result[T])(unsafe.Pointer(&p))
}
