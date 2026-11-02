// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

// Executable reference model of the deterministic lab (spec 0.4.0, D40-D49, W08 step 1): the
// baton scheduler over task scripts, virtual time, the step quota, the generator streams, and the
// canonical event stream that the runtime must reproduce exactly for the same choices
// (sgo/docs/w08-plan.md). labstep.go executes one step, labrun.go runs and replays, labexplore.go
// enumerates every choice sequence of a bounded scenario.

import (
	"fmt"
	"math/bits"
	"strings"
)

// LKind is an operation of a lab task script.
type LKind int

const (
	LWork       LKind = iota // N steps of computation (loop back-edges)
	LSleep                   // Sleep(N ms) on virtual time; returns the cancellation error if cancelled
	LSend                    // ch <- N
	LRecv                    // v, ok := <-ch
	LClose                   // close(ch)
	LSelect                  // select over Arms (and default if Default)
	LWrite                   // out.WriteString(Text)
	LCheckpoint              // return the cancellation error if cancelled
	LCancel                  // n.Cancel() of the nursery (root only)
	LError                   // return sgo.NewError(Text)
)

// LArm is a select arm: a send of Val, or a receive, on channel Ch.
type LArm struct {
	Send bool
	Ch   int
	Val  int
}

// LOp is one operation.
type LOp struct {
	Kind    LKind
	N       int
	Ch      int
	Text    string
	Arms    []LArm
	Default bool
}

// LScenario is a bounded lab program: main (task 0) opens one nursery, spawns one child per entry
// of Tasks (task IDs 1..n, in order), runs Root, and joins the nursery. K is the yield period
// (steps per scheduling slice), Quota the bound on the total steps, Policy "random" or "fifo".
type LScenario struct {
	Caps   []int
	Root   []LOp
	Tasks  [][]LOp
	K      int
	Quota  int
	Policy string
}

// LEvent is one canonical event (sgo/docs/w08-plan.md). Fields not used by a kind are zero.
type LEvent struct {
	Kind    string
	Ready   []int
	Pick    int
	Task    int
	Arms    []int
	Arm     int
	To      int64
	Timer   int
	Text    string
	Outcome string
	Status  int
}

// Line is the canonical JSONL encoding of e, field order fixed per kind.
func (e LEvent) Line() string {
	switch e.Kind {
	case "choose":
		return fmt.Sprintf(`{"e":"choose","ready":%s,"pick":%d}`, ints(e.Ready), e.Pick)
	case "select":
		return fmt.Sprintf(`{"e":"select","task":%d,"arms":%s,"arm":%d}`, e.Task, ints(e.Arms), e.Arm)
	case "advance":
		return fmt.Sprintf(`{"e":"advance","to":%d}`, e.To)
	case "fire":
		return fmt.Sprintf(`{"e":"fire","task":%d,"timer":%d}`, e.Task, e.Timer)
	case "write":
		return fmt.Sprintf(`{"e":"write","task":%d,"text":%s}`, e.Task, jsonString(e.Text))
	case "end":
		return fmt.Sprintf(`{"e":"end","task":%d,"outcome":%s}`, e.Task, jsonString(e.Outcome))
	case "outcome":
		return fmt.Sprintf(`{"e":"outcome","status":%d,"kind":%s}`, e.Status, jsonString(e.Outcome))
	}
	return "invalid event " + jsonString(e.Text) // a line the parser could not read
}

func ints(v []int) string {
	s := make([]string, len(v))
	for i, x := range v {
		s[i] = fmt.Sprint(x)
	}
	return "[" + strings.Join(s, ",") + "]"
}

// jsonString quotes s as a JSON string with the escapes the runtime's record writer uses.
func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\t':
			b.WriteString(`\t`)
		case c == '\r':
			b.WriteString(`\r`)
		case c < 0x20:
			fmt.Fprintf(&b, `\u%04x`, c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// PCG is the PCG-DXSM generator with 128-bit state, the algorithm of math/rand/v2's PCG and of
// sgo.Random (D26, D44).
type PCG struct{ Hi, Lo uint64 }

// Uint64 is math/rand/v2 (*PCG).Uint64.
func (p *PCG) Uint64() uint64 {
	const (
		mulHi = 2549297995355413924
		mulLo = 4865540595714422341
		incHi = 6364136223846793005
		incLo = 1442695040888963407
	)
	hi, lo := bits.Mul64(p.Lo, mulLo)
	hi += p.Hi*mulLo + p.Lo*mulHi
	lo, c := bits.Add64(lo, incLo, 0)
	hi, _ = bits.Add64(hi, incHi, c)
	p.Lo, p.Hi = lo, hi
	const cheapMul = 0xda942042e4dd58b5
	hi ^= hi >> 32
	hi *= cheapMul
	hi ^= hi >> (3 * 16)
	hi *= lo | 1
	return hi
}

// Uint64n is a uniform value in [0, n), n > 0, by the unbiased method of math/rand/v2.
func (p *PCG) Uint64n(n uint64) uint64 {
	if n&(n-1) == 0 {
		return p.Uint64() & (n - 1)
	}
	hi, lo := bits.Mul64(p.Uint64(), n)
	if lo < n {
		thresh := -n % n
		for lo < thresh {
			hi, lo = bits.Mul64(p.Uint64(), n)
		}
	}
	return hi
}

// SplitMix64 is the frozen stream-derivation step (D44): Steele, Lea, and Flood's SplitMix64.
func SplitMix64(x *uint64) uint64 {
	*x += 0x9e3779b97f4a7c15
	z := *x
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// Streams are the lab's independent generators, derived from the master seed: six SplitMix64
// outputs, in order, seed the sched, fault, and data streams as (Hi, Lo) pairs.
type Streams struct{ Sched, Fault, Data PCG }

// NewStreams derives the streams of seed.
func NewStreams(seed uint64) Streams {
	x := seed
	var s Streams
	s.Sched = PCG{SplitMix64(&x), SplitMix64(&x)}
	s.Fault = PCG{SplitMix64(&x), SplitMix64(&x)}
	s.Data = PCG{SplitMix64(&x), SplitMix64(&x)}
	return s
}
