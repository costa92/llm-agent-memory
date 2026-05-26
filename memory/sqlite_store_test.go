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
