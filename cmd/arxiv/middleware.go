package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// cacheEntry holds cached response data
type cacheEntry struct {
	etag      string
	lastMod   time.Time
	data      []byte
	headers   http.Header
	expiresAt time.Time
}

// cacheMiddleware provides HTTP caching with ETag support
type cacheMiddleware struct {
	cache      map[string]*cacheEntry
	mu         sync.RWMutex
	maxAge     time.Duration
	cleanupInt time.Duration
}

// newCacheMiddleware creates a new cache middleware
func newCacheMiddleware(maxAge time.Duration) *cacheMiddleware {
	cm := &cacheMiddleware{
		cache:      make(map[string]*cacheEntry),
		maxAge:     maxAge,
		cleanupInt: time.Hour,
	}
	go cm.cleanup()
	return cm
}

// cleanup removes expired entries periodically
func (cm *cacheMiddleware) cleanup() {
	ticker := time.NewTicker(cm.cleanupInt)
	defer ticker.Stop()
	for range ticker.C {
		cm.mu.Lock()
		now := time.Now()
		for key, entry := range cm.cache {
			if now.After(entry.expiresAt) {
				delete(cm.cache, key)
			}
		}
		cm.mu.Unlock()
	}
}

// generateETag creates an ETag from data
func generateETag(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf(`"%x"`, hash[:16])
}

// Handler wraps an http.Handler with caching
func (cm *cacheMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only cache GET requests
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		// SSE endpoints require http.Flusher - skip caching wrapper
		if strings.Contains(r.URL.Path, "/stream") || strings.Contains(r.URL.Path, "/generate") {
			next.ServeHTTP(w, r)
			return
		}

		if shouldBypassResponseCache(r) {
			next.ServeHTTP(w, r)
			return
		}

		key := r.URL.Path
		if r.URL.RawQuery != "" {
			key += "?" + r.URL.RawQuery
		}

		// Check for a valid cached response
		cm.mu.RLock()
		entry, exists := cm.cache[key]
		cm.mu.RUnlock()

		if exists && time.Now().Before(entry.expiresAt) {
			// Conditional request - return 304 Not Modified
			if r.Header.Get("If-None-Match") == entry.etag {
				w.Header().Set("ETag", entry.etag)
				w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(cm.maxAge.Seconds())))
				w.WriteHeader(http.StatusNotModified)
				return
			}

			// Serve directly from cache
			copyHeader(w.Header(), entry.headers)
			w.Header().Set("ETag", entry.etag)
			w.Header().Set("Last-Modified", entry.lastMod.Format(http.TimeFormat))
			if w.Header().Get("Cache-Control") == "" {
				w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(cm.maxAge.Seconds())))
			}
			w.Write(entry.data)
			return
		}

		// Cache miss - capture response while writing to client
		cw := &cachingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(cw, r)

		// Store successful responses in cache for future requests
		if cw.statusCode == http.StatusOK && len(cw.data) > 0 {
			etag := generateETag(cw.data)
			now := time.Now()
			headers := cw.Header().Clone()
			if headers.Get("Content-Type") == "" {
				headers.Set("Content-Type", detectContentType(cw.data))
			}

			cm.mu.Lock()
			cm.cache[key] = &cacheEntry{
				etag:      etag,
				lastMod:   now,
				data:      cw.data,
				headers:   headers,
				expiresAt: now.Add(cm.maxAge),
			}
			cm.mu.Unlock()
		}
	})
}

func shouldBypassResponseCache(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/admin") {
		return true
	}
	if r.URL.Query().Get("admin_token") != "" ||
		r.Header.Get("Authorization") != "" ||
		r.Header.Get("X-Admin-Token") != "" {
		return true
	}
	if _, err := r.Cookie(adminCookieName); err == nil {
		return true
	}
	return false
}

func copyHeader(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func detectContentType(data []byte) string {
	trimmed := bytes.TrimSpace(data)
	if bytes.HasPrefix(trimmed, []byte("<!DOCTYPE html")) ||
		bytes.HasPrefix(trimmed, []byte("<html")) {
		return "text/html; charset=utf-8"
	}
	return http.DetectContentType(data)
}

// cachingResponseWriter captures response data
type cachingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	data       []byte
}

func (cw *cachingResponseWriter) WriteHeader(code int) {
	cw.statusCode = code
	cw.ResponseWriter.WriteHeader(code)
}

func (cw *cachingResponseWriter) Write(b []byte) (int, error) {
	cw.data = append(cw.data, b...)
	return cw.ResponseWriter.Write(b)
}

// rateLimiter provides simple rate limiting
type rateLimiter struct {
	visitors          map[string]*visitor
	mu                sync.RWMutex
	rate              int           // requests per window
	window            time.Duration // time window
	trustProxyHeaders bool
}

type visitor struct {
	count    int
	lastSeen time.Time
}

// newRateLimiter creates a new rate limiter
func newRateLimiter(rate int, window time.Duration, trustProxyHeaders bool) *rateLimiter {
	rl := &rateLimiter{
		visitors:          make(map[string]*visitor),
		rate:              rate,
		window:            window,
		trustProxyHeaders: trustProxyHeaders,
	}
	go rl.cleanup()
	return rl
}

// cleanup removes old visitors periodically
func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, v := range rl.visitors {
			if now.Sub(v.lastSeen) > rl.window*2 {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Handler wraps an http.Handler with rate limiting
func (rl *rateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bypass rate limiting for SSE endpoints to support streaming
		if strings.Contains(r.URL.Path, "/stream") {
			next.ServeHTTP(w, r)
			return
		}
		if !rl.Allow(r) {
			writeRateLimitExceeded(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *rateLimiter) Allow(r *http.Request) bool {
	ip := rl.clientIP(r)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	now := time.Now()

	if !exists || now.Sub(v.lastSeen) > rl.window {
		rl.visitors[ip] = &visitor{count: 1, lastSeen: now}
		return true
	}

	if v.count >= rl.rate {
		return false
	}

	v.count++
	v.lastSeen = now
	return true
}

func writeRateLimitExceeded(w http.ResponseWriter, r *http.Request) {
	// Render a friendlier rate limit response.
	// For API routes, return structured JSON.
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "rate limit exceeded - please slow down and retry in a moment",
		})
		return
	}

	// For HTML routes, render an error template if available.
	w.WriteHeader(http.StatusTooManyRequests)
	data := map[string]any{
		"Title":   "Too Many Requests",
		"Message": "You've hit the per-IP rate limit. Please wait a bit and try again.",
	}
	if err := templates.ExecuteTemplate(w, "error", data); err != nil {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
	}
}

func (rl *rateLimiter) clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if parsedHost, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = parsedHost
	}

	if !rl.trustProxyHeaders {
		return host
	}

	if cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
		if net.ParseIP(cfIP) != nil {
			return cfIP
		}
	}

	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	return host
}
