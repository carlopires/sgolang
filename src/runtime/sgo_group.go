// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Groups of value-returning children (D172, SGO-SPAWN-007 draft; sgo.NewGroup, sgo.Group[T]).
//
// A group has one result channel, a native channel of capacity 1 that every g.Next() returns.
// Results are published in completion order and never block the child: one beyond the buffer goes
// to the channel's overflow list (sgoChan.grp), under the channel's lock, and each receive that
// takes the buffered value moves the oldest queued one into the buffer (sgoNativeTryRecv). Each
// result is received once, by whichever receiver takes it.
//
// A limit is a native channel of capacity limit used as a counting semaphore: g.Go sends on it
// before starting the child (a channel wait, so a cancellation point), and the child's end takes
// one value back after publishing its result, waking a waiting g.Go. Nothing is allocated under a
// runtime lock (W12 R9): boxes and list nodes are made first.

package runtime

import "unsafe"

type sgoGroup struct {
	res *sgoChan // results
	sem *sgoChan // running children, when the group has a limit
}

type sgoGroupQ struct {
	first, last *sgoGroupNode
}

type sgoGroupNode struct {
	box  unsafe.Pointer
	next *sgoGroupNode
}

// pop removes and returns the oldest queued result, under the result channel's lock.
func (q *sgoGroupQ) pop() unsafe.Pointer {
	n := q.first
	q.first = n.next
	if q.first == nil {
		q.last = nil
	}
	return n.box
}

func (q *sgoGroupQ) push(n *sgoGroupNode) {
	if q.last != nil {
		q.last.next = n
	} else {
		q.first = n
	}
	q.last = n
}

// sgo_groupNew makes a group whose results have type elem (a Result[T]), with at most limit
// children running (0: no limit).
//
//go:linkname sgo_groupNew sgo.runtime_groupNew
func sgo_groupNew(elem unsafe.Pointer, limit int) unsafe.Pointer {
	g := &sgoGroup{res: &sgoChan{cap: 1, buf: sgoChanBuf(1), elem: (*_type)(elem), native: true, grp: &sgoGroupQ{}}}
	if limit > 0 {
		var zero any = 0
		g.sem = &sgoChan{cap: int64(limit), buf: sgoChanBuf(int64(limit)), elem: efaceOf(&zero)._type, native: true}
	}
	return unsafe.Pointer(g)
}

// sgo_groupAcquire waits, as a channel send, until the group has fewer than limit children
// running, and counts one more.
//
//go:linkname sgo_groupAcquire sgo.runtime_groupAcquire
func sgo_groupAcquire(gp unsafe.Pointer) {
	g := (*sgoGroup)(gp)
	if g.sem == nil {
		return
	}
	v := 0
	sgochansend1(g.sem, unsafe.Pointer(&v))
}

// sgo_groupPublish delivers the result *v of a child of the group and then counts the child as
// ended. It never waits.
//
//go:linkname sgo_groupPublish sgo.runtime_groupPublish
func sgo_groupPublish(gp, v unsafe.Pointer) {
	g := (*sgoGroup)(gp)
	c := g.res
	box := sgoNativeBox(c, v)
	n := &sgoGroupNode{box: box}
	var wake *sgoChanWait
	lock(&c.lock)
	if c.grp.first == nil { // nothing queued: to a waiting receiver or the buffer, if free
		if w, st := sgoNativeTrySend(c, box); st == sgoDone {
			wake, n = w, nil
		}
	}
	if n != nil {
		c.grp.push(n)
	}
	unlock(&c.lock)
	sgoReadyOne(wake)
	if s := g.sem; s != nil {
		lock(&s.lock)
		_, _, w, _ := sgoNativeTryRecv(s)
		unlock(&s.lock)
		sgoReadyOne(w)
	}
}

// sgo_groupChan returns the group's result channel.
//
//go:linkname sgo_groupChan sgo.runtime_groupChan
func sgo_groupChan(gp unsafe.Pointer) unsafe.Pointer {
	return unsafe.Pointer((*sgoGroup)(gp).res)
}
