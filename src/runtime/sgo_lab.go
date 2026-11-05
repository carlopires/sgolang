// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The deterministic lab (spec 0.4.0, D40-D49, W08): a baton scheduler over S-Go tasks. Exactly one
// task holds the baton and runs; every other task is parked. A decision precedes every S-Go
// operation and every K work steps (D43): the ready set (task IDs, ascending) is computed, the next
// task is chosen from the sched stream (D44) or the fifo order (D49), and the baton passes to it.
// Wake-ups do not ready goroutines; they mark tasks ready, and a task runs only when chosen. Time
// is virtual (D45). The canonical event stream (sgo/docs/w08-plan.md) is the same as the model's,
// sgo/model/labmodel.go, for the same choices.
//
// Events are recorded into preallocated records (some are emitted under channel locks, where the
// runtime must not allocate); they are formatted after the run.

package runtime

const (
	sgoLabChoose = iota + 1
	sgoLabSelect
	sgoLabAdvance
	sgoLabFire
	sgoLabWrite
	sgoLabEnd
	sgoLabOutcome
)

type sgoLabEvent struct {
	kind    uint8
	task    int32
	pick    int32 // choose; select: the arm
	set     [2]int32
	to      int64 // advance
	timer   int32 // fire
	text    string
	outcome string
	status  int32
}

type sgoLabTimer struct {
	deadline int64
	id       int64
	t        *sgoTask
	r        *sgoSleep // the Sleep call it ends
	ch       *sgoChan  // or the After timer channel it delivers to (D160)
	scope    *sgoScope // or the nursery whose deadline it is (D162, sgo_deadline.go)
}

type sgoLabState struct {
	lock      mutex
	sched     sgoPCG
	fifo      bool
	k, quota  int64
	used      int64
	now       int64
	timers    []sgoLabTimer
	nextTmr   int64
	tasks     []*sgoTask // by task ID
	seq       int64
	events    []sgoLabEvent
	sets      []int32 // ready sets and arm sets of events (set = [start, end))
	scratch   []int32 // the ready set under construction
	finished  bool
	status    int32
	kind      string
	exit      func(status int32) // ends the process (lab binaries); nil in the runtime's tests
	stopped   bool               // the run has stopped: no task runs again
	replay    []int32            // replay (D46): the recorded pick of every choose and select event
	replaying bool
	epoch     int64 // WallClock at virtual time 0, in UTC milliseconds (manifest, D45)
	rpos      int
	doneG     *g // a test goroutine waiting for the stop (sgoLabWait)
	// Deadlines (sgo_deadline.go): nurseries whose deadline fired at the last advance, cancelled by
	// the driver, a goroutine that is not a task and takes the baton for it; and the number of
	// deadline timers, so pending's capacity is reserved before l.lock is taken.
	pending    []*sgoScope
	driver     *sgoTask
	deadlines  int
	interrupts int // injected interrupts delivered (D179)
}

// sgoLab is nil unless the lab is enabled: only lab binaries enable it (D41), and the runtime's
// own tests (export_sgo_lab_test.go).
var sgoLab *sgoLabState

// sgoPCG is the PCG-DXSM generator of math/rand/v2 (D44); sgo/model/labmodel.go has its vectors.
type sgoPCG struct{ hi, lo uint64 }

func (p *sgoPCG) next() uint64 {
	const (
		mulHi = 2549297995355413924
		mulLo = 4865540595714422341
		incHi = 6364136223846793005
		incLo = 1442695040888963407
	)
	hi, lo := sgoMul64(p.lo, mulLo)
	hi += p.hi*mulLo + p.lo*mulHi
	lo2 := lo + incLo
	c := uint64(0)
	if lo2 < lo {
		c = 1
	}
	p.lo, p.hi = lo2, hi+incHi+c
	h, l := p.hi, p.lo
	const cheapMul = 0xda942042e4dd58b5
	h ^= h >> 32
	h *= cheapMul
	h ^= h >> (3 * 16)
	h *= l | 1
	return h
}

func sgoMul64(x, y uint64) (hi, lo uint64) {
	const mask32 = 1<<32 - 1
	x0, x1 := x&mask32, x>>32
	y0, y1 := y&mask32, y>>32
	w0 := x0 * y0
	t := x1*y0 + w0>>32
	w1, w2 := t&mask32, t>>32
	w1 += x0 * y1
	return x1*y1 + w2 + w1>>32, x * y
}

// uint64n is math/rand/v2's unbiased bounded draw.
func (p *sgoPCG) uint64n(n uint64) uint64 {
	if n&(n-1) == 0 {
		return p.next() & (n - 1)
	}
	hi, lo := sgoMul64(p.next(), n)
	if lo < n {
		thresh := -n % n
		for lo < thresh {
			hi, lo = sgoMul64(p.next(), n)
		}
	}
	return hi
}

func sgoSplitMix64(x *uint64) uint64 {
	*x += 0x9e3779b97f4a7c15
	z := *x
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// sgoLabStreams derives the sched, fault, and data streams of seed (D44): six SplitMix64 outputs.
func sgoLabStreams(seed uint64) (sched, fault, data sgoPCG) {
	x := seed
	sched = sgoPCG{sgoSplitMix64(&x), sgoSplitMix64(&x)}
	fault = sgoPCG{sgoSplitMix64(&x), sgoSplitMix64(&x)}
	data = sgoPCG{sgoSplitMix64(&x), sgoSplitMix64(&x)}
	return
}

// sgoLabEnable starts a lab run in the calling (root) task, before any child exists.
func sgoLabEnable(seed uint64, fifo bool, k, quota int64, exit func(int32)) {
	l := &sgoLabState{fifo: fifo, k: max(k, 1), quota: quota, exit: exit}
	l.sched, _, _ = sgoLabStreams(seed)
	l.events = make([]sgoLabEvent, 0, 4096)
	l.sets = make([]int32, 0, 16384)
	root := sgoCurrent()
	l.tasks = append(l.tasks, root)
	root.labID, root.labReady, root.labG = 0, true, getg()
	l.reserve()
	sgoLab = l
}

// reserve makes room for the events of one operation; it is called only at points that hold no
// runtime lock (the start of an operation), so growing may allocate.
func (l *sgoLabState) reserve() {
	if cap(l.events)-len(l.events) < 256 {
		e := make([]sgoLabEvent, len(l.events), 2*cap(l.events))
		copy(e, l.events)
		l.events = e
	}
	if cap(l.sets)-len(l.sets) < 8*len(l.tasks)+256 {
		s := make([]int32, len(l.sets), 2*cap(l.sets)+8*len(l.tasks))
		copy(s, l.sets)
		l.sets = s
	}
	if cap(l.scratch) < len(l.tasks)+1 {
		l.scratch = make([]int32, 0, 2*len(l.tasks)+16)
	}
	if cap(l.tasks)-len(l.tasks) < 1 {
		l.tasks = append(make([]*sgoTask, 0, 2*len(l.tasks)+16), l.tasks...)
	}
	if cap(l.timers)-len(l.timers) < 1 {
		l.timers = append(make([]sgoLabTimer, 0, 2*len(l.timers)+16), l.timers...)
	}
}

func (l *sgoLabState) emit(e sgoLabEvent) {
	if len(l.events) == cap(l.events) {
		throw("sgo: lab event buffer exhausted")
	}
	l.events = append(l.events, e)
}

func (l *sgoLabState) addSet(ids []int32) [2]int32 {
	start := int32(len(l.sets))
	if cap(l.sets)-len(l.sets) < len(ids) {
		throw("sgo: lab set buffer exhausted")
	}
	l.sets = append(l.sets, ids...)
	return [2]int32{start, int32(len(l.sets))}
}
