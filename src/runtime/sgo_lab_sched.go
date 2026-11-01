// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The lab's decisions and baton hand-off (sgo_lab.go). Only the baton holder changes lab state;
// l.lock orders it with the goroutines waiting for the baton. A task waiting for the baton is in
// one of two places: parked by the lab (labParked, at a decision or at its start), or parked in an
// S-Go wait that a waker completed (labWoken). Either way it runs only when chosen.

package runtime

// readyIDs is the canonical ready set: lab task IDs, ascending.
func (l *sgoLabState) readyIDs() []int32 {
	ids := l.scratch[:0]
	for _, t := range l.tasks {
		if t.labReady {
			ids = append(ids, t.labID)
		}
	}
	return ids
}

// wakeLocked makes t ready; its goroutine is parked in an S-Go wait.
func (l *sgoLabState) wakeLocked(t *sgoTask) {
	t.labReady = true
	t.labWoken = true
	l.seq++
	t.labSeq = l.seq
}

// decide is one decision (D42): with no ready task, virtual time advances to the earliest timer
// and every timer due then fires in timer-ID order (D45); with no timer either, the run ends
// LabDeadlock (nil). Otherwise the next task is chosen and the choose event recorded.
func (l *sgoLabState) decide() *sgoTask {
	ids := l.readyIDs()
	if len(ids) == 0 {
		if len(l.timers) == 0 {
			l.finish(2, "LabDeadlock")
			return nil
		}
		d := l.timers[0].deadline
		for _, tm := range l.timers {
			d = min(d, tm.deadline)
		}
		l.now = d
		l.emit(sgoLabEvent{kind: sgoLabAdvance, to: d})
		// Timers are kept in timer-ID order; fire the due ones in that order.
		n := 0
		for _, tm := range l.timers {
			if tm.deadline != d {
				l.timers[n] = tm
				n++
				continue
			}
			l.emit(sgoLabEvent{kind: sgoLabFire, task: tm.t.labID, timer: int32(tm.id)})
			if tm.r != nil && tm.r.state.CompareAndSwap(0, 1) {
				l.wakeLocked(tm.t)
			}
		}
		clear(l.timers[n:])
		l.timers = l.timers[:n]
		ids = l.readyIDs()
		if len(ids) == 0 {
			return l.decide()
		}
	}
	pick := ids[0]
	switch {
	case l.replaying:
		p, ok := l.replayPick(ids)
		if !ok {
			return nil
		}
		pick = p
	case l.fifo:
		for _, id := range ids {
			if l.tasks[id].labSeq < l.tasks[pick].labSeq {
				pick = id
			}
		}
	case len(ids) > 1:
		pick = ids[l.sched.uint64n(uint64(len(ids)))]
	}
	l.emit(sgoLabEvent{kind: sgoLabChoose, set: l.addSet(ids), pick: pick})
	return l.tasks[pick]
}

// transfer gives the baton to p and returns the goroutine to ready after l.lock is released, or
// nil if p will take the baton itself (it has not parked yet).
func (l *sgoLabState) transfer(p *sgoTask) *g {
	p.labSince = 0
	switch {
	case p.labWoken:
		p.labWoken = false
		if p.labInWait { // parked in its S-Go wait (sgoLabCommit)
			p.labInWait = false
			return p.labG
		}
		p.labRun = true // still on its way to park: its commit aborts the park
		return nil
	case p.labParked:
		p.labParked = false
		return p.labG
	}
	p.labRun = true
	return nil
}

// waitBaton parks the calling task t until it is chosen. Called with l.lock held; returns
// without it.
func (l *sgoLabState) waitBaton(t *sgoTask) {
	if t.labRun {
		t.labRun = false
		unlock(&l.lock)
		return
	}
	t.labParked = true
	goparkunlock(&l.lock, waitReasonSGoJoin, traceBlockSync, 2)
}

// sgoLabPoint is the decision before an operation (or a yield) of the running task: it stays
// ready behind the others (fifo), and if another task is chosen the baton passes to it and the
// caller waits until it is chosen again.
func sgoLabPoint() {
	l := sgoLab
	if l == nil {
		return
	}
	if l.finished {
		l.stop() // the run has ended: no task runs on
		return
	}
	t := sgoCurrent()
	l.reserve()
	lock(&l.lock)
	l.seq++
	t.labSeq = l.seq
	p := l.decide()
	if p == nil {
		unlock(&l.lock)
		l.stop()
		return
	}
	if p == t {
		t.labSince = 0
		unlock(&l.lock)
		return
	}
	gp := l.transfer(p)
	if gp != nil {
		// Ready p only once this task is accounted as waiting: p may hand the baton straight back.
		unlock(&l.lock)
		goready(gp, 0)
		lock(&l.lock)
	}
	l.waitBaton(t)
}

// sgoLabBlock is the decision made by a task about to park in an S-Go wait (the caller may hold
// the wait's locks): it is no longer ready, and the baton passes to the chosen task. The caller
// then parks, and its park commit calls sgoLabCommit. If no task can be chosen, the run ends
// (LabDeadlock); the commit stops it, once the wait's locks are released.
func sgoLabBlock(t *sgoTask) {
	l := sgoLab
	if l.finished {
		return
	}
	lock(&l.lock)
	t.labReady = false
	t.labG = getg()
	p := l.decide()
	var gp *g
	if p != nil {
		gp = l.transfer(p)
	}
	unlock(&l.lock)
	if gp != nil {
		goready(gp, 0)
	}
}

// sgoLabCommit is the lab part of every S-Go park commit, after the wait's locks are released: it
// aborts the park (false) if the task was woken and chosen before it parked, or if the run has
// ended (the task then stops the run on its own stack, sgoLabAfterPark); otherwise the task is
// parked in its wait. It runs on the system stack and does not allocate.
func sgoLabCommit(gp *g) bool {
	l := sgoLab
	t := gp.sgo
	lock(&l.lock)
	if l.finished {
		unlock(&l.lock)
		return false
	}
	if t.labRun {
		t.labRun = false
		unlock(&l.lock)
		return false
	}
	t.labInWait = true
	unlock(&l.lock)
	return true
}

// sgoLabAfterPark follows every lab park: if the run ended while the task was going to park (its
// commit aborted the park), the task stops the run, which ends the process in a lab binary.
func sgoLabAfterPark() {
	if l := sgoLab; l != nil && l.finished {
		l.stop()
	}
}

// signalStop ends the process (lab binaries: the exit hook writes the trace) or wakes the test
// goroutine waiting for the run. Called on a task's own stack with no runtime lock held.
func (l *sgoLabState) signalStop() {
	if l.exit != nil {
		l.exit(l.status)
	}
	lock(&l.lock)
	if l.stopped {
		unlock(&l.lock)
		return
	}
	l.stopped = true
	gp := l.doneG
	l.doneG = nil
	unlock(&l.lock)
	if gp != nil {
		goready(gp, 0)
	}
}

// sgoLabWait parks the calling (non-task) goroutine until the run stops or the root ends it.
func sgoLabWait(l *sgoLabState) {
	lock(&l.lock)
	if l.stopped {
		unlock(&l.lock)
		return
	}
	l.doneG = getg()
	goparkunlock(&l.lock, waitReasonSGoJoin, traceBlockForever, 1)
}

// sgoLabEndTask is the decision after the calling task's end event: it leaves the lab and the
// baton passes on; the goroutine then exits.
func sgoLabEndTask(t *sgoTask) {
	l := sgoLab
	if l == nil {
		return
	}
	if l.finished {
		l.stop()
		return
	}
	lock(&l.lock)
	t.labReady = false
	t.labDone = true
	p := l.decide()
	var gp *g
	if p != nil {
		gp = l.transfer(p)
	}
	unlock(&l.lock)
	if p == nil {
		l.stop()
		return
	}
	if gp != nil {
		goready(gp, 0)
	}
}

// sgoWake readies gp, an S-Go task parked in an S-Go wait: in the lab it only becomes ready and
// runs when chosen.
func sgoWake(gp *g) {
	l := sgoLab
	if l == nil || gp.sgo == nil {
		goready(gp, 0)
		return
	}
	lock(&l.lock)
	l.wakeLocked(gp.sgo)
	unlock(&l.lock)
}

// sgoLabSpawned registers child c (ready, not yet running) in the run of its spawner.
func sgoLabSpawned(c *sgoTask) {
	l := sgoLab
	if l == nil {
		return
	}
	lock(&l.lock)
	c.labID = int32(len(l.tasks))
	l.tasks = append(l.tasks, c) // capacity reserved at the spawn point
	c.labReady = true
	l.seq++
	c.labSeq = l.seq
	unlock(&l.lock)
}

// sgoLabStart parks a new child's goroutine until it is chosen.
func sgoLabStart(c *sgoTask) {
	l := sgoLab
	if l == nil {
		return
	}
	lock(&l.lock)
	c.labG = getg()
	l.waitBaton(c)
}

// sgoLabStep counts one step of the running task (a loop back-edge or function entry of S-Go
// code, D43): the quota bounds the run, and every K steps since the last decision is a yield.
func sgoLabStep() {
	l := sgoLab
	if l == nil {
		return
	}
	if l.finished {
		l.stop()
		return
	}
	t := sgoCurrent()
	l.used++
	if l.used > l.quota {
		lock(&l.lock)
		l.finish(2, "LabQuotaExhausted")
		unlock(&l.lock)
		l.stop()
		return
	}
	t.labSince++
	if t.labSince >= l.k {
		sgoLabPoint()
	}
}

// replayPick is the decision a replay takes from the trace (D46): a set of two or more consumes
// the next recorded pick, which must be in the set; a single element or an empty select set
// (default) is forced and consumes nothing, so the driver records exactly the picks of the
// events whose set has two or more elements. A pick outside the set, or no pick left, ends the
// run with LabReplayDivergence; any other divergence shows in the driver's comparison of the
// whole stream with the trace. Called with l.lock held.
func (l *sgoLabState) replayPick(set []int32) (int32, bool) {
	switch len(set) {
	case 0:
		return -1, true
	case 1:
		return set[0], true
	}
	if l.rpos < len(l.replay) {
		p := l.replay[l.rpos]
		l.rpos++
		for _, id := range set {
			if id == p {
				return p, true
			}
		}
	}
	l.finish(2, "LabReplayDivergence")
	return 0, false
}
