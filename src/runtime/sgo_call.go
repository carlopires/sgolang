// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Request/reply calls (D167, D174; sgo.NewCall, src/sgo/call.go; model sgo/model/callmodel.go).
//
// A call point is a rendezvous native channel of requests: Call sends, Accept receives, so a
// caller cancelled before a server accepted withdraws its request and no server sees it. Each
// request carries a task-result record for its reply (sgo_taskres.go), and an accepted request is
// a resource of the accepting task's body (sgo_res.go) whose release answers NoReply if the body
// ends unanswered. Both waits are native channel waits: a cancelled waiter unwinds.

package runtime

import "unsafe"

//go:linkname sgo_callNew sgo.runtime_callNew
func sgo_callNew(chanType unsafe.Pointer) unsafe.Pointer {
	return unsafe.Pointer(sgomakechan((*chantype)(chanType), 0))
}

// sgoCallInside fails the current task unless it runs inside the call point's nursery.
func sgoCallInside(scope unsafe.Pointer, op string) {
	if !sgoInside(sgoCurrent(), (*sgoScope)(scope), nil) {
		panic(error(&sgoFailure{"MissingCapability", op + ": the task does not run inside the call point's nursery", ""}))
	}
}

//go:linkname sgo_callSend sgo.runtime_callSend
func sgo_callSend(scope, c, v unsafe.Pointer) {
	sgoCallInside(scope, "Call")
	sgochansend1((*sgoChan)(c), v)
}

//go:linkname sgo_callRecv sgo.runtime_callRecv
func sgo_callRecv(scope, c, dst unsafe.Pointer) {
	sgoCallInside(scope, "Accept")
	sgochanrecv1((*sgoChan)(c), dst)
}

//go:linkname sgo_noReply sgo.runtime_noReply
func sgo_noReply() error {
	return &sgoFailure{"NoReply", "Call: the server's handler ended without Reply", ""}
}

//go:linkname sgo_isNoReply sgo.runtime_isNoReply
func sgo_isNoReply(err error) bool {
	f, ok := any(err).(*sgoFailure)
	return ok && f.kind == "NoReply"
}
