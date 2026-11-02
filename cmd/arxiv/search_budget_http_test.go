package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lantos1618/arxiv.gg"
)

func TestQuickHTTPAuthorLookupWaitsForBudgetedCapacity(t *testing.T) {
	// A zero cache deliberately has no database: queued requests must respect
	// cancellation before trying even their preliminary author lookup.
	cache := &arxiv.Cache{}
	s := &server{cache: cache}
	acquired := make(chan struct{}, 2)
	release := make(chan struct{})
	finished := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			finished <- cache.WithQuickSearch(t.Context(), func(ctx context.Context) error {
				acquired <- struct{}{}
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
		}()
	}
	<-acquired
	<-acquired
	defer func() {
		close(release)
		for i := 0; i < 2; i++ {
			if err := <-finished; err != nil {
				t.Error(err)
			}
		}
	}()
	for _, tc := range []struct {
		name, path string
		handler    http.HandlerFunc
		status     int
	}{
		{"API", "/api/v1/search/quick?q=Ada+Lovelace", s.handleAPISearchQuick, http.StatusInternalServerError},
		{"page", "/search?q=Ada+Lovelace&mode=quick&static=1", s.handleSearch, http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
			defer cancel()
			request := httptest.NewRequest(http.MethodGet, tc.path, nil).WithContext(ctx)
			response := httptest.NewRecorder()
			tc.handler(response, request)
			if response.Code != tc.status || ctx.Err() != context.DeadlineExceeded {
				t.Fatalf("queued request: status=%d context=%v body=%q", response.Code, ctx.Err(), response.Body.String())
			}
		})
	}
}
