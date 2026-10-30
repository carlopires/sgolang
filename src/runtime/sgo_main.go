// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// S-Go program entry and exit (SGO-PROC-001, SGO-PROC-002) and the diagnostic record (SGO-ERR-002).
//
// The compiler links the user's func main(root *sgo.Context) error as main·sgo and synthesizes
// main.main as sgoExitMain(main·sgo(sgoEnterMain())) (cmd/compile/internal/noder/sgo_main.go).
// A failure that unwinds out of main·sgo reaches gopanic's end after the frame walk has completed
// every open nursery (SGO-DEF-005); sgoUnwoundOut then writes the record and exits, before the Go
// panic printer runs.

package runtime

import "unsafe"

// sgoProgram is set when an S-Go program's root task starts.
var sgoProgram bool

// sgoContext mirrors sgo.Context (struct{ t unsafe.Pointer }); the root context is made here.
type sgoContext struct{ t unsafe.Pointer }

// sgoEnterMain starts the root task (creation ID 0) on the main goroutine, turns on go-statement
// enforcement (sgo_service.go), and returns the root *sgo.Context.
func sgoEnterMain() unsafe.Pointer {
	sgoProgram = true
	sgoEnforce.Store(true)
	t := sgoEnterRoot()
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
	sgoWriteRecord("MainError", err.Error(), 0)
	exit(1)
}

// sgoUnwoundOut is called by gopanic when a failure has unwound every frame of the root task of an
// S-Go program. It writes the diagnostic record and exits: status 1 for ChildError, 4 for a runtime
// defect, else 2. Children never get here: sgoTaskMain recovers their failures.
func sgoUnwoundOut(p *_panic) {
	t := getg().sgo
	if !sgoProgram || p.goexit || t == nil || t.id != 0 {
		return
	}
	kind, msg := sgoFailureOf(p.arg)
	sgoWriteRecord(kind, msg, 0)
	switch kind {
	case "ChildError":
		exit(1)
	case "RuntimeDefect":
		exit(4)
	}
	exit(2)
}

// sgoFailureOf classifies an unwinding payload into a failure kind of SGO-DEF-003.
func sgoFailureOf(v any) (kind, msg string) {
	switch e := v.(type) {
	case string:
		return "Panic", e // panic(msg) (SGO-DEF-002)
	case *sgoFailure:
		return e.kind, e.msg
	case *sgoViolation:
		return "SpawnContractViolation", e.msg
	case Error:
		switch e.Error() {
		case "runtime error: integer divide by zero":
			return "DivideByZero", "integer divide by zero"
		case "runtime error: invalid memory address or nil pointer dereference":
			return "NilError", "method call on a nil error" // SGO-TYPE-006
		}
		return "RuntimeDefect", e.Error()
	}
	return "RuntimeDefect", "unexpected failure value"
}

// sgoDefect ends an S-Go program with a RuntimeDefect record and status 4 (SGO-ERR-003).
func sgoDefect(msg string) {
	sgoWriteRecord("RuntimeDefect", msg, 0)
	exit(4)
}

// sgoWriteRecord writes the single-line diagnostic record of SGO-ERR-002 to stderr. Source positions
// and the chain are W03 diagnostics work; until then position is null and the chain is empty.
func sgoWriteRecord(kind, msg string, task uint64) {
	b := make([]byte, 0, 96+len(kind)+len(msg))
	b = append(b, `{"sgo_diagnostic":1,"kind":`...)
	b = sgoAppendJSON(b, kind)
	b = append(b, `,"message":`...)
	b = sgoAppendJSON(b, msg)
	b = append(b, `,"position":null,"task_id":`...)
	b = sgoAppendUint(b, task)
	b = append(b, `,"scope_id":null,"chain":[]}`+"\n"...)
	for done := 0; done < len(b); {
		n := write(2, unsafe.Pointer(&b[done]), int32(len(b)-done))
		if n == -_EINTR {
			continue
		}
		if n <= 0 {
			return // SGO-ERR-002: the exit status is still produced
		}
		done += int(n)
	}
}

// sgoAppendJSON appends s as a JSON string: valid UTF-8, with ", \, and control characters escaped
// and invalid bytes replaced by U+FFFD.
func sgoAppendJSON(b []byte, s string) []byte {
	const hex = "0123456789abcdef"
	b = append(b, '"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < 0x80 {
			switch {
			case c == '"' || c == '\\':
				b = append(b, '\\', c)
			case c < 0x20:
				b = append(b, '\\', 'u', '0', '0', hex[c>>4], hex[c&15])
			default:
				b = append(b, c)
			}
			i++
			continue
		}
		r, next := decoderune(s, uint(i))
		if r == runeError && next == uint(i)+1 {
			b = append(b, "\ufffd"...)
		} else {
			b = append(b, s[i:next]...)
		}
		i = int(next)
	}
	return append(b, '"')
}

func sgoAppendUint(b []byte, v uint64) []byte {
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
	return append(b, buf[i:]...)
}
