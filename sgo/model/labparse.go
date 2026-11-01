package model

import "encoding/json"

type labLine struct {
	E       string `json:"e"`
	Ready   []int  `json:"ready"`
	Pick    int    `json:"pick"`
	Task    int    `json:"task"`
	Arms    []int  `json:"arms"`
	Arm     int    `json:"arm"`
	To      int64  `json:"to"`
	Timer   int    `json:"timer"`
	Text    string `json:"text"`
	Outcome string `json:"outcome"`
	Status  int    `json:"status"`
	Kind    string `json:"kind"`
}

func parseLabLine(l string) LEvent {
	var x labLine
	if err := json.Unmarshal([]byte(l), &x); err != nil {
		return LEvent{Kind: "invalid", Text: l}
	}
	e := LEvent{Kind: x.E, Ready: x.Ready, Pick: x.Pick, Task: x.Task, Arms: x.Arms, Arm: x.Arm,
		To: x.To, Timer: x.Timer, Text: x.Text, Outcome: x.Outcome, Status: x.Status}
	if x.E == "outcome" {
		e.Outcome = x.Kind
	}
	if e.Arms == nil && x.E == "select" {
		e.Arms = []int{}
	}
	return e
}
