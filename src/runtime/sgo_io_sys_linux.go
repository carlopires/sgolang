// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && (amd64 || arm64)

// System calls of the S-Go file adapters on linux (W09, D53): raw system calls that return the
// result, or the negated errno on failure.

package runtime

import (
	"internal/runtime/atomic"
	"internal/runtime/syscall/linux"
	"unsafe"
)

const (
	sgoO_RDONLY   = 0x0
	sgoO_WRONLY   = 0x1
	sgoO_CREAT    = 0x40
	sgoO_EXCL     = 0x80
	sgoO_TRUNC    = 0x200
	sgoO_CLOEXEC  = 0x80000
	sgoO_NONBLOCK = 0x800
	sgoO_PATH     = 0x200000

	sgoF_GETFL = 3
	sgoF_SETFL = 4

	sgoAT_FDCWD            = -100
	sgoAT_SYMLINK_NOFOLLOW = 0x100
	sgoAT_REMOVEDIR        = 0x200
	sgoAT_EMPTY_PATH       = 0x1000

	sgoENOENT    = 2
	sgoENXIO     = 6
	sgoEINTR     = 4
	sgoEEXIST    = 17
	sgoENOTDIR   = 20
	sgoEISDIR    = 21
	sgoELOOP     = 40
	sgoENOTEMPTY = 39
)

// sgoStatBuf holds a struct stat (144 bytes on linux/amd64, 128 on linux/arm64).
type sgoStatBuf [256]byte

// sgoSyscall6 and sgoSys cannot grow the stack, so a stack address passed as uintptr stays valid.
//
//go:nosplit
//go:uintptrkeepalive
func sgoSyscall6(num, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2, errno uintptr) {
	return linux.Syscall6(num, a1, a2, a3, a4, a5, a6)
}

// sgoSys tells the scheduler about the call (entersyscall): a file system call can block for
// long (a FIFO, NFS, FUSE), and a blocked call holding its P would stall stop-the-world and every
// other task (W12 R4). No caller holds a runtime lock.
//
//go:nosplit
//go:uintptrkeepalive
func sgoSys(num, a1, a2, a3, a4 uintptr) int32 {
	entersyscall()
	r, _, errno := linux.Syscall6(num, a1, a2, a3, a4, 0, 0)
	exitsyscall()
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

// sgoOpenHow is struct open_how (flags, mode, resolve) of openat2.
type sgoOpenHow struct{ flags, mode, resolve uint64 }

const (
	sgoRESOLVE_NO_MAGICLINKS = 0x02
	sgoRESOLVE_NO_SYMLINKS   = 0x04
	sgoRESOLVE_BENEATH       = 0x08
	sgoEAGAIN_RESOLVE        = 11 // EAGAIN: a concurrent rename raced the resolution
	sgoEXDEV                 = 18
)

// sgoNoOpenat2 is true when openat2 is unavailable. It is decided once, by sgoOpenat2Probe, with an
// open that cannot fail for file-specific reasons (W12 R6b): an EPERM from a real open (an
// immutable or append-only file) is that open's error, never a reason to stop using openat2.
// SGoForceWalk sets it in tests. Tasks read it concurrently.
var (
	sgoNoOpenat2     atomic.Bool
	sgoOpenat2Probed atomic.Bool
)

// sgoOpenat2Probe decides availability: openat2(dirfd, ".", O_PATH, RESOLVE_BENEATH) fails only
// with ENOSYS (kernel before 5.6) or EPERM (a seccomp filter blocking the call).
func sgoOpenat2Probe(dirfd int32) {
	dot := [2]byte{'.', 0}
	how := &sgoOpenHow{flags: sgoO_PATH | sgoO_CLOEXEC, resolve: sgoRESOLVE_BENEATH}
	r := sgoSys(sgoSYS_openat2, uintptr(int64(dirfd)), uintptr(unsafe.Pointer(&dot[0])), uintptr(unsafe.Pointer(how)), unsafe.Sizeof(*how))
	KeepAlive(how)
	if r >= 0 {
		sgoCloseFD(r)
	} else if r == -38 || r == -1 { // ENOSYS, EPERM
		sgoNoOpenat2.Store(true)
	}
	sgoOpenat2Probed.Store(true)
}

// sgoOpenat2Calls counts openat2 opens (not probes); sgoOpenat2Fault, when non-zero, is returned
// once instead of the next open's result. Both are for runtime tests only (W12 R6b).
var (
	sgoOpenat2Calls atomic.Int64
	sgoOpenat2Fault atomic.Int32
)

func sgoOpenBeneath(dirfd int32, path *byte, flags, mode int32) (fd int32, ok bool) {
	if !sgoOpenat2Probed.Load() {
		sgoOpenat2Probe(dirfd)
	}
	if sgoNoOpenat2.Load() {
		return 0, false
	}
	if flags&sgoO_CREAT == 0 {
		mode = 0
	}
	wantDir := flags&sgoO_DIRECTORY != 0
	flags &^= sgoO_DIRECTORY
	how := &sgoOpenHow{flags: uint64(uint32(flags)), mode: uint64(mode),
		resolve: sgoRESOLVE_BENEATH | sgoRESOLVE_NO_SYMLINKS | sgoRESOLVE_NO_MAGICLINKS}
	r := sgoSys(sgoSYS_openat2, uintptr(int64(dirfd)), uintptr(unsafe.Pointer(path)), uintptr(unsafe.Pointer(how)), unsafe.Sizeof(*how))
	KeepAlive(path)
	KeepAlive(how)
	sgoOpenat2Calls.Add(1)
	if f := sgoOpenat2Fault.Swap(0); f != 0 {
		if r >= 0 {
			sgoCloseFD(r)
		}
		r = -f
	}
	if r >= 0 && wantDir {
		var st sgoStatBuf
		if sgoFstat(r, &st) < 0 || sgoStatMode(&st)&0xF000 != 0x4000 {
			sgoCloseFD(r)
			return -sgoENOTDIR, true
		}
	}
	return r, true
}

// sgoFtruncate truncates fd to zero length.
func sgoFtruncate(fd int32) int32 { return sgoSys(sgoSYS_ftruncate, uintptr(fd), 0, 0, 0) }

// sgoClearNonblock clears O_NONBLOCK on fd.
func sgoClearNonblock(fd int32) {
	if fl, _ := fcntl(fd, sgoF_GETFL, 0); fl >= 0 {
		fcntl(fd, sgoF_SETFL, fl&^sgoO_NONBLOCK)
	}
}

// sgoCloseFD closes fd.
func sgoCloseFD(fd int32) int32 {
	// EINTR leaves the descriptor closed on these systems: not a failed close.
	if r := closefd(fd); r < 0 && r != -_EINTR {
		return r
	}
	return 0
}

// sgoStatSize is st_size (int64 at 48 on linux/amd64 and linux/arm64).
func sgoStatSize(b *sgoStatBuf) int64 { return *(*int64)(unsafe.Pointer(&b[48])) }

// Terminal ioctls (D178): TCGETS reads the termios, TIOCGWINSZ the window size.
const (
	sgoTIOCGETA   = 0x5401
	sgoTIOCGWINSZ = 0x5413
)

// sgoIoctl is ioctl(fd, req, arg); it returns 0 or a negated errno.
func sgoIoctl(fd int32, req uintptr, arg unsafe.Pointer) int32 {
	r := sgoSys(sgoSYS_ioctl, uintptr(int64(fd)), req, uintptr(arg), 0)
	KeepAlive(arg)
	return r
}
