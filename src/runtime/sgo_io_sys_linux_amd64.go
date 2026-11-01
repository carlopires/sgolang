// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "unsafe"

const (
	sgoSYS_getdents64  = 217
	sgoSYS_socket      = 41
	sgoSYS_connect     = 42
	sgoSYS_accept4     = 288
	sgoSYS_bind        = 49
	sgoSYS_listen      = 50
	sgoSYS_setsockopt  = 54
	sgoSYS_getsockopt  = 55
	sgoSYS_getsockname = 51
	sgoSYS_openat      = 257
	sgoSYS_fstatat     = 262 // newfstatat
	sgoSYS_mkdirat     = 258
	sgoSYS_unlinkat    = 263

	sgoO_NOFOLLOW  = 0x20000
	sgoO_DIRECTORY = 0x10000
)

// sgoStatID: st_dev uint64 at 0, st_ino uint64 at 8, st_nlink uint64 at 16, st_mode uint32 at 24.
func sgoStatID(b *sgoStatBuf) (dev, ino uint64, isDir, isLink bool) {
	dev = *(*uint64)(unsafe.Pointer(&b[0]))
	ino = *(*uint64)(unsafe.Pointer(&b[8]))
	mode := *(*uint32)(unsafe.Pointer(&b[24]))
	return dev, ino, mode&0xF000 == 0x4000, mode&0xF000 == 0xA000
}
