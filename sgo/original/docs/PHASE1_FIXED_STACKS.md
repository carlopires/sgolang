# Phase 1: Fixed Demand-Paged Stacks

## 1. Modifying the SSA Backend (`cmd/compile/internal/ssa`)
We must prevent the compiler from emitting stack overflow checks in function prologues.

**File: `cmd/compile/internal/ssa/stackframe.go`**
Locate `needStackCheck(f *Func) bool` and short-circuit it:
```go
func needStackCheck(f *Func) bool {
    // S-Go: Fixed 8MB virtual stacks. No dynamic checks needed.
    return false
}
```

**File: `cmd/compile/internal/amd64/ssa.go`**
Locate the assembly generator for `OpAMD64LoweredStackCheck` and neutralize it:
```go
case ssa.OpAMD64LoweredStackCheck:
    // S-Go: Remove the CMPQ SP, 16(R14) and JLS call_morestack
    // Do absolutely nothing.
    return
```

## 2. Modifying the Runtime (`src/runtime/stack.go`)
Delete Go's 2KB `newstack` and replace it with virtual memory mapping:
```go
const _SGoStackSize = 8 * 1024 * 1024 // 8 MB

func allocSGoStack() (stackBase, sp uintptr) {
    // 1. Reserve 8MB address space with PROT_NONE
    base := sysAlloc(_SGoStackSize, &memstats.stacks_sys)
    if base == nil {
        throw("out of virtual memory for S-Go stack")
    }
    
    // 2. Commit top 64KB for immediate use (PROT_READ | PROT_WRITE)
    topSize := uintptr(64 * 1024)
    topAddr := uintptr(base) + _SGoStackSize - topSize
    sysMap(unsafe.Pointer(topAddr), topSize, &memstats.stacks_sys)
    
    // The bottom page remains PROT_NONE to trigger SIGSEGV on overflow.
    return uintptr(base), uintptr(base) + _SGoStackSize - 8
}
```