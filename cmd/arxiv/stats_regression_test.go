package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	arxiv "github.com/lantos1618/arxiv.gg"
)

func TestHomepageStatsDoNotInferOfficialCoverage(t *testing.T) {
	rec := httptest.NewRecorder()
	(&server{}).renderTemplate(rec, httptest.NewRequest(http.MethodGet, "/", nil), "index", map[string]any{
		"Stats": &arxiv.CacheStats{TotalPapers: 3101899, QwenEmbeddingsCount: 2600766},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("render homepage: status=%d body=%s", rec.Code, rec.Body.String())
	}
	line := regexp.MustCompile(`<p id="stats-line">.*?</p>`).FindString(rec.Body.String())
	if !strings.Contains(line, "3,101,899") || !strings.Contains(line, "2,600,766") {
		t.Fatalf("homepage lost measured counts: %s", line)
	}
	for _, unsupported := range []string{"%", "official", "coverage"} {
		if strings.Contains(strings.ToLower(line), unsupported) {
			t.Fatalf("homepage infers %q from an unmatched reference: %s", unsupported, line)
		}
	}
}

func TestStatsAPIReportsOfficialCoverageAsUnavailable(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	cache, err := arxiv.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()

	srv := &server{
		cache: cache, paperBroadcast: newPaperBroadcaster(),
		officialArxivPapers: 3045638, officialArxivAsOf: "2026-05-16",
	}
	rec := httptest.NewRecorder()
	srv.handleAPIStats(rec, httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Success || response.Data["TotalPapers"] != float64(0) || response.Data["QwenEmbeddingsCount"] != float64(0) {
		t.Fatalf("measured counts changed: %s", rec.Body.String())
	}
	if response.Data["OfficialArxivPapers"] != float64(3045638) || response.Data["OfficialArxivPapersAsOf"] != "2026-05-16" {
		t.Fatalf("legacy historical reference changed: %s", rec.Body.String())
	}
	if percent, ok := response.Data["OfficialArxivCoveragePercent"]; !ok || percent != "" {
		t.Fatalf("coverage must remain an empty string when unavailable: %s", rec.Body.String())
	}
}
