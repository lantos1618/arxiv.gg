package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	sitemapCacheTTL            = time.Hour
	sitemapCacheMaxBytes       = 64 << 20
	sitemapCacheMaxItemSize    = 12 << 20
	sitemapCacheMaxEntries     = 128
	sitemapBuildTimeout        = 20 * time.Second
	sitemapMaxConcurrentBuilds = 2
)

// Sitemap documents are public and independent of cookies and authentication.
// Keep their larger bodies outside the cache used by personalized HTML pages.
type sitemapDocument struct {
	body        []byte
	etag        string
	generatedAt time.Time
	expiresAt   time.Time
	lastUsed    time.Time // Protected by the cache mutex; not an HTTP modification time.
}

type sitemapResponseCache struct {
	once      sync.Once
	mu        sync.Mutex
	entries   map[string]*sitemapDocument
	bytesUsed int64
	flights   singleflight.Group
	slots     chan struct{}
}

func isSitemapPath(path string) bool {
	return path == "/sitemap.xml" || path == "/sitemap-static.xml" || strings.HasPrefix(path, "/sitemaps/")
}

func (c *sitemapResponseCache) lookup(key string) *sitemapDocument {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if entry := c.entries[key]; entry != nil {
		if now.Before(entry.expiresAt) {
			entry.lastUsed = now
			return entry
		}
		delete(c.entries, key)
		c.bytesUsed -= int64(len(entry.body))
	}
	return nil
}

func (c *sitemapResponseCache) store(key string, entry *sitemapDocument) {
	size := int64(len(entry.body))
	// Serve valid oversized documents in full, but do not retain them in memory.
	if size > sitemapCacheMaxItemSize || size > sitemapCacheMaxBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for existingKey, existing := range c.entries {
		if existingKey == key || !now.Before(existing.expiresAt) {
			delete(c.entries, existingKey)
			c.bytesUsed -= int64(len(existing.body))
		}
	}
	for c.bytesUsed+size > sitemapCacheMaxBytes || len(c.entries) >= sitemapCacheMaxEntries {
		var oldestKey string
		var oldest *sitemapDocument
		for existingKey, existing := range c.entries {
			if oldest == nil || existing.lastUsed.Before(oldest.lastUsed) {
				oldestKey, oldest = existingKey, existing
			}
		}
		if oldest == nil {
			break
		}
		delete(c.entries, oldestKey)
		c.bytesUsed -= int64(len(oldest.body))
	}
	c.entries[key] = entry
	c.bytesUsed += size
}

func (c *sitemapResponseCache) get(ctx context.Context, key string, build func(context.Context) ([]byte, error)) (*sitemapDocument, error) {
	c.once.Do(func() {
		c.entries = make(map[string]*sitemapDocument)
		c.slots = make(chan struct{}, sitemapMaxConcurrentBuilds)
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if entry := c.lookup(key); entry != nil {
		return entry, nil
	}
	result := c.flights.DoChan(key, func() (any, error) {
		if entry := c.lookup(key); entry != nil {
			return entry, nil
		}
		// A disconnected requester must not cancel a shared build for other readers.
		// Bound both queueing and generation, including the database query.
		buildCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sitemapBuildTimeout)
		defer cancel()
		select {
		case c.slots <- struct{}{}:
			defer func() { <-c.slots }()
		case <-buildCtx.Done():
			return nil, buildCtx.Err()
		}
		body, err := build(buildCtx)
		if err != nil {
			return nil, err
		}
		if err := buildCtx.Err(); err != nil {
			return nil, err
		}
		now := time.Now()
		entry := &sitemapDocument{
			body: body, etag: generateETag(body), generatedAt: now,
			expiresAt: now.Add(sitemapCacheTTL), lastUsed: now,
		}
		c.store(key, entry)
		return entry, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-result:
		if result.Err != nil {
			return nil, result.Err
		}
		return result.Val.(*sitemapDocument), nil
	}
}

func (s *server) serveSitemap(w http.ResponseWriter, r *http.Request, key string, build func(context.Context) ([]byte, error)) {
	if rejectNonGetHead(w, r) {
		return
	}
	entry, err := s.sitemapCache.get(r.Context(), key, build)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		status := http.StatusInternalServerError
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusServiceUnavailable
			w.Header().Set("Retry-After", "60")
		}
		writeServerError(w, status, "sitemap unavailable", "build sitemap", err)
		return
	}
	remaining := max(time.Duration(0), time.Until(entry.expiresAt))
	maxAge := int((remaining + time.Second - 1) / time.Second)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))
	w.Header().Set("ETag", entry.etag)
	// ServeContent implements HEAD and HTTP conditional requests against the same
	// complete representation, including If-None-Match precedence and weak tags.
	http.ServeContent(w, r, "sitemap.xml", entry.generatedAt, bytes.NewReader(entry.body))
}

// Canonicalize numeric aliases and reject offsets that would overflow int.
func sitemapPage(path string) (int, bool) {
	name := strings.TrimPrefix(path, "/sitemaps/")
	if !strings.HasPrefix(name, "papers-") || !strings.HasSuffix(name, ".xml") {
		return 0, false
	}
	value := strings.TrimSuffix(strings.TrimPrefix(name, "papers-"), ".xml")
	page, err := strconv.Atoi(value)
	maxInt := int(^uint(0) >> 1)
	if err != nil || page < 1 || page > maxInt/sitemapPaperURLLimit {
		return 0, false
	}
	return page, true
}
