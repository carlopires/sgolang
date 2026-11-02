// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Lab fake of the network adapters (W09 step 4, D58): a loopback network inside the run. A Dial
// to a granted endpoint connects to the run's open listener on that endpoint (else
// ConnectionRefused); there is no host network. Each direction of a connection is an S-Go
// channel of string chunks and a listener's backlog is a channel of connections, so waiting,
// cancellation, choice points, and deadlock detection are those of channels. Closing one end
// closes both directions: the peer reads EOF after the data already sent, and its writes fail
// with ConnectionReset.

package runtime

import "unsafe"

const (
	sgoFakeChunks  = 16 // capacity of a direction, in chunks
	sgoFakeBacklog = 16 // pending connections of a listener
)

type sgoFakeConn struct {
	in, out *sgoChan // connections: received and sent chunks (*string)
	pend    string   // the rest of a chunk longer than a Read's max
	backlog *sgoChan // listeners: pending server ends (*sgoFakeConn)
	addr    string
	unix    bool
}

// sgoFakeListeners are the open fake listeners of the run (serialized by the baton).
var sgoFakeListeners []*sgoFakeConn

func sgoFakeListen(g *sgoNetGrant) (*sgoHandle, *sgoIOErr) {
	sgoLabPoint()
	for _, l := range sgoFakeListeners {
		if l.addr == g.addr {
			return nil, &sgoIOErr{kind: "AddressInUse"}
		}
	}
	l := &sgoFakeConn{backlog: sgoChanNew(sgoFakeBacklog), addr: g.addr, unix: g.unix}
	sgoFakeListeners = append(sgoFakeListeners, l)
	h := sgoHandleNew(sgoHListener, -1, g.addr)
	h.conn = l
	return h, nil
}

func sgoFakeDial(g *sgoNetGrant) (*sgoHandle, *sgoIOErr) {
	sgoLabPoint()
	t := sgoCurrent()
	if sgoTaskCancellation(t) != nil {
		return nil, &sgoIOErr{kind: "Cancelled"}
	}
	var l *sgoFakeConn
	for _, x := range sgoFakeListeners {
		if x.addr == g.addr {
			l = x
		}
	}
	if l == nil {
		return nil, &sgoIOErr{kind: "ConnectionRefused"}
	}
	a, b := sgoChanNew(sgoFakeChunks), sgoChanNew(sgoFakeChunks)
	client := &sgoFakeConn{in: a, out: b, addr: g.addr, unix: g.unix}
	server := &sgoFakeConn{in: b, out: a, addr: g.addr, unix: g.unix}
	p, err := sgoChanReserve(l.backlog, t) // a full backlog waits, as a real connect does
	if err != nil {
		if err == sgoErrReserveClosed {
			return nil, &sgoIOErr{kind: "ConnectionRefused"}
		}
		return nil, &sgoIOErr{kind: "Cancelled"}
	}
	if sgoChanCommit(p, unsafe.Pointer(server)) != nil {
		return nil, &sgoIOErr{kind: "ConnectionRefused"}
	}
	h := sgoHandleNew(sgoHConn, -1, g.addr)
	h.conn = client
	return h, nil
}

func sgoFakeAccept(h *sgoHandle) (*sgoHandle, *sgoIOErr) {
	sgoLabPoint()
	v, ok, err := sgoChanReceive(h.conn.backlog, sgoCurrent())
	switch {
	case err != nil:
		return nil, &sgoIOErr{kind: "Cancelled"}
	case !ok:
		return nil, &sgoIOErr{kind: "HandleClosed"}
	}
	c := sgoHandleNew(sgoHConn, -1, h.name)
	c.conn = (*sgoFakeConn)(v)
	return c, nil
}

func sgoFakeConnRead(h *sgoHandle, max int) (string, *sgoIOErr) {
	sgoLabPoint()
	c := h.conn
	if c.pend == "" {
		v, ok, err := sgoChanReceive(c.in, sgoCurrent())
		switch {
		case err != nil:
			return "", &sgoIOErr{kind: "Cancelled"}
		case !ok:
			return "", &sgoIOErr{kind: "EOF"}
		}
		c.pend = *(*string)(v)
	}
	n := min(max, len(c.pend))
	s := c.pend[:n]
	c.pend = c.pend[n:]
	return s, nil
}

func sgoFakeConnWrite(h *sgoHandle, s string) (int, *sgoIOErr) {
	c := h.conn
	t := sgoCurrent()
	done := 0
	for done < len(s) {
		sgoLabPoint()
		p, err := sgoChanReserve(c.out, t)
		if err != nil {
			if err == sgoErrReserveClosed {
				return done, &sgoIOErr{"ConnectionReset", done}
			}
			return done, &sgoIOErr{"Cancelled", done}
		}
		chunk := new(string)
		*chunk = s[done : done+min(len(s)-done, sgoIOChunk)]
		if sgoChanCommit(p, unsafe.Pointer(chunk)) != nil {
			return done, &sgoIOErr{"ConnectionReset", done}
		}
		done += len(*chunk)
	}
	return done, nil
}

// sgoFakeConnClose closes a fake connection (both directions) or listener (its pending
// connections are closed, so their dialers read EOF).
func sgoFakeConnClose(h *sgoHandle) {
	c := h.conn
	h.conn = nil
	if c.backlog == nil {
		sgoChanClose(c.in)
		sgoChanClose(c.out)
		return
	}
	for i, l := range sgoFakeListeners {
		if l == c {
			sgoFakeListeners = append(sgoFakeListeners[:i:i], sgoFakeListeners[i+1:]...)
			break
		}
	}
	sgoChanClose(c.backlog)
	// The pending connections are taken directly: closing must not depend on the closer's
	// cancellation state, and a closed channel accepts no new ones.
	// They are popped one at a time, so nothing is collected under the lock (W12 R9).
	for {
		lock(&c.backlog.lock)
		if c.backlog.count == 0 {
			sgoChanCheck(c.backlog)
			unlock(&c.backlog.lock)
			return
		}
		s := (*sgoFakeConn)(c.backlog.qpop())
		sgoChanCheck(c.backlog)
		unlock(&c.backlog.lock)
		sgoChanClose(s.in)
		sgoChanClose(s.out)
	}
}
