// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

// Confined path resolution (W09, D53): the reference for the runtime's resolver.
//
// A path is a sequence of names resolved one component at a time from the held root directory,
// never as a string. Every component is opened without following symlinks; a symlink anywhere is
// PathRejected. Because a directory opened during the walk may be moved by another process, the
// resolver verifies the chain after opening the last component: from the root, each name must
// still name the object that was opened for it. If not, it retries (at most ioMaxRetries times)
// and then fails with PathRejected. The guarantee: the object returned was inside the root at the
// moment of verification.

import "strings"

// FKind is the kind of a tree node.
type FKind byte

const (
	FDir  FKind = 'd'
	FFile FKind = 'f'
	FLink FKind = 'l'
)

// FNode is a file-system object. IDs are unique per tree (an inode).
type FNode struct {
	ID       int
	Kind     FKind
	Children map[string]*FNode // directories
}

// ioSkipVerify disables the verification step (the model's seeded-bug test only).
var ioSkipVerify bool

const (
	ioMaxName    = 255
	ioMaxPath    = 4096
	ioMaxRetries = 4
)

// ParsePath splits a granted path into names, or returns "PathRejected": empty path, empty
// names, ".", "..", NUL, a leading "/", a name over 255 bytes, or a path over 4096 bytes.
func ParsePath(p string) ([]string, string) {
	if p == "" || len(p) > ioMaxPath || p[0] == '/' || strings.IndexByte(p, 0) >= 0 {
		return nil, "PathRejected"
	}
	names := strings.Split(p, "/")
	for _, n := range names {
		if n == "" || n == "." || n == ".." || len(n) > ioMaxName {
			return nil, "PathRejected"
		}
	}
	return names, ""
}

// rWalk is one resolution in progress: the objects opened so far (opened[0] is the root).
type rWalk struct {
	names   []string
	opened  []*FNode
	retries int
	result  *FNode
	err     string
	done    bool
}

// step advances the resolution by one atomic action against the current tree: open the next
// component (O_NOFOLLOW; intermediate components must be directories), or, once all are open,
// verify the chain from the root. root is the granted directory (its identity never changes).
func (w *rWalk) step(root *FNode) {
	if w.done {
		return
	}
	if len(w.opened) <= len(w.names) {
		parent := w.opened[len(w.opened)-1]
		name := w.names[len(w.opened)-1]
		last := len(w.opened) == len(w.names)
		c := parent.Children[name]
		switch {
		case c == nil:
			w.err, w.done = "NotFound", true
		case c.Kind == FLink:
			w.err, w.done = "PathRejected", true
		case !last && c.Kind != FDir:
			w.err, w.done = "NotFound", true // ENOTDIR
		default:
			w.opened = append(w.opened, c)
		}
		return
	}
	// Verify: from the root, every name still names the object opened for it.
	if ioSkipVerify { // a seeded bug for the model's own test
		w.result, w.done = w.opened[len(w.opened)-1], true
		return
	}
	cur := root
	for i, name := range w.names {
		next := cur.Children[name]
		if next != w.opened[i+1] {
			w.retries++
			if w.retries > ioMaxRetries {
				w.err, w.done = "PathRejected", true
				return
			}
			w.opened = w.opened[:1]
			return
		}
		cur = next
	}
	w.result, w.done = w.opened[len(w.opened)-1], true
}

// Reachable reports whether n is reachable from root (n inside the grant).
func Reachable(root, n *FNode) bool {
	if root == n {
		return true
	}
	for _, c := range root.Children {
		if c.Kind == FDir && Reachable(c, n) {
			return true
		}
		if c == n {
			return true
		}
	}
	return false
}
