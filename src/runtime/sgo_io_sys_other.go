// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !(linux && (amd64 || arm64)) && !(darwin && arm64)

// The S-Go file adapters exist only on the S-Go targets (darwin/arm64, linux/amd64,
// linux/arm64); elsewhere every call fails with ENOSYS.

package runtime

const (
	sgoO_RDONLY, sgoO_WRONLY, sgoO_CREAT, sgoO_EXCL, sgoO_TRUNC, sgoO_CLOEXEC     = 0, 1, 0, 0, 0, 0
	sgoO_NOFOLLOW, sgoO_DIRECTORY                                                 = 0, 0
	sgoAT_SYMLINK_NOFOLLOW, sgoAT_REMOVEDIR                                       = 0, 0
	sgoENOENT, sgoEINTR, sgoEEXIST, sgoENOTDIR, sgoEISDIR, sgoELOOP, sgoENOTEMPTY = 2, 4, 17, 20, 21, 40, 39
	sgoAT_FDCWD                                                                   = -100
	sgoENOSYS                                                                     = 38
	sgoO_NONBLOCK, sgoENXIO                                                       = 0, 6
)

type sgoStatBuf [256]byte

func sgoStatID(b *sgoStatBuf) (dev, ino uint64, isDir, isLink bool)         { return }
func sgoOpenat(dirfd int32, path *byte, flags, mode int32) int32            { return -sgoENOSYS }
func sgoFstatat(dirfd int32, path *byte, st *sgoStatBuf, flags int32) int32 { return -sgoENOSYS }
func sgoFstat(fd int32, st *sgoStatBuf) int32                               { return -sgoENOSYS }
func sgoMkdirat(dirfd int32, path *byte, mode int32) int32                  { return -sgoENOSYS }
func sgoUnlinkat(dirfd int32, path *byte, flags int32) int32                { return -sgoENOSYS }
func sgoCloseFD(fd int32)                                                   {}
func sgoFtruncate(fd int32) int32                                           { return -sgoENOSYS }
func sgoClearNonblock(fd int32)                                             {}

func sgoStatSize(b *sgoStatBuf) int64 { return 0 }

const (
	sgoEAGAIN, sgoEINPROGRESS, sgoEPIPE, sgoEADDRINUSE, sgoECONNRESET = -1, -2, -3, -4, -5
	sgoECONNREFUSED, sgoENETUNREACH, sgoEHOSTUNREACH, sgoETIMEDOUT    = -6, -7, -8, -9
	sgoSOL_SOCKET, sgoSO_ERROR, sgoSO_REUSEADDR                       = 0, 0, 0
)

func sgoSockaddr(b *sgoSockaddrBuf, g *sgoNetGrant) uintptr       { return 0 }
func sgoSocket(g *sgoNetGrant) int32                              { return -sgoENOSYS }
func sgoConnectSys(fd int32, sa *sgoSockaddrBuf, n uintptr) int32 { return -sgoENOSYS }
func sgoBind(fd int32, sa *sgoSockaddrBuf, n uintptr) int32       { return -sgoENOSYS }
func sgoListenSys(fd int32, backlog int32) int32                  { return -sgoENOSYS }
func sgoAccept(fd int32) int32                                    { return -sgoENOSYS }
func sgoSetsockoptInt(fd, level, name, v int32) int32             { return -sgoENOSYS }
func sgoGetsockoptInt(fd, level, name int32) (int32, int32)       { return 0, -sgoENOSYS }
func sgoLocalPort(fd int32) (uint16, int32)                       { return 0, -sgoENOSYS }

func sgoReadDirNames(fd int32) ([]string, int32) { return nil, -sgoENOSYS }

func sgoPeerUID(fd int32) (int, int32) { return 0, -sgoENOSYS }
func sgoStatMode(b *sgoStatBuf) uint32 { return 0 }
