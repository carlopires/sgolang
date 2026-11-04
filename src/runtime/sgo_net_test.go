// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"runtime"
	"strings"
	"testing"
)

// waitNetWaiters yields until n tasks wait on sockets.
func waitNetWaiters(t *testing.T, n int) {
	for i := 0; runtime.SGoNetWaiters() != n; i++ {
		if i > 1_000_000 {
			t.Fatalf("%d socket waiters, want %d", runtime.SGoNetWaiters(), n)
		}
		runtime.Gosched()
	}
}

// TestSGoNet: TCP on the poller (D55–D57): echo, EOF, refused connections, writes larger than the
// socket buffers, and cancellation of Accept, Read and Write waits, after which the handles work.
func TestSGoNet(t *testing.T) {
	root(t, func() {
		l, port, k := runtime.SGoListen(0)
		if k != "" {
			t.Fatalf("listen: %s", k)
		}
		// Echo: a child connects and writes; the root accepts and reads to EOF.
		big := strings.Repeat("0123456789", 300_000) // 3 MB: blocks on full socket buffers
		s := mustOpen(t)
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			c, k := runtime.SGoConnect(port)
			if k != "" {
				t.Errorf("connect: %s", k)
				return nil
			}
			if n, k := runtime.SGoConnWrite(c, big); n != len(big) || k != "" {
				t.Errorf("write: %d %s", n, k)
			}
			runtime.SGoHandleClose(c)
			return nil
		})
		c, k := runtime.SGoAccept(l)
		if k != "" {
			t.Fatalf("accept: %s", k)
		}
		var got strings.Builder
		for {
			s, k := runtime.SGoConnRead(c, 1<<16)
			if k == "EOF" {
				break
			}
			if k != "" {
				t.Fatalf("read: %s", k)
			}
			got.WriteString(s)
		}
		mustJoin(t, s)
		runtime.SGoHandleClose(c)
		if got.String() != big {
			t.Fatalf("read %d bytes, want %d", got.Len(), len(big))
		}

		// A port nobody listens on refuses.
		l2, p2, _ := runtime.SGoListen(0)
		runtime.SGoHandleClose(l2)
		if _, k := runtime.SGoConnect(p2); k != "ConnectionRefused" {
			t.Errorf("connect to a closed port: %q", k)
		}

		// Cancellation ends an Accept wait; the listener then accepts again.
		s = mustOpen(t)
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			if _, k := runtime.SGoAccept(l); k != "Cancelled" {
				t.Errorf("accept under cancellation: %q", k)
			}
			return nil
		})
		waitNetWaiters(t, 1)
		runtime.SGoCancel(s)
		runtime.SGoJoin(s)
		cl, k1 := runtime.SGoConnect(port)
		sv, k2 := runtime.SGoAccept(l)
		if k1 != "" || k2 != "" {
			t.Fatalf("after a cancelled accept: %q %q", k1, k2)
		}

		// Cancellation ends a Read wait and a Write wait (with its progress); the connection works
		// afterwards.
		s = mustOpen(t)
		var wrote int
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			if _, k := runtime.SGoConnRead(sv, 10); k != "Cancelled" {
				t.Errorf("read under cancellation: %q", k)
			}
			return nil
		})
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			n, k := runtime.SGoConnWrite(sv, big+big+big) // nobody reads cl: blocks
			if k != "Cancelled" || n <= 0 || n >= 3*len(big) {
				t.Errorf("write under cancellation: %d %q", n, k)
			}
			wrote = n
			return nil
		})
		waitNetWaiters(t, 2)
		runtime.SGoCancel(s)
		runtime.SGoJoin(s)
		// Drain what was written, then the connection still carries data.
		for n := 0; n < wrote; {
			s, k := runtime.SGoConnRead(cl, 1<<20)
			if k != "" {
				t.Fatalf("drain: %s", k)
			}
			n += len(s)
		}
		if n, k := runtime.SGoConnWrite(cl, "ok"); n != 2 || k != "" {
			t.Fatalf("write after cancellation: %d %s", n, k)
		}
		if s, k := runtime.SGoConnRead(sv, 10); s != "ok" || k != "" {
			t.Fatalf("read after cancellation: %q %s", s, k)
		}

		for _, h := range []*runtime.SGoHandle{cl, sv, l} {
			if k := runtime.SGoHandleClose(h); k != "" {
				t.Errorf("close: %s", k)
			}
		}
		if runtime.SGoNetWaiters() != 0 {
			t.Error("socket waiters left")
		}
	})
}
