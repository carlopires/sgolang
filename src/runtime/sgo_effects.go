// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// First effects of subset 1 (W05, SGO-AUTH-003 draft, D26): the runtime primitives behind the
// handles of package sgo (src/sgo/effects.go). Package sgo holds the handles (kind, attenuation;
// no revocation, D169); every operation calls sgo_require first, so the calling task's ceiling is checked
// before any effect, and the primitives below act only after that check passed.
//
// TCB entries: TCB-P13 monotonic and wall clock reads (nanotime, time_now); TCB-P14 OS
// cryptographic randomness (readRandom); TCB-P15 the per-program simulated store.

package runtime

import (
	"internal/runtime/atomic"
	"unsafe"
)

// sgo_require fails the calling task with MissingCapability unless its ceiling contains kind.
//
//go:linkname sgo_require sgo.runtime_require
func sgo_require(kind uint8, op string) {
	sgoRequire(kind, op)
}

// sgo_rootGrant checks that task is the root task and that kind is granted to the program: root
// handles exist only for granted kinds and only on the root context (SGO-AUTH-002).
//
//go:linkname sgo_rootGrant sgo.runtime_rootGrant
func sgo_rootGrant(task unsafe.Pointer, kind uint8, op string) {
	t := (*sgoTask)(task)
	switch {
	case t == nil || t.id != 0 || t.scope != nil:
		panic(error(&sgoFailure{"MissingCapability", op + " called on a context that is not the root context", ""}))
	case sgoProgram && sgoRootCeiling&kind == 0:
		panic(error(&sgoFailure{"MissingCapability", op + ": " + sgoKindName(kind) + " is not granted in sgo.mod (SGO-AUTH-002)", ""}))
	}
	sgoRequire(kind, op)
}

// sgo_currentTask returns the calling task (owner of a *sgo.Random).
//
//go:linkname sgo_currentTask sgo.runtime_currentTask
func sgo_currentTask() unsafe.Pointer {
	return unsafe.Pointer(sgoCurrent())
}

// sgo_claimOwner returns the owner of a *sgo.Random, making the calling task its owner if it has
// none (a generator returned by Split is unowned until its first use, D38): one atomic step, so
// exactly one task claims it.
//
//go:linkname sgo_claimOwner sgo.runtime_claimOwner
func sgo_claimOwner(p *unsafe.Pointer) unsafe.Pointer {
	if o := atomic.Loadp(unsafe.Pointer(p)); o != nil {
		return o
	}
	t := unsafe.Pointer(sgoCurrent())
	if atomic_casPointer(p, nil, t) {
		return t
	}
	return atomic.Loadp(unsafe.Pointer(p))
}

// sgo_sinceMS is Clock.Since: monotonic milliseconds since the program started.
//
//go:linkname sgo_sinceMS sgo.runtime_sinceMS
func sgo_sinceMS() int {
	if l := sgoLab; l != nil {
		sgoLabPoint()
		return int(l.now) // virtual monotonic time (D45)
	}
	return int((nanotime() - runtimeInitTime) / 1_000_000)
}

// sgo_wallMS is WallClock.Now: UTC milliseconds since 1970-01-01.
//
//go:linkname sgo_wallMS sgo.runtime_wallMS
func sgo_wallMS() int {
	if l := sgoLab; l != nil {
		sgoLabPoint()
		return int(l.epoch + l.now) // the manifest's epoch plus virtual time (D45)
	}
	sec, nsec, _ := time_now()
	return int(sec*1000 + int64(nsec)/1_000_000)
}

// sgo_entropy is Entropy.Bytes(k): k bytes from the OS cryptographic source, never from a seeded
// generator (A08). A short read ends the program as ResourceExhausted (SGO-ERR-003).
//
//go:linkname sgo_entropy sgo.runtime_entropy
func sgo_entropy(k int) string {
	if sgoLab != nil {
		sgoLabPoint()
		sgoLabUnsupported() // unpredictable by definition: no seeded substitute (D45)
	}
	b := make([]byte, k)
	if readRandom(b) != k {
		sgoExhausted("entropy: the OS cryptographic source failed")
	}
	return string(b)
}

// The simulated store: one in-memory string map per program (TCB-P15). It is copy-on-write: a
// published map is never modified, a Put builds the next map outside the lock and publishes it
// only if no other Put published first (so nothing allocates under a runtime lock), and a Get
// reads the current map. Put linearizes at publication.
var (
	sgoStoreLock mutex
	sgoStore     map[string]string
)

func sgoStoreCurrent() map[string]string {
	lock(&sgoStoreLock)
	m := sgoStore
	unlock(&sgoStoreLock)
	return m
}

//go:linkname sgo_storeGet sgo.runtime_storeGet
func sgo_storeGet(key string) (string, bool) {
	sgoLabPoint()
	v, ok := sgoStoreCurrent()[key]
	return v, ok
}

//go:linkname sgo_storePut sgo.runtime_storePut
func sgo_storePut(key, value string) {
	sgoLabPoint()
	for {
		old := sgoStoreCurrent()
		next := make(map[string]string, len(old)+1)
		for k, v := range old {
			next[k] = v
		}
		next[key] = value
		lock(&sgoStoreLock)
		if mapIdentical(sgoStore, old) {
			sgoStore = next
			unlock(&sgoStoreLock)
			return
		}
		unlock(&sgoStoreLock)
	}
}

// mapIdentical reports whether a and b are the same map value (or both nil).
func mapIdentical(a, b map[string]string) bool {
	return *(*unsafe.Pointer)(unsafe.Pointer(&a)) == *(*unsafe.Pointer)(unsafe.Pointer(&b))
}
