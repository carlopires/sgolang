// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin && arm64

// Socket system calls of the S-Go network adapters on darwin (W09, D55, D57): libc through the
// runtime's variadic libc helper. Each returns the result, or the negated errno.

package runtime

import (
	"internal/abi"
	"unsafe"
)

const (
	sgoAF_UNIX      = 1
	sgoAF_INET      = 2
	sgoAF_INET6     = 30
	sgoSOCK_STREAM  = 1
	sgoSOCK_DGRAM   = 2
	sgoSOL_SOCKET   = 0xffff
	sgoSO_REUSEADDR = 0x4
	sgoSO_ERROR     = 0x1007

	sgoEPIPE        = 32
	sgoEAGAIN       = 35
	sgoEINPROGRESS  = 36
	sgoEADDRINUSE   = 48
	sgoENETUNREACH  = 51
	sgoECONNRESET   = 54
	sgoETIMEDOUT    = 60
	sgoECONNREFUSED = 61
	sgoEHOSTUNREACH = 65
)

// sgoLibc calls libc function fn. Like the syscall package's wrappers it cannot grow the stack, so
// a stack address passed as uintptr stays valid for the call.
//
//go:nosplit
//go:uintptrkeepalive
func sgoLibc(fn uintptr, args ...uintptr) int32 {
	r, _, errno := syscall_rawsyscalln(fn, args...)
	if int32(r) == -1 {
		return -int32(errno)
	}
	return int32(r)
}

// sgoSockaddr fills b with the sockaddr_un, sockaddr_in, or sockaddr_in6 of g (BSD: length
// byte, family byte) and returns its length.
func sgoSockaddr(b *sgoSockaddrBuf, g *sgoNetGrant) uintptr {
	*b = sgoSockaddrBuf{}
	if g.unix {
		n := 2 + len(g.path) + 1
		b[0], b[1] = byte(n), sgoAF_UNIX
		copy(b[2:], g.path)
		return uintptr(n)
	}
	ip, v6, port := &g.ip, g.v6, g.port
	b[2], b[3] = byte(port>>8), byte(port)
	if !v6 {
		b[0], b[1] = 16, sgoAF_INET
		copy(b[4:8], ip[12:16])
		return 16
	}
	b[0], b[1] = 28, sgoAF_INET6
	copy(b[8:24], ip[:])
	return 28
}

func sgoSocket(g *sgoNetGrant) int32 { return sgoSocketType(g, sgoSOCK_STREAM) }

// sgoSocketType opens a non-blocking, close-on-exec socket of type typ for g's address family.
func sgoSocketType(g *sgoNetGrant, typ uintptr) int32 {
	af := uintptr(sgoAF_INET)
	switch {
	case g.unix:
		af = sgoAF_UNIX
	case g.v6:
		af = sgoAF_INET6
	}
	fd := sgoLibc(abi.FuncPCABI0(sgo_socket_trampoline), af, typ, 0)
	if fd >= 0 {
		closeonexec(fd)
		setNonblock(fd)
	}
	return fd
}

func sgoConnectSys(fd int32, sa *sgoSockaddrBuf, n uintptr) int32 {
	r := sgoLibc(abi.FuncPCABI0(sgo_connect_trampoline), uintptr(fd), uintptr(unsafe.Pointer(sa)), n)
	KeepAlive(sa)
	return r
}

func sgoBind(fd int32, sa *sgoSockaddrBuf, n uintptr) int32 {
	r := sgoLibc(abi.FuncPCABI0(sgo_bind_trampoline), uintptr(fd), uintptr(unsafe.Pointer(sa)), n)
	KeepAlive(sa)
	return r
}

func sgoListenSys(fd int32, backlog int32) int32 {
	return sgoLibc(abi.FuncPCABI0(sgo_listen_trampoline), uintptr(fd), uintptr(backlog))
}

func sgoAccept(fd int32) int32 {
	nfd := sgoLibc(abi.FuncPCABI0(sgo_accept_trampoline), uintptr(fd), 0, 0)
	if nfd >= 0 {
		closeonexec(nfd)
		setNonblock(nfd)
	}
	return nfd
}

func sgoSetsockoptInt(fd, level, name, v int32) int32 {
	r := sgoLibc(abi.FuncPCABI0(sgo_setsockopt_trampoline), uintptr(fd), uintptr(level), uintptr(name), uintptr(unsafe.Pointer(&v)), 4)
	KeepAlive(&v)
	return r
}

func sgoGetsockoptInt(fd, level, name int32) (int32, int32) {
	var v, n int32 = 0, 4
	r := sgoLibc(abi.FuncPCABI0(sgo_getsockopt_trampoline), uintptr(fd), uintptr(level), uintptr(name), uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&n)))
	KeepAlive(&v)
	KeepAlive(&n)
	return v, r
}

// sgoLocalPort is the port fd is bound to.
func sgoLocalPort(fd int32) (uint16, int32) {
	var b [28]byte
	n := uint32(len(b))
	r := sgoLibc(abi.FuncPCABI0(sgo_getsockname_trampoline), uintptr(fd), uintptr(unsafe.Pointer(&b)), uintptr(unsafe.Pointer(&n)))
	KeepAlive(&b)
	KeepAlive(&n)
	return uint16(b[2])<<8 | uint16(b[3]), r
}

func sgo_socket_trampoline()
func sgo_connect_trampoline()
func sgo_bind_trampoline()
func sgo_listen_trampoline()
func sgo_accept_trampoline()
func sgo_setsockopt_trampoline()
func sgo_getsockopt_trampoline()
func sgo_getsockname_trampoline()
func sgo_getpeereid_trampoline()

// sgoPeerUID is the user ID of the process at the other end of Unix socket fd (getpeereid).
func sgoPeerUID(fd int32) (int, int32) {
	var uid, gid uint32
	r := sgoLibc(abi.FuncPCABI0(sgo_getpeereid_trampoline), uintptr(fd), uintptr(unsafe.Pointer(&uid)), uintptr(unsafe.Pointer(&gid)))
	KeepAlive(&uid)
	KeepAlive(&gid)
	if r < 0 {
		return 0, r
	}
	return int(uid), 0
}

// sgoStatMode is st_mode (uint16 at 4).
func sgoStatMode(b *sgoStatBuf) uint32 { return uint32(*(*uint16)(unsafe.Pointer(&b[4]))) }

//go:cgo_import_dynamic libc_socket socket "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_connect connect "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_bind bind "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_listen listen "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_accept accept "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_setsockopt setsockopt "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_getsockopt getsockopt "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_getsockname getsockname "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_getpeereid getpeereid "/usr/lib/libSystem.B.dylib"

// sgoReadDirNames returns the names in directory fd (closing fd), without "." and "..", or a
// negated errno. libc's fdopendir takes fd; readdir_r fills a struct dirent (d_namlen uint16 at
// 18, d_name at 21 on darwin/arm64).
func sgoReadDirNames(fd int32) ([]string, int32) {
	// The buffers are on the heap: syscall_syscalln may grow the stack before it enters the
	// system call, so their addresses must not be stack addresses. syscall_syscalln tells the
	// scheduler about each call (W12 R4).
	dir, _, errno := syscall_syscalln(abi.FuncPCABI0(sgo_fdopendir_trampoline), uintptr(fd))
	if dir == 0 {
		sgoCloseFD(fd)
		return nil, -int32(errno)
	}
	var names []string
	ent := new([1048]byte)
	res := new(uintptr)
	r := int32(0)
	for {
		e, _, _ := syscall_syscalln(abi.FuncPCABI0(sgo_readdir_r_trampoline), dir, uintptr(unsafe.Pointer(&ent[0])), uintptr(unsafe.Pointer(res)))
		if e != 0 {
			r = -int32(e)
			break
		}
		if *res == 0 {
			break
		}
		n := int(*(*uint16)(unsafe.Pointer(&ent[18])))
		if name := string(ent[21 : 21+n]); name != "." && name != ".." {
			names = append(names, name)
		}
	}
	KeepAlive(ent)
	KeepAlive(res)
	syscall_syscalln(abi.FuncPCABI0(sgo_closedir_trampoline), dir)
	return names, r
}

func sgo_fdopendir_trampoline()
func sgo_readdir_r_trampoline()
func sgo_closedir_trampoline()

//go:cgo_import_dynamic libc_fdopendir fdopendir "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_readdir_r readdir_r "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_closedir closedir "/usr/lib/libSystem.B.dylib"
