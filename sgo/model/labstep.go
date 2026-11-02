// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import "slices"

// next runs one decision: if no task is ready, time advances to the earliest timer and every timer
// at that deadline fires in timer-ID order; with no timer either, the run ends LabDeadlock.
// Otherwise it picks a ready task and runs one step of it. It reports false once the run ended.
func (s *lState) next() (bool, error) {
	if s.finished {
		return false, nil
	}
	ready := s.ready()
	if len(ready) == 0 {
		if len(s.timers) == 0 {
			s.finish(2, "LabDeadlock")
			return false, nil
		}
		d := s.timers[0].deadline
		for _, tm := range s.timers {
			d = min(d, tm.deadline)
		}
		s.now = d
		s.emit(LEvent{Kind: "advance", To: d})
		var due []lTimer
		s.timers = slices.DeleteFunc(s.timers, func(tm lTimer) bool {
			if tm.deadline == d {
				due = append(due, tm)
				return true
			}
			return false
		})
		slices.SortFunc(due, func(a, b lTimer) int { return a.id - b.id })
		for _, tm := range due {
			s.emit(LEvent{Kind: "fire", Task: tm.task, Timer: tm.id})
			s.makeReady(s.tasks[tm.task])
		}
		ready = s.ready()
	}
	var pick int
	if s.sc.Policy == "fifo" {
		pick = ready[0]
		for _, id := range ready {
			if s.tasks[id].readySeq < s.tasks[pick].readySeq {
				pick = id
			}
		}
	} else {
		var err error
		if pick, err = s.ch.Pick(ready); err != nil {
			return false, err
		}
	}
	s.emit(LEvent{Kind: "choose", Ready: ready, Pick: pick})
	if err := s.step(s.tasks[pick]); err != nil {
		return false, err
	}
	return !s.finished, nil
}

// step runs the chosen task: the operation at its point, if any, then its code up to its next
// point. A task that stays ready re-enters the ready order behind the others (fifo).
func (s *lState) step(t *lTask) error {
	if t.joinDone {
		t.joinDone = false
		s.finishMain()
		return nil
	}
	if t.atOp {
		t.atOp = false
		if err := s.exec(t); err != nil {
			return err
		}
	}
	if !s.finished && t.status == tReady && !t.atOp {
		s.segment(t)
	}
	if t.status == tReady && !s.finished {
		s.makeReady(t)
	}
	return nil
}

// segment runs t's code up to its next point: work steps (the quota is checked first; every K
// steps since the last decision is a point, D43), then the point of the next operation or end.
func (s *lState) segment(t *lTask) {
	since := 0
	k := max(s.sc.K, 1)
	for {
		if t.endWith != "" {
			if t.id == 0 {
				s.mainExits(t, t.endWith)
			} else {
				t.atOp = true // the end point
			}
			return
		}
		if t.pc >= len(t.script) || t.script[t.pc].Kind != LWork {
			t.atOp = true
			return
		}
		if t.rem < 0 {
			t.rem = t.script[t.pc].N
		}
		take := min(t.rem, k-since)
		if s.used+take > s.sc.Quota {
			s.finish(2, "LabQuotaExhausted")
			return
		}
		s.used += take
		since += take
		t.rem -= take
		if t.rem == 0 {
			t.pc++
			t.rem = -1
		}
		if since >= k {
			return // a yield point; the task stays ready mid-segment
		}
	}
}

// exec runs the operation at t's point.
func (s *lState) exec(t *lTask) error {
	if t.endWith != "" {
		s.endChild(t, t.endWith)
		return nil
	}
	if t.pc >= len(t.script) { // the callback returned nil
		s.endChild(t, "Succeeded")
		return nil
	}
	op := t.script[t.pc]
	switch op.Kind {
	case lSpawn:
		if !s.cancelled { // spawning into a cancelled nursery creates no child (SGO-SPAWN-002)
			c := &lTask{id: len(s.tasks), script: s.sc.Tasks[op.N], rem: -1}
			s.tasks = append(s.tasks, c)
			s.makeReady(c)
		}
		t.pc++
	case lJoin:
		t.inBody = false
		if s.childrenDone() {
			s.finishMain()
			return nil
		}
		t.status = tJoining
	case LSleep:
		if s.cancelledFor(t) {
			t.endWith = unwound(t) // Sleep returns the cancellation error, which the task returns
			return nil
		}
		t.pc++
		if op.N > 0 {
			s.timers = append(s.timers, lTimer{deadline: s.now + int64(op.N), id: s.nextTimer, task: t.id})
			s.nextTimer++
			t.status = tSleeping
		}
	case LCheckpoint:
		if s.cancelledFor(t) {
			t.endWith = unwound(t)
			return nil
		}
		t.pc++
	case LWrite:
		s.emit(LEvent{Kind: "write", Task: t.id, Text: op.Text})
		s.out.WriteString(op.Text)
		t.pc++
	case LCancel:
		s.cancel()
		t.pc++
	case LError:
		if t.id == 0 {
			s.mainExits(t, "errored")
		} else {
			s.endChild(t, "Errored")
		}
	case LSend, LRecv, LClose, LSelect:
		return s.chanStep(t, op)
	}
	return nil
}

// block parks t on its operation, or, if it observes cancellation, ends the operation with no
// effect so that t's next point unwinds it (D33).
func (s *lState) block(t *lTask, waits []chWait) {
	if s.cancelledFor(t) {
		t.endWith = unwound(t)
		return
	}
	for _, w := range waits {
		c := s.chans[w.ch()]
		c.waiters = append(c.waiters, w.lWait)
	}
	t.status = tBlocked
}
