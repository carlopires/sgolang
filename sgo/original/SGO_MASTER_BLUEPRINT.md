# S-Go: Master Orchestration Blueprint
## How to Build This Language Fork Without LLM Drift

Do not attempt to implement this entire repository at once. The Go toolchain is self-hosting and requires bootstrapping. If you introduce syntax, IR, and runtime changes simultaneously, the bootstrap compiler will fail with circular dependency errors.

### The Bootstrap Implementation Order:
1. **PHASE 1: Strip Relocatable Stacks.** Change `src/runtime` to use fixed 8MB virtual stacks and remove `LoweredStackCheck` from the SSA backend. **Compile and test standard Go programs first** to ensure the new stack model is stable.
2. **PHASE 2: Frontend Syntax.** Add the `nursery` keyword to the scanner and parser. Verify `go tool compile` can parse the AST.
3. **PHASE 3: IR Desugaring.** Map the `NurseryStmt` AST into `ir` nodes in the compiler frontend. This requires strict Garbage Collector alignment (`OpVARDEF`).
4. **PHASE 4: Deterministic Lab Scheduler.** Implement the PCG32 single-threaded event loop.
5. **PHASE 5: BlenD2 Contracts.** Refactor channels to the Two-Phase `Reserve -> Commit` protocol.
