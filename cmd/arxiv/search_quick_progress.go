package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/lantos1618/arxiv.gg"
)

func awaitQuickSearchWithProgress(ctx context.Context, search func(context.Context) ([]arxiv.Paper, error), progress func()) ([]arxiv.Paper, error) {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	return awaitQuickSearchWithProgressTimer(ctx, search, progress, timer.C)
}

// The caller owns all progress writes. The search worker only returns data, so
// progress and the caller's eventual result/error writes cannot overlap.
func awaitQuickSearchWithProgressTimer(ctx context.Context, search func(context.Context) ([]arxiv.Paper, error), progress func(), timer <-chan time.Time) ([]arxiv.Paper, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	searchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		papers []arxiv.Paper
		err    error
	}
	// A canceled caller need not wait for the database driver to finish unwinding.
	// Its eventual result must not leave the worker blocked on a channel send.
	results := make(chan result, 1)
	go func() {
		papers, err := search(searchCtx)
		results <- result{papers: papers, err: err}
	}()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case found := <-results:
			return found.papers, found.err
		case <-timer:
			timer = nil // A search emits at most one progress notice.
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			// Avoid a redundant notice if completion was already queued when the
			// timer fired. This preserves the search's result/error unchanged.
			select {
			case found := <-results:
				return found.papers, found.err
			default:
				if progress != nil {
					progress()
				}
			}
		}
	}
}

func writeQuickSearchProgress(w io.Writer, flusher http.Flusher) {
	fmt.Fprintf(w, "data: %s\n\n", toJSON(map[string]interface{}{
		"type":    "status",
		"message": "Finding matches. This search is taking a little longer.",
	}))
	flusher.Flush()
}
