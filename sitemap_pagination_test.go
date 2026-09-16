package arxiv

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestListSitemapPapersPreservesNumericPagination(t *testing.T) {
	cache := openTestCache(t)
	ctx := context.Background()
	updated := time.Date(2026, 9, 16, 12, 30, 0, 0, time.UTC)
	// Deliberately insert modern and legacy IDs out of lexical order. Large
	// metadata fields must not be loaded just to list sitemap URLs.
	ids := []string{"hep-th/9901001", "2501.00002", "0704.0001", "math/0301001", "astro-ph/0001234", "2201.00002", "2501.00001"}
	papers := make([]Paper, len(ids))
	for i, id := range ids {
		papers[i] = Paper{ID: id, Updated: updated, Title: "Not needed for sitemap", Abstract: "Not needed for sitemap", PDFText: "Not needed for sitemap"}
	}
	if err := cache.db.Create(&papers).Error; err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		offset int
		limit  int
		want   []string
	}{
		{"first", 0, 2, []string{"0704.0001", "2201.00002"}},
		{"middle", 2, 2, []string{"2501.00001", "2501.00002"}},
		{"legacy", 4, 2, []string{"astro-ph/0001234", "hep-th/9901001"}},
		{"partial_last", 6, 2, []string{"math/0301001"}},
		{"exact_end", 7, 2, []string{}},
		{"past_end", 100, 2, []string{}},
		{"default_limit", 4, 0, []string{"astro-ph/0001234", "hep-th/9901001", "math/0301001"}},
		{"negative_limit_uses_default", 6, -1, []string{"math/0301001"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cache.ListSitemapPapers(ctx, tt.offset, tt.limit)
			if err != nil {
				t.Fatal(err)
			}
			gotIDs := make([]string, len(got))
			for i, paper := range got {
				gotIDs[i] = paper.ID
				if !paper.Updated.Equal(updated) {
					t.Errorf("updated date for %s = %s, want %s", paper.ID, paper.Updated, updated)
				}
				if paper.Title != "" || paper.Abstract != "" || paper.PDFText != "" {
					t.Errorf("unneeded metadata was loaded for %s", paper.ID)
				}
			}
			if !reflect.DeepEqual(gotIDs, tt.want) {
				t.Fatalf("IDs = %v, want %v", gotIDs, tt.want)
			}
		})
	}
}

func TestListSitemapPapersEmptyAndCanceled(t *testing.T) {
	cache := openTestCache(t)
	papers, err := cache.ListSitemapPapers(context.Background(), 0, 2)
	if err != nil || len(papers) != 0 {
		t.Fatalf("empty catalog = %v, err=%v", papers, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cache.ListSitemapPapers(ctx, 0, 2); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled query error = %v, want context.Canceled", err)
	}
}
