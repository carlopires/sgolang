// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "unsafe"

// On darwin a note's semaphore may not be woken on the signal stack, so the interrupt thread waits
// on its own pipe, as sigsend does with sigNoteSetup (whose single pipe belongs to os/signal).
var sgoInterruptPipeRead, sgoInterruptPipeWrite int32

func sgoInterruptNoteSetup() {
	r, w, errno := pipe()
	if errno != 0 {
		throw("sgo: interrupt pipe failed")
	}
	closeonexec(r)
	closeonexec(w)
	setNonblock(w) // the handler never blocks
	sgoInterruptPipeRead, sgoInterruptPipeWrite = r, w
}

//go:nosplit
//go:nowritebarrierrec
func sgoInterruptNoteWake() {
	var b byte
	write(uintptr(sgoInterruptPipeWrite), unsafe.Pointer(&b), 1)
}

// sgoInterruptNoteSleep runs on the interrupt thread's g0, which has no P.
//
//go:nowritebarrierrec
func sgoInterruptNoteSleep() {
	for {
		var b byte
		if n := read(sgoInterruptPipeRead, unsafe.Pointer(&b), 1); n != -_EINTR {
			return
		}
	}
}
