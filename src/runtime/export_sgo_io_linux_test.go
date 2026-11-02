// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && (amd64 || arm64)

package runtime

// SGoForceWalk makes resolution skip openat2 (as on a kernel without it) while on is true, so a
// test can run the same cases through both paths (W12 R6).
func SGoForceWalk(on bool) {
	sgoOpenat2Probed.Store(true)
	sgoNoOpenat2.Store(on)
}

// SGoOpenat2 reports whether resolution uses openat2 and how many openat2 opens ran.
func SGoOpenat2() (on bool, calls int64) { return !sgoNoOpenat2.Load(), sgoOpenat2Calls.Load() }

// SGoOpenat2FaultOnce makes the next openat2 open return -errno (W12 R6b).
func SGoOpenat2FaultOnce(errno int32) { sgoOpenat2Fault.Store(errno) }
