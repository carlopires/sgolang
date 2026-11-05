// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Standard input (D178). fd 0 is usually a terminal or a pipe shared with the parent shell, so
// the runtime never sets O_NONBLOCK on it (that would change the shell's open file description).
// A pump goroutine (runtime service stdin-pump) makes one blocking read of fd 0 when a reader
// finds nothing buffered; the reader waits for it by sleeping briefly as a cancellation point, so
// a read of stdin ends on cancellation like any handle wait. The pump reads one chunk per demand:
// the process consumes from fd 0 only what its readers ask for, at most one chunk ahead. The
// handle's read turn and read buffer (SGO-FILE-004) then give Read, ReadLine and ReadAll their
// usual meaning. In the lab, stdin is the run's recorded text (the manifest's stdin lines).

package runtime

import "unsafe"

var sgoStdin struct {
	lock    mutex
	h       *sgoHandle
	data    string    // bytes the pump read that no reader has taken
	end     *sgoIOErr // EOF or IOError, once the pump has met it
	pumping bool      // a pump read is in progress
}

// sgoLabStdin is the lab run's standard input, from the manifest (sgo_lab_boot.go).
var sgoLabStdin string

// sgo_stdin returns the stdin handle, made on first use. It belongs to the root task's body,
// inside which every task runs, so any task given the *sgo.Input may read it; it is never
// released or closed. The caller is the root task (package sgo checks it).
//
//go:linkname sgo_stdin sgo.runtime_stdin
func sgo_stdin() unsafe.Pointer {
	// Allocate before taking the lock (no allocation under a runtime lock); a racing first use
	// keeps the handle published first.
	fresh := &sgoHandle{kind: sgoHStdin, fd: 0, body: sgoCurrent(), name: "stdin"}
	lock(&sgoStdin.lock)
	if sgoStdin.h == nil {
		sgoStdin.h = fresh
	}
	h := sgoStdin.h
	unlock(&sgoStdin.lock)
	return unsafe.Pointer(h)
}

// sgoStdinRead is one bounded read of standard input, below the read buffer.
func sgoStdinRead(h *sgoHandle, max int) (string, *sgoIOErr) {
	if e := sgoHandleBegin(h, "Stdin.Read"); e != nil {
		return "", e
	}
	defer sgoHandleEnd(h)
	if max <= 0 {
		return "", nil
	}
	if sgoLab != nil {
		sgoLabPoint()
		if sgoLabStdin == "" {
			return "", &sgoIOErr{"EOF", 0}
		}
		n := min(max, len(sgoLabStdin), sgoIOChunk)
		s := sgoLabStdin[:n]
		sgoLabStdin = sgoLabStdin[n:]
		return s, nil
	}
	t := sgoCurrent()
	wait := int64(1)
	for {
		lock(&sgoStdin.lock)
		if len(sgoStdin.data) > 0 {
			n := min(max, len(sgoStdin.data))
			s := sgoStdin.data[:n]
			sgoStdin.data = sgoStdin.data[n:]
			unlock(&sgoStdin.lock)
			return s, nil
		}
		if e := sgoStdin.end; e != nil {
			unlock(&sgoStdin.lock)
			return "", e
		}
		start := !sgoStdin.pumping
		sgoStdin.pumping = true
		unlock(&sgoStdin.lock)
		if start {
			go sgoStdinPump()
		}
		// Wait for the pump as a cancellation point, backing off to 16 ms while input is idle.
		if sgoSleepFor(t, wait) != nil {
			return "", &sgoIOErr{"Cancelled", 0}
		}
		if wait < 16 {
			wait *= 2
		}
	}
}

// sgoStdinPump makes one blocking read of fd 0 and publishes the bytes, end of stream, or error.
func sgoStdinPump() {
	buf := make([]byte, 64<<10)
	for {
		n := sgoSysRead(0, unsafe.Pointer(&buf[0]), int32(len(buf)))
		if n == -sgoEINTR {
			continue
		}
		if n == -sgoEAGAIN { // a parent left fd 0 non-blocking: poll it
			usleep(5000)
			continue
		}
		// Build the result before taking the lock (no allocation under a runtime lock).
		var end *sgoIOErr
		chunk := ""
		switch {
		case n < 0:
			end = &sgoIOErr{"IOError", 0}
		case n == 0:
			end = &sgoIOErr{"EOF", 0}
		default:
			chunk = string(buf[:n])
		}
		lock(&sgoStdin.lock)
		// The pump starts only when data is empty and delivers one chunk, so data is empty here.
		if end != nil {
			sgoStdin.end = end
		} else {
			sgoStdin.data = chunk
		}
		sgoStdin.pumping = false
		unlock(&sgoStdin.lock)
		return
	}
}

// sgo_stdinIsTerminal reports whether fd 0 is a terminal; never in the lab.
//
//go:linkname sgo_stdinIsTerminal sgo.runtime_stdinIsTerminal
func sgo_stdinIsTerminal() bool { return sgo_isTerminal(0) }

// sgo_readSecret is ReadLine with the terminal's echo off for its duration when fd 0 is a
// terminal; the terminal's settings are restored on every return, cancellation included. In the
// lab, and when stdin is not a terminal, it is ReadLine.
//
//go:linkname sgo_readSecret sgo.runtime_readSecret
func sgo_readSecret(h unsafe.Pointer, max int) (string, error) {
	hh := (*sgoHandle)(h)
	op := "Stdin.ReadSecret"
	if sgoLab != nil || !sgo_isTerminal(0) {
		s, e := sgoReadLine(hh, max, op)
		return s, sgoIOResult(e, op)
	}
	var saved, quiet [sgoTermiosSize]byte
	if sgoIoctl(0, sgoTIOCGETA, unsafe.Pointer(&saved[0])) != 0 {
		s, e := sgoReadLine(hh, max, op)
		return s, sgoIOResult(e, op)
	}
	quiet = saved
	sgoTermiosClearEcho(&quiet)
	sgoIoctl(0, sgoTIOCSETA, unsafe.Pointer(&quiet[0]))
	s, e := sgoReadLine(hh, max, op)
	sgoIoctl(0, sgoTIOCSETA, unsafe.Pointer(&saved[0]))
	// The user's newline was not echoed; end the prompt's line as the terminal would have.
	sgo_write(2, "\n")
	return s, sgoIOResult(e, op)
}
