package memory

import (
	"strconv"
	"strings"
	"testing"
)

func TestVersion_NonEmptyAndSemver(t *testing.T) {
	if Version == "" {
		t.Fatal("Version is empty")
	}
	parts := strings.Split(Version, ".")
	if len(parts) != 3 {
		t.Fatalf("Version = %q, want three dot-separated parts", Version)
	}
	for i, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			t.Errorf("Version part %d (%q) not numeric: %v", i, p, err)
		}
	}
}
