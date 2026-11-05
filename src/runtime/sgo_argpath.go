// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Paths named on the command line (D181). "grant path args ro|rw" in sgo.mod lets the root use the
// paths the user typed as arguments, and only those: the user's own command line is the authority.
// For every argument the runtime keeps two directory slots, opened on first use, never at start:
// the argument itself as a directory (root.ArgDir), and the directory holding its last name
// (root.OpenArg and root.CreateArg open that name only, without following a symlink). The slots
// have no name, so root.Dir never reaches them, and no handle on a parent reaches the program.

package runtime

import _ "unsafe" // go:linkname

var (
	sgoArgGranted bool     // grant path args is present
	sgoArgDirSlot []int    // per argument: the slot of the argument as a directory
	sgoArgParent  []int    // per argument: the slot of the directory holding its last name, or -1
	sgoArgBase    []string // per argument: its last name, or "" if it names no file
)

// sgoArgPathsInit adds the argument slots to the directory grants (at start, from sgoIOGrantsInit).
func sgoArgPathsInit(ro bool) {
	sgoArgGranted = true
	if len(argslice) <= 1 {
		return
	}
	for _, a := range argslice[1:] {
		sgoArgDirSlot = append(sgoArgDirSlot, sgoArgSlot(a, ro))
		dir, base := sgoSplitArg(a)
		if base == "" {
			sgoArgParent = append(sgoArgParent, -1)
		} else {
			sgoArgParent = append(sgoArgParent, sgoArgSlot(dir, ro))
		}
		sgoArgBase = append(sgoArgBase, base)
	}
}

// sgoArgSlot appends an unnamed directory slot for path, opened on first use.
func sgoArgSlot(path string, ro bool) int {
	g := sgoDirGrant{ro: ro, path: path, fd: -1}
	switch {
	case sgoLab != nil:
		g.fake = &sgoFakeNode{dir: true} // the lab has no host files: an empty directory
	case path == "" || sgoIndexByte(path, 0) >= 0:
		g.err = "PathRejected"
	default:
		g.lazy = sgoCString(path)
	}
	sgoDirGrants = append(sgoDirGrants, g)
	return len(sgoDirGrants) - 1
}

// sgoSplitArg splits an argument path into the directory holding its last name and that name. The
// name is "" when the path names no file: empty, ending in '/', or ending in "." or "..".
func sgoSplitArg(p string) (dir, base string) {
	i := len(p) - 1
	for i >= 0 && p[i] != '/' {
		i--
	}
	base = p[i+1:]
	switch {
	case i < 0:
		dir = "."
	case i == 0:
		dir = "/"
	default:
		dir = p[:i]
	}
	if base == "" || base == "." || base == ".." {
		return dir, ""
	}
	return dir, base
}

// sgoArgPathOpen opens an argument slot on its first use. It does not allocate.
func sgoArgPathOpen(d *sgoDirGrant) {
	lock(&sgoAppDirLock)
	if d.fd < 0 && d.lazy != nil {
		fd := sgoOpenat(sgoAT_FDCWD, d.lazy, sgoO_RDONLY|sgoO_DIRECTORY|sgoO_CLOEXEC, 0)
		if fd < 0 {
			d.err = sgoErrKind(fd)
		} else {
			d.fd, d.err = fd, ""
		}
		d.lazy = nil
	}
	unlock(&sgoAppDirLock)
}

// sgo_argPath is the runtime half of root.ArgDir (file false) and root.OpenArg/CreateArg (file
// true): the slot, the name to open in it (files only), the grant's mode, and an error; granted is
// false when sgo.mod has no "grant path args".
//
//go:linkname sgo_argPath sgo.runtime_argPath
func sgo_argPath(i int, file bool, op string) (g int, name string, ro bool, err error, granted bool) {
	if !sgoArgGranted {
		return 0, "", false, nil, false
	}
	if i < 0 || i >= len(sgoArgDirSlot) {
		return 0, "", false, sgoIOResult(&sgoIOErr{kind: "PathRejected"}, op), true
	}
	g = sgoArgDirSlot[i]
	if file {
		if g = sgoArgParent[i]; g < 0 {
			return 0, "", false, sgoIOResult(&sgoIOErr{kind: "PathRejected"}, op), true
		}
		name = sgoArgBase[i]
	}
	ro = sgoDirGrants[g].ro
	if sgoLab != nil { // an in-memory directory: its operations report what is missing
		if sgoTaskCancellation(sgoCurrent()) != nil {
			return 0, "", false, sgoIOResult(&sgoIOErr{kind: "Cancelled"}, op), true
		}
	} else if _, e := sgoDirBegin(g); e != nil {
		return 0, "", false, sgoIOResult(e, op), true
	}
	return g, name, ro, nil, true
}
