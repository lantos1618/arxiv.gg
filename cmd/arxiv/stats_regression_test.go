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

func TestHomepageStatsShowPreparationWithinCachedCatalog(t *testing.T) {
	for _, test := range []struct {
		name     string
		total    int64
		prepared int64
		percent  string
	}{
		{name: "reported catalog", total: 3101899, prepared: 2600766, percent: "83.8%"},
		{name: "half prepared", total: 10, prepared: 5, percent: "50.0%"},
		{name: "fully prepared", total: 10, prepared: 10, percent: "100.0%"},
		{name: "none prepared", total: 10, percent: "0.0%"},
		{name: "empty catalog"},
		{name: "inconsistent snapshot", total: 10, prepared: 11},
	} {
		t.Run(test.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			(&server{}).renderTemplate(rec, httptest.NewRequest(http.MethodGet, "/", nil), "index", map[string]any{
				"Stats": &arxiv.CacheStats{TotalPapers: test.total, QwenEmbeddingsCount: test.prepared},
			})
			if rec.Code != http.StatusOK {
				t.Fatalf("render homepage: status=%d body=%s", rec.Code, rec.Body.String())
			}
			line := regexp.MustCompile(`<p id="stats-line">.*?</p>`).FindString(rec.Body.String())
			if !strings.Contains(line, formatInt(test.total)) || !strings.Contains(line, formatInt(test.prepared)) {
				t.Fatalf("homepage lost measured counts: %s", line)
			}
			if test.percent == "" {
				if strings.Contains(line, `id="stat-prepared-percent"`) {
					t.Fatalf("invalid denominator or inconsistent counts must not imply coverage: %s", line)
				}
			} else if !strings.Contains(line, ">"+test.percent+"</span> prepared for semantic search") {
				t.Fatalf("preparation ratio must use local counts and a clear label: %s", line)
			}
			for _, unsupported := range []string{"official", "cached coverage", "NaN", "+Inf"} {
				if strings.Contains(line, unsupported) {
					t.Fatalf("homepage shows unsupported %q: %s", unsupported, line)
				}
			}
		})
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
