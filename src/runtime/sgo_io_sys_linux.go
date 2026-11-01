// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && (amd64 || arm64)

// System calls of the S-Go file adapters on linux (W09, D53): raw system calls that return the
// result, or the negated errno on failure.

package runtime

import (
	"internal/runtime/syscall/linux"
	"unsafe"
)

const (
	sgoO_RDONLY  = 0x0
	sgoO_WRONLY  = 0x1
	sgoO_CREAT   = 0x40
	sgoO_EXCL    = 0x80
	sgoO_TRUNC   = 0x200
	sgoO_CLOEXEC = 0x80000

	sgoAT_FDCWD            = -100
	sgoAT_SYMLINK_NOFOLLOW = 0x100
	sgoAT_REMOVEDIR        = 0x200
	sgoAT_EMPTY_PATH       = 0x1000

	sgoENOENT    = 2
	sgoEINTR     = 4
	sgoEEXIST    = 17
	sgoENOTDIR   = 20
	sgoEISDIR    = 21
	sgoELOOP     = 40
	sgoENOTEMPTY = 39
)

// sgoStatBuf holds a struct stat (144 bytes on linux/amd64, 128 on linux/arm64).
type sgoStatBuf [256]byte

func sgoSys(num, a1, a2, a3, a4 uintptr) int32 {
	r, _, errno := linux.Syscall6(num, a1, a2, a3, a4, 0, 0)
	if errno != 0 {
		return -int32(errno)
	}
	return int32(r)
}

func sgoOpenat(dirfd int32, path *byte, flags, mode int32) int32 {
	r := sgoSys(sgoSYS_openat, uintptr(int64(dirfd)), uintptr(unsafe.Pointer(path)), uintptr(flags), uintptr(mode))
	KeepAlive(path)
	return r
}

func sgoFstatat(dirfd int32, path *byte, st *sgoStatBuf, flags int32) int32 {
	r := sgoSys(sgoSYS_fstatat, uintptr(int64(dirfd)), uintptr(unsafe.Pointer(path)), uintptr(unsafe.Pointer(st)), uintptr(flags))
	KeepAlive(path)
	KeepAlive(st)
	return r
}

func sgoFstat(fd int32, st *sgoStatBuf) int32 {
	empty := [1]byte{}
	return sgoFstatat(fd, &empty[0], st, sgoAT_EMPTY_PATH)
}

func sgoMkdirat(dirfd int32, path *byte, mode int32) int32 {
	r := sgoSys(sgoSYS_mkdirat, uintptr(int64(dirfd)), uintptr(unsafe.Pointer(path)), uintptr(mode), 0)
	KeepAlive(path)
	return r
}

func sgoUnlinkat(dirfd int32, path *byte, flags int32) int32 {
	r := sgoSys(sgoSYS_unlinkat, uintptr(int64(dirfd)), uintptr(unsafe.Pointer(path)), uintptr(flags), 0)
	KeepAlive(path)
	return r
}

// sgoCloseFD closes fd.
func sgoCloseFD(fd int32) { closefd(fd) }

// sgoStatSize is st_size (int64 at 48 on linux/amd64 and linux/arm64).
func sgoStatSize(b *sgoStatBuf) int64 { return *(*int64)(unsafe.Pointer(&b[48])) }
