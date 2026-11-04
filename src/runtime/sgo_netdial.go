// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

// Delegated dialing (W18, D143; W21, D148): what Happy Eyeballs (RFC 8305) needs beyond nurseries.
//
// A dialer is a resource created by the root task (in its body or one of its nurseries) from
// endpoints its connect grants name. Every task inside the dialer's scope dials any of its
// endpoints, and the connection is a resource of the scope the call names.

import "unsafe"

type sgoDialer struct {
	allow    []*sgoNetGrant // the endpoints this dialer may connect to (IP, Unix, or host name)
	mu       mutex          // guards resolved: tasks the dialer is lent to resolve concurrently
	resolved []sgoNetGrant  // addresses Resolve returned for its host-name endpoints (D145)
}

//go:linkname sgo_dialerNew sgo.runtime_dialerNew
func sgo_dialerNew(task, nurs unsafe.Pointer, endpoints []string) unsafe.Pointer {
	n, b := sgoScopeOf(task, nurs)
	l := sgoResTarget(n, b, "Dialer")
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
	sgoResPlace(h, l, n, b)
	return unsafe.Pointer(h)
}

//go:linkname sgo_dialerDial sgo.runtime_dialerDial
func sgo_dialerDial(p, task, nurs unsafe.Pointer, addr string) (unsafe.Pointer, error) {
	h := (*sgoHandle)(p)
	n, b := sgoScopeOf(task, nurs)
	l := sgoResTarget(n, b, "Dialer.Dial")
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
	return sgoDialGrantInto(g, "Dialer.Dial", l, n, b)
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
			out = append(out, r.addr)
			if !sgoGrantIn(add, &r) {
				add = append(add, r)
			}
		}
	}
	sgoDialerPublish(d, add)
	return out, nil
}

// sgoDialerPublish adds the entries of add that d.resolved lacks. The grown table is built outside
// the lock (no allocation under a runtime lock, W12 R9); the lock only checks and publishes it.
func sgoDialerPublish(d *sgoDialer, add []sgoNetGrant) {
	for {
		lock(&d.mu)
		cur := d.resolved
		unlock(&d.mu)
		// Re-resolving keeps one entry per address and port (W18 review Y2).
		grown := make([]sgoNetGrant, len(cur), len(cur)+len(add))
		copy(grown, cur)
		for i := range add {
			if !sgoGrantIn(cur, &add[i]) {
				grown = append(grown, add[i])
			}
		}
		lock(&d.mu)
		if len(d.resolved) == len(cur) {
			d.resolved = grown
			unlock(&d.mu)
			return
		}
		unlock(&d.mu)
	}
}

// sgoGrantIn reports whether list holds an entry with g's address, family, and port.
func sgoGrantIn(list []sgoNetGrant, g *sgoNetGrant) bool {
	for i := range list {
		if list[i].ip == g.ip && list[i].v6 == g.v6 && list[i].port == g.port {
			return true
		}
	}
	return false
}

// A connection slot (D144, D157) hands one connection, by value, to the tasks of an enclosing
// scope: the slot is a resource of the scope that creates it, the first Put moves its connection
// into that scope (the slot's scope must enclose the connection's), and Take returns it to a task
// inside the slot's scope. Ownership follows the scopes (sgo_res.go); the slot only carries the
// value, which no channel may (D151).
type sgoSlot struct {
	mu   mutex
	set  bool       // a Put won (the slot accepts one connection, ever)
	held *sgoHandle // the connection put, published once it is in the slot's scope
}

//go:linkname sgo_slotNew sgo.runtime_slotNew
func sgo_slotNew(task, nurs unsafe.Pointer) unsafe.Pointer {
	n, b := sgoScopeOf(task, nurs)
	l := sgoResTarget(n, b, "NewConnSlot")
	h := sgoHandleNew(sgoHSlot, -1, "connection slot")
	h.slot = &sgoSlot{}
	sgoResPlace(h, l, n, b)
	return unsafe.Pointer(h)
}

//go:linkname sgo_slotPut sgo.runtime_slotPut
func sgo_slotPut(sp, cp unsafe.Pointer) error {
	sh, c := (*sgoHandle)(sp), (*sgoHandle)(cp)
	if e := sgoHandleBegin(sh, "ConnSlot.Put"); e != nil {
		return sgoIOResult(e, "ConnSlot.Put")
	}
	defer sgoHandleEnd(sh)
	if !sgoHandleUser(c, sgoCurrent()) {
		panic(error(&sgoFailure{"MissingCapability", "ConnSlot.Put: the task does not run inside the connection's scope", ""}))
	}
	if !sgoEncloses(sh.nurs, sh.body, c.nurs, c.body) {
		panic(error(&sgoFailure{"MissingCapability", "ConnSlot.Put: the slot's scope does not enclose the connection's scope", ""}))
	}
	if c.closed {
		return sgoIOResult(&sgoIOErr{"HandleClosed", 0}, "ConnSlot.Put")
	}
	if c.ops.Load() != 0 {
		return sgoIOResult(&sgoIOErr{"ResourceBusy", 0}, "ConnSlot.Put")
	}
	s := sh.slot
	lock(&s.mu)
	if s.set {
		unlock(&s.mu)
		return sgoIOResult(&sgoIOErr{"AlreadySet", 0}, "ConnSlot.Put") // c stays where it is
	}
	s.set = true
	unlock(&s.mu)
	// Only the winner gets here: move c into the slot's scope, then publish it.
	l := &sh.body.res
	if sh.nurs != nil {
		l = &sh.nurs.res
	}
	sgoResPlace(c, l, sh.nurs, sh.body)
	lock(&s.mu)
	s.held = c
	unlock(&s.mu)
	return nil
}

//go:linkname sgo_slotTake sgo.runtime_slotTake
func sgo_slotTake(sp unsafe.Pointer) (unsafe.Pointer, bool) {
	sh := (*sgoHandle)(sp)
	if !sgoHandleUser(sh, sgoCurrent()) {
		panic(error(&sgoFailure{"MissingCapability", "ConnSlot.Take: the task does not run inside the slot's scope", ""}))
	}
	s := sh.slot
	lock(&s.mu)
	c := s.held
	s.held = nil
	unlock(&s.mu)
	if c == nil {
		return nil, false
	}
	return unsafe.Pointer(c), true
}
