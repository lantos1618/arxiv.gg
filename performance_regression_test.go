package arxiv

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Count actual SELECTs, including raw GORM queries, after fixture setup.
type performanceQueryLog struct {
	logger.Interface
	mu         sync.Mutex
	statements []string
}

func (l *performanceQueryLog) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "SELECT") {
		l.mu.Lock()
		l.statements = append(l.statements, sql)
		l.mu.Unlock()
	}
}

func (l *performanceQueryLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.statements)
}

func countPerformanceQueries(cache *Cache) *performanceQueryLog {
	log := &performanceQueryLog{Interface: logger.Default.LogMode(logger.Silent)}
	cache.db = cache.db.Session(&gorm.Session{Logger: log})
	return log
}

func TestCitationViewsBatchColdCacheQueries(t *testing.T) {
	for _, view := range []string{"graph", "list"} {
		t.Run(view, func(t *testing.T) {
			cache := openTestCache(t)
			ctx := context.Background()
			const centralID = "2601.00000"
			const unknownID = "1901.99999"
			papers := []Paper{{ID: centralID, Title: "Central", Authors: "Central Author", Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}
			edges := []Citation{{FromID: centralID, ToID: unknownID}}
			for i := 0; i < 100; i++ {
				refID, citingID := fmt.Sprintf("2201.%05d", i), fmt.Sprintf("2301.%05d", i)
				papers = append(papers,
					Paper{ID: refID, Title: "Reference " + refID, Authors: "Reference Author", Created: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)},
					Paper{ID: citingID, Title: "Citing " + citingID, Authors: "Citing Author", Created: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)},
				)
				edges = append(edges, Citation{FromID: centralID, ToID: refID}, Citation{FromID: citingID, ToID: centralID})
			}
			edges = append(edges, Citation{FromID: "2201.00000", ToID: "2201.00001"})
			if err := cache.db.CreateInBatches(&papers, 50).Error; err != nil {
				t.Fatal(err)
			}
			if err := cache.db.Create(&edges).Error; err != nil {
				t.Fatal(err)
			}
			queries := countPerformanceQueries(cache)
			if view == "graph" {
				graph, err := cache.GetCitationGraph(ctx, centralID)
				if err != nil {
					t.Fatal(err)
				}
				if len(graph.Nodes) != 202 || len(graph.Edges) != 202 {
					t.Fatalf("graph lost nodes or edges: %d nodes, %d edges", len(graph.Nodes), len(graph.Edges))
				}
				nodes := make(map[string]GraphNode, len(graph.Nodes))
				for _, node := range graph.Nodes {
					nodes[node.ID] = node
				}
				if got := nodes[centralID]; got.Citations != 100 || got.Authors != "Central Author" {
					t.Fatalf("central metadata: %+v", got)
				}
				if got := nodes["2201.00001"]; got.Citations != 2 || got.Authors != "Reference Author" || got.Year != 2022 || !got.Cached {
					t.Fatalf("reference metadata: %+v", got)
				}
				if got := nodes[unknownID]; got.Title != unknownID || got.Year != 2019 || got.Citations != 1 || got.Cached {
					t.Fatalf("uncached reference: %+v", got)
				}
				if queries.count() > 6 {
					t.Fatalf("graph used %d SELECTs for 201 related papers; want at most 6", queries.count())
				}
			} else {
				items, err := cache.GetPaperList(ctx, centralID)
				if err != nil {
					t.Fatal(err)
				}
				if len(items) != 201 {
					t.Fatalf("list length = %d, want 201", len(items))
				}
				for i, item := range items {
					if i < 101 && (!item.IsRef || item.IsCiting) || i >= 101 && (!item.IsCiting || item.IsRef) {
						t.Fatalf("reference/citing ordering changed at %d: %+v", i, item)
					}
					if item.ID == "2201.00001" && (item.Citations != 2 || item.Authors != "Reference Author" || item.Year != 2022) {
						t.Fatalf("reference metadata: %+v", item)
					}
					if item.IsCiting && (item.Citations != 0 || item.Authors != "Citing Author" || item.Year != 2023) {
						t.Fatalf("citing metadata: %+v", item)
					}
				}
				if queries.count() > 4 {
					t.Fatalf("sidebar used %d SELECTs for 201 related papers; want at most 4", queries.count())
				}
			}
			before := queries.count()
			if count, err := cache.CitedByCount(ctx, "2301.00000"); err != nil || count != 0 || queries.count() != before {
				t.Fatalf("zero citation count was not cached: count=%d, err=%v", count, err)
			}
			if _, ok := cache.paperLRU.Get("2201.00001"); ok {
				t.Fatal("partial citation metadata populated the full paper cache")
			}
			t.Logf("%s: %d SELECTs for 201 related papers", view, before)
			if _, err := cache.GetPaperList(ctx, centralID); err != nil || queries.count() != before {
				t.Fatalf("warm sidebar repeated metadata/count queries: before=%d after=%d, err=%v", before, queries.count(), err)
			}
		})
	}
}

func TestCitationDetailsBatchBoundaryDeduplicatesAndHonorsCancellation(t *testing.T) {
	cache := openTestCache(t)
	ids := make([]string, 0, 1002)
	for i := 0; i < 501; i++ {
		id := fmt.Sprintf("2201.%05d", i)
		ids = append(ids, id, id)
	}
	queries := countPerformanceQueries(cache)
	details, err := cache.citationPaperDetails(context.Background(), ids)
	if err != nil || len(details) != 501 || queries.count() != 4 {
		t.Fatalf("batched deduplication: len=%d, queries=%d, err=%v", len(details), queries.count(), err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cache.citationPaperDetails(ctx, []string{"2401.00001"}); err == nil {
		t.Fatal("canceled metadata lookup returned success")
	}
}

func TestListCategoriesCachesClonesExpiresAndInvalidates(t *testing.T) {
	cache := openTestCache(t)
	ctx := context.Background()
	if err := cache.insertPapers(ctx, []Paper{{ID: "2601.00001", Categories: "cs.AI cs.LG"}}); err != nil {
		t.Fatal(err)
	}
	queries := countPerformanceQueries(cache)
	want := []CategoryCount{{Name: "cs.AI", Count: 1}, {Name: "cs.LG", Count: 1}}
	first, err := cache.ListCategories(ctx)
	if err != nil || !reflect.DeepEqual(first, want) {
		t.Fatalf("initial categories = %+v, err=%v", first, err)
	}
	first[0].Count = 999
	before := queries.count()
	second, err := cache.ListCategories(ctx)
	if err != nil || !reflect.DeepEqual(second, want) || queries.count() != before {
		t.Fatalf("cached result was mutated or re-queried: %+v, err=%v", second, err)
	}
	// Simulate a write outside this process, which the one-minute TTL must pick up.
	if err := cache.db.Create(&Paper{ID: "2601.00002", Categories: "cs.AI"}).Error; err != nil {
		t.Fatal(err)
	}
	cached, _ := cache.detailLRU.Get("category_counts")
	item := cached.(detailCacheItem)
	item.expires = time.Now().Add(-time.Second)
	cache.detailLRU.Put("category_counts", item)
	want[0].Count = 2
	third, err := cache.ListCategories(ctx)
	if err != nil || !reflect.DeepEqual(third, want) || queries.count() != before+1 {
		t.Fatalf("expired result failed to refresh: %+v, err=%v", third, err)
	}
	if err := cache.insertPapers(ctx, []Paper{{ID: "2601.00003", Categories: "cs.AI"}}); err != nil {
		t.Fatal(err)
	}
	want[0].Count = 3
	fourth, err := cache.ListCategories(ctx)
	if err != nil || !reflect.DeepEqual(fourth, want) {
		t.Fatalf("metadata write failed to invalidate cache: %+v, err=%v", fourth, err)
	}
}

func TestListCategoriesConcurrentRefreshIsCoalesced(t *testing.T) {
	cache := openTestCache(t)
	if err := cache.db.Create(&Paper{ID: "2601.00001", Categories: "cs.AI"}).Error; err != nil {
		t.Fatal(err)
	}
	queries := countPerformanceQueries(cache)
	const clients = 20
	start := make(chan struct{})
	errs := make(chan error, clients)
	var wg sync.WaitGroup
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			categories, err := cache.ListCategories(context.Background())
			if err != nil {
				errs <- err
			} else if len(categories) != 1 || categories[0].Count != 1 {
				errs <- fmt.Errorf("unexpected categories: %+v", categories)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if queries.count() != 1 {
		t.Fatalf("concurrent cold requests performed %d category SELECTs, want 1", queries.count())
	}
}
