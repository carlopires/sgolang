// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

// Test hooks for the W18 resolver (review Y2–Y4).

// SGoDNSAnswer runs the response parser on msg for a query (id, qtype, host) and returns the
// canonical addresses (port 0) it accepts.
func SGoDNSAnswer(msg []byte, id, qtype uint16, host string) ([]string, bool) {
	ips, ok := sgoDNSAnswer(msg, id, qtype, host)
	var out []string
	for i := range ips {
		out = append(out, sgoFormatAddr(&ips[i], 0))
	}
	return out, ok
}

// SGoResolvable reports whether the IP literal may be a resolution result.
func SGoResolvable(lit string) bool {
	g, ok := sgoParseIPLit(lit)
	return ok && sgoResolvable(&g)
}

// SGoResolvedGrowth publishes the addresses lits with each of ports into a fresh dialer rounds
// times, as repeated Resolve calls do, and returns the size of the dialer's resolved table.
func SGoResolvedGrowth(lits []string, ports []uint16, rounds int) int {
	d := new(sgoDialer)
	for k := 0; k < rounds; k++ {
		var add []sgoNetGrant
		for _, l := range lits {
			for _, p := range ports {
				g, _ := sgoParseIPLit(l)
				g.port = p
				g.addr = sgoFormatAddr(&g, p)
				if !sgoGrantIn(add, &g) {
					add = append(add, g)
				}
			}
		}
		sgoDialerPublish(d, add)
	}
	return len(d.resolved)
}
