// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgo

import "unsafe"

// Effect handles of subset 1 (SGO-AUTH-001, SGO-AUTH-003). A handle is obtained only from the root
// context of a program whose sgo.mod grants its kind, or by attenuating or lending another handle.
// Every operation checks, before any effect, that the calling task's ceiling contains the kind,
// that the handle is not revoked, and that the handle permits the operation.

func runtime_require(kind uint8, op string)
func runtime_rootGrant(task unsafe.Pointer, kind uint8, op string)
func runtime_scopeLive(scope unsafe.Pointer) bool
func runtime_canLend(scope unsafe.Pointer) bool
func runtime_currentTask() unsafe.Pointer
func runtime_claimOwner(p *unsafe.Pointer) unsafe.Pointer
func runtime_sinceMS() int
func runtime_wallMS() int
func runtime_entropy(k int) string
func runtime_storeGet(key string) (string, bool)
func runtime_storePut(key, value string)

// Effect kinds, as the runtime's ceiling bits (src/runtime/sgo_auth.go).
const (
	kindClock uint8 = 1 << iota
	kindWallClock
	kindRandom
	kindEntropy
	kindSimStore
)

// lent is the revocation state of a handle: the nurseries it was lent to, outermost first. The
// handle permits nothing once any of them is CLOSED.
type lent []unsafe.Pointer

func (l lent) check(op string) {
	for _, s := range l {
		if !runtime_scopeLive(s) {
			runtime_fail("MissingCapability", op+": the handle was lent to a nursery that has closed")
		}
	}
}

func (l lent) to(n *Nursery, op string) lent {
	if n == nil || !runtime_canLend(n.s) {
		runtime_violation(op + " into a nursery the caller cannot spawn into")
	}
	return append(append(lent(nil), l...), n.s)
}

// use checks the ceiling and revocation of an operation of kind k.
func use(k uint8, l lent, op string) {
	runtime_require(k, op)
	l.check(op)
}

// Clock reads monotonic time.
type Clock struct{ l lent }

// Clock returns the root clock handle; clock must be granted.
func (c *Context) Clock() *Clock {
	c.own()
	runtime_rootGrant(c.t, kindClock, "Clock")
	return &Clock{}
}

// Since returns the milliseconds of monotonic time since the program started; it never decreases.
func (h *Clock) Since() int {
	use(kindClock, h.l, "Clock.Since")
	return runtime_sinceMS()
}

// LendTo returns a copy of h that is revoked when n closes.
func (h *Clock) LendTo(n *Nursery) *Clock { return &Clock{h.l.to(n, "Clock.LendTo")} }

// WallClock reads UTC wall-clock time.
type WallClock struct{ l lent }

// WallClock returns the root wall-clock handle; wallclock must be granted.
func (c *Context) WallClock() *WallClock {
	c.own()
	runtime_rootGrant(c.t, kindWallClock, "WallClock")
	return &WallClock{}
}

// Now returns UTC milliseconds since 1970-01-01.
func (h *WallClock) Now() int {
	use(kindWallClock, h.l, "WallClock.Now")
	return runtime_wallMS()
}

// LendTo returns a copy of h that is revoked when n closes.
func (h *WallClock) LendTo(n *Nursery) *WallClock {
	return &WallClock{h.l.to(n, "WallClock.LendTo")}
}

// Entropy reads the operating system's cryptographic random source.
type Entropy struct{ l lent }

// Entropy returns the root entropy handle; entropy must be granted.
func (c *Context) Entropy() *Entropy {
	c.own()
	runtime_rootGrant(c.t, kindEntropy, "Entropy")
	return &Entropy{}
}

// Bytes returns k bytes (1 <= k <= 1024) from the OS cryptographic source, as a string. It is
// never served by a seeded generator.
func (h *Entropy) Bytes(k int) string {
	if k < 1 || k > 1024 {
		runtime_fail("InvalidArgument", "Entropy.Bytes requires 1 <= k <= 1024")
	}
	use(kindEntropy, h.l, "Entropy.Bytes")
	return runtime_entropy(k)
}

// LendTo returns a copy of h that is revoked when n closes.
func (h *Entropy) LendTo(n *Nursery) *Entropy { return &Entropy{h.l.to(n, "Entropy.LendTo")} }

// Random is a deterministic generator (PCG-DXSM with 128-bit state, the algorithm of
// math/rand/v2's PCG). It is task-affine: only its owner may use it
// (the task that obtained it from Random, or the first task that used it after Split).
type Random struct {
	l      lent
	owner  unsafe.Pointer
	hi, lo uint64
}

// Random returns a generator seeded with (seed, 0); random must be granted.
func (c *Context) Random(seed int) *Random {
	c.own()
	runtime_rootGrant(c.t, kindRandom, "Random")
	return &Random{owner: c.t, hi: uint64(seed)}
}

func (r *Random) own(op string) {
	if runtime_claimOwner(&r.owner) != runtime_currentTask() {
		runtime_violation(op + " on a generator that belongs to another task")
	}
	use(kindRandom, r.l, op)
}

// Intn returns a uniform value in [0, n), n > 0, by the unbiased method of math/rand/v2.
func (r *Random) Intn(n int) int {
	if n <= 0 {
		runtime_fail("InvalidArgument", "Random.Intn requires n > 0")
	}
	r.own("Random.Intn")
	return int(r.uint64n(uint64(n)))
}

// Split returns a new generator seeded from the next two outputs. It is unowned until its first
// use: the first task that uses it becomes its owner (D38), so a parent can split one generator
// per child before spawning it.
func (r *Random) Split() *Random {
	r.own("Random.Split")
	hi := r.next64()
	lo := r.next64()
	return &Random{l: r.l, hi: hi, lo: lo}
}

// LendTo returns a copy of r's current state that is revoked when n closes. The copy belongs to
// the calling task, like r (generators are task-affine).
func (r *Random) LendTo(n *Nursery) *Random {
	r.own("Random.LendTo")
	return &Random{l: r.l.to(n, "Random.LendTo"), owner: r.owner, hi: r.hi, lo: r.lo}
}

// next64 is math/rand/v2 (*PCG).Uint64: a 128-bit LCG step followed by the DXSM output function.
func (r *Random) next64() uint64 {
	const (
		mulHi = 2549297995355413924
		mulLo = 4865540595714422341
		incHi = 6364136223846793005
		incLo = 1442695040888963407
	)
	hi, lo := mul64(r.lo, mulLo)
	hi += r.hi*mulLo + r.lo*mulHi
	lo, c := add64(lo, incLo, 0)
	hi, _ = add64(hi, incHi, c)
	r.lo, r.hi = lo, hi
	const cheapMul = 0xda942042e4dd58b5
	hi ^= hi >> 32
	hi *= cheapMul
	hi ^= hi >> (3 * 16)
	hi *= lo | 1
	return hi
}

// uint64n is math/rand/v2 (*Rand).uint64n on a 64-bit target (every admitted target).
func (r *Random) uint64n(n uint64) uint64 {
	if n&(n-1) == 0 {
		return r.next64() & (n - 1)
	}
	hi, lo := mul64(r.next64(), n)
	if lo < n {
		thresh := -n % n
		for lo < thresh {
			hi, lo = mul64(r.next64(), n)
		}
	}
	return hi
}

// mul64 and add64 are math/bits.Mul64 and Add64 (package sgo imports only unsafe).
func mul64(x, y uint64) (hi, lo uint64) {
	const mask32 = 1<<32 - 1
	x0, x1 := x&mask32, x>>32
	y0, y1 := y&mask32, y>>32
	w0 := x0 * y0
	t := x1*y0 + w0>>32
	w1, w2 := t&mask32, t>>32
	w1 += x0 * y1
	hi = x1*y1 + w2 + w1>>32
	lo = x * y
	return
}

func add64(x, y, carry uint64) (sum, carryOut uint64) {
	sum = x + y + carry
	carryOut = ((x & y) | ((x | y) &^ sum)) >> 63
	return
}

// SimStore is the program's simulated key-value store: one in-memory string map per program.
type SimStore struct {
	l        lent
	readOnly bool
	prefix   string
}

// SimStore returns the root store handle; simstore must be granted.
func (c *Context) SimStore() *SimStore {
	c.own()
	runtime_rootGrant(c.t, kindSimStore, "SimStore")
	return &SimStore{}
}

func (s *SimStore) key(op, key string) string {
	use(kindSimStore, s.l, op)
	if len(key) < len(s.prefix) || key[:len(s.prefix)] != s.prefix {
		runtime_fail("MissingCapability", op+": key outside the handle's prefix")
	}
	return key
}

// Get returns the value stored under key, and whether there is one.
func (s *SimStore) Get(key string) (string, bool, error) {
	v, ok := runtime_storeGet(s.key("SimStore.Get", key))
	return v, ok, nil
}

// Put stores value under key. A read-only handle does not permit it.
func (s *SimStore) Put(key, value string) error {
	k := s.key("SimStore.Put", key)
	if s.readOnly {
		runtime_fail("MissingCapability", "SimStore.Put on a read-only handle")
	}
	runtime_storePut(k, value)
	return nil
}

// ReadOnly returns a handle that permits only Get.
func (s *SimStore) ReadOnly() *SimStore {
	return &SimStore{l: s.l, readOnly: true, prefix: s.prefix}
}

// Prefix returns a handle that permits only keys beginning with the handle's prefix followed by p.
func (s *SimStore) Prefix(p string) *SimStore {
	if p == "" {
		runtime_fail("InvalidArgument", "SimStore.Prefix requires a non-empty prefix")
	}
	return &SimStore{l: s.l, readOnly: s.readOnly, prefix: s.prefix + p}
}

// LendTo returns a copy of s that is revoked when n closes.
func (s *SimStore) LendTo(n *Nursery) *SimStore {
	return &SimStore{l: s.l.to(n, "SimStore.LendTo"), readOnly: s.readOnly, prefix: s.prefix}
}
