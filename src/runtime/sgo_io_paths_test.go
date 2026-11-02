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
