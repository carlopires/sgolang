// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssagen

import (
	"math"
	"strings"
	"sync"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/staticdata"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
)

// S-Go checked int arithmetic (SGO-VAL-001). Operations marked SGoChecked by the noder compute the
// two's-complement result and branch to a runtime failure call when the mathematical result is out
// of range, the divisor is zero, or a shift count is outside 0..63. The failure call receives the
// operation's source position ("file:line:col") for the diagnostic record (SGO-ERR-002). A check
// is omitted only when a constant operand proves it cannot fail; SSA passes may remove more only
// by proving the branch dead.

var sgoSyms struct {
	overflow, divideByZero, shiftRange, bounds, sliceBounds, nilDeref *obj.LSym
}

func sgoInitSyms() {
	sgoSyms.overflow = typecheck.LookupRuntimeFunc("sgoOverflow")
	sgoSyms.divideByZero = typecheck.LookupRuntimeFunc("sgoDivideByZero")
	sgoSyms.shiftRange = typecheck.LookupRuntimeFunc("sgoShiftRange")
	sgoSyms.bounds = typecheck.LookupRuntimeFunc("sgoBoundsViolation")
	sgoSyms.sliceBounds = typecheck.LookupRuntimeFunc("sgoSliceBoundsViolation")
	sgoSyms.nilDeref = typecheck.LookupRuntimeFunc("sgoNilDereference")
}

// sgoStringMu serializes creation of position-string symbols. SSA construction runs functions
// concurrently, and staticdata.StringSym is not safe for concurrent definition of one symbol (walk
// emits literal strings early for that reason); the same position occurs in several functions
// when a body is inlined.
var sgoStringMu sync.Mutex

type sgoPanicKey struct {
	fn  *obj.LSym
	pos string
}

// sgoPos is the position of the operation being compiled (base.SGoPos).
func (s *state) sgoPos() string { return base.SGoPos(s.peekPos()) }

// sgoCheck continues if ok is true and otherwise calls fn(position). Failure blocks are shared only
// between checks of the same kind at the same position.
func (s *state) sgoCheck(ok *ssa.Value, fn *obj.LSym) {
	b := s.endBlock()
	b.Kind = ssa.BlockIf
	b.SetControl(ok)
	b.Likely = ssa.BranchLikely
	bNext := s.f.NewBlock(ssa.BlockPlain)
	pos := s.sgoPos()
	key := sgoPanicKey{fn, pos}
	bPanic := s.sgoPanics[key]
	if bPanic == nil {
		if s.sgoPanics == nil {
			s.sgoPanics = make(map[sgoPanicKey]*ssa.Block)
		}
		bPanic = s.f.NewBlock(ssa.BlockPlain)
		s.sgoPanics[key] = bPanic
		s.startBlock(bPanic)
		sgoStringMu.Lock()
		staticdata.StringSym(s.peekPos(), pos)
		sgoStringMu.Unlock()
		str := s.entryNewValue0A(ssa.OpConstString, types.Types[types.TSTRING], ssa.StringToAux(pos))
		s.rtcall(fn, false, nil, str)
	}
	b.AddEdgeTo(bNext)
	b.AddEdgeTo(bPanic)
	s.startBlock(bNext)
}

func sgoConst(v *ssa.Value) (int64, bool) {
	if v.Op == ssa.OpConst64 {
		return v.AuxInt, true
	}
	return 0, false
}

func (s *state) sgoIntType(n ir.Node) *types.Type {
	t := n.Type()
	// int, byte, and rune (int32 in the compiler, SGO-TYPE-008) and named types over them.
	if !t.IsInteger() || !(t.IsSigned() && (t.Size() == 8 || t.Size() == 4) || !t.IsSigned() && t.Size() == 1) {
		base.FatalfAt(n.Pos(), "S-Go checked arithmetic on %v", t)
	}
	return t
}

func (s *state) sgoBool(op ir.Op, t *types.Type, x, y *ssa.Value) *ssa.Value {
	return s.newValue2(s.ssaOp(op, t), types.Types[types.TBOOL], x, y)
}

// sgoBinary computes a checked + - * / % << >> on int.
func (s *state) sgoBinary(n *ir.BinaryExpr, a, b *ssa.Value) *ssa.Value {
	t := s.sgoIntType(n)
	if t.Size() < 8 {
		return s.sgoByteBinary(n, t, a, b)
	}
	if n.Op() == ir.OLSH || n.Op() == ir.ORSH {
		b = s.sgoWiden(b) // a byte shift count
	}
	zero := s.constInt64(t, 0)
	switch n.Op() {
	case ir.OADD, ir.OSUB:
		r := s.newValue2(s.ssaOp(n.Op(), t), t, a, b)
		var x *ssa.Value
		if n.Op() == ir.OADD {
			// Overflow iff a and b have the same sign and r has the other one.
			x = s.newValue2(s.ssaOp(ir.OAND, t), t, s.newValue2(s.ssaOp(ir.OXOR, t), t, a, r), s.newValue2(s.ssaOp(ir.OXOR, t), t, b, r))
		} else {
			// Overflow iff a and b have different signs and r's sign differs from a's.
			x = s.newValue2(s.ssaOp(ir.OAND, t), t, s.newValue2(s.ssaOp(ir.OXOR, t), t, a, b), s.newValue2(s.ssaOp(ir.OXOR, t), t, a, r))
		}
		s.sgoCheck(s.sgoBool(ir.OLE, t, zero, x), sgoSyms.overflow) // x >= 0 (ssaOp has no OGE)
		return r

	case ir.OMUL:
		lo := s.newValue2(s.ssaOp(ir.OMUL, t), t, a, b)
		hi := s.newValue2(ssa.OpHmul64, t, a, b)
		sign := s.newValue2(s.ssaShiftOp(ir.ORSH, t, types.Types[types.TUINT64]), t, lo, s.constInt64(types.Types[types.TUINT64], 63))
		s.sgoCheck(s.sgoBool(ir.OEQ, t, hi, sign), sgoSyms.overflow)
		return lo

	case ir.ODIV, ir.OMOD:
		c, isConst := sgoConst(b)
		if !isConst || c == 0 {
			s.sgoCheck(s.sgoBool(ir.ONE, t, b, zero), sgoSyms.divideByZero)
		}
		if !isConst || c == -1 {
			// MinInt / -1 and MinInt % -1 overflow (the quotient is 2^63).
			bad := s.newValue2(ssa.OpAndB, types.Types[types.TBOOL],
				s.sgoBool(ir.OEQ, t, a, s.constInt64(t, math.MinInt64)),
				s.sgoBool(ir.OEQ, t, b, s.constInt64(t, -1)))
			s.sgoCheck(s.newValue1(ssa.OpNot, types.Types[types.TBOOL], bad), sgoSyms.overflow)
		}
		return s.newValue2(s.ssaOp(n.Op(), t), t, a, b)

	case ir.OLSH, ir.ORSH:
		u := types.Types[types.TUINT64]
		if c, ok := sgoConst(b); !ok || c < 0 || c > 63 {
			// 0 <= count <= 63, as one unsigned comparison.
			s.sgoCheck(s.newValue2(ssa.OpLeq64U, types.Types[types.TBOOL], b, s.constInt64(u, 63)), sgoSyms.shiftRange)
		}
		r := s.newValue2(s.ssaShiftOp(n.Op(), t, u), t, a, b)
		if n.Op() == ir.OLSH {
			// a << c overflows iff shifting back does not restore a.
			back := s.newValue2(s.ssaShiftOp(ir.ORSH, t, u), t, r, b)
			s.sgoCheck(s.sgoBool(ir.OEQ, t, back, a), sgoSyms.overflow)
		}
		return r
	}
	base.FatalfAt(n.Pos(), "S-Go checked arithmetic: unexpected %v", n.Op())
	return nil
}

// sgoNeg computes a checked unary - on int: -MinInt overflows.
func (s *state) sgoNeg(n *ir.UnaryExpr, a *ssa.Value) *ssa.Value {
	t := s.sgoIntType(n)
	if t.Size() < 8 {
		// -b is in 0..255, and -r in 0..0x10FFFF, only for 0 (SGO-TYPE-007, SGO-TYPE-008).
		u := types.Types[types.TUINT64]
		s.sgoCheck(s.newValue2(ssa.OpEq64, types.Types[types.TBOOL], s.sgoWiden(a), s.constInt64(u, 0)), sgoSyms.overflow)
		return a
	}
	s.sgoCheck(s.sgoBool(ir.ONE, t, a, s.constInt64(t, math.MinInt64)), sgoSyms.overflow)
	return s.newValue1(s.ssaOp(ir.ONEG, t), t, a)
}

// sgoSource reports whether the code being compiled at the current position comes from an S-Go
// source file (.sgo). Inlined bodies keep their source positions, so this holds for S-Go code
// inlined anywhere.
func (s *state) sgoSource() bool {
	return strings.HasSuffix(base.Ctxt.PosTable.Pos(s.peekPos()).Filename(), ".sgo")
}

// sgoBoundsFail ends the current block with the S-Go bounds failure (SGO-COMP-003): a call of
// runtime.sgoBoundsViolation(position, index, length) that does not return.
func (s *state) sgoBoundsFail(kind ssa.BoundsKind, idx, len *ssa.Value) {
	fn := sgoSyms.sliceBounds
	if kind == ssa.BoundsIndex || kind == ssa.BoundsIndexU {
		fn = sgoSyms.bounds
	}
	s.rtcall(fn, false, nil, s.sgoPosValue(), idx, len)
}

// sgoPosValue is the position of the operation being compiled as a string value.
func (s *state) sgoPosValue() *ssa.Value {
	pos := s.sgoPos()
	sgoStringMu.Lock()
	staticdata.StringSym(s.peekPos(), pos)
	sgoStringMu.Unlock()
	return s.entryNewValue0A(ssa.OpConstString, types.Types[types.TSTRING], ssa.StringToAux(pos))
}

// sgoNilCheck checks a pointer before it is dereferenced. In S-Go code a nil pointer is the checked
// failure NilDereference with the source position (SGO-COMP-007); elsewhere it is Go's nil check.
func (s *state) sgoNilCheck(p *ssa.Value) *ssa.Value {
	if !s.sgoSource() || base.Debug.DisableNil != 0 || s.curfn.NilCheckDisabled() {
		return s.nilCheck(p)
	}
	ok := s.newValue1(ssa.OpIsNonNil, types.Types[types.TBOOL], p)
	s.sgoCheck(ok, sgoSyms.nilDeref)
	return p
}

// sgoWiden zero-extends a byte or rune operand to 64 bits (both are never negative); 64-bit
// operands are returned as they are.
func (s *state) sgoWiden(v *ssa.Value) *ssa.Value {
	switch v.Type.Size() {
	case 1:
		return s.newValue1(ssa.OpZeroExt8to64, types.Types[types.TUINT64], v)
	case 4:
		return s.newValue1(ssa.OpZeroExt32to64, types.Types[types.TUINT64], v)
	}
	return v
}

// sgoByteBinary computes a checked + - * / % << >> on byte (SGO-TYPE-007) or rune (SGO-TYPE-008):
// the operation is done on 64-bit values, which cannot overflow for these operands, and the result
// must lie in 0..255 (byte) or 0..0x10FFFF (rune).
func (s *state) sgoByteBinary(n *ir.BinaryExpr, t *types.Type, a, b *ssa.Value) *ssa.Value {
	u := types.Types[types.TUINT64]
	bl := types.Types[types.TBOOL]
	wa, wb := s.sgoWiden(a), s.sgoWiden(b)
	limit, maxShift := int64(255), int64(7)
	if t.Size() == 4 {
		limit, maxShift = sgoMaxRune, 20
	}
	max := s.constInt64(u, limit)
	var r *ssa.Value
	switch n.Op() {
	case ir.OADD, ir.OSUB, ir.OMUL:
		r = s.newValue2(s.ssaOp(n.Op(), u), u, wa, wb)
		s.sgoCheck(s.newValue2(ssa.OpLeq64U, bl, r, max), sgoSyms.overflow) // a negative difference wraps above 255
	case ir.ODIV, ir.OMOD:
		s.sgoCheck(s.newValue2(ssa.OpNeq64, bl, wb, s.constInt64(u, 0)), sgoSyms.divideByZero)
		r = s.newValue2(s.ssaOp(n.Op(), u), u, wa, wb)
	case ir.OLSH, ir.ORSH:
		s.sgoCheck(s.newValue2(ssa.OpLeq64U, bl, wb, s.constInt64(u, 63)), sgoSyms.shiftRange)
		r = s.newValue2(s.ssaShiftOp(n.Op(), u, u), u, wa, wb)
		if n.Op() == ir.OLSH {
			// In range iff a is 0, or the count is at most maxShift (so the 64-bit shift cannot
			// lose bits) and the result at most the limit.
			ok := s.newValue2(ssa.OpOrB, bl, s.newValue2(ssa.OpEq64, bl, wa, s.constInt64(u, 0)),
				s.newValue2(ssa.OpAndB, bl, s.newValue2(ssa.OpLeq64U, bl, wb, s.constInt64(u, maxShift)), s.newValue2(ssa.OpLeq64U, bl, r, max)))
			s.sgoCheck(ok, sgoSyms.overflow)
		}
	default:
		base.FatalfAt(n.Pos(), "S-Go checked byte arithmetic: unexpected %v", n.Op())
	}
	if t.Size() == 4 {
		return s.newValue1(ssa.OpTrunc64to32, t, r)
	}
	return s.newValue1(ssa.OpTrunc64to8, t, r)
}

// sgoMaxRune is the largest rune value (SGO-TYPE-008).
const sgoMaxRune = 0x10FFFF

// sgoConvCheck checks a narrowing integer conversion in S-Go code: byte(i) of an int and byte(r)
// of a rune must lie in 0..255 (SGO-TYPE-007), rune(i) of an int in 0..0x10FFFF (SGO-TYPE-008).
// Each is one unsigned comparison of the zero- or sign-extended operand: a negative int compares
// above every limit. Widening conversions (int(b), int(r), rune(b)) need no check.
func (s *state) sgoConvCheck(from, to *types.Type, x *ssa.Value) {
	if !from.IsInteger() || !to.IsInteger() || to.Size() >= from.Size() {
		return
	}
	var limit int64
	switch {
	case !to.IsSigned() && to.Size() == 1:
		limit = 255
	case to.IsSigned() && to.Size() == 4:
		limit = sgoMaxRune
	default:
		return
	}
	s.sgoCheck(s.newValue2(ssa.OpLeq64U, types.Types[types.TBOOL], s.sgoWiden(x), s.constInt64(types.Types[types.TUINT64], limit)), sgoSyms.overflow)
}
