package memory

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// TestM8CMemoryItemSurface_ContractAliasNotCore guards the Phase 1a
// contract collapse: MemoryItem must alias the leaf contract package
// (contractmem.MemoryItem) and must NOT alias the framework package
// (coremem). The framework dependency is fully broken — the contract
// leaf is now the single source of truth for the data type.
func TestM8CMemoryItemSurface_ContractAliasNotCore(t *testing.T) {
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
			if !typeSpec.Assign.IsValid() {
				t.Fatalf("MemoryItem must be an alias of contractmem.MemoryItem, not a standalone redefinition")
			}
			sel, ok := typeSpec.Type.(*ast.SelectorExpr)
			if !ok {
				t.Fatalf("MemoryItem alias RHS is not a package selector: %T", typeSpec.Type)
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				t.Fatalf("MemoryItem alias RHS selector base is not an identifier")
			}
			if pkg.Name == "coremem" {
				t.Fatalf("MemoryItem still aliases the framework (coremem); it must alias the contract leaf (contractmem)")
			}
			if pkg.Name != "contractmem" {
				t.Fatalf("MemoryItem aliases %q.%s; want contractmem.MemoryItem", pkg.Name, sel.Sel.Name)
			}
			return
		}
	}
	t.Fatalf("MemoryItem type not found in %s", path)
}
