// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Lab run end, canonical formatting, and the lab paths of Sleep, select, write, and task end.

package runtime

import "unsafe"

// finish records the run's outcome. Called with l.lock held, by the baton holder.
func (l *sgoLabState) finish(status int32, kind string) {
	if l.finished {
		return
	}
	l.finished, l.status, l.kind = true, status, kind
	l.emit(sgoLabEvent{kind: sgoLabOutcome, status: status, outcome: kind})
}

// stop ends the run after finish: it calls the exit hook, which does not return in a lab binary
// (the process exits); in the runtime's tests it reports the result, and the calling task then
// stays parked.
func (l *sgoLabState) stop() {
	l.signalStop()
	gopark(nil, nil, waitReasonSGoJoin, traceBlockForever, 1)
}

// sgoLabProgramEnd records the outcome of a run whose main returned or failed (SGO-PROC-002).
func sgoLabProgramEnd(status int32, kind string) {
	l := sgoLab
	if l == nil || l.finished {
		return
	}
	lock(&l.lock)
	l.finish(status, kind)
	unlock(&l.lock)
	l.signalStop()
}

// sgoLabOutcomeOf is the canonical outcome of a child task (sgo/docs/w08-plan.md).
func sgoLabOutcomeOf(c *sgoTask) string {
	switch c.outcome {
	case sgoSucceeded:
		return "Succeeded"
	case sgoCancelled:
		return "Cancelled"
	case sgoErrored:
		return "Errored"
	case sgoFailed:
		return "Failed:" + sgoFailureOf(c.panic).kind
	}
	return "Running"
}

// sgoLabTaskEnded records the end event of c (called from sgoTaskExit, at c's end point).
func sgoLabTaskEnded(c *sgoTask) {
	l := sgoLab
	if l == nil || l.finished {
		return
	}
	lock(&l.lock)
	l.emit(sgoLabEvent{kind: sgoLabEnd, task: c.labID, outcome: sgoLabOutcomeOf(c)})
	unlock(&l.lock)
}

// sgoLabWriteText records a write of s by the running task (after its point).
func sgoLabWriteText(s string) {
	l := sgoLab
	if l == nil || l.finished {
		return
	}
	lock(&l.lock)
	l.emit(sgoLabEvent{kind: sgoLabWrite, task: sgoCurrent().labID, text: s})
	unlock(&l.lock)
}

// sgoLabSelectChoice chooses among the ready arms (cas indices, ascending) and records it; n == 0
// records a default.
func sgoLabSelectChoice(arms []int32) int {
	l := sgoLab
	lock(&l.lock)
	pick := int32(-1)
	switch {
	case l.replaying:
		p, ok := l.replayPick(arms)
		if !ok {
			// The run ended: the select proceeds as if by default and the task stops at its
			// next point.
			unlock(&l.lock)
			return -1
		}
		pick = p
	case len(arms) > 0:
		pick = arms[0]
		if len(arms) > 1 {
			pick = arms[l.sched.uint64n(uint64(len(arms)))]
		}
	}
	l.emit(sgoLabEvent{kind: sgoLabSelect, task: sgoCurrent().labID, set: l.addSet(arms), pick: pick})
	unlock(&l.lock)
	return int(pick)
}

// sgoLabSelectWoken records the arm by which a waker completed a waiting select (w.arm).
func sgoLabSelectWoken(w *sgoChanWait) {
	l := sgoLab
	if l == nil || w.sel == nil {
		return
	}
	lock(&l.lock)
	one := [1]int32{w.arm}
	l.emit(sgoLabEvent{kind: sgoLabSelect, task: w.task.labID, set: l.addSet(one[:]), pick: w.arm})
	unlock(&l.lock)
}

// sgoLabSleep is Sleep in the lab: a virtual timer (D45). Its point was taken by the caller.
func sgoLabSleep(t *sgoTask, ms int64) error {
	if err := sgoTaskCancellation(t); err != nil {
		return err
	}
	if ms <= 0 {
		return nil
	}
	l := sgoLab
	r := &sgoSleep{g: getg(), task: t}
	lock(&sgoSleepLock)
	if sgoOuterCancelled(t) != nil {
		unlock(&sgoSleepLock)
		return sgoTaskCancellation(t)
	}
	r.next = sgoSleepers
	if sgoSleepers != nil {
		sgoSleepers.prev = r
	}
	sgoSleepers = r
	lock(&l.lock)
	l.timers = append(l.timers, sgoLabTimer{deadline: l.now + ms, id: l.nextTmr, t: t, r: r}) // lock-alloc: ok (capacity reserved)
	l.nextTmr++
	unlock(&l.lock)
	sgoLabBlock(t, "Sleep")
	gopark(sgoLabSleepPark, nil, waitReasonSleep, traceBlockSleep, 1)
	sgoLabAfterPark()
	lock(&sgoSleepLock)
	sgoSleepRemove(r)
	unlock(&sgoSleepLock)
	lock(&l.lock)
	l.dropTimer(r)
	unlock(&l.lock)
	if r.state.Load() == 2 {
		return sgoTaskCancellation(t)
	}
	return nil
}

func sgoLabSleepPark(gp *g, _ unsafe.Pointer) bool {
	unlock(&sgoSleepLock)
	return sgoLabCommit(gp)
}

// Lines formats the canonical stream (for the trace writer and the runtime's tests).
func (l *sgoLabState) lines() []string {
	var out []string
	for _, e := range l.events {
		var b []byte
		set := l.sets[e.set[0]:e.set[1]]
		switch e.kind {
		case sgoLabChoose:
			b = append(b, `{"e":"choose","ready":`...)
			b = sgoLabInts(b, set)
			b = append(b, `,"pick":`...)
			b = sgoLabInt(b, int64(e.pick))
		case sgoLabSelect:
			b = append(b, `{"e":"select","task":`...)
			b = sgoLabInt(b, int64(e.task))
			b = append(b, `,"arms":`...)
			b = sgoLabInts(b, set)
			b = append(b, `,"arm":`...)
			b = sgoLabInt(b, int64(e.pick))
		case sgoLabAdvance:
			b = append(b, `{"e":"advance","to":`...)
			b = sgoLabInt(b, e.to)
		case sgoLabFire:
			b = append(b, `{"e":"fire","task":`...)
			b = sgoLabInt(b, int64(e.task))
			b = append(b, `,"timer":`...)
			b = sgoLabInt(b, int64(e.timer))
		case sgoLabWrite:
			b = append(b, `{"e":"write","task":`...)
			b = sgoLabInt(b, int64(e.task))
			b = append(b, `,"text":`...)
			b = sgoLabQuote(b, e.text)
		case sgoLabEnd:
			b = append(b, `{"e":"end","task":`...)
			b = sgoLabInt(b, int64(e.task))
			b = append(b, `,"outcome":`...)
			b = sgoLabQuote(b, e.outcome)
		case sgoLabOutcome:
			b = append(b, `{"e":"outcome","status":`...)
			b = sgoLabInt(b, int64(e.status))
			b = append(b, `,"kind":`...)
			b = sgoLabQuote(b, e.outcome)
		}
		b = append(b, '}')
		out = append(out, string(b))
	}
	return out
}

func sgoLabInts(b []byte, v []int32) []byte {
	b = append(b, '[')
	for i, x := range v {
		if i > 0 {
			b = append(b, ',')
		}
		b = sgoLabInt(b, int64(x))
	}
	return append(b, ']')
}

func sgoLabInt(b []byte, v int64) []byte {
	if v < 0 {
		b = append(b, '-')
		v = -v
	}
	var d [20]byte
	i := len(d)
	for {
		i--
		d[i] = byte('0' + v%10)
		v /= 10
		if v == 0 {
			break
		}
	}
	return append(b, d[i:]...)
}

// sgoLabQuote quotes s as the model's jsonString does.
func sgoLabQuote(b []byte, s string) []byte {
	const hex = "0123456789abcdef"
	b = append(b, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			b = append(b, '\\', c)
		case c == '\n':
			b = append(b, '\\', 'n')
		case c == '\t':
			b = append(b, '\\', 't')
		case c == '\r':
			b = append(b, '\\', 'r')
		case c < 0x20:
			b = append(b, '\\', 'u', '0', '0', hex[c>>4], hex[c&15])
		default:
			b = append(b, c)
		}
	}
	return append(b, '"')
}

// dropTimer removes the timer of Sleep call r, if it has not fired. Called with l.lock held.
func (l *sgoLabState) dropTimer(r *sgoSleep) {
	for i, tm := range l.timers {
		if tm.r == r {
			copy(l.timers[i:], l.timers[i+1:])
			l.timers[len(l.timers)-1] = sgoLabTimer{}
			l.timers = l.timers[:len(l.timers)-1]
			return
		}
	}
}

// sgoLabDropTimer drops the timer of a sleep a canceller claimed (under sgoSleepLock).
func sgoLabDropTimer(r *sgoSleep) {
	l := sgoLab
	lock(&l.lock)
	l.dropTimer(r)
	unlock(&l.lock)
}

// sgoLabUnsupported ends the run with UnsupportedLabEffect (D45, D47); it does not return.
func sgoLabUnsupported() {
	l := sgoLab
	lock(&l.lock)
	l.finish(2, "UnsupportedLabEffect")
	unlock(&l.lock)
	l.stop()
}
