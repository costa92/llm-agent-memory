package memory

import (
	"context"
	"errors"
	"os"
	"testing"

	coremem "github.com/costa92/llm-agent/memory"
)

// TestSQLiteStore_SatisfiesSnapshotStore is a compile-time + runtime
// assertion that SQLiteStore satisfies the coremem.SnapshotStore
// contract from llm-agent/memory/persistence.go:171-176.
func TestSQLiteStore_SatisfiesSnapshotStore(t *testing.T) {
	store := newTempSQLiteStore(t)
	var _ coremem.SnapshotStore = store // compile-time check
	// Touch each interface method via a no-op call.
	if _, err := store.List(context.Background()); err != nil {
		t.Errorf("List on empty store: %v", err)
	}
	if err := store.Delete(context.Background(), "nonexistent"); err != nil {
		t.Errorf("Delete on missing key should be a no-op, got: %v", err)
	}
	if _, err := store.Load(context.Background(), "nonexistent"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Load on missing key err = %v, want wraps os.ErrNotExist", err)
	}
}
