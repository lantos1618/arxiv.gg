package arxiv

import (
	"time"
)

// Paper represents an arXiv paper's metadata.
type Paper struct {
	// ID is the arXiv identifier (e.g., "2301.00001" or "hep-th/9901001")
	ID string `gorm:"primaryKey"`

	// Created is when the paper was first submitted
	Created time.Time `gorm:"index"`

	// Updated is when the paper was last updated
	Updated time.Time `gorm:"index"`

	// Title of the paper
	Title string

	// Abstract of the paper
	Abstract string

	// Authors as a single string (arXiv format)
	Authors string

	// Categories is a space-separated list of arXiv categories
	Categories string `gorm:"index"`

	// Comments from the submitter (e.g., "10 pages, 3 figures")
	Comments string

	// JournalRef is the journal reference if published
	JournalRef string

	// DOI is the Digital Object Identifier if available
	DOI string

	// License URL
	License string

	// PDFPath is the local path to the PDF (if downloaded)
	PDFPath string `gorm:"column:pdf_path"`

	// SourcePath is the local path to the TeX source (if downloaded)
	SourcePath string `gorm:"column:src_path"`

	// PDFText is extracted text from PDF for search
	PDFText string `gorm:"type:text;column:pdf_text"`

	// PDFDownloaded indicates if the PDF has been downloaded
	PDFDownloaded bool `gorm:"column:pdf_downloaded"`

	// SourceDownloaded indicates if the source has been downloaded
	SourceDownloaded bool `gorm:"column:src_downloaded"`

	// MetadataUpdated timestamp
	MetadataUpdated *time.Time `gorm:"column:metadata_updated"`

	// FetchedAt is when the paper was added/fetched to the local cache
	FetchedAt *time.Time `gorm:"column:fetched_at;index"`
}

func (Paper) TableName() string {
	return "papers"
}

// Citation represents a citation relationship between papers.
type Citation struct {
	FromID string `gorm:"primaryKey;column:from_id"`
	ToID   string `gorm:"primaryKey;column:to_id;index"`
}

func (Citation) TableName() string {
	return "citations"
}

// SyncState stores sync metadata.
type SyncState struct {
	Key   string `gorm:"primaryKey"`
	Value string
}

func (SyncState) TableName() string {
	return "sync_state"
}

// DownloadQueueItem represents a queued download.
type DownloadQueueItem struct {
	PaperID   string `gorm:"primaryKey;column:paper_id"`
	Type      string
	Priority  int
	Added     *time.Time
	Attempts  int
	LastError string `gorm:"column:last_error"`
}

func (DownloadQueueItem) TableName() string {
	return "download_queue"
}

// Embedding stores vector embeddings for semantic search.
// Note: The vector column is handled via raw SQL for pgvector compatibility.
type Embedding struct {
	PaperID string    `gorm:"primaryKey;column:paper_id"`
	Model   string    `gorm:"column:model"`
	Created time.Time `gorm:"column:created"`
	// Vector is managed via raw SQL (pgvector type in PostgreSQL)
}

func (Embedding) TableName() string {
	return "embeddings"
}

// EmbeddingJobStatus represents the status of an embedding job.
type EmbeddingJobStatus string

const (
	EmbeddingJobPending    EmbeddingJobStatus = "pending"
	EmbeddingJobProcessing EmbeddingJobStatus = "processing"
	EmbeddingJobCompleted  EmbeddingJobStatus = "completed"
	EmbeddingJobFailed     EmbeddingJobStatus = "failed"
)

// EmbeddingJob represents a queued embedding generation job.
type EmbeddingJob struct {
	PaperID   string             `gorm:"primaryKey;column:paper_id"`
	Status    EmbeddingJobStatus `gorm:"column:status;default:pending;index"`
	Priority  int                `gorm:"column:priority;default:0;index"`
	Attempts  int                `gorm:"column:attempts;default:0"`
	LastError string             `gorm:"column:last_error"`
	CreatedAt time.Time          `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time          `gorm:"column:updated_at;autoUpdateTime"`
}

func (EmbeddingJob) TableName() string {
	return "embedding_jobs"
}

// AuthorCollaboration represents a co-author relationship between two authors.
type AuthorCollaboration struct {
	Author1     string `gorm:"primaryKey;column:author1;index"`
	Author2     string `gorm:"primaryKey;column:author2;index"`
	PaperCount  int    `gorm:"column:paper_count"`
	PaperIDs    string `gorm:"column:paper_ids;type:text"` // JSON array of paper IDs
	FirstCollab time.Time `gorm:"column:first_collab"`
	LastCollab  time.Time `gorm:"column:last_collab"`
}

func (AuthorCollaboration) TableName() string {
	return "author_collaborations"
}

// AuthorEmbedding stores aggregated embeddings for an author.
// Vector is the average of all their paper embeddings.
type AuthorEmbedding struct {
	Author     string    `gorm:"primaryKey;column:author"`
	PaperCount int       `gorm:"column:paper_count"`
	Model      string    `gorm:"column:model"`
	Updated    time.Time `gorm:"column:updated"`
	// Vector is managed via raw SQL (pgvector type in PostgreSQL)
}

func (AuthorEmbedding) TableName() string {
	return "author_embeddings"
}
