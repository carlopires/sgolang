// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgoadmit

import (
	"os"
	"path/filepath"
	"testing"
)

// srcRoot is the tree under test: the host lane sets SGO_HOST_TEST_GOROOT; in place it is ../../../..
func srcRoot(t *testing.T) string {
	if r := os.Getenv("SGO_HOST_TEST_GOROOT"); r != "" {
		return filepath.Join(r, "src")
	}
	r, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func copyFile(t *testing.T, from, to string, extra string) string {
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, append(data, extra...), 0o644); err != nil {
		t.Fatal(err)
	}
	return to
}

// TestTrusted: trust is a function of (import path, file name, digest), never of content alone
// (SGO-TCB-002, W12 R1).
func TestTrusted(t *testing.T) {
	src := srcRoot(t)
	pinned := filepath.Join(src, "internal", "stringslite", "strings.go")
	tmp := t.TempDir()
	copySame := copyFile(t, pinned, filepath.Join(tmp, "strings.go"), "")
	copyRenamed := copyFile(t, pinned, filepath.Join(tmp, "other.go"), "")
	modified := copyFile(t, pinned, filepath.Join(t.TempDir(), "strings.go"), "// x\n")
	for _, tc := range []struct {
		name  string
		path  string
		files []string
		want  bool
	}{
		{"pinned file at its own path", "internal/stringslite", []string{pinned}, true},
		{"byte copy, same name, at its own path", "internal/stringslite", []string{copySame}, true},
		{"byte copy under a user path", "example.com/p2/sl", []string{copySame}, false},
		{"byte copy as package main", "main", []string{copySame}, false},
		{"byte copy renamed at the TCB path", "internal/stringslite", []string{copyRenamed}, false},
		{"modified file at the TCB path", "internal/stringslite", []string{modified}, false},
		{"pinned file under another TCB path", "internal/bytealg", []string{pinned}, false},
		{"no files", "internal/stringslite", nil, false},
	} {
		if got := Trusted(tc.path, tc.files); got != tc.want {
			t.Errorf("%s: Trusted(%q) = %v, want %v", tc.name, tc.path, got, tc.want)
		}
	}
	if !IsTCBPackage("runtime") || !IsTCBPackage("sgo") || IsTCBPackage("main") || IsTCBPackage("example.com/p2/sl") {
		t.Errorf("IsTCBPackage: wrong answer for runtime, sgo, main, or a user path")
	}
	if f, msg := CheckTCB("internal/stringslite", []string{modified}); f != modified || msg == "" {
		t.Errorf("CheckTCB on a modified TCB file = %q, %q; want the file and a message", f, msg)
	}
	if f, msg := CheckTCB("example.com/p2/sl", []string{copySame}); f != "" || msg != "" {
		t.Errorf("CheckTCB on a user path = %q, %q; want no TCB verdict (admission decides)", f, msg)
	}
}
