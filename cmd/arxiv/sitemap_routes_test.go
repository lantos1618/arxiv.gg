package main

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	arxiv "github.com/lantos1618/arxiv.gg"
	"gorm.io/gorm"
)

func TestSitemapRoutesPreservePublicXMLAndHeadRepresentation(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SITE_URL", "https://example.test")
	dir := t.TempDir()
	cache, err := arxiv.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	seed, err := gorm.Open(sqlite.Open(filepath.Join(dir, "index.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := seed.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	updated := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := seed.Create(&[]arxiv.Paper{
		{ID: "hep-th/9901001", Title: "Legacy", Categories: "hep-th", Updated: updated},
		{ID: "2501.00001", Title: "Modern", Categories: "cs.AI", Updated: updated},
	}).Error; err != nil {
		t.Fatal(err)
	}
	srv := &server{cache: cache}
	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap.xml", srv.handleSitemap)
	mux.HandleFunc("/sitemap-static.xml", srv.handleStaticSitemap)
	mux.HandleFunc("/sitemaps/", srv.handlePaperSitemap)
	general := newCacheMiddleware(time.Minute)
	handler := general.Handler(mux)

	// A cold HEAD must cache the full public representation. Closing the DB after
	// warming makes subsequent GETs prove they reuse that representation.
	heads := make(map[string]*httptest.ResponseRecorder)
	for _, path := range []string{"/sitemap.xml", "/sitemap-static.xml", "/sitemaps/papers-1.xml"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, path, nil))
		if rec.Code != http.StatusOK || rec.Body.Len() != 0 || rec.Header().Get("ETag") == "" || rec.Header().Get("Last-Modified") == "" {
			t.Fatalf("HEAD %s: status=%d bytes=%d headers=%v", path, rec.Code, rec.Body.Len(), rec.Header())
		}
		heads[path] = rec
	}
	if err := cache.Close(); err != nil {
		t.Fatal(err)
	}
	for path, head := range heads {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "reader-session"})
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.Len() == 0 || rec.Header().Get("ETag") != head.Header().Get("ETag") {
			t.Fatalf("GET after HEAD %s: status=%d bytes=%d", path, rec.Code, rec.Body.Len())
		}
		if !strings.HasPrefix(rec.Header().Get("Cache-Control"), "public, max-age=") {
			t.Fatalf("public sitemap cache headers = %v", rec.Header())
		}
		var doc struct {
			XMLName  xml.Name
			Sitemaps []struct {
				Loc string `xml:"loc"`
			} `xml:"sitemap"`
			URLs []struct {
				Loc string `xml:"loc"`
			} `xml:"url"`
		}
		if err := xml.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		if doc.XMLName.Space != "http://www.sitemaps.org/schemas/sitemap/0.9" {
			t.Fatalf("XML namespace = %q", doc.XMLName.Space)
		}
		if path == "/sitemap.xml" {
			var got []string
			for _, entry := range doc.Sitemaps {
				got = append(got, entry.Loc)
			}
			want := []string{"https://example.test/sitemap-static.xml", "https://example.test/sitemaps/papers-1.xml"}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("index URLs=%v, want %v", got, want)
			}
		} else if path == "/sitemaps/papers-1.xml" {
			var got []string
			for _, entry := range doc.URLs {
				got = append(got, entry.Loc)
			}
			want := []string{"https://example.test/abs/2501.00001", "https://example.test/abs/hep-th/9901001"}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("paper URLs=%v, want %v", got, want)
			}
		} else if len(doc.URLs) < 3 {
			t.Fatalf("static sitemap missing public URLs")
		}
	}
	alias := httptest.NewRecorder()
	handler.ServeHTTP(alias, httptest.NewRequest(http.MethodGet, "/sitemaps/papers-01.xml", nil))
	if alias.Code != http.StatusOK || alias.Header().Get("ETag") != heads["/sitemaps/papers-1.xml"].Header().Get("ETag") {
		t.Fatalf("numeric alias did not share the complete representation: status=%d", alias.Code)
	}
	general.mu.RLock()
	defer general.mu.RUnlock()
	if len(general.cache) != 0 || general.bytesUsed != 0 {
		t.Fatal("sitemap body duplicated into general HTML cache")
	}
}

func TestSitemapRoutesRejectInvalidPagesBeforeDatabaseWork(t *testing.T) {
	srv := &server{} // No database: invalid input must never start a build.
	maxInt := int(^uint(0) >> 1)
	for _, path := range []string{
		"/sitemaps/papers-0.xml", "/sitemaps/papers--1.xml", "/sitemaps/papers-1.xml/extra",
		"/sitemaps/papers-x.xml", "/sitemaps/papers-999999999999999999999.xml",
		fmt.Sprintf("/sitemaps/papers-%d.xml", maxInt),
	} {
		rec := httptest.NewRecorder()
		srv.handlePaperSitemap(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d", path, rec.Code)
		}
	}
	for path, handler := range map[string]http.HandlerFunc{
		"/sitemap.xml":           srv.handleSitemap,
		"/sitemap-static.xml":    srv.handleStaticSitemap,
		"/sitemaps/papers-1.xml": srv.handlePaperSitemap,
	} {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodPost, path, nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s status=%d", path, rec.Code)
		}
	}
}
