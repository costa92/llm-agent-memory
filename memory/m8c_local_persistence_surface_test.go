package memory

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// TestM8CLocalPersistenceSurface_ContractAliasesNotCore guards the
// Phase 1a contract collapse: persistence-facing types must alias the
// leaf contract package (contractmem) and must NOT alias the framework
// package (coremem). The framework dependency is fully broken.
func TestM8CLocalPersistenceSurface_ContractAliasesNotCore(t *testing.T) {
	t.Helper()

	path := filepath.Join(".", "types_alias.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	required := map[string]bool{
		"Snapshot":     true,
		"SnapshotItem": true,
		"ImportMode":   true,
		"ImportReport": true,
		"Exporter":     true,
		"Importer":     true,
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || !required[typeSpec.Name.Name] {
				continue
			}
			if !typeSpec.Assign.IsValid() {
				t.Fatalf("%s must be an alias of the contract leaf, not a standalone redefinition", typeSpec.Name.Name)
			}
			sel, ok := typeSpec.Type.(*ast.SelectorExpr)
			if !ok {
				t.Fatalf("%s alias RHS is not a package selector: %T", typeSpec.Name.Name, typeSpec.Type)
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				t.Fatalf("%s alias RHS selector base is not an identifier", typeSpec.Name.Name)
			}
			if pkg.Name == "coremem" {
				t.Fatalf("%s still aliases the framework (coremem); it must alias the contract leaf (contractmem)", typeSpec.Name.Name)
			}
			if pkg.Name != "contractmem" {
				t.Fatalf("%s aliases %q; want contractmem", typeSpec.Name.Name, pkg.Name)
			}
		}
	}
}
