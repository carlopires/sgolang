// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Effect authority: task ceilings and root grants (W05, SGO-AUTH-001/002 drafts, D24, D25).
//
// Every task has a ceiling, the set of effect kinds it may exercise. The root task's ceiling is
// the kinds granted in the main module's sgo.mod, which the driver passes to the linker
// (-sgogrants) and the linker records in sgoGrants. A child inherits its spawner's ceiling, minus
// the kinds a SpawnWithout removes; nothing widens a ceiling. Effect operations check the calling
// task's ceiling before they act (sgo_effects.go).

package runtime

import "unsafe"

// Effect kinds of subset 1 (SGO-AUTH-003), as ceiling bits.
const (
	sgoKindClock uint8 = 1 << iota
	sgoKindWallClock
	sgoKindRandom
	sgoKindEntropy
	sgoKindSimStore
	sgoKindAll = sgoKindClock | sgoKindWallClock | sgoKindRandom | sgoKindEntropy | sgoKindSimStore
)

var sgoKindNames = [...]string{"clock", "wallclock", "random", "entropy", "simstore"}

// sgoKindOf returns the ceiling bit of a kind name, or 0 for a name that is not a kind.
func sgoKindOf(name string) uint8 {
	for i, n := range sgoKindNames {
		if n == name {
			return 1 << i
		}
	}
	return 0
}

// sgoGrants is the comma-separated list of granted kinds, set by the linker from the driver's
// -sgogrants (the main module's grant lines); empty means none. No environment variable or
// run-time setting changes it. sgoRootCeiling is its value, parsed when the program starts.
var (
	sgoGrants      string
	sgoRootCeiling uint8
)

// sgoGrantsInit parses sgoGrants. The linker accepts only known kinds, so anything else is a
// RuntimeDefect.
func sgoGrantsInit() {
	var c uint8
	for s := sgoGrants; s != ""; {
		name := s
		for i := 0; i < len(s); i++ {
			if s[i] == ',' {
				name, s = s[:i], s[i+1:]
				goto have
			}
		}
		s = ""
	have:
		k := sgoKindOf(name)
		if k == 0 {
			throw("sgo: corrupt grant set")
		}
		c |= k
	}
	sgoRootCeiling = c
}

// sgoRequire fails the calling task with MissingCapability unless its ceiling contains kind k
// (SGO-AUTH-001). It runs before any effect.
func sgoRequire(k uint8, op string) {
	t := sgoCurrent()
	if t == nil || t.ceiling&k == 0 {
		panic(error(&sgoFailure{"MissingCapability", op + ": the task's ceiling does not include " + sgoKindName(k), ""}))
	}
}

// Package sgo entry points (src/sgo/auth.go).

// sgo_spawnWithout is n.SpawnWithout(kind, f): Spawn with kind removed from the child's ceiling.
//
//go:linkname sgo_spawnWithout sgo.runtime_spawnWithout
func sgo_spawnWithout(scope unsafe.Pointer, kind string, fn func(task unsafe.Pointer) error) error {
	k := sgoKindOf(kind)
	if k == 0 {
		panic(error(&sgoFailure{"InvalidArgument", "SpawnWithout with " + quoteKind(kind) + ", which is not an effect kind (clock, wallclock, random, entropy, simstore)", ""}))
	}
	_, err := sgoAdmit((*sgoScope)(scope), *(*func(*sgoTask) error)(unsafe.Pointer(&fn)), nil, k)
	return err
}

func quoteKind(s string) string {
	if len(s) > 32 {
		s = s[:32] + "..."
	}
	return "\"" + s + "\""
}

func sgoKindName(k uint8) string {
	for i := range sgoKindNames {
		if k == 1<<i {
			return sgoKindNames[i]
		}
	}
	return "an unknown kind"
}
