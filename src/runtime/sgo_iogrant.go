// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// File and network grants (W09, D52, D57).
//
// The main sgo.mod declares them; the driver passes their canonical text to the linker
// (-sgoiogrants), which records it in sgoIOGrants: one grant per line,
//
//	dir NAME ro|rw PATH
//	connect IP:PORT
//	listen IP:PORT
//
// with IP in canonical form (dotted IPv4, or [IPv6]). When the program starts, each granted
// directory is opened once, relative to the working directory at start unless PATH is absolute,
// and held: later renames of PATH do not move the grant. A directory that cannot be opened is
// not an error at start; every operation on it reports the failure.

package runtime

var sgoIOGrants string

type sgoDirGrant struct {
	name string
	ro   bool
	path string
	fd   int32
	err  string // the error kind of the open at start, or ""
}

type sgoNetGrant struct {
	listen bool
	addr   string // canonical IP:PORT
	ip     [16]byte
	v6     bool
	port   uint16
}

var (
	sgoDirGrants []sgoDirGrant
	sgoNetGrants []sgoNetGrant
)

// sgoIOGrantsInit parses sgoIOGrants and opens the granted directories. The linker accepts only
// well-formed text, so anything else is a defect of the build.
func sgoIOGrantsInit() {
	for s := sgoIOGrants; s != ""; {
		line := s
		s = ""
		for i := 0; i < len(line); i++ {
			if line[i] == '\n' {
				line, s = line[:i], line[i+1:]
				break
			}
		}
		f := sgoFields(line, 4)
		switch {
		case len(f) == 4 && f[0] == "dir" && (f[2] == "ro" || f[2] == "rw"):
			g := sgoDirGrant{name: f[1], ro: f[2] == "ro", path: f[3], fd: -1}
			if sgoLab == nil {
				fd := sgoOpenat(sgoAT_FDCWD, sgoCString(g.path), sgoO_RDONLY|sgoO_DIRECTORY|sgoO_CLOEXEC, 0)
				if fd < 0 {
					g.err = sgoErrKind(fd)
				} else {
					g.fd = fd
				}
			}
			sgoDirGrants = append(sgoDirGrants, g)
		case len(f) == 2 && (f[0] == "connect" || f[0] == "listen"):
			g := sgoNetGrant{listen: f[0] == "listen", addr: f[1]}
			if !sgoParseAddr(f[1], &g) {
				throw("sgo: corrupt network grant")
			}
			sgoNetGrants = append(sgoNetGrants, g)
		default:
			throw("sgo: corrupt I/O grant set")
		}
	}
}

// sgoFields splits s at single spaces into at most n fields (the last keeps the rest).
func sgoFields(s string, n int) []string {
	var f []string
	for len(f) < n-1 {
		i := 0
		for i < len(s) && s[i] != ' ' {
			i++
		}
		if i == len(s) {
			break
		}
		f, s = append(f, s[:i]), s[i+1:]
	}
	return append(f, s)
}

// sgoParseAddr parses a canonical IP:PORT: a.b.c.d:port or [h:h:...]:port (no zone).
func sgoParseAddr(a string, g *sgoNetGrant) bool {
	i := len(a) - 1
	for i >= 0 && a[i] != ':' {
		i--
	}
	if i <= 0 || i == len(a)-1 {
		return false
	}
	port := 0
	for _, c := range []byte(a[i+1:]) {
		if c < '0' || c > '9' || port > 65535 {
			return false
		}
		port = port*10 + int(c-'0')
	}
	if port < 1 || port > 65535 {
		return false
	}
	g.port = uint16(port)
	host := a[:i]
	if host[0] == '[' {
		if host[len(host)-1] != ']' {
			return false
		}
		g.v6 = true
		return sgoParseIP6(host[1:len(host)-1], &g.ip)
	}
	g.ip[10], g.ip[11] = 0xff, 0xff
	n, v, digits := 12, 0, 0
	for k := 0; k <= len(host); k++ {
		if k == len(host) || host[k] == '.' {
			if digits == 0 || v > 255 || n > 15 {
				return false
			}
			g.ip[n], n, v, digits = byte(v), n+1, 0, 0
			continue
		}
		c := host[k]
		if c < '0' || c > '9' {
			return false
		}
		v, digits = v*10+int(c-'0'), digits+1
	}
	return n == 16
}

// sgoParseIP6 parses an IPv6 literal with at most one "::".
func sgoParseIP6(s string, ip *[16]byte) bool {
	var groups [8]uint16
	n, gap := 0, -1
	for k := 0; k < len(s); {
		if s[k] == ':' {
			if k+1 < len(s) && s[k+1] == ':' {
				if gap >= 0 {
					return false // a second "::"
				}
				gap, k = n, k+2
				continue
			}
			if k == 0 || k == len(s)-1 {
				return false // a single ":" at either end
			}
			k++
			continue
		}
		v, d := 0, 0
		for k < len(s) && s[k] != ':' {
			c := s[k]
			switch {
			case c >= '0' && c <= '9':
				v = v<<4 | int(c-'0')
			case c >= 'a' && c <= 'f':
				v = v<<4 | int(c-'a'+10)
			default:
				return false
			}
			d, k = d+1, k+1
		}
		if d == 0 || d > 4 || n == 8 {
			return false
		}
		groups[n], n = uint16(v), n+1
	}
	if gap >= 0 {
		if n == 8 {
			return false
		}
		copy(groups[8-(n-gap):], groups[gap:n])
		for k := gap; k < 8-(n-gap); k++ {
			groups[k] = 0
		}
	} else if n != 8 {
		return false
	}
	for k, v := range groups {
		ip[2*k], ip[2*k+1] = byte(v>>8), byte(v)
	}
	return true
}

// sgoNetGrantFor returns the grant permitting addr for listening or connecting, or nil.
func sgoNetGrantFor(addr string, listen bool) *sgoNetGrant {
	var g sgoNetGrant
	if !sgoParseAddr(addr, &g) {
		return nil
	}
	for i := range sgoNetGrants {
		ng := &sgoNetGrants[i]
		if ng.listen == listen && ng.ip == g.ip && ng.v6 == g.v6 && ng.port == g.port {
			return ng
		}
	}
	return nil
}
