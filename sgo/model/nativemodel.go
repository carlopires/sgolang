package model

// Executable reference model of native channels, select, cancellation unwind, and the structured
// primitives Event and CapacityLimiter (spec 0.3.0, D31-D39, W07 step 1). A pure state machine over
// several channels and tasks; nativeexplore.go enumerates every interleaving and checks the
// invariants. Each operation is one atomic step; a capacity-0 send and receive meet in a joint step.
//
// Cancellation (D33): an operation that can proceed when attempted does not observe cancellation.
// One that cannot sets waiting; a waiting operation of a cancelled task may unwind instead of
// proceeding (whichever happens first), and the task ends Cancelled with no effect of that operation.

import "fmt"

// NKind is a native operation.
type NKind int

const (
	NSend    NKind = iota // ch <- Val
	NRecv                 // v, ok := <-ch
	NSelect               // select { Arms...; default if Default }
	NClose                // close(ch); a second close has no effect
	NSet                  // ev.Set()
	NWait                 // ev.Wait(task)
	NAcquire              // tok := lim.Acquire(task)
	NRelease              // tok.Release() of the task's oldest token
)

// NArm is a select arm: a send of Val, or a receive, on channel Ch.
type NArm struct {
	Send bool
	Ch   int
	Val  int
}

// NOp is one operation of a task's script. Ch indexes channels, events, or limiters by kind.
type NOp struct {
	Kind    NKind
	Ch      int
	Val     int
	Arms    []NArm
	Default bool
}

// NScenario is a bounded program: channel capacities, events, limiter sizes, task scripts, and
// the tasks whose scope is cancelled at some point (a nondeterministic event).
type NScenario struct {
	Caps        []int
	Events      int
	Limits      []int
	Tasks       [][]NOp
	Cancellable []bool
}

type nTask struct {
	pc        int
	waiting   bool // the current operation was attempted and could not proceed
	cancelled bool
	outcome   string // "" running; "done"; "Cancelled"; "ClosedChannel"
	tokens    []int  // limiter of each held token, oldest first
}

type nState struct {
	queues   [][]int
	closed   []bool
	set      []bool
	held     []int   // tokens held per limiter
	lwait    [][]int // FIFO of tasks waiting to acquire, per limiter
	tasks    []nTask
	commits  [][]int // per channel, values in commit order
	received [][]int // per channel, values in receive order
	selects  []int   // per task, number of select arms that took effect in its current select
	diags    int     // TokenAbandoned diagnostics
	waitsOK  []bool  // per event, some Wait returned nil
	trace    []string
}

func (s *nState) clone() *nState {
	c := *s
	c.queues = cloneInts2(s.queues)
	c.commits = cloneInts2(s.commits)
	c.received = cloneInts2(s.received)
	c.lwait = cloneInts2(s.lwait)
	c.closed = append([]bool(nil), s.closed...)
	c.set = append([]bool(nil), s.set...)
	c.waitsOK = append([]bool(nil), s.waitsOK...)
	c.held = append([]int(nil), s.held...)
	c.selects = append([]int(nil), s.selects...)
	c.tasks = make([]nTask, len(s.tasks))
	for i, t := range s.tasks {
		t.tokens = append([]int(nil), t.tokens...)
		c.tasks[i] = t
	}
	c.trace = append([]string(nil), s.trace...)
	return &c
}

func cloneInts2(a [][]int) [][]int {
	b := make([][]int, len(a))
	for i := range a {
		b[i] = append([]int(nil), a[i]...)
	}
	return b
}

func newNState(sc NScenario) *nState {
	n := len(sc.Caps)
	s := &nState{queues: make([][]int, n), closed: make([]bool, n), commits: make([][]int, n),
		received: make([][]int, n), set: make([]bool, sc.Events), waitsOK: make([]bool, sc.Events),
		held: make([]int, len(sc.Limits)), lwait: make([][]int, len(sc.Limits)),
		tasks: make([]nTask, len(sc.Tasks)), selects: make([]int, len(sc.Tasks))}
	return s
}

func (s *nState) op(sc NScenario, i int) (NOp, bool) {
	t := s.tasks[i]
	if t.outcome != "" || t.pc >= len(sc.Tasks[i]) {
		return NOp{}, false
	}
	return sc.Tasks[i][t.pc], true
}

// advance completes task i's current operation.
func (s *nState) advance(sc NScenario, i int, what string) {
	t := &s.tasks[i]
	t.pc++
	t.waiting = false
	s.selects[i] = 0
	s.trace = append(s.trace, fmt.Sprintf("t%d %s", i, what))
	if t.pc >= len(sc.Tasks[i]) {
		s.end(i, "done")
	}
}

// end finishes task i; unreleased tokens are released with a TokenAbandoned diagnostic (D39).
func (s *nState) end(i int, outcome string) {
	t := &s.tasks[i]
	t.outcome = outcome
	for _, l := range t.tokens {
		s.held[l]--
		s.diags++
	}
	t.tokens = nil
	for l := range s.lwait {
		s.lwait[l] = removeInt(s.lwait[l], i)
	}
}

func removeInt(a []int, v int) []int {
	out := a[:0:0]
	for _, x := range a {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

// canSend reports whether a send on ch can proceed alone (closed, or room in a buffer).
func (s *nState) canSendAlone(sc NScenario, ch int) bool {
	return s.closed[ch] || (sc.Caps[ch] > 0 && len(s.queues[ch]) < sc.Caps[ch])
}

func (s *nState) canRecvAlone(sc NScenario, ch int) bool {
	return len(s.queues[ch]) > 0 || s.closed[ch]
}
