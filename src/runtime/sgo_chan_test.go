// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"
)

// Sender scripts, as in sgo/model/chanmodel.go.
const (
	sendCommit = iota
	sendAbort
	sendCancelRace
	sendTaskEnd
	sendAliasDouble
)

type chanScenario struct {
	Cap       int
	Senders   []int
	Receivers int
	Close     bool
}

type chanGolden struct {
	Scenario chanScenario `json:"scenario"`
	Outcomes []string     `json:"outcomes"`
}

// normalizeOutcome sorts the received values of an outcome string: concurrent receivers in the
// test record their values after returning, so receive order across receivers is not observable
// here (FIFO is checked by TestSGoChanFIFO).
func normalizeOutcome(o string) string {
	i := strings.Index(o, "received=[")
	j := strings.Index(o, "] queued=")
	if i < 0 || j < 0 {
		return o
	}
	vals := strings.Fields(o[i+len("received=[") : j])
	slices.Sort(vals)
	return o[:i] + "received=[" + strings.Join(vals, " ") + o[j:]
}

// runChanScenario runs sc once with random yields and returns its outcome in the model's format.
func runChanScenario(t *testing.T, sc chanScenario, rng *rand.Rand) string {
	ch := runtime.SGoChanNew(int64(sc.Cap))
	n := len(sc.Senders)
	states := make([]string, n)
	permits := make([]*runtime.SGoPermit, n) // read after the nursery closes
	var rdone atomic.Int64                   // receivers that have returned
	var mu sync.Mutex
	var received []int
	var finished atomic.Int64 // senders and the close actor that have finished
	yields := make([]int, n+sc.Receivers+2)
	for i := range yields {
		yields[i] = rng.Intn(4)
	}
	yield := func(k int) {
		for range k {
			runtime.Gosched()
		}
	}
	var diags atomic.Int64
	root(t, func() {
		s := mustOpen(t)
		for i, script := range sc.Senders {
			v, k := i+1, yields[i]
			send := func(*runtime.SGoTask) error {
				yield(k)
				p, err := runtime.SGoReserve(ch)
				switch {
				case runtime.SGoIsCancelled(err):
					states[i] = "cancelled"
					return err
				case err != nil:
					states[i] = "refused-closed"
					return nil
				}
				permits[i] = p
				yield(k)
				switch script {
				case sendAbort:
					runtime.SGoAbort(p)
				case sendTaskEnd:
					diags.Add(1)
					return nil // the runtime aborts the permit at task end
				case sendCancelRace:
					if err := runtime.SGoCheckpoint(); err != nil {
						runtime.SGoAbort(p)
						return err
					}
					runtime.SGoCommit(p, v)
				case sendAliasDouble:
					runtime.SGoCommit(p, v)
					// The second alias's commit is a contract violation with no second effect.
					func() {
						defer func() {
							if r := recover(); r != nil && isViolation(r.(error)) {
								diags.Add(1)
							}
						}()
						runtime.SGoCommit(p, v)
					}()
				default:
					runtime.SGoCommit(p, v)
				}
				return nil
			}
			if script != sendCancelRace {
				mustSpawn(t, s, func(tk *runtime.SGoTask) error { defer finished.Add(1); return send(tk) })
				continue
			}
			// The cancel-race sender runs in its own nursery, whose owner cancels it at a random time.
			c := yields[i] + rng.Intn(3)
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				defer finished.Add(1)
				inner := mustOpen(t)
				mustSpawn(t, inner, send)
				yield(c)
				if err := runtime.SGoCancel(inner); err != nil {
					t.Error(err)
				}
				mustJoin(t, inner)
				return nil
			})
		}
		for j := range sc.Receivers {
			k := yields[n+j]
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				defer rdone.Add(1)
				yield(k)
				v, ok, err := runtime.SGoReceive(ch)
				if ok {
					mu.Lock()
					received = append(received, v)
					mu.Unlock()
				}
				return err
			})
		}
		actors := int64(n)
		if sc.Close {
			actors++
			k := yields[n+sc.Receivers]
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				defer finished.Add(1)
				yield(k)
				runtime.SGoClose(ch)
				return nil
			})
		}
		// Once every sender and the close actor have finished, receivers still waiting are blocked
		// histories (a value that will never come); cancel the nursery to end them.
		waitFor(t, "senders", func() bool { return finished.Load() == actors })
		waitFor(t, "receivers", func() bool {
			_, _, recvs := runtime.SGoChanState(ch)
			return rdone.Load()+int64(recvs) == int64(sc.Receivers)
		})
		if err := runtime.SGoCancel(s); err != nil {
			t.Error(err)
		}
		mustJoin(t, s)
	})
	if reserved, senders, recvs := runtime.SGoChanState(ch); reserved != 0 || senders != 0 || recvs != 0 {
		t.Fatalf("%+v: channel not quiescent: reserved=%d senders=%d receivers=%d", sc, reserved, senders, recvs)
	}
	for i, p := range permits {
		if p != nil {
			states[i] = runtime.SGoPermitState(p)
		}
	}
	want := int64(0)
	for i, script := range sc.Senders {
		switch {
		case script == sendTaskEnd && states[i] == "aborted":
			want++
		case script == sendAliasDouble && (states[i] == "committed" || states[i] == "failed-closed"):
			want++
		}
	}
	if d := diags.Load(); d != want {
		t.Fatalf("%+v: %d diagnostics, want %d (states %v)", sc, d, want, states)
	}
	slices.Sort(received)
	return fmt.Sprintf("permits=%v received=%v queued=%v", states, received, runtime.SGoChanQueued(ch))
}

// TestSGoChanModel checks the runtime channel against the model's outcome sets (D30): every
// outcome of every run must be an outcome the model admits for the same scenario.
func TestSGoChanModel(t *testing.T) {
	runs := 300
	if testing.Short() {
		runs = 50
	}
	checkChanModel(t, runs)
}

// TestSGoChanModelFaults reruns the reference scenarios with a yield injected, at random, before
// and after every terminal transition (W06 fault injection); every outcome must stay admitted.
func TestSGoChanModelFaults(t *testing.T) {
	var n atomic.Uint64
	hits := make([]atomic.Int64, runtime.SGoChanFaultPoints)
	runtime.SGoSetChanFault(func(point int) {
		hits[point].Add(1)
		if x := n.Add(1) * 0x9e3779b97f4a7c15; x>>62 != 0 {
			runtime.Gosched()
		}
	})
	defer runtime.SGoSetChanFault(nil)
	checkChanModel(t, 150)
	for i := range hits {
		if hits[i].Load() == 0 {
			t.Errorf("fault point %d never reached", i)
		}
	}
}

func checkChanModel(t *testing.T, runs int) {
	// A copy of sgo/model/testdata/chan-outcomes.json; the model's tests keep the two identical.
	data, err := os.ReadFile("testdata/sgo-chan-outcomes.json")
	if err != nil {
		t.Fatalf("model outcomes: %v", err)
	}
	var golden map[string]chanGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(golden))
	for name := range golden {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		g := golden[name]
		t.Run(name, func(t *testing.T) {
			allowed := map[string]bool{}
			for _, o := range g.Outcomes {
				allowed[normalizeOutcome(o)] = true
			}
			seen := map[string]int{}
			rng := rand.New(rand.NewSource(int64(len(name))))
			for range runs {
				o := runChanScenario(t, g.Scenario, rng)
				if !allowed[o] {
					t.Fatalf("outcome %q is not admitted by the model; model outcomes: %v", o, g.Outcomes)
				}
				seen[o]++
			}
			t.Logf("%d runs, %d of %d model outcomes observed", runs, len(seen), len(allowed))
		})
	}
}

// TestSGoChanFIFO: receives see values in commit order, and capacity bounds reservations (H01).
func TestSGoChanFIFO(t *testing.T) {
	root(t, func() {
		ch := runtime.SGoChanNew(3)
		s := mustOpen(t)
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			for v := 1; v <= 100; v++ {
				p, err := runtime.SGoReserve(ch)
				if err != nil {
					return err
				}
				if r, _, _ := runtime.SGoChanState(ch); r > 3 {
					t.Errorf("reserved %d > capacity", r)
				}
				if err := runtime.SGoCommit(p, v); err != nil {
					return err
				}
			}
			runtime.SGoClose(ch)
			return nil
		})
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			for want := 1; ; want++ {
				v, ok, err := runtime.SGoReceive(ch)
				if err != nil || !ok {
					if want != 101 {
						t.Errorf("channel ended after %d values (err %v)", want-1, err)
					}
					return err
				}
				if v != want {
					t.Errorf("received %d, want %d", v, want)
				}
			}
		})
		if r := mustJoin(t, s); r.Outcome != "Succeeded" {
			t.Fatalf("outcome %+v", r)
		}
	})
}

// TestSGoChanTaskAffinity: a permit used by another task is a violation with no effect (H05);
// unresolved permits are aborted at owner-body exit before the join (H10).
func TestSGoChanTaskAffinity(t *testing.T) {
	root(t, func() {
		ch := runtime.SGoChanNew(1)
		p, err := runtime.SGoReserve(ch)
		if err != nil {
			t.Fatal(err)
		}
		s := mustOpen(t)
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			defer func() {
				r := recover()
				if r == nil || !isViolation(r.(error)) {
					t.Errorf("commit by another task: recover = %v", r)
				}
			}()
			runtime.SGoCommit(p, 7)
			return nil
		})
		mustJoin(t, s)
		if st := runtime.SGoPermitState(p); st != "reserved" {
			t.Fatalf("foreign commit changed the permit: %s", st)
		}
		runtime.SGoAbort(p)

		// A permit reserved inside a nursery body is aborted when the body exits, before the join.
		s = mustOpen(t)
		q, err := runtime.SGoReserve(ch)
		if err != nil {
			t.Fatal(err)
		}
		before := runtime.SGoAbandoned()
		mustJoin(t, s)
		if st := runtime.SGoPermitState(q); st != "aborted" || runtime.SGoAbandoned() != before+1 {
			t.Fatalf("body-exit resolution: permit %s, abandoned %d -> %d", st, before, runtime.SGoAbandoned())
		}
		if r, _, _ := runtime.SGoChanState(ch); r != 0 {
			t.Fatalf("reservation leaked: %d", r)
		}
	})
}

// TestSGoChanPanicWithPermit: a task that panics holding a permit has the reservation released
// before its outcome is published; no capacity leaks (H09).
func TestSGoChanPanicWithPermit(t *testing.T) {
	root(t, func() {
		ch := runtime.SGoChanNew(1)
		s := mustOpen(t)
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			if _, err := runtime.SGoReserve(ch); err != nil {
				return err
			}
			panic("boom")
		})
		if r := mustJoin(t, s); r.Outcome != "Failed" {
			t.Fatalf("outcome %+v", r)
		}
		if reserved, _, _ := runtime.SGoChanState(ch); reserved != 0 {
			t.Fatalf("reservation leaked after the task failed: %d", reserved)
		}
		p, err := runtime.SGoReserve(ch) // the capacity is available again
		if err != nil {
			t.Fatal(err)
		}
		runtime.SGoAbort(p)
	})
}

// TestSGoChanValueLifetime: a queued value stays live under GC until it is received, and the
// channel drops its reference once the value is consumed (H11).
func TestSGoChanValueLifetime(t *testing.T) {
	type payload struct {
		id  int
		buf [256]byte
	}
	var freed atomic.Int64
	root(t, func() {
		ch := runtime.SGoChanNew(4)
		// A competing reservation is aborted before any value is sent: it holds no value and its
		// capacity is restored.
		if p, err := runtime.SGoReserve(ch); err != nil {
			t.Fatal(err)
		} else {
			runtime.SGoAbort(p)
		}
		for id := range 4 {
			p, err := runtime.SGoReserve(ch)
			if err != nil {
				t.Fatal(err)
			}
			v := &payload{id: id}
			runtime.SetFinalizer(v, func(*payload) { freed.Add(1) })
			if err := runtime.SGoCommitPtr(p, unsafe.Pointer(v)); err != nil {
				t.Fatal(err)
			}
		}
		if reserved, _, _ := runtime.SGoChanState(ch); reserved != 0 || len(runtime.SGoChanQueued(ch)) != 4 {
			t.Fatalf("reserved=%d queued=%v", reserved, runtime.SGoChanQueued(ch))
		}
		for range 5 {
			runtime.GC()
		}
		if n := freed.Load(); n != 0 {
			t.Fatalf("%d queued values were collected before delivery", n)
		}
		for id := range 4 {
			v, ok, err := runtime.SGoReceivePtr(ch)
			if !ok || err != nil || (*payload)(v).id != id {
				t.Fatalf("receive %d: ok=%v err=%v", id, ok, err)
			}
		}
	})
	waitFor(t, "consumed values to be collected", func() bool {
		runtime.GC()
		return freed.Load() == 4
	})
}

// TestSGoChanConservation: unique payload IDs across concurrent send, receive, close, abort, and
// cancellation; each successful commit is received or still queued exactly once, and no-effect
// outcomes never appear in the receive history (H12).
func TestSGoChanConservation(t *testing.T) {
	for round := range 20 {
		for _, capacity := range []int64{0, 1, 3} {
			conservationRound(t, capacity, int64(round))
		}
	}
}

func conservationRound(t *testing.T, capacity, seed int64) {
	const senders, perSender, receivers = 6, 20, 3
	rng := rand.New(rand.NewSource(seed*31 + capacity))
	closeAfter := rng.Intn(senders * perSender)
	var (
		mu        sync.Mutex
		committed = map[int]bool{}
		noEffect  = map[int]bool{}
		received  = map[int]int{}
		attempts  atomic.Int64
		done      atomic.Int64 // senders that have finished
	)
	ch := runtime.SGoChanNew(capacity)
	root(t, func() {
		s := mustOpen(t)
		for i := range senders {
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				defer done.Add(1)
				for k := range perSender {
					id := i*perSender + k
					if attempts.Add(1) == int64(closeAfter) {
						runtime.SGoClose(ch)
					}
					p, err := runtime.SGoReserve(ch)
					if err != nil {
						return nil // closed or cancelled: no permit
					}
					if id%7 == 3 {
						runtime.SGoAbort(p)
						mu.Lock()
						noEffect[id] = true
						mu.Unlock()
						continue
					}
					err = runtime.SGoCommit(p, id)
					mu.Lock()
					if err == nil {
						committed[id] = true
					} else {
						noEffect[id] = true
					}
					mu.Unlock()
				}
				return nil
			})
		}
		for range receivers {
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				for {
					v, ok, err := runtime.SGoReceive(ch)
					if !ok {
						return err
					}
					mu.Lock()
					received[v]++
					mu.Unlock()
				}
			})
		}
		// Close once the senders finish (no effect if already closed) to end the receivers.
		waitFor(t, "senders", func() bool { return done.Load() == senders })
		runtime.SGoClose(ch)
		mustJoin(t, s)
	})
	queued := runtime.SGoChanQueued(ch)
	for _, v := range queued {
		received[v]++
	}
	for id, n := range received {
		if n != 1 || !committed[id] {
			t.Fatalf("cap %d: payload %d received %d times (committed %v, no-effect %v)", capacity, id, n, committed[id], noEffect[id])
		}
	}
	for id := range committed {
		if received[id] != 1 {
			t.Fatalf("cap %d: committed payload %d was lost", capacity, id)
		}
	}
	if reserved, s, r := runtime.SGoChanState(ch); reserved != 0 || s != 0 || r != 0 {
		t.Fatalf("cap %d: not quiescent: reserved=%d senders=%d receivers=%d", capacity, reserved, s, r)
	}
}

// TestSGoChanRingStable: a buffered channel queues values in the ring allocated when it was made;
// 10 000 commit/receive pairs keep FIFO order, never replace the ring, and leave only queued values
// in it (W12 R9: values were once appended to a slice under c.lock, which reallocated).
func TestSGoChanRingStable(t *testing.T) {
	root(t, func() {
		ch := runtime.SGoChanNew(4)
		addr, n, _ := runtime.SGoChanRing(ch)
		if n != 4 || addr == 0 {
			t.Fatalf("ring of a cap-4 channel: len %d", n)
		}
		send := func(v int) {
			p, err := runtime.SGoReserve(ch)
			if err != nil {
				t.Fatal(err)
			}
			if err := runtime.SGoCommit(p, v); err != nil {
				t.Fatal(err)
			}
		}
		next, want := 0, 0
		for ; next < 3; next++ {
			send(next)
		}
		for i := 0; i < 10000; i++ {
			send(next)
			next++
			v, ok, err := runtime.SGoReceive(ch)
			if err != nil || !ok || v != want {
				t.Fatalf("receive %d: %d %v %v, want %d", i, v, ok, err, want)
			}
			want++
		}
		if a, m, live := runtime.SGoChanRing(ch); a != addr || m != 4 || live != 3 {
			t.Errorf("ring after 10000 pairs: addr changed %v, len %d, live slots %d (want 3)", a != addr, m, live)
		}
		if r, _, _ := runtime.SGoChanRing(runtime.SGoChanNew(0)); r != 0 {
			t.Errorf("a rendezvous channel has a ring")
		}
	})
}
