// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSGoNetUnix: Unix domain sockets (D60): a stale socket file is replaced, any other file is
// AddressInUse; echo; PeerUID is the peer's user ID (Unsupported on TCP); closing the listener
// removes its socket file; cancellation of an Accept wait; endpoint parsing limits.
func TestSGoNetUnix(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "sgo") // short: sun_path is 104 bytes on darwin
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "s.sock")
	stale, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	stale.(*net.UnixListener).SetUnlinkOnClose(false)
	stale.Close() // leaves a stale socket file
	regular := filepath.Join(dir, "regular")
	os.WriteFile(regular, []byte("x"), 0o644)
	root(t, func() {
		if _, k := runtime.SGoListenUnix(regular); k != "AddressInUse" {
			t.Errorf("listen over a regular file: %q", k)
		}
		if b, _ := os.ReadFile(regular); string(b) != "x" {
			t.Error("a regular file was removed")
		}
		if _, k := runtime.SGoConnectUnix(path); k != "ConnectionRefused" {
			t.Errorf("connect to a stale socket: %q", k)
		}
		l, k := runtime.SGoListenUnix(path)
		if k != "" {
			t.Fatalf("listen over a stale socket: %s", k)
		}
		s := mustOpen(t)
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			c, k := runtime.SGoConnectUnix(path)
			if k != "" {
				t.Errorf("connect: %s", k)
				return nil
			}
			runtime.SGoConnWrite(c, "ping")
			if got, k := runtime.SGoConnRead(c, 10); got != "pong" || k != "" {
				t.Errorf("client read: %q %s", got, k)
			}
			runtime.SGoHandleClose(c)
			return nil
		})
		c, k := runtime.SGoAccept(l)
		if k != "" {
			t.Fatalf("accept: %s", k)
		}
		if got, k := runtime.SGoConnRead(c, 10); got != "ping" || k != "" {
			t.Errorf("server read: %q %s", got, k)
		}
		if uid, k := runtime.SGoPeerUID(c); k != "" || uid != os.Getuid() {
			t.Errorf("PeerUID: %d %s, want %d", uid, k, os.Getuid())
		}
		runtime.SGoConnWrite(c, "pong")
		if _, k := runtime.SGoConnRead(c, 10); k != "EOF" {
			t.Errorf("after the client closed: %q", k)
		}
		mustJoin(t, s)
		runtime.SGoHandleClose(c)

		// Cancellation ends an Accept wait on a Unix listener.
		s = mustOpen(t)
		runtime.SGoHandleLend(l, s)
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			if _, k := runtime.SGoAccept(l); k != "Cancelled" {
				t.Errorf("accept under cancellation: %q", k)
			}
			return nil
		})
		waitNetWaiters(t, 1)
		runtime.SGoCancel(s)
		runtime.SGoJoin(s)
		runtime.SGoHandleClose(l)
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("socket file after close: %v", err)
		}

		// PeerUID on TCP is Unsupported.
		tl, port, _ := runtime.SGoListen(0)
		tc, _ := runtime.SGoConnect(port)
		ta, _ := runtime.SGoAccept(tl)
		if _, k := runtime.SGoPeerUID(ta); k != "Unsupported" {
			t.Errorf("PeerUID on TCP: %q", k)
		}
		for _, h := range []*runtime.SGoHandle{ta, tc, tl} {
			runtime.SGoHandleClose(h)
		}
	})
	for a, ok := range map[string]bool{"unix:/run/a.sock": true, "unix:rel/a.sock": true, "unix:": false,
		"unix:/" + strings.Repeat("x", 102): true, "unix:/" + strings.Repeat("x", 103): false, "unix:/a\x00b": false} {
		if runtime.SGoParseEndpoint(a) != ok {
			t.Errorf("endpoint %q: ok=%v", a, !ok)
		}
	}
}
