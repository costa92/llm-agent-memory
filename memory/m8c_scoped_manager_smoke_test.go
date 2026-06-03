package memory

import (
	"context"
	"errors"
	"testing"
)

func TestM8CScopedManager_LocalScopeStampAndFilter(t *testing.T) {
	w, e, s := newWorking(t), newEpisodic(t), newSemantic(t)
	mgr, err := NewManager(Options{
		Working:  TierOptions{Memory: w, Lister: w, Exporter: w, Importer: w},
		Episodic: TierOptions{Memory: e, Lister: e, Exporter: e, Importer: e},
		Semantic: TierOptions{Memory: s, Lister: s, Exporter: s, Importer: s},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	sm, err := NewScopedManager(mgr)
	if err != nil {
		t.Fatalf("NewScopedManager: %v", err)
	}

	alice := WithScope(context.Background(), Scope{User: "alice"})
	bob := WithScope(context.Background(), Scope{User: "bob"})

	id, err := sm.Add(alice, KindWorking, MemoryItem{Content: "alice-memory", Importance: 0.8})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := sm.Get(bob, KindWorking, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get from other scope err = %v, want ErrNotFound", err)
	}
	got, err := sm.Get(alice, KindWorking, id)
	if err != nil {
		t.Fatalf("Get from same scope: %v", err)
	}
	if readScope(got).User != "alice" {
		t.Fatalf("scope user = %q, want alice", readScope(got).User)
	}
}
