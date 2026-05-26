package arxiv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"gorm.io/gorm/clause"
)

// EmbeddingWorkerConfig configures the background embedding worker.
type EmbeddingWorkerConfig struct {
	// ServiceURL is the URL of the embedding service (e.g., "http://localhost:8001")
	ServiceURL string
	// BatchSize is the number of papers to process in each batch
	BatchSize int
	// PollInterval is how often to check for new papers to embed
	PollInterval time.Duration
	// MaxRetries is the maximum number of retries for failed embeddings
	MaxRetries int
	// Enabled controls whether the worker runs
	Enabled bool
}

// DefaultEmbeddingWorkerConfig returns sensible defaults.
func DefaultEmbeddingWorkerConfig() EmbeddingWorkerConfig {
	return EmbeddingWorkerConfig{
		ServiceURL:   "http://localhost:8001",
		BatchSize:    32,
		PollInterval: 10 * time.Second,
		MaxRetries:   3,
		Enabled:      true,
	}
}

// EmbeddingWorker processes embedding jobs in the background.
type EmbeddingWorker struct {
	cache    *Cache
	config   EmbeddingWorkerConfig
	client   *http.Client
	mu       sync.RWMutex
	running  bool
	stopping bool
	stats    EmbeddingWorkerStats
	stopChan chan struct{}
	doneChan chan struct{}
}

// EmbeddingWorkerStats tracks worker statistics.
type EmbeddingWorkerStats struct {
	Processed int64     `json:"processed"`
	Failed    int64     `json:"failed"`
	Pending   int64     `json:"pending"`
	LastRun   time.Time `json:"lastRun"`
	LastError string    `json:"lastError,omitempty"`
	IsRunning bool      `json:"isRunning"`
	ServiceUp bool      `json:"serviceUp"`
}

// NewEmbeddingWorker creates a new embedding worker.
func NewEmbeddingWorker(cache *Cache, config EmbeddingWorkerConfig) *EmbeddingWorker {
	return &EmbeddingWorker{
		cache:  cache,
		config: config,
		client: &http.Client{
			Timeout: 60 * time.Second, // Allow time for batch processing
		},
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}
}

// Start begins the background worker.
func (w *EmbeddingWorker) Start(ctx context.Context) {
	if !w.config.Enabled {
		log.Println("Embedding worker disabled")
		return
	}

	w.mu.Lock()
	if w.running || w.stopping {
		w.mu.Unlock()
		return
	}
	w.stopChan = make(chan struct{})
	w.doneChan = make(chan struct{})
	w.running = true
	w.stats.IsRunning = true
	w.mu.Unlock()

	log.Printf("Starting embedding worker (service: %s, batch: %d, poll: %v)",
		w.config.ServiceURL, w.config.BatchSize, w.config.PollInterval)

	go w.run(ctx)
}

// Stop gracefully stops the worker.
func (w *EmbeddingWorker) Stop() {
	w.mu.Lock()
	if !w.running && !w.stopping {
		w.mu.Unlock()
		return
	}
	if !w.stopping {
		close(w.stopChan)
		w.stopping = true
	}
	done := w.doneChan
	w.mu.Unlock()

	<-done

	log.Println("Embedding worker stopped")
}

// Stats returns current worker statistics.
func (w *EmbeddingWorker) Stats() EmbeddingWorkerStats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stats
}

// run is the main worker loop.
func (w *EmbeddingWorker) run(ctx context.Context) {
	defer func() {
		w.mu.Lock()
		w.running = false
		w.stopping = false
		w.stats.IsRunning = false
		done := w.doneChan
		w.mu.Unlock()
		close(done)
	}()

	for {
		processed := w.processBatch(ctx)
		nextPoll := w.config.PollInterval
		if !processed {
			nextPoll = 5 * time.Minute
		}

		timer := time.NewTimer(nextPoll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-w.stopChan:
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// processBatch processes a batch of papers without embeddings.
func (w *EmbeddingWorker) processBatch(ctx context.Context) bool {
	// Check if embedding service is available
	if !w.checkServiceHealth() {
		w.mu.Lock()
		w.stats.ServiceUp = false
		w.stats.LastError = "embedding service unavailable"
		w.mu.Unlock()
		return false
	}

	w.mu.Lock()
	w.stats.ServiceUp = true
	w.stats.LastRun = time.Now()
	w.mu.Unlock()

	// Get papers without embeddings
	papers, err := w.getPendingPapers(ctx, w.config.BatchSize)
	if err != nil {
		log.Printf("Error getting pending papers: %v", err)
		w.mu.Lock()
		w.stats.LastError = err.Error()
		w.mu.Unlock()
		return false
	}

	if len(papers) == 0 {
		w.mu.Lock()
		w.stats.Pending = 0
		w.stats.LastError = ""
		w.mu.Unlock()
		return false
	}

	// Update pending count
	pendingCount, _ := w.countPendingPapers(ctx)
	w.mu.Lock()
	w.stats.Pending = pendingCount
	w.mu.Unlock()

	// Send to embedding service
	success, failed := w.embedPapers(ctx, papers)

	w.mu.Lock()
	w.stats.Processed += int64(success)
	w.stats.Failed += int64(failed)
	if failed > 0 {
		w.stats.LastError = fmt.Sprintf("%d papers failed in last batch", failed)
	} else {
		w.stats.LastError = ""
	}
	w.mu.Unlock()

	if success > 0 {
		log.Printf("Embedded %d papers (%d failed, %d pending)", success, failed, pendingCount-int64(success))
	}
	return true
}

// checkServiceHealth checks if the embedding service is responding.
func (w *EmbeddingWorker) checkServiceHealth() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", w.config.ServiceURL+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var health struct {
		Ready bool `json:"ready"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return false
	}

	return health.Ready
}

// getPendingPapers gets papers that need embeddings.
func (w *EmbeddingWorker) getPendingPapers(ctx context.Context, limit int) ([]Paper, error) {
	var papers []Paper

	// Get only the columns needed by the embedding service.
	err := w.cache.db.WithContext(ctx).
		Raw(`
			SELECT p.id, p.title, p.abstract FROM papers p
			WHERE p.title != '' AND p.abstract != ''
			AND NOT EXISTS (SELECT 1 FROM embeddings e WHERE e.paper_id = p.id)
			ORDER BY p.fetched_at DESC NULLS LAST
			LIMIT ?
		`, limit).
		Scan(&papers).Error

	return papers, err
}

// countPendingPapers counts papers without embeddings.
func (w *EmbeddingWorker) countPendingPapers(ctx context.Context) (int64, error) {
	var count int64
	err := w.cache.db.WithContext(ctx).
		Raw(`
			SELECT COUNT(*) FROM papers p
			WHERE p.title != '' AND p.abstract != ''
			AND NOT EXISTS (SELECT 1 FROM embeddings e WHERE e.paper_id = p.id)
		`).
		Scan(&count).Error
	return count, err
}

// paperEmbedRequest matches the FastAPI request schema.
type paperEmbedRequest struct {
	PaperID  string `json:"paper_id"`
	Title    string `json:"title"`
	Abstract string `json:"abstract"`
}

type papersEmbedRequest struct {
	Papers []paperEmbedRequest `json:"papers"`
}

type papersEmbedResponse struct {
	Success   bool   `json:"success"`
	Processed int    `json:"processed"`
	Skipped   int    `json:"skipped"`
	Message   string `json:"message"`
}

// embedPapers sends papers to the embedding service for processing.
func (w *EmbeddingWorker) embedPapers(ctx context.Context, papers []Paper) (success, failed int) {
	if len(papers) == 0 {
		return 0, 0
	}

	// Prepare request
	req := papersEmbedRequest{
		Papers: make([]paperEmbedRequest, len(papers)),
	}
	for i, p := range papers {
		req.Papers[i] = paperEmbedRequest{
			PaperID:  p.ID,
			Title:    p.Title,
			Abstract: p.Abstract,
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		log.Printf("Error marshaling embed request: %v", err)
		return 0, len(papers)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", w.config.ServiceURL+"/embed/papers", bytes.NewReader(body))
	if err != nil {
		log.Printf("Error creating embed request: %v", err)
		return 0, len(papers)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(httpReq)
	if err != nil {
		log.Printf("Error calling embed service: %v", err)
		return 0, len(papers)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Embed service error (status %d): %s", resp.StatusCode, string(body))
		return 0, len(papers)
	}

	var result papersEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Error decoding embed response: %v", err)
		return 0, len(papers)
	}

	return result.Processed, result.Skipped
}

// QueuePaper adds a paper to the embedding queue with optional priority.
func (c *Cache) QueueEmbedding(ctx context.Context, paperID string, priority int) error {
	job := EmbeddingJob{
		PaperID:  paperID,
		Status:   EmbeddingJobPending,
		Priority: priority,
	}

	return c.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "paper_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"priority", "updated_at"}),
		}).
		Create(&job).Error
}

// QueueAllPendingEmbeddings queues all papers without embeddings.
func (c *Cache) QueueAllPendingEmbeddings(ctx context.Context) (int64, error) {
	result := c.db.WithContext(ctx).Exec(`
		INSERT INTO embedding_jobs (paper_id, status, priority, created_at, updated_at)
		SELECT p.id, 'pending', 0, NOW(), NOW()
		FROM papers p
		WHERE p.title != '' AND p.abstract != ''
		AND NOT EXISTS (SELECT 1 FROM embeddings e WHERE e.paper_id = p.id)
		AND NOT EXISTS (SELECT 1 FROM embedding_jobs j WHERE j.paper_id = p.id)
	`)
	return result.RowsAffected, result.Error
}

// EmbeddingJobStats returns statistics about embedding jobs.
func (c *Cache) EmbeddingJobStats(ctx context.Context) (map[string]int64, error) {
	type statusCount struct {
		Status string
		Count  int64
	}

	var counts []statusCount
	err := c.db.WithContext(ctx).
		Model(&EmbeddingJob{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&counts).Error

	if err != nil {
		return nil, err
	}

	result := make(map[string]int64)
	for _, c := range counts {
		result[c.Status] = c.Count
	}

	return result, nil
}
