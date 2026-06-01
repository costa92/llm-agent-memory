package memory

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

const (
	internalScoreBenchmarkItems      = 10000
	internalScoreBenchmarkGoroutines = 16
	internalScoreBenchmarkTopK       = 8
)

// BenchmarkInternalScore_ConcurrentReadWrite_10k establishes the M8c
// baseline for the current scoredStore-backed memories before the package
// cluster lift and concurrency refactor. Workload mix is fixed at 80% read
// (40% Search, 40% Get) and 20% write (Update), with a 10k-item corpus and
// 16 concurrent goroutines.
func BenchmarkInternalScore_ConcurrentReadWrite_10k(b *testing.B) {
	ctx := context.Background()
	mem, err := NewWorking(newEmbedder(), WorkingOptions{Capacity: internalScoreBenchmarkItems + 1024})
	if err != nil {
		b.Fatalf("memory.NewWorking: %v", err)
	}

	ids := seedInternalScoreBenchmarkCorpus(b, ctx, mem, internalScoreBenchmarkItems)
	queries := benchmarkQueries()

	b.ReportAllocs()
	b.ReportMetric(internalScoreBenchmarkItems, "items")
	b.ReportMetric(internalScoreBenchmarkGoroutines, "goroutines")
	b.ReportMetric(80, "read_pct")
	b.ReportMetric(20, "write_pct")
	b.ResetTimer()

	opsPerWorker := b.N / internalScoreBenchmarkGoroutines
	remainder := b.N % internalScoreBenchmarkGoroutines

	var (
		wg       sync.WaitGroup
		errMu    sync.Mutex
		firstErr error
	)

	for worker := 0; worker < internalScoreBenchmarkGoroutines; worker++ {
		iterations := opsPerWorker
		if worker < remainder {
			iterations++
		}
		workerID := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				op := workerID + i*internalScoreBenchmarkGoroutines
				id := ids[op%len(ids)]

				var err error
				switch op % 5 {
				case 0:
					err = mem.Update(ctx, id, func(item *MemoryItem) {
						item.Content = fmt.Sprintf("updated benchmark note %d", op%2048)
						item.Importance = float64((op%10)+1) / 10
						item.Tags = []string{
							fmt.Sprintf("topic-%d", op%32),
							fmt.Sprintf("segment-%d", op%8),
						}
					})
				case 1, 3:
					_, err = mem.Search(ctx, queries[op%len(queries)], internalScoreBenchmarkTopK)
				default:
					_, err = mem.Get(ctx, id)
				}
				if err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
					return
				}
			}
		}()
	}
	wg.Wait()

	if firstErr != nil {
		b.Fatalf("benchmark workload failed: %v", firstErr)
	}
}

func seedInternalScoreBenchmarkCorpus(b *testing.B, ctx context.Context, mem *WorkingMemory, total int) []string {
	b.Helper()

	ids := make([]string, 0, total)
	for i := 0; i < total; i++ {
		id, err := mem.Add(ctx, MemoryItem{
			Content:    fmt.Sprintf("benchmark note %d topic-%d segment-%d", i, i%32, i%8),
			Tags:       []string{fmt.Sprintf("topic-%d", i%32), fmt.Sprintf("segment-%d", i%8)},
			Importance: float64((i%10)+1) / 10,
		})
		if err != nil {
			b.Fatalf("seed Add(%d): %v", i, err)
		}
		ids = append(ids, id)
	}
	return ids
}

func benchmarkQueries() []string {
	return []string{
		"topic-0 segment-0 benchmark",
		"topic-7 segment-3 preference",
		"topic-12 segment-4 recent note",
		"topic-19 segment-7 project context",
		"topic-24 segment-1 user memory",
		"topic-31 segment-6 summary",
	}
}
