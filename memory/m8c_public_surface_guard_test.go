package memory

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// TestM8CPublicSurface_PrefersLocalAliases guards the next decoupling slice:
// sibling-owned public surfaces should expose local alias names (Kind,
// MemoryItem, Snapshot, etc.) rather than spelling coremem selectors directly
// in exported signatures.
func TestM8CPublicSurface_PrefersLocalAliases(t *testing.T) {
	t.Helper()

	files := []string{
		"engine_working.go",
		"engine_episodic.go",
		"engine_semantic.go",
		"manager.go",
		"consolidator.go",
		"parallel_search.go",
		"recall_engine.go",
		"unified_search.go",
		"write_policy.go",
		"scoped_manager_local.go",
		"scoped_lifecycle.go",
	}

	fset := token.NewFileSet()
	for _, name := range files {
		path := filepath.Join(".", name)
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if !d.Name.IsExported() {
					continue
				}
				if name == "write_policy.go" && d.Name.Name == "Sanitize" {
					continue
				}
				if containsCorememSelectorInFieldList(d.Type.Params) || containsCorememSelectorInFieldList(d.Type.Results) {
					t.Fatalf("%s: exported func %s still exposes coremem selectors in its signature", name, d.Name.Name)
				}
			case *ast.GenDecl:
				if d.Tok != token.TYPE {
					continue
				}
				for _, spec := range d.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok || !typeSpec.Name.IsExported() {
						continue
					}
					switch tt := typeSpec.Type.(type) {
					case *ast.StructType:
						if containsCorememSelectorInFieldList(tt.Fields) {
							t.Fatalf("%s: exported type %s still exposes coremem selectors in struct fields", name, typeSpec.Name.Name)
						}
					case *ast.InterfaceType:
						if containsCorememSelectorInFieldList(tt.Methods) {
							t.Fatalf("%s: exported type %s still exposes coremem selectors in interface methods", name, typeSpec.Name.Name)
						}
					}
				}
			}
		}
	}
}

func containsCorememSelectorInFieldList(fields *ast.FieldList) bool {
	if fields == nil {
		return false
	}
	for _, field := range fields.List {
		if containsCorememSelector(field.Type) {
			return true
		}
	}
	return false
}

func containsCorememSelector(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if ok && id.Name == "coremem" {
			found = true
			return false
		}
		return true
	})
	return found
}
