// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgo

// runtime_copy returns a deep copy of v's value (src/runtime/sgo_copy.go).
func runtime_copy(v any) any

// Copy returns a deep copy of x: every slice, map, and pointed-to variable reachable from x is
// copied, references to one object stay references to one copied object, and cycles are kept;
// strings and channels are shared (SGO-COMP-004). A value of a task-local type crosses to another
// task only as the result of Copy.
func Copy[T any](x T) T {
	r := runtime_copy(x)
	if r == nil {
		var zero T // x is a nil interface value: Copy(nil) is nil (SGO-COMP-004, D132)
		return zero
	}
	return r.(T)
}
