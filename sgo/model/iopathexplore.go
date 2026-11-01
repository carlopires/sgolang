// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import (
	"fmt"
	"sort"
	"strings"
)

// RScenario is a confinement scenario: a world tree (entries "d path", "f path", "l path", paths
// from the world root; the grant is the directory "grant"), a path resolved in the grant, and
// renames ("from to", world paths) that another process performs in this order, interleaved
// with the resolution in every possible way.
type RScenario struct {
	Entries []string
	Path    string
	Renames []string
}

type rWorld struct {
	top    *FNode
	origin map[*FNode]string // the world path an object was created at
}

func buildWorld(entries []string) (*rWorld, error) {
	w := &rWorld{top: &FNode{ID: 0, Kind: FDir, Children: map[string]*FNode{}}, origin: map[*FNode]string{}}
	w.origin[w.top] = "/"
	for i, e := range entries {
		kind, path, ok := strings.Cut(e, " ")
		if !ok || len(kind) != 1 {
			return nil, fmt.Errorf("bad entry %q", e)
		}
		names := strings.Split(path, "/")
		dir := w.top
		for _, n := range names[:len(names)-1] {
			if dir = dir.Children[n]; dir == nil || dir.Kind != FDir {
				return nil, fmt.Errorf("entry %q: no directory %q", e, n)
			}
		}
		n := &FNode{ID: i + 1, Kind: FKind(kind[0])}
		if n.Kind == FDir {
			n.Children = map[string]*FNode{}
		}
		dir.Children[names[len(names)-1]] = n
		w.origin[n] = path
	}
	if g := w.top.Children["grant"]; g == nil || g.Kind != FDir {
		return nil, fmt.Errorf("no grant directory")
	}
	return w, nil
}

// clone copies the world and extra (objects held by a resolution that a rename may have removed
// from the tree); remap maps old objects to their copies.
func (w *rWorld) clone(extra []*FNode) (*rWorld, map[*FNode]*FNode) {
	remap := map[*FNode]*FNode{}
	var cp func(n *FNode) *FNode
	cp = func(n *FNode) *FNode {
		if c, ok := remap[n]; ok {
			return c
		}
		c := &FNode{ID: n.ID, Kind: n.Kind}
		remap[n] = c
		if n.Children != nil {
			c.Children = map[string]*FNode{}
			for k, v := range n.Children {
				c.Children[k] = cp(v)
			}
		}
		return c
	}
	nw := &rWorld{top: cp(w.top), origin: map[*FNode]string{}}
	for _, e := range extra {
		cp(e)
	}
	for n, o := range w.origin {
		if c, ok := remap[n]; ok {
			nw.origin[c] = o
		}
	}
	return nw, remap
}

func (w *rWorld) lookup(path string) (*FNode, *FNode, string) {
	names := strings.Split(path, "/")
	dir := w.top
	for _, n := range names[:len(names)-1] {
		if dir = dir.Children[n]; dir == nil {
			return nil, nil, ""
		}
	}
	last := names[len(names)-1]
	return dir, dir.Children[last], last
}

// rename moves from to to (an existing target is replaced, as rename(2) does for files).
func (w *rWorld) rename(op string) error {
	from, to, ok := strings.Cut(op, " ")
	if !ok {
		return fmt.Errorf("bad rename %q", op)
	}
	fd, n, fname := w.lookup(from)
	td, _, tname := w.lookup(to)
	if n == nil || td == nil {
		return nil // the rename fails: nothing changes
	}
	delete(fd.Children, fname)
	td.Children[tname] = n
	return nil
}

type rState struct {
	world *rWorld
	walk  *rWalk
	next  int // renames done
}

// ExploreResolve returns the outcome set of a scenario: "ok <origin>" or the error kind. It
// checks, on every path, that a successful resolution returns an object reachable from the grant
// at the moment of verification.
func ExploreResolve(sc RScenario) ([]string, error) {
	w, err := buildWorld(sc.Entries)
	if err != nil {
		return nil, err
	}
	names, perr := ParsePath(sc.Path)
	if perr != "" {
		return []string{perr}, nil
	}
	root := w.top.Children["grant"]
	start := &rState{world: w, walk: &rWalk{names: names, opened: []*FNode{root}}}
	outcomes := map[string]bool{}
	var visit func(s *rState) error
	visit = func(s *rState) error {
		if s.walk.done {
			if s.walk.err != "" {
				outcomes[s.walk.err] = true
			} else {
				outcomes["ok "+s.world.origin[s.walk.result]] = true
			}
			return nil
		}
		// The resolver takes a step.
		a := s.copy()
		groot := a.world.top.Children["grant"]
		a.walk.step(groot)
		if a.walk.done && a.walk.err == "" && !Reachable(groot, a.walk.result) {
			return fmt.Errorf("resolved %s outside the grant", a.world.origin[a.walk.result])
		}
		if err := visit(a); err != nil {
			return err
		}
		// Or the other process performs its next rename.
		if s.next < len(sc.Renames) {
			b := s.copy()
			if err := b.world.rename(sc.Renames[b.next]); err != nil {
				return err
			}
			b.next++
			if err := visit(b); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(start); err != nil {
		return nil, err
	}
	var out []string
	for o := range outcomes {
		out = append(out, o)
	}
	sort.Strings(out)
	return out, nil
}

func (s *rState) copy() *rState {
	extra := append([]*FNode(nil), s.walk.opened...)
	if s.walk.result != nil {
		extra = append(extra, s.walk.result)
	}
	w, remap := s.world.clone(extra)
	wk := *s.walk
	wk.opened = make([]*FNode, len(s.walk.opened))
	for i, n := range s.walk.opened {
		wk.opened[i] = remap[n]
	}
	if s.walk.result != nil {
		wk.result = remap[s.walk.result]
	}
	return &rState{world: w, walk: &wk, next: s.next}
}
