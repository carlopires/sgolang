// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "unsafe"

const (
	sgoSYS_getdents64  = 61
	sgoSYS_socket      = 198
	sgoSYS_connect     = 203
	sgoSYS_accept4     = 242
	sgoSYS_bind        = 200
	sgoSYS_listen      = 201
	sgoSYS_setsockopt  = 208
	sgoSYS_getsockopt  = 209
	sgoSYS_getsockname = 204
	sgoSYS_openat      = 56
	sgoSYS_fstatat     = 79 // newfstatat
	sgoSYS_mkdirat     = 34
	sgoSYS_unlinkat    = 35

	sgoO_NOFOLLOW  = 0x8000
	sgoO_DIRECTORY = 0x4000
)

// sgoStatID: st_dev uint64 at 0, st_ino uint64 at 8, st_mode uint32 at 16.
func sgoStatID(b *sgoStatBuf) (dev, ino uint64, isDir, isLink bool) {
	dev = *(*uint64)(unsafe.Pointer(&b[0]))
	ino = *(*uint64)(unsafe.Pointer(&b[8]))
	mode := *(*uint32)(unsafe.Pointer(&b[16]))
	return dev, ino, mode&0xF000 == 0x4000, mode&0xF000 == 0xA000
}

// sgoStatMode is st_mode (uint32 at 16).
func sgoStatMode(b *sgoStatBuf) uint32 { return *(*uint32)(unsafe.Pointer(&b[16])) }
