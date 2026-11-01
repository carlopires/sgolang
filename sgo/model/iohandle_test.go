// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model

import (
	"slices"
	"testing"
)

// HandleScenarios are the I03, I04, I05 scenarios of the handle contract (W09, D54–D56), with
// their outcome sets over every interleaving and every short-transfer choice.
var HandleScenarios = map[string]HScenario{
	// I03: a read returns what is available (short), never more than asked.
	"partial-read": {Tasks: [][]HOp{{{HOpen, 0, 0}, {HRead, 0, 5}}, {{HDeliver, 0, 3}}}, Peer: true, Cap: 4},
	// I03: cancellation after partial progress reports the bytes written.
	"cancel-after-partial-write": {Tasks: [][]HOp{{{HOpen, 0, 0}, {HWrite, 0, 5}}, {{HCancel, 0, 0}}}, Cap: 2},
	// I04: the owner cannot close a handle lent to a live nursery; cancelling one borrower ends
	// only its own operation.
	"shared-handle-cancel": {Tasks: [][]HOp{{{HOpen, 0, 0}, {HLend, 0, 0}, {HCancel, 0, 1}, {HClose, 0, 0}, {HUnlend, 0, 0}, {HClose, 0, 0}},
		{{HRead, 0, 4}}, {{HRead, 0, 4}}, {{HDeliver, 0, 1}, {HDeliver, 0, 1}}}, Peer: true, Cap: 1},
	// I05: an operation on a closed handle has no effect, and data sent to the old generation
	// is never read through the reopened one.
	"close-reuse": {Tasks: [][]HOp{{{HOpen, 0, 0}, {HWrite, 0, 2}, {HClose, 0, 0}, {HWrite, 0, 1}, {HOpen, 0, 0}, {HRead, 0, 1}},
		{{HDeliver, 0, 1}}}, Peer: true, Cap: 2},
	// D54: only the owner closes.
	"borrower-close": {Tasks: [][]HOp{{{HOpen, 0, 0}, {HLend, 0, 0}, {HUnlend, 0, 0}, {HClose, 0, 0}}, {{HClose, 0, 0}}}, Cap: 1},
}

var handleWant = map[string][]string{
	"partial-read":               {"open,read 1", "open,read 3"},
	"cancel-after-partial-write": {"open,err Cancelled 2 | cancel"},
	// A borrower that reads both bytes leaves the other waiting, so the nursery (and the final
	// close) never completes in that interleaving.
	"shared-handle-cancel": {
		"open,lent,cancel,err HandleLent,stuck | read 2 | ,stuck",
		"open,lent,cancel,err HandleLent,unlent,closed | err Cancelled 0 | read 1",
		"open,lent,cancel,err HandleLent,unlent,closed | err Cancelled 0 | read 2",
		"open,lent,cancel,err HandleLent,unlent,closed | read 1 | read 1",
	},
	"close-reuse":    {"open,wrote 2,closed,err HandleClosed 0,open,read 1", "open,wrote 2,closed,err HandleClosed 0,open,stuck"},
	"borrower-close": {"open,lent,unlent,closed | err NotOwner"},
}

func TestHandleScenarios(t *testing.T) {
	for name, sc := range HandleScenarios {
		got, err := ExploreHandles(sc)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !slices.Equal(got, handleWant[name]) {
			t.Errorf("%s: outcomes %q, want %q", name, got, handleWant[name])
		}
	}
}

// A close of a lent handle breaks the contract: the invariant check fires.
func TestHandleCloseWhileLentDetected(t *testing.T) {
	defer func() { hBugCloseWhileLent = false }()
	hBugCloseWhileLent = true
	if _, err := ExploreHandles(HandleScenarios["shared-handle-cancel"]); err == nil {
		t.Fatal("closing a lent handle was not detected")
	}
}
