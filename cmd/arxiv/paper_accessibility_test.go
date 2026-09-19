package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	arxiv "github.com/lantos1618/arxiv.gg"
)

func TestPaperDatesAndRelatedListAccessibility(t *testing.T) {
	for _, suspect := range []bool{false, true} {
		created := time.Date(2017, 6, 12, 0, 0, 0, 0, time.UTC)
		if suspect {
			created = time.Date(2023, 8, 2, 0, 0, 0, 0, time.UTC)
		}
		paper := &arxiv.Paper{ID: "1706.03762", Title: "Attention Is All You Need", Created: created, Updated: time.Date(2023, 8, 2, 0, 0, 0, 0, time.UTC)}
		r := httptest.NewRecorder()
		(&server{}).renderTemplate(r, httptest.NewRequest(http.MethodGet, "/abs/1706.03762", nil), "paper", map[string]any{"Paper": paper, "CitedByCount": 3461})
		if r.Code != http.StatusOK {
			t.Fatalf("render failed: %s", r.Body.String())
		}
		body := r.Body.String()
		for _, want := range []string{`<main id="main-content"`, "3461 citations in our index", `id="related-papers-list"`, "Similarity does not establish a citation relationship"} {
			if !strings.Contains(body, want) {
				t.Errorf("missing %q", want)
			}
		}
		metadata := string(paperStructuredData(paper))
		if suspect {
			if !strings.Contains(body, "Submission dates need verification") || strings.Contains(metadata, `"datePublished"`) || strings.Contains(metadata, `"dateModified"`) {
				t.Errorf("suspect dates presented as facts: %s", metadata)
			}
		} else {
			if !strings.Contains(body, "First submitted") || !strings.Contains(body, "Latest revision") || !strings.Contains(metadata, `"datePublished":"2017-06-12"`) {
				t.Errorf("valid date history lost: %s", metadata)
			}
		}
	}
}
