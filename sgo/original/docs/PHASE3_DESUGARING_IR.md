# Phase 3: AST to IR Translation (`cmd/compile/internal/noder`)

This is the most critical phase. If the Garbage Collector is not informed about the stack-allocated `NurseryScope`, it will aggressively garbage-collect child task pointers, causing catastrophic memory corruption.

**File: `cmd/compile/internal/noder/stmt.go`**
Implement the AST to IR translation for `syntax.NurseryStmt`:
```go
func (g *gen) nurseryStmt(stmt *syntax.NurseryStmt) *ir.BlockStmt {
    pos := stmt.Pos()
    block := ir.NewBlockStmt(pos, nil)

    // 1. Allocate 'var __scope runtime.NurseryScope' on the stack
    scopeType := typecheck.LookupRuntime("NurseryScope")
    scopeVar := ir.NewNameAt(pos, typecheck.Lookup("__scope"))
    scopeVar.SetType(scopeType)
    scopeVar.Class = ir.PAUTO // Local stack variable
    
    decl := ir.NewDecl(pos, ir.ODCL, scopeVar)
    block.List.Append(decl)

    // 2. EMIT GC ROOT: Tell GC this memory is now live
    vardef := ir.NewUnaryExpr(pos, ir.OVARDEF, scopeVar)
    block.List.Append(vardef)

    // 3. runtime.ScopeInit(&__scope, runtime.getg())
    initFn := typecheck.LookupRuntime("ScopeInit")
    addrExpr := typecheck.Expr(ir.NewAddrExpr(pos, scopeVar))
    getgFn := typecheck.LookupRuntime("getCurTask")
    getgCall := ir.NewCallExpr(pos, ir.OCALL, getgFn, nil)
    
    initCall := ir.NewCallExpr(pos, ir.OCALL, initFn, []ir.Node{addrExpr, getgCall})
    block.List.Append(initCall)

    // 4. defer runtime.ScopeQuiesce(&__scope)
    quiesceFn := typecheck.LookupRuntime("ScopeQuiesce")
    quiesceCall := ir.NewCallExpr(pos, ir.OCALL, quiesceFn, []ir.Node{addrExpr})
    deferStmt := ir.NewDeferStmt(pos, quiesceCall)
    block.List.Append(deferStmt)

    // 5. Setup the user's Context variable if provided
    if stmt.Context != nil {
        cxFn := typecheck.LookupRuntime("ContextInit")
        cxCall := ir.NewCallExpr(pos, ir.OCALL, cxFn, []ir.Node{addrExpr})
        
        cxVar := ir.NewNameAt(stmt.Context.Pos(), typecheck.Lookup(stmt.Context.Value))
        cxVar.SetType(typecheck.LookupRuntime("Context"))
        cxVar.Class = ir.PAUTO
        
        cxDecl := ir.NewDecl(pos, ir.ODCL, cxVar)
        cxAssign := ir.NewAssignStmt(pos, cxVar, cxCall)
        
        block.List.Append(cxDecl)
        block.List.Append(cxAssign)
    }

    // 6. Append the user's block body
    body := g.stmts(stmt.Body.List)
    block.List.Append(body...)

    return block
}
```