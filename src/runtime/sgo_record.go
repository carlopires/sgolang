// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The failure chain and origin of the diagnostic record (SGO-ERR-002, SGO-NUR-004, SGO-DEF-004).
//
// A task's unwinding has one primary failure and a chain of later ones in occurrence order. Two
// sources feed the chain: failures raised by deferred calls while the task unwinds (the earlier
// panics Go keeps on _panic.link), and the failures and errors of children that are not the
// primary of the nursery that joins them (recorded at join time in sgoTask.entries). Every panic
// of a task and every recorded entry takes the next number of the task's failure sequence, so the
// two sources merge in occurrence order. The chain of a child that failed travels with its
// outcome. Recovering an unwinding drops the entries recorded since it began.
//
// The record also names where the primary failure was raised: the task, and the scope that task
// was spawned into (none for the root task). A failure propagated by a nursery keeps the origin
// of the child failure it propagates.

package runtime

// sgoEntry is a chain entry recorded by a join, with its place in the task's failure sequence.
type sgoEntry struct {
	seq  uint64
	info sgoFailureInfo
}

// sgoOrigin is where a failure was raised: a task creation ID and its spawn scope (0: none).
type sgoOrigin struct{ task, scope uint64 }

// sgoOriginOf is the origin of a failure raised by task t itself.
func sgoOriginOf(t *sgoTask) sgoOrigin {
	o := sgoOrigin{task: t.id}
	if t.scope != nil {
		o.scope = t.scope.id
	}
	return o
}

// sgoPanicStart numbers a new panic of task t and records its origin: the one a nursery set for
// the failure it is about to propagate (sgoPropagate), else t itself. It does not allocate.
func sgoPanicStart(t *sgoTask, p *_panic) {
	if t.pend.seq != 0 {
		p.sgoSeq, p.sgoTask, p.sgoScope = t.pend.seq, t.pend.origin.task, t.pend.origin.scope
		t.pend.seq = 0
		return
	}
	t.seq++
	o := sgoOriginOf(t)
	p.sgoSeq, p.sgoTask, p.sgoScope = t.seq, o.task, o.scope
}

// sgoRecovered drops the chain entries recorded since the oldest panic that a recovery discarded
// began (its sequence number is gone): they belong to an unwinding that no longer exists.
func sgoRecovered(t *sgoTask, gone uint64) {
	n := 0
	for _, e := range t.entries {
		if e.seq < gone {
			t.entries[n] = e
			n++
		}
	}
	clear(t.entries[n:])
	t.entries = t.entries[:n]
}

// sgoRecord appends to t's entries the chain contributions of a closed scope's children, in
// creation order: each failed or errored child other than skip (the primary, or nil when the
// owner's own failure is primary) followed by that child's own chain.
func sgoRecord(t *sgoTask, children []*sgoTask, skip *sgoTask) {
	for _, c := range children {
		if c == skip {
			continue
		}
		switch c.outcome {
		case sgoFailed:
			t.seq++
			t.entries = append(t.entries, sgoEntry{t.seq, sgoFailureOf(c.panic)})
		case sgoErrored:
			t.seq++
			t.entries = append(t.entries, sgoEntry{t.seq, sgoFailureInfo{"ChildError", c.err.Error(), ""}})
		default:
			continue
		}
		sgoAppendChain(t, c.chain)
	}
}

// sgoAppendChain records a finished child's chain in t's failure sequence.
func sgoAppendChain(t *sgoTask, chain []sgoFailureInfo) {
	for _, f := range chain {
		t.seq++
		t.entries = append(t.entries, sgoEntry{t.seq, f})
	}
}

// sgoPropagate prepares task t to unwind with the outcome of a closed scope whose primary child is
// c (SGO-NUR-004 rules 2-3): the coming panic keeps c's origin and comes first in the sequence,
// followed by c's own chain and then the other children's failures.
func sgoPropagate(t *sgoTask, children []*sgoTask, c *sgoTask) {
	t.seq++
	t.pend.seq, t.pend.origin = t.seq, c.origin
	sgoAppendChain(t, c.chain)
	sgoRecord(t, children, c)
}

// sgoChain returns the chain of the unwinding p is part of: the later panics on p.link and t's
// recorded entries, merged by sequence number (each source is already in sequence order).
func sgoChain(t *sgoTask, p *_panic) []sgoFailureInfo {
	_, later := sgoUnwound(p)
	var seqs []uint64
	for q := p; q != nil && !q.goexit; q = q.link {
		seqs = append(seqs, q.sgoSeq)
	}
	// seqs is newest first and includes the primary (last); later is oldest first without it.
	var out []sgoFailureInfo
	i, j := 0, 0
	for i < len(later) || j < len(t.entries) {
		if j == len(t.entries) || (i < len(later) && seqs[len(later)-1-i] < t.entries[j].seq) {
			out = append(out, sgoFailureOf(later[i]))
			i++
		} else {
			out = append(out, t.entries[j].info)
			j++
		}
	}
	return out
}

// sgoPrimaryOrigin is the origin recorded on the primary panic of the unwinding p is part of.
func sgoPrimaryOrigin(p *_panic) sgoOrigin {
	q := p
	for q.link != nil && !q.link.goexit {
		q = q.link
	}
	return sgoOrigin{q.sgoTask, q.sgoScope}
}
