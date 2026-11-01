// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Lab fake of the file adapters (W09 step 4, D58): an in-memory directory tree per grant, empty
// at the start of the run, with the contracts and errors of the real adapters (sgo_iodir.go,
// sgo_io.go). Every operation is a choice point (SGO-LAB-003). Lab runs are serialized by the
// baton, so the tree needs no lock. Real host I/O is never reached from a lab binary.

package runtime

type sgoFakeNode struct {
	dir  bool
	data []byte
	kids []sgoFakeEntry // directories: entries in creation order
}

type sgoFakeEntry struct {
	name string
	node *sgoFakeNode
}

// sgoFakeFile is the open-file state of a fake file handle.
type sgoFakeFile struct {
	node *sgoFakeNode
	off  int
}

func (d *sgoFakeNode) lookup(name string) *sgoFakeNode {
	for _, e := range d.kids {
		if e.name == name {
			return e.node
		}
	}
	return nil
}

// sgoFakeWalk resolves path in grant g: the parent directory, the last name, and the node it
// names (nil if none). path "" names the grant's root itself (parent nil).
func sgoFakeWalk(g int, path string) (parent *sgoFakeNode, last string, node *sgoFakeNode, e *sgoIOErr) {
	sgoLabPoint()
	if sgoTaskCancellation(sgoCurrent()) != nil {
		return nil, "", nil, &sgoIOErr{kind: "Cancelled"}
	}
	root := sgoDirGrants[g].fake
	if path == "" {
		return nil, "", root, nil
	}
	names, ok := sgoParsePath(path)
	if !ok {
		return nil, "", nil, &sgoIOErr{kind: "PathRejected"}
	}
	d := root
	for _, n := range names[:len(names)-1] {
		d = d.lookup(n)
		if d == nil || !d.dir {
			return nil, "", nil, &sgoIOErr{kind: "NotFound"}
		}
	}
	last = names[len(names)-1]
	return d, last, d.lookup(last), nil
}

func sgoFakeIsDir(g int, path string) *sgoIOErr {
	p, _, n, e := sgoFakeWalk(g, path)
	switch {
	case e != nil:
		return e
	case p == nil:
		return &sgoIOErr{kind: "PathRejected"} // "" is a path only for List, as in sgoResolve
	case n == nil || !n.dir:
		return &sgoIOErr{kind: "NotFound"}
	}
	return nil
}

func sgoFakeMkdir(g int, path string) *sgoIOErr {
	p, last, n, e := sgoFakeWalk(g, path)
	switch {
	case e != nil:
		return e
	case p == nil:
		return &sgoIOErr{kind: "PathRejected"}
	case n != nil:
		return &sgoIOErr{kind: "Exists"}
	}
	p.kids = append(p.kids, sgoFakeEntry{last, &sgoFakeNode{dir: true}})
	return nil
}

func sgoFakeRemove(g int, path string) *sgoIOErr {
	p, _, n, e := sgoFakeWalk(g, path)
	switch {
	case e != nil:
		return e
	case p == nil:
		return &sgoIOErr{kind: "PathRejected"}
	case n == nil:
		return &sgoIOErr{kind: "NotFound"}
	case n.dir && len(n.kids) > 0:
		return &sgoIOErr{kind: "NotEmpty"}
	}
	for i, k := range p.kids {
		if k.node == n {
			p.kids = append(p.kids[:i:i], p.kids[i+1:]...)
			break
		}
	}
	return nil
}

func sgoFakeList(g int, path string) ([]string, *sgoIOErr) {
	_, _, n, e := sgoFakeWalk(g, path)
	switch {
	case e != nil:
		return nil, e
	case n == nil || !n.dir:
		return nil, &sgoIOErr{kind: "NotFound"}
	}
	names := make([]string, len(n.kids))
	for i, k := range n.kids {
		names[i] = k.name
	}
	sgoSortStrings(names)
	return names, nil
}

// sgoFakeOpen opens path in grant g as a fake file handle of the calling task.
func sgoFakeOpen(g int, path string, write bool) (*sgoHandle, *sgoIOErr) {
	if write && sgoDirGrants[g].ro {
		return nil, &sgoIOErr{kind: "ReadOnly"}
	}
	p, last, n, e := sgoFakeWalk(g, path)
	switch {
	case e != nil:
		return nil, e
	case p == nil:
		return nil, &sgoIOErr{kind: "PathRejected"}
	case n != nil && n.dir:
		return nil, &sgoIOErr{kind: "IsDirectory"}
	case n == nil && !write:
		return nil, &sgoIOErr{kind: "NotFound"}
	case n == nil:
		n = &sgoFakeNode{}
		p.kids = append(p.kids, sgoFakeEntry{last, n})
	case write:
		n.data = nil // truncate
	}
	h := sgoHandleNew(sgoHFile, -1, path)
	h.write = write
	h.fake = &sgoFakeFile{node: n}
	return h, nil
}

func sgoFakeRead(h *sgoHandle, max int) (string, *sgoIOErr) {
	sgoLabPoint()
	f := h.fake
	if f.off >= len(f.node.data) {
		return "", &sgoIOErr{kind: "EOF"}
	}
	end := min(len(f.node.data), f.off+min(max, sgoIOChunk))
	s := string(f.node.data[f.off:end])
	f.off = end
	return s, nil
}

func sgoFakeWrite(h *sgoHandle, s string) (int, *sgoIOErr) {
	f := h.fake
	done := 0
	for done < len(s) {
		sgoLabPoint() // each bounded call is a choice point; cancellation is seen between calls
		if done > 0 && sgoTaskCancellation(sgoCurrent()) != nil {
			return done, &sgoIOErr{"Cancelled", done}
		}
		chunk := s[done : done+min(len(s)-done, sgoIOChunk)]
		if f.off > len(f.node.data) {
			f.node.data = append(f.node.data, make([]byte, f.off-len(f.node.data))...)
		}
		n := copy(f.node.data[f.off:], chunk)
		f.node.data = append(f.node.data, chunk[n:]...)
		f.off += len(chunk)
		done += len(chunk)
	}
	return done, nil
}
