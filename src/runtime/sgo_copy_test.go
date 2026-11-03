// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"runtime"
	"runtime/debug"
	"testing"
)

type copyB struct {
	x int
	y []int
}

type copyV struct {
	px *int
	b  *copyB
}

// TestSGoCopyInteriorPointer copies a graph where a pointer to a struct and a pointer to its first
// field share an address (W13 review V1). Each copy must be allocated at its own type and size;
// with the GC running constantly, a copy of copyB allocated at the size of an int is reported by
// the collector or corrupts the slices written into it.
func TestSGoCopyInteriorPointer(t *testing.T) {
	defer debug.SetGCPercent(debug.SetGCPercent(1))
	b := &copyB{x: 1, y: []int{7, 8, 9}}
	sum := 0
	for i := 0; i < 10000; i++ {
		w := runtime.SGoCopy(copyV{px: &b.x, b: b}).(copyV)
		if len(w.b.y) != 3 || w.b.y[2] != 9 || *w.px != 1 || w.b.x != 1 {
			t.Fatalf("iteration %d: corrupt copy: len(y)=%d px=%d x=%d", i, len(w.b.y), *w.px, w.b.x)
		}
		*w.px = 5 // interior aliasing is not preserved: the copies are separate (SGO-COMP-004 v3)
		if w.b.x != 1 {
			t.Fatalf("iteration %d: the copy of &b.x aliases the copy of b", i)
		}
		w.b.y = make([]int, 4)
		sum += len(w.b.y)
	}
	runtime.GC()
	if sum != 40000 || b.x != 1 {
		t.Fatalf("sum %d, b.x %d", sum, b.x)
	}
}

// TestSGoCopyCycleAndSharing keeps the properties V1's fix must not change: pointers of one type to
// one variable are one copied object, and cycles are preserved.
func TestSGoCopyCycleAndSharing(t *testing.T) {
	type node struct {
		v    int
		next *node
	}
	a := &node{v: 1}
	a.next = &node{v: 2, next: a}
	type pair struct{ p, q *node }
	c := runtime.SGoCopy(pair{a, a}).(pair)
	if c.p != c.q || c.p == a || c.p.next.next != c.p || c.p.next.v != 2 {
		t.Fatalf("sharing or cycle lost")
	}
}
