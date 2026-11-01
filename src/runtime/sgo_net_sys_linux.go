// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && (amd64 || arm64)

// Socket system calls of the S-Go network adapters on linux (W09, D55, D57): raw system calls that
// return the result, or the negated errno.

package runtime

import "unsafe"

const (
	sgoAF_UNIX      = 1
	sgoAF_INET      = 2
	sgoAF_INET6     = 10
	sgoSO_PEERCRED  = 17
	sgoSOCK_STREAM  = 1
	sgoSOCK_NB_CX   = 0x800 | 0x80000 // SOCK_NONBLOCK | SOCK_CLOEXEC
	sgoSOL_SOCKET   = 1
	sgoSO_REUSEADDR = 2
	sgoSO_ERROR     = 4

	sgoEAGAIN       = 11
	sgoEPIPE        = 32
	sgoEADDRINUSE   = 98
	sgoENETUNREACH  = 101
	sgoECONNRESET   = 104
	sgoETIMEDOUT    = 110
	sgoECONNREFUSED = 111
	sgoEHOSTUNREACH = 113
	sgoEINPROGRESS  = 115
)

// sgoSockaddr fills b with the sockaddr_un, sockaddr_in, or sockaddr_in6 of g and returns its
// length.
func sgoSockaddr(b *sgoSockaddrBuf, g *sgoNetGrant) uintptr {
	*b = sgoSockaddrBuf{}
	if g.unix {
		b[0] = sgoAF_UNIX
		copy(b[2:], g.path)
		return uintptr(2 + len(g.path) + 1)
	}
	ip, v6, port := &g.ip, g.v6, g.port
	b[2], b[3] = byte(port>>8), byte(port)
	if !v6 {
		b[0] = sgoAF_INET
		copy(b[4:8], ip[12:16])
		return 16
	}
	b[0] = sgoAF_INET6
	copy(b[8:24], ip[:])
	return 28
}

func sgoSocket(g *sgoNetGrant) int32 {
	af := uintptr(sgoAF_INET)
	switch {
	case g.unix:
		af = sgoAF_UNIX
	case g.v6:
		af = sgoAF_INET6
	}
	return sgoSys(sgoSYS_socket, af, sgoSOCK_STREAM|sgoSOCK_NB_CX, 0, 0)
}

func sgoConnectSys(fd int32, sa *sgoSockaddrBuf, n uintptr) int32 {
	r := sgoSys(sgoSYS_connect, uintptr(fd), uintptr(unsafe.Pointer(sa)), n, 0)
	KeepAlive(sa)
	return r
}

func sgoBind(fd int32, sa *sgoSockaddrBuf, n uintptr) int32 {
	r := sgoSys(sgoSYS_bind, uintptr(fd), uintptr(unsafe.Pointer(sa)), n, 0)
	KeepAlive(sa)
	return r
}

func sgoListenSys(fd int32, backlog int32) int32 {
	return sgoSys(sgoSYS_listen, uintptr(fd), uintptr(backlog), 0, 0)
}

func sgoAccept(fd int32) int32 {
	return sgoSys(sgoSYS_accept4, uintptr(fd), 0, 0, sgoSOCK_NB_CX)
}

func sgoSetsockoptInt(fd, level, name, v int32) int32 {
	r, _, errno := sgoSyscall6(sgoSYS_setsockopt, uintptr(fd), uintptr(level), uintptr(name), uintptr(unsafe.Pointer(&v)), 4, 0)
	KeepAlive(&v)
	if errno != 0 {
		return -int32(errno)
	}
	return int32(r)
}

func sgoGetsockoptInt(fd, level, name int32) (int32, int32) {
	var v, n int32 = 0, 4
	r, _, errno := sgoSyscall6(sgoSYS_getsockopt, uintptr(fd), uintptr(level), uintptr(name), uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&n)), 0)
	KeepAlive(&v)
	KeepAlive(&n)
	if errno != 0 {
		return 0, -int32(errno)
	}
	return v, int32(r)
}

// sgoLocalPort is the port fd is bound to.
func sgoLocalPort(fd int32) (uint16, int32) {
	var b [28]byte
	n := uint32(len(b))
	r := sgoSys(sgoSYS_getsockname, uintptr(fd), uintptr(unsafe.Pointer(&b)), uintptr(unsafe.Pointer(&n)), 0)
	KeepAlive(&b)
	KeepAlive(&n)
	return uint16(b[2])<<8 | uint16(b[3]), r
}

// sgoReadDirNames returns the names in directory fd (closing fd), without "." and "..", or a
// negated errno: getdents64 records (d_reclen uint16 at 16, d_name NUL-terminated at 19).
func sgoReadDirNames(fd int32) ([]string, int32) {
	defer sgoCloseFD(fd)
	var names []string
	buf := make([]byte, 8192)
	for {
		n := sgoSys(sgoSYS_getdents64, uintptr(fd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
		if n == -sgoEINTR {
			continue
		}
		if n <= 0 {
			return names, min(n, 0)
		}
		for off := 0; off < int(n); {
			reclen := int(*(*uint16)(unsafe.Pointer(&buf[off+16])))
			end := off + 19
			for buf[end] != 0 {
				end++
			}
			if name := string(buf[off+19 : end]); name != "." && name != ".." {
				names = append(names, name)
			}
			off += reclen
		}
	}
}

// sgoPeerUID is the user ID of the process at the other end of Unix socket fd (SO_PEERCRED:
// struct ucred {pid, uid, gid int32}).
func sgoPeerUID(fd int32) (int, int32) {
	var cred [3]int32
	n := int32(len(cred) * 4)
	_, _, errno := sgoSyscall6(sgoSYS_getsockopt, uintptr(fd), sgoSOL_SOCKET, sgoSO_PEERCRED, uintptr(unsafe.Pointer(&cred)), uintptr(unsafe.Pointer(&n)), 0)
	KeepAlive(&cred)
	KeepAlive(&n)
	if errno != 0 {
		return 0, -int32(errno)
	}
	return int(uint32(cred[1])), 0
}
