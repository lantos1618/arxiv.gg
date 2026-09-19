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
