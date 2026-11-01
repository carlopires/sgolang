// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// libc trampolines of the S-Go file adapters (sgo_io_sys_darwin.go). Each returns the result, or
// the negated errno when libc returns -1.

#include "go_asm.h"
#include "textflag.h"

// openat(dirfd int32 @0, path @8, flags int32 @16, mode int32 @20); mode is variadic: on the stack.
TEXT runtime·sgo_openat_trampoline(SB),NOSPLIT,$0
	SUB	$16, RSP
	MOVW	20(R0), R3
	MOVW	R3, (RSP)
	MOVW	16(R0), R2
	MOVD	8(R0), R1
	MOVW	0(R0), R0
	BL	libc_openat(SB)
	ADD	$16, RSP
	MOVD	$-1, R1
	CMP	R0, R1
	BNE	noerr
	BL	libc_error(SB)
	MOVW	(R0), R0
	NEG	R0, R0
noerr:
	RET

// fstatat(dirfd int32 @0, path @8, st @16, flags int32 @24)
TEXT runtime·sgo_fstatat_trampoline(SB),NOSPLIT,$0
	MOVW	24(R0), R3
	MOVD	16(R0), R2
	MOVD	8(R0), R1
	MOVW	0(R0), R0
	BL	libc_fstatat(SB)
	MOVD	$-1, R1
	CMP	R0, R1
	BNE	noerr
	BL	libc_error(SB)
	MOVW	(R0), R0
	NEG	R0, R0
noerr:
	RET

// fstat(fd int32 @0, st @8)
TEXT runtime·sgo_fstat_trampoline(SB),NOSPLIT,$0
	MOVD	8(R0), R1
	MOVW	0(R0), R0
	BL	libc_fstat(SB)
	MOVD	$-1, R1
	CMP	R0, R1
	BNE	noerr
	BL	libc_error(SB)
	MOVW	(R0), R0
	NEG	R0, R0
noerr:
	RET

// mkdirat(dirfd int32 @0, path @8, mode int32 @16)
TEXT runtime·sgo_mkdirat_trampoline(SB),NOSPLIT,$0
	MOVW	16(R0), R2
	MOVD	8(R0), R1
	MOVW	0(R0), R0
	BL	libc_mkdirat(SB)
	MOVD	$-1, R1
	CMP	R0, R1
	BNE	noerr
	BL	libc_error(SB)
	MOVW	(R0), R0
	NEG	R0, R0
noerr:
	RET

// unlinkat(dirfd int32 @0, path @8, flags int32 @16)
TEXT runtime·sgo_unlinkat_trampoline(SB),NOSPLIT,$0
	MOVW	16(R0), R2
	MOVD	8(R0), R1
	MOVW	0(R0), R0
	BL	libc_unlinkat(SB)
	MOVD	$-1, R1
	CMP	R0, R1
	BNE	noerr
	BL	libc_error(SB)
	MOVW	(R0), R0
	NEG	R0, R0
noerr:
	RET
