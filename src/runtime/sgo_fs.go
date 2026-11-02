// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Confined path resolution of the S-Go file adapters (W09, D53; reference: sgo/model/iopath.go).
//
// A path is a sequence of names, resolved one component at a time from the held directory
// descriptor of a grant, never as a string and never from the working directory. Every component
// is opened with O_NOFOLLOW (intermediate ones with O_DIRECTORY): a symlink anywhere is
// PathRejected. After the last component is open, the chain is verified from the grant's
// directory down: each name must still name the object opened for it (device and inode), or
// the resolution starts over (at most sgoResolveRetries times, then PathRejected).

package runtime

const (
	sgoMaxName        = 255
	sgoMaxPath        = 4096
	sgoResolveRetries = 4
)

// sgoParsePath splits a granted path into names; ok is false for an empty path or name, ".",
// "..", NUL, a leading "/", a name over 255 bytes, or a path over 4096 bytes.
func sgoParsePath(p string) ([]string, bool) {
	if p == "" || len(p) > sgoMaxPath || p[0] == '/' {
		return nil, false
	}
	var names []string
	start := 0
	for i := 0; i <= len(p); i++ {
		if i < len(p) && p[i] == 0 {
			return nil, false
		}
		if i == len(p) || p[i] == '/' {
			n := p[start:i]
			if n == "" || n == "." || n == ".." || len(n) > sgoMaxName {
				return nil, false
			}
			names = append(names, n)
			start = i + 1
		}
	}
	return names, true
}

// sgoCString returns s with a terminating NUL (s has no NUL: sgoParsePath checked).
func sgoCString(s string) *byte {
	b := make([]byte, len(s)+1)
	copy(b, s)
	return &b[0]
}

// sgoErrKind classifies a negated errno of a resolution step.
func sgoErrKind(e int32) string {
	switch -e {
	case sgoENOENT, sgoENOTDIR:
		return "NotFound"
	case sgoELOOP:
		return "PathRejected"
	case sgoEEXIST:
		return "Exists"
	case sgoEISDIR:
		return "IsDirectory"
	case sgoENOTEMPTY:
		return "NotEmpty"
	case sgoENXIO:
		// open(O_WRONLY|O_NONBLOCK) of a FIFO without a reader, or of a device (W12 R4).
		return "NotRegularFile"
	}
	return "IOError"
}

type sgoFileID struct{ dev, ino uint64 }

func sgoFdID(fd int32) (sgoFileID, bool, bool) {
	var st sgoStatBuf
	if sgoFstat(fd, &st) < 0 {
		return sgoFileID{}, false, false
	}
	dev, ino, isDir, isLink := sgoStatID(&st)
	return sgoFileID{dev, ino}, isDir, isLink
}

// sgoResolve opens path in the grant directory root with the given flags for the last component
// (plus O_NOFOLLOW and O_CLOEXEC). It returns the descriptor, or "" and an error kind.
func sgoResolve(root int32, path string, lastFlags, mode int32) (int32, string) {
	names, ok := sgoParsePath(path)
	if !ok {
		return -1, "PathRejected"
	}
	// linux: one openat2 beneath the grant (the kernel's guarantee, D53, W12 R6).
	joined := names[0]
	for _, n := range names[1:] {
		joined += "/" + n
	}
	cjoined := sgoCString(joined)
	beneath := false
	for attempt := 0; attempt <= sgoResolveRetries; attempt++ {
		fd, ok := sgoOpenBeneath(root, cjoined, lastFlags|sgoO_NOFOLLOW|sgoO_CLOEXEC, mode)
		if !ok {
			break // no openat2: walk and verify below
		}
		beneath = true
		switch {
		case fd >= 0:
			return fd, ""
		case fd == -sgoEINTR || fd == -sgoEAGAIN_RESOLVE:
			continue
		case fd == -sgoEXDEV || fd == -sgoELOOP:
			return -1, "PathRejected"
		}
		return -1, sgoErrKind(fd)
	}
	if beneath {
		return -1, "PathRejected" // renames kept racing the resolution
	}
	cnames := make([]*byte, len(names))
	for i, n := range names {
		cnames[i] = sgoCString(n)
	}
	for attempt := 0; attempt <= sgoResolveRetries; attempt++ {
		fds := []int32{root}
		ids := make([]sgoFileID, len(names))
		kind := ""
		created := false // this walk created the last name (W12 R11)
		for i := range names {
			flags := int32(sgoO_RDONLY | sgoO_DIRECTORY)
			if i == len(names)-1 {
				flags = lastFlags
			}
			open := func(flags int32) int32 {
				fd := sgoOpenat(fds[i], cnames[i], flags|sgoO_NOFOLLOW|sgoO_CLOEXEC, mode)
				for fd == -sgoEINTR {
					fd = sgoOpenat(fds[i], cnames[i], flags|sgoO_NOFOLLOW|sgoO_CLOEXEC, mode)
				}
				return fd
			}
			var fd int32
			if flags&sgoO_CREAT != 0 {
				// Create in two steps, so the walk knows whether it created the file: exclusively
				// first, then (it exists) without O_CREAT.
				// A file removed between the two steps is created again (bounded), so Create does
				// not report NotFound for it.
				for try := 0; try < 4; try++ {
					if fd = open(flags | sgoO_EXCL); fd >= 0 {
						created = true
					} else if fd == -sgoEEXIST {
						if fd = open(flags &^ sgoO_CREAT); fd == -sgoENOENT {
							continue
						}
					}
					break
				}
			} else {
				fd = open(flags)
			}
			if fd < 0 {
				kind = sgoErrKind(fd)
				if -fd == sgoENOTDIR || -fd == sgoELOOP {
					// A symlink in the path (darwin reports ENOTDIR for O_DIRECTORY on one).
					var st sgoStatBuf
					if sgoFstatat(fds[i], cnames[i], &st, sgoAT_SYMLINK_NOFOLLOW) == 0 {
						if _, _, _, isLink := sgoStatID(&st); isLink {
							kind = "PathRejected"
						}
					}
				}
				break
			}
			fds = append(fds, fd)
			ids[i], _, _ = sgoFdID(fd)
		}
		if kind != "" {
			sgoCloseFDs(fds[1:])
			return -1, kind
		}
		if sgoIORaceHook != nil {
			sgoIORaceHook("walk")
		}
		// Verify the chain from the grant down.
		good := true
		for i := range names {
			var st sgoStatBuf
			if sgoFstatat(fds[i], cnames[i], &st, sgoAT_SYMLINK_NOFOLLOW) < 0 {
				good = false
				break
			}
			dev, ino, _, _ := sgoStatID(&st)
			if (sgoFileID{dev, ino}) != ids[i] {
				good = false
				break
			}
		}
		last := fds[len(fds)-1]
		sgoCloseFDs(fds[1 : len(fds)-1])
		if good {
			return last, ""
		}
		sgoCloseFD(last)
		if created {
			// A concurrent rename moved the path out of the grant while this walk created the file
			// there: the effect happened outside, and is reported (D56, W12 R11). It is not undone:
			// removing it would act outside the grant too.
			return -1, "PathRaced"
		}
	}
	return -1, "PathRejected"
}

func sgoCloseFDs(fds []int32) {
	for _, fd := range fds {
		sgoCloseFD(fd)
	}
}
