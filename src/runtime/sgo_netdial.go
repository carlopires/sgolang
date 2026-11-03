// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

// Delegated dialing and the connection slot (W18, D143, D144): what Happy Eyeballs (RFC 8305)
// needs beyond nurseries.
//
// A dialer is a capability handle created by the root task from endpoints its connect grants
// name. Like any handle it is owned by its creator and may be lent to the creator's nurseries; a
// task that may use it dials any of its endpoints, and the connection belongs to that task.
//
// A connection slot is a handle that receives one connection from another task. Put, by the
// connection's owner (a task the slot is lent to), moves the connection out of that task's handle
// list into the slot; only the first Put succeeds (AlreadySet after it), which makes the slot the
// arbiter of a race. Take, by the slot's owner, links the connection into the owner's list. A
// connection still in the slot is closed when the slot is. Handle lists stay touched only by
// their own task: the putter unlinks, the taker links (the model: sgo/model/handoff.go).

import "unsafe"

type sgoDialer struct {
	allow []*sgoNetGrant // the endpoints this dialer may connect to
}

type sgoSlot struct {
	mu   mutex
	set  bool       // a Put succeeded (the slot accepts one connection, ever)
	held *sgoHandle // the connection put and not yet taken
}

//go:linkname sgo_dialerNew sgo.runtime_dialerNew
func sgo_dialerNew(endpoints []string) unsafe.Pointer {
	d := &sgoDialer{}
	for _, a := range endpoints {
		g := sgoNetGrantFor(a, false)
		if g == nil {
			panic(error(&sgoFailure{"MissingCapability", "Dialer " + quoteKind(a) + ": no connect grant for this endpoint", ""}))
		}
		d.allow = append(d.allow, g)
	}
	h := sgoHandleNew(sgoHDialer, -1, "dialer")
	h.dial = d
	return unsafe.Pointer(h)
}

//go:linkname sgo_dialerDial sgo.runtime_dialerDial
func sgo_dialerDial(p unsafe.Pointer, addr string) (unsafe.Pointer, error) {
	h := (*sgoHandle)(p)
	if e := sgoHandleBegin(h, "Dialer.Dial"); e != nil {
		return nil, sgoIOResult(e, "Dialer.Dial")
	}
	defer sgoHandleEnd(h)
	g := sgoNetGrantFor(addr, false)
	ok := false
	for _, x := range h.dial.allow {
		ok = ok || x == g
	}
	if g == nil || !ok {
		panic(error(&sgoFailure{"MissingCapability", "Dialer.Dial " + quoteKind(addr) + ": not an endpoint of this dialer", ""}))
	}
	return sgoDialGrant(g, "Dialer.Dial")
}

//go:linkname sgo_slotNew sgo.runtime_slotNew
func sgo_slotNew() unsafe.Pointer {
	h := sgoHandleNew(sgoHSlot, -1, "connection slot")
	h.slot = &sgoSlot{}
	return unsafe.Pointer(h)
}

//go:linkname sgo_slotPut sgo.runtime_slotPut
func sgo_slotPut(sp, cp unsafe.Pointer) error {
	sh, c := (*sgoHandle)(sp), (*sgoHandle)(cp)
	if e := sgoHandleBegin(sh, "ConnSlot.Put"); e != nil {
		return sgoIOResult(e, "ConnSlot.Put")
	}
	defer sgoHandleEnd(sh)
	t := sgoCurrent()
	if c.owner != t {
		panic(error(&sgoFailure{"MissingCapability", "ConnSlot.Put: only the connection's owner may hand it over", ""}))
	}
	if c.closed {
		return sgoIOResult(&sgoIOErr{"HandleClosed", 0}, "ConnSlot.Put")
	}
	if sgoHandleLentOpen(c) || c.ops.Load() != 0 {
		return sgoIOResult(&sgoIOErr{"HandleLent", 0}, "ConnSlot.Put")
	}
	// One critical section: the winner check, the unlink from the putter's own list (only this
	// task touches it; nothing allocates), and the publication, so Put is atomic as in the model.
	s := sh.slot
	lock(&s.mu)
	if s.set {
		unlock(&s.mu)
		return sgoIOResult(&sgoIOErr{"AlreadySet", 0}, "ConnSlot.Put")
	}
	s.set = true
	if c.prev != nil {
		c.prev.next = c.next
	} else if t.handles == c {
		t.handles = c.next
	}
	if c.next != nil {
		c.next.prev = c.prev
	}
	c.prev, c.next = nil, nil
	c.lentTo = nil
	c.owner = sh.owner
	c.scope = nil
	s.held = c
	unlock(&s.mu)
	return nil
}

//go:linkname sgo_slotTake sgo.runtime_slotTake
func sgo_slotTake(sp unsafe.Pointer) (unsafe.Pointer, bool) {
	sh := (*sgoHandle)(sp)
	t := sgoCurrent()
	if t != sh.owner {
		panic(error(&sgoFailure{"MissingCapability", "ConnSlot.Take: only the slot's owner may take its connection", ""}))
	}
	if sh.closed {
		return nil, false
	}
	s := sh.slot
	lock(&s.mu)
	c := s.held
	s.held = nil
	unlock(&s.mu)
	if c == nil {
		return nil, false
	}
	// Link into the taker's own list, in its current scope (closed at that body's exit if left).
	c.owner = t
	c.scope = t.currentScope()
	c.next = t.handles
	if t.handles != nil {
		t.handles.prev = c
	}
	t.handles = c
	return unsafe.Pointer(c), true
}

// sgoSlotFree closes the connection still held by slot h (the slot is being closed).
func sgoSlotFree(h *sgoHandle) {
	s := h.slot
	lock(&s.mu)
	c := s.held
	s.held = nil
	unlock(&s.mu)
	if c != nil && !c.closed {
		sgoHandleFree(c) // unlinked: owned by the slot's owner, in no list
	}
}
