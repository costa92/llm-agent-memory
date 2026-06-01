package memory

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// TestM8CScoredStore_UsesRWMutex guards the next concurrency slice:
// scoredStore should no longer serialize every operation behind a
// single sync.Mutex.
func TestM8CScoredStore_UsesRWMutex(t *testing.T) {
	t.Helper()

	path := filepath.Join(".", "engine_scored_store.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != "scoredStore" {
				continue
			}
			st, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				t.Fatalf("scoredStore is no longer a struct")
			}
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					if name.Name != "mu" {
						continue
					}
					sel, ok := field.Type.(*ast.SelectorExpr)
					if !ok {
						t.Fatalf("scoredStore.mu is no longer a selector type")
					}
					pkg, _ := sel.X.(*ast.Ident)
					if pkg == nil || pkg.Name != "sync" || sel.Sel.Name != "RWMutex" {
						t.Fatalf("scoredStore.mu = %v, want sync.RWMutex", field.Type)
					}
					return
				}
			}
			t.Fatalf("scoredStore.mu field not found")
		}
	}

	t.Fatalf("scoredStore type not found in %s", path)
}
