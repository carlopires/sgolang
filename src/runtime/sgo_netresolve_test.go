// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"runtime"
	"slices"
	"testing"
)

// dnsMsg builds a response to (id, host, qtype) whose answer section is the given records.
type dnsRR struct {
	owner string
	typ   uint16
	data  []byte
}

func dnsName(n string) []byte {
	var b []byte
	start := 0
	for k := 0; k <= len(n); k++ {
		if k == len(n) || n[k] == '.' {
			b = append(b, byte(k-start))
			b = append(b, n[start:k]...)
			start = k + 1
		}
	}
	return append(b, 0)
}

func dnsMsg(id uint16, qname string, qtype uint16, rcode byte, rrs []dnsRR) []byte {
	m := []byte{byte(id >> 8), byte(id), 0x81, 0x80 | rcode, 0, 1, 0, byte(len(rrs)), 0, 0, 0, 0}
	m = append(m, dnsName(qname)...)
	m = append(m, byte(qtype>>8), byte(qtype), 0, 1)
	for _, r := range rrs {
		m = append(m, dnsName(r.owner)...)
		m = append(m, byte(r.typ>>8), byte(r.typ), 0, 1, 0, 0, 0, 60, byte(len(r.data)>>8), byte(len(r.data)))
		m = append(m, r.data...)
	}
	return m
}

func TestSGoResolveDNSAnswer(t *testing.T) {
	a := func(o string, ip ...byte) dnsRR { return dnsRR{o, 1, ip} }
	cname := func(o, target string) dnsRR { return dnsRR{o, 5, dnsName(target)} }
	cases := []struct {
		name string
		msg  []byte
		want []string
		ok   bool
	}{
		{"plain", dnsMsg(7, "example.test", 1, 0, []dnsRR{a("example.test", 192, 0, 2, 1)}), []string{"192.0.2.1:0"}, true},
		{"question case-insensitive", dnsMsg(7, "Example.TEST", 1, 0, []dnsRR{a("EXAMPLE.test", 192, 0, 2, 1)}), []string{"192.0.2.1:0"}, true},
		{"wrong question name", dnsMsg(7, "evil.test", 1, 0, []dnsRR{a("evil.test", 192, 0, 2, 9)}), nil, false},
		{"wrong question type", dnsMsg(7, "example.test", 28, 0, nil), nil, false},
		{"wrong id", dnsMsg(8, "example.test", 1, 0, []dnsRR{a("example.test", 192, 0, 2, 1)}), nil, false},
		{"unrelated owner ignored", dnsMsg(7, "example.test", 1, 0, []dnsRR{a("other.test", 192, 0, 2, 9), a("example.test", 192, 0, 2, 1)}), []string{"192.0.2.1:0"}, true},
		{"cname chain", dnsMsg(7, "example.test", 1, 0, []dnsRR{cname("example.test", "a.cdn.test"), cname("a.cdn.test", "b.cdn.test"), a("b.cdn.test", 192, 0, 2, 5), a("stray.test", 192, 0, 2, 6)}), []string{"192.0.2.5:0"}, true},
		{"cname out of order", dnsMsg(7, "example.test", 1, 0, []dnsRR{a("b.cdn.test", 192, 0, 2, 5), cname("a.cdn.test", "b.cdn.test"), cname("example.test", "a.cdn.test")}), []string{"192.0.2.5:0"}, true},
		{"nxdomain", dnsMsg(7, "example.test", 1, 3, nil), nil, true},
	}
	for _, c := range cases {
		got, ok := runtime.SGoDNSAnswer(c.msg, 7, 1, "example.test")
		if ok != c.ok || !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, %v; want %v, %v", c.name, got, ok, c.want, c.ok)
		}
	}
	// A compressed question name is refused; a compressed answer owner is followed.
	m := dnsMsg(7, "example.test", 1, 0, nil)
	comp := append([]byte{}, m[:12]...)
	comp = append(comp, 0xc0, 12, 0, 1, 0, 1)
	if _, ok := runtime.SGoDNSAnswer(comp, 7, 1, "example.test"); ok {
		t.Errorf("compressed question accepted")
	}
	m = dnsMsg(7, "example.test", 1, 0, nil)
	m[7] = 1
	m = append(m, 0xc0, 12, 0, 1, 0, 1, 0, 0, 0, 60, 0, 4, 192, 0, 2, 7)
	if got, ok := runtime.SGoDNSAnswer(m, 7, 1, "example.test"); !ok || !slices.Equal(got, []string{"192.0.2.7:0"}) {
		t.Errorf("compressed owner: got %v, %v", got, ok)
	}
	// A pointer loop ends.
	loop := dnsMsg(7, "example.test", 1, 0, nil)
	loop[7] = 1
	at := len(loop)
	loop = append(loop, 0xc0, byte(at), 0, 1, 0, 1, 0, 0, 0, 60, 0, 4, 1, 2, 3, 4)
	if _, ok := runtime.SGoDNSAnswer(loop, 7, 1, "example.test"); ok {
		t.Errorf("pointer loop accepted")
	}
}

func FuzzSGoDNSAnswer(f *testing.F) {
	f.Add(dnsMsg(7, "example.test", 1, 0, []dnsRR{{"example.test", 1, []byte{192, 0, 2, 1}}}))
	f.Add(dnsMsg(7, "example.test", 1, 0, []dnsRR{{"example.test", 5, dnsName("x.test")}, {"x.test", 1, []byte{1, 2, 3, 4}}}))
	f.Fuzz(func(t *testing.T, msg []byte) {
		got, ok := runtime.SGoDNSAnswer(msg, 7, 1, "example.test")
		if !ok && len(got) > 0 {
			t.Fatalf("addresses from a refused message: %v", got)
		}
		if len(got) > len(msg)/14 {
			t.Fatalf("more addresses (%d) than records could hold in %d bytes", len(got), len(msg))
		}
	})
}

func TestSGoResolveResolvable(t *testing.T) {
	for lit, want := range map[string]bool{
		"0.0.0.0": false, "::": false, "::ffff:127.0.0.1": false, "::ffff:192.0.2.1": false,
		"127.0.0.1": true, "::1": true, "192.0.2.1": true, "2001:db8::1": true, "64:ff9b::1": true,
	} {
		if got := runtime.SGoResolvable(lit); got != want {
			t.Errorf("SGoResolvable(%q) = %v, want %v", lit, got, want)
		}
	}
}

func TestSGoResolveNoGrowth(t *testing.T) {
	n := runtime.SGoResolvedGrowth([]string{"127.0.0.1", "::1", "127.0.0.1"}, []uint16{443, 8443}, 1000)
	if n != 4 {
		t.Fatalf("resolved table after 1000 rounds: %d entries, want 4 (2 addresses x 2 ports)", n)
	}
}
