// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

// Host name resolution for dialers (W18, D145): what Dialer.Resolve does outside the lab.
//
// A name is looked up in /etc/hosts first; a name the file lists is answered from it alone (as the
// C library does). Otherwise the resolver asks the name servers of /etc/resolv.conf (127.0.0.1
// when it names none) over UDP: one query for one record type (A or AAAA), two rounds over the
// servers with a one second wait each. Waits are cancellation points, so an attempt to resolve
// ends with its scope. Truncated answers are used as they are (no TCP retry), CNAME chains are
// followed only as far as the answer section carries them, and search domains are not applied:
// granted names are absolute.

import "unsafe"

// sgoResolveHost returns the addresses of host for one family (v6: AAAA, else A), in answer order.
func sgoResolveHost(host string, v6 bool) ([]sgoNetGrant, *sgoIOErr) {
	if sgoLab != nil {
		return sgoFakeResolve(host, v6)
	}
	if found, ips := sgoHostsLookup(host, v6); found {
		return sgoKeepResolvable(ips), nil
	}
	if err := sgoTaskCancellation(sgoCurrent()); err != nil {
		return nil, &sgoIOErr{"Cancelled", 0}
	}
	servers := sgoNameServers()
	qtype := uint16(1)
	if v6 {
		qtype = 28
	}
	last := &sgoIOErr{"IOError", 0}
	for round := 0; round < 2; round++ {
		for i := range servers {
			ips, e := sgoDNSQuery(&servers[i], host, qtype)
			if e == nil {
				return sgoKeepResolvable(ips), nil
			}
			if e.kind == "Cancelled" {
				return nil, e
			}
			last = e
		}
	}
	last.kind = "IOError"
	return nil, last
}

// sgoReadSmallFile returns up to 64 KiB of the file at path, or "" if it cannot be read.
func sgoReadSmallFile(path string) string {
	fd := sgoOpenat(sgoAT_FDCWD, sgoCString(path), sgoO_RDONLY|sgoO_CLOEXEC, 0)
	if fd < 0 {
		return ""
	}
	defer sgoCloseFD(fd)
	buf := make([]byte, 64<<10)
	n := 0
	for n < len(buf) {
		r := sgoSysRead(fd, unsafe.Pointer(&buf[n]), int32(len(buf)-n))
		if r == -sgoEINTR {
			continue
		}
		if r <= 0 {
			break
		}
		n += int(r)
	}
	return string(buf[:n])
}

// sgoLines calls f with each line of s, its comment ('#' on) removed, split into fields.
func sgoLines(s string, f func(fields []string)) {
	for s != "" {
		line := s
		s = ""
		for i := 0; i < len(line); i++ {
			if line[i] == '\n' {
				line, s = line[:i], line[i+1:]
				break
			}
		}
		if i := sgoIndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		var fields []string
		start := -1
		for i := 0; i <= len(line); i++ {
			if i == len(line) || line[i] == ' ' || line[i] == '\t' || line[i] == '\r' {
				if start >= 0 {
					fields = append(fields, line[start:i])
					start = -1
				}
			} else if start < 0 {
				start = i
			}
		}
		if len(fields) > 0 {
			f(fields)
		}
	}
}

// sgoParseIPLit parses an IPv4 or IPv6 literal (no zone) into an address with port 0.
func sgoParseIPLit(s string) (sgoNetGrant, bool) {
	var g sgoNetGrant
	if sgoIndexByte(s, ':') >= 0 {
		return g, sgoParseAddr("["+s+"]:1", &g)
	}
	return g, sgoParseAddr(s+":1", &g)
}

// sgoHostsLookup reports whether /etc/hosts lists host and, if so, its addresses of the family.
func sgoHostsLookup(host string, v6 bool) (bool, []sgoNetGrant) {
	found := false
	var out []sgoNetGrant
	sgoLines(sgoReadSmallFile("/etc/hosts"), func(f []string) {
		for _, name := range f[1:] {
			if sgoLowerASCII(name) != host {
				continue
			}
			found = true
			if g, ok := sgoParseIPLit(f[0]); ok && g.v6 == v6 {
				out = append(out, g)
			}
		}
	})
	return found, out
}

// sgoLowerASCII returns s with ASCII upper-case letters lowered.
func sgoLowerASCII(s string) string {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if b[j] >= 'A' && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// sgoNameServers returns port-53 addresses of the name servers of /etc/resolv.conf (at most three).
func sgoNameServers() []sgoNetGrant {
	var out []sgoNetGrant
	sgoLines(sgoReadSmallFile("/etc/resolv.conf"), func(f []string) {
		if len(f) >= 2 && f[0] == "nameserver" && len(out) < 3 {
			if g, ok := sgoParseIPLit(f[1]); ok {
				g.port = 53
				out = append(out, g)
			}
		}
	})
	if len(out) == 0 {
		g, _ := sgoParseIPLit("127.0.0.1")
		g.port = 53
		out = append(out, g)
	}
	return out
}

// sgoDNSQuery asks server for the qtype records of host once, waiting up to one second.
func sgoDNSQuery(server *sgoNetGrant, host string, qtype uint16) ([]sgoNetGrant, *sgoIOErr) {
	id := uint16(cheaprand())
	q := []byte{byte(id >> 8), byte(id), 0x01, 0x00, 0, 1, 0, 0, 0, 0, 0, 0}
	for start, k := 0, 0; k <= len(host); k++ {
		if k == len(host) || host[k] == '.' {
			q = append(q, byte(k-start))
			q = append(q, host[start:k]...)
			start = k + 1
		}
	}
	q = append(q, 0, byte(qtype>>8), byte(qtype), 0, 1)

	fd := sgoSocketType(server, sgoSOCK_DGRAM)
	if fd < 0 {
		return nil, &sgoIOErr{sgoNetErrKind(fd), 0}
	}
	h, e := sgoNetHandle(sgoHConn, fd, "resolver")
	if e != nil {
		return nil, e
	}
	defer sgoHandleFree(h)
	sa := new(sgoSockaddrBuf)
	if r := sgoConnectSys(fd, sa, sgoSockaddr(sa, server)); r < 0 {
		return nil, &sgoIOErr{sgoNetErrKind(r), 0}
	}
	if r := sgoSysWrite(fd, unsafe.Pointer(&q[0]), int32(len(q))); r != int32(len(q)) {
		return nil, &sgoIOErr{"IOError", 0}
	}
	buf := make([]byte, 1500)
	deadline := nanotime() + 1e9
	for {
		r := sgoSysRead(fd, unsafe.Pointer(&buf[0]), int32(len(buf)))
		switch {
		case r == -sgoEINTR:
			continue
		case r == -sgoEAGAIN:
			if nanotime() >= deadline {
				return nil, &sgoIOErr{"Timeout", 0}
			}
			poll_runtime_pollSetDeadline(h.pd, deadline, 'r')
			e := sgoNetAwait(h, 'r')
			poll_runtime_pollSetDeadline(h.pd, 0, 'r')
			if e != nil {
				return nil, e
			}
			continue
		case r < 0:
			return nil, &sgoIOErr{sgoNetErrKind(r), 0}
		}
		if ips, ok := sgoDNSAnswer(buf[:r], id, qtype, host); ok {
			return ips, nil
		}
		// Not an answer to this query (or malformed): keep waiting for one until the deadline.
	}
}

// sgoDNSAnswer extracts the qtype addresses of a response to query id for host; ok is false if
// msg is not such a response. The response must echo the one question (host, qtype, IN), and an
// answer record counts only if its owner is host or the target of a CNAME record of this answer
// whose owner counts (W18 review Y4). A name that does not exist yields no addresses.
func sgoDNSAnswer(msg []byte, id, qtype uint16, host string) (ips []sgoNetGrant, ok bool) {
	u16 := func(i int) int { return int(msg[i])<<8 | int(msg[i+1]) }
	if len(msg) < 12 || u16(0) != int(id) || msg[2]&0x80 == 0 || u16(4) != 1 {
		return nil, false
	}
	rcode := msg[3] & 0x0f
	if rcode != 0 && rcode != 3 {
		return nil, false
	}
	// The question: an uncompressed name, then type and class.
	qname, i, qok := sgoDNSName(msg, 12, false)
	if !qok || i+4 > len(msg) || qname != host || u16(i) != int(qtype) || u16(i+2) != 1 {
		return nil, false
	}
	i += 4
	if rcode == 3 {
		return nil, true
	}
	type rr struct {
		owner      string
		typ, class int
		at, n      int
	}
	var rrs []rr
	for k := u16(6); k > 0; k-- {
		owner, next, nok := sgoDNSName(msg, i, true)
		if !nok || next+10 > len(msg) {
			return nil, false
		}
		r := rr{owner: owner, typ: u16(next), class: u16(next + 2), at: next + 10, n: u16(next + 8)}
		if r.at+r.n > len(msg) {
			return nil, false
		}
		rrs = append(rrs, r)
		i = r.at + r.n
	}
	names := []string{host}
	known := func(n string) bool {
		for _, m := range names {
			if m == n {
				return true
			}
		}
		return false
	}
	for grew := true; grew && len(names) <= 16; {
		grew = false
		for _, r := range rrs {
			if r.typ != 5 || r.class != 1 || !known(r.owner) {
				continue
			}
			if target, end, tok := sgoDNSName(msg, r.at, true); tok && end <= r.at+r.n && !known(target) {
				names = append(names, target)
				grew = true
			}
		}
	}
	for _, r := range rrs {
		if r.class != 1 || r.typ != int(qtype) || !known(r.owner) {
			continue
		}
		var g sgoNetGrant
		switch {
		case r.typ == 1 && r.n == 4:
			g.ip[10], g.ip[11] = 0xff, 0xff
			copy(g.ip[12:], msg[r.at:r.at+4])
			ips = append(ips, g)
		case r.typ == 28 && r.n == 16:
			g.v6 = true
			copy(g.ip[:], msg[r.at:r.at+16])
			ips = append(ips, g)
		}
	}
	return ips, true
}

// sgoDNSName decodes the domain name at msg[i:] in lower case, without the trailing dot; next is
// the offset after it in place. Compression pointers are followed (at most 32) only if compressed.
func sgoDNSName(msg []byte, i int, compressed bool) (name string, next int, ok bool) {
	var b []byte
	next = -1
	for hops := 0; i < len(msg); {
		n := int(msg[i])
		switch {
		case n == 0:
			if next < 0 {
				next = i + 1
			}
			return string(b), next, len(b) <= 253
		case n&0xc0 == 0xc0:
			if !compressed || i+1 >= len(msg) || hops == 32 {
				return "", 0, false
			}
			if next < 0 {
				next = i + 2
			}
			hops++
			i = (n&0x3f)<<8 | int(msg[i+1])
			continue
		case n > 63 || i+1+n > len(msg):
			return "", 0, false
		}
		if len(b) > 0 {
			b = append(b, '.')
		}
		for _, c := range msg[i+1 : i+1+n] {
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			b = append(b, c)
		}
		i += 1 + n
	}
	return "", 0, false
}

// sgoResolvable reports whether a resolved address may be a result (W18 review Y3): not the
// unspecified address of either family (0.0.0.0 and :: reach the local host on Linux) and not an
// IPv4-mapped IPv6 address (D57 refuses those forms in grants).
func sgoResolvable(g *sgoNetGrant) bool {
	zero := func(b []byte) bool {
		for _, c := range b {
			if c != 0 {
				return false
			}
		}
		return true
	}
	if !g.v6 {
		return !zero(g.ip[12:])
	}
	return !zero(g.ip[:]) && !(zero(g.ip[:10]) && g.ip[10] == 0xff && g.ip[11] == 0xff)
}

// sgoKeepResolvable drops the addresses that may not be results.
func sgoKeepResolvable(ips []sgoNetGrant) []sgoNetGrant {
	out := ips[:0]
	for i := range ips {
		if sgoResolvable(&ips[i]) {
			out = append(out, ips[i])
		}
	}
	return out
}

// sgoFormatAddr is the canonical text of an IP address and port, as grants write it: a.b.c.d:port
// or [h:...]:port with the longest run of two or more zero groups shortened to "::" (RFC 5952).
func sgoFormatAddr(g *sgoNetGrant, port uint16) string {
	var b []byte
	dec := func(v int) {
		var d [5]byte
		k := len(d)
		for {
			k--
			d[k] = byte('0' + v%10)
			if v /= 10; v == 0 {
				break
			}
		}
		b = append(b, d[k:]...)
	}
	if !g.v6 {
		for k := 12; k < 16; k++ {
			if k > 12 {
				b = append(b, '.')
			}
			dec(int(g.ip[k]))
		}
	} else {
		const hexd = "0123456789abcdef"
		best, bestLen := -1, 1
		for k := 0; k < 8; {
			if g.ip[2*k] != 0 || g.ip[2*k+1] != 0 {
				k++
				continue
			}
			j := k
			for j < 8 && g.ip[2*j] == 0 && g.ip[2*j+1] == 0 {
				j++
			}
			if j-k > bestLen {
				best, bestLen = k, j-k
			}
			k = j
		}
		b = append(b, '[')
		for k := 0; k < 8; k++ {
			if k == best {
				b = append(b, ':', ':')
				k += bestLen - 1
				continue
			}
			if k > 0 && b[len(b)-1] != ':' {
				b = append(b, ':')
			}
			v := int(g.ip[2*k])<<8 | int(g.ip[2*k+1])
			started := false
			for s := 12; s >= 0; s -= 4 {
				if d := (v >> s) & 0xf; d != 0 || started || s == 0 {
					b = append(b, hexd[d])
					started = true
				}
			}
		}
		b = append(b, ']')
	}
	b = append(b, ':')
	dec(int(port))
	return string(b)
}
