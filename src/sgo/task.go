// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgo

import "unsafe"

// Value-returning tasks (D161, spelling D168; SGO-SPAWN-006 draft).

// Implemented in the runtime (sgo_taskres.go).
func runtime_taskResNew(elem unsafe.Pointer) unsafe.Pointer
func runtime_taskResSet(r, v unsafe.Pointer)
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

type sgoMover interface{ MoveTo(Scope) error }

// Go starts f as a child of n, like n.Spawn, and returns a handle to its result (D161). If f
// returns a resource with a nil error, the resource is moved into n (D158): it is released when n
// ends unless a task of n moves it on. If the child ends without returning (it is cancelled or
// fails), its result's error is the cancellation or failure.
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
// with a resource moved into n (D158), or the cancellation or failure it ends with.
func start[T any](n *Nursery, f func(*Context) (T, error), publish func(*Result[T])) {
	if err := runtime_spawn(n.s, func(t unsafe.Pointer) error {
		set := false
		defer func() {
			if !set { // the child unwinds: publish why, so no reader waits forever
				res := Result[T]{Err: runtime_taskEndErr(t)}
				publish(&res)
			}
		}()
		v, err := f(&Context{t: t})
		if err == nil {
			if m, ok := any(v).(sgoMover); ok {
				err = m.MoveTo(n)
			} else if u, ok := any(v).(Releaser); ok {
				err = MoveTo(u, n)
			}
		}
		res := Result[T]{Value: v, Err: err}
		publish(&res)
		set = true
		return nil
	}); err != nil {
		panic(err)
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

// Result returns the task's result channel, for a select arm: case r := <-h.Result():. Every call
// returns the same channel. Once the task ends its result is latched: every receive, by any
// reader, any number of times, gets it, so an arm on it proceeds on every iteration after the end
// (level-triggered); nothing is consumed and nothing is registered per call (W22 review G2).
func (h *Task[T]) Result() <-chan Result[T] {
	p := runtime_taskResChan(h.r)
	return *(*<-chan Result[T])(unsafe.Pointer(&p))
}
