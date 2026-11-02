// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && (amd64 || arm64)

package runtime

// SGoForceWalk makes resolution skip openat2 (as on a kernel without it) while on is true, so a
// test can run the same cases through both paths (W12 R6).
func SGoForceWalk(on bool) { sgoNoOpenat2.Store(on) }
