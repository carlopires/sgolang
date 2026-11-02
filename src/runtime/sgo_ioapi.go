// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sgo entry points of the file and network adapters (src/sgo/io.go; W09, D52–D59).

package runtime

import "unsafe"

// sgoIOError is an I/O error as package sgo returns it (D56): its kind, the operation, and the
// bytes transferred before it. A Cancelled error also counts as a cancellation (sgoIsCancelled).
type sgoIOError struct {
	kind, op, msg string
	progress      int
}

func (e *sgoIOError) Error() string {
	s := e.kind + ": " + e.op
	if e.msg != "" {
		s = e.msg + " (" + e.op + ")"
	}
	if e.progress > 0 {
		s += " after " + sgoItoa(uint64(e.progress)) + " bytes"
	}
	return s
}

// sgoIOResult converts a runtime I/O result to the error package sgo returns (nil for nil).
func sgoIOResult(e *sgoIOErr, op string) error {
	if e == nil {
		return nil
	}
	r := &sgoIOError{kind: e.kind, op: op, progress: e.progress}
	if e.kind == "Cancelled" {
		if c := sgoTaskCancellation(sgoCurrent()); c != nil {
			r.msg = c.Error()
		}
	}
	return r
}

//go:linkname sgo_ioInfo sgo.runtime_ioInfo
func sgo_ioInfo(err error) (progress int, noEffect, eof bool) {
	e, ok := any(err).(*sgoIOError)
	if !ok {
		return 0, false, false
	}
	// IOError leaves the effect unknown, and PathRaced reports an effect that happened (W12 R6);
	// every other kind with no progress had none.
	return e.progress, e.progress == 0 && e.kind != "IOError" && e.kind != "PathRaced", e.kind == "EOF"
}

//go:linkname sgo_dirGrant sgo.runtime_dirGrant
func sgo_dirGrant(name string) (g int, ro, ok bool) {
	for i := range sgoDirGrants {
		if sgoDirGrants[i].name == name {
			return i, sgoDirGrants[i].ro, true
		}
	}
	return 0, false, false
}

//go:linkname sgo_dirIsDir sgo.runtime_dirIsDir
func sgo_dirIsDir(g int, path string) error { return sgoIOResult(sgoDirIsDir(g, path), "Dir.Sub") }

//go:linkname sgo_dirMkdir sgo.runtime_dirMkdir
func sgo_dirMkdir(g int, path string) error { return sgoIOResult(sgoDirMkdir(g, path), "Dir.Mkdir") }

//go:linkname sgo_dirRemove sgo.runtime_dirRemove
func sgo_dirRemove(g int, path string) error {
	return sgoIOResult(sgoDirRemove(g, path), "Dir.Remove")
}

//go:linkname sgo_dirOpen sgo.runtime_dirOpen
func sgo_dirOpen(g int, path string, write bool) (unsafe.Pointer, error) {
	op := "Dir.Open"
	if write {
		op = "Dir.Create"
	}
	h, e := sgoDirOpenGrant(g, path, write)
	if e != nil {
		return nil, sgoIOResult(e, op)
	}
	return unsafe.Pointer(h), nil
}

//go:linkname sgo_dirReadFile sgo.runtime_dirReadFile
func sgo_dirReadFile(g int, path string) (string, error) {
	s, e := sgoDirReadFile(g, path)
	return s, sgoIOResult(e, "Dir.ReadFile")
}

//go:linkname sgo_dirWriteFile sgo.runtime_dirWriteFile
func sgo_dirWriteFile(g int, path, data string) error {
	return sgoIOResult(sgoDirWriteFile(g, path, data), "Dir.WriteFile")
}

//go:linkname sgo_read sgo.runtime_read
func sgo_read(h unsafe.Pointer, max int) (string, error) {
	hh := (*sgoHandle)(h)
	if hh.kind == sgoHConn {
		s, e := sgoConnRead(hh, max)
		return s, sgoIOResult(e, "Conn.Read")
	}
	s, e := sgoFileRead(hh, max)
	return s, sgoIOResult(e, "File.Read")
}

//go:linkname sgo_ioWrite sgo.runtime_ioWrite
func sgo_ioWrite(h unsafe.Pointer, s string) (int, error) {
	hh := (*sgoHandle)(h)
	if hh.kind == sgoHConn {
		n, e := sgoConnWrite(hh, s)
		return n, sgoIOResult(e, "Conn.Write")
	}
	n, e := sgoFileWrite(hh, s)
	return n, sgoIOResult(e, "File.Write")
}

//go:linkname sgo_fileSize sgo.runtime_fileSize
func sgo_fileSize(h unsafe.Pointer) (int, error) {
	n, e := sgoFileSize((*sgoHandle)(h))
	return n, sgoIOResult(e, "File.Size")
}

//go:linkname sgo_close sgo.runtime_close
func sgo_close(h unsafe.Pointer, op string) error {
	return sgoIOResult(sgoHandleClose((*sgoHandle)(h)), op)
}

//go:linkname sgo_lend sgo.runtime_lend
func sgo_lend(h, scope unsafe.Pointer) { sgoHandleLend((*sgoHandle)(h), (*sgoScope)(scope)) }

//go:linkname sgo_dial sgo.runtime_dial
func sgo_dial(addr string) (unsafe.Pointer, error) {
	g := sgoNetGrantFor(addr, false)
	if g == nil {
		panic(error(&sgoFailure{"MissingCapability", "Dial " + quoteKind(addr) + ": no connect grant for this endpoint", ""}))
	}
	var h *sgoHandle
	var e *sgoIOErr
	if sgoLab != nil {
		h, e = sgoFakeDial(g) // lab runs: the run's loopback network (D58)
	} else {
		h, e = sgoNetConnect(g)
	}
	if e != nil {
		return nil, sgoIOResult(e, "Dial")
	}
	return unsafe.Pointer(h), nil
}

//go:linkname sgo_listen sgo.runtime_listen
func sgo_listen(addr string) (unsafe.Pointer, error) {
	g := sgoNetGrantFor(addr, true)
	if g == nil {
		panic(error(&sgoFailure{"MissingCapability", "Listen " + quoteKind(addr) + ": no listen grant for this endpoint", ""}))
	}
	var h *sgoHandle
	var e *sgoIOErr
	if sgoLab != nil {
		h, e = sgoFakeListen(g)
	} else {
		h, e = sgoNetListen(g)
	}
	if e != nil {
		return nil, sgoIOResult(e, "Listen")
	}
	return unsafe.Pointer(h), nil
}

//go:linkname sgo_accept sgo.runtime_accept
func sgo_accept(h unsafe.Pointer) (unsafe.Pointer, error) {
	c, e := sgoNetAccept((*sgoHandle)(h))
	if e != nil {
		return nil, sgoIOResult(e, "Listener.Accept")
	}
	return unsafe.Pointer(c), nil
}

//go:linkname sgo_dirList sgo.runtime_dirList
func sgo_dirList(g int, path string) ([]string, error) {
	names, e := sgoDirList(g, path)
	return names, sgoIOResult(e, "Dir.List")
}

//go:linkname sgo_peerUID sgo.runtime_peerUID
func sgo_peerUID(h unsafe.Pointer) (int, error) {
	uid, e := sgoNetPeerUID((*sgoHandle)(h))
	return uid, sgoIOResult(e, "Conn.PeerUID")
}
