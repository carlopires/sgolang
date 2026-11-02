// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

// TestSGoResolutionPathsAgree: on linux, resolution with openat2 and the verified walk give the
// same outcome for every case of file-confined-paths (W12 R6 reopen: the openat2 path once made
// every read an IOError and a trailing symlink NotFound).
func TestSGoResolutionPathsAgree(t *testing.T) {
	runtime.SGoEnterRoot()
	defer runtime.SGoLeaveRoot()
	grant := t.TempDir()
	if err := os.WriteFile(filepath.Join(grant, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(grant, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("f", filepath.Join(grant, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("d", filepath.Join(grant, "dlink")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(grant, "fifo"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ op, path, want string }{
		{"open", "f", ""},
		{"open", "../f", "PathRejected"},
		{"open", "/etc/passwd", "PathRejected"},
		{"open", "link", "PathRejected"},
		{"open", "dlink/x", "PathRejected"},
		{"open", "f/x", "NotFound"},
		{"open", "missing", "NotFound"},
		{"open", "fifo", "NotRegularFile"},
		{"open", "d", "IsDirectory"},
		{"isdir", "d", ""},
		{"isdir", "dlink", "PathRejected"},
		{"isdir", "link", "PathRejected"},
		{"isdir", "f", "NotFound"},
		{"create", "d/new", ""},
	}
	for _, walk := range []bool{false, true} {
		runtime.SGoForceWalk(walk)
		for _, c := range cases {
			if got, _ := runtime.SGoDirOp(grant, c.op, c.path, nil); got != c.want {
				t.Errorf("walk=%v %s %q: %q, want %q", walk, c.op, c.path, got, c.want)
			}
		}
	}
	runtime.SGoForceWalk(false)
}

// TestSGoOpenat2EPERMIsAnOpenError: an EPERM from a real open (an immutable or append-only file)
// is that open's error; it never switches resolution off openat2 (W12 R6b).
func TestSGoOpenat2EPERMIsAnOpenError(t *testing.T) {
	runtime.SGoEnterRoot()
	defer runtime.SGoLeaveRoot()
	grant := t.TempDir()
	if err := os.WriteFile(filepath.Join(grant, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := runtime.SGoDirOp(grant, "open", "f", nil); got != "" {
		t.Fatalf("open f: %q", got)
	}
	on, before := runtime.SGoOpenat2()
	if !on {
		t.Skip("no openat2 on this system: resolution walks")
	}
	runtime.SGoOpenat2FaultOnce(1) // EPERM
	if got, _ := runtime.SGoDirOp(grant, "open", "f", nil); got != "IOError" {
		t.Errorf("open with an EPERM from the open itself: %q, want IOError", got)
	}
	if got, _ := runtime.SGoDirOp(grant, "open", "f", nil); got != "" {
		t.Errorf("open after the EPERM: %q", got)
	}
	if on, after := runtime.SGoOpenat2(); !on || after != before+2 {
		t.Errorf("after an open's EPERM: openat2 on=%v, opens %d -> %d; want on and 2 more opens", on, before, after)
	}
}
