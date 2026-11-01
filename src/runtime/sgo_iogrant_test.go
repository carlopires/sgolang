// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"net/netip"
	"reflect"
	"runtime"
	"testing"
)

// TestSGoParseAddr checks the runtime's endpoint parser against net/netip.
func TestSGoParseAddr(t *testing.T) {
	good := []string{"127.0.0.1:80", "0.0.0.0:65535", "10.1.2.3:1", "[::1]:8080", "[::]:1",
		"[2001:db8::1]:443", "[fe80::1:2:3:4]:9", "[1:2:3:4:5:6:7:8]:22", "[::ffff:102:304]:5", "[1::]:3"}
	bad := []string{"", ":80", "127.0.0.1", "127.0.0.1:", "127.0.0.1:0", "127.0.0.1:65536", "1.2.3:4",
		"1.2.3.4.5:6", "256.0.0.1:1", "a.b.c.d:1", "[::1:80", "[1:2:3:4:5:6:7:8:9]:1", "[1::2::3]:1",
		"[:1]:1", "[1:]:1", "[1:2::3::]:1", "[12345::]:1", "localhost:80", "[::1]:x"}
	for _, a := range good {
		ip, v6, port, ok := runtime.SGoParseAddr(a)
		want := netip.MustParseAddrPort(a)
		if !ok || port != int(want.Port()) || v6 != want.Addr().Is6() || ip != want.Addr().As16() {
			t.Errorf("%q: ok=%v v6=%v port=%d ip=%v, want %v", a, ok, v6, port, ip, want)
		}
	}
	for _, a := range bad {
		if _, _, _, ok := runtime.SGoParseAddr(a); ok {
			t.Errorf("%q accepted", a)
		}
	}
	if got := runtime.SGoFields("dir data rw /a b/c", 4); !reflect.DeepEqual(got, []string{"dir", "data", "rw", "/a b/c"}) {
		t.Errorf("fields: %q", got)
	}
}
