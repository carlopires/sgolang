// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !darwin

package runtime

// Off darwin a note may be woken from a signal handler (futex), as sigsend does.
func sgoInterruptNoteSetup() {}

//go:nosplit
//go:nowritebarrierrec
func sgoInterruptNoteWake() { notewakeup(&sgoInterruptNote) }

//go:nowritebarrierrec
func sgoInterruptNoteSleep() { notesleep(&sgoInterruptNote) }
