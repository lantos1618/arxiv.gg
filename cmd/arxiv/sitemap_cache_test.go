package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func sitemapTestRequest(t *testing.T, client *http.Client, method, url string, headers http.Header) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header = headers
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, body
}

func TestSitemapHTTPCachesLargeGETAndHEAD(t *testing.T) {
	for _, firstMethod := range []string{http.MethodGet, http.MethodHead} {
		t.Run("cold_"+firstMethod, func(t *testing.T) {
			payload := bytes.Repeat([]byte("x"), (2<<20)+1)
			var builds atomic.Int32
			srv := &server{}
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				srv.serveSitemap(w, r, "papers-1", func(context.Context) ([]byte, error) {
					builds.Add(1)
					return payload, nil
				})
			}))
			defer httpServer.Close()
			client := httpServer.Client()
			client.Timeout = 5 * time.Second
			var etag, modified string
			for _, method := range []string{firstMethod, http.MethodGet, http.MethodHead} {
				resp, body := sitemapTestRequest(t, client, method, httpServer.URL, nil)
				if resp.StatusCode != http.StatusOK || resp.ContentLength != int64(len(payload)) {
					t.Fatalf("%s: status=%d length=%d", method, resp.StatusCode, resp.ContentLength)
				}
				if method == http.MethodGet && !bytes.Equal(body, payload) || method == http.MethodHead && len(body) != 0 {
					t.Fatalf("%s returned incorrect body length %d", method, len(body))
				}
				if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/xml") {
					t.Fatalf("Content-Type=%q", resp.Header.Get("Content-Type"))
				}
				maxAge, err := strconv.Atoi(strings.TrimPrefix(resp.Header.Get("Cache-Control"), "public, max-age="))
				if err != nil || maxAge <= 0 || maxAge > 3600 {
					t.Fatalf("Cache-Control=%q", resp.Header.Get("Cache-Control"))
				}
				if etag == "" {
					etag, modified = resp.Header.Get("ETag"), resp.Header.Get("Last-Modified")
					if etag == "" || modified == "" {
						t.Fatal("missing validators on cold response")
					}
				} else if resp.Header.Get("ETag") != etag || resp.Header.Get("Last-Modified") != modified {
					t.Fatal("validators changed between GET and HEAD for the cached representation")
				}
			}
			for _, test := range []struct {
				name    string
				method  string
				headers http.Header
				status  int
			}{
				{"matching_etag", http.MethodGet, http.Header{"If-None-Match": {etag}}, http.StatusNotModified},
				{"head_matching_etag", http.MethodHead, http.Header{"If-None-Match": {etag}}, http.StatusNotModified},
				{"weak_matching_etag", http.MethodGet, http.Header{"If-None-Match": {"W/" + etag}}, http.StatusNotModified},
				{"modified_since", http.MethodGet, http.Header{"If-Modified-Since": {modified}}, http.StatusNotModified},
				{"matching_etag_precedes_old_date", http.MethodGet, http.Header{"If-None-Match": {etag}, "If-Modified-Since": {time.Unix(0, 0).UTC().Format(http.TimeFormat)}}, http.StatusNotModified},
				{"different_etag_precedes_matching_date", http.MethodGet, http.Header{"If-None-Match": {`"different"`}, "If-Modified-Since": {modified}}, http.StatusOK},
			} {
				t.Run(test.name, func(t *testing.T) {
					resp, body := sitemapTestRequest(t, client, test.method, httpServer.URL, test.headers)
					if resp.StatusCode != test.status {
						t.Fatalf("status=%d, want %d", resp.StatusCode, test.status)
					}
					if test.status == http.StatusNotModified && len(body) != 0 {
						t.Fatalf("304 returned %d body bytes", len(body))
					}
					if test.status == http.StatusOK && !bytes.Equal(body, payload) {
						t.Fatal("validator mismatch did not return the full representation")
					}
				})
			}
			if builds.Load() != 1 {
				t.Fatalf("large GET/HEAD and conditional requests built %d times, want 1", builds.Load())
			}
		})
	}
}

func TestSitemapConcurrentHTTPGETAndHEADShareBuild(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	const requests = 12
	entered := make(chan struct{}, requests)
	start := make(chan struct{})
	building := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var builds atomic.Int32
	srv := &server{}
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		select {
		case <-start:
		case <-ctx.Done():
			return
		}
		srv.serveSitemap(w, r, "shared", func(buildCtx context.Context) ([]byte, error) {
			builds.Add(1)
			once.Do(func() { close(building) })
			select {
			case <-release:
				return []byte("<urlset/>"), nil
			case <-buildCtx.Done():
				return nil, buildCtx.Err()
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
	}))
	defer httpServer.Close()
	client := httpServer.Client()
	client.Timeout = 5 * time.Second
	errs := make(chan error, requests)
	for i := 0; i < requests; i++ {
		method := http.MethodGet
		if i%2 == 0 {
			method = http.MethodHead
		}
		go func() {
			req, err := http.NewRequestWithContext(ctx, method, httpServer.URL, nil)
			if err != nil {
				errs <- err
				return
			}
			resp, err := client.Do(req)
			if err == nil {
				_, err = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					err = fmt.Errorf("%s status=%d", method, resp.StatusCode)
				}
			}
			errs <- err
		}()
	}
	for i := 0; i < requests; i++ {
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("requests did not arrive before deadline")
		}
	}
	close(start)
	select {
	case <-building:
	case <-ctx.Done():
		t.Fatal("build did not start")
	}
	close(release)
	for i := 0; i < requests; i++ {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
	if builds.Load() != 1 {
		t.Fatalf("concurrent GET/HEAD requests built %d times, want 1", builds.Load())
	}
}

func TestSitemapCanceledReaderDoesNotCancelSharedBuild(t *testing.T) {
	for _, cancelStarter := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel_starter_%t", cancelStarter), func(t *testing.T) {
			ctx, stop := context.WithTimeout(t.Context(), 5*time.Second)
			defer stop()
			canceledCtx, cancelReader := context.WithCancel(ctx)
			defer cancelReader()
			var cache sitemapResponseCache
			var calls atomic.Int32
			started := make(chan context.Context, 1)
			release := make(chan struct{})
			build := func(buildCtx context.Context) ([]byte, error) {
				calls.Add(1)
				started <- buildCtx
				select {
				case <-release:
					return []byte("ready"), nil
				case <-buildCtx.Done():
					return nil, buildCtx.Err()
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			canceledResult, activeResult := make(chan error, 1), make(chan error, 1)
			get := func(requestCtx context.Context, result chan<- error) {
				doc, err := cache.get(requestCtx, "shared", build)
				if err == nil && string(doc.body) != "ready" {
					err = fmt.Errorf("unexpected body %q", doc.body)
				}
				result <- err
			}
			if cancelStarter {
				go get(canceledCtx, canceledResult)
			} else {
				go get(ctx, activeResult)
			}
			var buildCtx context.Context
			select {
			case buildCtx = <-started:
			case <-ctx.Done():
				t.Fatal("build did not start")
			}
			if cancelStarter {
				go get(ctx, activeResult)
			} else {
				go get(canceledCtx, canceledResult)
			}
			cancelReader()
			select {
			case err := <-canceledResult:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("canceled reader error=%v", err)
				}
			case <-ctx.Done():
				t.Fatal("canceled reader remained blocked")
			}
			if buildCtx.Err() != nil {
				t.Fatalf("reader canceled shared build: %v", buildCtx.Err())
			}
			close(release)
			select {
			case err := <-activeResult:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("active reader never received shared result")
			}
			if calls.Load() != 1 {
				t.Fatalf("built %d times, want 1", calls.Load())
			}
		})
	}
}

func TestSitemapCacheRetriesErrorsAndRefreshesExpiredDocuments(t *testing.T) {
	var cache sitemapResponseCache
	ctx := t.Context()
	failure := errors.New("temporary fixture failure")
	calls := 0
	build := func(context.Context) ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, failure
		}
		return []byte(fmt.Sprintf("document-%d", calls)), nil
	}
	if _, err := cache.get(ctx, "retry", build); !errors.Is(err, failure) {
		t.Fatalf("initial error=%v, want fixture failure", err)
	}
	first, err := cache.get(ctx, "retry", build)
	if err != nil || string(first.body) != "document-2" {
		t.Fatalf("retry failed: document=%v err=%v", first, err)
	}
	if _, err := cache.get(ctx, "retry", build); err != nil || calls != 2 {
		t.Fatalf("successful retry was not cached: calls=%d err=%v", calls, err)
	}
	cache.mu.Lock()
	cache.entries["retry"].expiresAt = time.Now().Add(-time.Second)
	cache.mu.Unlock()
	refreshed, err := cache.get(ctx, "retry", build)
	if err != nil || string(refreshed.body) != "document-3" || refreshed.etag == first.etag {
		t.Fatalf("expired document did not refresh: document=%v err=%v", refreshed, err)
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.bytesUsed != int64(len(refreshed.body)) || len(cache.entries) != 1 {
		t.Fatalf("expired body retained in accounting: bytes=%d entries=%d", cache.bytesUsed, len(cache.entries))
	}
}

func TestSitemapCacheMemoryAndEntryBudgets(t *testing.T) {
	t.Run("bytes_and_lru", func(t *testing.T) {
		var cache sitemapResponseCache
		// Share an immutable fixture allocation; every key still represents a
		// full-size sitemap for the cache's retained-byte accounting.
		body := bytes.Repeat([]byte("x"), sitemapCacheMaxItemSize)
		build := func(context.Context) ([]byte, error) { return body, nil }
		for i := 0; i < 5; i++ {
			if _, err := cache.get(t.Context(), fmt.Sprint(i), build); err != nil {
				t.Fatal(err)
			}
		}
		cache.mu.Lock()
		for i := 0; i < 5; i++ {
			cache.entries[fmt.Sprint(i)].lastUsed = time.Unix(int64(i), 0)
		}
		cache.mu.Unlock()
		if _, err := cache.get(t.Context(), "0", build); err != nil {
			t.Fatal(err)
		}
		if _, err := cache.get(t.Context(), "5", build); err != nil {
			t.Fatal(err)
		}
		cache.mu.Lock()
		defer cache.mu.Unlock()
		if cache.bytesUsed > sitemapCacheMaxBytes || len(cache.entries) != 5 || cache.entries["1"] != nil || cache.entries["0"] == nil {
			t.Fatalf("memory/LRU eviction failed: bytes=%d entries=%d", cache.bytesUsed, len(cache.entries))
		}
	})
	t.Run("entry_count", func(t *testing.T) {
		var cache sitemapResponseCache
		for i := 0; i <= sitemapCacheMaxEntries; i++ {
			if _, err := cache.get(t.Context(), fmt.Sprint(i), func(context.Context) ([]byte, error) { return []byte("x"), nil }); err != nil {
				t.Fatal(err)
			}
		}
		cache.mu.Lock()
		defer cache.mu.Unlock()
		if len(cache.entries) != sitemapCacheMaxEntries || cache.bytesUsed != int64(sitemapCacheMaxEntries) {
			t.Fatalf("entry budget failed: entries=%d bytes=%d", len(cache.entries), cache.bytesUsed)
		}
	})
	t.Run("oversized_served_without_retaining", func(t *testing.T) {
		var cache sitemapResponseCache
		body := bytes.Repeat([]byte("x"), sitemapCacheMaxItemSize+1)
		calls := 0
		for i := 0; i < 2; i++ {
			doc, err := cache.get(t.Context(), "oversized", func(context.Context) ([]byte, error) { calls++; return body, nil })
			if err != nil || !bytes.Equal(doc.body, body) {
				t.Fatalf("oversized document was truncated or rejected: err=%v", err)
			}
		}
		if calls != 2 {
			t.Fatalf("oversized document was retained: builds=%d", calls)
		}
	})
}

func TestSitemapBuildConcurrencyIsBoundedAcrossKeys(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var cache sitemapResponseCache
	const requests = 6
	started := make(chan struct{}, requests)
	release := make(chan struct{}, requests)
	errs := make(chan error, requests)
	var active, peak atomic.Int32
	build := func(buildCtx context.Context) ([]byte, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for previous := peak.Load(); current > previous; previous = peak.Load() {
			if peak.CompareAndSwap(previous, current) {
				break
			}
		}
		started <- struct{}{}
		select {
		case <-release:
			return []byte("ready"), nil
		case <-buildCtx.Done():
			return nil, buildCtx.Err()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	for i := 0; i < requests; i++ {
		go func() {
			_, err := cache.get(ctx, fmt.Sprint(i), build)
			errs <- err
		}()
	}
	for i := 0; i < sitemapMaxConcurrentBuilds; i++ {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("initial builds did not start")
		}
	}
	release <- struct{}{}
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("queued build did not start after releasing a slot")
	}
	for i := 1; i < requests; i++ {
		release <- struct{}{}
	}
	for i := 0; i < requests; i++ {
		select {
		case err := <-errs:
			if err != nil {
				t.Error(err)
			}
		case <-ctx.Done():
			t.Fatal("queued builds did not finish")
		}
	}
	if got := peak.Load(); got != sitemapMaxConcurrentBuilds {
		t.Fatalf("peak concurrent builds=%d, want %d", got, sitemapMaxConcurrentBuilds)
	}
}
