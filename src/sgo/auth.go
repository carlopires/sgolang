// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgo

import "unsafe"

// Task ceilings (SGO-AUTH-001). The runtime holds each task's ceiling (src/runtime/sgo_auth.go).

func runtime_spawnWithout(scope unsafe.Pointer, kind string, fn func(task unsafe.Pointer) error) error

// SpawnWithout is Spawn with the effect kind removed from the child's ceiling: the child, and
// every task it spawns, cannot exercise that kind, whatever handles it holds. kind is one of
// "clock", "wallclock", "random", "entropy", "simstore"; any other string is InvalidArgument.
func (n *Nursery) SpawnWithout(kind string, f func(*Context) error) {
	if f == nil {
		runtime_violation("spawn with a nil callback")
	}
	if err := runtime_spawnWithout(n.s, kind, func(t unsafe.Pointer) error { return f(&Context{t: t}) }); err != nil {
		panic(err)
	}
}
