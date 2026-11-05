// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import (
	"strings"
	"testing"
)

// groupCases cover no limit, a limit below and at the child count, and every number of results
// read, so an owner that reads nothing is explored too.
func groupCases(f func(k, lim, r int)) {
	for k := 1; k <= 4; k++ {
		for _, lim := range []int{0, 1, 2, k} {
			for r := 0; r <= k; r++ {
				f(k, lim, r)
			}
		}
	}
}

func TestGroupProtocol(t *testing.T) {
	total := 0
	groupCases(func(k, lim, r int) {
		n, err := GroupExplore(k, lim, r)
		if err != nil {
			t.Fatalf("k=%d lim=%d r=%d: %v", k, lim, r, err)
		}
		total += n
	})
	t.Logf("group: %d states", total)
}

func TestTimerRearm(t *testing.T) {
	n, err := TimerExplore()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("timer: %d runs", n)
}

func TestCancelTree(t *testing.T) {
	n, err := CancelTreeExplore()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("cancel tree: %d states", n)
}

func TestDeadlineDriver(t *testing.T) {
	if err := DriverCheck(); err != nil {
		t.Fatal(err)
	}
}

// TestW22Mutations seeds each protocol bug and requires the exploration to catch it with the
// property it breaks.
func TestW22Mutations(t *testing.T) {
	groupFails := func() error {
		var first error
		groupCases(func(k, lim, r int) {
			if first == nil {
				_, first = GroupExplore(k, lim, r)
			}
		})
		return first
	}
	ct := func() error { _, err := CancelTreeExplore(); return err }
	cases := []struct {
		name string
		flag *bool
		run  func() error
		want string
	}{
		{"no refill", &grNoRefill, groupFails, "G4"},
		{"overwrite a full buffer", &grOverwrite, groupFails, "G"},
		{"no slot release", &grNoSlotRelease, groupFails, "G3"},
		{"refill from the newest", &grRefillLast, groupFails, "G2"},
		{"stale timer fire", &tmNoStaleCheck, func() error { _, err := TimerExplore(); return err }, "T"},
		{"inline cancel", &dlInlineCancel, DriverCheck, "D1"},
		{"no deadline inheritance", &cnOwnCauseOnly, ct, "C1"},
		{"cancel not seen inside", &cnCancelOnlySelf, ct, "C1"},
		{"error exit without cancel", &cnExitNoCancel, ct, "C3"},
		{"deadline after close", &cnFireAfterClose, ct, "C4"},
		{"cancel the parent too", &cnCancelParentToo, ct, "C2"},
	}
	for _, c := range cases {
		*c.flag = true
		err := c.run()
		*c.flag = false
		if err == nil {
			t.Errorf("%s: not caught", c.name)
			continue
		}
		if !strings.HasPrefix(err.Error(), c.want) && !strings.Contains(err.Error(), ": "+c.want) {
			t.Errorf("%s: caught as %v, want %s", c.name, err, c.want)
			continue
		}
		t.Logf("%s: %v", c.name, err)
	}
}

func TestCallProtocol(t *testing.T) {
	n, err := CallExplore()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("call: %d states", n)
}

func TestCallMutations(t *testing.T) {
	for _, c := range []struct {
		name string
		flag *bool
		want string
	}{
		{"no NoReply release", &caNoRelease, "P4"},
		{"late withdrawal", &caLateWithdraw, "P2"},
		{"misrouted reply", &caMisroute, "P3"},
		{"second Reply delivers", &caReplyTwice, "P1"},
	} {
		*c.flag = true
		_, err := CallExplore()
		*c.flag = false
		if err == nil {
			t.Errorf("%s: not caught", c.name)
			continue
		}
		if got := err.Error()[:2]; got != c.want {
			t.Errorf("%s: caught as %v, want %s", c.name, err, c.want)
			continue
		}
		t.Logf("%s: %v", c.name, err)
	}
}

func TestPoolLeases(t *testing.T) {
	n, err := PoolExplore()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("pool: %d states", n)
}

func TestPoolMutations(t *testing.T) {
	for _, c := range []struct {
		name string
		flag *bool
		want string
	}{
		{"scope end puts back a returned item", &plReturnTwice, "P3"},
		{"dropped item put back", &plDropReturns, "P2"},
		{"cancelled waiter takes the item", &plCancelLoses, "P2"},
		{"put back without waking a waiter", &plNoWake, "P4"},
	} {
		*c.flag = true
		_, err := PoolExplore()
		*c.flag = false
		if err == nil {
			t.Errorf("%s: not caught", c.name)
			continue
		}
		if got := err.Error()[:2]; got != c.want {
			t.Errorf("%s: caught as %v, want %s", c.name, err, c.want)
			continue
		}
		t.Logf("%s: %v", c.name, err)
	}
}

func TestInterrupt(t *testing.T) {
	n, err := InterruptExplore()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("interrupt: %d states", n)
}

func TestInterruptMutations(t *testing.T) {
	for _, c := range []struct {
		name string
		flag *bool
		want string
	}{
		{"check and park outside the sleep lock", &inSplitPark, "I1"},
		{"second signal queued to the watcher", &inCoalesce, "I3"},
		{"nursery opened after the interrupt misses it", &inNewNursery, "I1"},
		{"root checks miss the virtual root scope", &inNoRootCheck, "I5"},
	} {
		*c.flag = true
		_, err := InterruptExplore()
		*c.flag = false
		if err == nil {
			t.Errorf("%s: not caught", c.name)
			continue
		}
		if got := err.Error()[:2]; got != c.want {
			t.Errorf("%s: caught as %v, want %s", c.name, err, c.want)
			continue
		}
		t.Logf("%s: %v", c.name, err)
	}
}
