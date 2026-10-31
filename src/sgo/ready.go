// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgo

import "unsafe"

// Start with a readiness handshake (SGO-READY-002). The protocol lives in the runtime
// (src/runtime/sgo_ready.go).

func runtime_start(scope unsafe.Pointer, fn func(task unsafe.Pointer) error) error
func runtime_readySignal(ready unsafe.Pointer)
func runtime_taskReady(task unsafe.Pointer) unsafe.Pointer

// Ready is the readiness capability of a child started by Nursery.Start; only that child may
// signal it, once.
type Ready struct{ r unsafe.Pointer }

// Start admits f as a child of n, as Spawn does, and waits until the child signals readiness
// (nil), the child ends first (its error, or an error naming the child), or a scope linked to the
// caller is cancelled (a cancellation error). In every case the child is a registered child of n.
func (n *Nursery) Start(f func(*Context, *Ready) error) error {
	if f == nil {
		runtime_violation("start with a nil callback")
	}
	return runtime_start(n.s, func(t unsafe.Pointer) error {
		return f(&Context{t: t}, &Ready{runtime_taskReady(t)})
	})
}

// Signal reports that the started child is ready. A second Signal, or a Signal by another task,
// is a SpawnContractViolation.
func (r *Ready) Signal() {
	if r == nil {
		runtime_violation("ready signal without a readiness handle")
	}
	runtime_readySignal(r.r)
}
