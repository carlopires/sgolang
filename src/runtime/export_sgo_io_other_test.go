// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !(linux && (amd64 || arm64))

package runtime

// SGoForceWalk has nothing to switch: without openat2 resolution always walks (W12 R6).
func SGoForceWalk(on bool) {}

func SGoOpenat2() (on bool, calls int64) { return false, 0 }
func SGoOpenat2FaultOnce(errno int32)    {}
