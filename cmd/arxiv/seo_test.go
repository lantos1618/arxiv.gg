package main

import (
	"strings"
	"testing"
)

func TestCategoryMetadata(t *testing.T) {
	title := categoryTitle("cond-mat")
	if title != "arXiv cond-mat papers: condensed matter" {
		t.Fatalf("unexpected category title: %q", title)
	}

	description := categoryDescription("cs.LG", nil)
	for _, want := range []string{"cs.LG", "machine learning", "semantic search"} {
		if !strings.Contains(description, want) {
			t.Fatalf("description %q missing %q", description, want)
		}
	}
}

func TestIndexNowKeyValidation(t *testing.T) {
	for _, key := range []string{"34af0c26368622541e3ca8aa555c3ad7", "indexnow-key_2026"} {
		if !isSafeIndexNowKey(key) {
			t.Fatalf("expected key %q to be accepted", key)
		}
	}
	for _, key := range []string{"short", "has/slash", "has.dot", "has space"} {
		if isSafeIndexNowKey(key) {
			t.Fatalf("expected key %q to be rejected", key)
		}
	}
}
