// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

// Channel operations under the baton: each is one atomic step. Waiters are served in registration
// order (FIFO); a woken select reports the arm its waker used as a select event.

type chWait struct {
	lWait
	c int
}

func (w chWait) ch() int { return w.c }

func (s *lState) canSend(ch int) bool {
	c := s.chans[ch]
	if c.closed || len(c.queue) < c.cap {
		return true
	}
	for _, w := range c.waiters {
		if !w.send {
			return true
		}
	}
	return false
}

func (s *lState) canRecv(ch int) bool {
	c := s.chans[ch]
	if c.closed || len(c.queue) > 0 {
		return true
	}
	for _, w := range c.waiters {
		if w.send {
			return true
		}
	}
	return false
}

// wake completes waiter w's operation: the task leaves every channel it waits on, its operation
// is done (pc advances), and it is ready; a select reports the arm used.
func (s *lState) wake(w lWait, closedSend bool) {
	t := s.tasks[w.task]
	s.unregister(t.id)
	if w.arm >= 0 {
		s.emit(LEvent{Kind: "select", Task: t.id, Arms: []int{w.arm}, Arm: w.arm})
	}
	if closedSend {
		s.fail(t, "ClosedChannel") // a waiting send claimed by close
	} else {
		t.pc++
	}
	s.makeReady(t)
}

// first returns the first waiter of c with the given direction.
func first(c *lChan, send bool) (lWait, bool) {
	for _, w := range c.waiters {
		if w.send == send {
			return w, true
		}
	}
	return lWait{}, false
}

// send performs a send that can proceed (canSend); it reports false if the channel is closed.
func (s *lState) send(ch, v int) bool {
	c := s.chans[ch]
	if c.closed {
		return false
	}
	if r, ok := first(c, false); ok {
		s.wake(r, false)
		return true
	}
	c.queue = append(c.queue, v)
	return true
}

// recv performs a receive that can proceed (canRecv).
func (s *lState) recv(ch int) {
	c := s.chans[ch]
	if len(c.queue) > 0 {
		c.queue = c.queue[1:]
		if w, ok := first(c, true); ok {
			c.queue = append(c.queue, w.val)
			s.wake(w, false)
		}
		return
	}
	if w, ok := first(c, true); ok {
		s.wake(w, false)
	}
	// closed and empty: zero value, ok == false
}

func (s *lState) chanStep(t *lTask, op LOp) error {
	switch op.Kind {
	case LSend:
		if !s.canSend(op.Ch) {
			s.block(t, []chWait{{lWait{task: t.id, send: true, val: op.N, arm: -1}, op.Ch}})
			return nil
		}
		if !s.send(op.Ch, op.N) {
			s.fail(t, "ClosedChannel")
			return nil
		}
	case LRecv:
		if !s.canRecv(op.Ch) {
			s.block(t, []chWait{{lWait{task: t.id, arm: -1}, op.Ch}})
			return nil
		}
		s.recv(op.Ch)
	case LClose:
		c := s.chans[op.Ch]
		if !c.closed {
			c.closed = true
			for len(c.waiters) > 0 {
				w := c.waiters[0]
				s.wake(w, w.send)
			}
		}
	case LSelect:
		var arms []int
		for i, a := range op.Arms {
			if (a.Send && s.canSend(a.Ch)) || (!a.Send && s.canRecv(a.Ch)) {
				arms = append(arms, i)
			}
		}
		switch {
		case len(arms) > 0:
			k, err := s.ch.Arm(t.id, arms)
			if err != nil {
				return err
			}
			s.emit(LEvent{Kind: "select", Task: t.id, Arms: arms, Arm: k})
			a := op.Arms[k]
			if a.Send {
				if !s.send(a.Ch, a.Val) {
					s.fail(t, "ClosedChannel")
					return nil
				}
			} else {
				s.recv(a.Ch)
			}
		case op.Default:
			s.emit(LEvent{Kind: "select", Task: t.id, Arms: []int{}, Arm: -1})
		default:
			var ws []chWait
			for i, a := range op.Arms {
				ws = append(ws, chWait{lWait{task: t.id, send: a.Send, val: a.Val, arm: i}, a.Ch})
			}
			s.block(t, ws)
			return nil
		}
	}
	t.pc++
	return nil
}
