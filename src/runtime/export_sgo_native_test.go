// Copyright 2026 The S-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test hooks for native channels, select, Event, and CapacityLimiter (W07).

package runtime

import "unsafe"

var sgoChanIntType = func() *chantype {
	var i any = (chan int)(nil)
	return (*chantype)(unsafe.Pointer(efaceOf(&i)._type))
}()

func SGoNativeNew(capacity int) *SGoChan { return sgomakechan(sgoChanIntType, capacity) }
func SGoNSend(c *SGoChan, v int)         { sgochansend1(c, unsafe.Pointer(&v)) }
func SGoNRecv(c *SGoChan) (int, bool) {
	var v int
	ok := sgochanrecv2(c, unsafe.Pointer(&v))
	return v, ok
}
func SGoNClose(c *SGoChan) { sgoclosechan(c) }
func SGoNBlock()           { sgoblock() }

// SGoNArm is a select arm for SGoNSelect.
type SGoNArm struct {
	Send bool
	Ch   *SGoChan
	Val  int
}

// SGoNSelect runs a select over arms, laid out as walk does (sends in source order first, then the
// receives from the end) with the source index of each case in order (D50). It returns the index
// of the chosen arm in arms (-1: default), the received value, and ok.
func SGoNSelect(arms []SGoNArm, block bool) (int, int, bool) {
	n := len(arms)
	idx := make([]int, n)
	nsends, nrecvs := 0, 0
	for i, a := range arms {
		if a.Send {
			idx[nsends] = i
			nsends++
		} else {
			nrecvs++
			idx[n-nrecvs] = i
		}
	}
	cas := make([]sgoScase, n)
	vals := make([]int, n)
	order := make([]uint16, 2*n+1)
	for k, i := range idx {
		vals[k] = arms[i].Val
		cas[k] = sgoScase{arms[i].Ch, unsafe.Pointer(&vals[k])}
		order[k] = uint16(i)
	}
	var c0 *sgoScase
	if n > 0 {
		c0 = &cas[0]
	}
	k, ok := sgoselectgo(c0, &order[0], nil, nsends, nrecvs, block)
	if k < 0 {
		return -1, 0, false
	}
	return idx[k], vals[k], ok
}

// SGoNWaiters counts the waiters listed on c (loser cleanup: zero once nobody waits).
func SGoNWaiters(c *SGoChan) int {
	lock(&c.lock)
	n := 0
	for w := c.senders.first; w != nil; w = w.next {
		n++
	}
	for w := c.recvs.first; w != nil; w = w.next {
		n++
	}
	unlock(&c.lock)
	return n
}

// SGoIsCancelUnwind reports whether v is the internal cancellation-unwind signal.
func SGoIsCancelUnwind(v any) bool { _, ok := v.(*sgoCancelUnwind); return ok }

// SGoFailureKind returns the kind of a failure panic value ("" if none).
func SGoFailureKind(v any) string {
	if f, ok := v.(*sgoFailure); ok {
		return f.kind
	}
	if v == nil {
		return ""
	}
	return sgoFailureOf(v).kind
}

type SGoEvent = sgoEvent
type SGoLimiter = sgoLimiter
type SGoToken = sgoToken

func SGoEventNew() *SGoEvent             { return &sgoEvent{} }
func SGoEventSet(e *SGoEvent)            { sgoEventSet(e) }
func SGoEventWait(e *SGoEvent) error     { return sgoEventWait(e, sgoCurrent()) }
func SGoEventIsSet(e *SGoEvent) bool     { return sgoEventIsSet(e) }
func SGoLimiterNew(n int) *SGoLimiter    { return sgoLimiterNew(int64(n)) }
func SGoLimiterUsed(l *SGoLimiter) int64 { lock(&l.c.lock); defer unlock(&l.c.lock); return l.used }
func SGoAcquire(l *SGoLimiter) (*SGoToken, error) {
	return sgoLimiterAcquire(l, sgoCurrent())
}
func SGoRelease(k *SGoToken) { sgoTokenRelease(k) }

// Task result records (sgo_taskres.go; W22 review G2), with an int result.
func SGoTaskResNew() *SGoChan { return (*SGoChan)(sgo_taskResNew(unsafe.Pointer(sgoChanIntType.Elem))) }
func SGoTaskResSet(c *SGoChan, v int) {
	sgo_taskResSet(unsafe.Pointer(c), unsafe.Pointer(&v))
}
func SGoTaskResChan(c *SGoChan) *SGoChan { return (*SGoChan)(sgo_taskResChan(unsafe.Pointer(c))) }
