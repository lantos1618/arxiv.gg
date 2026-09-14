package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	arxiv "github.com/lantos1618/arxiv.gg"
)

func TestPublicSearchRejectsOversizedQueriesBeforeDatabaseWork(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	cache, err := arxiv.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// A closed database makes accidental work observable without a production DB.
	if err := cache.Close(); err != nil {
		t.Fatal(err)
	}
	srv := &server{cache: cache}
	for path, handler := range map[string]http.HandlerFunc{
		"/api/v1/search":          srv.handleAPISearch,
		"/api/v1/search/quick":    srv.handleAPISearchQuick,
		"/api/v1/search/semantic": srv.handleAPISearchSemantic,
		"/api/v1/search/stream":   srv.handleAPISearchStream,
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path+"?q="+url.QueryEscape(strings.Repeat("x", 2049)), nil)
			rec := httptest.NewRecorder()
			handler(rec, req)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "2048 bytes") {
				t.Fatalf("oversized query reached search: status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
	for _, mode := range []string{"quick", "keyword", "semantic", "deep"} {
		t.Run("mcp/"+mode, func(t *testing.T) {
			body := postMCP(t, srv, map[string]any{
				"jsonrpc": "2.0", "id": 1, "method": "tools/call",
				"params": map[string]any{"name": "arxiv_search", "arguments": map[string]any{
					"query": strings.Repeat("x", 2049), "mode": mode,
				}},
			})
			if !strings.Contains(body, `"isError":true`) || !strings.Contains(body, "2048 bytes") {
				t.Fatalf("oversized MCP query was not rejected: %s", body)
			}
		})
	}
}

func TestMCPBackendErrorsDoNotReachToolResponses(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	cache, err := arxiv.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"arxiv_search", "arxiv_citations", "arxiv_cited_by"} {
		t.Run(tool, func(t *testing.T) {
			body := postMCP(t, &server{cache: cache}, map[string]any{
				"jsonrpc": "2.0", "id": 1, "method": "tools/call",
				"params": map[string]any{"name": tool, "arguments": map[string]any{
					"query": "attention", "mode": "keyword", "id": "2501.00001",
				}},
			})
			if !strings.Contains(body, `"isError":true`) {
				t.Fatalf("expected tool failure: %s", body)
			}
			if strings.Contains(body, "database is closed") || strings.Contains(body, "sql:") {
				t.Fatalf("backend details leaked to public MCP response: %s", body)
			}
		})
	}
}

func TestPublicQueuedEmbeddingStatusHidesWorkerDiagnostics(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	cache, err := arxiv.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	ctx := t.Context()
	const paperID = "2501.00005"
	const privateError = "inference host=worker.internal credential=private-test-value failed"
	if _, err := cache.EnsureQwenPaperJobs(ctx, paperID, 50); err != nil {
		t.Fatal(err)
	}
	jobs, err := cache.ClaimQwenEmbeddingJobs(ctx, []string{arxiv.QwenEmbeddingJobKindAbstract}, 1, "worker.internal", time.Minute)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("claim jobs=%d err=%v", len(jobs), err)
	}
	job := jobs[0]
	if err := cache.FailQwenEmbeddingJob(ctx, job.ID, job.LeaseOwner, job.Attempts, errors.New(privateError)); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	(&server{cache: cache}).respondQwenEmbeddingQueued(rec, httptest.NewRequest(http.MethodPost, "/api/v1/papers/"+paperID+"/embeddings", nil), &arxiv.Paper{ID: paperID}, "queued")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, private := range []string{"lastError", "leaseOwner", "worker.internal", "private-test-value"} {
		if strings.Contains(body, private) {
			t.Errorf("public readiness response exposes %q: %s", private, body)
		}
	}
	if !strings.Contains(body, `"failedJobs":1`) || !strings.Contains(body, `"status":"failed"`) {
		t.Fatalf("public failure status was lost: %s", body)
	}
	stored, err := cache.GetQwenEmbeddingJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.LastError != privateError || stored.LeaseOwner != "worker.internal" {
		t.Fatal("worker diagnostics must remain stored for administrators")
	}
}

func TestMCPSafeMessagesPreserveRetryAndValidationWithoutUpstreamDetail(t *testing.T) {
	privateError := errors.New("inference host=worker.internal credential=private-test-value")
	for _, err := range []error{privateError, fmt.Errorf("%w: %v", errQwenQueryEmbeddingQueued, privateError)} {
		_, notice, retryAfter := mcpSemanticFallback(err)
		for _, public := range []string{notice, mcpSafeErrorMessage(err)} {
			if strings.Contains(public, "worker.internal") || strings.Contains(public, "private-test-value") {
				t.Fatalf("private inference error exposed: %s", public)
			}
		}
		if retryAfter <= 0 {
			t.Fatal("fallback lost retry guidance")
		}
	}
	if got := mcpSafeErrorMessage(mcpClientError("query is required")); got != "query is required" {
		t.Fatalf("validation became opaque: %q", got)
	}
}

func TestSearchQueryBoundaryAndEmbeddingGuard(t *testing.T) {
	for _, query := range []string{strings.Repeat("x", maxSearchQueryBytes), strings.Repeat("é", maxSearchQueryBytes/2)} {
		if got, err := validateSearchQuery(query); err != nil || got != query {
			t.Fatalf("valid boundary rejected: %v", err)
		}
	}
	for _, query := range []string{strings.Repeat("x", maxSearchQueryBytes+1), strings.Repeat(" ", maxSearchQueryBytes) + "x", strings.Repeat("é", maxSearchQueryBytes/2+1), " \t\n"} {
		if _, err := (&server{}).generateQwenQueryEmbedding(t.Context(), query); err == nil {
			t.Fatal("invalid query reached embedding work")
		}
	}
}
