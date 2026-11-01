// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin && arm64

// System calls of the S-Go file adapters on darwin (W09, D53): libc trampolines that return the
// result, or the negated errno on failure (sgo_io_sys_darwin_arm64.s).

package runtime

import (
	"internal/abi"
	"unsafe"
)

const (
	sgoO_RDONLY    = 0x0
	sgoO_WRONLY    = 0x1
	sgoO_CREAT     = 0x200
	sgoO_TRUNC     = 0x400
	sgoO_EXCL      = 0x800
	sgoO_NOFOLLOW  = 0x100
	sgoO_DIRECTORY = 0x100000
	sgoO_CLOEXEC   = 0x1000000

	sgoAT_FDCWD            = -2
	sgoAT_SYMLINK_NOFOLLOW = 0x20
	sgoAT_REMOVEDIR        = 0x80

	sgoENOENT    = 2
	sgoEINTR     = 4
	sgoEEXIST    = 17
	sgoENOTDIR   = 20
	sgoEISDIR    = 21
	sgoELOOP     = 62
	sgoENOTEMPTY = 66
)

// sgoStatBuf holds a struct stat (144 bytes on darwin/arm64).
type sgoStatBuf [256]byte

// sgoStatID returns the device and inode of a stat buffer, and whether it is a directory or a
// symlink (st_dev int32 at 0, st_mode uint16 at 4, st_ino uint64 at 8).
func sgoStatID(b *sgoStatBuf) (dev, ino uint64, isDir, isLink bool) {
	dev = uint64(*(*uint32)(unsafe.Pointer(&b[0])))
	mode := *(*uint16)(unsafe.Pointer(&b[4]))
	ino = *(*uint64)(unsafe.Pointer(&b[8]))
	return dev, ino, mode&0xF000 == 0x4000, mode&0xF000 == 0xA000
}

//go:nosplit
//go:cgo_unsafe_args
func sgoOpenat(dirfd int32, path *byte, flags, mode int32) int32 {
	ret := libcCall(unsafe.Pointer(abi.FuncPCABI0(sgo_openat_trampoline)), unsafe.Pointer(&dirfd))
	KeepAlive(path)
	return ret
}
func sgo_openat_trampoline()

//go:nosplit
//go:cgo_unsafe_args
func sgoFstatat(dirfd int32, path *byte, st *sgoStatBuf, flags int32) int32 {
	ret := libcCall(unsafe.Pointer(abi.FuncPCABI0(sgo_fstatat_trampoline)), unsafe.Pointer(&dirfd))
	KeepAlive(path)
	KeepAlive(st)
	return ret
}
func sgo_fstatat_trampoline()

//go:nosplit
//go:cgo_unsafe_args
func sgoFstat(fd int32, st *sgoStatBuf) int32 {
	ret := libcCall(unsafe.Pointer(abi.FuncPCABI0(sgo_fstat_trampoline)), unsafe.Pointer(&fd))
	KeepAlive(st)
	return ret
}
func sgo_fstat_trampoline()

//go:nosplit
//go:cgo_unsafe_args
func sgoMkdirat(dirfd int32, path *byte, mode int32) int32 {
	ret := libcCall(unsafe.Pointer(abi.FuncPCABI0(sgo_mkdirat_trampoline)), unsafe.Pointer(&dirfd))
	KeepAlive(path)
	return ret
}
func sgo_mkdirat_trampoline()

//go:nosplit
//go:cgo_unsafe_args
func sgoUnlinkat(dirfd int32, path *byte, flags int32) int32 {
	ret := libcCall(unsafe.Pointer(abi.FuncPCABI0(sgo_unlinkat_trampoline)), unsafe.Pointer(&dirfd))
	KeepAlive(path)
	return ret
}
func sgo_unlinkat_trampoline()

// sgoCloseFD closes fd.
func sgoCloseFD(fd int32) { closefd(fd) }

//go:cgo_import_dynamic libc_openat openat "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_fstatat fstatat "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_fstat fstat "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_mkdirat mkdirat "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_unlinkat unlinkat "/usr/lib/libSystem.B.dylib"

// sgoStatSize is st_size (int64 at 96: after the four timespecs).
func sgoStatSize(b *sgoStatBuf) int64 { return *(*int64)(unsafe.Pointer(&b[96])) }
