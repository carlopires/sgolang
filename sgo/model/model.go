// Package model is the executable reference model (oracle) of S-Go subset-0 structured concurrency
// (W01 step 5). It is a pure state machine: no goroutines, no clocks. Every step function names the
// clause it implements. Explore (explore.go) enumerates every interleaving of small programs and
// checks the invariants of plan §3 after each step.
package model

import (
	"fmt"
	"sort"
	"strings"
)

// Op is a program instruction.
type Op int

const (
	OpNursery    Op = iota // nursery (Name) { Body }                         SGO-NUR-001
	OpSpawn                // Name.Spawn(Child)                               SGO-SPAWN-001
	OpCancel               // Name.Cancel()                                   SGO-CANCEL-001
	OpCheckpoint           // if err := task.Checkpoint(); err != nil {...}   SGO-CANCEL-002
	OpSleep                // if err := task.Sleep(ms); err != nil {...}      SGO-WAIT-001
	OpReturnNil            // return nil
	OpReturnErr            // return sgo.NewError(Msg)
	OpPanic                // panic(Msg)                                      SGO-DEF-002
	OpSpin                 // a loop with no cancellation point (non-cooperative)
	OpMark                 // records Msg in the trace (stands in for an observable effect)
)

// Instr is one instruction. On a cancellation error from OpCheckpoint/OpSleep the task returns that
// error (propagation), unless Continue is set, in which case it records the observation and continues.
type Instr struct {
	Op       Op
	Name     string  // handle name for OpNursery/OpSpawn/OpCancel
	Body     []Instr // OpNursery
	Child    []Instr // OpSpawn
	Msg      string  // OpReturnErr, OpPanic, OpMark
	Continue bool    // OpCheckpoint/OpSleep: continue after observing cancellation
}

// Program is the body of main (the root task, creation ID 0).
type Program []Instr

// Scope states, SGO-NUR-002.
type ScopeState int

const (
	Open ScopeState = iota
	Joining
	Cancelling
	Finalizing
	Closed
)

func (s ScopeState) String() string {
	return [...]string{"OPEN", "JOINING", "CANCELLING", "FINALIZING", "CLOSED"}[s]
}

// Outcome kinds of a child task, SGO-SPAWN-004.
const (
	Succeeded = "Succeeded"
	Cancelled = "Cancelled"
	Errored   = "Errored"
	Failed    = "Failed"
)

// Failure is a task failure (SGO-DEF-003) with its chain (SGO-DEF-004, SGO-NUR-004).
type Failure struct {
	Kind  string
	Msg   string
	Chain []string
}

// Value is what a task is returning: nil, an error, or a cancellation error.
type Value struct {
	Err       bool
	Cancelled bool
	Msg       string
}

type frame struct {
	code    []Instr
	pc      int
	env     map[string]int // handle name -> scope ID
	nursery int            // scope ID if this frame is a nursery body, else -1
	joining bool           // the nursery body finished; waiting for children
}

type taskStatus int

const (
	running taskStatus = iota
	sleeping
	joined   // blocked in a nursery join (SGO-WAIT-002)
	spinning // non-cooperative forever
	done
)

// Task is a model task.
type Task struct {
	ID        int
	Scope     int // scope it was spawned into; -1 for root (SGO-SPAWN-003)
	frames    []frame
	status    taskStatus
	unwinding *Failure // set while unwinding (SGO-DEF-005)
	returning *Value   // set while returning through nurseries (SGO-CTL-004)
	waitID    int      // current sleep wait, 0 if none
	Outcome   string
	Result    *Value
	Fail      *Failure
}

// Scope is a nursery scope.
type Scope struct {
	ID       int
	Owner    int
	State    ScopeState
	Live     map[int]bool // live registered children
	Children []int        // all registered children, creation order
	Cause    string       // first cancellation cause (CAN-1)
	Outcomes map[int]string
	Errors   map[int]string
	Fails    map[int]*Failure
}

// Config selects options and deliberate mutations (bugs) used to test the oracle itself.
type Config struct {
	DrainBudget bool // SGO-DRAIN-002 enabled
	Mut         Mutation
}

// Mutation is a deliberately wrong variant of the semantics. The test suite must detect each one.
type Mutation int

const (
	MutNone             Mutation = iota
	MutEarlyJoin                 // join completes when the body ends, ignoring live children (breaks OWN-3/OWN-4)
	MutLateRegister              // a spawned child is not counted live until it first runs (breaks OWN-2)
	MutNoCancelOnFail            // a failing child does not cancel its scope (breaks SGO-SPAWN-004)
	MutTaskStickyCancel          // cancellation is sticky per task, not per scope (breaks SGO-CANCEL-002/004)
	MutLatestFailure             // nursery outcome uses the latest-created failed child (breaks SGO-NUR-004)
	MutNoCurrentScopeCheck       // spawn allowed into a scope that is not the caller's current scope (breaks SGO-SPAWN-002 (c))
	MutDoubleWake                // a sleep may complete twice when cancellation races expiry (breaks CAN-2)
)

// State is the whole model state.
type State struct {
	Tasks       []*Task
	Scopes      []*Scope
	nextWait    int
	waitsDone   map[int]int // wait ID -> number of terminal dispositions (CAN-2)
	taskSticky  map[int]bool
	Trace       []string
	Exit        int // -1 while running
	ExitKind    string
	ExitMsg     string
	cfg         *Config
	closedOnce  map[int]bool
	observedRun map[int]bool // tasks that have taken at least one step
}

// NewState creates the initial state for a program: the root task about to run main.
func NewState(p Program, cfg *Config) *State {
	root := &Task{ID: 0, Scope: -1, frames: []frame{{code: p, env: map[string]int{}, nursery: -1}}}
	return &State{Tasks: []*Task{root}, Exit: -1, waitsDone: map[int]int{}, taskSticky: map[int]bool{},
		cfg: cfg, closedOnce: map[int]bool{}, observedRun: map[int]bool{}}
}

// Clone deep-copies the state (programs are shared, they are immutable).
func (s *State) Clone() *State {
	n := *s
	n.Tasks = make([]*Task, len(s.Tasks))
	for i, t := range s.Tasks {
		c := *t
		c.frames = make([]frame, len(t.frames))
		for j, f := range t.frames {
			f.env = copyEnv(f.env)
			c.frames[j] = f
		}
		if t.unwinding != nil {
			u := *t.unwinding
			u.Chain = append([]string(nil), u.Chain...)
			c.unwinding = &u
		}
		if t.returning != nil {
			r := *t.returning
			c.returning = &r
		}
		n.Tasks[i] = &c
	}
	n.Scopes = make([]*Scope, len(s.Scopes))
	for i, sc := range s.Scopes {
		c := *sc
		c.Live = map[int]bool{}
		for k, v := range sc.Live {
			c.Live[k] = v
		}
		c.Children = append([]int(nil), sc.Children...)
		c.Outcomes, c.Errors, c.Fails = map[int]string{}, map[int]string{}, map[int]*Failure{}
		for k, v := range sc.Outcomes {
			c.Outcomes[k] = v
		}
		for k, v := range sc.Errors {
			c.Errors[k] = v
		}
		for k, v := range sc.Fails {
			c.Fails[k] = v
		}
		n.Scopes[i] = &c
	}
	n.waitsDone = copyIntMap(s.waitsDone)
	n.taskSticky = copyBoolMap(s.taskSticky)
	n.closedOnce = copyBoolMap(s.closedOnce)
	n.observedRun = copyBoolMap(s.observedRun)
	n.Trace = append([]string(nil), s.Trace...)
	return &n
}

func copyEnv(m map[string]int) map[string]int {
	c := make(map[string]int, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func copyIntMap(m map[int]int) map[int]int {
	c := make(map[int]int, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func copyBoolMap(m map[int]bool) map[int]bool {
	c := make(map[int]bool, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// Key is a canonical encoding of the state used to deduplicate exploration. It excludes the trace.
func (s *State) Key() string {
	var b strings.Builder
	for _, t := range s.Tasks {
		fmt.Fprintf(&b, "T%d s%d st%d w%d o%s|", t.ID, t.Scope, t.status, t.waitID, t.Outcome)
		if t.unwinding != nil {
			fmt.Fprintf(&b, "U%s:%s:%v|", t.unwinding.Kind, t.unwinding.Msg, t.unwinding.Chain)
		}
		if t.returning != nil {
			fmt.Fprintf(&b, "R%v|", *t.returning)
		}
		for _, f := range t.frames {
			keys := make([]string, 0, len(f.env))
			for k, v := range f.env {
				keys = append(keys, fmt.Sprintf("%s=%d", k, v))
			}
			sort.Strings(keys)
			fmt.Fprintf(&b, "F%p:%d:%d:%v:%v|", f.code, f.pc, f.nursery, f.joining, keys)
		}
	}
	for _, sc := range s.Scopes {
		live := make([]int, 0, len(sc.Live))
		for k := range sc.Live {
			live = append(live, k)
		}
		sort.Ints(live)
		fmt.Fprintf(&b, "S%d %s %v %s %v %v|", sc.ID, sc.State, live, sc.Cause, sc.Outcomes, sc.Errors)
	}
	fmt.Fprintf(&b, "E%d %s %s %v", s.Exit, s.ExitKind, s.ExitMsg, s.taskSticky)
	return b.String()
}
