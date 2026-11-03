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
	overflow, divideByZero, shiftRange, bounds *obj.LSym
}

func sgoInitSyms() {
	sgoSyms.overflow = typecheck.LookupRuntimeFunc("sgoOverflow")
	sgoSyms.divideByZero = typecheck.LookupRuntimeFunc("sgoDivideByZero")
	sgoSyms.shiftRange = typecheck.LookupRuntimeFunc("sgoShiftRange")
	sgoSyms.bounds = typecheck.LookupRuntimeFunc("sgoBoundsViolation")
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
	if !t.IsInteger() || !t.IsSigned() || t.Size() != 8 {
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
func (s *state) sgoBoundsFail(idx, len *ssa.Value) {
	pos := s.sgoPos()
	sgoStringMu.Lock()
	staticdata.StringSym(s.peekPos(), pos)
	sgoStringMu.Unlock()
	str := s.entryNewValue0A(ssa.OpConstString, types.Types[types.TSTRING], ssa.StringToAux(pos))
	s.rtcall(sgoSyms.bounds, false, nil, str, idx, len)
}
