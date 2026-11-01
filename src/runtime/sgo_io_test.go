// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// recovered runs f and returns the error it panicked with, or nil.
func recovered(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()
	f()
	return nil
}

// TestSGoHandle checks the handle rules of sgo/model/iohandle.go (I03–I05, D54–D56) on real files.
func TestSGoHandle(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "in"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	root(t, func() {
		d := runtime.SGoDirOpen(dir, false)
		ro := runtime.SGoDirOpen(dir, true)
		if d == nil || ro == nil {
			t.Fatal("grant directory")
		}
		// Write, size, read to EOF; a read-only grant cannot create; directories are not files.
		f, k := runtime.SGoDirFile(d, "out", true)
		if k != "" {
			t.Fatalf("create: %s", k)
		}
		if n, k := runtime.SGoFileWrite(f, "abc"); n != 3 || k != "" {
			t.Fatalf("write: %d %s", n, k)
		}
		if k := runtime.SGoHandleClose(f); k != "" {
			t.Fatalf("close: %s", k)
		}
		if k := runtime.SGoHandleClose(f); k != "HandleClosed" {
			t.Fatalf("second close: %q", k)
		}
		if _, k := runtime.SGoDirFile(ro, "x", true); k != "ReadOnly" {
			t.Fatalf("create in ro grant: %q", k)
		}
		if _, k := runtime.SGoDirFile(d, ".", false); k != "IsDirectory" && k != "PathRejected" {
			t.Fatalf("open directory as file: %q", k)
		}
		in, k := runtime.SGoDirFile(ro, "in", false)
		if k != "" {
			t.Fatalf("open: %s", k)
		}
		if n, k := runtime.SGoFileSize(in); n != 5 || k != "" {
			t.Fatalf("size: %d %s", n, k)
		}
		if _, k := runtime.SGoFileWrite(in, "x"); k != "ReadOnly" {
			t.Fatalf("write to a read handle: %q", k)
		}
		var got string
		for {
			s, k := runtime.SGoFileRead(in, 2)
			if k == "EOF" {
				break
			}
			if k != "" || len(s) == 0 || len(s) > 2 {
				t.Fatalf("read: %q %s", s, k)
			}
			got += s
		}
		if got != "hello" {
			t.Fatalf("read %q", got)
		}

		// I04: a handle lent to a nursery is usable inside it and cannot be closed until it closes;
		// a task outside the lend may not use it, and only the owner may close it.
		s := mustOpen(t)
		runtime.SGoHandleLend(in, s)
		var notOwner, outside error
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			if _, k := runtime.SGoFileSize(in); k != "" {
				t.Errorf("lent use: %s", k)
			}
			notOwner = recovered(func() { runtime.SGoHandleClose(in) })
			return nil
		})
		if k := runtime.SGoHandleClose(in); k != "HandleLent" {
			t.Errorf("close while lent: %q", k)
		}
		mustJoin(t, s)
		if notOwner == nil || !isViolation(notOwner) {
			t.Errorf("close by a borrower: %v", notOwner)
		}
		s = mustOpen(t)
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			outside = recovered(func() { runtime.SGoFileSize(in) })
			return nil
		})
		mustJoin(t, s)
		if outside == nil {
			t.Error("use outside the lend was allowed")
		}
		if k := runtime.SGoHandleClose(in); k != "" {
			t.Fatalf("close after the lend: %s", k)
		}

		// I05: a closed handle reaches nothing, even when its descriptor number is reused.
		again, _ := runtime.SGoDirFile(ro, "in", false)
		if _, k := runtime.SGoFileRead(in, 5); k != "HandleClosed" {
			t.Errorf("read of a closed handle: %q", k)
		}
		if s, k := runtime.SGoFileRead(again, 5); s != "hello" || k != "" {
			t.Errorf("new handle: %q %s", s, k)
		}
		runtime.SGoHandleClose(again)

		// I03: a handle left open in a nursery body is closed at its exit, recorded as abandoned.
		before, abandoned := runtime.SGoOpenHandles(), runtime.SGoAbandoned()
		s = mustOpen(t)
		left, _ := runtime.SGoDirFile(ro, "in", false)
		mustJoin(t, s)
		if runtime.SGoOpenHandles() != before || runtime.SGoAbandoned() != abandoned+1 {
			t.Errorf("abandoned handle: open %d (want %d), abandoned %d", runtime.SGoOpenHandles(), before, runtime.SGoAbandoned()-abandoned)
		}
		if runtime.SGoHandleFD(left) != -1 {
			t.Error("abandoned handle still holds a descriptor")
		}

		// D55: an operation started after cancellation observes it (the nursery body is cancelled).
		h, _ := runtime.SGoDirFile(ro, "in", false)
		s = mustOpen(t)
		runtime.SGoCancel(s)
		if _, k := runtime.SGoFileRead(h, 5); k != "Cancelled" {
			t.Errorf("read after cancellation: %q", k)
		}
		runtime.SGoJoin(s)
		if _, k := runtime.SGoFileRead(h, 5); k != "" {
			t.Errorf("read after the cancelled nursery: %q", k)
		}
		runtime.SGoHandleClose(h)
		runtime.SGoHandleClose(d)
		runtime.SGoHandleClose(ro)
	})
}
