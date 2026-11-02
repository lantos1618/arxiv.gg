package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	arxiv "github.com/lantos1618/arxiv.gg"
)

func TestResearchValidation(t *testing.T) {
	for _, query := range []string{"q=ocean&limit=101", "q=ocean&limit=-1", "q=ocean&minSimilarity=NaN", "q=ocean&recencyBoost=Inf", "q=ocean&waitSeconds=9", "q=ocean&since=2025-15", "q=ocean&since=2026&until=2025", "q=ocean&year=2025&since=2024", "q=ocean&mode=unknown", "q=ocean&category=cs.GR%27"} {
		v, _ := url.ParseQuery(query)
		if _, err := researchFromURL(v); err == nil {
			t.Errorf("accepted %s", query)
		}
	}
	v, _ := url.ParseQuery("q=ocean&year=2025&category=cs.GR&minSimilarity=0.5&recencyBoost=0.2&waitSeconds=3")
	q, err := researchFromURL(v)
	if err != nil || q.filter.Since.Year() != 2025 || q.filter.Until.Year() != 2026 || q.filter.Category != "cs.GR" {
		t.Fatalf("invalid valid query: %+v %v", q, err)
	}
}
func TestResearchPaperContract(t *testing.T) {
	p := researchPaperView(&arxiv.Paper{ID: "2601.00001", Title: "Ocean", Abstract: "Code: https://github.com/example/ocean. https://github.com/example/ocean", Authors: "A, B", Categories: "cs.GR cs.CV"})
	if len(p.CodeLinks) != 1 || p.CodeLinks[0] != "https://github.com/example/ocean" || p.CodeAvailability != "linked" {
		t.Fatalf("links: %+v", p)
	}
	raw, _ := json.Marshal(p)
	var body map[string]any
	json.Unmarshal(raw, &body)
	for _, key := range []string{"id", "title", "abstract", "authors", "categories", "created", "updated", "arxivUrl", "url", "codeLinks", "codeAvailability"} {
		if _, ok := body[key]; !ok {
			t.Errorf("missing %s", key)
		}
	}
	if _, ok := body["Title"]; ok {
		t.Fatal("PascalCase leaked")
	}
	if researchPaperView(&arxiv.Paper{}).CodeAvailability != "unknown" {
		t.Fatal("absence must not imply code is unavailable")
	}
}
func TestResearchRejectsUnsafeRequestsBeforeDatabase(t *testing.T) {
	s := &server{}
	for _, tc := range []struct {
		path, body string
		handler    http.HandlerFunc
		status     int
	}{
		{"/api/v1/research?q=ocean&mode=deep", "", s.handleAPIResearch, 401},
		{"/api/v1/account", "", s.handleAPIAccountAccess, 401},
		{"/api/v1/research/status?id=bad", "", s.handleAPIResearchStatus, 400},
		{"/api/v1/research/batch", `{"queries":[]}`, s.handleAPIResearchBatch, 400},
		{"/api/v1/research/batch", `{"queries":[{"q":"a","bad":true}]}`, s.handleAPIResearchBatch, 400},
		{"/api/v1/research/batch", `{"queries":[{"q":"a"}]}{}`, s.handleAPIResearchBatch, 400},
		{"/api/v1/papers/2601.00001/request", "{}", s.handleAPIRequestPaper, 401},
	} {
		method := "GET"
		if tc.body != "" {
			method = "POST"
		}
		r := httptest.NewRequest(method, tc.path, strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		tc.handler(w, r)
		if w.Code != tc.status {
			t.Errorf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
}
func TestResearchInvalidKeyNotMistakenForPublicSuccess(t *testing.T) {
	s := &server{}
	r := httptest.NewRequest("GET", "/api/v1/research?q=ocean", nil)
	r.Header.Set("Authorization", "Bearer invalid")
	w := httptest.NewRecorder()
	s.handleAPIResearch(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestSearchPageDoesNotWaitForDatabase(t *testing.T) {
	w := httptest.NewRecorder()
	(&server{}).handleSearch(w, httptest.NewRequest("GET", "/search?q=ocean+whitecap+foam+rendering", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Finding papers…") || !strings.Contains(w.Body.String(), "restoreSearch();") || !strings.Contains(w.Body.String(), "static=1") {
		t.Fatalf("page did not render immediate progress with fallback: %d", w.Code)
	}
}
func TestResearchOpenAPISchema(t *testing.T) {
	var spec map[string]any
	if err := json.Unmarshal(researchOpenAPI, &spec); err != nil {
		t.Fatal(err)
	}
	paths := spec["paths"].(map[string]any)
	for _, path := range []string{"/research", "/research/batch", "/research/status", "/account", "/papers/{id}/request"} {
		if paths[path] == nil {
			t.Errorf("missing %s", path)
		}
	}
}
