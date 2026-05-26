package memory

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

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

func TestSQLiteStore_Save_Load_RoundTripsSingleKind(t *testing.T) {
	store := newTempSQLiteStore(t)
	ctx := context.Background()
	snap := coremem.Snapshot{
		Version: coremem.SnapshotVersion,
		Kind:    coremem.KindEpisodic,
		Items: []coremem.SnapshotItem{
			{
				Item: coremem.MemoryItem{
					ID: "a", Content: "alpha", Importance: 0.5,
					CreatedAt: time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC),
				},
				Vector: []float32{0.1, 0.2, 0.3},
			},
		},
	}
	if err := store.Save(ctx, "session-1", snap); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Load(ctx, "session-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	assertSnapshotEqual(t, got, snap)
}

func TestSQLiteStore_LoadKind_SelectsExactKind(t *testing.T) {
	store := newTempSQLiteStore(t)
	ctx := context.Background()
	mk := func(kind coremem.Kind, content string) coremem.Snapshot {
		return coremem.Snapshot{
			Version: coremem.SnapshotVersion,
			Kind:    kind,
			Items: []coremem.SnapshotItem{
				{Item: coremem.MemoryItem{ID: "x", Content: content}, Vector: []float32{1}},
			},
		}
	}
	if err := store.Save(ctx, "k", mk(coremem.KindWorking, "w")); err != nil {
		t.Fatalf("Save working: %v", err)
	}
	if err := store.Save(ctx, "k", mk(coremem.KindEpisodic, "e")); err != nil {
		t.Fatalf("Save episodic: %v", err)
	}
	wsnap, err := store.LoadKind(ctx, "k", coremem.KindWorking)
	if err != nil {
		t.Fatalf("LoadKind working: %v", err)
	}
	if wsnap.Items[0].Item.Content != "w" {
		t.Errorf("LoadKind working Content = %q, want %q", wsnap.Items[0].Item.Content, "w")
	}
	esnap, err := store.LoadKind(ctx, "k", coremem.KindEpisodic)
	if err != nil {
		t.Fatalf("LoadKind episodic: %v", err)
	}
	if esnap.Items[0].Item.Content != "e" {
		t.Errorf("LoadKind episodic Content = %q, want %q", esnap.Items[0].Item.Content, "e")
	}
}

func TestSQLiteStore_LoadKind_MissingReturnsErrNotExist(t *testing.T) {
	store := newTempSQLiteStore(t)
	_, err := store.LoadKind(context.Background(), "missing", coremem.KindWorking)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("LoadKind missing err = %v, want wraps os.ErrNotExist", err)
	}
}

func TestSQLiteStore_Delete_RemovesAllKindsAtKey(t *testing.T) {
	store := newTempSQLiteStore(t)
	ctx := context.Background()
	for _, kind := range []coremem.Kind{coremem.KindWorking, coremem.KindEpisodic} {
		if err := store.Save(ctx, "k", coremem.Snapshot{
			Version: coremem.SnapshotVersion,
			Kind:    kind,
			Items:   []coremem.SnapshotItem{{Item: coremem.MemoryItem{ID: "x"}, Vector: []float32{1}}},
		}); err != nil {
			t.Fatalf("Save %v: %v", kind, err)
		}
	}
	if err := store.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.LoadKind(ctx, "k", coremem.KindWorking); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("LoadKind working post-delete err = %v, want os.ErrNotExist", err)
	}
	if _, err := store.LoadKind(ctx, "k", coremem.KindEpisodic); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("LoadKind episodic post-delete err = %v, want os.ErrNotExist", err)
	}
}

func TestSQLiteStore_List_ReturnsSortedUniqueKeys(t *testing.T) {
	store := newTempSQLiteStore(t)
	ctx := context.Background()
	keys := []string{"charlie", "alpha", "bravo"}
	for _, k := range keys {
		if err := store.Save(ctx, k, coremem.Snapshot{
			Version: coremem.SnapshotVersion,
			Kind:    coremem.KindWorking,
			Items:   []coremem.SnapshotItem{{Item: coremem.MemoryItem{ID: "x"}, Vector: []float32{1}}},
		}); err != nil {
			t.Fatalf("Save %q: %v", k, err)
		}
	}
	// Save same key with a different kind — must not duplicate in List.
	if err := store.Save(ctx, "alpha", coremem.Snapshot{
		Version: coremem.SnapshotVersion,
		Kind:    coremem.KindEpisodic,
		Items:   []coremem.SnapshotItem{{Item: coremem.MemoryItem{ID: "x"}, Vector: []float32{1}}},
	}); err != nil {
		t.Fatalf("Save alpha episodic: %v", err)
	}
	got, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"alpha", "bravo", "charlie"}
	if len(got) != len(want) {
		t.Fatalf("List got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("List[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSQLiteStore_Save_OnConflict_OverwritesExistingRow(t *testing.T) {
	store := newTempSQLiteStore(t)
	ctx := context.Background()
	snap1 := coremem.Snapshot{
		Version: coremem.SnapshotVersion,
		Kind:    coremem.KindWorking,
		Items:   []coremem.SnapshotItem{{Item: coremem.MemoryItem{ID: "a", Content: "v1"}, Vector: []float32{1}}},
	}
	snap2 := coremem.Snapshot{
		Version: coremem.SnapshotVersion,
		Kind:    coremem.KindWorking,
		Items:   []coremem.SnapshotItem{{Item: coremem.MemoryItem{ID: "a", Content: "v2"}, Vector: []float32{2}}},
	}
	if err := store.Save(ctx, "k", snap1); err != nil {
		t.Fatalf("Save v1: %v", err)
	}
	if err := store.Save(ctx, "k", snap2); err != nil {
		t.Fatalf("Save v2: %v", err)
	}
	got, err := store.LoadKind(ctx, "k", coremem.KindWorking)
	if err != nil {
		t.Fatalf("LoadKind: %v", err)
	}
	if got.Items[0].Item.Content != "v2" {
		t.Errorf("Content = %q, want %q (UPSERT did not overwrite)", got.Items[0].Item.Content, "v2")
	}
}

func TestSQLiteStore_Migration_IsIdempotentAcrossReopens(t *testing.T) {
	// Open, close, reopen the SAME shared in-memory DSN. The second
	// open must NOT fail and must NOT insert a duplicate version row.
	dsn := "file:sqlitetest_idem?mode=memory&cache=shared"

	// Hold a sentinel open to keep the shared-memory DB alive across the
	// two NewSQLiteStore calls in this test (closing the only conn would
	// destroy the DB).
	sentinel, err := NewSQLiteStore(dsn)
	if err != nil {
		t.Fatalf("sentinel open: %v", err)
	}
	t.Cleanup(func() { _ = sentinel.Close() })

	first, err := NewSQLiteStore(dsn)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	v1, err := first.currentVersion(context.Background())
	if err != nil {
		t.Fatalf("first currentVersion: %v", err)
	}
	if v1 != SchemaVersion {
		t.Errorf("first currentVersion = %d, want %d", v1, SchemaVersion)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}

	second, err := NewSQLiteStore(dsn)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
	v2, err := second.currentVersion(context.Background())
	if err != nil {
		t.Fatalf("second currentVersion: %v", err)
	}
	if v2 != SchemaVersion {
		t.Errorf("second currentVersion = %d, want %d", v2, SchemaVersion)
	}

	// Count rows: must be exactly 1, not 2 (no duplicate INSERTs).
	var n int
	if err := second.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM memory_store_schema`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != SchemaVersion {
		t.Errorf("memory_store_schema rows = %d, want %d", n, SchemaVersion)
	}
}

func TestSQLiteStore_NewSQLiteStore_RefusesFutureSchemaVersion(t *testing.T) {
	dsn := "file:sqlitetest_future?mode=memory&cache=shared"
	sentinel, err := NewSQLiteStore(dsn)
	if err != nil {
		t.Fatalf("sentinel open: %v", err)
	}
	t.Cleanup(func() { _ = sentinel.Close() })

	// Forge a future-version row by writing directly to the shared DB.
	if _, err := sentinel.db.ExecContext(context.Background(),
		`INSERT INTO memory_store_schema (version, applied_at) VALUES (?, ?)`,
		SchemaVersion+1, "2099-01-01T00:00:00Z",
	); err != nil {
		t.Fatalf("forge future version: %v", err)
	}

	// Re-opening should detect SchemaVersion+1 > SchemaVersion and refuse.
	_, err = NewSQLiteStore(dsn)
	if !errors.Is(err, ErrSchemaVersionAhead) {
		t.Errorf("NewSQLiteStore err = %v, want errors.Is ErrSchemaVersionAhead", err)
	}
}
