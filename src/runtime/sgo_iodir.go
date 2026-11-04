// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Directory operations of the S-Go file adapters (W09, D53, D59): every path is resolved from the
// grant's held directory by sgoResolve; nothing is resolved as a string.

package runtime

const sgoMaxReadFile = 16 << 20 // ReadFile bound (D59)

// sgoDirRoot returns the descriptor of grant g, or the error of its open at start.
func sgoDirRoot(g int) (int32, *sgoIOErr) {
	d := &sgoDirGrants[g]
	if d.fd < 0 {
		return -1, &sgoIOErr{kind: d.err}
	}
	return d.fd, nil
}

// sgoDirBegin checks cancellation before a directory operation (D55).
func sgoDirBegin(g int) (int32, *sgoIOErr) {
	if sgoTaskCancellation(sgoCurrent()) != nil {
		return -1, &sgoIOErr{kind: "Cancelled"}
	}
	return sgoDirRoot(g)
}

// sgoDirParent resolves the directory holding the last name of path. It returns the parent
// descriptor (to be closed by sgoDirParentDone), and the last name as a C string.
func sgoDirParent(root int32, path string) (int32, *byte, *sgoIOErr) {
	names, ok := sgoParsePath(path)
	if !ok {
		return -1, nil, &sgoIOErr{kind: "PathRejected"}
	}
	last := sgoCString(names[len(names)-1])
	if len(names) == 1 {
		return root, last, nil
	}
	fd, kind := sgoResolve(root, path[:len(path)-len(names[len(names)-1])-1], sgoO_RDONLY|sgoO_DIRECTORY, 0)
	if kind != "" {
		return -1, nil, &sgoIOErr{kind: kind}
	}
	return fd, last, nil
}

// sgoDirParentStill reports whether the parent of path in root is still the directory fd after an
// effect on fd (W12 R6): if another process moved it out of the grant between the resolution and
// the effect, the effect happened outside, and the caller reports PathRaced (D56: no unreported
// partial effect). On linux the parent was opened with openat2, so only this window remains.
func sgoDirParentStill(root, fd int32, path string) bool {
	if fd == root {
		return true
	}
	names, _ := sgoParsePath(path)
	again, kind := sgoResolve(root, path[:len(path)-len(names[len(names)-1])-1], sgoO_RDONLY|sgoO_DIRECTORY, 0)
	if kind != "" {
		return false
	}
	defer sgoCloseFD(again)
	a, _, _ := sgoFdID(fd)
	b, _, _ := sgoFdID(again)
	return a == b
}

// sgoIORaceHook, when set by a runtime test, runs in the windows a concurrent rename can race:
// between resolving a parent and acting on it, and between a walk's last open and its
// verification (W12 R6). It is never set outside tests.
var sgoIORaceHook func(site string)

func sgoDirParentDone(root, fd int32) {
	if fd != root {
		sgoCloseFD(fd)
	}
}

// sgoDirIsDir checks that path names a directory in grant g (Dir.Sub).
func sgoDirIsDir(g int, path string) *sgoIOErr {
	if sgoLab != nil {
		return sgoFakeIsDir(g, path)
	}
	root, e := sgoDirBegin(g)
	if e != nil {
		return e
	}
	fd, kind := sgoResolve(root, path, sgoO_RDONLY|sgoO_DIRECTORY, 0)
	if kind != "" {
		return &sgoIOErr{kind: kind}
	}
	sgoCloseFD(fd)
	return nil
}

// sgoDirMkdir creates directory path in grant g.
func sgoDirMkdir(g int, path string) *sgoIOErr {
	if sgoLab != nil {
		return sgoFakeMkdir(g, path)
	}
	root, e := sgoDirBegin(g)
	if e != nil {
		return e
	}
	fd, name, e := sgoDirParent(root, path)
	if e != nil {
		return e
	}
	defer sgoDirParentDone(root, fd)
	if sgoIORaceHook != nil {
		sgoIORaceHook("effect")
	}
	if r := sgoMkdirat(fd, name, 0o755); r < 0 {
		return &sgoIOErr{kind: sgoErrKind(r)}
	}
	if !sgoDirParentStill(root, fd, path) {
		return &sgoIOErr{kind: "PathRaced"}
	}
	return nil
}

// sgoDirRemove removes the file, or the empty directory, path in grant g. A symlink is removed
// itself, never followed.
func sgoDirRemove(g int, path string) *sgoIOErr {
	if sgoLab != nil {
		return sgoFakeRemove(g, path)
	}
	root, e := sgoDirBegin(g)
	if e != nil {
		return e
	}
	fd, name, e := sgoDirParent(root, path)
	if e != nil {
		return e
	}
	defer sgoDirParentDone(root, fd)
	var st sgoStatBuf
	if r := sgoFstatat(fd, name, &st, sgoAT_SYMLINK_NOFOLLOW); r < 0 {
		return &sgoIOErr{kind: sgoErrKind(r)}
	}
	flags := int32(0)
	if _, _, isDir, _ := sgoStatID(&st); isDir {
		flags = sgoAT_REMOVEDIR
	}
	if sgoIORaceHook != nil {
		sgoIORaceHook("effect")
	}
	if r := sgoUnlinkat(fd, name, flags); r < 0 {
		return &sgoIOErr{kind: sgoErrKind(r)}
	}
	if !sgoDirParentStill(root, fd, path) {
		return &sgoIOErr{kind: "PathRaced"}
	}
	return nil
}

// sgoDirOpenGrant opens path in grant g as a file handle of the calling task.
func sgoDirOpenGrant(g int, path string, write bool) (*sgoHandle, *sgoIOErr) {
	if sgoLab != nil {
		return sgoFakeOpen(g, path, write)
	}
	root, e := sgoDirBegin(g)
	if e != nil {
		return nil, e
	}
	if write && sgoDirGrants[g].ro {
		return nil, &sgoIOErr{kind: "ReadOnly"}
	}
	d := &sgoHandle{kind: sgoHDir, fd: root, body: sgoCurrent(), readOnly: sgoDirGrants[g].ro} // a transient handle, in no list
	return sgoDirOpenFile(d, path, write)
}

// sgoDirReadFile reads the whole file path of grant g (at most 16 MiB: TooLarge).
func sgoDirReadFile(g int, path string) (string, *sgoIOErr) {
	f, e := sgoDirOpenGrant(g, path, false)
	if e != nil {
		return "", e
	}
	defer sgoHandleFree(f)
	size, e := sgoFileSize(f)
	if e != nil {
		return "", e
	}
	if size > sgoMaxReadFile {
		return "", &sgoIOErr{kind: "TooLarge"}
	}
	var out []byte
	for {
		s, e := sgoFileRead(f, sgoIOChunk)
		if e != nil && e.kind == "EOF" {
			return string(out), nil
		}
		if e != nil {
			return "", e
		}
		if len(out)+len(s) > sgoMaxReadFile {
			return "", &sgoIOErr{kind: "TooLarge"}
		}
		out = append(out, s...)
	}
}

// sgoDirWriteFile creates or truncates path in grant g and writes data.
func sgoDirWriteFile(g int, path, data string) *sgoIOErr {
	f, e := sgoDirOpenGrant(g, path, true)
	if e != nil {
		return e
	}
	defer sgoHandleFree(f)
	_, e = sgoFileWrite(f, data)
	return e
}

// sgoDirList returns the sorted names in directory path of grant g ("" is the grant directory).
func sgoDirList(g int, path string) ([]string, *sgoIOErr) {
	if sgoLab != nil {
		return sgoFakeList(g, path)
	}
	root, e := sgoDirBegin(g)
	if e != nil {
		return nil, e
	}
	var fd int32
	if path == "" {
		fd = sgoOpenat(root, sgoCString("."), sgoO_RDONLY|sgoO_DIRECTORY|sgoO_CLOEXEC, 0)
		if fd < 0 {
			return nil, &sgoIOErr{kind: sgoErrKind(fd)}
		}
	} else {
		var kind string
		if fd, kind = sgoResolve(root, path, sgoO_RDONLY|sgoO_DIRECTORY, 0); kind != "" {
			return nil, &sgoIOErr{kind: kind}
		}
	}
	names, r := sgoReadDirNames(fd) // closes fd
	if r < 0 {
		return nil, &sgoIOErr{kind: sgoErrKind(r)}
	}
	sgoSortStrings(names)
	return names, nil
}

// sgoSortStrings sorts s by byte order (merge sort; the runtime cannot import sort).
func sgoSortStrings(s []string) {
	if len(s) < 2 {
		return
	}
	tmp := make([]string, len(s))
	for w := 1; w < len(s); w *= 2 {
		for lo := 0; lo < len(s)-w; lo += 2 * w {
			mid, hi := lo+w, min(lo+2*w, len(s))
			i, j, k := lo, mid, lo
			for i < mid && j < hi {
				if s[j] < s[i] {
					tmp[k], j = s[j], j+1
				} else {
					tmp[k], i = s[i], i+1
				}
				k++
			}
			k += copy(tmp[k:], s[i:mid])
			copy(tmp[k:], s[j:hi])
			copy(s[lo:hi], tmp[lo:hi])
		}
	}
}
