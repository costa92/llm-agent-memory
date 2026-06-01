package memory

import (
	"context"
	"time"

	coremem "github.com/costa92/llm-agent/memory"
)

type Kind = coremem.Kind

const (
	KindWorking  = coremem.KindWorking
	KindEpisodic = coremem.KindEpisodic
	KindSemantic = coremem.KindSemantic

	CategoryUser      = coremem.CategoryUser
	CategoryFeedback  = coremem.CategoryFeedback
	CategoryProject   = coremem.CategoryProject
	CategoryReference = coremem.CategoryReference
)

type MemoryItem struct {
	ID         string
	Content    string
	Tags       []string
	Importance float64
	CreatedAt  time.Time
	AccessedAt time.Time
	Metadata   map[string]any
}

type SearchResult struct {
	Item  MemoryItem
	Score float64
}

type Stats = coremem.Stats
type Embedder = coremem.Embedder
type ListFilter = coremem.ListFilter
type Scope = coremem.Scope
type Source = coremem.Source
type Category = coremem.Category
type ForgetStrategy = coremem.ForgetStrategy
type ConsolidateOptions = coremem.ConsolidateOptions
type ForgetOptions = coremem.ForgetOptions

type ListPage struct {
	Items      []MemoryItem
	NextCursor string
}

type Memory any

type Lister interface {
	List(ctx context.Context, filter ListFilter, pageSize int, cursor string) (ListPage, error)
}

type Snapshot struct {
	Version int            `json:"version"`
	Kind    Kind           `json:"kind"`
	Items   []SnapshotItem `json:"items"`
}

type SnapshotItem struct {
	Item   MemoryItem `json:"item"`
	Vector []float32  `json:"vector"`
}

type ImportMode string

type ImportReport struct {
	Loaded   int     `json:"loaded"`
	Skipped  int     `json:"skipped"`
	Replaced int     `json:"replaced"`
	Errors   []error `json:"-"`
}

type Exporter interface {
	Export(ctx context.Context) (Snapshot, error)
}

type Importer interface {
	Import(ctx context.Context, snap Snapshot, mode ImportMode) (ImportReport, error)
}

type SnapshotStore interface {
	Save(ctx context.Context, key string, snap Snapshot) error
	Load(ctx context.Context, key string) (Snapshot, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context) ([]string, error)
}

const (
	SnapshotVersion = coremem.SnapshotVersion

	ImportReplace ImportMode = "replace"
	ImportMerge   ImportMode = "merge"
	ImportUpsert  ImportMode = "upsert"

	ForgetByImportance = coremem.ForgetByImportance
	ForgetByAge        = coremem.ForgetByAge
	ForgetByCapacity   = coremem.ForgetByCapacity
)

var (
	ErrSnapshotVersionMismatch    = coremem.ErrSnapshotVersionMismatch
	ErrSnapshotKindMismatch       = coremem.ErrSnapshotKindMismatch
	ErrSnapshotStoreNotConfigured = coremem.ErrSnapshotStoreNotConfigured
)

func snapshotFromCore(snap coremem.Snapshot) Snapshot {
	items := make([]SnapshotItem, len(snap.Items))
	for i, item := range snap.Items {
		items[i] = SnapshotItem{
			Item:   memoryItemFromCore(item.Item),
			Vector: append([]float32(nil), item.Vector...),
		}
	}
	return Snapshot{
		Version: snap.Version,
		Kind:    snap.Kind,
		Items:   items,
	}
}

func snapshotToCore(snap Snapshot) coremem.Snapshot {
	items := make([]coremem.SnapshotItem, len(snap.Items))
	for i, item := range snap.Items {
		items[i] = coremem.SnapshotItem{
			Item:   memoryItemToCore(item.Item),
			Vector: append([]float32(nil), item.Vector...),
		}
	}
	return coremem.Snapshot{
		Version: snap.Version,
		Kind:    snap.Kind,
		Items:   items,
	}
}

func importReportFromCore(rpt coremem.ImportReport) ImportReport {
	return ImportReport{
		Loaded:   rpt.Loaded,
		Skipped:  rpt.Skipped,
		Replaced: rpt.Replaced,
		Errors:   append([]error(nil), rpt.Errors...),
	}
}

func memoryItemFromCore(item coremem.MemoryItem) MemoryItem {
	return MemoryItem{
		ID:         item.ID,
		Content:    item.Content,
		Tags:       append([]string(nil), item.Tags...),
		Importance: item.Importance,
		CreatedAt:  item.CreatedAt,
		AccessedAt: item.AccessedAt,
		Metadata:   cloneMetadata(item.Metadata),
	}
}

func memoryItemToCore(item MemoryItem) coremem.MemoryItem {
	return coremem.MemoryItem{
		ID:         item.ID,
		Content:    item.Content,
		Tags:       append([]string(nil), item.Tags...),
		Importance: item.Importance,
		CreatedAt:  item.CreatedAt,
		AccessedAt: item.AccessedAt,
		Metadata:   cloneMetadata(item.Metadata),
	}
}

func searchResultsFromCore(results []coremem.SearchResult) []SearchResult {
	out := make([]SearchResult, len(results))
	for i, result := range results {
		out[i] = SearchResult{
			Item:  memoryItemFromCore(result.Item),
			Score: result.Score,
		}
	}
	return out
}

func listPageFromCore(page coremem.ListPage) ListPage {
	items := make([]MemoryItem, len(page.Items))
	for i, item := range page.Items {
		items[i] = memoryItemFromCore(item)
	}
	return ListPage{
		Items:      items,
		NextCursor: page.NextCursor,
	}
}

func cloneMetadata(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
