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
	// A copy of sgo/model/testdata/chan-outcomes.json; the model's tests keep the two identical.
	data, err := os.ReadFile("testdata/sgo-chan-outcomes.json")
	if err != nil {
		t.Fatalf("model outcomes: %v", err)
	}
	var golden map[string]chanGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	runs := 300
	if testing.Short() {
		runs = 50
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
