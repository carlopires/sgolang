# Phase 5: BlenD2 Boundary Protocols

Replace Go's atomic single-step channels with a two-phase Reserve/Commit model.
```go
type hchan struct {
    // ... existing fields ...
    sendReserve uint32
    recvReserve uint32
}

func (c *hchan) ReserveSend(cx *Context) (*SendPermit, error) {
    lock(&c.lock)
    for {
        if cx.cancelFlag != 0 {
            unlock(&c.lock)
            return nil, ErrCancelled
        }
        if c.qcount + c.sendReserve < c.dataqsiz {
            c.sendReserve++
            unlock(&c.lock)
            return &SendPermit{c: c}, nil
        }
        gopark(unlockf, &c.lock, waitReasonChanSend, traceEvGoBlockSend, 1)
        lock(&c.lock)
    }
}

func (p *SendPermit) Commit(val unsafe.Pointer) {
    c := p.c
    lock(&c.lock)
    c.sendReserve--
    // Actual memory write - cannot fail
    typedmemmove(c.elemtype, chanbuf(c, c.sendx), val)
    c.sendx = (c.sendx + 1) % c.dataqsiz
    c.qcount++
    unlock(&c.lock)
}
```