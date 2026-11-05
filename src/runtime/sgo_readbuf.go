// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Whole-message reads (D164, D173): the read turn and the read buffer of a connection or file.
// Every read takes the handle's read turn for its duration, so one read at a time runs on a
// handle and the buffered bytes are never reordered between readers. ReadLine and ReadAll read
// past what they return; the rest stays in h.rbuf and later reads return it first. A ReadLine or
// ReadAll that fails (cancelled, too long, an I/O error) consumes nothing: what it read stays
// buffered.

package runtime

// sgoReadLineHook, set only by the runtime's tests, runs between two reads of one ReadLine, with
// the read turn held: the window a Close must not enter (W22 review H2).
var sgoReadLineHook func(h *sgoHandle)

// sgoRawRead is one bounded read of the handle's stream, below the buffer.
func sgoRawRead(h *sgoHandle, max int) (string, *sgoIOErr) {
	switch h.kind {
	case sgoHConn:
		return sgoConnRead(h, max)
	case sgoHStdin:
		return sgoStdinRead(h, max)
	}
	return sgoFileRead(h, max)
}

// sgoReadTurn waits for h's read turn; a task that finds it taken sleeps briefly, as a
// cancellation point, and tries again. The handle is checked again once the turn is held, so a
// read that waited for the turn sees a Close made meanwhile (HandleClosed) before it touches the
// buffer (W22 review H2); Close refuses with ResourceBusy while the turn is held.
func sgoReadTurn(h *sgoHandle, op string) *sgoIOErr {
	if e := sgoHandleBegin(h, op); e != nil {
		return e
	}
	sgoHandleEnd(h)
	t := sgoCurrent()
	for !h.rturn.CompareAndSwap(0, 1) {
		if sgoSleepFor(t, 1) != nil {
			return &sgoIOErr{"Cancelled", 0}
		}
	}
	if e := sgoHandleBegin(h, op); e != nil {
		h.rturn.Store(0)
		return e
	}
	sgoHandleEnd(h)
	return nil
}

// sgoBufRead is Read: buffered bytes first, else one read of the stream.
func sgoBufRead(h *sgoHandle, max int, op string) (string, *sgoIOErr) {
	if e := sgoReadTurn(h, op); e != nil {
		return "", e
	}
	defer h.rturn.Store(0)
	if h.rbuf != "" && max > 0 {
		n := min(max, len(h.rbuf))
		s := h.rbuf[:n]
		h.rbuf = h.rbuf[n:]
		return s, nil
	}
	return sgoRawRead(h, max)
}

// sgoReadLine is ReadLine: the next line without "\n" and one "\r" before it. The line with its
// terminator is at most max bytes, else TooLong; a last line without "\n" is returned at end of
// stream, and EOF comes only when nothing is left.
func sgoReadLine(h *sgoHandle, max int, op string) (string, *sgoIOErr) {
	if e := sgoReadTurn(h, op); e != nil {
		return "", e
	}
	defer h.rturn.Store(0)
	buf := h.rbuf
	for {
		for i := 0; i < len(buf) && i < max; i++ {
			if buf[i] == '\n' {
				h.rbuf = buf[i+1:]
				line := buf[:i]
				if len(line) > 0 && line[len(line)-1] == '\r' {
					line = line[:len(line)-1]
				}
				return line, nil
			}
		}
		if len(buf) > max || (len(buf) == max && max <= 0) {
			h.rbuf = buf
			return "", &sgoIOErr{"TooLong", 0}
		}
		if len(buf) == max {
			// max bytes and no "\n": the last line if the stream ends here, else too long (one
			// more byte decides; W22 review H3).
			s, e := sgoRawRead(h, 1)
			switch {
			case e != nil && e.kind == "EOF":
				h.rbuf = ""
				return buf, nil
			case e != nil:
				h.rbuf = buf
				return "", &sgoIOErr{e.kind, 0}
			}
			h.rbuf = buf + s
			return "", &sgoIOErr{"TooLong", 0}
		}
		s, e := sgoRawRead(h, max-len(buf))
		if e != nil {
			if e.kind == "EOF" && buf != "" {
				h.rbuf = ""
				return buf, nil
			}
			h.rbuf = buf
			return "", &sgoIOErr{e.kind, 0}
		}
		buf += s
		if sgoReadLineHook != nil {
			sgoReadLineHook(h)
		}
	}
}

// sgoReadAll is ReadAll: the rest of the stream, at most max bytes, else TooLong.
func sgoReadAll(h *sgoHandle, max int, op string) (string, *sgoIOErr) {
	if e := sgoReadTurn(h, op); e != nil {
		return "", e
	}
	defer h.rturn.Store(0)
	buf := h.rbuf
	for len(buf) <= max {
		s, e := sgoRawRead(h, max+1-len(buf))
		if e != nil {
			if e.kind == "EOF" {
				h.rbuf = ""
				return buf, nil
			}
			h.rbuf = buf
			return "", &sgoIOErr{e.kind, 0}
		}
		buf += s
	}
	h.rbuf = buf
	return "", &sgoIOErr{"TooLong", 0}
}
