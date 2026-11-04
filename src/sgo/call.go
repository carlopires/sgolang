// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgo

import "unsafe"

// Request/reply calls (D167, D174; draft SGO-SPAWN-008).

// Implemented in the runtime (sgo_call.go).
func runtime_callNew(chanType unsafe.Pointer) unsafe.Pointer
func runtime_callSend(scope, c, v unsafe.Pointer)
func runtime_callRecv(scope, c, dst unsafe.Pointer)
func runtime_noReply() error
func runtime_isNoReply(err error) bool

// Call is a call point served inside a nursery: callers hand it requests, and a server accepts
// them and replies to each (D174). The handle may be shared by every task inside the nursery's
// scope; a task outside it fails with MissingCapability.
type Call[Req, Resp any] struct {
	s unsafe.Pointer // the nursery's scope
	c unsafe.Pointer // the rendezvous channel of *Request[Req, Resp]
}

// Request is a request a server accepted: a resource of the accepting task's body. Its caller
// waits until Reply answers it; if the body ends first, the caller's Call returns NoReply.
type Request[Req, Resp any] struct {
	value   Req
	res     unsafe.Pointer // the reply record (sgo_taskres.go)
	replied bool
}

// chanTypeOf returns the runtime type of chan E.
func chanTypeOf[E any]() unsafe.Pointer {
	var e any = (chan E)(nil)
	return (*[2]unsafe.Pointer)(unsafe.Pointer(&e))[0]
}

// NewCall returns a call point served inside nursery n.
func NewCall[Req, Resp any](n *Nursery) *Call[Req, Resp] {
	if n == nil {
		runtime_violation("NewCall with a nil nursery handle")
	}
	return &Call[Req, Resp]{s: n.s, c: runtime_callNew(chanTypeOf[*Request[Req, Resp]]())}
}

// Call hands req to a server and waits for its reply. Both waits are cancellation points: a
// caller cancelled before a server accepted the request withdraws it (no server sees it); one
// cancelled later unwinds, and the reply, when it comes, is dropped. If the server's handler ends
// without replying, Call returns the error NoReply (sgo.IsNoReply).
func (c *Call[Req, Resp]) Call(t *Context, req Req) (Resp, error) {
	t.own()
	r := &Request[Req, Resp]{value: req, res: runtime_taskResNew(resultType[Resp]())}
	runtime_callSend(c.s, c.c, unsafe.Pointer(&r))
	var res Result[Resp]
	runtime_nativeRecv(runtime_taskResChan(r.res), unsafe.Pointer(&res))
	return res.Value, res.Err
}

// Accept waits for a request and returns it, a resource of the calling task's body: answer it
// with Reply before the body ends, or its caller gets NoReply. The wait is a cancellation point.
func (c *Call[Req, Resp]) Accept(t *Context) *Request[Req, Resp] {
	t.own()
	var r *Request[Req, Resp]
	runtime_callRecv(c.s, c.c, unsafe.Pointer(&r))
	runtime_resOwn(t.t, nil, unsafe.Pointer(r), func(bool) error {
		if !r.replied {
			r.replied = true
			res := Result[Resp]{Err: runtime_noReply()}
			runtime_taskResSet(r.res, unsafe.Pointer(&res))
		}
		return nil
	})
	return r
}

// Value returns the request.
func (r *Request[Req, Resp]) Value() Req { return r.value }

// Reply answers the request with resp and err; the caller's Call returns them. A reply to a caller
// that was cancelled meanwhile is dropped. Replying twice is a SpawnContractViolation.
func (r *Request[Req, Resp]) Reply(resp Resp, err error) {
	if r.replied {
		runtime_violation("Reply to a request that was already answered")
	}
	r.replied = true
	res := Result[Resp]{Value: resp, Err: err}
	runtime_taskResSet(r.res, unsafe.Pointer(&res))
}

// IsNoReply reports whether err is a Call whose request the server's handler left unanswered.
func IsNoReply(err error) bool { return runtime_isNoReply(err) }
