package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	arxiv "github.com/lantos1618/arxiv.gg"
)

// Opt-in only: this opens/migrates an explicitly disposable PostgreSQL database.
func TestResearchHTTPPostgres(t *testing.T) {
	dsn := os.Getenv("ARXIV_RESEARCH_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set ARXIV_RESEARCH_TEST_POSTGRES_DSN to a disposable database")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("INDEXNOW_QUEUE_FILE", "")
	cache, err := arxiv.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	const id = "2601.99991"
	_, err = db.Exec(`INSERT INTO papers(id,title,abstract,authors,categories,created,updated) VALUES($1,'Fixture Ocean','Ocean rendering at 42 ms. Code https://github.com/example/ocean','Example Author','cs.GR','2026-01-01','2026-01-01') ON CONFLICT(id) DO UPDATE SET title=EXCLUDED.title,abstract=EXCLUDED.abstract`, id)
	if err != nil {
		t.Fatal(err)
	}
	user, err := cache.FindOrCreateUser(ctx, "research-test@example.test", "Research Test", "", true, "google", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := cache.CreateUserAPIKey(ctx, user.ID, "Research integration "+time.Now().Format("150405.000000"))
	if err != nil {
		t.Fatal(err)
	}
	s := &server{cache: cache, qwenAsyncWorkerEnabled: true, paperBroadcast: newPaperBroadcaster()}
	request := func(method, path, body string, auth bool, handler http.HandlerFunc) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if auth {
			r.Header.Set("Authorization", "Bearer "+key)
		}
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	w := request("GET", "/api/v1/research?q=Fixture+Ocean&mode=keyword&category=cs.GR&since=2025", "", false, s.handleAPIResearch)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"title":"Fixture Ocean"`) {
		t.Fatalf("keyword response %d: %s", w.Code, w.Body.String())
	}
	w = request("GET", "/api/v1/account", "", true, s.handleAPIAccountAccess)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"authenticated":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("POST", "/api/v1/papers/"+id+"/request", "{}", true, s.handleAPIRequestPaper)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	query := "unseen fixture query " + time.Now().Format("150405.000000")
	w = request("GET", "/api/v1/research?q="+url.QueryEscape(query)+"&mode=semantic", "", false, s.handleAPIResearch)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var queued struct {
		Data researchResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &queued); err != nil || queued.Data.Job == nil {
		t.Fatal(err, w.Body.String())
	}
	w = request("GET", queued.Data.Job.StatusURL, "", false, s.handleAPIResearchStatus)
	if w.Code != 200 || strings.Contains(w.Body.String(), query) {
		t.Fatal("status must not expose query", w.Code, w.Body.String())
	}
	vector := make([]float32, arxiv.QwenEmbeddingDim)
	vector[0] = 1
	if err := cache.StoreQwenQueryEmbeddingForQuery(ctx, query, vector); err != nil {
		t.Fatal(err)
	}
	w = request("GET", queued.Data.Job.StatusURL, "", false, s.handleAPIResearchStatus)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"ready"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("POST", "/api/v1/research/batch", `{"queries":[{"q":"Fixture Ocean","mode":"keyword"},{"q":"unmatchedtoken","mode":"keyword"}]}`, false, s.handleAPIResearchBatch)
	if w.Code != 200 || strings.Index(w.Body.String(), `"query":"Fixture Ocean"`) > strings.Index(w.Body.String(), `"query":"unmatchedtoken"`) {
		t.Fatal(w.Code, w.Body.String())
	}
}
