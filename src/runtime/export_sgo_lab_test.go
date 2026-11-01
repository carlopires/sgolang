// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test hooks for the lab execution configuration (W08, sgo_lab.go).

package runtime

// SGoLabResult is one lab run: its canonical stream and outcome.
type SGoLabResult struct {
	Lines  []string
	Status int32
	Kind   string
}

// SGoLabRun runs root as the root task of a lab run (seed, policy, K, quota) in a new goroutine
// and waits until the run stops: root ended it (SGoLabEnd), or the lab did (LabDeadlock,
// LabQuotaExhausted). A run the lab stopped leaves its tasks parked for good.
func SGoLabRun(seed uint64, fifo bool, k, quota int64, root func()) SGoLabResult {
	return sgoLabRun(seed, fifo, k, quota, nil, false, root)
}

// SGoLabReplay is SGoLabRun taking every choice from picks (D46, replayPick).
func SGoLabReplay(picks []int32, fifo bool, k, quota int64, root func()) SGoLabResult {
	return sgoLabRun(0, fifo, k, quota, picks, true, root)
}

func sgoLabRun(seed uint64, fifo bool, k, quota int64, picks []int32, replay bool, root func()) SGoLabResult {
	started := make(chan *sgoLabState, 1)
	go func() {
		sgoEnterRoot()
		sgoLabEnable(seed, fifo, k, quota, nil)
		sgoLab.replay, sgoLab.replaying = picks, replay
		started <- sgoLab
		root()
		sgoLeaveRoot()
	}()
	l := <-started
	sgoLabWait(l)
	sgoLab = nil
	// The run has stopped: nothing changes its state any more.
	return SGoLabResult{l.lines(), l.status, l.kind}
}

// SGoLabEnd ends the run with the program's outcome (root only, after its join).
func SGoLabEnd(status int32, kind string) { sgoLabProgramEnd(status, kind) }

// SGoLabOutcomes is the outcome of every child, by lab task ID 1..n.
func SGoLabOutcomes() []string {
	l := sgoLab
	lock(&l.lock)
	defer unlock(&l.lock)
	var out []string
	for _, t := range l.tasks[1:] {
		out = append(out, sgoLabOutcomeOf(t))
	}
	return out
}

func SGoLabStep()  { sgoLabStep() }
func SGoLabPoint() { sgoLabPoint() }

// SGoLabWrite is a write of s (its point, then its event); nothing reaches fd 1.
func SGoLabWrite(s string) {
	sgoLabPoint()
	sgoLabWriteText(s)
}

// SGoLabCancel is n.Cancel() of s (its point, then the cancellation).
func SGoLabCancel(s *SGoScope) error {
	sgoLabPoint()
	return sgoNurseryCancel(s)
}

// SGoLabCheckpoint is task.Checkpoint().
func SGoLabCheckpoint() error {
	sgoLabPoint()
	return sgoTaskCancellation(sgoCurrent())
}
