// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Native channel operations and select (W07); see sgo_native.go.

package runtime

import "unsafe"

const (
	sgoNotReady = iota
	sgoDone
	sgoClosedSend
)

// sgoNativeTrySend publishes box without waiting, under c.lock: to a waiting receiver, else into
// free buffer space. It returns the receiver to ready.
func sgoNativeTrySend(c *sgoChan, box unsafe.Pointer) (*sgoChanWait, int) {
	if c.closed {
		return nil, sgoClosedSend
	}
	for r := c.recvs.first; r != nil; r = r.next {
		if r.claim(sgoWaitDone) {
			c.recvs.remove(r)
			r.val, r.delivered = box, true
			sgoLabSelectWoken(r)
			return r, sgoDone
		}
	}
	if c.count < c.cap {
		c.qpush(box)
		sgoChanCheck(c)
		return nil, sgoDone
	}
	return nil, sgoNotReady
}

// sgoNativeTryRecv receives without waiting, under c.lock: the oldest queued value (refilling the
// buffer from a waiting sender), a waiting sender's value, or closed (ok == false).
func sgoNativeTryRecv(c *sgoChan) (box unsafe.Pointer, ok bool, wake *sgoChanWait, ready bool) {
	if a := c.tmr; a != nil { // a timer (D160): its value once the deadline has passed
		if sgoAfterDue(a) {
			a.fired = true
			return a.box, true, nil, true
		}
		return nil, false, nil, false
	}
	if c.count > 0 {
		box = c.qpop()
		if q := c.grp; q != nil && q.first != nil { // a group's queued results (D172)
			c.qpush(q.pop())
		}
		for s := c.senders.first; s != nil; s = s.next {
			if s.claim(sgoWaitDone) {
				c.senders.remove(s)
				c.qpush(s.box)
				s.box, wake = nil, s
				sgoLabSelectWoken(s)
				break
			}
		}
		sgoChanCheck(c)
		return box, true, wake, true
	}
	for s := c.senders.first; s != nil; s = s.next {
		if s.claim(sgoWaitDone) {
			c.senders.remove(s)
			box, s.box = s.box, nil
			sgoLabSelectWoken(s)
			return box, true, s, true
		}
	}
	if c.closed {
		return nil, false, nil, true
	}
	return nil, false, nil, false
}

func sgoReadyOne(w *sgoChanWait) {
	if w != nil {
		sgoWake(w.g)
	}
}

// sgochansend1 is ch <- v.
func sgochansend1(c *sgoChan, elem unsafe.Pointer) {
	sgoLabPoint()
	t := sgoCurrent()
	box := sgoNativeBox(c, elem)
	if raceenabled {
		racereleasemerge(box)
	}
	sgoRaceRendezvousStart(c)
	// The wait record is allocated with c.lock released, then the send is retried: nothing is
	// allocated under a runtime lock (W12 R9).
	var w *sgoChanWait
	lock(&c.lock)
	for {
		wake, st := sgoNativeTrySend(c, box)
		if st != sgoNotReady {
			unlock(&c.lock)
			sgoReadyOne(wake)
			if st == sgoClosedSend {
				sgoClosedChannel()
			}
			sgoRaceRendezvousEnd(c)
			sgoChanInject(sgoFaultAfterCommit)
			return
		}
		if w != nil {
			break
		}
		unlock(&c.lock)
		w = &sgoChanWait{task: t, box: box}
		lock(&c.lock)
	}
	if !sgoChanPark(c, &c.senders, w, waitReasonChanSend, false) || w.state.Load() == sgoWaitCancelled {
		unlock(&c.lock)
		sgoNativeCancel(t)
	}
	closed := w.closed
	unlock(&c.lock)
	if closed {
		sgoClosedChannel()
	}
	sgoRaceRendezvousEnd(c) // a rendezvous receive happens before the send completes
	sgoChanInject(sgoFaultAfterCommit)
}

// sgochanrecv2 is v, ok := <-ch (and each step of range).
func sgochanrecv2(c *sgoChan, dst unsafe.Pointer) bool {
	if c.tmr != nil { // a one-arm select on a timer, lowered to a receive (D160)
		defer sgoAfterStop(c)
	}
	sgoLabPoint()
	t := sgoCurrent()
	sgoRaceRendezvousStart(c)
	var w *sgoChanWait // allocated with c.lock released, then the receive is retried (W12 R9)
	lock(&c.lock)
	box, ok, wake, ready := sgoNativeTryRecv(c)
	for !ready && w == nil {
		unlock(&c.lock)
		w = &sgoChanWait{task: t}
		lock(&c.lock)
		box, ok, wake, ready = sgoNativeTryRecv(c)
	}
	if !ready {
		if !sgoChanPark(c, &c.recvs, w, waitReasonChanReceive, false) || w.state.Load() == sgoWaitCancelled {
			unlock(&c.lock)
			sgoNativeCancel(t)
		}
		box, ok = w.val, w.delivered
		w.val = nil
	}
	unlock(&c.lock)
	sgoReadyOne(wake)
	if ok {
		sgoNativeTake(c, dst, box)
		if raceenabled {
			racereleasemerge(box)
		}
	} else {
		sgoNativeZero(c, dst)
	}
	sgoRaceRendezvousEnd(c)
	sgoChanInject(sgoFaultAfterReceive)
	return ok
}

// sgochanrecv1 is <-ch.
func sgochanrecv1(c *sgoChan, dst unsafe.Pointer) {
	sgochanrecv2(c, dst)
}

// sgoclosechan is close(ch); closing a closed channel has no effect (D35).
func sgoclosechan(c *sgoChan) {
	sgoLabPoint()
	sgoChanInject(sgoFaultBeforeClose)
	if raceenabled {
		racerelease(unsafe.Pointer(c))
	}
	lock(&c.lock)
	if c.closed {
		unlock(&c.lock)
		return
	}
	c.closed = true
	var wake *sgoChanWait
	for s := c.senders.first; s != nil; {
		next := s.next
		if s.claim(sgoWaitDone) {
			c.senders.remove(s)
			s.closed, s.box = true, nil
			sgoLabSelectWoken(s)
			s.wake, wake = wake, s
		}
		s = next
	}
	for r := c.recvs.first; r != nil; {
		next := r.next
		if r.claim(sgoWaitDone) {
			c.recvs.remove(r)
			r.delivered = false
			sgoLabSelectWoken(r)
			r.wake, wake = wake, r
		}
		r = next
	}
	sgoChanCheck(c)
	unlock(&c.lock)
	sgoChanReady(wake)
	sgoChanInject(sgoFaultAfterClose)
}

// sgoScase is one arm of a select, as laid out by walk (cmd/compile/internal/walk/select.go).
type sgoScase struct {
	c    *sgoChan
	elem unsafe.Pointer
}

// sgoSelLocks returns the distinct channels of cas in address order (N06: duplicate arms).
func sgoSelLocks(cas []sgoScase) []*sgoChan {
	var cs []*sgoChan
	for i := range cas {
		c := cas[i].c
		j := len(cs)
		for j > 0 && uintptr(unsafe.Pointer(cs[j-1])) > uintptr(unsafe.Pointer(c)) {
			j--
		}
		if j > 0 && cs[j-1] == c {
			continue
		}
		cs = append(cs, nil)
		copy(cs[j+1:], cs[j:])
		cs[j] = c
	}
	return cs
}

func sgoSelLock(cs []*sgoChan) {
	for _, c := range cs {
		lock(&c.lock)
	}
}

func sgoSelUnlock(cs []*sgoChan) {
	for i := len(cs) - 1; i >= 0; i-- {
		unlock(&cs[i].lock)
	}
}

// sgoSelCanProceed reports, under the arm's channel lock, whether the arm can proceed now.
func sgoSelCanProceed(c *sgoChan, send bool) bool {
	if c.tmr != nil {
		return !send && sgoAfterDue(c.tmr)
	}
	if c.closed {
		return true
	}
	if send {
		for r := c.recvs.first; r != nil; r = r.next {
			if r.waiting() {
				return true
			}
		}
		return c.count < c.cap
	}
	if c.count > 0 {
		return true
	}
	for s := c.senders.first; s != nil; s = s.next {
		if s.waiting() {
			return true
		}
	}
	return false
}

type sgoSelPark struct {
	locks []*sgoChan
	rep   *sgoChanWait
}

func sgoSelParkUnlock(gp *g, p unsafe.Pointer) bool {
	sp := (*sgoSelPark)(p)
	unlock(&sgoSleepLock)
	sgoSelUnlock(sp.locks)
	if sgoLab != nil {
		return sgoLabCommit(gp)
	}
	return true
}

// sgoselectgo is a select: the first nsends arms are sends, then nrecvs receives. It returns the
// chosen arm and, for a receive, ok; -1 if block is false and no arm can proceed (default).
func sgoselectgo(cas0 *sgoScase, order0 *uint16, pc0 *uintptr, nsends, nrecvs int, block bool) (int, bool) {
	if sgoSelHasTimer(cas0, nsends, nrecvs) { // timer arms are dropped when select ends (D160)
		return sgoselectgoTimers(cas0, order0, pc0, nsends, nrecvs, block)
	}
	return sgoselectgo1(cas0, order0, pc0, nsends, nrecvs, block)
}

func sgoselectgoTimers(cas0 *sgoScase, order0 *uint16, pc0 *uintptr, nsends, nrecvs int, block bool) (int, bool) {
	defer sgoSelStopTimers(cas0, nsends, nrecvs)
	return sgoselectgo1(cas0, order0, pc0, nsends, nrecvs, block)
}

func sgoselectgo1(cas0 *sgoScase, order0 *uint16, pc0 *uintptr, nsends, nrecvs int, block bool) (int, bool) {
	sgoLabPoint()
	t := sgoCurrent()
	n := nsends + nrecvs
	var cas []sgoScase
	if n > 0 {
		cas = unsafe.Slice(cas0, n)
	}
	boxes := make([]unsafe.Pointer, nsends)
	for i := 0; i < nsends; i++ {
		boxes[i] = sgoNativeBox(cas[i].c, cas[i].elem)
		if raceenabled {
			racereleasemerge(boxes[i])
		}
	}
	// The lab names arms by source index (D50): order0 holds each case's index in the select
	// statement as written (walk/select.go). Allocated before any channel lock is taken.
	var src, arms []int32
	if sgoLab != nil {
		src = sgoSelSourceIndex(order0, len(cas))
		arms = make([]int32, 0, len(cas))
	}
	locks := sgoSelLocks(cas)
	for _, c := range locks {
		sgoRaceRendezvousStart(c)
	}
	// A blocking select allocates its wait records before taking the arm locks: nothing is
	// allocated while a channel lock is held (W12 R9).
	var sel *sgoSel
	var ws []sgoChanWait
	var sp *sgoSelPark
	if block {
		sel = &sgoSel{}
		ws = make([]sgoChanWait, n+1) // ws[n]: the cancellation registration (an empty select has no arm)
		sp = &sgoSelPark{locks: locks}
	}
	sgoSelLock(locks)
	// Uniform choice among the arms that can proceed (D34), by reservoir sampling.
	pick, ready := -1, uint32(0)
	if sgoLab != nil {
		// The lab records the ready arms (source indices, ascending) and chooses from the sched
		// stream (D34, D43).
		for i := range cas {
			if sgoSelCanProceed(cas[i].c, i < nsends) {
				arms = append(arms, src[i]) // lock-alloc: ok (preallocated, bounded by len(cas))
			}
		}
		sgoSortInt32(arms)
		if len(arms) > 0 || !block {
			if a := sgoLabSelectChoice(arms); a >= 0 {
				for i := range src {
					if src[i] == int32(a) {
						pick = i
					}
				}
			}
		}
	} else {
		for i := range cas {
			if sgoSelCanProceed(cas[i].c, i < nsends) {
				ready++
				if cheaprandn(ready) == 0 {
					pick = i
				}
			}
		}
	}
	if pick >= 0 {
		c := cas[pick].c
		if pick < nsends {
			wake, st := sgoNativeTrySend(c, boxes[pick])
			sgoSelUnlock(locks)
			sgoReadyOne(wake)
			if st == sgoClosedSend {
				sgoClosedChannel()
			}
			sgoRaceRendezvousEnd(c)
			return pick, false
		}
		box, ok, wake, _ := sgoNativeTryRecv(c)
		sgoSelUnlock(locks)
		sgoReadyOne(wake)
		sgoSelFinishRecv(c, cas[pick].elem, box, ok)
		sgoRaceRendezvousEnd(c)
		return pick, ok
	}
	if !block {
		sgoSelUnlock(locks)
		return -1, false
	}
	// Wait on every arm under one decision.
	gp := getg()
	for i := range ws {
		w := &ws[i]
		w.g, w.task, w.sel, w.arm = gp, t, sel, int32(i)
		if i < len(src) {
			w.arm = src[i] // the arm's source index (D50); ws[n] is no arm
		}
		if i == n {
			break
		}
		w.ch = cas[i].c
		if i < nsends {
			w.box = boxes[i]
			cas[i].c.senders.push(w)
		} else {
			cas[i].c.recvs.push(w)
		}
	}
	rep := &ws[n]
	lock(&sgoSleepLock)
	if sgoOuterCancelled(t) != nil {
		unlock(&sgoSleepLock)
		sgoSelRemove(cas, ws, nsends)
		sgoSelUnlock(locks)
		sgoNativeCancel(t)
	}
	rep.cnext = sgoChanWaiters
	if sgoChanWaiters != nil {
		sgoChanWaiters.cprev = rep
	}
	sgoChanWaiters = rep
	sp.rep = rep
	if sgoLab != nil {
		sgoLabBlock(t, "select")
	}
	gopark(sgoSelParkUnlock, unsafe.Pointer(sp), waitReasonSelect, traceBlockSelect, 1)
	sgoLabAfterPark()
	// Loser cleanup: no arm stays registered once select returns.
	sgoSelLock(locks)
	lock(&sgoSleepLock)
	sgoChanUnregister(rep)
	unlock(&sgoSleepLock)
	sgoSelRemove(cas, ws, nsends)
	w := sel.winner
	st := sel.state.Load()
	var box unsafe.Pointer
	var ok, closed bool
	if w != nil {
		box, ok, closed = w.val, w.delivered, w.closed
	}
	sgoSelUnlock(locks)
	if st == sgoWaitCancelled {
		sgoNativeCancel(t)
	}
	i := -1
	for j := range ws[:n] {
		if &ws[j] == w {
			i = j
		}
	}
	if i < 0 {
		throw("sgo: select woken without a winning arm")
	}
	if i < nsends {
		if closed {
			sgoClosedChannel()
		}
		sgoRaceRendezvousEnd(cas[i].c)
		return i, false
	}
	sgoSelFinishRecv(cas[i].c, cas[i].elem, box, ok)
	sgoRaceRendezvousEnd(cas[i].c)
	return i, ok
}

func sgoSelRemove(cas []sgoScase, ws []sgoChanWait, nsends int) {
	for i := range cas {
		if i < nsends {
			cas[i].c.senders.remove(&ws[i])
		} else {
			cas[i].c.recvs.remove(&ws[i])
		}
	}
}

func sgoSelFinishRecv(c *sgoChan, dst, box unsafe.Pointer, ok bool) {
	if ok {
		sgoNativeTake(c, dst, box)
		if raceenabled {
			racereleasemerge(box)
		}
	} else {
		sgoNativeZero(c, dst)
	}
}

// sgoselectnbsend is select { case ch <- v: ...; default: ... }.
func sgoselectnbsend(c *sgoChan, elem unsafe.Pointer) bool {
	cas := [1]sgoScase{{c, elem}}
	i, _ := sgoselectgo(&cas[0], nil, nil, 1, 0, false)
	return i == 0
}

// sgoselectnbrecv is select { case v, ok = <-ch: ...; default: ... }.
func sgoselectnbrecv(elem unsafe.Pointer, c *sgoChan) (selected, received bool) {
	cas := [1]sgoScase{{c, elem}}
	i, ok := sgoselectgo(&cas[0], nil, nil, 0, 1, false)
	return i == 0, ok
}

// sgoblock is select {}: it waits until the task is cancelled, then unwinds (D33, D34).
func sgoblock() {
	sgoselectgo(nil, nil, nil, 0, 0, true)
}

// sgoSelSourceIndex is the source index of each of the n cases of a select (D50): order0[k] for
// compiled S-Go code, k when there is no order array.
func sgoSelSourceIndex(order0 *uint16, n int) []int32 {
	src := make([]int32, n)
	for k := range src {
		src[k] = int32(k)
		if order0 != nil {
			src[k] = int32(*(*uint16)(add(unsafe.Pointer(order0), uintptr(k)*2)))
		}
	}
	return src
}

// sgoSortInt32 sorts a short slice ascending (insertion sort: it does not allocate).
func sgoSortInt32(v []int32) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}
