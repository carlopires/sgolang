// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgo_test

import (
	"errors"
	"sgo"
	"strings"
	"testing"
)

// The runtime-provided linknames resolve and behave as specified outside any S-Go task.
func TestAPIOutsideTask(t *testing.T) {
	e := sgo.NewError("boom")
	if e == nil || e.Error() != "boom" || sgo.IsCancelled(e) {
		t.Fatalf("NewError/IsCancelled: %v", e)
	}
	var w sgo.Writer
	if err := w.WriteString(""); err != nil {
		t.Fatalf("empty write: %v", err)
	}
	// A context that does not belong to the calling task is a SpawnContractViolation.
	func() {
		defer func() {
			err, _ := recover().(error)
			if err == nil || !strings.HasPrefix(err.Error(), "SpawnContractViolation") {
				t.Errorf("foreign context: recovered %v", err)
			}
		}()
		var c sgo.Context
		_ = c.Checkpoint()
	}()
	// Spawning with a nursery handle that has no scope is refused before any child exists.
	func() {
		defer func() {
			err, _ := recover().(error)
			if err == nil || !strings.Contains(err.Error(), "nil nursery handle") {
				t.Errorf("nil scope: recovered %v", err)
			}
		}()
		var n sgo.Nursery
		n.Spawn(func(*sgo.Context) error { return errors.New("must not run") })
	}()
}
