package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	arxiv "github.com/lantos1618/arxiv.gg"
	"gorm.io/gorm"
)

// The research contract is additive: legacy UI/REST clients retain their fields.
type researchRequest struct {
	Query         string  `json:"q"`
	Mode          string  `json:"mode,omitempty"`
	Category      string  `json:"category,omitempty"`
	Since         string  `json:"since,omitempty"`
	Until         string  `json:"until,omitempty"`
	Year          int     `json:"year,omitempty"`
	Limit         int     `json:"limit,omitempty"`
	MinSimilarity float64 `json:"minSimilarity,omitempty"`
	RecencyBoost  float64 `json:"recencyBoost,omitempty"`
	WaitSeconds   int     `json:"waitSeconds,omitempty"`
	filter        arxiv.ResearchFilter
}
type researchPaper struct {
	ID               string    `json:"id"`
	PublishedMonth   string    `json:"publishedMonth"`
	Title            string    `json:"title"`
	Abstract         string    `json:"abstract"`
	Authors          string    `json:"authors"`
	Categories       []string  `json:"categories"`
	Created          time.Time `json:"created"`
	Updated          time.Time `json:"updated"`
	ArxivURL         string    `json:"arxivUrl"`
	URL              string    `json:"url"`
	CodeLinks        []string  `json:"codeLinks"`
	CodeAvailability string    `json:"codeAvailability"`
}
type researchResult struct {
	PaperID    string         `json:"paperId"`
	Paper      *researchPaper `json:"paper"`
	Similarity *float64       `json:"similarity"`
	Score      float64        `json:"score"`
	Snippet    string         `json:"snippet"`
	Section    string         `json:"section,omitempty"`
	ChunkID    string         `json:"chunkId,omitempty"`
}
type researchJob struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	StatusURL         string `json:"statusUrl"`
	RetryAfterSeconds int    `json:"retryAfterSeconds"`
}
type researchResponse struct {
	EffectiveMode string           `json:"effectiveMode"`
	Partial       bool             `json:"partial"`
	Query         string           `json:"query"`
	Mode          string           `json:"mode"`
	Results       []researchResult `json:"results"`
	Count         int              `json:"count"`
	Complete      bool             `json:"complete"`
	Job           *researchJob     `json:"job,omitempty"`
	Warnings      []string         `json:"warnings"`
}

var codeLinkPattern = regexp.MustCompile(`https?://(?:github\.com|gitlab\.com|codeberg\.org|bitbucket\.org)/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+`)

func researchPaperView(p *arxiv.Paper) *researchPaper {
	if p == nil {
		return nil
	}
	links := []string{}
	seen := map[string]bool{}
	for _, link := range codeLinkPattern.FindAllString(p.Abstract+" "+p.Comments+" "+p.PDFText, -1) {
		link = strings.TrimRight(link, ".")
		if !seen[link] {
			links = append(links, link)
			seen[link] = true
		}
	}
	state := "unknown"
	if len(links) > 0 {
		state = "linked"
	}
	return &researchPaper{ID: p.ID, PublishedMonth: arxiv.PaperSubmissionMonth(p), Title: p.Title, Abstract: p.Abstract, Authors: p.Authors, Categories: strings.Fields(p.Categories), Created: p.Created, Updated: p.Updated, ArxivURL: "https://arxiv.org/abs/" + p.ID, URL: "https://arxiv.gg" + paperPath(p.ID), CodeLinks: links, CodeAvailability: state}
}
func parseResearchDate(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{"2006-01", "2006"} {
		if date, err := time.Parse(layout, raw); err == nil {
			return date, nil
		}
	}
	return time.Time{}, fmt.Errorf("dates must use YYYY or YYYY-MM (original arXiv submission month)")
}
func (q *researchRequest) validate() error {
	var err error
	q.Query, err = validateSearchQuery(q.Query)
	if err != nil {
		return err
	}
	if q.Mode == "" {
		q.Mode = "hybrid"
	}
	switch q.Mode {
	case "keyword", "semantic", "hybrid", "deep":
	default:
		return fmt.Errorf("mode must be keyword, semantic, hybrid, or deep")
	}
	if q.Limit == 0 {
		q.Limit = 20
	}
	if q.Limit < 1 || q.Limit > 100 {
		return fmt.Errorf("limit must be between 1 and 100")
	}
	if q.WaitSeconds < 0 || q.WaitSeconds > 8 {
		return fmt.Errorf("waitSeconds must be between 0 and 8")
	}
	for _, v := range []float64{q.MinSimilarity, q.RecencyBoost} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return fmt.Errorf("minSimilarity and recencyBoost must be between 0 and 1")
		}
	}
	q.filter.Category, err = validateSearchCategory(q.Category)
	if err != nil {
		return err
	}
	if q.Year != 0 {
		if q.Year < 1991 || q.Year > time.Now().Year()+1 || q.Since != "" || q.Until != "" {
			return fmt.Errorf("year must be a valid arXiv year and cannot be combined with since/until")
		}
		q.Since = strconv.Itoa(q.Year)
		q.Until = strconv.Itoa(q.Year + 1)
	}
	q.filter.Since, err = parseResearchDate(q.Since)
	if err != nil {
		return err
	}
	q.filter.Until, err = parseResearchDate(q.Until)
	if err != nil {
		return err
	}
	if !q.filter.Since.IsZero() && !q.filter.Until.IsZero() && !q.filter.Since.Before(q.filter.Until) {
		return fmt.Errorf("until must be later than since")
	}
	q.filter.MinSimilarity = q.MinSimilarity
	return nil
}
func researchFromURL(v url.Values) (researchRequest, error) {
	q := researchRequest{Query: v.Get("q"), Mode: v.Get("mode"), Category: v.Get("category"), Since: v.Get("since"), Until: v.Get("until")}
	for name, dst := range map[string]*int{"limit": &q.Limit, "year": &q.Year, "waitSeconds": &q.WaitSeconds} {
		if raw := v.Get(name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				return q, fmt.Errorf("invalid %s", name)
			}
			*dst = n
		}
	}
	for name, dst := range map[string]*float64{"minSimilarity": &q.MinSimilarity, "recencyBoost": &q.RecencyBoost} {
		if raw := v.Get(name); raw != "" {
			n, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return q, fmt.Errorf("invalid %s", name)
			}
			*dst = n
		}
	}
	return q, q.validate()
}
func (s *server) initResearch() {
	s.researchOnce.Do(func() {
		s.researchLimiter = newRateLimiter(60, time.Minute, s.trustProxyHeaders)
		s.ingestLimiter = newRateLimiter(6, time.Minute, s.trustProxyHeaders)
		s.researchSlots = make(chan struct{}, 4)
		s.ingestSlots = make(chan struct{}, 2)
	})
}
func (s *server) allowResearch(w http.ResponseWriter, r *http.Request, queries []researchRequest) bool {
	s.initResearch()
	_, signedIn := s.currentUser(r)
	if apiKeyFromRequest(r) != "" && !signedIn {
		respondJSON(w, 401, APIResponse{Error: "invalid or revoked account API key"})
		return false
	}
	for _, q := range queries {
		if q.Mode == "deep" && !signedIn {
			respondJSON(w, 401, APIResponse{Error: "Deep Search requires an account key: Authorization: Bearer <key> or X-API-Key: <key>"})
			return false
		}
		if !s.researchLimiter.Allow(r) {
			w.Header().Set("Retry-After", "60")
			writeRateLimitExceeded(w, r)
			return false
		}
	}
	return true
}
func (s *server) handleAPIResearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondJSON(w, 405, APIResponse{Error: "use GET"})
		return
	}
	q, err := researchFromURL(r.URL.Query())
	if err != nil {
		respondJSON(w, 400, APIResponse{Error: err.Error()})
		return
	}
	if !s.allowResearch(w, r, []researchRequest{q}) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	result, err := s.runResearch(ctx, q)
	if err != nil {
		respondAPIInternalError(w, 503, "research search", "search could not finish; retry or narrow the query", err)
		return
	}
	code := 200
	if result.Partial {
		code = http.StatusPartialContent
	}
	if result.Job != nil {
		code = 202
		w.Header().Set("Retry-After", "3")
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, code, APIResponse{Success: true, Data: result})
}
func (s *server) runResearch(ctx context.Context, q researchRequest) (researchResponse, error) {
	out := researchResponse{Query: q.Query, Mode: q.Mode, Results: []researchResult{}, Warnings: []string{}, Complete: true}
	select {
	case s.researchSlots <- struct{}{}:
		defer func() { <-s.researchSlots }()
	case <-ctx.Done():
		return out, ctx.Err()
	}
	var vector []float32
	if q.Mode != "keyword" {
		var err error
		vector, err = s.generateQwenQueryEmbedding(ctx, q.Query)
		if errors.Is(err, errQwenQueryEmbeddingQueued) && q.WaitSeconds > 0 {
			waitCtx, cancel := context.WithTimeout(ctx, time.Duration(q.WaitSeconds)*time.Second)
			ticker := time.NewTicker(250 * time.Millisecond)
			waiting := true
			for waiting {
				select {
				case <-waitCtx.Done():
					waiting = false
				case <-ticker.C:
					cached, ready, cacheErr := s.cache.GetQwenQueryEmbedding(waitCtx, q.Query)
					if cacheErr != nil {
						waiting = false
					} else if ready {
						vector = cached
						err = nil
						waiting = false
					}
				}
			}
			ticker.Stop()
			cancel()
		}
		if err != nil {
			if errors.Is(err, errQwenQueryEmbeddingQueued) {
				_, hash := arxiv.QwenQueryHash(q.Query)
				out.Complete = false
				out.Partial = true
				out.Job = &researchJob{ID: hash, Status: "queued", StatusURL: "/api/v1/research/status?id=" + hash, RetryAfterSeconds: 3}
				out.Warnings = append(out.Warnings, "Query embedding is queued. Poll job.statusUrl; retry this search when ready.")
			} else {
				out.Partial = true
				out.Warnings = append(out.Warnings, "Semantic execution is unavailable; no similarity scores are inferred.")
			}
		}
	}
	usedKeyword, usedVector := false, false
	var lexical []arxiv.Paper
	if q.Mode == "keyword" || q.Mode == "hybrid" || (q.Mode == "semantic" && len(vector) == 0) {
		var err error
		lexical, err = s.cache.ResearchKeywords(ctx, q.Query, q.filter, min(100, q.Limit*3))
		if err != nil {
			if q.Mode == "keyword" {
				return out, err
			}
			out.Partial = true
			out.Warnings = append(out.Warnings, "Keyword matches could not finish within the search budget.")
		} else {
			usedKeyword = true
		}
	}
	merged := map[string]*researchResult{}
	for i := range lexical {
		p := &lexical[i]
		snippet := []rune(p.Abstract)
		if len(snippet) > 350 {
			snippet = append(snippet[:350], []rune("…")...)
		}
		merged[p.ID] = &researchResult{PaperID: p.ID, Paper: researchPaperView(p), Score: 1 / float64(60+i+1), Snippet: string(snippet)}
	}
	if len(vector) > 0 {
		rows, err := s.cache.ResearchVectors(ctx, vector, q.filter, min(100, q.Limit*3), q.Mode == "deep")
		if err != nil {
			if len(lexical) == 0 {
				return out, err
			}
			out.Partial = true
			out.Warnings = append(out.Warnings, "Vector retrieval could not finish; returning keyword matches.")
		} else {
			usedVector = true
			for i, row := range rows {
				item := merged[row.PaperID]
				if item == nil {
					item = &researchResult{PaperID: row.PaperID, Paper: researchPaperView(row.Paper)}
					merged[row.PaperID] = item
				}
				sim := row.Similarity
				item.Similarity = &sim
				item.Score += 1 / float64(60+i+1)
				item.ChunkID = row.ChunkID
				item.Section = row.Section
				if row.Snippet != "" {
					item.Snippet = row.Snippet
				} else if item.Paper != nil && item.Snippet == "" {
					text := []rune(item.Paper.Abstract)
					if len(text) > 350 {
						text = append(text[:350], '…')
					}
					item.Snippet = string(text)
				}
			}
		}
	}
	now := time.Now()
	for _, row := range merged {
		if q.MinSimilarity > 0 && (row.Similarity == nil || *row.Similarity < q.MinSimilarity) {
			continue
		}
		if row.Paper == nil {
			continue
		}
		if q.Mode == "hybrid" || q.Mode == "keyword" {
			if strings.EqualFold(strings.TrimSpace(row.Paper.Title), q.Query) || row.PaperID == q.Query {
				row.Score += 1
			} else if strings.Contains(strings.ToLower(row.Paper.Title), strings.ToLower(q.Query)) {
				row.Score += .1
			}
		}
		if q.RecencyBoost > 0 && !row.Paper.Created.IsZero() {
			published, _ := time.Parse("2006-01", row.Paper.PublishedMonth)
			years := math.Max(0, now.Sub(published).Hours()/(365.25*24))
			row.Score += q.RecencyBoost * .02 / (1 + years)
		}
		out.Results = append(out.Results, *row)
	}
	sort.Slice(out.Results, func(i, j int) bool {
		a, b := out.Results[i], out.Results[j]
		if a.Score == b.Score {
			return a.PaperID < b.PaperID
		}
		return a.Score > b.Score
	})
	if len(out.Results) > q.Limit {
		out.Results = out.Results[:q.Limit]
	}
	out.Count = len(out.Results)
	out.EffectiveMode = "none"
	if usedKeyword {
		out.EffectiveMode = "keyword"
	}
	if usedVector {
		out.EffectiveMode = "semantic"
		if q.Mode == "deep" {
			out.EffectiveMode = "deep"
		}
		if usedKeyword {
			out.EffectiveMode = "hybrid"
		}
	}

	if out.Job != nil && out.Count == 0 {
		out.Warnings = append(out.Warnings, "No keyword fallback matched. This does not mean semantic search has no matches.")
	}
	return out, nil
}
func (s *server) handleAPIResearchBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, 405, APIResponse{Error: "use POST"})
		return
	}
	var body struct {
		Queries []researchRequest `json:"queries"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		respondJSON(w, 400, APIResponse{Error: "invalid batch JSON; maximum body is 16 KiB"})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		respondJSON(w, 400, APIResponse{Error: "send one JSON object"})
		return
	}
	if len(body.Queries) < 1 || len(body.Queries) > 4 {
		respondJSON(w, 400, APIResponse{Error: "send between 1 and 4 queries"})
		return
	}
	for i := range body.Queries {
		if err := body.Queries[i].validate(); err != nil {
			respondJSON(w, 400, APIResponse{Error: fmt.Sprintf("query %d: %s", i, err)})
			return
		}
	}
	if !s.allowResearch(w, r, body.Queries) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	results := make([]APIResponse, len(body.Queries))
	var wg sync.WaitGroup
	for i, q := range body.Queries {
		wg.Add(1)
		go func(i int, q researchRequest) {
			defer wg.Done()
			result, err := s.runResearch(ctx, q)
			if err != nil {
				logAPIInternalError("batch research", err)
				results[i] = APIResponse{Error: "search could not finish; retry or narrow the query"}
			} else {
				results[i] = APIResponse{Success: true, Data: result}
			}
		}(i, q)
	}
	wg.Wait()
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, 200, APIResponse{Success: true, Data: map[string]any{"queries": results}})
}

var researchJobID = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (s *server) handleAPIResearchStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondJSON(w, 405, APIResponse{Error: "use GET"})
		return
	}
	id := r.URL.Query().Get("id")
	if !researchJobID.MatchString(id) {
		respondJSON(w, 400, APIResponse{Error: "id must be the query job ID returned by research search"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	status, err := s.cache.QwenQueryStatusByHash(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondJSON(w, 404, APIResponse{Error: "query job not found"})
		return
	}
	if err != nil {
		respondAPIInternalError(w, 503, "query status", "query status is temporarily unavailable", err)
		return
	}
	state := "queued"
	retry := 3
	if status.Ready {
		state = "ready"
		retry = 0
	} else if status.RunningJobs > 0 {
		state = "running"
	} else {
		for _, job := range status.Jobs {
			if job.Status == arxiv.QwenEmbeddingJobFailed && job.Attempts >= arxiv.MaxQwenEmbeddingJobAttempts {
				state = "failed"
				retry = 0
			}
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	if retry > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(retry))
	}
	respondJSON(w, 200, APIResponse{Success: true, Data: researchJob{ID: id, Status: state, StatusURL: "/api/v1/research/status?id=" + id, RetryAfterSeconds: retry}})
}
func (s *server) handleAPIAccountAccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondJSON(w, 405, APIResponse{Error: "use GET"})
		return
	}
	user, ok := s.currentUser(r)
	if !ok {
		respondJSON(w, 401, APIResponse{Error: "valid account key required: Authorization: Bearer <key> or X-API-Key: <key>"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, 200, APIResponse{Success: true, Data: map[string]any{"authenticated": true, "plan": user.Plan, "capabilities": []string{"publicRead", "researchSearch", "deepSearch", "requestPaper"}, "admin": s.hasAdminAccess(r), "rateLimits": map[string]any{"globalPerIPPerMinute": 1000, "researchQueriesPerIPPerMinute": 60, "paperRequestsPerIPPerMinute": 6, "higherLimitsWithKey": false}}})
}

// Account ingestion is metadata-only. Source downloads and force refresh stay administrative.
func (s *server) handleAPIRequestPaper(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, 405, APIResponse{Error: "use POST"})
		return
	}
	if _, ok := s.currentUser(r); !ok {
		respondJSON(w, 401, APIResponse{Error: "sign in or provide an account API key to request a paper"})
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/papers/"), "/request")
	if !isArxivID(id) {
		respondJSON(w, 400, APIResponse{Error: "invalid arXiv paper ID"})
		return
	}
	s.initResearch()
	if !s.ingestLimiter.Allow(r) {
		w.Header().Set("Retry-After", "60")
		writeRateLimitExceeded(w, r)
		return
	}
	select {
	case s.ingestSlots <- struct{}{}:
		defer func() { <-s.ingestSlots }()
	default:
		w.Header().Set("Retry-After", "3")
		respondJSON(w, 429, APIResponse{Error: "paper ingestion is busy; retry in 3 seconds"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	paper, err := s.cache.FetchMetadataOnly(ctx, id)
	if err != nil {
		respondAPIInternalError(w, 502, "request paper", "paper metadata could not be retrieved from arXiv; verify the ID and retry", err)
		return
	}
	warning := ""
	queued := false
	if s.qwenExecutionConfigured() {
		if status, err := s.cache.EnsureQwenPaperJobs(ctx, paper.ID, 100); err != nil {
			warning = "Metadata added; embeddings could not be queued. Retry POST /api/v1/papers/" + paper.ID + "/embeddings."
		} else {
			queued = status.QueuedJobs > 0 || status.RunningJobs > 0
		}
	}
	s.publishPaper(paperEvent{Paper: *paper, HasEmbedding: s.cache.HasQwenEmbedding(ctx, paper.ID)})
	respondJSON(w, 200, APIResponse{Success: true, Data: map[string]any{"paper": researchPaperView(paper), "embeddingQueued": queued, "statusUrl": "/api/v1/papers/" + paper.ID + "/embedding-status", "warning": warning}})
}
