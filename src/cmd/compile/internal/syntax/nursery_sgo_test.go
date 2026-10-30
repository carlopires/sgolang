// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import (
	"strings"
	"testing"
)

func parseSGo(t *testing.T, src string) (*File, []error) {
	t.Helper()
	var errs []error
	f, _ := Parse(NewFileBase("x.sgo"), strings.NewReader(src), func(err error) { errs = append(errs, err) }, nil, CheckBranches)
	return f, errs
}

func findNursery(f *File) []*NurseryStmt {
	var ns []*NurseryStmt
	Inspect(f, func(n Node) bool {
		if s, ok := n.(*NurseryStmt); ok {
			ns = append(ns, s)
		}
		return true
	})
	return ns
}

// SGO-NUR-001: both forms parse; the binder and body are recorded; printing round-trips.
func TestNurseryParse(t *testing.T) {
	src := `package p

func f() {
	nursery (n) {
		n.Spawn(g)
	}
	nursery {
	}
}
`
	f, errs := parseSGo(t, src)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	ns := findNursery(f)
	if len(ns) != 2 {
		t.Fatalf("found %d nursery statements, want 2", len(ns))
	}
	if ns[0].Name == nil || ns[0].Name.Value != "n" || len(ns[0].Body.List) != 1 {
		t.Errorf("named form: %+v", ns[0])
	}
	if ns[1].Name != nil || len(ns[1].Body.List) != 0 {
		t.Errorf("anonymous form: %+v", ns[1])
	}
	if line, col := ns[0].Pos().Line(), ns[0].Pos().Col(); line != 4 || col != 2 {
		t.Errorf("position %d:%d, want 4:2", line, col)
	}
	out := String(f)
	if !strings.Contains(out, "nursery (n) {") || !strings.Contains(out, "nursery {") {
		t.Errorf("printer output:\n%s", out)
	}
	f2, errs := parseSGo(t, out)
	if len(errs) != 0 || len(findNursery(f2)) != 2 {
		t.Errorf("printed form does not re-parse: %v", errs)
	}
}

// SGO-CTL-004 / SGO-CTL-001: break and continue may leave a nursery to an enclosing loop or switch;
// a break with no enclosing target is still an error inside a nursery.
func TestNurseryBranches(t *testing.T) {
	ok := `package p

func f() {
outer:
	for {
		nursery (n) {
			if c() {
				break
			}
			if d() {
				continue outer
			}
			break outer
		}
	}
}
`
	if _, errs := parseSGo(t, ok); len(errs) != 0 {
		t.Errorf("valid branches out of a nursery rejected: %v", errs)
	}
	bad := `package p

func f() {
	nursery (n) {
		break
	}
}
`
	_, errs := parseSGo(t, bad)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "break is not in a loop") {
		t.Errorf("break without target inside a nursery: %v", errs)
	}
}

// SGO-LEX-003: nursery is reserved and cannot be a name.
func TestNurseryReserved(t *testing.T) {
	for _, src := range []string{
		"package p\n\nfunc f() {\n\tnursery := 1\n}\n",
		"package p\n\nfunc nursery() {}\n",
	} {
		if _, errs := parseSGo(t, src); len(errs) == 0 {
			t.Errorf("nursery accepted as a name in:\n%s", src)
		}
	}
}
