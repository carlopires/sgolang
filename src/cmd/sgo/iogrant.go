// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strings"
)

// File and network grants of the main sgo.mod (W09, D52, D57):
//
//	grant dir NAME PATH ro|rw
//	grant connect IP:PORT | unix:PATH
//	grant listen IP:PORT | unix:PATH
//
// The driver keeps them in canonical form, sorted: "dir NAME ro|rw PATH", "connect IP:PORT",
// "listen IP:PORT" (netip's canonical address form). The linker records the text for the runtime
// (-sgoiogrants), and the build ID names its digest (/io=).

var grantName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// maxUnixPath is the portable limit of a Unix socket path: sun_path is 104 bytes on darwin,
// including the terminating NUL.
const maxUnixPath = 103

// parseIOGrant handles a grant line for a directory or an endpoint (fields[1] is dir, connect,
// or listen); it reports whether the line was one.
func (m *module) parseIOGrant(fields []string, text, at string) (bool, error) {
	usage := func(msg string) error { return &toolErr{at, "SGO_E_USAGE", msg + " (D52)", 2} }
	switch fields[1] {
	case "dir":
		if len(fields) != 5 || (fields[4] != "ro" && fields[4] != "rw") {
			return true, usage(fmt.Sprintf("%q: a directory grant is \"grant dir NAME PATH ro|rw\"", text))
		}
		name, path := fields[2], fields[3]
		if !grantName.MatchString(name) {
			return true, usage(fmt.Sprintf("directory grant name %q must be a lower-case letter followed by at most 31 of [a-z0-9_-]", name))
		}
		if strings.ContainsRune(path, 0) {
			return true, usage("a directory grant path contains NUL")
		}
		for _, g := range m.ioGrants {
			if f := strings.Fields(g); f[0] == "dir" && f[1] == name {
				return true, usage(fmt.Sprintf("directory grant %s is declared twice", name))
			}
		}
		m.ioGrants = append(m.ioGrants, "dir "+name+" "+fields[4]+" "+path)
	case "connect", "listen":
		if len(fields) != 3 {
			return true, usage(fmt.Sprintf("%q: an endpoint grant is \"grant %s IP:PORT\" or \"grant %s unix:PATH\"", text, fields[1], fields[1]))
		}
		if path, ok := strings.CutPrefix(fields[2], "unix:"); ok {
			// A Unix domain socket (D60): an exact path, absolute or relative to the working
			// directory at start, within the portable sun_path limit.
			if path == "" || len(path) > maxUnixPath || strings.ContainsRune(path, 0) {
				return true, usage(fmt.Sprintf("%q is not a Unix socket endpoint (unix:PATH with a path of 1 to %d bytes, D60)", fields[2], maxUnixPath))
			}
			line := fields[1] + " unix:" + path
			if slices.Contains(m.ioGrants, line) {
				return true, usage(fmt.Sprintf("%s is granted twice", line))
			}
			m.ioGrants = append(m.ioGrants, line)
			break
		}
		if host, port, ok := hostPort(fields[2]); ok {
			// A host name (D145): connect only; resolved at run time through a Dialer.
			if fields[1] != "connect" {
				return true, usage(fmt.Sprintf("%q: a listen grant names an IP:PORT or unix:PATH, not a host name (D145)", fields[2]))
			}
			line := "connect " + host + ":" + port
			if slices.Contains(m.ioGrants, line) {
				return true, usage(fmt.Sprintf("%s is granted twice", line))
			}
			m.ioGrants = append(m.ioGrants, line)
			break
		}
		ap, err := netip.ParseAddrPort(fields[2])
		if err != nil || ap.Port() == 0 || ap.Addr().Zone() != "" || ap.Addr().Is4In6() {
			return true, usage(fmt.Sprintf("%q is not an IP:PORT endpoint (an IPv4 or [IPv6] literal, not IPv4-mapped, and a port 1-65535; or for connect, a lower-case HOST:PORT, D57, D145)", fields[2]))
		}
		line := fields[1] + " " + ap.String()
		if slices.Contains(m.ioGrants, line) {
			return true, usage(fmt.Sprintf("%s is granted twice", line))
		}
		m.ioGrants = append(m.ioGrants, line)
	default:
		return false, nil
	}
	slices.Sort(m.ioGrants)
	return true, nil
}

// ioDigest is the build ID component of the I/O grants: the first 16 hex digits of the SHA-256
// of their canonical text, or "" when there are none.
func ioDigest(grants []string) string {
	if len(grants) == 0 {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.Join(grants, "\n")))
	return hex.EncodeToString(sum[:8])
}

// hostPort splits a NAME:PORT endpoint whose NAME is a canonical host name (D145): dot-separated
// labels of [a-z0-9-] of 1 to 63 bytes not starting or ending with '-', 253 bytes in all, with at
// least one letter or '-' (so not an IP literal), and a port 1-65535 without leading zeros. The
// runtime applies the same rule (sgoParseHostPort).
func hostPort(s string) (host, port string, ok bool) {
	i := strings.LastIndexByte(s, ':')
	if i <= 0 {
		return "", "", false
	}
	host, port = s[:i], s[i+1:]
	if len(host) > 253 || port == "" || port[0] == '0' || len(port) > 5 {
		return "", "", false
	}
	n := 0
	for _, c := range port {
		if c < '0' || c > '9' {
			return "", "", false
		}
		n = n*10 + int(c-'0')
	}
	if n > 65535 {
		return "", "", false
	}
	letter := false
	for _, l := range strings.Split(host, ".") {
		if l == "" || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
			return "", "", false
		}
		for _, c := range l {
			switch {
			case c >= 'a' && c <= 'z', c == '-':
				letter = true
			case c >= '0' && c <= '9':
			default:
				return "", "", false
			}
		}
	}
	return host, port, letter
}
