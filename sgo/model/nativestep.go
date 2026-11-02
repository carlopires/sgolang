// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import "fmt"

// nSteps returns every enabled successor of s.
func nSteps(sc NScenario, s *nState) []*nState {
	var out []*nState
	for i := range s.tasks {
		t := s.tasks[i]
		if t.outcome == "" && i < len(sc.Cancellable) && sc.Cancellable[i] && !t.cancelled {
			n := s.clone()
			n.tasks[i].cancelled = true
			n.trace = append(n.trace, fmt.Sprintf("t%d cancelled", i))
			out = append(out, n)
		}
		op, ok := s.op(sc, i)
		if !ok {
			continue
		}
		alone := s.aloneSteps(sc, i, op)
		joint := s.jointSteps(sc, i, op)
		out = append(out, alone...)
		out = append(out, joint...)
		ready := len(alone)+len(joint) > 0
		switch {
		case op.Kind == NSelect && op.Default && !ready:
			n := s.clone()
			n.advance(sc, i, "select default")
			out = append(out, n)
		case !ready && !t.waiting && t.cancelled:
			// It would wait while cancelled: the unwind (D33).
			out = append(out, s.unwind(i))
		case !ready && !t.waiting:
			n := s.clone()
			n.tasks[i].waiting = true
			if op.Kind == NAcquire {
				n.lwait[op.Ch] = append(n.lwait[op.Ch], i)
			}
			n.trace = append(n.trace, fmt.Sprintf("t%d waits", i))
			out = append(out, n)
		case t.waiting && t.cancelled:
			out = append(out, s.unwind(i))
		}
	}
	return out
}

func (s *nState) unwind(i int) *nState {
	n := s.clone()
	n.trace = append(n.trace, fmt.Sprintf("t%d unwinds: Cancelled", i))
	n.end(i, "Cancelled")
	return n
}

// aloneSteps are the successors in which task i's operation proceeds without a partner.
func (s *nState) aloneSteps(sc NScenario, i int, op NOp) []*nState {
	var out []*nState
	switch op.Kind {
	case NSend:
		if s.canSendAlone(sc, op.Ch) {
			n := s.clone()
			n.send(sc, i, op.Ch, op.Val, "send")
			out = append(out, n)
		}
	case NRecv:
		if s.canRecvAlone(sc, op.Ch) {
			n := s.clone()
			n.recv(sc, i, op.Ch, "recv")
			out = append(out, n)
		}
	case NSelect:
		for k, a := range op.Arms {
			if a.Send && s.canSendAlone(sc, a.Ch) || !a.Send && s.canRecvAlone(sc, a.Ch) {
				n := s.clone()
				n.selects[i]++
				what := fmt.Sprintf("select arm %d", k)
				if a.Send {
					n.send(sc, i, a.Ch, a.Val, what)
				} else {
					n.recv(sc, i, a.Ch, what)
				}
				out = append(out, n)
			}
		}
	case NClose:
		n := s.clone()
		n.closed[op.Ch] = true
		n.advance(sc, i, fmt.Sprintf("close c%d", op.Ch))
		out = append(out, n)
	case NSet:
		n := s.clone()
		n.set[op.Ch] = true
		n.advance(sc, i, fmt.Sprintf("set e%d", op.Ch))
		out = append(out, n)
	case NWait:
		if s.set[op.Ch] {
			n := s.clone()
			n.waitsOK[op.Ch] = true
			n.advance(sc, i, fmt.Sprintf("wait e%d", op.Ch))
			out = append(out, n)
		}
	case NAcquire:
		q := s.lwait[op.Ch]
		if s.held[op.Ch] < sc.Limits[op.Ch] && (len(q) == 0 || q[0] == i) {
			n := s.clone()
			n.held[op.Ch]++
			n.tasks[i].tokens = append(n.tasks[i].tokens, op.Ch)
			n.lwait[op.Ch] = removeInt(n.lwait[op.Ch], i)
			n.advance(sc, i, fmt.Sprintf("acquire l%d", op.Ch))
			out = append(out, n)
		}
	case NRelease:
		n := s.clone()
		t := &n.tasks[i]
		if len(t.tokens) == 0 {
			n.trace = append(n.trace, fmt.Sprintf("t%d releases a token it does not hold", i))
			n.end(i, "Violation")
		} else {
			n.held[t.tokens[0]]--
			t.tokens = t.tokens[1:]
			n.advance(sc, i, "release")
		}
		out = append(out, n)
	}
	return out
}

// jointSteps are rendezvous on capacity-0 channels between task i as sender and another task as
// receiver (each through a plain operation or a select arm).
func (s *nState) jointSteps(sc NScenario, i int, op NOp) []*nState {
	var out []*nState
	for _, sa := range sendArms(op) {
		if sc.Caps[sa.Ch] != 0 || s.closed[sa.Ch] {
			continue
		}
		for j := range s.tasks {
			if j == i {
				continue
			}
			rop, ok := s.op(sc, j)
			if !ok {
				continue
			}
			for _, ra := range recvArms(rop) {
				if ra.Ch != sa.Ch {
					continue
				}
				n := s.clone()
				n.commits[sa.Ch] = append(n.commits[sa.Ch], sa.Val)
				n.received[sa.Ch] = append(n.received[sa.Ch], sa.Val)
				if op.Kind == NSelect {
					n.selects[i]++
				}
				if rop.Kind == NSelect {
					n.selects[j]++
				}
				n.advance(sc, i, fmt.Sprintf("rendezvous send %d on c%d", sa.Val, sa.Ch))
				n.advance(sc, j, fmt.Sprintf("rendezvous recv on c%d", sa.Ch))
				out = append(out, n)
			}
		}
	}
	return out
}

func sendArms(op NOp) []NArm {
	switch op.Kind {
	case NSend:
		return []NArm{{Send: true, Ch: op.Ch, Val: op.Val}}
	case NSelect:
		var a []NArm
		for _, x := range op.Arms {
			if x.Send {
				a = append(a, x)
			}
		}
		return a
	}
	return nil
}

func recvArms(op NOp) []NArm {
	switch op.Kind {
	case NRecv:
		return []NArm{{Ch: op.Ch}}
	case NSelect:
		var a []NArm
		for _, x := range op.Arms {
			if !x.Send {
				a = append(a, x)
			}
		}
		return a
	}
	return nil
}

// send performs a send that can proceed alone: on a closed channel the task fails with
// ClosedChannel and nothing is published (D35); otherwise the value is buffered.
func (s *nState) send(sc NScenario, i, ch, v int, what string) {
	if s.closed[ch] {
		s.trace = append(s.trace, fmt.Sprintf("t%d %s on closed c%d: ClosedChannel", i, what, ch))
		s.end(i, "ClosedChannel")
		return
	}
	s.queues[ch] = append(s.queues[ch], v)
	s.commits[ch] = append(s.commits[ch], v)
	s.advance(sc, i, fmt.Sprintf("%s %d on c%d", what, v, ch))
}

// recv performs a receive that can proceed alone: the head of the buffer, or ok=false once the
// channel is closed and empty.
func (s *nState) recv(sc NScenario, i, ch int, what string) {
	if len(s.queues[ch]) > 0 {
		v := s.queues[ch][0]
		s.queues[ch] = s.queues[ch][1:]
		s.received[ch] = append(s.received[ch], v)
		s.advance(sc, i, fmt.Sprintf("%s %d from c%d", what, v, ch))
		return
	}
	s.advance(sc, i, fmt.Sprintf("%s closed c%d", what, ch))
}
