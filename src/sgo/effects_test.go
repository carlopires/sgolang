// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgo

import (
	"math"
	"math/bits"
	"math/rand/v2"
	"testing"
)

// The seeded generator is exactly math/rand/v2's PCG with (seed, 0), and Intn is exactly
// (*rand.Rand).IntN over it, so a seed gives the same sequence on every target (SGO-AUTH-003).
func TestRandomMatchesMathRandV2(t *testing.T) {
	for _, seed := range []int{0, 1, 42, -7, math.MaxInt64, math.MinInt64} {
		r := &Random{hi: uint64(seed)}
		ref := rand.NewPCG(uint64(seed), 0)
		for i := range 1000 {
			if got, want := r.next64(), ref.Uint64(); got != want {
				t.Fatalf("seed %d output %d: %#x, want %#x", seed, i, got, want)
			}
		}
		r = &Random{hi: uint64(seed)}
		refRand := rand.New(rand.NewPCG(uint64(seed), 0))
		for i, n := range []uint64{1, 2, 3, 7, 10, 64, 1000, 1 << 40, 1<<63 - 1, 1<<63 + 5, math.MaxUint64} {
			for range 50 {
				if got, want := r.uint64n(n), refRand.Uint64N(n); got != want {
					t.Fatalf("seed %d bound %d (case %d): %d, want %d", seed, n, i, got, want)
				}
			}
		}
	}
}

// Split seeds the new generator from the next two outputs, as rand.NewPCG(a, b).
func TestRandomSplit(t *testing.T) {
	r := &Random{hi: 9}
	ref := rand.NewPCG(9, 0)
	a, b := ref.Uint64(), ref.Uint64()
	hi, lo := r.next64(), r.next64()
	if hi != a || lo != b {
		t.Fatalf("outputs differ")
	}
	child, refChild := &Random{hi: hi, lo: lo}, rand.NewPCG(a, b)
	for range 100 {
		if child.next64() != refChild.Uint64() {
			t.Fatal("split stream differs")
		}
	}
}

func TestMulAdd64(t *testing.T) {
	vals := []uint64{0, 1, 2, 1<<32 - 1, 1 << 32, 1<<63 - 1, 1 << 63, math.MaxUint64, 0x9e3779b97f4a7c15}
	for _, x := range vals {
		for _, y := range vals {
			h1, l1 := mul64(x, y)
			h2, l2 := bits.Mul64(x, y)
			if h1 != h2 || l1 != l2 {
				t.Fatalf("mul64(%#x, %#x)", x, y)
			}
			for c := uint64(0); c < 2; c++ {
				s1, c1 := add64(x, y, c)
				s2, c2 := bits.Add64(x, y, c)
				if s1 != s2 || c1 != c2 {
					t.Fatalf("add64(%#x, %#x, %d)", x, y, c)
				}
			}
		}
	}
}
