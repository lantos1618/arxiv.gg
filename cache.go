package arxiv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func getEmbeddingScriptPath() string {
	for _, path := range []string{
		"/app/tools/generate_embeddings.py",
		"tools/generate_embeddings.py",
	} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return "/app/tools/generate_embeddings.py"
}

// DBType indicates which database backend is in use
type DBType string

const (
	DBTypeSQLite   DBType = "sqlite"
	DBTypePostgres DBType = "postgres"
)

// Cache manages a local offline cache of arXiv papers.
type Cache struct {
	root     string
	db       *gorm.DB
	dbType   DBType
	paperLRU *LRUCache
}

// DBType returns the database type in use.
func (c *Cache) DBType() DBType {
	return c.dbType
}

// Open opens or creates an arXiv cache at the given root directory.
// If DATABASE_URL env var is set, uses PostgreSQL; otherwise uses SQLite.
func Open(root string) (*Cache, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	// Create subdirectories for PDFs and source files
	for _, dir := range []string{"pdf", "src", "meta"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			return nil, fmt.Errorf("create %s dir: %w", dir, err)
		}
	}

	var db *gorm.DB
	var dbType DBType
	var err error

	// Check for PostgreSQL connection string
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		db, err = gorm.Open(postgres.Open(dbURL), &gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
		})
		if err != nil {
			return nil, fmt.Errorf("open postgres database: %w", err)
		}
		dbType = DBTypePostgres
		fmt.Println("Connected to PostgreSQL database")
	} else {
		// Fall back to SQLite
		dbPath := filepath.Join(root, "index.db")
		db, err = gorm.Open(sqlite.Open(dbPath+"?_pragma=foreign_keys(1)&_pragma=journal_mode=WAL&_pragma=synchronous=NORMAL"), &gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
		})
		if err != nil {
			return nil, fmt.Errorf("open sqlite database: %w", err)
		}
		dbType = DBTypeSQLite
		fmt.Println("Using SQLite database")
	}

	// Configure connection pool for PostgreSQL
	if dbType == DBTypePostgres {
		sqlDB, _ := db.DB()
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
	}

	// LRU cache size: with 15GB+ RAM, we can cache hundreds of thousands of papers
	lruSize := 500000
	c := &Cache{
		root:     root,
		db:       db,
		dbType:   dbType,
		paperLRU: NewLRUCache(lruSize),
	}
	if err := c.initSchema(); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return c, nil
}

// Close closes the cache database.
func (c *Cache) Close() error {
	sqlDB, err := c.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Root returns the cache root directory.
func (c *Cache) Root() string {
	return c.root
}

func (c *Cache) initSchema() error {
	// GORM AutoMigrate handles all regular tables
	if err := c.db.AutoMigrate(&Paper{}, &Citation{}, &SyncState{}, &DownloadQueueItem{}, &Embedding{}, &EmbeddingJob{}, &AuthorCollaboration{}, &AuthorEmbedding{}); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	if c.dbType == DBTypePostgres {
		return c.initPostgresSchema()
	}
	return c.initSQLiteSchema()
}

func (c *Cache) initSQLiteSchema() error {
	// Add indexes for common queries (idempotent - IF NOT EXISTS)
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_papers_src_downloaded ON papers(src_downloaded)",
		"CREATE INDEX IF NOT EXISTS idx_papers_pdf_downloaded ON papers(pdf_downloaded)",
		"CREATE INDEX IF NOT EXISTS idx_papers_fetched_at ON papers(fetched_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_papers_src_fetched ON papers(src_downloaded, fetched_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_citations_to_id ON citations(to_id)",
		"CREATE INDEX IF NOT EXISTS idx_citations_from_id ON citations(from_id)",
	}
	for _, idx := range indexes {
		c.db.Exec(idx)
	}

	// FTS5 virtual tables and triggers for SQLite
	ftsSchema := `
	CREATE VIRTUAL TABLE IF NOT EXISTS papers_fts USING fts5(
		title,
		abstract,
		content='papers',
		content_rowid='rowid'
	);

	CREATE TRIGGER IF NOT EXISTS papers_ai AFTER INSERT ON papers BEGIN
		INSERT INTO papers_fts(rowid, title, abstract)
		VALUES (NEW.rowid, NEW.title, NEW.abstract);
	END;

	CREATE TRIGGER IF NOT EXISTS papers_ad AFTER DELETE ON papers BEGIN
		INSERT INTO papers_fts(papers_fts, rowid, title, abstract)
		VALUES ('delete', OLD.rowid, OLD.title, OLD.abstract);
	END;

	CREATE TRIGGER IF NOT EXISTS papers_au AFTER UPDATE ON papers BEGIN
		INSERT INTO papers_fts(papers_fts, rowid, title, abstract)
		VALUES ('delete', OLD.rowid, OLD.title, OLD.abstract);
		INSERT INTO papers_fts(rowid, title, abstract)
		VALUES (NEW.rowid, NEW.title, NEW.abstract);
	END;
	`
	if err := c.db.Exec(ftsSchema).Error; err != nil {
		fmt.Printf("Warning: FTS5 not available (%v), full-text search will use fallback methods\n", err)
	}
	return nil
}

func (c *Cache) initPostgresSchema() error {
	// Add indexes for common queries
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_papers_src_downloaded ON papers(src_downloaded)",
		"CREATE INDEX IF NOT EXISTS idx_papers_pdf_downloaded ON papers(pdf_downloaded)",
		"CREATE INDEX IF NOT EXISTS idx_papers_fetched_at ON papers(fetched_at DESC NULLS LAST)",
		"CREATE INDEX IF NOT EXISTS idx_papers_src_fetched ON papers(src_downloaded, fetched_at DESC NULLS LAST)",
		"CREATE INDEX IF NOT EXISTS idx_citations_to_id ON citations(to_id)",
		"CREATE INDEX IF NOT EXISTS idx_citations_from_id ON citations(from_id)",
	}
	for _, idx := range indexes {
		c.db.Exec(idx)
	}

	// Add tsvector column for full-text search if it doesn't exist
	c.db.Exec(`
		DO $$ BEGIN
			ALTER TABLE papers ADD COLUMN IF NOT EXISTS search_vector tsvector;
		EXCEPTION WHEN duplicate_column THEN NULL;
		END $$;
	`)

	// Create GIN index on search_vector
	c.db.Exec(`CREATE INDEX IF NOT EXISTS idx_papers_search ON papers USING GIN(search_vector)`)

	// Create function to update search_vector
	c.db.Exec(`
		CREATE OR REPLACE FUNCTION papers_search_trigger() RETURNS trigger AS $$
		BEGIN
			NEW.search_vector :=
				setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
				setweight(to_tsvector('english', COALESCE(NEW.abstract, '')), 'B');
			RETURN NEW;
		END
		$$ LANGUAGE plpgsql;
	`)

	// Create trigger for automatic updates
	c.db.Exec(`
		DROP TRIGGER IF EXISTS papers_search_update ON papers;
		CREATE TRIGGER papers_search_update
			BEFORE INSERT OR UPDATE ON papers
			FOR EACH ROW EXECUTE FUNCTION papers_search_trigger();
	`)

	// Create HNSW index for fast vector similarity search (pgvector)
	// This dramatically improves semantic search performance at scale
	c.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_embeddings_vector_hnsw
		ON embeddings USING hnsw (vector vector_cosine_ops)
		WITH (m = 16, ef_construction = 64);
	`)

	// Create index on embedding_jobs for efficient queue processing
	c.db.Exec(`CREATE INDEX IF NOT EXISTS idx_embedding_jobs_status_priority ON embedding_jobs(status, priority DESC, created_at)`)

	// Add vector column to author_embeddings for pgvector
	c.db.Exec(`ALTER TABLE author_embeddings ADD COLUMN IF NOT EXISTS vector vector(384)`)
	c.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_author_embeddings_vector_hnsw
		ON author_embeddings USING hnsw (vector vector_cosine_ops)
		WITH (m = 16, ef_construction = 64);
	`)

	fmt.Println("PostgreSQL schema initialized with full-text search and pgvector HNSW index")
	return nil
}

// Stats returns cache statistics.
func (c *Cache) Stats(ctx context.Context) (*CacheStats, error) {
	stats := &CacheStats{}

	if err := c.db.WithContext(ctx).Model(&Paper{}).Count(&stats.TotalPapers).Error; err != nil {
		return nil, err
	}

	if err := c.db.WithContext(ctx).Model(&Paper{}).Where("pdf_downloaded = ?", true).Count(&stats.PDFsDownloaded).Error; err != nil {
		return nil, err
	}

	if err := c.db.WithContext(ctx).Model(&Paper{}).Where("src_downloaded = ?", true).Count(&stats.SourcesDownloaded).Error; err != nil {
		return nil, err
	}

	if err := c.db.WithContext(ctx).Model(&DownloadQueueItem{}).Count(&stats.QueuedDownloads).Error; err != nil {
		return nil, err
	}

	if err := c.db.WithContext(ctx).Model(&Embedding{}).Count(&stats.EmbeddingsCount).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// CountEmbeddings counts the total number of embeddings in the database.
func (c *Cache) CountEmbeddings(ctx context.Context) (int64, error) {
	var count int64
	err := c.db.WithContext(ctx).Model(&Embedding{}).Count(&count).Error
	return count, err
}

// HasEmbedding checks if a paper has an embedding.
func (c *Cache) HasEmbedding(ctx context.Context, paperID string) bool {
	var count int64
	c.db.WithContext(ctx).Model(&Embedding{}).Where("paper_id = ?", paperID).Count(&count)
	return count > 0
}

// GetEmbeddingIDs returns a set of paper IDs that have embeddings.
func (c *Cache) GetEmbeddingIDs(ctx context.Context) (map[string]bool, error) {
	var ids []string
	err := c.db.WithContext(ctx).Model(&Embedding{}).Pluck("paper_id", &ids).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}

// CacheStats contains statistics about the cache.
type CacheStats struct {
	TotalPapers       int64
	PDFsDownloaded    int64
	SourcesDownloaded int64
	QueuedDownloads   int64
	EmbeddingsCount   int64
}

// GenerateEmbeddingForPaper generates an embedding for a single paper.
func (c *Cache) GenerateEmbeddingForPaper(ctx context.Context, paperID string) error {
	// Check if embedding already exists
	var existingCount int64
	c.db.WithContext(ctx).Model(&Embedding{}).Where("paper_id = ?", paperID).Count(&existingCount)
	if existingCount > 0 {
		return nil
	}

	cmd := exec.CommandContext(ctx, "python3", getEmbeddingScriptPath(), c.root, "--paper-id", paperID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %v, output: %s", err, string(output))
	}

	outputStr := strings.TrimSpace(string(output))
	if strings.HasPrefix(outputStr, "ERROR:") {
		return fmt.Errorf("embedding script error: %s", outputStr)
	}

	return nil
}

// RebuildSearchIndex rebuilds the full-text search index.
func (c *Cache) RebuildSearchIndex(ctx context.Context) error {
	if c.dbType == DBTypePostgres {
		fmt.Println("Rebuilding PostgreSQL search vectors...")
		return c.db.WithContext(ctx).Exec(`
			UPDATE papers SET search_vector =
				setweight(to_tsvector('english', COALESCE(title, '')), 'A') ||
				setweight(to_tsvector('english', COALESCE(abstract, '')), 'B')
		`).Error
	}
	// SQLite FTS rebuild
	return c.RebuildFTSIndex(ctx)
}
