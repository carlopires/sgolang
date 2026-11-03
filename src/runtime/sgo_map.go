// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

// S-Go maps with insertion order (SGO-COMP-006, D88). Compiled S-Go code reaches maps only through
// the sgomap* entry points below (cmd/compile/internal/walk/sgo_map.go); Go's map implementation,
// with its randomized iteration order, is never used for an S-Go map.
//
// A map is an insertion-ordered array of slots (key, element, sequence number) with an
// open-addressing hash index over it. A new key is appended; an overwrite keeps its slot; a delete
// kills the slot (sequence 0). Iteration visits live slots in increasing sequence order, so the
// order is a function of the program's operations only: the hash seed affects where keys land in
// the index, never the order.

import (
	"internal/abi"
	"unsafe"
)

type sgoMap struct {
	used    uint64         // live entries; must be first (the compiler's len reads it)
	n       int            // slots in use, live or dead
	cap     int            // slots allocated
	keys    unsafe.Pointer // [cap]K
	elems   unsafe.Pointer // [cap]V
	seqs    []uint64       // per slot: insertion sequence number, 0 for a dead slot
	hashes  []uintptr      // per slot: hash of the key
	index   []int32        // hash index: 0 empty, -1 deleted, else slot+1; len is a power of 2
	tomb    int            // deleted index entries
	nextSeq uint64         // next insertion sequence number (starts at 1, never reused)
	epoch   uint32         // incremented whenever slots are renumbered (compaction, clear)
	seed    uintptr
}

// sgoMapIter overlays the iterator the compiler allocates for a range over a map (the layout of
// internal/runtime/maps.Iter: key and elem first, written nil at the end).
type sgoMapIter struct {
	key   unsafe.Pointer
	elem  unsafe.Pointer
	t     *abi.MapType
	m     *sgoMap
	slot  int
	seq   uint64 // sequence number of the last visited entry
	epoch uint32
}

func (m *sgoMap) key(t *abi.MapType, i int) unsafe.Pointer {
	return add(m.keys, uintptr(i)*t.Key.Size_)
}

func (m *sgoMap) elem(t *abi.MapType, i int) unsafe.Pointer {
	return add(m.elems, uintptr(i)*t.Elem.Size_)
}

func sgomakemap(t *abi.MapType, hint int) *sgoMap {
	if hint < 0 {
		sgoInvalidArgument("negative map size hint")
	}
	m := new(sgoMap)
	m.seed = uintptr(rand())
	m.nextSeq = 1
	if hint > 0 {
		m.grow(t, hint)
	}
	return m
}

// find returns the slot holding key, or -1, and the key's hash.
func (m *sgoMap) find(t *abi.MapType, key unsafe.Pointer) (int, uintptr) {
	if m == nil {
		return -1, 0
	}
	h := t.Hasher(key, m.seed)
	if len(m.index) == 0 {
		return -1, h
	}
	mask := uintptr(len(m.index) - 1)
	for i := h & mask; ; i = (i + 1) & mask {
		e := m.index[i]
		if e == 0 {
			return -1, h
		}
		if e > 0 {
			s := int(e - 1)
			if m.hashes[s] == h && t.Key.Equal(key, m.key(t, s)) {
				return s, h
			}
		}
	}
}

// reindex rebuilds the hash index over the live slots, sized for want live entries.
func (m *sgoMap) reindex(want int) {
	size := 8
	for size < 2*want+2 {
		size <<= 1
	}
	m.index = make([]int32, size)
	m.tomb = 0
	mask := uintptr(size - 1)
	for s := 0; s < m.n; s++ {
		if m.seqs[s] == 0 {
			continue
		}
		i := m.hashes[s] & mask
		for m.index[i] != 0 {
			i = (i + 1) & mask
		}
		m.index[i] = int32(s + 1)
	}
}

// grow makes room for at least one more slot (at least want slots in total): it compacts the dead
// slots away when they are at least half, and otherwise doubles the capacity keeping slot numbers.
func (m *sgoMap) grow(t *abi.MapType, want int) {
	live := int(m.used)
	compact := m.n > 0 && m.n-live >= m.n/2
	ncap := m.cap
	if !compact {
		ncap = max(2*m.cap, want, 8)
	}
	if ncap > maxAlloc/int(max(t.Key.Size_, t.Elem.Size_, 1)) {
		sgoExhausted("map too large")
	}
	keys := newarray(t.Key, ncap)
	elems := newarray(t.Elem, ncap)
	seqs := make([]uint64, ncap)
	hashes := make([]uintptr, ncap)
	j := 0
	for s := 0; s < m.n; s++ {
		if compact && m.seqs[s] == 0 {
			continue
		}
		typedmemmove(t.Key, add(keys, uintptr(j)*t.Key.Size_), m.key(t, s))
		typedmemmove(t.Elem, add(elems, uintptr(j)*t.Elem.Size_), m.elem(t, s))
		seqs[j], hashes[j] = m.seqs[s], m.hashes[s]
		j++
	}
	m.keys, m.elems, m.seqs, m.hashes, m.cap, m.n = keys, elems, seqs, hashes, ncap, j
	if compact {
		m.epoch++
	}
	m.reindex(max(live+1, want))
}

func sgomapaccess1(t *abi.MapType, m *sgoMap, key unsafe.Pointer) unsafe.Pointer {
	if s, _ := m.find(t, key); s >= 0 {
		return m.elem(t, s)
	}
	return unsafe.Pointer(&zeroVal[0])
}

func sgomapaccess1_fat(t *abi.MapType, m *sgoMap, key, zero unsafe.Pointer) unsafe.Pointer {
	if s, _ := m.find(t, key); s >= 0 {
		return m.elem(t, s)
	}
	return zero
}

func sgomapaccess2(t *abi.MapType, m *sgoMap, key unsafe.Pointer) (unsafe.Pointer, bool) {
	if s, _ := m.find(t, key); s >= 0 {
		return m.elem(t, s), true
	}
	return unsafe.Pointer(&zeroVal[0]), false
}

func sgomapaccess2_fat(t *abi.MapType, m *sgoMap, key, zero unsafe.Pointer) (unsafe.Pointer, bool) {
	if s, _ := m.find(t, key); s >= 0 {
		return m.elem(t, s), true
	}
	return zero, false
}

// sgomapassign returns the element slot for key, appending a new zeroed entry if key is absent.
// pos is the source position of the assignment, for NilMapWrite (SGO-COMP-008).
func sgomapassign(t *abi.MapType, m *sgoMap, key unsafe.Pointer, pos string) unsafe.Pointer {
	if m == nil {
		panic(error(&sgoFailure{"NilMapWrite", "assignment to an entry of a nil map", pos}))
	}
	s, h := m.find(t, key)
	if s >= 0 {
		if t.NeedKeyUpdate() {
			typedmemmove(t.Key, m.key(t, s), key)
		}
		return m.elem(t, s)
	}
	if m.n == m.cap {
		m.grow(t, m.n+1)
	} else if 2*(int(m.used)+m.tomb+1) >= len(m.index) {
		m.reindex(int(m.used) + 1)
	}
	s = m.n
	m.n++
	typedmemmove(t.Key, m.key(t, s), key)
	m.seqs[s], m.hashes[s] = m.nextSeq, h
	m.nextSeq++
	m.used++
	mask := uintptr(len(m.index) - 1)
	i := h & mask
	for m.index[i] > 0 {
		i = (i + 1) & mask
	}
	if m.index[i] < 0 {
		m.tomb--
	}
	m.index[i] = int32(s + 1)
	return m.elem(t, s)
}

func sgomapdelete(t *abi.MapType, m *sgoMap, key unsafe.Pointer) {
	s, h := m.find(t, key)
	if s < 0 {
		return
	}
	mask := uintptr(len(m.index) - 1)
	for i := h & mask; ; i = (i + 1) & mask {
		if m.index[i] == int32(s+1) {
			m.index[i] = -1
			m.tomb++
			break
		}
	}
	typedmemclr(t.Key, m.key(t, s))
	typedmemclr(t.Elem, m.elem(t, s))
	m.seqs[s] = 0
	m.used--
}

func sgomapclear(t *abi.MapType, m *sgoMap) {
	if m == nil || m.n == 0 {
		return
	}
	memclrHasPointers(m.keys, uintptr(m.n)*t.Key.Size_)
	memclrHasPointers(m.elems, uintptr(m.n)*t.Elem.Size_)
	clear(m.seqs[:m.n])
	clear(m.index)
	m.n, m.used, m.tomb = 0, 0, 0
	m.epoch++
}

func sgomapIterStart(t *abi.MapType, m *sgoMap, it *sgoMapIter) {
	*it = sgoMapIter{t: t, m: m}
	if m != nil {
		it.epoch = m.epoch
	}
	sgomapIterNext(it)
}

// sgomapIterNext advances to the live entry with the smallest sequence number greater than the
// last one visited (SGO-COMP-006): deleted entries are skipped, entries inserted during the range
// are visited, and renumbering by compaction or clear is followed through the sequence numbers.
func sgomapIterNext(it *sgoMapIter) {
	m := it.m
	if m == nil {
		it.key, it.elem = nil, nil
		return
	}
	if it.epoch != m.epoch {
		it.epoch, it.slot = m.epoch, 0
	}
	for ; it.slot < m.n; it.slot++ {
		if q := m.seqs[it.slot]; q != 0 && q > it.seq {
			it.seq = q
			it.key, it.elem = m.key(it.t, it.slot), m.elem(it.t, it.slot)
			it.slot++
			return
		}
	}
	it.key, it.elem = nil, nil
}

// sgoInvalidMake is make([]T, n, c) with n < 0 or c < n in S-Go code (SGO-COMP-005).
func sgoInvalidMake(pos string) {
	panic(error(&sgoFailure{"InvalidArgument", "make of a slice with a negative length or a capacity below the length", pos}))
}

// sgoInvalidArgument is the checked failure InvalidArgument (SGO-COMP-005, SGO-COMP-008).
func sgoInvalidArgument(msg string) {
	panic(error(&sgoFailure{"InvalidArgument", msg, ""}))
}
