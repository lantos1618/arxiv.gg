package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	arxiv "github.com/lantos1618/arxiv.gg"
)

func TestResponseCacheDoesNotLeakAPIKeyIdentity(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	cache, err := arxiv.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	user, err := cache.FindOrCreateUser(t.Context(), "private-reader@example.com", "Reader", "", true, "google", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := cache.CreateUserAPIKey(t.Context(), user.ID, "Agent")
	if err != nil {
		t.Fatal(err)
	}
	srv := &server{cache: cache}
	cm := newCacheMiddleware(time.Minute)
	handler := cm.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.renderTemplate(w, r, "nav", map[string]any{})
	}))
	authenticated := httptest.NewRequest(http.MethodGet, "/", nil)
	authenticated.Header.Set("X-API-Key", key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authenticated)
	if !strings.Contains(rec.Body.String(), user.Email) {
		t.Fatal("fixture did not render authenticated identity")
	}
	public := httptest.NewRecorder()
	handler.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/", nil))
	if strings.Contains(public.Body.String(), user.Email) {
		t.Fatal("anonymous visitor received cached API-key user's email")
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("authenticated Cache-Control = %q", got)
	}
}

func TestResponseCacheHonorsUnshareableResponses(t *testing.T) {
	for _, headers := range []http.Header{
		{"Cache-Control": {"private, max-age=60"}},
		{"Cache-Control": {"no-store"}},
		{"Cache-Control": {"no-cache"}},
		{"Set-Cookie": {"session=new; HttpOnly"}},
		{"Vary": {"Authorization"}},
		{"Vary": {"*"}},
	} {
		t.Run(fmt.Sprint(headers), func(t *testing.T) {
			calls := 0
			handler := newCacheMiddleware(time.Minute).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				copyHeader(w.Header(), headers)
				fmt.Fprintf(w, "response %d", calls)
			}))
			for i := 0; i < 2; i++ {
				handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
			}
			if calls != 2 {
				t.Fatalf("unshareable response reused; handler calls=%d", calls)
			}
		})
	}
}

func TestAuthenticatedQueryPageIsPrivate(t *testing.T) {
	handler := newCacheMiddleware(time.Minute).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "personalized") }))
	req := httptest.NewRequest(http.MethodGet, "/?utm_source=example", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "signed-in"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("query page Cache-Control=%q", got)
	}
}
func TestRateLimiterResetsWindowDuringContinuousTraffic(t *testing.T) {
	limiter := newRateLimiter(2, time.Minute, false)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if !limiter.Allow(req) || !limiter.Allow(req) || limiter.Allow(req) {
		t.Fatal("initial request quota not enforced")
	}
	// A request accepted late in the old window must not extend that window.
	limiter.mu.Lock()
	limiter.visitors[limiter.clientIP(req)].windowStart = time.Now().Add(-2 * time.Minute)
	limiter.visitors[limiter.clientIP(req)].lastSeen = time.Now()
	limiter.mu.Unlock()
	if !limiter.Allow(req) {
		t.Fatal("active visitor was not given a fresh window")
	}
	if !limiter.Allow(req) || limiter.Allow(req) {
		t.Fatal("fresh window quota not enforced")
	}
}

func TestHTMLSearchRejectsOversizedQueryBeforeDatabaseWork(t *testing.T) {
	rec := httptest.NewRecorder()
	(&server{}).handleSearch(rec, httptest.NewRequest(http.MethodGet, "/search?q="+strings.Repeat("x", maxSearchQueryBytes+1), nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversized query status=%d", rec.Code)
	}
}

func TestResponseSettingCookieOverridesPublicCachePolicy(t *testing.T) {
	handler := newCacheMiddleware(time.Minute).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "private", HttpOnly: true})
		fmt.Fprint(w, "personalized")
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("cookie response Cache-Control=%q", got)
	}
}

func TestAuthenticatedStreamAndAuthorPagesArePrivate(t *testing.T) {
	for _, path := range []string{"/api/v1/search/stream", "/author/stream-reader", "/author/generate-reader"} {
		handler := newCacheMiddleware(time.Minute).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "personalized") }))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Key", "credential")
		handler.ServeHTTP(rec, req)
		if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Fatalf("%s Cache-Control=%q", path, got)
		}
	}
}
