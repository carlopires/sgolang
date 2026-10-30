# Phase 2: Lexer and AST Nodes

**File: `cmd/compile/internal/syntax/tokens.go`**
```go
const (
    _Nursery // nursery
    // ... existing tokens
)
var keywordMap = map[string]token{
    "nursery": _Nursery,
}
```

**File: `cmd/compile/internal/syntax/nodes.go`**
```go
type NurseryStmt struct {
    Context *Name
    Body    *BlockStmt
    stmt
}
```

**File: `cmd/compile/internal/syntax/parser.go`**
```go
func (p *parser) nurseryStmt() *NurseryStmt {
    s := new(NurseryStmt)
    s.pos = p.pos()
    p.next() // consume 'nursery'

    if p.got(_Lparen) {
        if p.tok == _Name {
            s.Context = p.name()
        } else {
            p.syntaxError("expecting identifier inside nursery()")
        }
        p.want(_Rparen)
    }
    s.Body = p.blockStmt("nursery block")
    return s
}
```