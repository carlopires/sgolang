# Phase 4: Deterministic Lab Scheduler (`src/runtime/lab_sched.go`)

Must be 100% single-threaded. Time is virtual.
```go
package runtime

var lab struct {
    enabled   bool
    seed      uint64
    prngState uint64
    prngInc   uint64
    vClock    uint64
    runq      []*g
    timers    *labTimer
}

func labNextG() *g {
    if len(lab.runq) == 0 { return nil }
    
    // PCG32 Rand
    old := lab.prngState
    lab.prngState = old*6364136223846793005 + lab.prngInc
    x := uint32(((old >> 18) ^ old) >> 27)
    r := uint32(old >> 59)
    rnd := (x >> r) | (x << ((-r) & 31))
    
    idx := rnd % uint32(len(lab.runq))
    gp := lab.runq[idx]
    
    // Remove from slice
    lab.runq[idx] = lab.runq[len(lab.runq)-1]
    lab.runq = lab.runq[:len(lab.runq)-1]
    return gp
}
```