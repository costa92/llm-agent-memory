package memory

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	coremem "github.com/costa92/llm-agent/memory"
)

var sqliteTestCounter uint64

// newTempSQLiteStore creates a fresh in-memory SQLiteStore with a
// unique URL-encoded name so concurrent tests do not share state.
// The returned cleanup function MUST be deferred (or registered via
// t.Cleanup, which the helper already does).
func newTempSQLiteStore(t *testing.T) *SQLiteStore {
	t.Helper()
	n := atomic.AddUint64(&sqliteTestCounter, 1)
	// `file:` + a unique `name=` parameter + `mode=memory` + `cache=shared`
	// gives each test its own in-memory database that survives across
	// pool connections within this test only.
	dsn := fmt.Sprintf("file:sqlitetest_%d?mode=memory&cache=shared", n)
	store, err := NewSQLiteStore(dsn)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("SQLiteStore.Close: %v", err)
		}
	})
	return store
}

// assertSnapshotEqual compares two coremem.Snapshot values without
// using reflect.DeepEqual — Item.CreatedAt / AccessedAt come back
// from JSON without monotonic clock, so reflect.DeepEqual on a
// round-trip is a known false-negative trap (see M2 Task 11 BLOCKED).
func assertSnapshotEqual(t *testing.T, got, want coremem.Snapshot) {
	t.Helper()
	if got.Version != want.Version {
		t.Errorf("Version: got %d, want %d", got.Version, want.Version)
	}
	if got.Kind != want.Kind {
		t.Errorf("Kind: got %q, want %q", got.Kind, want.Kind)
	}
	if len(got.Items) != len(want.Items) {
		t.Fatalf("len(Items): got %d, want %d", len(got.Items), len(want.Items))
	}
	for i := range got.Items {
		gi, wi := got.Items[i], want.Items[i]
		if gi.Item.ID != wi.Item.ID {
			t.Errorf("Items[%d].Item.ID: got %q, want %q", i, gi.Item.ID, wi.Item.ID)
		}
		if gi.Item.Content != wi.Item.Content {
			t.Errorf("Items[%d].Item.Content: got %q, want %q", i, gi.Item.Content, wi.Item.Content)
		}
		if !gi.Item.CreatedAt.Equal(wi.Item.CreatedAt) {
			t.Errorf("Items[%d].Item.CreatedAt: got %v, want %v", i, gi.Item.CreatedAt, wi.Item.CreatedAt)
		}
		if len(gi.Vector) != len(wi.Vector) {
			t.Errorf("Items[%d].Vector len: got %d, want %d", i, len(gi.Vector), len(wi.Vector))
			continue
		}
		for j := range gi.Vector {
			if gi.Vector[j] != wi.Vector[j] {
				t.Errorf("Items[%d].Vector[%d]: got %v, want %v", i, j, gi.Vector[j], wi.Vector[j])
				break
			}
		}
	}
	_ = context.Background() // pin the import so cleanup of unused imports doesn't bite later
}
