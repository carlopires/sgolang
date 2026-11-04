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
