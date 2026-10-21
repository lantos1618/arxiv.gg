package arxiv

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestQuickSearchOperationBoundsConcurrencyAndReleasesSlots(t *testing.T) {
	slots := make(chan struct{}, 2)
	entered := make(chan struct{}, 6)
	release := make(chan struct{})
	results := make(chan error, 6)
	var active, peak atomic.Int32
	for i := 0; i < cap(results); i++ {
		go func() {
			results <- withQuickSearchOperation(t.Context(), slots, func(ctx context.Context) error {
				n := active.Add(1)
				defer active.Add(-1)
				for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
				}
				entered <- struct{}{}
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
		}()
	}
	<-entered
	<-entered
	if got := len(slots); got != 2 {
		t.Errorf("two active operations hold %d slots", got)
	}
	close(release)
	for i := 0; i < cap(results); i++ {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
	if peak.Load() != 2 || active.Load() != 0 || len(slots) != 0 {
		t.Fatalf("gate state: peak=%d active=%d held=%d", peak.Load(), active.Load(), len(slots))
	}
	sentinel := errors.New("callback failed")
	if err := withQuickSearchOperation(t.Context(), slots, func(context.Context) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("callback error = %v", err)
	}
	if len(slots) != 0 {
		t.Fatal("failed callback leaked a slot")
	}
}

func TestQuickSearchOperationQueuedCancellationAndDeadline(t *testing.T) {
	for _, kind := range []string{"cancel", "deadline"} {
		t.Run(kind, func(t *testing.T) {
			slots := make(chan struct{}, 2)
			slots <- struct{}{}
			slots <- struct{}{}
			ctx, cancel := context.WithCancel(t.Context())
			want := context.Canceled
			if kind == "deadline" {
				cancel()
				ctx, cancel = context.WithTimeout(t.Context(), 20*time.Millisecond)
				want = context.DeadlineExceeded
			}
			defer cancel()
			started := make(chan struct{})
			result := make(chan error, 1)
			var called atomic.Bool
			go func() {
				close(started)
				result <- withQuickSearchOperation(ctx, slots, func(context.Context) error {
					called.Store(true)
					return nil
				})
			}()
			<-started
			if kind == "cancel" {
				cancel()
			}
			if err := <-result; !errors.Is(err, want) {
				t.Fatalf("queued operation error = %v, want %v", err, want)
			}
			if called.Load() || len(slots) != 2 {
				t.Fatalf("canceled queued operation ran or changed held slots: called=%v slots=%d", called.Load(), len(slots))
			}
			<-slots
			<-slots
			if err := withQuickSearchOperation(t.Context(), slots, func(context.Context) error { return nil }); err != nil || len(slots) != 0 {
				t.Fatalf("gate did not recover: err=%v slots=%d", err, len(slots))
			}
		})
	}
}

func TestQuickSearchOperationNestedCallSharesBudgetAndSlot(t *testing.T) {
	slots := make(chan struct{}, 1)
	parent, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	parentDeadline, _ := parent.Deadline()
	var escaped context.Context
	err := withQuickSearchOperation(parent, slots, func(ctx context.Context) error {
		escaped = ctx
		return withQuickSearchOperation(ctx, slots, func(nested context.Context) error {
			deadline, ok := nested.Deadline()
			if !ok || !deadline.Equal(parentDeadline) || len(slots) != 1 {
				t.Errorf("nested call changed deadline or acquired another slot: deadline=%v slots=%d", deadline, len(slots))
			}
			return nil
		})
	})
	if err != nil || len(slots) != 0 {
		t.Fatalf("nested operation: err=%v slots=%d", err, len(slots))
	}
	if err := withQuickSearchOperation(escaped, slots, func(context.Context) error {
		t.Fatal("expired operation context bypassed the gate")
		return nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("escaped operation context error = %v", err)
	}
}

func TestQuickSearchStatementTimeoutUsesRemainingBudget(t *testing.T) {
	if got, err := quickSearchStatementTimeout(t.Context()); err != nil || got != "30000ms" {
		t.Fatalf("default statement budget = %q, %v", got, err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 400*time.Millisecond)
	defer cancel()
	got, err := quickSearchStatementTimeout(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ms, err := strconv.Atoi(strings.TrimSuffix(got, "ms"))
	if err != nil || ms < 1 || ms > 400 {
		t.Fatalf("caller deadline was ignored or timeout disabled: %q", got)
	}
	cancel()
	if _, err := quickSearchStatementTimeout(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled statement budget = %v", err)
	}
}
