// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test hooks for the S-Go file adapters (W09).

package runtime

// SGoResolve opens the grant directory dir (from the working directory, as the runtime does once
// at start) and resolves path in it for reading. It returns the inode of the resolved object, or
// an error kind.
func SGoResolve(dir, path string) (uint64, string) {
	root := sgoOpenat(sgoAT_FDCWD, sgoCString(dir), sgoO_RDONLY|sgoO_DIRECTORY|sgoO_CLOEXEC, 0)
	if root < 0 {
		return 0, "no grant directory"
	}
	defer sgoCloseFD(root)
	fd, kind := sgoResolve(root, path, sgoO_RDONLY, 0)
	if kind != "" {
		return 0, kind
	}
	id, _, _ := sgoFdID(fd)
	sgoCloseFD(fd)
	return id.ino, ""
}

type SGoHandle = sgoHandle

func sgoIOErrKind(e *sgoIOErr) string {
	if e == nil {
		return ""
	}
	return e.kind
}

// SGoDirOpen opens grant directory dir as a Dir handle of the calling task (ro: read-only grant).
func SGoDirOpen(dir string, ro bool) *SGoHandle {
	fd := sgoOpenat(sgoAT_FDCWD, sgoCString(dir), sgoO_RDONLY|sgoO_DIRECTORY|sgoO_CLOEXEC, 0)
	if fd < 0 {
		return nil
	}
	h := sgoHandleNew(sgoHDir, fd, dir)
	h.readOnly = ro
	return h
}

func SGoDirFile(h *SGoHandle, path string, write bool) (*SGoHandle, string) {
	f, e := sgoDirOpenFile(h, path, write)
	return f, sgoIOErrKind(e)
}
func SGoFileRead(h *SGoHandle, max int) (string, string) {
	s, e := sgoFileRead(h, max)
	return s, sgoIOErrKind(e)
}
func SGoFileWrite(h *SGoHandle, s string) (int, string) {
	n, e := sgoFileWrite(h, s)
	return n, sgoIOErrKind(e)
}
func SGoFileSize(h *SGoHandle) (int, string) {
	n, e := sgoFileSize(h)
	return n, sgoIOErrKind(e)
}
func SGoHandleClose(h *SGoHandle) string      { return sgoIOErrKind(sgoHandleClose(h)) }
func SGoHandleLend(h *SGoHandle, s *SGoScope) { sgoHandleLend(h, s) }
func SGoHandleFD(h *SGoHandle) int32          { return h.fd }
func SGoOpenHandles() (n int) {
	for h := sgoCurrent().handles; h != nil; h = h.next {
		n++
	}
	return n
}
