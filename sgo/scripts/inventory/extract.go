// Command extract lists the compiler/runtime surfaces that S-Go admission must cover (W01 step 3).
// It parses the fork's own sources with go/parser and prints one JSON object per surface:
// {"kind":..., "name":..., "file":..., "line":...}. Built and run with the pinned bootstrap Go.
//
// Usage: extract <fork root>
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

type surface struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
}

var (
	root string
	fset = token.NewFileSet()
	out  []surface
	ident = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func parse(rel string) *ast.File {
	f, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintln(os.Stderr, "extract:", err)
		os.Exit(1)
	}
	return f
}

func add(kind, name string, pos token.Pos) {
	p := fset.Position(pos)
	rel, _ := filepath.Rel(root, p.Filename)
	out = append(out, surface{kind, name, filepath.ToSlash(rel), p.Line})
}

// typedConsts records the constants of an iota block whose first spec has type typ.
func typedConsts(kind, rel, typ string, skip func(string) bool) {
	f := parse(rel)
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		inBlock := false
		for _, s := range g.Specs {
			vs := s.(*ast.ValueSpec)
			if id, ok := vs.Type.(*ast.Ident); ok {
				inBlock = id.Name == typ
			}
			if !inBlock {
				continue
			}
			for _, n := range vs.Names {
				if n.Name == "_" || (skip != nil && skip(n.Name)) {
					continue
				}
				add(kind, n.Name, n.Pos())
			}
		}
	}
}

func syntaxNodes(rel string) {
	f := parse(rel)
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.TYPE {
			continue
		}
		for _, s := range g.Specs {
			ts := s.(*ast.TypeSpec)
			st, ok := ts.Type.(*ast.StructType)
			if !ok || !ast.IsExported(ts.Name.Name) {
				continue
			}
			// Concrete syntax nodes embed one of the node kinds.
			for _, fld := range st.Fields.List {
				if len(fld.Names) == 0 {
					if id, ok := fld.Type.(*ast.Ident); ok {
						switch id.Name {
						case "node", "expr", "stmt", "simpleStmt", "decl":
							add("syntax-node", ts.Name.Name, ts.Name.Pos())
						}
					}
				}
			}
		}
	}
}

// universe records predeclared identifiers from types2/universe.go.
func universe(rel string) {
	f := parse(rel)
	tables := map[string]bool{"Typ": true, "basicAliases": true, "predeclaredConsts": true, "predeclaredFuncs": true}
	seen := map[string]bool{}
	record := func(name string, pos token.Pos) {
		if ident.MatchString(name) && !seen[name] {
			seen[name] = true
			add("universe", name, pos)
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ValueSpec:
			if len(n.Names) == 1 && tables[n.Names[0].Name] && len(n.Values) == 1 {
				if cl, ok := n.Values[0].(*ast.CompositeLit); ok {
					for _, e := range cl.Elts {
						if kv, ok := e.(*ast.KeyValueExpr); ok {
							e = kv.Value
						}
						if el, ok := e.(*ast.CompositeLit); ok {
							for _, x := range el.Elts {
								if bl, ok := x.(*ast.BasicLit); ok && bl.Kind == token.STRING {
									s, _ := strconv.Unquote(bl.Value)
									record(s, bl.Pos())
									break
								}
							}
						}
					}
				}
				return false
			}
		case *ast.CallExpr:
			if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "NewTypeName" && len(n.Args) >= 3 {
				if bl, ok := n.Args[2].(*ast.BasicLit); ok && bl.Kind == token.STRING {
					s, _ := strconv.Unquote(bl.Value)
					record(s, bl.Pos())
				}
			}
		case *ast.KeyValueExpr:
			if k, ok := n.Key.(*ast.Ident); ok && k.Name == "name" {
				if bl, ok := n.Value.(*ast.BasicLit); ok && bl.Kind == token.STRING {
					s, _ := strconv.Unquote(bl.Value)
					record(s, bl.Pos())
				}
			}
		}
		return true
	})
}

func funcs(kind, rel string) {
	f := parse(rel)
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			add(kind, fd.Name.Name, fd.Name.Pos())
		}
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: extract <fork root>")
		os.Exit(2)
	}
	root = os.Args[1]
	syn := "src/cmd/compile/internal/syntax/"
	typedConsts("token", syn+"tokens.go", "token", nil)
	typedConsts("operator", syn+"tokens.go", "Operator", nil)
	syntaxNodes(syn + "nodes.go")
	universe("src/cmd/compile/internal/types2/universe.go")
	funcs("runtime-builtin", "src/cmd/compile/internal/typecheck/_builtin/runtime.go")
	funcs("runtime-builtin", "src/cmd/compile/internal/typecheck/_builtin/coverage.go")
	typedConsts("ir-op", "src/cmd/compile/internal/ir/node.go", "Op", func(n string) bool { return n == "OEND" })
	sort.SliceStable(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	enc := json.NewEncoder(os.Stdout)
	for _, s := range out {
		enc.Encode(s)
	}
}
