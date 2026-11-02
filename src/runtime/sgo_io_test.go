// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
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

// TestSGoDirList: sorted names without "." and "..", subdirectories, many entries (several
// getdents/readdir batches), symlinks listed but not followed, and the sort itself.
func TestSGoDirList(t *testing.T) {
	dir := t.TempDir()
	var want []string
	for i := range 700 {
		n := fmt.Sprintf("f%04d-%s", (i*7919)%1000, strings.Repeat("x", i%40))
		want = append(want, n)
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "sub", "b"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, "sub", "a"), nil, 0o644)
	os.Symlink("/etc", filepath.Join(dir, "link"))
	want = append(want, "sub", "link")
	sort.Strings(want)
	root(t, func() {
		got, k := runtime.SGoDirList(dir, "")
		if k != "" || !slices.Equal(got, want) {
			t.Fatalf("list: %s, %d names (want %d)", k, len(got), len(want))
		}
		if got, k := runtime.SGoDirList(dir, "sub"); k != "" || !slices.Equal(got, []string{"a", "b"}) {
			t.Errorf("sub: %q %s", got, k)
		}
		for path, kind := range map[string]string{"link": "PathRejected", "missing": "NotFound", "..": "PathRejected", "sub/a": "NotFound"} {
			if _, k := runtime.SGoDirList(dir, path); k != kind {
				t.Errorf("list %q: %q, want %q", path, k, kind)
			}
		}
	})
	s := []string{"b", "a", "c", "", "ab", "a", "z", "\xff", "0"}
	ref := slices.Clone(s)
	sort.Strings(ref)
	runtime.SGoSortStrings(s)
	if !slices.Equal(s, ref) {
		t.Errorf("sort: %q", s)
	}
}

// TestSGoBlockedOpenDoesNotStallGC: a task blocked in the adapter's openat (a FIFO with no writer)
// must not stop the world from stopping: the adapter tells the scheduler about the system call
// (entersyscall). Before W12 R4 the call held its P and the first GC waited for it forever.
func TestSGoBlockedOpenDoesNotStallGC(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skip("mkfifo:", err)
	}
	opened := make(chan int32, 1)
	go func() { opened <- runtime.SGoOpenBlocking(fifo) }()
	time.Sleep(50 * time.Millisecond) // let the open block in the kernel
	done := make(chan struct{})
	go func() {
		for i := 0; i < 20; i++ {
			runtime.GC()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("GC did not complete while a task was blocked in the adapter's openat")
	}
	// Unblock the reader: open the FIFO for writing.
	w, err := os.OpenFile(fifo, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if fd := <-opened; fd < 0 {
		t.Fatalf("blocking open failed: %d", fd)
	} else {
		runtime.SGoCloseFD(fd)
	}
}

// TestSGoRenameRaces: a directory moved out of the grant while an operation is between resolving
// and acting (W12 R6). Remove and Mkdir report the effect they had (PathRaced, NoEffect false);
// Create never truncates a file outside the grant.
func TestSGoRenameRaces(t *testing.T) {
	runtime.SGoEnterRoot()
	defer runtime.SGoLeaveRoot()
	for _, op := range []string{"remove", "mkdir"} {
		base := t.TempDir()
		grant, outside := filepath.Join(base, "grant"), filepath.Join(base, "outside")
		for _, d := range []string{filepath.Join(grant, "a", "b"), outside} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(grant, "a", "b", "x"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		target := "a/b/x"
		if op == "mkdir" {
			target = "a/b/y"
		}
		moved := false
		kind, noEffect := runtime.SGoDirOp(grant, op, target, func(site string) {
			if site == "effect" && !moved {
				moved = true
				if err := os.Rename(filepath.Join(grant, "a"), filepath.Join(outside, "a")); err != nil {
					t.Error(err)
				}
			}
		})
		if kind != "PathRaced" || noEffect {
			t.Errorf("%s with the parent moved out: kind %q, NoEffect %v; want PathRaced, false", op, kind, noEffect)
		}
	}

	base := t.TempDir()
	grant, outside := filepath.Join(base, "grant"), filepath.Join(base, "outside")
	for _, d := range []string{filepath.Join(grant, "a"), outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	precious := filepath.Join(grant, "a", "f")
	if err := os.WriteFile(precious, []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}
	moved := false
	kind, _ := runtime.SGoDirOp(grant, "create", "a/f", func(site string) {
		if site == "walk" && !moved {
			moved = true
			if err := os.Rename(filepath.Join(grant, "a"), filepath.Join(outside, "a")); err != nil {
				t.Error(err)
			}
		}
	})
	if data, err := os.ReadFile(filepath.Join(outside, "a", "f")); moved && (err != nil || string(data) != "precious") {
		t.Errorf("Create through a raced path changed the file outside the grant: %q, %v (kind %q)", data, err, kind)
	}
	// The verification fails and the retry finds a/ gone from the grant: NotFound (or
	// PathRejected if renames kept racing). Either way nothing inside the grant was created.
	if moved && kind != "NotFound" && kind != "PathRejected" {
		t.Errorf("Create through a raced path: kind %q, want NotFound or PathRejected", kind)
	}
	if !moved && !runtime.SGoHasOpenat2() {
		t.Error("the walk's race window was never reached")
	}
}
