// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// I/O handles of the S-Go adapters (W09, D54–D56; W21, D148–D153; reference: sgo/model/iohandle.go,
// sgo/model/scoperes.go).
//
// A handle is a resource of a scope (sgo_res.go): usable by every task inside the scope, released
// when the scope ends, or earlier by Close from any task inside it while no other task is in an
// operation on it (ResourceBusy otherwise). Every open makes a new handle record, never reused:
// once closed, a handle refers to no descriptor, so no operation on it can reach a descriptor the
// system later reuses.
//
// File reads and writes are blocking system calls on the calling thread, bounded to 1 MiB per
// call (the processor is handed off as for any system call); cancellation is observed before
// each call and between the calls of one Write, never inside one (D55).

package runtime

import (
	"internal/runtime/atomic"
	"unsafe"
)

const (
	sgoHDir = iota + 1
	sgoHFile
	sgoHConn
	sgoHListener
	sgoHDialer // a delegated dialer (sgo_netdial.go, D143)
	_          // formerly the connection slot (withdrawn, D158)

	sgoIOChunk = 1 << 20
)

type sgoHandle struct {
	kind     uint8
	fd       int32
	pd       *pollDesc    // sockets: the poller registration (sgo_net.go)
	fake     *sgoFakeFile // lab: a fake file (sgo_iofake.go)
	conn     *sgoFakeConn // lab: a fake connection or listener (sgo_netfake.go)
	dial     *sgoDialer   // a dialer: its endpoints (sgo_netdial.go)
	tmr      *sgoChan     // a scope timer: its channel (sgo_timer.go, D171)
	unix     bool         // a Unix domain socket (D60)
	sockPath *byte        // a Unix listener: its socket file, removed at close (sgo_net.go)
	sockID   sgoFileID
	readOnly bool // a directory opened from an ro grant, or ReadOnly
	write    bool // a file opened for writing
	// The scope it is a resource of (sgo_res.go): a nursery, or the body of task body; and the list
	// it is in (nil once released or closed).
	nurs    *sgoScope
	body    *sgoTask
	list    *sgoResList
	release func(ok bool) error // a user resource (sgoHUser): its Release method (D152)
	key     unsafe.Pointer      // a user resource: the registered value, found by sgo.MoveTo (D158)
	closed  bool
	ops     atomic.Int32 // operations in progress
	// Sockets: whether a task is waiting in the poller to read (Read, Accept) or to write. The
	// poller keeps one waiter per descriptor and direction, so other tasks wait for the turn.
	awaitR, awaitW atomic.Uint32
	// Connections and files: the read turn and the bytes read but not yet returned (D173,
	// sgo_readbuf.go). Only the holder of the turn touches rbuf.
	rturn atomic.Uint32
	rbuf  string
	name           string     // for messages: the path or address
	prev           *sgoHandle // the scope's resources (sgoResList)
	next           *sgoHandle
}

// sgoIOErr is the result of a failed operation: its kind and the bytes transferred before it.
type sgoIOErr struct {
	kind     string
	progress int
}

// sgoHandleNew creates a handle in the current task's body list; an API entry point that names
// another scope moves it there (sgoResAdopt).
func sgoHandleNew(kind uint8, fd int32, name string) *sgoHandle {
	t := sgoCurrent()
	h := &sgoHandle{kind: kind, fd: fd, body: t, name: name}
	t.res.push(h)
	return h
}

// sgoHandleUser reports whether task t may operate on h: it runs inside h's scope (D150).
func sgoHandleUser(h *sgoHandle, t *sgoTask) bool { return sgoInside(t, h.nurs, h.body) }

// sgoHandleBegin starts an operation of the current task on h; nil if it may proceed.
func sgoHandleBegin(h *sgoHandle, op string) *sgoIOErr {
	t := sgoCurrent()
	if !sgoHandleUser(h, t) {
		panic(error(&sgoFailure{"MissingCapability", op + ": the task does not run inside the handle's scope", ""}))
	}
	if h.closed {
		return &sgoIOErr{"HandleClosed", 0}
	}
	if err := sgoTaskCancellation(t); err != nil {
		return &sgoIOErr{"Cancelled", 0}
	}
	h.ops.Add(1)
	return nil
}

func sgoHandleEnd(h *sgoHandle) { h.ops.Add(-1) }

// sgoHandleClose releases h early (SGO-RES-002): a task inside its scope, while no other task is
// in an operation on it; the release's error goes to the caller.
func sgoHandleClose(h *sgoHandle) *sgoIOErr {
	t := sgoCurrent()
	if !sgoHandleUser(h, t) {
		panic(error(&sgoFailure{"MissingCapability", "Close: the task does not run inside the handle's scope", ""}))
	}
	if h.closed {
		return &sgoIOErr{"HandleClosed", 0}
	}
	if h.ops.Load() != 0 {
		return &sgoIOErr{"ResourceBusy", 0}
	}
	return sgoHandleFree(h)
}

// sgoHandleFree closes h and removes it from its scope; the result is a failed close (IOError).
func sgoHandleFree(h *sgoHandle) *sgoIOErr {
	h.closed = true
	if h.tmr != nil {
		sgoTimerDisarm(h.tmr)
	}
	if h.sockPath != nil {
		sgoNetUnlinkSocket(h)
	}
	if h.pd != nil {
		poll_runtime_pollUnblock(h.pd)
		poll_runtime_pollClose(h.pd)
		h.pd = nil
	}
	if h.conn != nil {
		sgoFakeConnClose(h)
	}
	var e *sgoIOErr
	if h.fd >= 0 {
		if sgoCloseFD(h.fd) < 0 {
			e = &sgoIOErr{"IOError", 0}
		}
	}
	h.fd = -1
	sgoResUnlink(h)
	return e
}

// sgoSysRead and sgoSysWrite are blocking system calls with the processor handed off.
//
//go:nosplit
func sgoSysRead(fd int32, p unsafe.Pointer, n int32) int32 {
	entersyscall()
	r := read(fd, p, n)
	exitsyscall()
	return r
}

//go:nosplit
func sgoSysWrite(fd int32, p unsafe.Pointer, n int32) int32 {
	entersyscall()
	r := write1(uintptr(fd), p, n)
	exitsyscall()
	return r
}

// sgoFileRead reads at most max bytes from file h (one bounded call): at least one byte, or an
// error with no progress (EOF at end of file).
func sgoFileRead(h *sgoHandle, max int) (string, *sgoIOErr) {
	if e := sgoHandleBegin(h, "File.Read"); e != nil {
		return "", e
	}
	defer sgoHandleEnd(h)
	if max <= 0 {
		return "", nil
	}
	if h.fake != nil {
		return sgoFakeRead(h, max)
	}
	buf := make([]byte, min(max, sgoIOChunk))
	for {
		n := sgoSysRead(h.fd, unsafe.Pointer(&buf[0]), int32(len(buf)))
		switch {
		case n == -sgoEINTR:
			continue
		case n < 0:
			return "", &sgoIOErr{"IOError", 0}
		case n == 0:
			return "", &sgoIOErr{"EOF", 0}
		}
		return string(buf[:n]), nil
	}
}

// sgoFileWrite writes all of s to file h, in calls of at most 1 MiB; cancellation between calls
// ends it with the progress so far (D55, D56).
func sgoFileWrite(h *sgoHandle, s string) (int, *sgoIOErr) {
	if e := sgoHandleBegin(h, "File.Write"); e != nil {
		return 0, e
	}
	defer sgoHandleEnd(h)
	if !h.write {
		return 0, &sgoIOErr{"ReadOnly", 0}
	}
	if h.fake != nil {
		return sgoFakeWrite(h, s)
	}
	done := 0
	for done < len(s) {
		if done > 0 && sgoTaskCancellation(sgoCurrent()) != nil {
			return done, &sgoIOErr{"Cancelled", done}
		}
		chunk := min(len(s)-done, sgoIOChunk)
		n := sgoSysWrite(h.fd, add(unsafe.Pointer(unsafe.StringData(s)), uintptr(done)), int32(chunk))
		switch {
		case n == -sgoEINTR:
			continue
		case n <= 0:
			return done, &sgoIOErr{"IOError", done}
		}
		done += int(n)
	}
	return done, nil
}

// sgoFileSize is the size of file h in bytes.
func sgoFileSize(h *sgoHandle) (int, *sgoIOErr) {
	if e := sgoHandleBegin(h, "File.Size"); e != nil {
		return 0, e
	}
	defer sgoHandleEnd(h)
	if h.fake != nil {
		sgoLabPoint()
		return len(h.fake.node.data), nil
	}
	var st sgoStatBuf
	if sgoFstat(h.fd, &st) < 0 {
		return 0, &sgoIOErr{"IOError", 0}
	}
	return int(sgoStatSize(&st)), nil
}

// sgoDirOpenFile opens path in directory h: for reading, or (write) created or truncated. Only
// regular files are opened (W12 R4): the last component is opened with O_NONBLOCK, so a FIFO or
// device cannot block the open, and anything that is not a regular file is closed again and
// reported as NotRegularFile with no effect. Create truncates only after the resolution has been
// verified (W12 R6): a file reached through a raced path is never truncated.
func sgoDirOpenFile(h *sgoHandle, path string, write bool) (*sgoHandle, *sgoIOErr) {
	op := "Dir.Open"
	if write {
		op = "Dir.Create"
	}
	if e := sgoHandleBegin(h, op); e != nil {
		return nil, e
	}
	defer sgoHandleEnd(h)
	flags := int32(sgoO_RDONLY | sgoO_NONBLOCK)
	if write {
		if h.readOnly {
			return nil, &sgoIOErr{"ReadOnly", 0}
		}
		flags = sgoO_WRONLY | sgoO_CREAT | sgoO_NONBLOCK
	}
	fd, kind := sgoResolve(h.fd, path, flags, 0o644)
	if kind != "" {
		return nil, &sgoIOErr{kind, 0}
	}
	var st sgoStatBuf
	if sgoFstat(fd, &st) < 0 {
		sgoCloseFD(fd)
		return nil, &sgoIOErr{"IOError", 0}
	}
	switch sgoStatMode(&st) & 0xF000 {
	case 0x8000: // regular file
	case 0x4000:
		sgoCloseFD(fd)
		return nil, &sgoIOErr{"IsDirectory", 0}
	default:
		sgoCloseFD(fd)
		return nil, &sgoIOErr{"NotRegularFile", 0}
	}
	if write && sgoFtruncate(fd) < 0 {
		sgoCloseFD(fd)
		return nil, &sgoIOErr{"IOError", 0}
	}
	sgoClearNonblock(fd)
	f := sgoHandleNew(sgoHFile, fd, path)
	f.write = write
	return f, nil
}
