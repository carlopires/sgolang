package model

import (
	"fmt"
	"slices"
	"strings"
)

// LResult is one run: the canonical event stream, stdout, and the outcome.
type LResult struct {
	Events []LEvent
	Stdout string
	Status int
	Kind   string
}

// Lines is the canonical stream, one JSON line per event.
func (r *LResult) Lines() []string {
	out := make([]string, len(r.Events))
	for i, e := range r.Events {
		out[i] = e.Line()
	}
	return out
}

// Outcome is the schedule-independent semantic outcome: exit status, kind, and stdout.
func (r *LResult) Outcome() string {
	return fmt.Sprintf("exit=%d kind=%q stdout=%q", r.Status, r.Kind, r.Stdout)
}

// maxLabSteps bounds a model run; scenarios are bounded by their quota and scripts.
const maxLabSteps = 100000

func runLab(sc LScenario, ch Chooser) (*LResult, error) {
	s := newLabState(sc, ch)
	for range maxLabSteps {
		more, err := s.next()
		if err != nil {
			return &LResult{Events: s.events, Stdout: s.out.String()}, err
		}
		if !more {
			return &LResult{Events: s.events, Stdout: s.out.String(), Status: s.status, Kind: s.kind}, nil
		}
	}
	return nil, fmt.Errorf("lab model: no outcome after %d steps", maxLabSteps)
}

// seeded chooses from the sched stream: a uniform index when there is more than one option, and
// no draw otherwise (D44).
type seeded struct{ p *PCG }

func (c seeded) Pick(ready []int) (int, error) { return ready[c.pick(len(ready))], nil }

func (c seeded) Arm(task int, arms []int) (int, error) { return arms[c.pick(len(arms))], nil }

func (c seeded) pick(n int) int {
	if n == 1 {
		return 0
	}
	return int(c.p.Uint64n(uint64(n)))
}

// RunLab runs sc with the scheduler stream of seed.
func RunLab(sc LScenario, seed uint64) (*LResult, error) {
	st := NewStreams(seed)
	return runLab(sc, seeded{&st.Sched})
}

// Divergence reports the first event of a replay that does not match the trace (D05).
type Divergence struct {
	Seq          int // index of the first mismatching event
	LastMatching int // index of the last matching event, -1 if none
	Expected     string
	Actual       string
}

func (d *Divergence) Error() string {
	return fmt.Sprintf("replay diverged at event %d (last matching %d): expected %s, actual %s",
		d.Seq, d.LastMatching, d.Expected, d.Actual)
}

// replayer takes every decision from the trace and checks each emitted event against it.
type replayer struct {
	trace []LEvent
	s     *lState
}

func (r *replayer) at(kind string) (LEvent, error) {
	i := len(r.s.events) // the next event to be emitted
	if err := r.check(); err != nil {
		return LEvent{}, err
	}
	if i >= len(r.trace) || r.trace[i].Kind != kind {
		exp := "end of trace"
		if i < len(r.trace) {
			exp = r.trace[i].Line()
		}
		return LEvent{}, &Divergence{i, i - 1, exp, "a " + kind + " decision"}
	}
	return r.trace[i], nil
}

// check compares every event emitted so far with the trace.
func (r *replayer) check() error {
	for i, e := range r.s.events {
		if i >= len(r.trace) {
			return &Divergence{i, i - 1, "end of trace", e.Line()}
		}
		if e.Line() != r.trace[i].Line() {
			return &Divergence{i, i - 1, r.trace[i].Line(), e.Line()}
		}
	}
	return nil
}

func (r *replayer) Pick(ready []int) (int, error) {
	e, err := r.at("choose")
	if err != nil {
		return 0, err
	}
	if !slices.Equal(e.Ready, ready) || !slices.Contains(ready, e.Pick) {
		got := LEvent{Kind: "choose", Ready: ready, Pick: e.Pick}
		return 0, &Divergence{len(r.s.events), len(r.s.events) - 1, e.Line(), got.Line()}
	}
	return e.Pick, nil
}

func (r *replayer) Arm(task int, arms []int) (int, error) {
	e, err := r.at("select")
	if err != nil {
		return 0, err
	}
	if e.Task != task || !slices.Equal(e.Arms, arms) || !slices.Contains(arms, e.Arm) {
		got := LEvent{Kind: "select", Task: task, Arms: arms, Arm: e.Arm}
		return 0, &Divergence{len(r.s.events), len(r.s.events) - 1, e.Line(), got.Line()}
	}
	return e.Arm, nil
}

// ReplayLab runs sc taking every decision from trace (never from a generator) and stops at the
// first divergence (D03, D05). A replay that completes reproduces the trace exactly.
func ReplayLab(sc LScenario, trace []LEvent) (*LResult, error) {
	r := &replayer{trace: trace}
	r.s = newLabState(sc, r)
	for range maxLabSteps {
		more, err := r.s.next()
		if err == nil {
			err = r.check()
		}
		if err != nil {
			return nil, err
		}
		if !more {
			if len(r.s.events) != len(trace) {
				return nil, &Divergence{len(r.s.events), len(r.s.events) - 1, trace[len(r.s.events)].Line(), "end of run"}
			}
			return &LResult{Events: r.s.events, Stdout: r.s.out.String(), Status: r.s.status, Kind: r.s.kind}, nil
		}
	}
	return nil, fmt.Errorf("lab model: replay did not end")
}

// vector chooses by a fixed sequence of option indices, recording how many options each decision
// had (for enumeration).
type vector struct {
	picks []int
	n     []int
}

func (v *vector) choose(n int) int {
	i := len(v.n)
	v.n = append(v.n, n)
	if i < len(v.picks) {
		return v.picks[i]
	}
	return 0
}

func (v *vector) Pick(ready []int) (int, error)         { return ready[v.choose(len(ready))], nil }
func (v *vector) Arm(task int, arms []int) (int, error) { return arms[v.choose(len(arms))], nil }

// ExploreLab enumerates every sequence of scheduler decisions of sc (depth-first over the option
// counts) and returns the set of semantic outcomes and the number of distinct runs.
func ExploreLab(sc LScenario, limit int) (map[string]bool, int, error) {
	outcomes := map[string]bool{}
	runs := 0
	picks := []int{}
	for {
		v := &vector{picks: picks}
		r, err := runLab(sc, v)
		if err != nil {
			return nil, runs, err
		}
		outcomes[r.Outcome()] = true
		runs++
		if runs >= limit {
			return nil, runs, fmt.Errorf("lab model: more than %d runs", limit)
		}
		// Next sequence: increment the last decision that has an untried option.
		full := make([]int, len(v.n))
		copy(full, picks)
		i := len(full) - 1
		for i >= 0 && full[i]+1 >= v.n[i] {
			i--
		}
		if i < 0 {
			return outcomes, runs, nil
		}
		full[i]++
		picks = full[:i+1]
	}
}

// ParseLabTrace is the inverse of LEvent.Line for the model's own tests (the driver has the
// production reader).
func ParseLabTrace(lines []string) []LEvent {
	var out []LEvent
	for _, l := range lines {
		out = append(out, parseLabLine(strings.TrimSpace(l)))
	}
	return out
}
