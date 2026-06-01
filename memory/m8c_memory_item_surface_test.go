package memory

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// TestM8CMemoryItemSurface_NotCoreAlias guards the next breaking M8c slice:
// MemoryItem should eventually become sibling-owned rather than a direct core
// alias.
func TestM8CMemoryItemSurface_NotCoreAlias(t *testing.T) {
	t.Helper()

	path := filepath.Join(".", "types_alias.go")
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
			if !ok || typeSpec.Name.Name != "MemoryItem" {
				continue
			}
			if typeSpec.Assign.IsValid() {
				t.Fatalf("MemoryItem is still declared as a core alias in types_alias.go")
			}
			return
		}
	}
	t.Fatalf("MemoryItem type not found in %s", path)
}
