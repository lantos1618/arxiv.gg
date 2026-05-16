package arxiv

import (
	"bytes"
	"testing"
	"time"
)

func TestBuildSitemapIndexXML(t *testing.T) {
	lastMod := time.Date(2026, 5, 16, 7, 0, 0, 0, time.UTC)

	data, err := BuildSitemapIndexXML(SitemapIndex{
		{Loc: "https://arxiv.gg/sitemap-static.xml", LastMod: &lastMod},
		{Loc: "https://arxiv.gg/sitemaps/papers-1.xml"},
		{Loc: "https://arxiv.gg/sitemaps/papers-1.xml"},
	})
	if err != nil {
		t.Fatalf("BuildSitemapIndexXML returned error: %v", err)
	}

	checks := [][]byte{
		[]byte(`<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`),
		[]byte(`<loc>https://arxiv.gg/sitemap-static.xml</loc>`),
		[]byte(`<lastmod>2026-05-16T07:00:00Z</lastmod>`),
		[]byte(`<loc>https://arxiv.gg/sitemaps/papers-1.xml</loc>`),
	}
	for _, check := range checks {
		if !bytes.Contains(data, check) {
			t.Fatalf("sitemap index missing %q:\n%s", check, data)
		}
	}

	if count := bytes.Count(data, []byte(`<loc>https://arxiv.gg/sitemaps/papers-1.xml</loc>`)); count != 1 {
		t.Fatalf("expected duplicate sitemap entry to be removed, got %d copies", count)
	}
}
