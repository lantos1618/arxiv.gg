package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/lantos1618/arxiv.gg"
)

func TestQuickSearchProgressWaitsFiveSecondsAndEmitsOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		done := make(chan error, 1)
		var calls atomic.Int32
		notices := make(chan time.Time, 2)
		started := time.Now()
		go func() {
			papers, err := awaitQuickSearchWithProgress(t.Context(), func(context.Context) ([]arxiv.Paper, error) {
				calls.Add(1)
				<-release
				return []arxiv.Paper{{ID: "1706.03762"}}, nil
			}, func() { notices <- time.Now() })
			if err == nil && (len(papers) != 1 || papers[0].ID != "1706.03762") {
				err = errors.New("search result changed")
			}
			done <- err
		}()
		synctest.Wait()
		time.Sleep(5*time.Second - time.Nanosecond)
		synctest.Wait()
		if len(notices) != 0 {
			t.Fatal("progress was sent before five seconds")
		}
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		if len(notices) != 1 {
			t.Fatalf("notices=%d, want one after five seconds", len(notices))
		}
		if elapsed := (<-notices).Sub(started); elapsed != 5*time.Second {
			t.Fatalf("progress at %s, want five seconds", elapsed)
		}
		time.Sleep(20 * time.Second)
		synctest.Wait()
		if len(notices) != 0 || calls.Load() != 1 {
			t.Fatalf("extra notices=%d search calls=%d, want zero and one", len(notices), calls.Load())
		}
		close(release)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}

func TestQuickSearchProgressPayloadIsPlainStatus(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ticks := make(chan time.Time, 2)
		release := make(chan struct{})
		done := make(chan error, 1)
		recorder := httptest.NewRecorder()
		go func() {
			_, err := awaitQuickSearchWithProgressTimer(t.Context(), func(context.Context) ([]arxiv.Paper, error) {
				<-release
				return nil, nil
			}, func() { writeQuickSearchProgress(recorder, recorder) }, ticks)
			done <- err
		}()
		synctest.Wait()
		ticks <- time.Now()
		synctest.Wait()
		if !recorder.Flushed {
			t.Fatal("progress must be flushed to the browser")
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(recorder.Body.String(), "data: ")), &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload) != 2 || payload["type"] != "status" || payload["message"] != "Finding matches. This search is taking a little longer." {
			t.Fatalf("progress must contain only status/message, without queued/retry: %v", payload)
		}
		ticks <- time.Now()
		synctest.Wait()
		if strings.Count(recorder.Body.String(), "data: ") != 1 {
			t.Fatal("multiple timer ticks emitted duplicate status")
		}
		close(release)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}

func TestQuickSearchProgressFastResultAndErrorSkipNotice(t *testing.T) {
	for _, searchErr := range []error{nil, errors.New("search failed")} {
		synctest.Test(t, func(t *testing.T) {
			var calls, notices int
			papers, err := awaitQuickSearchWithProgress(t.Context(), func(context.Context) ([]arxiv.Paper, error) {
				calls++
				return []arxiv.Paper{{ID: "1706.03762"}}, searchErr
			}, func() { notices++ })
			if !errors.Is(err, searchErr) || len(papers) != 1 || papers[0].ID != "1706.03762" || calls != 1 || notices != 0 {
				t.Fatalf("papers=%v err=%v calls=%d notices=%d", papers, err, calls, notices)
			}
		})
	}
}

func TestQuickSearchProgressCancellationReachesSearch(t *testing.T) {
	for _, afterNotice := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			ticks := make(chan time.Time, 1)
			searchCanceled := make(chan struct{})
			done := make(chan error, 1)
			var notices int
			go func() {
				_, err := awaitQuickSearchWithProgressTimer(ctx, func(searchCtx context.Context) ([]arxiv.Paper, error) {
					<-searchCtx.Done()
					close(searchCanceled)
					return nil, searchCtx.Err()
				}, func() { notices++ }, ticks)
				done <- err
			}()
			synctest.Wait()
			if afterNotice {
				ticks <- time.Now()
				synctest.Wait()
			}
			cancel()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel returned %v", err)
			}
			<-searchCanceled
			synctest.Wait()
			wantNotices := 0
			if afterNotice {
				wantNotices = 1
			}
			if notices != wantNotices {
				t.Fatalf("notices=%d, want %d", notices, wantNotices)
			}
		})
	}
}

func TestQuickSearchProgressLateResultDoesNotLeakWorker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		release := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			_, err := awaitQuickSearchWithProgressTimer(ctx, func(context.Context) ([]arxiv.Paper, error) {
				// Model a driver that takes time to finish after cancellation.
				<-release
				return nil, errors.New("late driver error")
			}, func() { t.Error("unexpected progress") }, nil)
			done <- err
		}()
		synctest.Wait()
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel returned %v", err)
		}
		close(release)
		// The bubble fails if the late result leaves a sender goroutine blocked.
	})
}

func TestQuickSearchProgressCanceledContextSkipsSearch(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := awaitQuickSearchWithProgress(ctx, func(context.Context) ([]arxiv.Paper, error) {
		t.Error("search started after cancellation")
		return nil, nil
	}, func() { t.Error("progress emitted after cancellation") })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel returned %v", err)
	}
}
