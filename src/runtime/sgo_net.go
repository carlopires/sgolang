// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TCP handles of the S-Go network adapters (W09, D55–D57): non-blocking sockets on the runtime's
// network poller. A task waiting for readiness is registered in sgoNetWaiters under sgoSleepLock;
// cancellation of a linked scope (sgoWakeCancelled) marks the record and sets the poller deadline
// of the waited direction to the past, which ends the wait. The waiter then restores the deadline
// and returns Cancelled with the progress so far. Address checking against the grants is the
// sgo package's (D57); the runtime connects to and listens on the exact endpoints it is given.

package runtime

import (
	"internal/runtime/atomic"
	"unsafe"
)

// sgoNetWait is the wait record of one task waiting for readiness on a socket.
type sgoNetWait struct {
	task       *sgoTask
	pd         *pollDesc
	mode       int
	state      atomic.Uint32 // 0 waiting, 2 cancellation marked it, 3 its deadline is set
	prev, next *sgoNetWait   // in sgoNetWaiters, under sgoSleepLock
	wake       *sgoNetWait   // wake list of sgoWakeCancelled
}

var sgoNetWaiters *sgoNetWait

// sgoNetWakeCancelled marks the waiters whose task is now cancelled; the caller holds
// sgoSleepLock and calls sgoNetWakeAll on the result after releasing it.
func sgoNetWakeCancelled() (wake *sgoNetWait) {
	for r := sgoNetWaiters; r != nil; r = r.next {
		if sgoOuterCancelled(r.task) != nil && r.state.CompareAndSwap(0, 2) {
			r.wake, wake = wake, r
		}
	}
	return wake
}

func sgoNetWakeAll(wake *sgoNetWait) {
	for wake != nil {
		r := wake
		wake, r.wake = r.wake, nil
		poll_runtime_pollSetDeadline(r.pd, -1, r.mode)
		r.state.Store(3)
	}
}

// sgoNetAwait waits until h's socket is ready in mode ('r' or 'w'), or the task is cancelled.
func sgoNetAwait(h *sgoHandle, mode int) *sgoIOErr {
	t := sgoCurrent()
	r := &sgoNetWait{task: t, pd: h.pd, mode: mode}
	lock(&sgoSleepLock)
	if sgoOuterCancelled(t) != nil {
		unlock(&sgoSleepLock)
		return &sgoIOErr{"Cancelled", 0}
	}
	r.next = sgoNetWaiters
	if sgoNetWaiters != nil {
		sgoNetWaiters.prev = r
	}
	sgoNetWaiters = r
	unlock(&sgoSleepLock)
	res := poll_runtime_pollWait(h.pd, mode)
	lock(&sgoSleepLock)
	if r.prev != nil {
		r.prev.next = r.next
	} else {
		sgoNetWaiters = r.next
	}
	if r.next != nil {
		r.next.prev = r.prev
	}
	unlock(&sgoSleepLock)
	if r.state.Load() != 0 {
		// The canceller sets the deadline after releasing the lock: wait until it has, so that
		// the reset below is the last word and a later wait on h is not ended by it.
		for r.state.Load() != 3 {
			osyield()
		}
		poll_runtime_pollSetDeadline(h.pd, 0, mode)
		return &sgoIOErr{"Cancelled", 0}
	}
	switch res {
	case pollNoError:
		return nil
	case pollErrClosing:
		return &sgoIOErr{"HandleClosed", 0}
	}
	return &sgoIOErr{"IOError", 0}
}

// sgoNetErrKind is the S-Go error kind of a negated socket errno.
func sgoNetErrKind(r int32) string {
	switch -r {
	case sgoECONNREFUSED:
		return "ConnectionRefused"
	case sgoECONNRESET, sgoEPIPE:
		return "ConnectionReset"
	case sgoEADDRINUSE:
		return "AddressInUse"
	case sgoENETUNREACH, sgoEHOSTUNREACH:
		return "Unreachable"
	case sgoETIMEDOUT:
		return "TimedOut"
	}
	return "IOError"
}

// sgoNetHandle registers fd with the poller and makes it a handle of the calling task.
func sgoNetHandle(kind uint8, fd int32, name string) (*sgoHandle, *sgoIOErr) {
	netpollGenericInit()
	pd, errno := poll_runtime_pollOpen(uintptr(fd))
	if errno != 0 {
		sgoCloseFD(fd)
		return nil, &sgoIOErr{"IOError", 0}
	}
	h := sgoHandleNew(kind, fd, name)
	h.pd = pd
	return h, nil
}

// sgoNetConnect connects to ip:port (ip in 16-byte form; v6 selects IPv6).
func sgoNetConnect(ip [16]byte, v6 bool, port uint16, name string) (*sgoHandle, *sgoIOErr) {
	if err := sgoTaskCancellation(sgoCurrent()); err != nil {
		return nil, &sgoIOErr{"Cancelled", 0}
	}
	fd := sgoSocket(v6)
	if fd < 0 {
		return nil, &sgoIOErr{sgoNetErrKind(fd), 0}
	}
	h, e := sgoNetHandle(sgoHConn, fd, name)
	if e != nil {
		return nil, e
	}
	var sa [28]byte
	n := sgoSockaddr(&sa, &ip, v6, port)
	r := sgoConnectSys(fd, &sa, n)
	for r == -sgoEINTR {
		r = sgoConnectSys(fd, &sa, n)
	}
	if r == -sgoEINPROGRESS {
		h.ops.Add(1)
		e = sgoNetAwait(h, 'w')
		h.ops.Add(-1)
		if e == nil {
			v, gr := sgoGetsockoptInt(fd, sgoSOL_SOCKET, sgoSO_ERROR)
			r = -v
			if gr < 0 {
				r = gr
			}
		}
	}
	if e == nil && r < 0 {
		e = &sgoIOErr{sgoNetErrKind(r), 0}
	}
	if e != nil {
		sgoHandleFree(h)
		return nil, e
	}
	return h, nil
}

// sgoNetListen listens on ip:port; port 0 binds a free port (sgoNetPort reports it).
func sgoNetListen(ip [16]byte, v6 bool, port uint16, name string) (*sgoHandle, *sgoIOErr) {
	fd := sgoSocket(v6)
	if fd < 0 {
		return nil, &sgoIOErr{sgoNetErrKind(fd), 0}
	}
	sgoSetsockoptInt(fd, sgoSOL_SOCKET, sgoSO_REUSEADDR, 1)
	var sa [28]byte
	n := sgoSockaddr(&sa, &ip, v6, port)
	r := sgoBind(fd, &sa, n)
	if r == 0 {
		r = sgoListenSys(fd, 128)
	}
	if r < 0 {
		sgoCloseFD(fd)
		return nil, &sgoIOErr{sgoNetErrKind(r), 0}
	}
	return sgoNetHandle(sgoHListener, fd, name)
}

// sgoNetPort is the local port of a socket handle.
func sgoNetPort(h *sgoHandle) int {
	p, r := sgoLocalPort(h.fd)
	if r < 0 {
		return 0
	}
	return int(p)
}

// sgoNetAccept waits for and accepts one connection on listener h; the connection is a handle of
// the calling task.
func sgoNetAccept(h *sgoHandle) (*sgoHandle, *sgoIOErr) {
	if e := sgoHandleBegin(h, "Listener.Accept"); e != nil {
		return nil, e
	}
	defer sgoHandleEnd(h)
	if h.conn != nil {
		return sgoFakeAccept(h)
	}
	for {
		fd := sgoAccept(h.fd)
		switch {
		case fd >= 0:
			return sgoNetHandle(sgoHConn, fd, h.name)
		case fd == -sgoEINTR:
			continue
		case fd == -sgoEAGAIN:
			if e := sgoNetAwait(h, 'r'); e != nil {
				return nil, e
			}
			continue
		}
		return nil, &sgoIOErr{sgoNetErrKind(fd), 0}
	}
}

// sgoConnRead reads at most max bytes from connection h: at least one, or EOF when the peer has
// closed its side, or Cancelled while waiting.
func sgoConnRead(h *sgoHandle, max int) (string, *sgoIOErr) {
	if e := sgoHandleBegin(h, "Conn.Read"); e != nil {
		return "", e
	}
	defer sgoHandleEnd(h)
	if max <= 0 {
		return "", nil
	}
	if h.conn != nil {
		return sgoFakeConnRead(h, max)
	}
	buf := make([]byte, min(max, sgoIOChunk))
	for {
		n := read(h.fd, unsafe.Pointer(&buf[0]), int32(len(buf)))
		switch {
		case n > 0:
			return string(buf[:n]), nil
		case n == 0:
			return "", &sgoIOErr{"EOF", 0}
		case n == -sgoEINTR:
			continue
		case n == -sgoEAGAIN:
			if e := sgoNetAwait(h, 'r'); e != nil {
				return "", e
			}
			continue
		}
		return "", &sgoIOErr{sgoNetErrKind(n), 0}
	}
}

// sgoConnWrite writes all of s to connection h; cancellation while waiting for buffer space ends
// it with the progress so far (D55, D56).
func sgoConnWrite(h *sgoHandle, s string) (int, *sgoIOErr) {
	if e := sgoHandleBegin(h, "Conn.Write"); e != nil {
		return 0, e
	}
	defer sgoHandleEnd(h)
	if h.conn != nil {
		return sgoFakeConnWrite(h, s)
	}
	done := 0
	for done < len(s) {
		chunk := min(len(s)-done, sgoIOChunk)
		n := write1(uintptr(h.fd), add(unsafe.Pointer(unsafe.StringData(s)), uintptr(done)), int32(chunk))
		switch {
		case n > 0:
			done += int(n)
			continue
		case n == -sgoEINTR:
			continue
		case n == -sgoEAGAIN:
			if e := sgoNetAwait(h, 'w'); e != nil {
				e.progress = done
				return done, e
			}
			continue
		}
		return done, &sgoIOErr{sgoNetErrKind(n), done}
	}
	return done, nil
}
