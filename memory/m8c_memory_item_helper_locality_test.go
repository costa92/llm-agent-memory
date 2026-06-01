package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestM8CMemoryItemHelpers_PackageLogicUsesLocalHelpers guards the pre-lift
// slice before MemoryItem becomes sibling-owned: package logic should stop
// reaching into core helper functions for item flags/source/category.
func TestM8CMemoryItemHelpers_PackageLogicUsesLocalHelpers(t *testing.T) {
	t.Helper()

	files := []string{
		"engine_list.go",
		"engine_scored_store.go",
		"engine_working.go",
		"engine_episodic.go",
		"engine_semantic.go",
		"scoped_lifecycle.go",
	}
	for _, name := range files {
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		text := string(src)
		for _, forbidden := range []string{
			"coremem.IsPinned(",
			"coremem.IsDisabled(",
			"coremem.GetSource(",
			"coremem.GetCategory(",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s still references %s; prefer sibling-local item helpers", name, forbidden)
			}
		}
	}
}
