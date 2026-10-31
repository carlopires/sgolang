// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// S-Go program entry and exit (SGO-PROC-001, SGO-PROC-002) and the diagnostic record (SGO-ERR-002).
//
// The compiler links the user's func main(root *sgo.Context) error as main·sgo and synthesizes
// main.main as sgoExitMain(main·sgo(sgoEnterMain())) (cmd/compile/internal/noder/sgo_main.go).
// A failure that unwinds out of main·sgo reaches gopanic's end after the frame walk has completed
// every open nursery (SGO-DEF-005); sgoUnwoundOut then writes the record and exits, before the Go
// panic printer runs. Runtime throws in an S-Go program end with status 4 (sgoThrowExit).

package runtime

import "unsafe"

// sgoProgram is set when an S-Go program's root task starts.
var sgoProgram bool

// sgoContext mirrors sgo.Context (struct{ t unsafe.Pointer }); the root context is made here.
type sgoContext struct{ t unsafe.Pointer }

// sgoFailureInfo is one entry of a diagnostic record: the primary failure or a chain entry. An
// empty pos is written as null.
type sgoFailureInfo struct{ kind, msg, pos string }

// sgoEnterMain starts the root task (creation ID 0) on the main goroutine, turns on go-statement
// enforcement (sgo_service.go), and returns the root *sgo.Context.
func sgoEnterMain() unsafe.Pointer {
	sgoProgram = true
	sgoDrainInit()  // drain budget recorded in the binary (SGO-DRAIN-002)
	sgoGrantsInit() // root grants recorded in the binary (SGO-AUTH-002)
	sgoEnforce.Store(true)
	t := sgoEnterRoot()
	t.ceiling = sgoRootCeiling
	return unsafe.Pointer(&sgoContext{t: unsafe.Pointer(t)})
}

// sgoExitMain handles main's result: nil returns to runtime.main, which exits with status 0 and
// writes nothing to stderr; an error ends the process with MainError and status 1.
func sgoExitMain(v any) {
	t := getg().sgo
	if t == nil || t.id != 0 || len(t.nest) != 0 {
		sgoDefect("main returned with an open nursery or outside the root task") // OWN-3
	}
	if v == nil {
		return
	}
	err, ok := v.(error)
	if !ok {
		sgoDefect("main returned a non-error value")
	}
	sgoWriteRecord(sgoFailureInfo{"MainError", err.Error(), ""}, nil, sgoOrigin{})
	exit(1)
}

// sgoUnwound returns the primary failure of the unwinding that p is part of and the later failures
// in occurrence order (SGO-DEF-004): a failure raised by a deferred call while the task is already
// unwinding does not replace the one in progress. Go keeps the earlier panics on p.link.
func sgoUnwound(p *_panic) (primary any, later []any) {
	for q := p; q != nil; q = q.link {
		if q.goexit {
			break
		}
		later = append(later, q.arg)
	}
	n := len(later)
	primary = later[n-1]
	later = later[:n-1]
	for i, j := 0, len(later)-1; i < j; i, j = i+1, j-1 {
		later[i], later[j] = later[j], later[i]
	}
	return primary, later
}

// sgoUnwoundOut is called by gopanic when a failure has unwound every frame of the root task of an
// S-Go program. It writes the diagnostic record and exits: status 1 for ChildError, 4 for a runtime
// defect, else 2. Children never get here: sgoTaskMain recovers their failures.
func sgoUnwoundOut(p *_panic) {
	t := getg().sgo
	if !sgoProgram || p.goexit || t == nil || t.id != 0 {
		return
	}
	primary, _ := sgoUnwound(p)
	f := sgoFailureOf(primary)
	sgoWriteRecord(f, sgoChain(t, p), sgoPrimaryOrigin(p))
	switch f.kind {
	case "ChildError":
		exit(1)
	case "RuntimeDefect":
		exit(4)
	}
	exit(2)
}

// sgoFailureOf classifies an unwinding payload into a failure kind of SGO-DEF-003.
func sgoFailureOf(v any) sgoFailureInfo {
	switch e := v.(type) {
	case string:
		return sgoFailureInfo{"Panic", e, ""} // panic(msg) (SGO-DEF-002)
	case *sgoFailure:
		return sgoFailureInfo{e.kind, e.msg, e.pos}
	case *sgoViolation:
		return sgoFailureInfo{"SpawnContractViolation", e.msg, ""}
	case Error:
		switch e.Error() {
		case "runtime error: integer divide by zero":
			return sgoFailureInfo{"DivideByZero", "integer divide by zero", ""}
		case "runtime error: invalid memory address or nil pointer dereference":
			return sgoFailureInfo{"NilError", "method call on a nil error", ""} // SGO-TYPE-006
		}
		return sgoFailureInfo{"RuntimeDefect", e.Error(), ""}
	}
	return sgoFailureInfo{"RuntimeDefect", "unexpected failure value", ""}
}

// sgoDefect ends an S-Go program with a RuntimeDefect record and status 4 (SGO-ERR-003).
// The record names the task that hit the defect.
func sgoDefect(msg string) {
	var o sgoOrigin
	if t := getg().sgo; t != nil {
		o = sgoOriginOf(t)
	}
	sgoWriteRecord(sgoFailureInfo{"RuntimeDefect", msg, ""}, nil, o)
	exit(4)
}

// sgoThrowExit ends an S-Go program on a runtime throw (SGO-ERR-003): exhausted resources are
// ResourceExhausted, every other throw is a violated invariant, RuntimeDefect. Status 4; deferred
// calls do not run. It runs on the system stack and does not allocate.
func sgoThrowExit(s string) {
	kind := "RuntimeDefect"
	switch s {
	case "stack overflow", "out of memory":
		kind = "ResourceExhausted"
	}
	var o sgoOrigin
	if gp := getg().m.curg; gp != nil && gp.sgo != nil {
		o.task = gp.sgo.id
		if gp.sgo.scope != nil {
			o.scope = gp.sgo.scope.id
		}
	}
	sgoWriteRecord(sgoFailureInfo{kind, s, ""}, nil, o)
	exit(4)
}

// sgoWriteRecord writes the single-line diagnostic record of SGO-ERR-002 to stderr. It does not
// allocate, so it can run on the system stack. Positions come from the compiler (checked
// arithmetic, panic); failures raised inside the runtime or package sgo have none yet (null).
//
// task_id and scope_id name where the primary failure was raised: the task, and the scope that
// task was spawned into (null for the root task) (sgo_record.go).
func sgoWriteRecord(f sgoFailureInfo, chain []sgoFailureInfo, o sgoOrigin) {
	var w sgoRecordWriter
	w.str(`{"sgo_diagnostic":1,"kind":`)
	w.json(f.kind)
	w.str(`,"message":`)
	w.json(f.msg)
	w.str(`,"position":`)
	w.pos(f.pos)
	w.str(`,"task_id":`)
	w.uint(o.task)
	w.str(`,"scope_id":`)
	if o.scope == 0 {
		w.str("null")
	} else {
		w.uint(o.scope)
	}
	w.str(`,"chain":[`)
	for i, c := range chain {
		if i > 0 {
			w.byte(',')
		}
		w.str(`{"kind":`)
		w.json(c.kind)
		w.str(`,"message":`)
		w.json(c.msg)
		w.str(`,"position":`)
		w.pos(c.pos)
		w.byte('}')
	}
	w.str("]}\n")
	w.flush()
}

// sgoRecordWriter buffers record bytes in a fixed array and writes them to fd 2.
type sgoRecordWriter struct {
	buf [256]byte
	n   int
}

func (w *sgoRecordWriter) flush() {
	for done := 0; done < w.n; {
		n := write(2, unsafe.Pointer(&w.buf[done]), int32(w.n-done))
		if n == -_EINTR {
			continue
		}
		if n <= 0 {
			break // SGO-ERR-002: the exit status is still produced
		}
		done += int(n)
	}
	w.n = 0
}

func (w *sgoRecordWriter) byte(c byte) {
	if w.n == len(w.buf) {
		w.flush()
	}
	w.buf[w.n] = c
	w.n++
}

func (w *sgoRecordWriter) str(s string) {
	for i := 0; i < len(s); i++ {
		w.byte(s[i])
	}
}

// json writes s as a JSON string: valid UTF-8, with ", \, and control characters escaped and
// invalid bytes replaced by U+FFFD.
func (w *sgoRecordWriter) json(s string) {
	const hex = "0123456789abcdef"
	w.byte('"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < 0x80 {
			switch {
			case c == '"' || c == '\\':
				w.byte('\\')
				w.byte(c)
			case c < 0x20:
				w.str(`\u00`)
				w.byte(hex[c>>4])
				w.byte(hex[c&15])
			default:
				w.byte(c)
			}
			i++
			continue
		}
		r, next := decoderune(s, uint(i))
		if r == runeError && next == uint(i)+1 {
			w.str("\ufffd")
		} else {
			for k := uint(i); k < next; k++ {
				w.byte(s[k])
			}
		}
		i = int(next)
	}
	w.byte('"')
}

// pos writes a position as a JSON string, or null if it is unknown.
func (w *sgoRecordWriter) pos(p string) {
	if p == "" {
		w.str("null")
		return
	}
	w.json(p)
}

func (w *sgoRecordWriter) uint(v uint64) {
	var buf [20]byte
	i := len(buf)
	for {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
		if v == 0 {
			break
		}
	}
	for ; i < len(buf); i++ {
		w.byte(buf[i])
	}
}
