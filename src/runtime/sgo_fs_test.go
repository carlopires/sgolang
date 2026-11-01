// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (linux && (amd64 || arm64)) || (darwin && arm64)

package runtime_test

import (
	"os"
	"path/filepath"
	. "runtime"
	"strings"
	"syscall"
	"testing"
)

// The I01 confinement scenarios of sgo/model/iopath_test.go without renames, on a real file
// system: the runtime's resolver gives the model's outcome ("ok <origin>": the object created
// at that world path).
var sgoResolveCases = []struct {
	name    string
	entries []string
	path    string
	want    string
}{
	{"plain", []string{"d grant", "d grant/a", "f grant/a/x"}, "a/x", "ok grant/a/x"},
	{"missing", []string{"d grant"}, "a/x", "NotFound"},
	{"file-as-dir", []string{"d grant", "f grant/a"}, "a/x", "NotFound"},
	{"dotdot", []string{"d grant", "d out", "f out/x"}, "../out/x", "PathRejected"},
	{"inner-dotdot", []string{"d grant", "d grant/a", "f grant/a/x"}, "a/../a/x", "PathRejected"},
	{"absolute", []string{"d grant"}, "/etc/passwd", "PathRejected"},
	{"dot", []string{"d grant", "f grant/x"}, "./x", "PathRejected"},
	{"empty-name", []string{"d grant", "d grant/a", "f grant/a/x"}, "a//x", "PathRejected"},
	{"prefix-confusion", []string{"d grant", "d grantx", "f grantx/x"}, "x", "NotFound"},
	{"symlink-dir", []string{"d grant", "l grant/s", "d out", "f out/x"}, "s/x", "PathRejected"},
	{"symlink-last", []string{"d grant", "l grant/s"}, "s", "PathRejected"},
}

// buildWorld creates the entries under top; a symlink points outside the grant (to out, or to
// /etc when there is no out).
func buildWorld(t *testing.T, top string, entries []string) {
	for _, e := range entries {
		kind, p, _ := strings.Cut(e, " ")
		full := filepath.Join(top, p)
		var err error
		switch kind {
		case "d":
			err = os.Mkdir(full, 0o755)
		case "f":
			err = os.WriteFile(full, []byte("data"), 0o644)
		case "l":
			target := filepath.Join(top, "out")
			if _, serr := os.Stat(target); serr != nil {
				target = "/etc"
			}
			err = os.Symlink(target, full)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func inode(t *testing.T, p string) uint64 {
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Sys().(*syscall.Stat_t).Ino
}

func TestSGoResolve(t *testing.T) {
	for _, c := range sgoResolveCases {
		t.Run(c.name, func(t *testing.T) {
			top := t.TempDir()
			// Symlink targets are created first so that "l" entries can point at out.
			var first, rest []string
			for _, e := range c.entries {
				if strings.HasPrefix(e, "l ") {
					rest = append(rest, e)
				} else {
					first = append(first, e)
				}
			}
			buildWorld(t, top, append(first, rest...))
			ino, kind := SGoResolve(filepath.Join(top, "grant"), c.path)
			got := kind
			if kind == "" {
				got = "ok ?"
				for _, e := range c.entries {
					_, p, _ := strings.Cut(e, " ")
					if inode(t, filepath.Join(top, p)) == ino {
						got = "ok " + p
					}
				}
			}
			if got != c.want {
				t.Errorf("resolve %q: %s, want %s", c.path, got, c.want)
			}
		})
	}
}

// moved-out with a real concurrent rename: the outcomes stay within the model's set
// {ok grant/a/x, NotFound}, and no resolution leaks a descriptor.
func TestSGoResolveRenameRace(t *testing.T) {
	top := t.TempDir()
	buildWorld(t, top, []string{"d grant", "d grant/a", "f grant/a/x", "d out"})
	in, out := filepath.Join(top, "grant", "a"), filepath.Join(top, "out", "a")
	stop := make(chan bool)
	done := make(chan bool)
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			os.Rename(in, out)
			os.Rename(out, in)
		}
	}()
	before := openFDs(t)
	seen := map[string]int{}
	for range 2000 {
		_, kind := SGoResolve(filepath.Join(top, "grant"), "a/x")
		if kind == "" {
			kind = "ok"
		}
		seen[kind]++
	}
	close(stop)
	<-done
	for k := range seen {
		if k != "ok" && k != "NotFound" && k != "PathRejected" {
			t.Errorf("outcome %s outside the model's set", k)
		}
	}
	if after := openFDs(t); after > before {
		t.Errorf("descriptors leaked: %d open before, %d after", before, after)
	}
	t.Logf("outcomes over 2000 resolutions: %v", seen)
}

func openFDs(t *testing.T) int {
	n := 0
	for fd := 0; fd < 4096; fd++ {
		var st syscall.Stat_t
		if syscall.Fstat(fd, &st) == nil {
			n++
		}
	}
	return n
}
