// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import (
	"strings"
	"testing"
)

func snd(ch, v int) NOp { return NOp{Kind: NSend, Ch: ch, Val: v} }
func rcv(ch int) NOp    { return NOp{Kind: NRecv, Ch: ch} }
func cls(ch int) NOp    { return NOp{Kind: NClose, Ch: ch} }

// Bounded exploration of native channel, select, cancellation, Event, and CapacityLimiter
// scenarios: every interleaving keeps the invariants of nCheck.
func TestNativeModelInvariants(t *testing.T) {
	for name, sc := range map[string]NScenario{
		"buffered":   {Caps: []int{1}, Tasks: [][]NOp{{snd(0, 1), snd(0, 2)}, {rcv(0), rcv(0)}}},
		"rendezvous": {Caps: []int{0}, Tasks: [][]NOp{{snd(0, 1)}, {snd(0, 2)}, {rcv(0), rcv(0)}}},
		"close-race": {Caps: []int{1}, Tasks: [][]NOp{{snd(0, 1), snd(0, 2)}, {cls(0)}, {rcv(0), rcv(0)}}},
		"select-two": {Caps: []int{1, 1}, Tasks: [][]NOp{{snd(0, 1)}, {snd(1, 2)}, {{Kind: NSelect, Arms: []NArm{{Ch: 0}, {Ch: 1}}}, {Kind: NSelect, Arms: []NArm{{Ch: 0}, {Ch: 1}}}}}},
		// The receiver of the arm not chosen ends by the close (ok = false).
		"select-send":   {Caps: []int{0, 0}, Tasks: [][]NOp{{{Kind: NSelect, Arms: []NArm{{Send: true, Ch: 0, Val: 1}, {Send: true, Ch: 1, Val: 2}}}, cls(0), cls(1)}, {rcv(0)}, {rcv(1)}}},
		"cancel-recv":   {Caps: []int{0}, Tasks: [][]NOp{{rcv(0)}, {snd(0, 7)}}, Cancellable: []bool{true, true}},
		"cancel-select": {Caps: []int{0, 1}, Tasks: [][]NOp{{{Kind: NSelect, Arms: []NArm{{Ch: 0}, {Send: true, Ch: 1, Val: 3}}}}, {snd(0, 4)}, {rcv(1)}}, Cancellable: []bool{true, false, true}},
		"event":         {Caps: []int{}, Events: 1, Tasks: [][]NOp{{{Kind: NWait}}, {{Kind: NWait}}, {{Kind: NSet}, {Kind: NSet}}}, Cancellable: []bool{true, false, false}},
		"limiter":       {Caps: []int{}, Limits: []int{1}, Tasks: [][]NOp{{{Kind: NAcquire}, {Kind: NRelease}}, {{Kind: NAcquire}}, {{Kind: NAcquire}, {Kind: NRelease}}}, Cancellable: []bool{false, false, true}},
		"limiter-two":   {Caps: []int{}, Limits: []int{2}, Tasks: [][]NOp{{{Kind: NAcquire}, {Kind: NRelease}}, {{Kind: NAcquire}, {Kind: NRelease}}, {{Kind: NAcquire}, {Kind: NRelease}}}},
	} {
		r := ExploreNative(sc)
		if r.Violation != "" {
			t.Fatalf("%s: %s\n  trace: %s", name, r.Violation, r.Trace)
		}
		if r.Terminal == 0 {
			t.Fatalf("%s: no complete history", name)
		}
		t.Logf("%s: %d states, %d complete, %d blocked, %d outcomes", name, r.States, r.Terminal, r.Blocked, len(r.Outcomes))
	}
}

func outcomes(r *NResult) string {
	var s []string
	for o := range r.Outcomes {
		s = append(s, o)
	}
	return strings.Join(s, " | ")
}

// D33: an operation that can proceed when attempted never observes cancellation; a waiting one
// may unwind (Cancelled) or still proceed if its partner comes first.
func TestNativeCancelOnlyWhileWaiting(t *testing.T) {
	r := ExploreNative(NScenario{Caps: []int{1}, Tasks: [][]NOp{{snd(0, 1), rcv(0)}}, Cancellable: []bool{true}})
	for o := range r.Outcomes {
		if strings.Contains(o, "Cancelled") {
			t.Fatalf("a non-waiting operation observed cancellation: %s", o)
		}
	}
	r = ExploreNative(NScenario{Caps: []int{0}, Tasks: [][]NOp{{rcv(0)}}, Cancellable: []bool{true}})
	want := map[string]bool{"t0=Cancelled c0=[]/[] diags=0": true, "t0=blocked c0=[]/[] diags=0": true}
	for o := range r.Outcomes {
		if !want[o] {
			t.Fatalf("unexpected outcome %s (all: %s)", o, outcomes(r))
		}
	}
}

// D35: a send on a closed channel fails ClosedChannel and publishes nothing.
func TestNativeSendOnClosed(t *testing.T) {
	r := ExploreNative(NScenario{Caps: []int{1}, Tasks: [][]NOp{{cls(0), snd(0, 9)}, {rcv(0)}}})
	for o := range r.Outcomes {
		if !strings.Contains(o, "t0=ClosedChannel") || strings.Contains(o, "[9]") {
			t.Fatalf("outcome %s", o)
		}
	}
}

// D34: with two ready arms both choices occur; default is taken only when no arm is ready.
func TestNativeSelectChoice(t *testing.T) {
	r := ExploreNative(NScenario{Caps: []int{1, 1}, Tasks: [][]NOp{{snd(0, 1), snd(1, 2), {Kind: NSelect, Arms: []NArm{{Ch: 0}, {Ch: 1}}}}}})
	if len(r.Outcomes) != 2 {
		t.Fatalf("want both arms chosen across histories, got %s", outcomes(r))
	}
	r = ExploreNative(NScenario{Caps: []int{1}, Tasks: [][]NOp{{{Kind: NSelect, Arms: []NArm{{Ch: 0}}, Default: true}}}})
	if outs := outcomes(r); outs != "t0=done c0=[]/[] diags=0" {
		t.Fatalf("default: %s", outs)
	}
	// A rendezvous select takes exactly one arm even when both partners are present.
	r = ExploreNative(NScenario{Caps: []int{0, 0}, Tasks: [][]NOp{{{Kind: NSelect, Arms: []NArm{{Send: true, Ch: 0, Val: 1}, {Send: true, Ch: 1, Val: 2}}}}, {rcv(0)}, {rcv(1)}}})
	for o := range r.Outcomes {
		if strings.Contains(o, "c0=[1]") && strings.Contains(o, "c1=[2]") {
			t.Fatalf("both arms took effect: %s", o)
		}
	}
}

// D39: a token left unreleased at task end is released with a diagnostic; the limiter never
// exceeds its size and no token leaks.
func TestNativeLimiterAbandon(t *testing.T) {
	r := ExploreNative(NScenario{Caps: []int{}, Limits: []int{1}, Tasks: [][]NOp{{{Kind: NAcquire}}, {{Kind: NAcquire}, {Kind: NRelease}}}})
	if r.Violation != "" || r.Blocked != 0 {
		t.Fatalf("violation %q, blocked %d", r.Violation, r.Blocked)
	}
	for o := range r.Outcomes {
		if !strings.HasSuffix(o, "diags=1") {
			t.Fatalf("abandoned token not diagnosed: %s", o)
		}
	}
}

// The checker detects seeded protocol bugs.
func TestNativeCheckerDetects(t *testing.T) {
	sc := NScenario{Caps: []int{1}, Limits: []int{1}, Events: 1, Tasks: [][]NOp{{}}}
	s := newNState(sc)
	s.selects[0] = 2
	if v := nCheck(sc, s, false); !strings.Contains(v, "select arms") {
		t.Fatalf("two select arms not detected: %q", v)
	}
	s = newNState(sc)
	s.waitsOK[0] = true
	if v := nCheck(sc, s, false); !strings.Contains(v, "before Set") {
		t.Fatalf("Wait before Set not detected: %q", v)
	}
	s = newNState(sc)
	s.held[0] = 1
	if v := nCheck(sc, s, false); !strings.Contains(v, "held") {
		t.Fatalf("unowned token not detected: %q", v)
	}
	s = newNState(sc)
	s.commits[0] = []int{1}
	if v := nCheck(sc, s, true); !strings.Contains(v, "lost") {
		t.Fatalf("lost value not detected: %q", v)
	}
}
