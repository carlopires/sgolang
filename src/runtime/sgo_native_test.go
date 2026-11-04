// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Native channels, select, Event, and CapacityLimiter (W07) against the executable model
// (sgo/model/nativemodel.go) and targeted gate tests.

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

// Mirrors of the model's scenario types (field names as in the JSON).
type nArm struct {
	Send bool
	Ch   int
	Val  int
}

type nOp struct {
	Kind    int
	Ch      int
	Val     int
	Arms    []nArm
	Default bool
}

type nScenario struct {
	Caps        []int
	Events      int
	Limits      []int
	Tasks       [][]nOp
	Cancellable []bool
}

type nGolden struct {
	Scenario nScenario `json:"scenario"`
	Outcomes []string  `json:"outcomes"`
	Blocked  bool      `json:"blocked_histories"`
}

const (
	nSend = iota
	nRecv
	nSelect
	nClose
	nSet
	nWait
	nAcquire
	nRelease
)

// normalizeNative sorts the received values of each channel: concurrent receivers record their
// values after the operation, so their recording order is not the linearization order.
func normalizeNative(o string) string {
	f := strings.Fields(o) // cN=[a b]/[c] spans fields
	var out []string
	for i := 0; i < len(f); i++ {
		x := f[i]
		if strings.HasPrefix(x, "c") && strings.Contains(x, "=[") && !strings.Contains(x, "]/") {
			j := i
			for !strings.Contains(f[j], "]/") {
				j++
			}
			x = strings.Join(f[i:j+1], " ")
			i = j
		}
		if k := strings.Index(x, "=["); strings.HasPrefix(x, "c") && k > 0 {
			e := strings.Index(x, "]/")
			vals := strings.Fields(x[k+2 : e])
			slices.Sort(vals)
			x = x[:k+2] + strings.Join(vals, " ") + x[e:]
		}
		out = append(out, x)
	}
	return strings.Join(out, " ")
}

func runNativeScenario(t *testing.T, sc nScenario, rng *rand.Rand) string {
	chs := make([]*runtime.SGoChan, len(sc.Caps))
	for i, c := range sc.Caps {
		chs[i] = runtime.SGoNativeNew(c)
	}
	evs := make([]*runtime.SGoEvent, sc.Events)
	for i := range evs {
		evs[i] = runtime.SGoEventNew()
	}
	lims := make([]*runtime.SGoLimiter, len(sc.Limits))
	for i, n := range sc.Limits {
		lims[i] = runtime.SGoLimiterNew(n)
	}
	var mu sync.Mutex
	received := make([][]int, len(chs))
	outcomes := make([]string, len(sc.Tasks))
	var diags atomic.Int64
	yield := func(k int) {
		for range k {
			runtime.Gosched()
		}
	}
	yields := make([]int, 2*len(sc.Tasks))
	for i := range yields {
		yields[i] = rng.Intn(4)
	}
	recordRecv := func(ch, v int) {
		mu.Lock()
		received[ch] = append(received[ch], v)
		mu.Unlock()
	}
	root(t, func() {
		s := mustOpen(t)
		for i, script := range sc.Tasks {
			cancel := i < len(sc.Cancellable) && sc.Cancellable[i]
			body := func(*runtime.SGoTask) error {
				var toks []*runtime.SGoToken
				defer func() { diags.Add(int64(len(toks))) }() // left to the runtime: TokenAbandoned
				yield(yields[i])
				for _, op := range script {
					switch op.Kind {
					case nSend:
						runtime.SGoNSend(chs[op.Ch], op.Val)
					case nRecv:
						if v, ok := runtime.SGoNRecv(chs[op.Ch]); ok {
							recordRecv(op.Ch, v)
						}
					case nSelect:
						arms := make([]runtime.SGoNArm, len(op.Arms))
						for k, a := range op.Arms {
							arms[k] = runtime.SGoNArm{Send: a.Send, Ch: chs[a.Ch], Val: a.Val}
						}
						k, v, ok := runtime.SGoNSelect(arms, !op.Default)
						if k >= 0 && !op.Arms[k].Send && ok {
							recordRecv(op.Arms[k].Ch, v)
						}
					case nClose:
						runtime.SGoNClose(chs[op.Ch])
					case nSet:
						runtime.SGoEventSet(evs[op.Ch])
					case nWait:
						if err := runtime.SGoEventWait(evs[op.Ch]); err != nil {
							return err
						}
					case nAcquire:
						k, err := runtime.SGoAcquire(lims[op.Ch])
						if err != nil {
							return err
						}
						toks = append(toks, k)
					case nRelease:
						runtime.SGoRelease(toks[0])
						toks = toks[1:]
					}
				}
				return nil
			}
			c := yields[len(sc.Tasks)+i]
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				inner := mustOpen(t)
				mustSpawn(t, inner, body)
				if cancel {
					yield(c)
					if err := runtime.SGoCancel(inner); err != nil {
						t.Error(err)
					}
				}
				r, err := runtime.SGoJoin(inner)
				if err != nil {
					t.Error(err)
				}
				switch {
				case r.Outcome == "Succeeded" && r.Cancelled > 0:
					outcomes[i] = "Cancelled"
				case r.Outcome == "Succeeded":
					outcomes[i] = "done"
				default:
					outcomes[i] = runtime.SGoFailureKind(r.Panic)
				}
				return nil
			})
		}
		mustJoin(t, s)
	})
	var b strings.Builder
	for i, o := range outcomes {
		fmt.Fprintf(&b, "t%d=%s ", i, o)
	}
	for c := range chs {
		q := runtime.SGoChanQueued(chs[c])
		if q == nil {
			q = []int{}
		}
		r := received[c]
		if r == nil {
			r = []int{}
		}
		fmt.Fprintf(&b, "c%d=%v/%v ", c, r, q)
	}
	fmt.Fprintf(&b, "diags=%d", diags.Load())
	return normalizeNative(b.String())
}

// TestSGoNativeOutcomes runs every model scenario many times with random yields and cancellation
// times; every observed outcome must be one the model admits (D30).
func TestSGoNativeOutcomes(t *testing.T) {
	data, err := os.ReadFile("testdata/sgo-native-outcomes.json") // copy of sgo/model/testdata/native-outcomes.json
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]nGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	runs := 300
	if testing.Short() {
		runs = 50
	}
	names := make([]string, 0, len(golden))
	for n := range golden {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, name := range names {
		g := golden[name]
		t.Run(name, func(t *testing.T) {
			if g.Blocked {
				t.Skip("the model admits histories that block forever; checked in the model lane only")
			}
			allowed := map[string]bool{}
			for _, o := range g.Outcomes {
				allowed[normalizeNative(o)] = true
			}
			seen := map[string]int{}
			rng := rand.New(rand.NewSource(int64(len(name))))
			for range runs {
				o := runNativeScenario(t, g.Scenario, rng)
				if !allowed[o] {
					t.Fatalf("outcome %q is not admitted by the model; model outcomes: %v", o, g.Outcomes)
				}
				seen[o]++
			}
			t.Logf("%d runs, %d of %d model outcomes observed", runs, len(seen), len(allowed))
			// The same runs with yields injected at the fault points after each transition
			// (send published, receive done, close before and after), as in W06.
			// The callback runs on many tasks at once: it uses the concurrency-safe package-level
			// generator, not rng.
			runtime.SGoSetChanFault(func(int) {
				for range rand.Intn(3) {
					runtime.Gosched()
				}
			})
			defer runtime.SGoSetChanFault(nil)
			faulted := map[string]int{}
			for range runs / 2 {
				o := runNativeScenario(t, g.Scenario, rand.New(rand.NewSource(rng.Int63())))
				if !allowed[o] {
					t.Fatalf("with fault injection, outcome %q is not admitted by the model; model outcomes: %v", o, g.Outcomes)
				}
				faulted[o]++
			}
			t.Logf("with fault injection: %d runs, %d of %d model outcomes observed", runs/2, len(faulted), len(allowed))
		})
	}
}

// TestSGoNativeSelectStress: many selects over shared channels with several arms ready; every
// value sent is received exactly once, exactly one arm takes effect per select, and no losing arm
// stays registered (N01, N06, H12).
func TestSGoNativeSelectStress(t *testing.T) {
	const producers, per = 4, 300
	root(t, func() {
		a, b := runtime.SGoNativeNew(2), runtime.SGoNativeNew(0)
		var mu sync.Mutex
		got := map[int]int{}
		s := mustOpen(t)
		for p := range producers {
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				for k := range per {
					v := p*per + k + 1
					// Two send arms on the same channel and one on another (duplicate arms).
					arms := []runtime.SGoNArm{{true, a, v}, {true, b, v}, {true, a, v}}
					if i, _, _ := runtime.SGoNSelect(arms, true); i < 0 {
						t.Error("blocking select returned default")
					}
				}
				return nil
			})
		}
		for range 2 {
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				for {
					arms := []runtime.SGoNArm{{false, a, 0}, {false, b, 0}, {false, b, 0}}
					i, v, ok := runtime.SGoNSelect(arms, true)
					if !ok {
						return nil // both closed below once everything is received
					}
					_ = i
					mu.Lock()
					got[v]++
					n := len(got)
					mu.Unlock()
					if n == producers*per {
						runtime.SGoNClose(a)
						runtime.SGoNClose(b)
					}
				}
			})
		}
		if r := mustJoin(t, s); r.Outcome != "Succeeded" {
			t.Fatalf("outcome %+v", r)
		}
		for v := 1; v <= producers*per; v++ {
			if got[v] != 1 {
				t.Fatalf("value %d received %d times", v, got[v])
			}
		}
		if n := runtime.SGoNWaiters(a) + runtime.SGoNWaiters(b); n != 0 {
			t.Fatalf("%d select arms still registered", n)
		}
	})
}

// TestSGoNativeCancelUnwind: a task blocked in a native receive under cancellation unwinds:
// deferred calls run, its own nursery is joined first, the task ends Cancelled with no effect, and
// its waiter is gone (D33, N08). A child failure joined during the unwind becomes primary.
func TestSGoNativeCancelUnwind(t *testing.T) {
	root(t, func() {
		ch := runtime.SGoNativeNew(0)
		var deferred, childJoined atomic.Bool
		outer := mustOpen(t)
		mustSpawn(t, outer, func(*runtime.SGoTask) error {
			defer deferred.Store(true)
			inner := runtime.SGoScopeOf(runtime.SGoNurseryEnter()) // records this frame, as compiled code does
			mustSpawn(t, inner, func(*runtime.SGoTask) error {
				defer childJoined.Store(true)
				runtime.SGoNRecv(ch) // also cancelled: the owner's unwind cancels its nursery
				return nil
			})
			runtime.SGoNRecv(ch)
			t.Error("receive returned under cancellation")
			return nil
		})
		for runtime.SGoNWaiters(ch) < 2 {
			runtime.Gosched()
		}
		if err := runtime.SGoCancel(outer); err != nil {
			t.Fatal(err)
		}
		r := mustJoin(t, outer)
		if r.Outcome != "Succeeded" || r.Cancelled != 1 || !deferred.Load() || !childJoined.Load() {
			t.Fatalf("outcome %+v deferred=%v childJoined=%v", r, deferred.Load(), childJoined.Load())
		}
		if n := runtime.SGoNWaiters(ch); n != 0 {
			t.Fatalf("%d waiters left", n)
		}

		// The child fails; its failure cancels the scope the owner waits in, and is primary.
		s := mustOpen(t)
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			inner := runtime.SGoScopeOf(runtime.SGoNurseryEnter()) // records this frame, as compiled code does
			mustSpawn(t, inner, func(*runtime.SGoTask) error {
				runtime.SGoNSend(ch, 1)
				runtime.SGoNClose(ch)
				runtime.SGoNSend(ch, 2) // ClosedChannel
				return nil
			})
			// Receives 1, then waits on a second channel until the child's failure cancels it.
			runtime.SGoNRecv(ch)
			runtime.SGoNRecv(runtime.SGoNativeNew(0))
			return nil
		})
		r = mustJoin(t, s)
		if r.Outcome != "Failed" || runtime.SGoFailureKind(r.Panic) != "ClosedChannel" {
			t.Fatalf("outcome %+v (%v)", r, r.Panic)
		}
	})
}

// TestSGoNativeEmptySelect: select {} waits until cancellation and then unwinds (D34, N03).
func TestSGoNativeEmptySelect(t *testing.T) {
	root(t, func() {
		s := mustOpen(t)
		var started atomic.Bool
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			started.Store(true)
			runtime.SGoNBlock()
			return nil
		})
		for !started.Load() {
			runtime.Gosched()
		}
		runtime.SGoCancel(s)
		if r := mustJoin(t, s); r.Outcome != "Succeeded" || r.Cancelled != 1 {
			t.Fatalf("outcome %+v", r)
		}
	})
}

// TestSGoNativeGCStack: values survive GC and stack movement while queued or held by a parked
// sender, and receive into a moving stack (N09, H11).
func TestSGoNativeGCStack(t *testing.T) {
	var grow func(n int) int
	grow = func(n int) int {
		var pad [64]int
		pad[n%64] = n
		if n == 0 {
			return pad[0]
		}
		return grow(n-1) + pad[n%64] - n
	}
	root(t, func() {
		ch := runtime.SGoNativeNew(3)
		s := mustOpen(t)
		const n = 2000
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			for v := range n {
				grow(v % 200)
				runtime.SGoNSend(ch, v)
				if v%97 == 0 {
					runtime.GC()
				}
			}
			runtime.SGoNClose(ch)
			return nil
		})
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			for want := 0; ; want++ {
				grow(want % 300)
				v, ok := runtime.SGoNRecv(ch)
				if !ok {
					if want != n {
						t.Errorf("ended after %d", want)
					}
					return nil
				}
				if v != want {
					t.Errorf("got %d want %d", v, want)
				}
			}
		})
		mustJoin(t, s)
	})
}

// TestSGoLimiter: FIFO slots, direct hand-off, misuse violations, and abandoned tokens (D39).
func TestSGoLimiter(t *testing.T) {
	root(t, func() {
		lim := runtime.SGoLimiterNew(2)
		var inUse, peak atomic.Int64
		s := mustOpen(t)
		for range 8 {
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				for range 50 {
					k, err := runtime.SGoAcquire(lim)
					if err != nil {
						return err
					}
					n := inUse.Add(1)
					for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
					}
					runtime.Gosched()
					inUse.Add(-1)
					runtime.SGoRelease(k)
				}
				return nil
			})
		}
		mustJoin(t, s)
		if peak.Load() > 2 || runtime.SGoLimiterUsed(lim) != 0 {
			t.Fatalf("peak %d, used %d", peak.Load(), runtime.SGoLimiterUsed(lim))
		}
		// An abandoned token is released at task end; double and foreign releases are violations.
		var tok *runtime.SGoToken
		s = mustOpen(t)
		mustSpawn(t, s, func(*runtime.SGoTask) error {
			k, _ := runtime.SGoAcquire(lim)
			tok = k
			return nil
		})
		mustJoin(t, s)
		if runtime.SGoLimiterUsed(lim) != 0 {
			t.Fatal("abandoned token not released")
		}
		k, _ := runtime.SGoAcquire(lim)
		runtime.SGoRelease(k)
		for _, bad := range []*runtime.SGoToken{k, tok} {
			func() {
				defer func() {
					if r := recover(); r == nil || !isViolation(r.(error)) {
						t.Errorf("release misuse: %v", r)
					}
				}()
				runtime.SGoRelease(bad)
			}()
		}
	})
}

// TestSGoEvent: Set wakes every waiter; Wait on a set event does not observe cancellation.
func TestSGoEvent(t *testing.T) {
	root(t, func() {
		ev := runtime.SGoEventNew()
		var woke atomic.Int64
		s := mustOpen(t)
		for range 5 {
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				if err := runtime.SGoEventWait(ev); err != nil {
					return err
				}
				woke.Add(1)
				return nil
			})
		}
		runtime.SGoEventSet(ev)
		runtime.SGoEventSet(ev)
		mustJoin(t, s)
		if woke.Load() != 5 || !runtime.SGoEventIsSet(ev) {
			t.Fatalf("woke %d", woke.Load())
		}
		s = mustOpen(t)
		runtime.SGoCancel(s)
		mustSpawn(t, s, func(*runtime.SGoTask) error { return runtime.SGoEventWait(ev) })
		if r := mustJoin(t, s); r.Outcome != "Succeeded" || r.Cancelled != 0 {
			t.Fatalf("set event observed cancellation: %+v", r)
		}
	})
}

// TestSGoTaskResult: every Result call gets its own channel, which delivers the result once
// (edge-triggered); receivers parked before publication on their own channels all get it; a
// channel abandoned by a select that took another arm leaves nothing listed on the task; and a
// channel that delivered stays silent, so a select loop with another arm keeps taking that arm
// (W22 review G2, H1).
func TestSGoTaskResult(t *testing.T) {
	root(t, func() {
		r := runtime.SGoTaskResNew()
		timer := runtime.SGoNativeNew(1)
		// Abandoned channels: a select that takes another arm unlists its result channel.
		for range 1000 {
			runtime.SGoNSend(timer, 1)
			if arm, _, _ := runtime.SGoNSelect([]runtime.SGoNArm{{Ch: runtime.SGoTaskResChan(r)}, {Ch: timer}}, true); arm != 1 {
				t.Fatalf("arm %d before publication", arm)
			}
		}
		if n := runtime.SGoTaskResParked(r); n != 0 {
			t.Fatalf("%d result channels still listed", n)
		}
		var got atomic.Int64
		s := mustOpen(t)
		for range 5 {
			mustSpawn(t, s, func(*runtime.SGoTask) error {
				v, ok := runtime.SGoNRecv(runtime.SGoTaskResChan(r))
				if ok && v == 7 {
					got.Add(1)
				}
				return nil
			})
		}
		for runtime.SGoTaskResParked(r) < 5 {
			runtime.Gosched()
		}
		runtime.SGoTaskResSet(r, 7)
		runtime.SGoTaskResSet(r, 8) // a second publication has no effect
		mustJoin(t, s)
		if got.Load() != 5 {
			t.Fatalf("%d of 5 parked receivers got the result", got.Load())
		}
		// A kept channel delivers once; a loop with a timer arm then takes the timer arm.
		res := runtime.SGoTaskResChan(r)
		seen := 0
		for range 5 {
			runtime.SGoNSend(timer, 1)
			arm, v, _ := runtime.SGoNSelect([]runtime.SGoNArm{{Ch: res}, {Ch: timer}}, true)
			if arm == 0 {
				if v != 7 {
					t.Fatalf("result %d", v)
				}
				seen++
				runtime.SGoNRecv(timer) // the timer value sent for this iteration
			}
		}
		if seen != 1 {
			t.Fatalf("a kept result channel delivered %d times", seen)
		}
		if v, ok := runtime.SGoNRecv(runtime.SGoTaskResChan(r)); !ok || v != 7 {
			t.Fatalf("a new channel after publication: %d %v", v, ok)
		}
		if runtime.SGoTaskResParked(r) != 0 {
			t.Fatal("result channels listed after publication")
		}
	})
}

// TestSGoTaskResultOnce: two publications racing on one record (two Replies to one request) have
// exactly one winner, decided under the record's lock (D174 review K1); run it under -race.
func TestSGoTaskResultOnce(t *testing.T) {
	for range 200 {
		r := runtime.SGoTaskResNew()
		var wins atomic.Int64
		var wg sync.WaitGroup
		for v := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if runtime.SGoTaskResSet(r, v) {
					wins.Add(1)
				}
			}()
		}
		wg.Wait()
		if wins.Load() != 1 {
			t.Fatalf("%d publications won", wins.Load())
		}
	}
}
