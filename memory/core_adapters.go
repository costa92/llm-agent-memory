package memory

import (
	"context"

	coremem "github.com/costa92/llm-agent/memory"
)

type coreMemoryAdapter struct {
	inner coremem.Memory
}

type coreListerAdapter struct {
	inner coremem.Lister
}

type coreExporterAdapter struct {
	inner coremem.Exporter
}

type coreImporterAdapter struct {
	inner coremem.Importer
}

func AdaptCoreMemory(inner coremem.Memory) Memory {
	if inner == nil {
		return nil
	}
	return coreMemoryAdapter{inner: inner}
}

func AdaptCoreLister(inner coremem.Lister) Lister {
	if inner == nil {
		return nil
	}
	return coreListerAdapter{inner: inner}
}

func AdaptCoreExporter(inner coremem.Exporter) Exporter {
	if inner == nil {
		return nil
	}
	return coreExporterAdapter{inner: inner}
}

func AdaptCoreImporter(inner coremem.Importer) Importer {
	if inner == nil {
		return nil
	}
	return coreImporterAdapter{inner: inner}
}

func (a coreMemoryAdapter) Type() Kind { return a.inner.Type() }

func (a coreMemoryAdapter) Add(ctx context.Context, item MemoryItem) (string, error) {
	return a.inner.Add(ctx, memoryItemToCore(item))
}

func (a coreMemoryAdapter) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	results, err := a.inner.Search(ctx, query, topK)
	if err != nil {
		return nil, err
	}
	return searchResultsFromCore(results), nil
}

func (a coreMemoryAdapter) Get(ctx context.Context, id string) (MemoryItem, error) {
	item, err := a.inner.Get(ctx, id)
	if err != nil {
		return MemoryItem{}, err
	}
	return memoryItemFromCore(item), nil
}

func (a coreMemoryAdapter) Update(ctx context.Context, id string, fn func(*MemoryItem)) error {
	return a.inner.Update(ctx, id, func(item *coremem.MemoryItem) {
		local := memoryItemFromCore(*item)
		fn(&local)
		*item = memoryItemToCore(local)
	})
}

func (a coreMemoryAdapter) Remove(ctx context.Context, id string) error {
	return a.inner.Remove(ctx, id)
}

func (a coreMemoryAdapter) Stats() Stats { return a.inner.Stats() }

func (a coreListerAdapter) List(ctx context.Context, filter ListFilter, pageSize int, cursor string) (ListPage, error) {
	page, err := a.inner.List(ctx, filter, pageSize, cursor)
	if err != nil {
		return ListPage{}, err
	}
	return listPageFromCore(page), nil
}

func (a coreExporterAdapter) Export(ctx context.Context) (Snapshot, error) {
	snap, err := a.inner.Export(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	return snapshotFromCore(snap), nil
}

func (a coreImporterAdapter) Import(ctx context.Context, snap Snapshot, mode ImportMode) (ImportReport, error) {
	rpt, err := a.inner.Import(ctx, snapshotToCore(snap), coremem.ImportMode(mode))
	if err != nil {
		return ImportReport{}, err
	}
	return importReportFromCore(rpt), nil
}
