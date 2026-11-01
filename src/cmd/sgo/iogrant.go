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
//	grant connect IP:PORT
//	grant listen IP:PORT
//
// The driver keeps them in canonical form, sorted: "dir NAME ro|rw PATH", "connect IP:PORT",
// "listen IP:PORT" (netip's canonical address form). The linker records the text for the runtime
// (-sgoiogrants), and the build ID names its digest (/io=).

var grantName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

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
			return true, usage(fmt.Sprintf("%q: an endpoint grant is \"grant %s IP:PORT\"", text, fields[1]))
		}
		ap, err := netip.ParseAddrPort(fields[2])
		if err != nil || ap.Port() == 0 || ap.Addr().Zone() != "" || ap.Addr().Is4In6() {
			return true, usage(fmt.Sprintf("%q is not an IP:PORT endpoint (an IPv4 or [IPv6] literal, not IPv4-mapped, and a port 1-65535; no host names, D57)", fields[2]))
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
