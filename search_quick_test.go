package arxiv

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestQuickSearchSQLiteKnownPaperPrecedesNewerPartialTitles(t *testing.T) {
	cache := openTestCache(t)
	exact := Paper{ID: "1706.03762", Title: "Attention Is All You Need", Created: time.Date(2017, 6, 12, 0, 0, 0, 0, time.UTC)}
	papers := []Paper{exact, {ID: "1706.037620", Title: "Longer ID", Created: exact.Created.Add(time.Hour)}}
	for i := 0; i < 20; i++ {
		papers = append(papers, Paper{ID: fmt.Sprintf("2601.%05d", i), Title: "Attention Is All You Need for another task", Created: exact.Created.Add(24 * time.Hour)})
	}
	if err := cache.db.Create(&papers).Error; err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"attention is all you need", "  ATTENTION\tIS ALL\nYOU NEED  "} {
		var prefix []string
		for _, limit := range []int{1, 8, 100} {
			got, total, err := cache.QuickSearch(t.Context(), query, limit)
			if err != nil || total != 21 || len(got) == 0 || got[0].ID != exact.ID {
				t.Fatalf("query=%q limit=%d: total=%d papers=%v err=%v", query, limit, total, got, err)
			}
			ids := make([]string, len(got))
			for i := range got {
				ids[i] = got[i].ID
			}
			if len(prefix) > 0 && !reflect.DeepEqual(prefix, ids[:len(prefix)]) {
				t.Fatalf("limit changed prefix: %v versus %v", prefix, ids)
			}
			prefix = ids
		}
	}
	got, total, err := cache.QuickSearch(t.Context(), "1706.03762", 1)
	if err != nil || total != 2 || len(got) != 1 || got[0].ID != exact.ID {
		t.Fatalf("exact ID: papers=%v total=%d err=%v", got, total, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := cache.QuickSearch(ctx, "attention", 10); err == nil {
		t.Fatal("canceled query returned success")
	}
}

func TestNormalizeQuickSearchID(t *testing.T) {
	for _, tc := range []struct{ query, want string }{
		{"1706.03762v7", "1706.03762"},
		{"1706.03762V7", "1706.03762"},
		{"hep-th/9901001v2", "hep-th/9901001"},
		{"https://arxiv.org/abs/1706.03762v7?context=cs#top", "1706.03762"},
		{"http://www.arxiv.org/pdf/1706.03762v7.pdf", "1706.03762"},
		{"export.arxiv.org/pdf/hep-th/9901001v2.PDF", "hep-th/9901001"},
		{"https://ARXIV.ORG/abs/1706.03762V7", "1706.03762"},
		{"1706.037", "1706.037"},
		{"1706.03762v", "1706.03762v"},
		{"Model v2", "Model v2"},
		{"https://example.com/abs/1706.03762v7", "https://example.com/abs/1706.03762v7"},
		{"https://arxiv.org.evil/abs/1706.03762v7", "https://arxiv.org.evil/abs/1706.03762v7"},
		{"https://user@arxiv.org/abs/1706.03762v7", "https://user@arxiv.org/abs/1706.03762v7"},
		{"https://arxiv.org:444/abs/1706.03762v7", "https://arxiv.org:444/abs/1706.03762v7"},
		{"https://arxiv.org/abs/1706.03762v7/extra", "https://arxiv.org/abs/1706.03762v7/extra"},
	} {
		if got := normalizeQuickSearchID(tc.query); got != tc.want {
			t.Errorf("normalizeQuickSearchID(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}
}

func TestQuickSearchSQLiteVersionedID(t *testing.T) {
	cache := openTestCache(t)
	paper := Paper{ID: "1706.03762", Title: "Attention Is All You Need"}
	if err := cache.db.Create(&paper).Error; err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"1706.03762v7", "1706.03762V7", "https://arxiv.org/pdf/1706.03762v7.pdf"} {
		papers, total, err := cache.QuickSearch(t.Context(), query, 5)
		if err != nil || total != 1 || len(papers) != 1 || papers[0].ID != paper.ID {
			t.Fatalf("versioned ID %q: papers=%v total=%d err=%v", query, papers, total, err)
		}
	}
}
