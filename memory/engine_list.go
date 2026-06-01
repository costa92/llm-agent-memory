package memory

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const localMetaKeyScope = "_scope"

type listCursor struct {
	AfterCreatedAt time.Time `json:"after_created_at"`
	AfterID        string    `json:"after_id"`
}

func encodeCursor(c listCursor) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeCursor(s string) (listCursor, error) {
	if s == "" {
		return listCursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return listCursor{}, fmt.Errorf("memory: bad cursor: %w", err)
	}
	var c listCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return listCursor{}, fmt.Errorf("memory: bad cursor: %w", err)
	}
	return c, nil
}

func matchesFilter(it MemoryItem, f ListFilter) bool {
	if !f.IncludeDisabled && IsDisabled(it) {
		return false
	}
	if f.PinnedOnly && !IsPinned(it) {
		return false
	}
	if f.Source != "" && GetSource(it) != f.Source {
		return false
	}
	if f.Category != "" && GetCategory(it) != f.Category {
		return false
	}
	if !f.Scope.IsZero() && !f.Scope.Matches(readScope(it)) {
		return false
	}
	if f.MinImportance > 0 && it.Importance < f.MinImportance {
		return false
	}
	if len(f.Tags) > 0 {
		tagSet := make(map[string]bool, len(it.Tags))
		for _, t := range it.Tags {
			tagSet[strings.ToLower(t)] = true
		}
		match := false
		for _, q := range f.Tags {
			if tagSet[strings.ToLower(q)] {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}

func listFromStore(s *scoredStore, filter ListFilter, pageSize int, cursor string) (ListPage, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	cur, err := decodeCursor(cursor)
	if err != nil {
		return ListPage{}, err
	}
	items, _ := s.snapshot()
	candidates := make([]MemoryItem, 0, len(items))
	for _, it := range items {
		if !matchesFilter(it, filter) {
			continue
		}
		candidates = append(candidates, it)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
		}
		return candidates[i].ID < candidates[j].ID
	})
	start := 0
	if !cur.AfterCreatedAt.IsZero() || cur.AfterID != "" {
		start = len(candidates)
		for i, it := range candidates {
			if it.CreatedAt.Before(cur.AfterCreatedAt) {
				start = i
				break
			}
			if it.CreatedAt.Equal(cur.AfterCreatedAt) && it.ID > cur.AfterID {
				start = i
				break
			}
		}
	}
	end := start + pageSize
	if end > len(candidates) {
		end = len(candidates)
	}
	page := ListPage{Items: candidates[start:end]}
	if end < len(candidates) {
		last := candidates[end-1]
		nc, err := encodeCursor(listCursor{AfterCreatedAt: last.CreatedAt, AfterID: last.ID})
		if err != nil {
			return ListPage{}, err
		}
		page.NextCursor = nc
	}
	return page, nil
}

func readScope(it MemoryItem) Scope {
	if it.Metadata == nil {
		return Scope{}
	}
	raw, ok := it.Metadata[localMetaKeyScope]
	if !ok {
		return Scope{}
	}
	if m, ok := raw.(map[string]string); ok {
		return Scope{User: m["user"], Project: m["project"], Session: m["session"]}
	}
	if mAny, ok := raw.(map[string]any); ok {
		return Scope{
			User:    stringOr(mAny["user"]),
			Project: stringOr(mAny["project"]),
			Session: stringOr(mAny["session"]),
		}
	}
	return Scope{}
}

func stringOr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
