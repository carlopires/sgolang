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
	sgoAF_INET      = 2
	sgoAF_INET6     = 30
	sgoSOCK_STREAM  = 1
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

func sgoLibc(fn uintptr, args ...uintptr) int32 {
	r, _, errno := syscall_rawsyscalln(fn, args...)
	if int32(r) == -1 {
		return -int32(errno)
	}
	return int32(r)
}

// sgoSockaddr fills b with a sockaddr_in or sockaddr_in6 (BSD: length byte, family byte) and
// returns its length.
func sgoSockaddr(b *[28]byte, ip *[16]byte, v6 bool, port uint16) uintptr {
	*b = [28]byte{}
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

func sgoSocket(v6 bool) int32 {
	af := uintptr(sgoAF_INET)
	if v6 {
		af = sgoAF_INET6
	}
	fd := sgoLibc(abi.FuncPCABI0(sgo_socket_trampoline), af, sgoSOCK_STREAM, 0)
	if fd >= 0 {
		closeonexec(fd)
		setNonblock(fd)
	}
	return fd
}

func sgoConnectSys(fd int32, sa *[28]byte, n uintptr) int32 {
	r := sgoLibc(abi.FuncPCABI0(sgo_connect_trampoline), uintptr(fd), uintptr(unsafe.Pointer(sa)), n)
	KeepAlive(sa)
	return r
}

func sgoBind(fd int32, sa *[28]byte, n uintptr) int32 {
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

//go:cgo_import_dynamic libc_socket socket "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_connect connect "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_bind bind "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_listen listen "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_accept accept "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_setsockopt setsockopt "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_getsockopt getsockopt "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_getsockname getsockname "/usr/lib/libSystem.B.dylib"
