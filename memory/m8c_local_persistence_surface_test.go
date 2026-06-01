package memory

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// TestM8CLocalPersistenceSurface_NotCoreAliases guards the next M8c slice:
// persistence-facing sibling types should no longer be plain aliases to core.
func TestM8CLocalPersistenceSurface_NotCoreAliases(t *testing.T) {
	t.Helper()

	path := filepath.Join(".", "types_alias.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	blocked := map[string]bool{
		"Snapshot":    true,
		"SnapshotItem": true,
		"ImportMode":  true,
		"ImportReport": true,
		"Exporter":    true,
		"Importer":    true,
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || !blocked[typeSpec.Name.Name] {
				continue
			}
			if typeSpec.Assign.IsValid() {
				t.Fatalf("%s is still declared as a core alias in types_alias.go", typeSpec.Name.Name)
			}
		}
	}
}
