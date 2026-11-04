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
func runtime_dirOpen(task, nurs unsafe.Pointer, g int, path string, write bool) (unsafe.Pointer, error)
func runtime_dirReadFile(g int, path string) (string, error)
func runtime_dirList(g int, path string) ([]string, error)
func runtime_dirWriteFile(g int, path, data string) error
func runtime_read(h unsafe.Pointer, max int) (string, error)
func runtime_ioWrite(h unsafe.Pointer, s string) (int, error)
func runtime_fileSize(h unsafe.Pointer) (int, error)
func runtime_close(h unsafe.Pointer, op string) error
func runtime_resMove(h, task, nurs unsafe.Pointer) error
func runtime_resOwn(task, nurs, key unsafe.Pointer, release func(bool) error)
func runtime_resMoveUser(key, task, nurs unsafe.Pointer) error
func runtime_currentIsRoot() bool
func runtime_dial(task, nurs unsafe.Pointer, addr string) (unsafe.Pointer, error)
func runtime_listen(task, nurs unsafe.Pointer, addr string) (unsafe.Pointer, error)
func runtime_accept(h, task, nurs unsafe.Pointer) (unsafe.Pointer, error)
func runtime_peerUID(h unsafe.Pointer) (int, error)
func runtime_dialerNew(task, nurs unsafe.Pointer, endpoints []string) unsafe.Pointer
func runtime_dialerDial(h, task, nurs unsafe.Pointer, addr string) (unsafe.Pointer, error)
func runtime_dialerResolve(h unsafe.Pointer, host, family string) ([]string, error)
func runtime_readLine(h unsafe.Pointer, max int) (string, error)
func runtime_readAll(h unsafe.Pointer, max int) (string, error)
func runtime_isTooLong(err error) bool

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

// IsTooLong reports whether err is a ReadLine or ReadAll that met more than its max bytes; it
// consumed nothing (D173).
func IsTooLong(err error) bool { return runtime_isTooLong(err) }

// IsEOF reports whether err is the end of a file or connection.
func IsEOF(err error) bool {
	_, _, e := runtime_ioInfo(err)
	return e
}

// Dir is a directory grant, or a part of one: paths are resolved from the granted directory,
// component by component, never following symlinks (D53).
type Dir struct {
	g        int
	sub      string // prefix of every path ("" or "a/b/")
	readOnly bool
}

// Dir returns the root handle of the directory grant name; name must be declared in sgo.mod.
func (c *Context) Dir(name string) *Dir {
	c.own()
	if !runtime_isRoot(c.t) {
		runtime_fail("MissingCapability", "Dir: only the root task holds the grants; children use a handle the root passes to them")
	}
	g, ro, ok := runtime_dirGrant(name)
	if !ok {
		runtime_fail("MissingCapability", "Dir: no directory grant named "+quote(name))
	}
	return &Dir{g: g, readOnly: ro}
}

func (d *Dir) path(op, p string) string {
	return d.sub + p
}

// Open opens path for reading, as a resource of scope s (SGO-RES-001).
func (d *Dir) Open(s Scope, path string) (*File, error) {
	task, nurs := s.sgoScope()
	h, err := runtime_dirOpen(task, nurs, d.g, d.path("Dir.Open", path), false)
	if err != nil {
		return nil, err
	}
	return &File{h}, nil
}

// Create creates or truncates path for writing, as a resource of scope s; a read-only handle does
// not permit it.
func (d *Dir) Create(s Scope, path string) (*File, error) {
	p := d.path("Dir.Create", path)
	if d.readOnly {
		runtime_fail("MissingCapability", "Dir.Create on a read-only handle")
	}
	task, nurs := s.sgoScope()
	h, err := runtime_dirOpen(task, nurs, d.g, p, true)
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
	return &Dir{g: d.g, sub: p + "/", readOnly: d.readOnly}, nil
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
func (d *Dir) ReadOnly() *Dir { return &Dir{g: d.g, sub: d.sub, readOnly: true} }

// File is an open file, a resource of the scope it was opened in (SGO-RES-001).
type File struct{ h unsafe.Pointer }

// Read reads at most max bytes: at least one, or an error (IsEOF at the end of the file).
func (f *File) Read(max int) (string, error) { return runtime_read(f.h, max) }

// ReadLine returns the next line without its "\n" and one "\r" before it (D164, D173). The line
// with its terminator is at most max bytes, else the error is TooLong and nothing is consumed; a
// last line without "\n" is returned at end of file, and EOF comes only when nothing is left.
func (f *File) ReadLine(max int) (string, error) { return runtime_readLine(f.h, max) }

// ReadAll returns the rest of the file, at most max bytes, else the error is TooLong and nothing
// is consumed (D164, D173).
func (f *File) ReadAll(max int) (string, error) { return runtime_readAll(f.h, max) }

// Write writes all of s, or returns an error with the bytes written (Progress).
func (f *File) Write(s string) (int, error) { return runtime_ioWrite(f.h, s) }

// Size returns the size of the file in bytes.
func (f *File) Size() (int, error) { return runtime_fileSize(f.h) }

// Close releases the file early (SGO-RES-002).
func (f *File) Close() error { return runtime_close(f.h, "File.Close") }

// MoveTo makes f a resource of s, a scope enclosing f's scope (SGO-RES-003).
func (f *File) MoveTo(s Scope) error { return moveTo(f.h, s) }

// Conn is a connection, a resource of the scope its Dial or Accept named (SGO-RES-001).
type Conn struct{ h unsafe.Pointer }

// Dial connects to addr (IP:PORT, or unix:PATH for a Unix domain socket), which a connect grant
// must name exactly (D57, D60). The connection is a resource of the root task's body.
func (c *Context) Dial(addr string) (*Conn, error) {
	c.own()
	return dial(c, "Dial", addr)
}

// Dial is c.Dial into nursery n, a nursery of the root task (SGO-RES-001).
func (n *Nursery) Dial(addr string) (*Conn, error) { return dial(n, "Dial", addr) }

func dial(s Scope, op, addr string) (*Conn, error) {
	rootOnly(op)
	task, nurs := s.sgoScope()
	h, err := runtime_dial(task, nurs, addr)
	if err != nil {
		return nil, err
	}
	return &Conn{h}, nil
}

// rootOnly fails the task unless it is the root task, which alone holds the grants.
func rootOnly(op string) {
	if !runtime_currentIsRoot() {
		runtime_fail("MissingCapability", op+": only the root task holds the grants; children use the resources of the scopes they run in")
	}
}

// Read reads at most max bytes: at least one, or an error (IsEOF when the peer has closed).
func (c *Conn) Read(max int) (string, error) { return runtime_read(c.h, max) }

// ReadLine returns the next line without its "\n" and one "\r" before it (D164, D173). The line
// with its terminator is at most max bytes, else the error is TooLong and nothing is consumed; a
// last line without "\n" is returned when the peer closes, and EOF comes only when nothing is
// left. Reads of one connection run one at a time; bytes read past the line wait for the next.
func (c *Conn) ReadLine(max int) (string, error) { return runtime_readLine(c.h, max) }

// ReadAll returns what the peer sends until it closes its side, at most max bytes, else the
// error is TooLong and nothing is consumed (D164, D173).
func (c *Conn) ReadAll(max int) (string, error) { return runtime_readAll(c.h, max) }

// Write writes all of s, or returns an error with the bytes written (Progress).
func (c *Conn) Write(s string) (int, error) { return runtime_ioWrite(c.h, s) }

// PeerUID returns the user ID of the process at the other end of a Unix domain socket
// connection; on a TCP connection it returns an Unsupported error (D60).
func (c *Conn) PeerUID() (int, error) { return runtime_peerUID(c.h) }

// Close releases the connection early (SGO-RES-002).
func (c *Conn) Close() error { return runtime_close(c.h, "Conn.Close") }

// MoveTo makes c a resource of s, a scope enclosing c's scope (SGO-RES-003).
func (c *Conn) MoveTo(s Scope) error { return moveTo(c.h, s) }

// Dialer is delegated connect authority (D143): a resource for a fixed set of endpoints the main
// module grants. The root task creates it; every task inside its scope may dial, and each
// connection is a resource of the scope its Dial names.
type Dialer struct{ h unsafe.Pointer }

// Dialer returns a dialer for endpoints, each of which a connect grant must name exactly, as a
// resource of the root task's body. Only the root task holds the grants.
func (c *Context) Dialer(endpoints []string) *Dialer {
	c.own()
	return dialer(c, endpoints)
}

// Dialer is c.Dialer into nursery n, a nursery of the root task.
func (n *Nursery) Dialer(endpoints []string) *Dialer { return dialer(n, endpoints) }

func dialer(s Scope, endpoints []string) *Dialer {
	rootOnly("Dialer")
	task, nurs := s.sgoScope()
	return &Dialer{runtime_dialerNew(task, nurs, endpoints)}
}

// Resolve looks up host, a host name one of the dialer's endpoints names (grant connect
// NAME:PORT, D145), for family "ip4" or "ip6", and returns an IP:PORT for each address found and
// each such endpoint's port, in answer order; none is not an error. The dialer may dial the
// returned addresses from then on. Waiting for the answer is a cancellation point.
func (d *Dialer) Resolve(host, family string) ([]string, error) {
	return runtime_dialerResolve(d.h, host, family)
}

// Dial connects to addr, one of the dialer's IP or Unix endpoints or an address its Resolve
// returned; the connection is a resource of scope s. The caller runs inside the dialer's scope.
// Waiting for the connection is a cancellation point.
func (d *Dialer) Dial(s Scope, addr string) (*Conn, error) {
	task, nurs := s.sgoScope()
	h, err := runtime_dialerDial(d.h, task, nurs, addr)
	if err != nil {
		return nil, err
	}
	return &Conn{h}, nil
}

// Close releases the dialer early (SGO-RES-002).
func (d *Dialer) Close() error { return runtime_close(d.h, "Dialer.Close") }

// MoveTo makes d a resource of s, a scope enclosing d's scope (SGO-RES-003).
func (d *Dialer) MoveTo(s Scope) error { return moveTo(d.h, s) }

// Listener accepts TCP connections on a granted endpoint.
type Listener struct{ h unsafe.Pointer }

// Listen listens on addr (IP:PORT, or unix:PATH), which a listen grant must name exactly (D57,
// D60). A stale socket file at PATH is removed first; any other file there is AddressInUse.
func (c *Context) Listen(addr string) (*Listener, error) {
	c.own()
	return listen(c, addr)
}

// Listen is c.Listen into nursery n, a nursery of the root task.
func (n *Nursery) Listen(addr string) (*Listener, error) { return listen(n, addr) }

func listen(s Scope, addr string) (*Listener, error) {
	rootOnly("Listen")
	task, nurs := s.sgoScope()
	h, err := runtime_listen(task, nurs, addr)
	if err != nil {
		return nil, err
	}
	return &Listener{h}, nil
}

// Accept waits for a connection, a resource of scope s.
func (l *Listener) Accept(s Scope) (*Conn, error) {
	task, nurs := s.sgoScope()
	h, err := runtime_accept(l.h, task, nurs)
	if err != nil {
		return nil, err
	}
	return &Conn{h}, nil
}

// Close releases the listener early (SGO-RES-002).
func (l *Listener) Close() error { return runtime_close(l.h, "Listener.Close") }

// MoveTo makes l a resource of s, a scope enclosing l's scope (SGO-RES-003).
func (l *Listener) MoveTo(s Scope) error { return moveTo(l.h, s) }

// Scope is a scope resources are acquired into (SGO-RES-001): a task body, named by its
// *Context, or a nursery, named by its *Nursery. The scope releases its resources when it ends.
type Scope interface {
	sgoScope() (task, nurs unsafe.Pointer)
}

func (c *Context) sgoScope() (task, nurs unsafe.Pointer) { return c.t, nil }
func (n *Nursery) sgoScope() (task, nurs unsafe.Pointer) { return nil, n.s }

func moveTo(h unsafe.Pointer, s Scope) error {
	task, nurs := s.sgoScope()
	return runtime_resMove(h, task, nurs)
}

// Releaser is a user resource (SGO-RES-004): Release(ok) runs when its scope ends, with ok true
// when the scope ends normally and false when it unwinds (a failure or cancellation).
// Resource marks a resource interface (D176, SGO-RES-006): an interface that embeds Resource,
// directly or through another resource interface, may hold resources, and its values are tied to
// the scope of what they hold, like the resources themselves. No other interface, any and error
// included, ever holds a resource. Resource has no methods.
type Resource interface{}

type Releaser interface {
	Release(ok bool) error
}

// Own makes r a resource of the task body of c (SGO-RES-004).
func (c *Context) Own(r Releaser) { runtime_resOwn(c.t, nil, ownKey(r), r.Release) }

// Own makes r a resource of nursery n (SGO-RES-004).
func (n *Nursery) Own(r Releaser) { runtime_resOwn(nil, n.s, ownKey(r), r.Release) }

// MoveTo moves user resource r, registered with Own, to the enclosing scope s (D158), as the
// built-in handles' MoveTo methods do: the caller runs inside r's scope and s encloses it, else
// MissingCapability with no effect. r is found by identity, so a user resource is a pointer.
func MoveTo(r Releaser, s Scope) error {
	task, nurs := s.sgoScope()
	return runtime_resMoveUser(ownKey(r), task, nurs)
}

// ownKey is the identity of a registered user resource: the interface's data word.
func ownKey(r Releaser) unsafe.Pointer { return (*[2]unsafe.Pointer)(unsafe.Pointer(&r))[1] }

func quote(s string) string {
	if len(s) > 64 {
		s = s[:64] + "..."
	}
	return "\"" + s + "\""
}
