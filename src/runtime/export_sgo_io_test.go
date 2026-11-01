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
