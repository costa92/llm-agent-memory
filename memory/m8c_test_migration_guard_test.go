package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestM8CTestMigration_SelectedSuitesUseLocalConstructors guards the
// first M8c migration slice: the sibling's own manager / recall /
// benchmark tests should stop depending on core-only test constructors
// once local engine constructors exist in this package.
func TestM8CTestMigration_SelectedSuitesUseLocalConstructors(t *testing.T) {
	t.Helper()

	files := []string{
		"consolidator_test.go",
		"scoped_lifecycle_test.go",
		"sqlite_store_test.go",
		"write_policy_test.go",
		"manager_test.go",
		"recall_engine_test.go",
		"internal_score_bench_test.go",
	}
	for _, name := range files {
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		text := string(src)
		for _, forbidden := range []string{
			"newCoreManager(",
			"newCoreWorking(",
			"newCoreEpisodic(",
			"newCoreSemantic(",
			"newCoreWorkingWithCapacity(",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s still references %s; migrate this suite to local constructors first", name, forbidden)
			}
		}
	}
}
