package model

import "slices"

// next runs one choice point: if no task is ready, time advances to the earliest timer and every
// timer at that deadline fires in timer-ID order; with no timer either, the run ends LabDeadlock.
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

// step runs one operation of t. A task that stays ready re-enters the ready set behind the others
// (its ready sequence is renewed, which the fifo policy uses).
func (s *lState) step(t *lTask) error {
	switch t.pend {
	case pUnwind:
		t.pend = pNone
		s.unwind(t)
		return nil
	case pClosedFail:
		t.pend = pNone
		s.fail(t, "ClosedChannel")
		return nil
	}
	if t.pc >= len(t.script) { // a child's callback returned nil
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
		if s.childrenDone() {
			s.finishMain()
			return nil
		}
		t.inBody = false
		t.status = tJoining
		return nil
	case LWork:
		if t.rem < 0 {
			t.rem = op.N
		}
		chunk := min(t.rem, max(s.sc.K, 1))
		if s.used+chunk > s.sc.Quota {
			s.finish(2, "LabQuotaExhausted")
			return nil
		}
		s.used += chunk
		t.rem -= chunk
		if t.rem == 0 {
			t.pc++
			t.rem = -1
		}
	case LSleep:
		if s.cancelledFor(t) {
			s.unwind(t) // Sleep returns the cancellation error, which the task returns
			return nil
		}
		t.pc++
		if op.N > 0 {
			s.timers = append(s.timers, lTimer{deadline: s.now + int64(op.N), id: s.nextTimer, task: t.id})
			s.nextTimer++
			t.status = tSleeping
			return nil
		}
	case LCheckpoint:
		if s.cancelledFor(t) {
			s.unwind(t)
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
		return nil
	case LSend, LRecv, LClose, LSelect:
		return s.chanStep(t, op)
	}
	if t.status == tReady {
		s.makeReady(t)
	}
	return nil
}

// unwind ends t's operation with no effect and ends the task Cancelled (main: leaves the body).
func (s *lState) unwind(t *lTask) {
	if t.id == 0 {
		s.mainExits(t, "cancelled")
		return
	}
	s.endChild(t, "Cancelled")
}

// block parks t on its operation, or unwinds it if it observes cancellation (D33).
func (s *lState) block(t *lTask, waits []chWait) {
	if s.cancelledFor(t) {
		s.unwind(t)
		return
	}
	for _, w := range waits {
		c := s.chans[w.ch()]
		c.waiters = append(c.waiters, w.lWait)
	}
	t.status = tBlocked
}
