// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgo

import "unsafe"

// Groups of value-returning children (D172; SGO-SPAWN-007 draft).

// Implemented in the runtime (sgo_group.go).
func runtime_groupNew(elem unsafe.Pointer, limit int) unsafe.Pointer
func runtime_groupAcquire(g unsafe.Pointer)
func runtime_groupPublish(g, v unsafe.Pointer)
func runtime_groupChan(g unsafe.Pointer) unsafe.Pointer

// Group is a set of value-returning children of one nursery whose results are received in
// completion order (D172). Its children are children of the nursery: they are joined, and
// cancelled, with it.
type Group[T any] struct {
	n *Nursery
	g unsafe.Pointer
}

// NewGroup returns a group of children of n with at most limit of them running at once (0: no
// limit).
func NewGroup[T any](n *Nursery, limit int) *Group[T] {
	if n == nil {
		runtime_violation("NewGroup with a nil nursery handle")
	}
	if limit < 0 {
		runtime_fail("InvalidArgument", "NewGroup with a negative limit")
	}
	return &Group[T]{n: n, g: runtime_groupNew(resultType[T](), limit)}
}

// Go starts f as a child of the group's nursery, as sgo.Go does, first waiting while limit
// children of the group are running. The wait is a cancellation point of the calling task t. The
// child's result is queued for Next when it ends; publishing never waits, so a child is never held
// up by an owner that has stopped reading. The limit bounds running children only: results not
// yet received from Next queue without bound. The wait for a slot is a blocking point of the
// caller, which an owner that also serves other events must account for.
func (g *Group[T]) Go(t *Context, f func(*Context) (T, error)) {
	t.own()
	if f == nil {
		runtime_violation("Group.Go with a nil callback")
	}
	runtime_groupAcquire(g.g)
	start(g.n, f, func(r *Result[T]) { runtime_groupPublish(g.g, unsafe.Pointer(r)) })
}

// Next returns the group's result channel, for case r := <-g.Next(): or r := <-g.Next(). It
// receives the results of the group's children in the order they ended, each result once. Unlike
// Task.Result, every call returns the same channel: one queue that receivers consume.
func (g *Group[T]) Next() <-chan Result[T] {
	p := runtime_groupChan(g.g)
	return *(*<-chan Result[T])(unsafe.Pointer(&p))
}
