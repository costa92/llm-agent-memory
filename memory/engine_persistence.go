package memory

import (
	"fmt"
	"sort"
)

func exportFromStore(s *scoredStore, kind Kind) Snapshot {
	items, vecs := s.snapshot()
	snap := Snapshot{
		Version: SnapshotVersion,
		Kind:    kind,
		Items:   make([]SnapshotItem, 0, len(items)),
	}
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		ai, aj := items[ids[i]], items[ids[j]]
		if !ai.CreatedAt.Equal(aj.CreatedAt) {
			return ai.CreatedAt.Before(aj.CreatedAt)
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		snap.Items = append(snap.Items, SnapshotItem{
			Item:   items[id],
			Vector: vecs[id],
		})
	}
	return snap
}

func importIntoStore(s *scoredStore, snap Snapshot, mode ImportMode) (ImportReport, error) {
	if snap.Version != SnapshotVersion {
		return ImportReport{}, fmt.Errorf("%w: got %d, want %d", ErrSnapshotVersionMismatch, snap.Version, SnapshotVersion)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var rpt ImportReport
	if mode == ImportReplace {
		s.items = make(map[string]MemoryItem, len(snap.Items))
		s.vectors = make(map[string][]float32, len(snap.Items))
	}
	for _, si := range snap.Items {
		id := si.Item.ID
		if id == "" {
			rpt.Errors = append(rpt.Errors, fmt.Errorf("memory: snapshot item with empty ID, skipped"))
			continue
		}
		_, exists := s.items[id]
		switch mode {
		case ImportMerge:
			if exists {
				rpt.Skipped++
				continue
			}
			s.items[id] = si.Item
			if si.Vector != nil {
				cp := make([]float32, len(si.Vector))
				copy(cp, si.Vector)
				s.vectors[id] = cp
			}
			rpt.Loaded++
		case ImportUpsert:
			s.items[id] = si.Item
			if si.Vector != nil {
				cp := make([]float32, len(si.Vector))
				copy(cp, si.Vector)
				s.vectors[id] = cp
			} else {
				delete(s.vectors, id)
			}
			if exists {
				rpt.Replaced++
			} else {
				rpt.Loaded++
			}
		case ImportReplace:
			s.items[id] = si.Item
			if si.Vector != nil {
				cp := make([]float32, len(si.Vector))
				copy(cp, si.Vector)
				s.vectors[id] = cp
			}
			rpt.Loaded++
		default:
			return rpt, fmt.Errorf("memory: unknown import mode %q", mode)
		}
	}
	return rpt, nil
}
