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
	allow    []*sgoNetGrant // the endpoints this dialer may connect to (IP, Unix, or host name)
	mu       mutex          // guards resolved: tasks the dialer is lent to resolve concurrently
	resolved []sgoNetGrant  // addresses Resolve returned for its host-name endpoints (D145)
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
		ok = ok || (x == g && g.host == "")
	}
	if !ok {
		// An address a Resolve of this dialer returned: dial it under the host name's grant.
		var a sgoNetGrant
		d := h.dial
		if sgoParseAddr(addr, &a) {
			lock(&d.mu)
			for i := range d.resolved {
				if r := &d.resolved[i]; r.ip == a.ip && r.v6 == a.v6 && r.port == a.port {
					g, ok = r, true
					break
				}
			}
			unlock(&d.mu)
		}
	}
	if !ok {
		panic(error(&sgoFailure{"MissingCapability", "Dialer.Dial " + quoteKind(addr) + ": not an endpoint of this dialer nor an address it resolved", ""}))
	}
	return sgoDialGrant(g, "Dialer.Dial")
}

// sgo_dialerResolve resolves host, a host name some endpoint of the dialer names, for family
// "ip4" (A records) or "ip6" (AAAA): it returns IP:PORT for each address and each such endpoint's
// port, and the dialer may dial those addresses from then on (D145).
//
//go:linkname sgo_dialerResolve sgo.runtime_dialerResolve
func sgo_dialerResolve(p unsafe.Pointer, host, family string) ([]string, error) {
	h := (*sgoHandle)(p)
	if family != "ip4" && family != "ip6" {
		panic(error(&sgoFailure{"InvalidArgument", "Dialer.Resolve: family " + quoteKind(family) + " is neither \"ip4\" nor \"ip6\"", ""}))
	}
	if e := sgoHandleBegin(h, "Dialer.Resolve"); e != nil {
		return nil, sgoIOResult(e, "Dialer.Resolve")
	}
	defer sgoHandleEnd(h)
	d := h.dial
	var ports []*sgoNetGrant
	for _, x := range d.allow {
		if x.host == host {
			ports = append(ports, x)
		}
	}
	if len(ports) == 0 {
		panic(error(&sgoFailure{"MissingCapability", "Dialer.Resolve " + quoteKind(host) + ": no endpoint of this dialer names this host", ""}))
	}
	ips, e := sgoResolveHost(host, family == "ip6")
	if e != nil {
		return nil, sgoIOResult(e, "Dialer.Resolve")
	}
	// Build the new entries outside the lock (no allocation under a runtime
	// lock, W12 R9); the lock only publishes the grown table.
	var out []string
	var add []sgoNetGrant
	for i := range ips {
		for _, pg := range ports {
			r := ips[i]
			r.port, r.host = pg.port, ""
			r.addr = sgoFormatAddr(&r, r.port)
			add = append(add, r)
			out = append(out, r.addr)
		}
	}
	for {
		lock(&d.mu)
		cur := d.resolved
		unlock(&d.mu)
		grown := make([]sgoNetGrant, len(cur), len(cur)+len(add))
		copy(grown, cur)
		grown = append(grown, add...)
		lock(&d.mu)
		if len(d.resolved) == len(cur) {
			d.resolved = grown
			unlock(&d.mu)
			return out, nil
		}
		unlock(&d.mu)
	}
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
