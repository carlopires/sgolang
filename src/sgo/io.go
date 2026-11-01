// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// File and network adapters (W09, D52–D59; spec 0.5.0 draft).

package sgo

import "unsafe"

func runtime_ioInfo(err error) (progress int, noEffect, eof bool)
func runtime_dirGrant(name string) (g int, ro, ok bool)
func runtime_dirIsDir(g int, path string) error
func runtime_dirMkdir(g int, path string) error
func runtime_dirRemove(g int, path string) error
func runtime_dirOpen(g int, path string, write bool) (unsafe.Pointer, error)
func runtime_dirReadFile(g int, path string) (string, error)
func runtime_dirList(g int, path string) ([]string, error)
func runtime_dirWriteFile(g int, path, data string) error
func runtime_read(h unsafe.Pointer, max int) (string, error)
func runtime_ioWrite(h unsafe.Pointer, s string) (int, error)
func runtime_fileSize(h unsafe.Pointer) (int, error)
func runtime_close(h unsafe.Pointer, op string) error
func runtime_lend(h, scope unsafe.Pointer)
func runtime_dial(addr string) (unsafe.Pointer, error)
func runtime_listen(addr string) (unsafe.Pointer, error)
func runtime_accept(h unsafe.Pointer) (unsafe.Pointer, error)

// Progress returns the bytes an I/O operation transferred before err (0 for other errors).
func Progress(err error) int {
	p, _, _ := runtime_ioInfo(err)
	return p
}

// NoEffect reports whether err is an I/O error after which nothing was transferred and the
// operation may be retried safely.
func NoEffect(err error) bool {
	_, n, _ := runtime_ioInfo(err)
	return n
}

// IsEOF reports whether err is the end of a file or connection.
func IsEOF(err error) bool {
	_, _, e := runtime_ioInfo(err)
	return e
}

// Dir is a directory grant, or a part of one: paths are resolved from the granted directory,
// component by component, never following symlinks (D53).
type Dir struct {
	l        lent
	g        int
	sub      string // prefix of every path ("" or "a/b/")
	readOnly bool
}

// Dir returns the root handle of the directory grant name; name must be declared in sgo.mod.
func (c *Context) Dir(name string) *Dir {
	c.own()
	if !runtime_isRoot(c.t) {
		runtime_fail("MissingCapability", "Dir: only the root task holds the grants; children use a handle lent to them")
	}
	g, ro, ok := runtime_dirGrant(name)
	if !ok {
		runtime_fail("MissingCapability", "Dir: no directory grant named "+quote(name))
	}
	return &Dir{g: g, readOnly: ro}
}

func (d *Dir) path(op, p string) string {
	d.l.check(op)
	return d.sub + p
}

// Open opens path for reading.
func (d *Dir) Open(path string) (*File, error) {
	h, err := runtime_dirOpen(d.g, d.path("Dir.Open", path), false)
	if err != nil {
		return nil, err
	}
	return &File{h}, nil
}

// Create creates or truncates path for writing; a read-only handle does not permit it.
func (d *Dir) Create(path string) (*File, error) {
	p := d.path("Dir.Create", path)
	if d.readOnly {
		runtime_fail("MissingCapability", "Dir.Create on a read-only handle")
	}
	h, err := runtime_dirOpen(d.g, p, true)
	if err != nil {
		return nil, err
	}
	return &File{h}, nil
}

// ReadFile returns the contents of path (at most 16 MiB).
func (d *Dir) ReadFile(path string) (string, error) {
	return runtime_dirReadFile(d.g, d.path("Dir.ReadFile", path))
}

// WriteFile creates or truncates path and writes data.
func (d *Dir) WriteFile(path, data string) error {
	p := d.path("Dir.WriteFile", path)
	if d.readOnly {
		runtime_fail("MissingCapability", "Dir.WriteFile on a read-only handle")
	}
	return runtime_dirWriteFile(d.g, p, data)
}

// Remove removes the file or empty directory path.
func (d *Dir) Remove(path string) error {
	p := d.path("Dir.Remove", path)
	if d.readOnly {
		runtime_fail("MissingCapability", "Dir.Remove on a read-only handle")
	}
	return runtime_dirRemove(d.g, p)
}

// Mkdir creates the directory path.
func (d *Dir) Mkdir(path string) error {
	p := d.path("Dir.Mkdir", path)
	if d.readOnly {
		runtime_fail("MissingCapability", "Dir.Mkdir on a read-only handle")
	}
	return runtime_dirMkdir(d.g, p)
}

// Sub returns a handle that permits only paths under the directory path.
func (d *Dir) Sub(path string) (*Dir, error) {
	p := d.path("Dir.Sub", path)
	if err := runtime_dirIsDir(d.g, p); err != nil {
		return nil, err
	}
	return &Dir{l: d.l, g: d.g, sub: p + "/", readOnly: d.readOnly}, nil
}

// List returns the names in directory path ("" is d itself), sorted by byte order.
func (d *Dir) List(path string) (*Listing, error) {
	p := d.path("Dir.List", path)
	if path == "" && d.sub != "" {
		p = d.sub[:len(d.sub)-1]
	}
	names, err := runtime_dirList(d.g, p)
	if err != nil {
		return nil, err
	}
	return &Listing{names}, nil
}

// Listing is the result of Dir.List: an immutable sequence of names.
type Listing struct{ names []string }

// Len returns the number of names.
func (l *Listing) Len() int { return len(l.names) }

// Name returns the i-th name, 0 <= i < Len().
func (l *Listing) Name(i int) string {
	if i < 0 || i >= len(l.names) {
		runtime_fail("InvalidArgument", "Listing.Name("+itoa(i)+") with "+itoa(len(l.names))+" names")
	}
	return l.names[i]
}

// ReadOnly returns a handle that permits only Open, ReadFile, List, and Sub.
func (d *Dir) ReadOnly() *Dir { return &Dir{l: d.l, g: d.g, sub: d.sub, readOnly: true} }

// LendTo returns a copy of d that is revoked when n closes.
func (d *Dir) LendTo(n *Nursery) *Dir {
	return &Dir{l: d.l.to(n, "Dir.LendTo"), g: d.g, sub: d.sub, readOnly: d.readOnly}
}

// File is an open file, owned by the task that opened it (D54).
type File struct{ h unsafe.Pointer }

// Read reads at most max bytes: at least one, or an error (IsEOF at the end of the file).
func (f *File) Read(max int) (string, error) { return runtime_read(f.h, max) }

// Write writes all of s, or returns an error with the bytes written (Progress).
func (f *File) Write(s string) (int, error) { return runtime_ioWrite(f.h, s) }

// Size returns the size of the file in bytes.
func (f *File) Size() (int, error) { return runtime_fileSize(f.h) }

// Close closes the file; only its owner may, and not while it is lent to an open nursery.
func (f *File) Close() error { return runtime_close(f.h, "File.Close") }

// LendTo lets the tasks of nursery n use f until n closes; n must be a nursery of f's owner.
func (f *File) LendTo(n *Nursery) *File { runtime_lend(f.h, n.s); return f }

// Conn is a TCP connection, owned by the task that dialed or accepted it.
type Conn struct{ h unsafe.Pointer }

// Dial connects to addr (IP:PORT), which a connect grant must name exactly (D57).
func (c *Context) Dial(addr string) (*Conn, error) {
	c.own()
	if !runtime_isRoot(c.t) {
		runtime_fail("MissingCapability", "Dial: only the root task holds the grants; children use a connection lent to them")
	}
	h, err := runtime_dial(addr)
	if err != nil {
		return nil, err
	}
	return &Conn{h}, nil
}

// Read reads at most max bytes: at least one, or an error (IsEOF when the peer has closed).
func (c *Conn) Read(max int) (string, error) { return runtime_read(c.h, max) }

// Write writes all of s, or returns an error with the bytes written (Progress).
func (c *Conn) Write(s string) (int, error) { return runtime_ioWrite(c.h, s) }

// Close closes the connection.
func (c *Conn) Close() error { return runtime_close(c.h, "Conn.Close") }

// LendTo lets the tasks of nursery n use c until n closes; n must be a nursery of c's owner.
func (c *Conn) LendTo(n *Nursery) *Conn { runtime_lend(c.h, n.s); return c }

// Listener accepts TCP connections on a granted endpoint.
type Listener struct{ h unsafe.Pointer }

// Listen listens on addr (IP:PORT), which a listen grant must name exactly (D57).
func (c *Context) Listen(addr string) (*Listener, error) {
	c.own()
	if !runtime_isRoot(c.t) {
		runtime_fail("MissingCapability", "Listen: only the root task holds the grants; children use a listener lent to them")
	}
	h, err := runtime_listen(addr)
	if err != nil {
		return nil, err
	}
	return &Listener{h}, nil
}

// Accept waits for a connection; the connection is owned by the calling task.
func (l *Listener) Accept() (*Conn, error) {
	h, err := runtime_accept(l.h)
	if err != nil {
		return nil, err
	}
	return &Conn{h}, nil
}

// Close closes the listener.
func (l *Listener) Close() error { return runtime_close(l.h, "Listener.Close") }

// LendTo lets the tasks of nursery n use l until n closes; n must be a nursery of l's owner.
func (l *Listener) LendTo(n *Nursery) *Listener { runtime_lend(l.h, n.s); return l }

func quote(s string) string {
	if len(s) > 64 {
		s = s[:64] + "..."
	}
	return "\"" + s + "\""
}
