package arxiv

import (
	"reflect"
	"testing"
)

func TestParsePgVectorText(t *testing.T) {
	got, err := parsePgVectorText("[0.25,-1,3.5]")
	if err != nil {
		t.Fatalf("parsePgVectorText returned error: %v", err)
	}
	want := []float64{0.25, -1, 3.5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePgVectorText = %#v, want %#v", got, want)
	}
}

func TestFirstNIsRuneSafe(t *testing.T) {
	got := firstN("ab\u03b3de", 3)
	want := "ab\u03b3"
	if got != want {
		t.Fatalf("firstN = %q, want %q", got, want)
	}
}

func TestExtractMapTermsFiltersLatexNoise(t *testing.T) {
	got := extractMapTerms(`CP violation \mathrm rightarrow ksksks pm0 clean-token ` + "\ufffd")
	forbidden := map[string]bool{
		"mathrm":     true,
		"rightarrow": true,
		"ksksks":     true,
		"pm0":        true,
		"\ufffd":     true,
	}
	for _, term := range got {
		if forbidden[term] {
			t.Fatalf("extractMapTerms kept noisy term %q in %#v", term, got)
		}
	}
}

func TestBuildSemanticMapUsesCompactPointMetadata(t *testing.T) {
	rows := []semanticVectorRow{
		{PaperID: "paper-a", Similarity: 1, Vector: []float64{1, 0, 0}, Anchor: true},
		{PaperID: "paper-b", Similarity: 0.9, Vector: []float64{0.9, 0.1, 0}, Anchor: false},
		{PaperID: "paper-c", Similarity: 0.8, Vector: []float64{0.1, 0.9, 0}, Anchor: false},
	}
	paperMap := map[string]*Paper{
		"paper-a": {ID: "paper-a", Title: "Anchor paper", Authors: "A. Author", Categories: "hep-ph"},
		"paper-b": {ID: "paper-b", Title: "Nearby paper", Authors: "B. Author", Categories: "hep-ph"},
		"paper-c": {ID: "paper-c", Title: "Other paper", Authors: "C. Author", Categories: "cs.AI"},
	}

	semanticMap := buildSemanticMap(rows, paperMap)
	if len(semanticMap.Points) != len(rows) {
		t.Fatalf("points = %d, want %d", len(semanticMap.Points), len(rows))
	}
	if !semanticMap.Points[0].Anchor || semanticMap.Points[0].PaperID != "paper-a" {
		t.Fatalf("first point should be anchor paper: %#v", semanticMap.Points[0])
	}
	if semanticMap.Points[0].Title != "Anchor paper" || semanticMap.Points[0].Categories != "hep-ph" {
		t.Fatalf("point metadata not populated: %#v", semanticMap.Points[0])
	}
	if semanticMap.Dimensions != 3 {
		t.Fatalf("dimensions = %d, want 3", semanticMap.Dimensions)
	}
	if len(semanticMap.Links) == 0 {
		t.Fatal("expected nearest-neighbor links")
	}
}
