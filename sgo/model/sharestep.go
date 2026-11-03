// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

// Dynamic semantics of the sharing rule: objects with an owner, tasks, and channels. One step is
// one statement of one task, executed atomically.

import (
	"fmt"
	"slices"
	"strings"
)

const (
	sInFlight = -2 // owned by a channel (sent, not yet received)
)

type sObj struct {
	Owner  int // task index, or sInFlight
	Frozen bool
	Child  int // object index, or -1
}

type sTask struct {
	Body, PC int
	Vars     map[string]int
	Kinds    map[string]SKind // static kinds, as the checker assigns them
}

type sState struct {
	Objs  []sObj
	Tasks []sTask
	Chans [][]int
}

func (s *sState) clone() *sState {
	c := &sState{Objs: slices.Clone(s.Objs), Chans: make([][]int, len(s.Chans))}
	for i, q := range s.Chans {
		c.Chans[i] = slices.Clone(q)
	}
	for _, t := range s.Tasks {
		v := make(map[string]int, len(t.Vars))
		for k, x := range t.Vars {
			v[k] = x
		}
		kd := make(map[string]SKind, len(t.Kinds))
		for k, x := range t.Kinds {
			kd[k] = x
		}
		c.Tasks = append(c.Tasks, sTask{t.Body, t.PC, v, kd})
	}
	return c
}

func (s *sState) key() string {
	var b strings.Builder
	fmt.Fprint(&b, s.Objs, s.Chans)
	for _, t := range s.Tasks {
		ks := make([]string, 0, len(t.Vars))
		for k, x := range t.Vars {
			ks = append(ks, fmt.Sprint(k, "=", x))
		}
		slices.Sort(ks)
		fmt.Fprint(&b, "|", t.Body, ",", t.PC, ks)
	}
	return b.String()
}

func (s *sState) alloc(o sObj) int {
	s.Objs = append(s.Objs, o)
	return len(s.Objs) - 1
}

// reach lists the objects reachable from o (o first).
func (s *sState) reach(o int) []int {
	var out []int
	for o >= 0 && !slices.Contains(out, o) {
		out = append(out, o)
		o = s.Objs[o].Child
	}
	return out
}

// access checks task t's access to object o: a frozen object may be read by anyone and written by
// no one; a mutable one may be accessed only by its owner.
func (s *sState) access(t, o int, write bool) error {
	if o < 0 {
		return nil // a nil child: no object is touched
	}
	ob := s.Objs[o]
	if ob.Frozen {
		if write {
			return fmt.Errorf("task %d wrote frozen object %d", t, o)
		}
		return nil
	}
	if ob.Owner != t {
		return fmt.Errorf("task %d accessed object %d owned by %d", t, o, ob.Owner)
	}
	return nil
}

// deepCopy copies the objects reachable from o, preserving cycles. Frozen objects are shared,
// not copied; with frozen set, every copy is frozen. The sender reads each object it copies.
func (s *sState) deepCopy(t, o, owner int, frozen bool) (int, error) {
	if o < 0 {
		return -1, nil
	}
	if s.Objs[o].Frozen {
		return o, nil
	}
	if shareShallowCopy && !frozen {
		if err := s.access(t, o, false); err != nil {
			return 0, err
		}
		return s.alloc(sObj{Owner: owner, Child: s.Objs[o].Child}), nil
	}
	copies := map[int]int{}
	var chain []int
	for x := o; x >= 0 && !s.Objs[x].Frozen; x = s.Objs[x].Child {
		if _, ok := copies[x]; ok {
			break
		}
		if err := s.access(t, x, false); err != nil {
			return 0, err
		}
		copies[x] = s.alloc(sObj{Owner: owner, Frozen: frozen, Child: -1})
		chain = append(chain, x)
	}
	for _, x := range chain {
		c := s.Objs[x].Child
		if n, ok := copies[c]; ok {
			c = n
		}
		s.Objs[copies[x]].Child = c
	}
	return copies[o], nil
}

// passValue is the value a task receives by capture or a plain send: Val is copied, Frozen and
// (only under a checker mutation) Ref are shared.
func (s *sState) passValue(t, o int, k SKind, owner int) (int, error) {
	if k == SVal {
		return s.deepCopy(t, o, owner, false)
	}
	return o, nil
}

// step executes task t's next statement. It returns blocked for a receive on an empty channel.
func (s *sState) step(p SProgram, t int) (blocked bool, err error) {
	tk := &s.Tasks[t]
	st := p.Bodies[tk.Body][tk.PC]
	v := tk.Vars
	switch st.Op {
	case SAlloc:
		v[st.V] = s.alloc(sObj{Owner: t, Child: -1})
		tk.Kinds[st.V] = st.Kind
	case SChild:
		if err = s.access(t, v[st.V], true); err == nil {
			n := s.alloc(sObj{Owner: t, Child: -1})
			s.Objs[v[st.V]].Child = n
		}
	case SAlias:
		v[st.V] = v[st.W]
		tk.Kinds[st.V] = tk.Kinds[st.W]
	case SStore:
		if err = s.access(t, v[st.V], true); err == nil {
			s.Objs[v[st.V]].Child = v[st.W]
		}
	case SWrite, SRead:
		if tk.Kinds[st.V] == SErr {
			break // the Error method's receiver is its own copy of the boxed value
		}
		err = s.access(t, v[st.V], st.Op == SWrite)
	case SWriteChild, SReadChild:
		if err = s.access(t, v[st.V], false); err == nil {
			err = s.access(t, s.Objs[v[st.V]].Child, st.Op == SWriteChild)
		}
	case SSend:
		var o int
		switch st.Mode {
		case SPlain:
			o, err = s.passValue(t, v[st.V], p.Chans[st.Ch], sInFlight)
		case SCopy:
			o, err = s.deepCopy(t, v[st.V], sInFlight, false)
		case SMove:
			o = v[st.V]
			for _, r := range s.reach(o) {
				if err = s.access(t, r, !s.Objs[r].Frozen); err != nil {
					break
				}
				if !s.Objs[r].Frozen {
					s.Objs[r].Owner = sInFlight
				}
			}
		}
		if err == nil {
			s.Chans[st.Ch] = append(s.Chans[st.Ch], o)
		}
	case SRecv:
		q := s.Chans[st.Ch]
		if len(q) == 0 {
			return true, nil
		}
		o := q[0]
		s.Chans[st.Ch] = q[1:]
		for _, r := range s.reach(o) {
			if s.Objs[r].Owner == sInFlight {
				s.Objs[r].Owner = t
			}
		}
		v[st.V] = o
		tk.Kinds[st.V] = p.Chans[st.Ch]
	case SFreeze:
		v[st.V], err = s.deepCopy(t, v[st.W], -1, true)
		tk.Kinds[st.V] = SFrozen
	case SToErr:
		// The box is immutable and shared by every holder; a value with no references boxes
		// nothing further, a referencing value (accepted only under a checker mutation) keeps
		// W's object reachable from the box, so Error writing through it touches W's object.
		switch tk.Kinds[st.W] {
		case SErr:
			v[st.V] = v[st.W]
		case SRef:
			v[st.V] = s.alloc(sObj{Frozen: true, Child: v[st.W]})
		default:
			v[st.V] = s.alloc(sObj{Frozen: true, Child: -1})
		}
		tk.Kinds[st.V] = SErr
	case SSpawn:
		child := sTask{Body: st.Body, Vars: map[string]int{}, Kinds: map[string]SKind{}}
		n := len(s.Tasks)
		for _, c := range st.Caps {
			var o int
			if o, err = s.passValue(t, v[c], tk.Kinds[c], n); err != nil {
				break
			}
			child.Vars[c] = o
			child.Kinds[c] = tk.Kinds[c]
		}
		if err == nil {
			s.Tasks = append(s.Tasks, child)
			tk = &s.Tasks[t] // the append may have moved the slice
		}
	}
	if err == nil {
		tk.PC++
	}
	return false, err
}
