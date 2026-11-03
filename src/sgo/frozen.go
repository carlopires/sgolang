// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgo

// Frozen is a deeply immutable value of type T that any number of tasks may read without copying
// and without synchronization (SGO-COMP-009). It is made only by Freeze; it has no zero value of
// its own, no nil, and no ==. Its contents are read only through access paths that start with
// Get: the compiler refuses every use that could write the frozen object or keep a reference into
// it (SGO_E_FROZEN_WRITE).
type Frozen[T any] struct{ p *T }

// Freeze returns a frozen deep copy of x (D98): later changes to x, or to anything x refers to,
// do not affect it. The copy is the one Copy makes (SGO-COMP-004).
func Freeze[T any](x T) Frozen[T] {
	c := runtime_copy(x).(T)
	return Frozen[T]{p: &c}
}

// Get returns the frozen value without copying what it refers to; it may only head an access path
// (a chain of field selections and index expressions, a len argument, or a range operand).
func (f Frozen[T]) Get() T {
	return *f.p
}
