// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// I/O handles of the S-Go adapters (W09, D54–D56; reference: sgo/model/iohandle.go).
//
// A handle is task-affine: owned by the task that opened it, closed only by it. It may be lent to
// nurseries; a task inside a lent nursery that is still open may read and write it, and the owner
// cannot close it until those nurseries have closed (HandleLent). Every open makes a new handle
// record, never reused: once closed, a handle refers to no descriptor, so no operation on it can
// reach a descriptor the system later reuses. A handle left open is closed when the owner's
// nursery body exits or the owner ends, with a HandleAbandoned entry (like tokens, D39).
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

	sgoIOChunk = 1 << 20
)

type sgoHandle struct {
	kind     uint8
	fd       int32
	pd       *pollDesc    // sockets: the poller registration (sgo_net.go)
	fake     *sgoFakeFile // lab: a fake file (sgo_iofake.go)
	conn     *sgoFakeConn // lab: a fake connection or listener (sgo_netfake.go)
	unix     bool         // a Unix domain socket (D60)
	sockPath *byte        // a Unix listener: its socket file, removed at close (sgo_net.go)
	sockID   sgoFileID
	readOnly bool // a directory opened from an ro grant, or ReadOnly
	write    bool // a file opened for writing
	owner    *sgoTask
	scope    *sgoScope // the owner's innermost nursery at open (nil: the task body)
	closed   bool
	lentTo   []*sgoScope  // nurseries the handle is lent to (touched by the owner)
	ops      atomic.Int32 // operations in progress
	// Sockets: whether a task is waiting in the poller to read (Read, Accept) or to write. The
	// poller keeps one waiter per descriptor and direction, so other tasks wait for the turn.
	awaitR, awaitW atomic.Uint32
	name           string     // for messages: the path or address
	prev           *sgoHandle // the owner's open handles
	next           *sgoHandle
}

// sgoIOErr is the result of a failed operation: its kind and the bytes transferred before it.
type sgoIOErr struct {
	kind     string
	progress int
}

func sgoHandleNew(kind uint8, fd int32, name string) *sgoHandle {
	t := sgoCurrent()
	h := &sgoHandle{kind: kind, fd: fd, owner: t, scope: t.currentScope(), name: name}
	h.next = t.handles
	if t.handles != nil {
		t.handles.prev = h
	}
	t.handles = h
	return h
}

// sgoScopeOpen reports whether nursery s has not closed yet.
func sgoScopeOpen(s *sgoScope) bool {
	lock(&s.lock)
	open := s.state != sgoClosed
	unlock(&s.lock)
	return open
}

// sgoHandleLentOpen reports whether h is lent to a nursery that has not closed: at the exit of the
// body it was opened in it stays open for the borrowers, and is closed at the owner's end.
func sgoHandleLentOpen(h *sgoHandle) bool {
	for _, s := range h.lentTo {
		if sgoScopeOpen(s) {
			return true
		}
	}
	return false
}

// sgoHandleUser reports whether task t may operate on h: its owner, or a task inside a nursery h
// is lent to that has not closed.
func sgoHandleUser(h *sgoHandle, t *sgoTask) bool {
	if t == h.owner {
		return true
	}
	for _, s := range h.lentTo {
		if !sgoScopeOpen(s) {
			continue
		}
		for c := t.currentScope(); c != nil; c = c.parent {
			if c == s {
				return true
			}
		}
	}
	return false
}

// sgoHandleBegin starts an operation of the current task on h; nil if it may proceed.
func sgoHandleBegin(h *sgoHandle, op string) *sgoIOErr {
	t := sgoCurrent()
	if !sgoHandleUser(h, t) {
		panic(error(&sgoFailure{"MissingCapability", op + ": the handle is not the task's and is not lent to its nursery", ""}))
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

// sgoHandleLend records that h is lent to nursery s (the sgo package checks the caller may spawn
// into s).
func sgoHandleLend(h *sgoHandle, s *sgoScope) {
	if sgoCurrent() != h.owner {
		panic(&sgoViolation{"LendTo of an I/O handle by a task that does not own it"})
	}
	if s.owner != h.owner {
		// Only nurseries of the owner: they all close before it ends, so no borrower outlives it.
		panic(&sgoViolation{"LendTo of an I/O handle to a nursery its owner does not own"})
	}
	h.lentTo = append(h.lentTo, s)
}

// sgoHandleClose closes h: only its owner, not while lent to an open nursery (D54).
func sgoHandleClose(h *sgoHandle) *sgoIOErr {
	t := sgoCurrent()
	if t != h.owner {
		panic(&sgoViolation{"Close of an I/O handle by a task that does not own it"})
	}
	if h.closed {
		return &sgoIOErr{"HandleClosed", 0}
	}
	live := h.lentTo[:0]
	for _, s := range h.lentTo {
		if sgoScopeOpen(s) {
			live = append(live, s)
		}
	}
	h.lentTo = live
	if len(live) > 0 || h.ops.Load() != 0 {
		return &sgoIOErr{"HandleLent", 0}
	}
	sgoHandleFree(h)
	return nil
}

func sgoHandleFree(h *sgoHandle) {
	h.closed = true
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
	if h.fd >= 0 {
		sgoCloseFD(h.fd)
	}
	h.fd = -1
	t := h.owner
	if h.prev != nil {
		h.prev.next = h.next
	} else if t.handles == h {
		t.handles = h.next
	}
	if h.next != nil {
		h.next.prev = h.prev
	}
	h.prev, h.next = nil, nil
}

// sgoHandlesResolve closes the task's open handles opened inside scope s (all of them at task
// end), each with a HandleAbandoned entry in the task's failure sequence (D54).
func sgoHandlesResolve(t *sgoTask, s *sgoScope, all bool) {
	for h := t.handles; h != nil; {
		next := h.next
		if all || h.scope == s && !sgoHandleLentOpen(h) {
			t.abandoned++
			t.seq++
			t.entries = append(t.entries, sgoEntry{t.seq, sgoFailureInfo{"HandleAbandoned",
				"I/O handle " + h.name + " opened by task " + sgoItoa(t.id) + " was not closed; the runtime closed it", ""}})
			sgoHandleFree(h)
		}
		h = next
	}
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
