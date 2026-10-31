package model

// Executable reference model of typed managed channels with send permits (SGO-PERMIT-002 draft,
// D28/D30, plan §7). A pure state machine explored over every interleaving of small scenarios;
// the runtime implementation (W06) is checked against the histories it admits.

import (
	"fmt"
	"strings"
)

// Permit states (plan §7.2).
const (
	PermitNone = iota
	PermitReserved
	PermitCommitted
	PermitFailedNoEffect // commit after close: closed result, no effect
	PermitAborted
	PermitRefused // reserve on a closed channel: no permit
	PermitCancelledBeforeReserve
)

// PermitName is the stable name of a permit state used in reference outcomes.
func PermitName(p int) string {
	return [...]string{"none", "reserved", "committed", "failed-closed", "aborted", "refused-closed", "cancelled"}[p]
}

// Sender scripts: what a sender does after its reservation.
const (
	SendCommit = iota // Reserve, Commit(v)
	SendAbort         // Reserve, Abort
	// SendCancelRace: cancellation of the sender's task races with Reserve and Commit (H06, H07):
	// before reserve no permit; after reserve and before commit the reservation is aborted; after
	// commit it is not retroactive.
	SendCancelRace
	// SendTaskEnd: the task ends without resolving its permit; the runtime aborts it before the
	// task's outcome is published and records a diagnostic (H09, H10).
	SendTaskEnd
	// SendAliasDouble: two aliases of one permit both commit; the first takes effect, the second
	// is a misuse with no effect (H04).
	SendAliasDouble
)

// ChanScenario is one bounded scenario: a channel of capacity Cap, senders (each sends value i+1
// with the given script), Receivers receive operations, and an optional Close actor.
type ChanScenario struct {
	Cap       int
	Senders   []int
	Receivers int
	Close     bool
}

type chanState struct {
	queue     []int // committed, not yet received (C > 0)
	reserved  int   // reservations not yet terminal
	closed    bool
	permit    []int // per sender
	spc       []int // sender program counter: 0 reserve, 1 resolve, 2 done
	match     []int // C == 0: receiver matched to sender i (-1: none)
	rpc       []int // receiver program counter: 0 start, 1 waiting (C == 0), 2 done
	rmatched  []int // C == 0: sender matched to receiver j (-1: none)
	received  []int // values in receive order
	recvOK    []bool
	commits   []int // values in commit order
	closeDone bool
	trace     []string
	cancelled []bool // SendCancelRace: the task has been cancelled
	aliasDone []bool // SendAliasDouble: the second alias has acted
	diags     int    // diagnostics recorded by the runtime (abandoned permits, misuse)
}

func (s *chanState) clone() *chanState {
	c := *s
	c.queue = append([]int(nil), s.queue...)
	c.permit = append([]int(nil), s.permit...)
	c.spc = append([]int(nil), s.spc...)
	c.match = append([]int(nil), s.match...)
	c.rpc = append([]int(nil), s.rpc...)
	c.rmatched = append([]int(nil), s.rmatched...)
	c.received = append([]int(nil), s.received...)
	c.recvOK = append([]bool(nil), s.recvOK...)
	c.commits = append([]int(nil), s.commits...)
	c.trace = append([]string(nil), s.trace...)
	c.cancelled = append([]bool(nil), s.cancelled...)
	c.aliasDone = append([]bool(nil), s.aliasDone...)
	return &c
}

// release ends sender i's reservation and any rendezvous match it holds.
func (s *chanState) release(sc ChanScenario, i int) {
	s.reserved--
	if sc.Cap == 0 && s.match[i] >= 0 {
		s.rmatched[s.match[i]] = -1
		s.match[i] = -1
	}
}

// chanSteps returns every enabled step from s.
func chanSteps(sc ChanScenario, s *chanState) []*chanState {
	var out []*chanState
	for i, script := range sc.Senders {
		v := i + 1
		// Cancellation of the task (SendCancelRace) is an event that races with every step.
		if script == SendCancelRace && !s.cancelled[i] && s.spc[i] < 2 {
			n := s.clone()
			n.cancelled[i] = true
			if n.spc[i] == 0 {
				n.permit[i], n.spc[i] = PermitCancelledBeforeReserve, 2
			} else {
				n.release(sc, i)
				n.permit[i], n.spc[i] = PermitAborted, 2
			}
			n.trace = append(n.trace, fmt.Sprintf("s%d cancelled", v))
			out = append(out, n)
		}
		// The second alias of a committed or failed permit acts after the first: no effect.
		if script == SendAliasDouble && s.spc[i] == 2 && !s.aliasDone[i] && s.permit[i] != PermitRefused {
			n := s.clone()
			n.aliasDone[i] = true
			n.diags++
			n.trace = append(n.trace, fmt.Sprintf("s%d second alias commit: misuse, no effect", v))
			out = append(out, n)
		}
		switch s.spc[i] {
		case 0: // Reserve
			switch {
			case s.closed:
				n := s.clone()
				n.permit[i], n.spc[i] = PermitRefused, 2
				n.trace = append(n.trace, fmt.Sprintf("s%d reserve: closed", v))
				out = append(out, n)
			case sc.Cap > 0 && len(s.queue)+s.reserved < sc.Cap:
				n := s.clone()
				n.reserved++
				n.permit[i], n.spc[i] = PermitReserved, 1
				n.trace = append(n.trace, fmt.Sprintf("s%d reserve", v))
				out = append(out, n)
			case sc.Cap == 0:
				// Rendezvous: a reservation requires a waiting, unmatched receiver.
				for j := range s.rpc {
					if s.rpc[j] == 1 && s.rmatched[j] < 0 {
						n := s.clone()
						n.reserved++
						n.permit[i], n.spc[i] = PermitReserved, 1
						n.match[i], n.rmatched[j] = j, i
						n.trace = append(n.trace, fmt.Sprintf("s%d reserve with r%d", v, j))
						out = append(out, n)
					}
				}
			}
		case 1: // Commit or Abort
			n := s.clone()
			n.reserved--
			n.spc[i] = 2
			switch {
			case script == SendTaskEnd:
				n.permit[i] = PermitAborted
				n.diags++
				if sc.Cap == 0 {
					n.rmatched[n.match[i]] = -1
					n.match[i] = -1
				}
				n.trace = append(n.trace, fmt.Sprintf("s%d task ends: runtime aborts the permit", v))
			case script == SendAbort:
				n.permit[i] = PermitAborted
				if sc.Cap == 0 {
					n.rmatched[n.match[i]] = -1
					n.match[i] = -1
				}
				n.trace = append(n.trace, fmt.Sprintf("s%d abort", v))
			case s.closed:
				n.permit[i] = PermitFailedNoEffect
				if sc.Cap == 0 {
					n.rmatched[n.match[i]] = -1
					n.match[i] = -1
				}
				n.trace = append(n.trace, fmt.Sprintf("s%d commit: closed, no effect", v))
			default:
				n.permit[i] = PermitCommitted
				n.commits = append(n.commits, v)
				if sc.Cap > 0 {
					n.queue = append(n.queue, v)
				} else {
					j := n.match[i]
					n.received = append(n.received, v)
					n.recvOK[j] = true
					n.rpc[j] = 2
				}
				n.trace = append(n.trace, fmt.Sprintf("s%d commit", v))
			}
			out = append(out, n)
		}
	}
	for j := range s.rpc {
		switch {
		case s.rpc[j] == 0 && sc.Cap > 0 && len(s.queue) > 0:
			n := s.clone()
			n.received = append(n.received, n.queue[0])
			n.queue = n.queue[1:]
			n.recvOK[j], n.rpc[j] = true, 2
			n.trace = append(n.trace, fmt.Sprintf("r%d receive %d", j, n.received[len(n.received)-1]))
			out = append(out, n)
		case s.rpc[j] == 0 && sc.Cap > 0 && s.closed && s.reserved == 0:
			n := s.clone()
			n.rpc[j] = 2
			n.trace = append(n.trace, fmt.Sprintf("r%d receive: closed", j))
			out = append(out, n)
		case s.rpc[j] == 0 && sc.Cap == 0:
			n := s.clone()
			n.rpc[j] = 1
			n.trace = append(n.trace, fmt.Sprintf("r%d wait", j))
			out = append(out, n)
		case s.rpc[j] == 1 && s.closed && s.rmatched[j] < 0:
			n := s.clone()
			n.rpc[j] = 2
			n.trace = append(n.trace, fmt.Sprintf("r%d receive: closed", j))
			out = append(out, n)
		}
	}
	if sc.Close && !s.closeDone {
		n := s.clone()
		n.closed, n.closeDone = true, true
		n.trace = append(n.trace, "close")
		out = append(out, n)
	}
	return out
}

// chanCheck checks the invariants of SGO-PERMIT-002 in s; final reports whether s is terminal.
func chanCheck(sc ChanScenario, s *chanState, final bool) string {
	if sc.Cap > 0 && len(s.queue)+s.reserved > sc.Cap {
		return "capacity exceeded"
	}
	if s.reserved < 0 {
		return "negative reservations"
	}
	// FIFO and no duplication: received values are a prefix of the commit order (C > 0), and
	// every received value was committed exactly once.
	seen := map[int]bool{}
	for k, v := range s.received {
		if seen[v] {
			return fmt.Sprintf("value %d received twice", v)
		}
		seen[v] = true
		if sc.Cap > 0 && (k >= len(s.commits) || s.commits[k] != v) {
			return "receive order differs from commit order"
		}
	}
	committed := map[int]bool{}
	for _, c := range s.commits {
		if committed[c] {
			return fmt.Sprintf("value %d committed twice", c)
		}
		committed[c] = true
	}
	for _, v := range s.received {
		found := false
		for _, c := range s.commits {
			found = found || c == v
		}
		if !found {
			return fmt.Sprintf("value %d received but never committed", v)
		}
	}
	if final {
		if len(s.received)+len(s.queue) != len(s.commits) {
			return "a committed value was lost"
		}
		wantDiags := 0
		for i, p := range s.permit {
			if p == PermitReserved || p == PermitNone && s.spc[i] != 2 {
				return fmt.Sprintf("sender %d ended without a terminal permit state", i+1)
			}
			switch {
			case sc.Senders[i] == SendTaskEnd && p == PermitAborted:
				wantDiags++
			case sc.Senders[i] == SendAliasDouble && p != PermitRefused:
				wantDiags++
			}
			if sc.Senders[i] == SendCancelRace && s.cancelled[i] && p == PermitCommitted {
				// Allowed: commit won before cancellation, which is not retroactive (H07).
				continue
			}
		}
		if s.diags != wantDiags {
			return fmt.Sprintf("%d diagnostics recorded, want %d", s.diags, wantDiags)
		}
	}
	return ""
}

// ChanResult summarizes an exploration.
type ChanResult struct {
	States, Terminal, Deadlocked int
	Violation                    string
	Trace                        string
	Outcomes                     map[string]bool
}

// ExploreChan enumerates every interleaving of sc and checks the invariants after each step.
// Terminal states where some actor could not finish (a receiver waiting with no sender left, a
// sender waiting for capacity) are counted as deadlocked, not as violations: they are the blocking
// a real program would observe as a wait.
func ExploreChan(sc ChanScenario) *ChanResult {
	n := len(sc.Senders)
	s := &chanState{permit: make([]int, n), spc: make([]int, n), match: make([]int, n),
		rpc: make([]int, sc.Receivers), rmatched: make([]int, sc.Receivers), recvOK: make([]bool, sc.Receivers),
		cancelled: make([]bool, n), aliasDone: make([]bool, n)}
	for i := range s.match {
		s.match[i] = -1
	}
	for j := range s.rmatched {
		s.rmatched[j] = -1
	}
	r := &ChanResult{Outcomes: map[string]bool{}}
	var walk func(s *chanState)
	walk = func(s *chanState) {
		if r.Violation != "" {
			return
		}
		r.States++
		next := chanSteps(sc, s)
		done := true
		for _, p := range s.spc {
			done = done && p == 2
		}
		for _, p := range s.rpc {
			done = done && p == 2
		}
		if len(next) == 0 {
			if done {
				r.Terminal++
			} else {
				r.Deadlocked++
			}
			if v := chanCheck(sc, s, done); v != "" {
				r.Violation, r.Trace = v, strings.Join(s.trace, "; ")
				return
			}
			names := make([]string, len(s.permit))
			for i, p := range s.permit {
				names[i] = PermitName(p)
			}
			r.Outcomes[fmt.Sprintf("permits=%v received=%v queued=%v", names, s.received, s.queue)] = true
			return
		}
		for _, t := range next {
			if v := chanCheck(sc, t, false); v != "" {
				r.Violation, r.Trace = v, strings.Join(t.trace, "; ")
				return
			}
			walk(t)
		}
	}
	walk(s)
	return r
}
