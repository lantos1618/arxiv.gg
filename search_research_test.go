package arxiv

import (
	"context"
	"testing"
	"time"
)

func TestResearchPostgresFiltersAndExactRanking(t *testing.T) {
	c := quickSearchPostgresTestCache(t)
	for _, sql := range []string{`ALTER TABLE papers ADD COLUMN comments text, ADD COLUMN doi text`, `SET enable_seqscan=on`} {
		if err := c.db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct{ id, title, category, date string }{
		{"2601.00001", "Ocean whitecap foam rendering", "cs.GR", "2026-01-01"},
		{"2601.00002", "Ocean ocean ocean rendering rendering", "cs.CV", "2026-01-01"},
		{"2401.00003", "Ocean whitecap foam rendering", "cs.GR", "2024-01-01"},
	} {
		if err := c.db.Exec(`INSERT INTO papers(id,title,categories,created,updated)VALUES(?,?,?,?,?)`, row.id, row.title, row.category, row.date, row.date).Error; err != nil {
			t.Fatal(err)
		}
	}
	f := ResearchFilter{Category: "cs.GR", Since: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
	papers, err := c.ResearchKeywords(context.Background(), "ocean whitecap foam rendering", f, 1)
	if err != nil || len(papers) != 1 || papers[0].ID != "2601.00001" {
		t.Fatalf("filter before LIMIT: %+v %v", papers, err)
	}
	papers, err = c.ResearchKeywords(context.Background(), "ocean unknownconcept", f, 20)
	if err != nil || len(papers) != 1 {
		t.Fatalf("concept terms should OR-match: %+v %v", papers, err)
	}
	papers, err = c.ResearchKeywords(context.Background(), "2601.00001", f, 1)
	if err != nil || len(papers) != 1 || papers[0].ID != "2601.00001" {
		t.Fatalf("exact ID: %+v %v", papers, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.ResearchKeywords(ctx, "ocean", f, 20); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestResearchPostgresVectorFiltersAndDeepSnippets(t *testing.T) {
	c := quickSearchPostgresTestCache(t)
	for _, sql := range []string{
		`CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public`,
		`SET search_path=pg_temp,public`,
		`SET enable_seqscan=on`,
		`ALTER TABLE papers ADD COLUMN comments text, ADD COLUMN doi text`,
		`CREATE TEMP TABLE embeddings_v2(paper_id text,scope text,model text,dim int,vector vector(1024))`,
		`CREATE TEMP TABLE paper_chunks(id text,paper_id text,scope text,section text,text text,text_hash text)`,
		`CREATE TEMP TABLE chunk_embeddings_v2(chunk_id text,model text,dim int,vector vector(1024),source_hash text)`,
	} {
		if err := c.db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	vector := make([]float32, QwenEmbeddingDim)
	vector[0] = 1
	other := make([]float32, QwenEmbeddingDim)
	other[1] = 1
	for _, row := range []struct {
		id, category, date string
		v                  []float32
	}{
		{"2601.00001", "cs.GR", "2026-01-01", vector},
		{"2601.00002", "cs.CV", "2026-01-01", vector},
		{"2401.00003", "cs.GR", "2024-01-01", vector},
		{"2601.00004", "cs.GR", "2026-01-01", other},
	} {
		if err := c.db.Exec(`INSERT INTO papers(id,title,categories,created)VALUES(?,?,?,?)`, row.id, "Ocean", row.category, row.date).Error; err != nil {
			t.Fatal(err)
		}
		if err := c.db.Exec(`INSERT INTO embeddings_v2 VALUES(?,'abstract',?,?,?::vector)`, row.id, QwenEmbeddingModel, QwenEmbeddingDim, float32SliceToVectorString(row.v)).Error; err != nil {
			t.Fatal(err)
		}
	}
	filter := ResearchFilter{Category: "cs.GR", Since: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), MinSimilarity: .5}
	rows, err := c.ResearchVectors(context.Background(), vector, filter, 1, false)
	if err != nil || len(rows) != 1 || rows[0].PaperID != "2601.00001" || rows[0].Paper == nil {
		t.Fatalf("vector filter: %+v %v", rows, err)
	}
	if err := c.db.Exec(`INSERT INTO paper_chunks VALUES('fresh','2601.00001','pdf_text','Results','Latency 42 ms; bitrate 12 Mbps.','new'),('stale','2601.00002','pdf_text','Results','Stale text','new')`).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id, hash string }{{"fresh", "new"}, {"stale", "old"}} {
		if err := c.db.Exec(`INSERT INTO chunk_embeddings_v2 VALUES(?,?,?,?::vector,?)`, row.id, QwenEmbeddingModel, QwenEmbeddingDim, float32SliceToVectorString(vector), row.hash).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows, err = c.ResearchVectors(context.Background(), vector, ResearchFilter{}, 10, true)
	if err != nil || len(rows) != 1 || rows[0].ChunkID != "fresh" || rows[0].Snippet != "Latency 42 ms; bitrate 12 Mbps." {
		t.Fatalf("deep freshness and snippet: %+v %v", rows, err)
	}
}
