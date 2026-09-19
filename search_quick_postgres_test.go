package arxiv

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// These tests require an explicitly supplied disposable PostgreSQL database.
// Every fixture lives in a connection-local temporary table; Open is deliberately
// avoided because it runs application migrations against the configured database.
func quickSearchPostgresTestCache(t *testing.T) *Cache {
	t.Helper()
	dsn := os.Getenv("ARXIV_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set ARXIV_TEST_POSTGRES_DSN to run PostgreSQL quick-search regression tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("connect to test PostgreSQL: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// Retain the same session, so unqualified papers references always resolve to
	// our temporary table and never to a table in the connection's normal schema.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, statement := range []string{
		`SET search_path = pg_temp`,
		`CREATE TEMP TABLE papers (
			id text PRIMARY KEY,
			created timestamptz,
			updated timestamptz,
			title text NOT NULL,
			abstract text NOT NULL DEFAULT '',
			authors text NOT NULL DEFAULT '',
			categories text NOT NULL DEFAULT '',
			pdf_downloaded boolean NOT NULL DEFAULT false,
			src_downloaded boolean NOT NULL DEFAULT false,
			fetched_at timestamptz,
			search_vector tsvector GENERATED ALWAYS AS (
				setweight(to_tsvector('english', title), 'A') ||
				setweight(to_tsvector('english', abstract), 'B')
			) STORED
		)`,
		// Write directly to the GIN posting lists; a pending-list-only fixture
		// would not exercise gin_fuzzy_search_limit's random reduction.
		`CREATE INDEX ON papers USING gin(search_vector) WITH (fastupdate = off)`,
		`SET enable_seqscan = off`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("prepare temporary quick-search fixture: %v", err)
		}
	}
	return &Cache{db: db, dbType: DBTypePostgres}
}

func addQuickSearchPostgresPaper(t *testing.T, cache *Cache, id, title, abstract, authors string) {
	t.Helper()
	if err := cache.db.Exec(`INSERT INTO papers
		(id, title, abstract, authors, created, updated)
		VALUES (?, ?, ?, ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
		id, title, abstract, authors).Error; err != nil {
		t.Fatalf("insert test paper %s: %v", id, err)
	}
}

func quickSearchPaperIDs(papers []Paper) []string {
	ids := make([]string, len(papers))
	for i := range papers {
		ids[i] = papers[i].ID
	}
	return ids
}

func TestQuickSearchPostgresExactTitleAndCompatibleLimits(t *testing.T) {
	cache := quickSearchPostgresTestCache(t)
	addQuickSearchPostgresPaper(t, cache, "2601.00001", "  Canonical\n\tQuantum   Search  ", "", "")
	for i := 2; i <= 20; i++ {
		// Repetition gives these nonexact titles a stronger ts_rank than the exact title.
		addQuickSearchPostgresPaper(t, cache, fmt.Sprintf("2601.%05d", i),
			"Canonical Quantum Search Canonical Quantum Search Extensions", "", "")
	}
	// The old capitalized-name heuristic would choose this author match first.
	addQuickSearchPostgresPaper(t, cache, "2601.99999", "Unrelated discovery", "", "Canonical Quantum Search")
	var baseline []string
	for _, query := range []string{"canonical quantum search", "Canonical Quantum Search", "  CANONICAL\tQUANTUM\nSEARCH  "} {
		for repeat := 0; repeat < 3; repeat++ {
			for _, limit := range []int{100, 1, 8} {
				papers, total, err := cache.QuickSearch(context.Background(), query, limit)
				if err != nil {
					t.Fatalf("QuickSearch(%q, %d): %v", query, limit, err)
				}
				if len(papers) == 0 || papers[0].ID != "2601.00001" {
					t.Fatalf("QuickSearch(%q, %d) exact title must rank first: %v", query, limit, quickSearchPaperIDs(papers))
				}
				if total != 20 {
					t.Fatalf("QuickSearch(%q, %d) total = %d, want 20", query, limit, total)
				}
				ids := quickSearchPaperIDs(papers)
				if baseline == nil {
					baseline = ids
				}
				wantLen := limit
				if wantLen > len(baseline) {
					wantLen = len(baseline)
				}
				if !reflect.DeepEqual(ids, baseline[:wantLen]) {
					t.Fatalf("QuickSearch(%q, %d) unstable prefix: got %v, want %v", query, limit, ids, baseline[:wantLen])
				}
			}
		}
	}
}

func TestQuickSearchPostgresStableTies(t *testing.T) {
	cache := quickSearchPostgresTestCache(t)
	// Reverse insertion order makes the final ID ordering observable.
	for i := 12; i >= 1; i-- {
		addQuickSearchPostgresPaper(t, cache, fmt.Sprintf("2602.%05d", i), "Tiekey", "", "")
	}
	for repeat := 0; repeat < 3; repeat++ {
		papers, total, err := cache.QuickSearch(context.Background(), "tiekey", 8)
		if err != nil || total != 12 || len(papers) != 8 {
			t.Fatalf("QuickSearch ties: count=%d total=%d err=%v", len(papers), total, err)
		}
		for i, paper := range papers {
			if want := fmt.Sprintf("2602.%05d", i+1); paper.ID != want {
				t.Fatalf("tie result %d = %s, want %s", i, paper.ID, want)
			}
		}
	}
}

func TestQuickSearchPostgresExactIDBeforePrefix(t *testing.T) {
	cache := quickSearchPostgresTestCache(t)
	addQuickSearchPostgresPaper(t, cache, "0704.0001", "Exact ID", "", "")
	addQuickSearchPostgresPaper(t, cache, "0704.00010", "Longer prefix", "", "")
	for _, limit := range []int{1, 8, 100} {
		papers, total, err := cache.QuickSearch(context.Background(), "  0704.0001  ", limit)
		if err != nil || total != 2 || len(papers) == 0 || papers[0].ID != "0704.0001" {
			t.Fatalf("exact ID with limit %d: papers=%v total=%d err=%v", limit, quickSearchPaperIDs(papers), total, err)
		}
	}
}

func TestQuickSearchPostgresAuthorAndStopwordFallback(t *testing.T) {
	cache := quickSearchPostgresTestCache(t)
	addQuickSearchPostgresPaper(t, cache, "2603.00001", "Unique discovery", "", "Ada Lovelace")
	addQuickSearchPostgresPaper(t, cache, "2603.00002", "Another discovery", "", "Lovelace, Ada")
	addQuickSearchPostgresPaper(t, cache, "2603.00003", "  The\tAnd\nOf  ", "", "")
	addQuickSearchPostgresPaper(t, cache, "2603.00004", "the and of example", "", "")
	addQuickSearchPostgresPaper(t, cache, "2603.00005", "Unrelated finding", "", "The And Of")
	papers, total, err := cache.QuickSearch(context.Background(), "Ada Lovelace", 8)
	if err != nil || total != 2 || len(papers) != 2 {
		t.Fatalf("author fallback: papers=%v total=%d err=%v", quickSearchPaperIDs(papers), total, err)
	}
	for _, query := range []string{"the and of", "The And Of"} {
		papers, total, err = cache.QuickSearch(context.Background(), query, 1)
		if err != nil || total != 3 || len(papers) != 1 || papers[0].ID != "2603.00003" {
			t.Fatalf("stopword fallback %q: papers=%v total=%d err=%v", query, quickSearchPaperIDs(papers), total, err)
		}
	}
}

func TestQuickSearchPostgresFallbackCompatibleLimits(t *testing.T) {
	cache := quickSearchPostgresTestCache(t)
	for i := 12; i >= 1; i-- {
		addQuickSearchPostgresPaper(t, cache, fmt.Sprintf("2607.%05d", i), "The And Of", "", "")
	}
	for _, limit := range []int{1, 8, 100} {
		papers, total, err := cache.QuickSearch(context.Background(), "the and of", limit)
		if err != nil || total != 12 {
			t.Fatalf("fallback search with limit %d: total=%d err=%v", limit, total, err)
		}
		wantLen := limit
		if wantLen > 12 {
			wantLen = 12
		}
		if len(papers) != wantLen {
			t.Fatalf("fallback search with limit %d: got %d rows, want %d", limit, len(papers), wantLen)
		}
		for i, paper := range papers {
			if want := fmt.Sprintf("2607.%05d", i+1); paper.ID != want {
				t.Fatalf("fallback search with limit %d: result %d = %s, want %s", limit, i, paper.ID, want)
			}
		}
	}
}

func TestQuickSearchPostgresAuthorCompatibleLimits(t *testing.T) {
	cache := quickSearchPostgresTestCache(t)
	for i := 12; i >= 1; i-- {
		addQuickSearchPostgresPaper(t, cache, fmt.Sprintf("2605.%05d", i), "Unrelated discovery", "", "Ada Lovelace")
	}
	for _, limit := range []int{1, 8, 100} {
		papers, total, err := cache.QuickSearch(context.Background(), "Ada Lovelace", limit)
		if err != nil || total != 12 {
			t.Fatalf("author search with limit %d: total=%d err=%v", limit, total, err)
		}
		wantLen := limit
		if wantLen > 12 {
			wantLen = 12
		}
		if len(papers) != wantLen {
			t.Fatalf("author search with limit %d: got %d rows, want %d", limit, len(papers), wantLen)
		}
		for i, paper := range papers {
			if want := fmt.Sprintf("2605.%05d", i+1); paper.ID != want {
				t.Fatalf("author search with limit %d: result %d = %s, want %s", limit, i, paper.ID, want)
			}
		}
	}
}

func TestQuickSearchPostgresDoesNotSampleGINResults(t *testing.T) {
	cache := quickSearchPostgresTestCache(t)
	const count = 12001
	if err := cache.db.Exec(`INSERT INTO papers (id, title, created, updated)
		SELECT '2604.' || lpad(n::text, 5, '0'),
			CASE WHEN n = 1 THEN 'Common Signal' ELSE 'Common Signal Common Signal Extensions' END,
			'2026-01-01T00:00:00Z'::timestamptz, '2026-01-01T00:00:00Z'::timestamptz
		FROM generate_series(1, ?) AS n`, count).Error; err != nil {
		t.Fatalf("seed GIN regression fixture: %v", err)
	}
	if err := cache.db.Exec("ANALYZE papers").Error; err != nil {
		t.Fatal(err)
	}
	var baseline []string
	for repeat := 0; repeat < 3; repeat++ {
		for _, limit := range []int{100, 1, 8} {
			papers, total, err := cache.QuickSearch(context.Background(), "common signal", limit)
			if err != nil {
				t.Fatalf("QuickSearch broad query: %v", err)
			}
			if total != count {
				t.Fatalf("sampled/inexact total = %d, want %d", total, count)
			}
			if len(papers) != limit || papers[0].ID != "2604.00001" {
				t.Fatalf("broad-query exact title missing with limit %d: %v", limit, quickSearchPaperIDs(papers))
			}
			ids := quickSearchPaperIDs(papers)
			if baseline == nil {
				baseline = ids
			}
			if !reflect.DeepEqual(ids, baseline[:limit]) {
				t.Fatalf("GIN result prefix changed: got %v, want %v", ids, baseline[:limit])
			}
		}
	}
}

func TestQuickSearchPostgresCanceledContextReturnsError(t *testing.T) {
	cache := quickSearchPostgresTestCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	_, _, err := cache.QuickSearch(ctx, "canceled query", 8)
	if err == nil {
		t.Fatal("QuickSearch with canceled context unexpectedly succeeded")
	}
	if time.Since(started) > time.Second {
		t.Fatal("canceled QuickSearch did not return promptly")
	}
}

func TestQuickSearchPostgresDoesNotHideFullTextErrors(t *testing.T) {
	cache := quickSearchPostgresTestCache(t)
	addQuickSearchPostgresPaper(t, cache, "2606.00001", "Fallback masking", "", "")
	if err := cache.db.Exec("ALTER TABLE papers RENAME COLUMN search_vector TO unavailable_vector").Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := cache.QuickSearch(context.Background(), "fallback masking", 8); err == nil {
		t.Fatal("QuickSearch hid a fulltext SQL error behind a successful broad fallback")
	}
}
