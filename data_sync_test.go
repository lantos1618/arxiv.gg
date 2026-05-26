package arxiv

import (
	"context"
	"testing"
	"time"
)

func TestInsertPapersPreservesLocalDownloadFields(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	cache, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer cache.Close()

	oldFetchedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	original := Paper{
		ID:               "2605.14604",
		Created:          time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC),
		Updated:          time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC),
		Title:            "Original title",
		Abstract:         "Original abstract",
		Authors:          "A. Author",
		Categories:       "cs.AI",
		PDFPath:          "/data/pdf/2605.14604.pdf",
		SourcePath:       "/data/src/2605.14604.tar.gz",
		PDFText:          "extracted full text",
		PDFDownloaded:    true,
		SourceDownloaded: true,
		FetchedAt:        &oldFetchedAt,
	}
	if err := cache.db.Create(&original).Error; err != nil {
		t.Fatalf("create original paper: %v", err)
	}

	incoming := []Paper{{
		ID:         "2605.14604",
		Created:    time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC),
		Updated:    time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC),
		Title:      "Updated metadata title",
		Abstract:   "Updated metadata abstract",
		Authors:    "A. Author, B. Author",
		Categories: "cs.AI cs.CL",
		Comments:   "12 pages",
		DOI:        "10.0000/example",
	}}
	if err := cache.insertPapers(context.Background(), incoming); err != nil {
		t.Fatalf("insert papers: %v", err)
	}

	var got Paper
	if err := cache.db.Where("id = ?", "2605.14604").First(&got).Error; err != nil {
		t.Fatalf("load paper: %v", err)
	}

	if got.Title != "Updated metadata title" || got.Abstract != "Updated metadata abstract" {
		t.Fatalf("metadata was not updated: title=%q abstract=%q", got.Title, got.Abstract)
	}
	if got.Authors != "A. Author, B. Author" || got.Categories != "cs.AI cs.CL" {
		t.Fatalf("metadata fields were not updated: authors=%q categories=%q", got.Authors, got.Categories)
	}
	if got.PDFPath != original.PDFPath || got.SourcePath != original.SourcePath || got.PDFText != original.PDFText {
		t.Fatalf("local paths/text were overwritten: pdf=%q src=%q text=%q", got.PDFPath, got.SourcePath, got.PDFText)
	}
	if !got.PDFDownloaded || !got.SourceDownloaded {
		t.Fatalf("download flags were overwritten: pdf=%v src=%v", got.PDFDownloaded, got.SourceDownloaded)
	}
	if got.FetchedAt == nil || !got.FetchedAt.Equal(oldFetchedAt) {
		t.Fatalf("fetched_at was overwritten: got=%v want=%v", got.FetchedAt, oldFetchedAt)
	}
	if got.MetadataUpdated == nil {
		t.Fatal("metadata_updated was not set")
	}
}
