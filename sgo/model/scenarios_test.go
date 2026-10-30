package model

import (
	"strings"
	"testing"
)

// Program-building helpers.
func N(name string, body ...Instr) Instr  { return Instr{Op: OpNursery, Name: name, Body: body} }
func Sp(name string, child ...Instr) Instr { return Instr{Op: OpSpawn, Name: name, Child: child} }
func C(name string) Instr                 { return Instr{Op: OpCancel, Name: name} }
func CP() Instr                           { return Instr{Op: OpCheckpoint} }
func CPc() Instr                          { return Instr{Op: OpCheckpoint, Continue: true} }
func SL() Instr                           { return Instr{Op: OpSleep} }
func SLc() Instr                          { return Instr{Op: OpSleep, Continue: true} }
func RN() Instr                           { return Instr{Op: OpReturnNil} }
func RE(msg string) Instr                 { return Instr{Op: OpReturnErr, Msg: msg} }
func P(msg string) Instr                  { return Instr{Op: OpPanic, Msg: msg} }
func Spin() Instr                         { return Instr{Op: OpSpin} }
func M(msg string) Instr                  { return Instr{Op: OpMark, Msg: msg} }

type scenario struct {
	name     string
	clauses  string // clauses the scenario checks
	prog     Program
	cfg      Config
	expect   []string             // exact set of terminal outcomes over all interleavings
	property func(*State) string // checked on every terminal state; "" means ok
}

func scope0CauseIsChild(s *State) string {
	if len(s.Scopes) == 0 || !strings.HasPrefix(s.Scopes[0].Cause, "child ") {
		return "S0 was not cancelled by its failing child (SGO-SPAWN-004)"
	}
	return ""
}

var scenarios = []scenario{
	{name: "normal-join", clauses: "SGO-NUR-003 OWN-3 OWN-4",
		prog:   Program{N("n", Sp("n", M("a")), Sp("n", M("b"))), RN()},
		expect: []string{"exit=0"}},
	{name: "descendant-during-join", clauses: "SGO-SPAWN-002 SGO-NUR-003",
		prog:   Program{N("n", Sp("n", Sp("n", M("grandchild")))), RN()},
		expect: []string{"exit=0"}},
	{name: "child-error-exit-1", clauses: "SGO-NUR-004 SGO-PROC-002",
		prog:   Program{N("n", Sp("n", RE("boom"))), RN()},
		expect: []string{"exit=1 ChildError: boom"}},
	{name: "main-error-exit-1", clauses: "SGO-PROC-002",
		prog:   Program{RE("bad")},
		expect: []string{"exit=1 MainError: bad"}},
	{name: "child-panic-cancels-sibling", clauses: "SGO-SPAWN-004 SGO-DEF-005",
		prog:     Program{N("n", Sp("n", P("p1")), Sp("n", SL(), RN())), RN()},
		expect:   []string{"exit=2 Panic: p1"},
		property: scope0CauseIsChild},
	{name: "two-failures-lowest-id", clauses: "SGO-NUR-004",
		prog:   Program{N("n", Sp("n", P("first")), Sp("n", P("second"))), RN()},
		expect: []string{"exit=2 Panic: first"}},
	{name: "owner-failure-primary", clauses: "SGO-NUR-004 rule 1, SGO-DEF-005",
		prog:   Program{N("n", Sp("n", P("child")), P("owner")), RN()},
		expect: []string{"exit=2 Panic: owner"}},
	{name: "cancel-propagates", clauses: "SGO-CANCEL-001 SGO-WAIT-001",
		prog:   Program{N("n", Sp("n", SL(), RN()), C("n")), RN()},
		expect: []string{"exit=0"}},
	{name: "cancel-consumed-at-boundary", clauses: "SGO-CANCEL-002 SGO-CANCEL-004",
		prog:   Program{N("n", Sp("n", SL(), RN()), C("n"), CPc()), CP(), RN()},
		expect: []string{"exit=0"}},
	{name: "inner-cancel-not-outer", clauses: "SGO-NUR-005 SGO-CANCEL-004",
		prog:   Program{N("a", Sp("a", N("b", Sp("b", SL(), RN()), C("b")), CP(), RN())), RN()},
		expect: []string{"exit=0"}},
	{name: "outer-cancel-reaches-inner", clauses: "SGO-CANCEL-001 propagation",
		prog:   Program{N("a", Sp("a", N("b", Sp("b", SL(), RN())), RN()), C("a")), RN()},
		expect: []string{"exit=0"}},
	{name: "spawn-into-outer-from-inner", clauses: "SGO-SPAWN-002 (c)",
		prog:   Program{N("a", N("b", Sp("a", M("x")))), RN()},
		expect: []string{"exit=2 SpawnContractViolation: T0 spawn into S0 (OPEN)"}},
	{name: "spawn-when-cancelling", clauses: "SGO-SPAWN-002 R5",
		prog:   Program{N("n", C("n"), Sp("n", P("never"))), RN()},
		expect: []string{"exit=0"}},
	{name: "return-crossing-nursery", clauses: "SGO-CTL-004",
		prog:   Program{N("n", Sp("n", SL(), RN()), RE("early")), RN()},
		expect: []string{"exit=1 MainError: early"}},
	{name: "noncooperative-budget", clauses: "SGO-DRAIN-001 SGO-DRAIN-002",
		prog:   Program{N("n", Sp("n", Spin()), C("n")), RN()},
		cfg:    Config{DrainBudget: true},
		expect: []string{"exit=3 DrainBudgetExceeded: S0"}},
	{name: "noncooperative-no-budget", clauses: "SGO-DRAIN-001 CAN-5",
		prog:   Program{N("n", Sp("n", Spin()), C("n")), RN()},
		expect: []string{"stuck"}},
	{name: "sleep-cancel-race", clauses: "SGO-WAIT-001 CAN-2",
		prog:   Program{N("n", Sp("n", SLc(), M("after-sleep"), RN()), C("n")), RN()},
		expect: []string{"exit=0"}},
	{name: "owner-woken-by-child-failure", clauses: "SGO-CANCEL-004 SGO-NUR-004 rule 3",
		prog:     Program{N("n", Sp("n", RE("x")), SL(), RN()), RN()},
		expect:   []string{"exit=1 ChildError: x"},
		property: scope0CauseIsChild},
}

func TestScenarios(t *testing.T) {
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			r := Explore(sc.prog, sc.cfg, 200000)
			if r.Violation != "" {
				t.Fatalf("[%s] invariant violation: %s", sc.clauses, r.Violation)
			}
			got := r.OutcomeSet()
			if strings.Join(got, "|") != strings.Join(sc.expect, "|") {
				for _, o := range got {
					t.Logf("outcome %q example trace:\n%s", o, join(r.Examples[o]))
				}
				t.Fatalf("[%s] outcomes %q, want %q", sc.clauses, got, sc.expect)
			}
			if sc.property != nil {
				for _, s := range r.Terminals {
					if msg := sc.property(s); msg != "" {
						t.Fatalf("[%s] property failed: %s\ntrace:\n%s", sc.clauses, msg, join(s.Trace))
					}
				}
			}
			t.Logf("%d states, %d terminal states, outcomes %q", r.States, len(r.Terminals), got)
		})
	}
}
