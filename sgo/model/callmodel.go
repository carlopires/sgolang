// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import "fmt"

// Request/reply calls (D167, D174): callers and a server inside one nursery. A caller hands its
// request over by rendezvous, then waits for its reply; it may be cancelled at any step. The
// server accepts requests (until its scope is cancelled) and gives each to a handler, whose body
// replies, or ends without replying. When nothing can step, the scope ends and cancels whatever
// still waits to hand over. Properties: (P1) every call ends exactly once, with its reply, NoReply,
// or its cancellation; (P2) no handler holds a request whose caller withdrew before it was
// accepted; (P3) a reply reaches only the caller that made the request; (P4) no call is stuck: no
// caller waits for a reply after its request's handler ended (only its cancellation could free
// it).

const caMax = 3

var (
	caNoRelease    bool // a handler whose body ends without Reply leaves its caller waiting
	caLateWithdraw bool // a caller cancelled while handing over leaves its request to be accepted
	caMisroute     bool // a reply goes to the first waiting caller, not to the request's caller
	caReplyTwice   bool // a second Reply delivers again
)

// Caller phases.
const (
	caSending = iota // waiting for the server to accept the request
	caWaiting        // accepted: waiting for the reply
	caDone           // the call returned
)

// Outcomes of a call.
const (
	caNone = iota
	caReply
	caNoReply
	caCancelled
)

// Handler states, one per request.
const (
	hNone    = iota // not accepted
	hHeld           // accepted, not answered
	hReplied        // answered, body still running
	hEnded          // the handler's body ended
)

type caState struct {
	phase    [caMax]int8
	outcome  [caMax]int8
	ends     [caMax]int8 // how many times the call returned
	got      [caMax]int8 // the request id of the reply the caller got (+1)
	withdrew [caMax]bool // cancelled while handing over
	handler  [caMax]int8
	replies  [caMax]int8
	stopped  bool // the server's scope was cancelled: no more Accept
	scopeEnd bool
}

func (s caState) key() string { return fmt.Sprint(s) }

func (s *caState) deliver(i int, o int8, req int) {
	s.phase[i], s.outcome[i] = caDone, o
	s.ends[i]++
	if o == caReply {
		s.got[i] = int8(req) + 1
	}
}

func caSteps(s caState) []caState {
	var out []caState
	for i := 0; i < caMax; i++ {
		switch s.phase[i] {
		case caSending:
			if !s.stopped { // rendezvous: the server accepts request i and gives it to a handler
				n := s
				n.phase[i], n.handler[i] = caWaiting, hHeld
				out = append(out, n)
			}
			n := s // cancelled while handing over: withdrawn
			n.withdrew[i] = true
			n.deliver(i, caCancelled, -1)
			if caLateWithdraw && !s.stopped {
				n.handler[i] = hHeld // the request is still accepted
			}
			out = append(out, n)
		case caWaiting:
			n := s // cancelled while waiting: a later reply is dropped
			n.deliver(i, caCancelled, -1)
			out = append(out, n)
		}
	}
	if !s.stopped {
		n := s
		n.stopped = true // the server's scope is cancelled
		out = append(out, n)
	}
	for h := 0; h < caMax; h++ {
		switch s.handler[h] {
		case hHeld, hReplied:
			if s.handler[h] == hHeld || caReplyTwice && s.replies[h] < 2 { // Reply
				n := s
				n.handler[h] = hReplied
				n.replies[h]++
				to := h
				if caMisroute {
					for j := 0; j < caMax; j++ {
						if n.phase[j] == caWaiting {
							to = j
							break
						}
					}
				}
				if n.phase[to] == caWaiting || caReplyTwice && n.replies[h] == 2 {
					n.deliver(to, caReply, h)
				}
				out = append(out, n)
			}
			n := s // the handler's body ends; an unanswered request is released with NoReply
			n.handler[h] = hEnded
			if s.handler[h] == hHeld && !caNoRelease && n.phase[h] == caWaiting {
				n.deliver(h, caNoReply, -1)
			}
			out = append(out, n)
		}
	}
	return out
}

func caCheck(s caState, final bool) error {
	for i := 0; i < caMax; i++ {
		if s.ends[i] > 1 {
			return fmt.Errorf("P1: call %d returned %d times", i, s.ends[i])
		}
		if s.got[i] != 0 && s.got[i] != int8(i)+1 {
			return fmt.Errorf("P3: call %d got the reply to request %d", i, s.got[i]-1)
		}
		if s.withdrew[i] && s.handler[i] != hNone {
			return fmt.Errorf("P2: a handler holds the request of call %d, withdrawn before acceptance", i)
		}
		if s.phase[i] == caWaiting && s.handler[i] == hEnded || final && s.phase[i] == caWaiting {
			return fmt.Errorf("P4: call %d waits for a reply that cannot come: %+v", i, s)
		}
	}
	return nil
}

// CallExplore explores every run of the request/reply scenario; it returns the number of states.
func CallExplore() (int, error) {
	s0 := caState{}
	seen := map[string]bool{s0.key(): true}
	work := []caState{s0}
	for len(work) > 0 {
		s := work[len(work)-1]
		work = work[:len(work)-1]
		next := caSteps(s)
		if len(next) == 0 {
			for i := 0; i < caMax; i++ { // the scope ends: it cancels what still waits to hand over
				if s.phase[i] == caSending {
					s.deliver(i, caCancelled, -1)
				}
			}
			s.scopeEnd = true
		}
		if err := caCheck(s, len(next) == 0); err != nil {
			return len(seen), err
		}
		for _, n := range next {
			if k := n.key(); !seen[k] {
				seen[k] = true
				work = append(work, n)
			}
		}
	}
	return len(seen), nil
}
