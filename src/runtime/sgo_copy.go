// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

// sgo.Copy (SGO-COMP-004, D89): a deep copy of a task-local value, so that a value crossing to
// another task shares no mutable memory with the sender's.
//
// The walk is depth first in field, element, and insertion order. Slices, maps, and pointed-to
// variables are copied once each: references to the same object (pointers of the same type to the
// same variable, a map, or slices of the same element type with the same first element and
// capacity) refer to one copied object in the result, and cycles are preserved. References of
// different types at one address (a struct and its first field, an array and its first element)
// are copied separately (W13 review V1): each copy is allocated at its own type and size.
// Strings and channels are shared: they are immutable or synchronized. An interface value (an
// error) is shared when its dynamic type belongs to the runtime or package sgo, and otherwise its
// dynamic value is copied (defence in depth: the compiler admits only shareable user types as
// errors, review V2).

import (
	"internal/abi"
	"internal/goarch"
	"unsafe"
)

type sgoCopier struct {
	seen map[sgoCopyKey]unsafe.Pointer
}

type sgoCopyKey struct {
	p   unsafe.Pointer
	t   *_type // the object's type (a slice's element type, a map's type)
	cap int    // slices: the capacity; other objects: -1
}

// sgo_copy is sgo.Copy for the dynamic type of v.
//
//go:linkname sgo_copy sgo.runtime_copy
func sgo_copy(v any) any {
	e := efaceOf(&v)
	t := e._type
	if t == nil || !t.Pointers() {
		return v
	}
	c := &sgoCopier{}
	if t.IsDirectIface() {
		// The data word is the value itself (a pointer, map, or channel).
		var out unsafe.Pointer
		c.copyValue(t, unsafe.Pointer(&out), unsafe.Pointer(&e.data))
		var r any
		re := efaceOf(&r)
		re._type, re.data = t, out
		return r
	}
	dst := mallocgc(t.Size_, t, true)
	c.copyValue(t, dst, e.data)
	var r any
	re := efaceOf(&r)
	re._type, re.data = t, dst
	return r
}

// copyValue writes into dst (zeroed, of type t) a deep copy of the value at src.
func (c *sgoCopier) copyValue(t *_type, dst, src unsafe.Pointer) {
	if !t.Pointers() {
		memmove(dst, src, t.Size_)
		return
	}
	switch t.Kind() {
	case abi.Interface:
		c.copyIface(t, dst, src)
	case abi.String, abi.Chan, abi.Func, abi.UnsafePointer:
		typedmemmove(t, dst, src) // shared: immutable or synchronized (or not task-local)
	case abi.Struct:
		st := (*structtype)(unsafe.Pointer(t))
		for i := range st.Fields {
			f := &st.Fields[i]
			c.copyValue(f.Typ, add(dst, f.Offset), add(src, f.Offset))
		}
	case abi.Array:
		at := (*arraytype)(unsafe.Pointer(t))
		for i := uintptr(0); i < at.Len; i++ {
			c.copyValue(at.Elem, add(dst, i*at.Elem.Size_), add(src, i*at.Elem.Size_))
		}
	case abi.Pointer:
		p := *(*unsafe.Pointer)(src)
		*(*unsafe.Pointer)(dst) = c.copyObject((*ptrtype)(unsafe.Pointer(t)).Elem, p)
	case abi.Slice:
		s := (*slice)(src)
		d := (*slice)(dst)
		d.len, d.cap = s.len, s.cap
		if s.array == nil {
			return
		}
		et := (*slicetype)(unsafe.Pointer(t)).Elem
		k := sgoCopyKey{s.array, et, s.cap}
		if q, ok := c.seen[k]; ok {
			d.array = q
			return
		}
		q := newarray(et, s.cap)
		c.remember(k, q)
		for i := 0; i < s.cap; i++ {
			c.copyValue(et, add(q, uintptr(i)*et.Size_), add(s.array, uintptr(i)*et.Size_))
		}
		d.array = q
	case abi.Map:
		m := *(**sgoMap)(src)
		if m == nil {
			return
		}
		k := sgoCopyKey{unsafe.Pointer(m), t, -1}
		if q, ok := c.seen[k]; ok {
			*(*unsafe.Pointer)(dst) = q
			return
		}
		mt := (*abi.MapType)(unsafe.Pointer(t))
		n := sgomakemap(mt, int(m.used))
		c.remember(k, unsafe.Pointer(n))
		key := newobject(mt.Key)
		for s := 0; s < m.n; s++ {
			if m.seqs[s] == 0 {
				continue
			}
			typedmemclr(mt.Key, key)
			c.copyValue(mt.Key, key, m.key(mt, s))
			e := sgomapassign(mt, n, key, "")
			c.copyValue(mt.Elem, e, m.elem(mt, s))
		}
		*(*unsafe.Pointer)(dst) = unsafe.Pointer(n)
	default:
		typedmemmove(t, dst, src)
	}
}

// copyObject copies the variable of type t at p (once per distinct p).
func (c *sgoCopier) copyObject(t *_type, p unsafe.Pointer) unsafe.Pointer {
	if p == nil {
		return nil
	}
	k := sgoCopyKey{p, t, -1}
	if q, ok := c.seen[k]; ok {
		return q
	}
	q := mallocgc(t.Size_, t, true)
	c.remember(k, q)
	c.copyValue(t, q, p)
	return q
}

// copyIface copies an interface value (of interface type t): the dynamic value is shared when its
// type has no pointers or belongs to the runtime or package sgo (errors made by sgo.NewError,
// cancellation, runtime failures), and deep-copied otherwise.
func (c *sgoCopier) copyIface(t *_type, dst, src unsafe.Pointer) {
	var dt *_type
	var data unsafe.Pointer
	if len((*interfacetype)(unsafe.Pointer(t)).Methods) == 0 {
		e := (*eface)(src)
		dt, data = e._type, e.data
	} else {
		i := (*iface)(src)
		if i.tab != nil {
			dt = i.tab.Type
		}
		data = i.data
	}
	if dt == nil || !dt.Pointers() {
		typedmemmove(t, dst, src)
		return
	}
	switch toRType(dt).pkgpath() {
	case "runtime", "sgo", "errors":
		typedmemmove(t, dst, src)
		return
	}
	var out unsafe.Pointer
	if dt.IsDirectIface() {
		c.copyValue(dt, unsafe.Pointer(&out), unsafe.Pointer(&data))
	} else {
		out = mallocgc(dt.Size_, dt, true)
		c.copyValue(dt, out, data)
	}
	typedmemmove(t, dst, src) // the type word (and itab)
	*(*unsafe.Pointer)(add(dst, goarch.PtrSize)) = out
}

func (c *sgoCopier) remember(k sgoCopyKey, q unsafe.Pointer) {
	if c.seen == nil {
		c.seen = make(map[sgoCopyKey]unsafe.Pointer)
	}
	c.seen[k] = q
}
