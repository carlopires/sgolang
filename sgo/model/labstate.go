package model

import (
	"slices"
	"strings"
)

// Internal operations of main's script: spawn child N, then (after Root) join the nursery.
const (
	lSpawn LKind = 100 + iota
	lJoin
)

const (
	tReady = iota
	tBlocked
	tSleeping
	tJoining
	tDone
)

// Pending results set by a waker; the task acts on them at its next step.
const (
	pNone       = iota
	pUnwind     // claimed by cancellation: the operation has no effect and the task unwinds
	pClosedFail // a waiting send was claimed by close: ClosedChannel
)

type lTask struct {
	id       int
	script   []LOp
	pc       int
	rem      int // remaining steps of the current LWork op; -1 when not started
	status   int
	pend     int
	readySeq int
	inBody   bool   // main only: still inside the nursery body (a cancellation point there)
	outcome  string // children: the outcome once done
}

type lWait struct {
	task int
	send bool
	val  int
	arm  int // select arm index; -1 for a plain operation
}

type lChan struct {
	cap     int
	queue   []int
	closed  bool
	waiters []lWait // FIFO in registration order
}

type lTimer struct {
	deadline int64
	id, task int
}

// Chooser supplies the scheduler's decisions: the next task among ready (ascending IDs), and a
// select's arm among arms (ascending arm indices). Ready and arms have at least one element.
type Chooser interface {
	Pick(ready []int) (int, error)
	Arm(task int, arms []int) (int, error)
}

type lState struct {
	sc        LScenario
	ch        Chooser
	tasks     []*lTask
	chans     []*lChan
	timers    []lTimer
	nextTimer int
	now       int64
	used      int
	cancelled bool
	seq       int
	events    []LEvent
	out       strings.Builder
	rootErr   string // main's own result: "" | "cancelled" | "errored"
	finished  bool
	status    int
	kind      string
}

func newLabState(sc LScenario, ch Chooser) *lState {
	s := &lState{sc: sc, ch: ch}
	for _, c := range sc.Caps {
		s.chans = append(s.chans, &lChan{cap: c})
	}
	var main []LOp
	for i := range sc.Tasks {
		main = append(main, LOp{Kind: lSpawn, N: i})
	}
	main = append(main, sc.Root...)
	main = append(main, LOp{Kind: lJoin})
	s.tasks = append(s.tasks, &lTask{id: 0, script: main, rem: -1, inBody: true})
	s.makeReady(s.tasks[0])
	return s
}

func (s *lState) emit(e LEvent) { s.events = append(s.events, e) }

func (s *lState) makeReady(t *lTask) {
	t.status = tReady
	s.seq++
	t.readySeq = s.seq
}

func (s *lState) ready() []int {
	var r []int
	for _, t := range s.tasks {
		if t.status == tReady {
			r = append(r, t.id)
		}
	}
	return r
}

// cancelledFor reports whether t's operations observe the nursery's cancellation: every child,
// and main while it is in the nursery body (D33; the join is not a cancellation point).
func (s *lState) cancelledFor(t *lTask) bool {
	return s.cancelled && (t.id != 0 || t.inBody)
}

// unregister removes every waiter of task id from every channel (all arms of a select).
func (s *lState) unregister(id int) {
	for _, c := range s.chans {
		c.waiters = slices.DeleteFunc(c.waiters, func(w lWait) bool { return w.task == id })
	}
	s.timers = slices.DeleteFunc(s.timers, func(tm lTimer) bool { return tm.task == id })
}

// cancel cancels the nursery: every waiting or sleeping task that observes it is claimed now and
// unwinds at its next step (sgoWakeCancelled).
func (s *lState) cancel() {
	if s.cancelled {
		return
	}
	s.cancelled = true
	for _, t := range s.tasks {
		if (t.status == tBlocked || t.status == tSleeping) && s.cancelledFor(t) {
			s.unregister(t.id)
			t.pend = pUnwind
			s.makeReady(t)
		}
	}
}

// endChild ends child t with outcome; a failed or errored child cancels the nursery
// (SGO-SPAWN-004), and the last child to end wakes main's join.
func (s *lState) endChild(t *lTask, outcome string) {
	t.status = tDone
	t.outcome = outcome
	s.emit(LEvent{Kind: "end", Task: t.id, Outcome: outcome})
	if outcome != "Succeeded" && outcome != "Cancelled" {
		s.cancel()
	}
	if s.childrenDone() && s.tasks[0].status == tJoining {
		s.makeReady(s.tasks[0])
	}
}

func (s *lState) childrenDone() bool {
	for _, t := range s.tasks[1:] {
		if t.status != tDone {
			return false
		}
	}
	return true
}

// fail ends t's operation with a failure of kind: a child ends Failed; main unwinds (its nursery
// is cancelled and joined) and the run ends with the failure unless a child's failure is primary.
func (s *lState) fail(t *lTask, kind string) {
	if t.id != 0 {
		s.endChild(t, "Failed:"+kind)
		return
	}
	s.mainExits(t, "failed:"+kind)
	s.cancel()
}

// mainExits leaves the nursery body: the remaining body operations are skipped and main joins.
func (s *lState) mainExits(t *lTask, why string) {
	s.rootErr = why
	t.inBody = false
	t.pc = len(t.script) - 1
	t.rem = -1
	t.pend = pNone
	if why == "cancelled" {
		s.cancel() // an unwinding owner cancels its nursery before joining (SGO-DEF-005)
	}
	s.makeReady(t)
}

// finishMain ends the run once the nursery has joined (SGO-NUR-004, SGO-PROC-002).
func (s *lState) finishMain() {
	t := s.tasks[0]
	t.status = tDone
	status, kind := 0, ""
	if k, ok := strings.CutPrefix(s.rootErr, "failed:"); ok {
		s.finish(2, k) // the owner's failure stays primary (SGO-NUR-004 rule 1)
		return
	}
	for _, c := range s.tasks[1:] {
		o := c.outcome
		if strings.HasPrefix(o, "Failed:") {
			status, kind = 2, strings.TrimPrefix(o, "Failed:")
			break
		}
		if o == "Errored" {
			status, kind = 1, "ChildError"
			break
		}
	}
	if status == 0 && s.rootErr != "" {
		status, kind = 1, "MainError"
	}
	s.finish(status, kind)
}

func (s *lState) finish(status int, kind string) {
	s.finished, s.status, s.kind = true, status, kind
	s.emit(LEvent{Kind: "outcome", Status: status, Outcome: kind})
}
