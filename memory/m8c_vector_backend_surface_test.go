package memory

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// TestM8CVectorBackendSurface_EpisodicAndSemanticExposeBackend guards the
// next M8c slice: read-mostly memories need an explicit vector backend seam.
func TestM8CVectorBackendSurface_EpisodicAndSemanticExposeBackend(t *testing.T) {
	t.Helper()

	assertStructHasField(t, "engine_episodic.go", "EpisodicOptions", "VectorBackend")
	assertStructHasField(t, "engine_semantic.go", "SemanticOptions", "VectorBackend")
}

func assertStructHasField(t *testing.T, fileName, typeName, fieldName string) {
	t.Helper()

	path := filepath.Join(".", fileName)
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
			if !ok || typeSpec.Name.Name != typeName {
				continue
			}
			st, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				t.Fatalf("%s in %s is no longer a struct", typeName, fileName)
			}
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					if name.Name == fieldName {
						return
					}
				}
			}
			t.Fatalf("%s in %s is missing field %s", typeName, fileName, fieldName)
		}
	}
	t.Fatalf("%s not found in %s", typeName, fileName)
}
