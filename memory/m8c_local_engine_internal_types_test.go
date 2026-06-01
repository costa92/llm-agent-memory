package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestM8CLocalEngineInternals_PreferLocalAliasTypes guards the next
// M8c slice: internal sibling engine files should stop spelling local
// aliasable types through coremem selectors.
func TestM8CLocalEngineInternals_PreferLocalAliasTypes(t *testing.T) {
	t.Helper()

	files := []string{
		"engine_scored_store.go",
		"engine_list.go",
	}
	for _, name := range files {
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		text := string(src)
		for _, forbidden := range []string{
			"coremem.MemoryItem",
			"coremem.Stats",
			"coremem.ListFilter",
			"coremem.ListPage",
			"coremem.Scope",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s still references %s; prefer sibling-local alias types here", name, forbidden)
			}
		}
	}
}
