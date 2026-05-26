// Package main provides a migration tool from SQLite to PostgreSQL.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/lantos1618/arxiv.gg"
)

func main() {
	sqliteDB := flag.String("sqlite", "/data/arxiv/index.db", "SQLite database path")
	postgresURL := flag.String("postgres", "", "PostgreSQL connection URL (or use DATABASE_URL env)")
	batchSize := flag.Int("batch", 500, "Batch size for inserts (keep under 3000 for PostgreSQL parameter limit)")
	flag.Parse()

	pgURL := *postgresURL
	if pgURL == "" {
		pgURL = os.Getenv("DATABASE_URL")
	}
	if pgURL == "" {
		log.Fatal("PostgreSQL URL required: use -postgres flag or DATABASE_URL env")
	}

	ctx := context.Background()

	// Open SQLite source
	log.Printf("Opening SQLite database: %s", *sqliteDB)
	srcDB, err := gorm.Open(sqlite.Open(*sqliteDB), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to open SQLite: %v", err)
	}

	// Open PostgreSQL destination
	log.Printf("Connecting to PostgreSQL...")
	dstDB, err := gorm.Open(postgres.Open(pgURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to open PostgreSQL: %v", err)
	}

	// Configure connection pool
	sqlDB, _ := dstDB.DB()
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)

	// Run migrations on PostgreSQL
	log.Println("Running schema migrations...")
	if err := dstDB.AutoMigrate(&arxiv.Paper{}, &arxiv.Citation{}, &arxiv.SyncState{}, &arxiv.DownloadQueueItem{}, &arxiv.Embedding{}); err != nil {
		log.Fatalf("Failed to migrate schema: %v", err)
	}

	// Initialize PostgreSQL-specific schema (tsvector, indexes)
	initPostgresSchema(dstDB)

	// Migrate papers
	log.Println("Migrating papers...")
	if err := migratePapers(ctx, srcDB, dstDB, *batchSize); err != nil {
		log.Fatalf("Failed to migrate papers: %v", err)
	}

	// Migrate citations
	log.Println("Migrating citations...")
	if err := migrateCitations(ctx, srcDB, dstDB, *batchSize); err != nil {
		log.Fatalf("Failed to migrate citations: %v", err)
	}

	// Migrate embeddings
	log.Println("Migrating embeddings...")
	if err := migrateEmbeddings(ctx, srcDB, dstDB, *batchSize); err != nil {
		log.Fatalf("Failed to migrate embeddings: %v", err)
	}

	// Migrate sync state
	log.Println("Migrating sync state...")
	if err := migrateSyncState(ctx, srcDB, dstDB); err != nil {
		log.Fatalf("Failed to migrate sync state: %v", err)
	}

	// Rebuild search index
	log.Println("Building full-text search index...")
	if err := rebuildSearchIndex(ctx, dstDB); err != nil {
		log.Fatalf("Failed to rebuild search index: %v", err)
	}

	log.Println("Migration complete!")
}

func initPostgresSchema(db *gorm.DB) {
	// Add indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_papers_src_downloaded ON papers(src_downloaded)",
		"CREATE INDEX IF NOT EXISTS idx_papers_pdf_downloaded ON papers(pdf_downloaded)",
		"CREATE INDEX IF NOT EXISTS idx_papers_fetched_at ON papers(fetched_at DESC NULLS LAST)",
		"CREATE INDEX IF NOT EXISTS idx_papers_src_fetched ON papers(src_downloaded, fetched_at DESC NULLS LAST)",
		"CREATE INDEX IF NOT EXISTS idx_citations_to_id ON citations(to_id)",
		"CREATE INDEX IF NOT EXISTS idx_citations_from_id ON citations(from_id)",
	}
	for _, idx := range indexes {
		db.Exec(idx)
	}

	// Add tsvector column
	db.Exec(`ALTER TABLE papers ADD COLUMN IF NOT EXISTS search_vector tsvector`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_papers_search ON papers USING GIN(search_vector)`)

	// Create trigger function
	db.Exec(`
		CREATE OR REPLACE FUNCTION papers_search_trigger() RETURNS trigger AS $$
		BEGIN
			NEW.search_vector :=
				setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
				setweight(to_tsvector('english', COALESCE(NEW.abstract, '')), 'B');
			RETURN NEW;
		END
		$$ LANGUAGE plpgsql;
	`)

	// Create trigger
	db.Exec(`
		DROP TRIGGER IF EXISTS papers_search_update ON papers;
		CREATE TRIGGER papers_search_update
			BEFORE INSERT OR UPDATE ON papers
			FOR EACH ROW EXECUTE FUNCTION papers_search_trigger();
	`)
}

func migratePapers(ctx context.Context, src, dst *gorm.DB, batchSize int) error {
	var total int64
	src.Model(&arxiv.Paper{}).Count(&total)

	var existing int64
	dst.Model(&arxiv.Paper{}).Count(&existing)
	log.Printf("Found %d papers in SQLite, %d already in PostgreSQL", total, existing)

	// Get IDs already in PostgreSQL
	var existingIDs []string
	dst.Model(&arxiv.Paper{}).Pluck("id", &existingIDs)
	existingSet := make(map[string]bool, len(existingIDs))
	for _, id := range existingIDs {
		existingSet[id] = true
	}

	var offset int
	for {
		var papers []arxiv.Paper
		if err := src.Offset(offset).Limit(batchSize).Find(&papers).Error; err != nil {
			return err
		}
		if len(papers) == 0 {
			break
		}

		// Filter out existing papers
		var newPapers []arxiv.Paper
		for _, p := range papers {
			if !existingSet[p.ID] {
				newPapers = append(newPapers, p)
				existingSet[p.ID] = true // Mark as will-be-inserted
			}
		}

		// Insert new papers in small batches (18 columns × 200 = 3600 params, safe)
		if len(newPapers) > 0 {
			if err := dst.WithContext(ctx).CreateInBatches(newPapers, 200).Error; err != nil {
				// On error, try smaller batches
				for _, p := range newPapers {
					dst.WithContext(ctx).Create(&p)
				}
			}
		}

		offset += len(papers)
		log.Printf("  Processed %d/%d papers (%.1f%%), inserted %d new", offset, total, float64(offset)/float64(total)*100, len(newPapers))

		if offset >= int(total) {
			break
		}
	}

	return nil
}

func migrateCitations(ctx context.Context, src, dst *gorm.DB, batchSize int) error {
	var total int64
	src.Model(&arxiv.Citation{}).Count(&total)
	log.Printf("Found %d citations to migrate", total)

	if total == 0 {
		return nil
	}

	var offset int
	for {
		var citations []arxiv.Citation
		if err := src.Offset(offset).Limit(batchSize).Find(&citations).Error; err != nil {
			return err
		}
		if len(citations) == 0 {
			break
		}

		if err := dst.WithContext(ctx).Create(&citations).Error; err != nil {
			for _, c := range citations {
				dst.WithContext(ctx).Create(&c)
			}
		}

		offset += len(citations)
		log.Printf("  Migrated %d/%d citations (%.1f%%)", offset, total, float64(offset)/float64(total)*100)

		if offset >= int(total) {
			break
		}
	}

	return nil
}

func migrateEmbeddings(ctx context.Context, src, dst *gorm.DB, batchSize int) error {
	var total int64
	src.Model(&arxiv.Embedding{}).Count(&total)
	log.Printf("Found %d embeddings to migrate", total)

	if total == 0 {
		return nil
	}

	var offset int
	for {
		var embeddings []arxiv.Embedding
		if err := src.Offset(offset).Limit(batchSize).Find(&embeddings).Error; err != nil {
			return err
		}
		if len(embeddings) == 0 {
			break
		}

		if err := dst.WithContext(ctx).Create(&embeddings).Error; err != nil {
			for _, e := range embeddings {
				dst.WithContext(ctx).Create(&e)
			}
		}

		offset += len(embeddings)
		log.Printf("  Migrated %d/%d embeddings (%.1f%%)", offset, total, float64(offset)/float64(total)*100)

		if offset >= int(total) {
			break
		}
	}

	return nil
}

func migrateSyncState(ctx context.Context, src, dst *gorm.DB) error {
	var states []arxiv.SyncState
	if err := src.Find(&states).Error; err != nil {
		return err
	}

	for _, s := range states {
		dst.WithContext(ctx).Create(&s)
	}

	log.Printf("  Migrated %d sync state entries", len(states))
	return nil
}

func rebuildSearchIndex(ctx context.Context, db *gorm.DB) error {
	start := time.Now()

	// Update all search vectors in batches
	result := db.WithContext(ctx).Exec(`
		UPDATE papers SET search_vector =
			setweight(to_tsvector('english', COALESCE(title, '')), 'A') ||
			setweight(to_tsvector('english', COALESCE(abstract, '')), 'B')
		WHERE search_vector IS NULL
	`)

	if result.Error != nil {
		return result.Error
	}

	log.Printf("  Updated %d search vectors in %v", result.RowsAffected, time.Since(start))
	return nil
}
