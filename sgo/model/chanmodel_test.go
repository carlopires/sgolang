package model

import "testing"

// Bounded exploration of SGO-PERMIT-002 scenarios (H01-H08, H12): every interleaving keeps
// capacity, FIFO, single-terminal-state, close-versus-commit, and conservation invariants.
func TestChanModelInvariants(t *testing.T) {
	for _, sc := range []ChanScenario{
		{Cap: 1, Senders: []int{SendCommit, SendCommit}, Receivers: 2},              // H01, H12
		{Cap: 2, Senders: []int{SendCommit, SendAbort, SendCommit}, Receivers: 2},   // H01, H03
		{Cap: 1, Senders: []int{SendCommit, SendCommit}, Receivers: 1, Close: true}, // H08
		{Cap: 0, Senders: []int{SendCommit, SendCommit}, Receivers: 2},              // H02
		{Cap: 0, Senders: []int{SendCommit, SendAbort}, Receivers: 1, Close: true},  // H02, H03, H08
		{Cap: 3, Senders: []int{SendCommit, SendCommit, SendCommit}, Receivers: 3, Close: true},
	} {
		r := ExploreChan(sc)
		if r.Violation != "" {
			t.Fatalf("%+v: %s\n  trace: %s", sc, r.Violation, r.Trace)
		}
		if r.Terminal == 0 {
			t.Fatalf("%+v: no complete history", sc)
		}
		t.Logf("%+v: %d states, %d complete histories, %d blocked, %d distinct outcomes", sc, r.States, r.Terminal, r.Deadlocked, len(r.Outcomes))
	}
}

// Close wins before publication: the permit fails with no effect and the value is not received;
// publication wins before close: the value stays receivable (plan §7.3, H08).
func TestChanModelCloseVersusCommit(t *testing.T) {
	r := ExploreChan(ChanScenario{Cap: 1, Senders: []int{SendCommit}, Receivers: 1, Close: true})
	want := map[string]bool{
		"permits=[committed] received=[1] queued=[]":     true, // commit before close, then received
		"permits=[failed-closed] received=[] queued=[]":  true, // close before commit: no effect
		"permits=[refused-closed] received=[] queued=[]": true, // close before reserve: no permit
	}
	for o := range want {
		if !r.Outcomes[o] {
			t.Errorf("missing outcome %s (got %v)", o, r.Outcomes)
		}
	}
	for o := range r.Outcomes {
		if !want[o] {
			t.Errorf("unexpected outcome %s", o)
		}
	}
}

// Cancellation, task-end resolution, and aliases (H04, H06, H07, H09, H10) keep every invariant,
// alone and mixed with close, abort, and rendezvous.
func TestChanModelObligations(t *testing.T) {
	for _, sc := range []ChanScenario{
		{Cap: 1, Senders: []int{SendCancelRace, SendCommit}, Receivers: 2},
		{Cap: 0, Senders: []int{SendCancelRace, SendCommit}, Receivers: 2, Close: true},
		{Cap: 1, Senders: []int{SendTaskEnd, SendCommit}, Receivers: 1},
		{Cap: 0, Senders: []int{SendTaskEnd, SendCommit}, Receivers: 1},
		{Cap: 2, Senders: []int{SendAliasDouble, SendCommit}, Receivers: 2, Close: true},
		{Cap: 1, Senders: []int{SendCancelRace, SendTaskEnd, SendAliasDouble}, Receivers: 2, Close: true},
	} {
		r := ExploreChan(sc)
		if r.Violation != "" {
			t.Fatalf("%+v: %s\n  trace: %s", sc, r.Violation, r.Trace)
		}
		if r.Terminal == 0 {
			t.Fatalf("%+v: no complete history", sc)
		}
		t.Logf("%+v: %d states, %d complete histories, %d blocked, %d outcomes", sc, r.States, r.Terminal, r.Deadlocked, len(r.Outcomes))
	}
}

// Cancellation racing commit (plan §7.3): before reserve no permit, after reserve the reservation
// is aborted, after commit the value is published and received.
func TestChanModelCancelVersusCommit(t *testing.T) {
	r := ExploreChan(ChanScenario{Cap: 1, Senders: []int{SendCancelRace}, Receivers: 1})
	want := map[string]bool{
		"permits=[committed] received=[1] queued=[]": true,
		"permits=[aborted] received=[] queued=[]":    true,
		"permits=[cancelled] received=[] queued=[]":  true,
	}
	for o := range r.Outcomes {
		if !want[o] {
			t.Errorf("unexpected outcome %s", o)
		}
	}
	if r.Violation != "" || len(r.Outcomes) == 0 {
		t.Fatalf("violation %q, outcomes %v", r.Violation, r.Outcomes)
	}
	t.Logf("outcomes: %v (blocked histories, a receiver waiting for a value that was never sent: %d)", r.Outcomes, r.Deadlocked)
}

// The checker catches a seeded protocol bug: a commit that publishes after close.
func TestChanModelDetectsLostValue(t *testing.T) {
	s := &chanState{permit: []int{PermitCommitted}, spc: []int{2}, commits: []int{1}}
	if v := chanCheck(ChanScenario{Cap: 1, Senders: []int{SendCommit}}, s, true); v != "a committed value was lost" {
		t.Fatalf("checker missed a lost value: %q", v)
	}
	s = &chanState{permit: []int{PermitReserved}, spc: []int{1}, reserved: 2}
	if v := chanCheck(ChanScenario{Cap: 1, Senders: []int{SendCommit}}, s, false); v != "capacity exceeded" {
		t.Fatalf("checker missed a capacity violation: %q", v)
	}
}
