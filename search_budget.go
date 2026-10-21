package arxiv

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

const quickSearchTimeout = 30 * time.Second

// A cold exact-ranked search can read thousands of scattered vector pages.
// Bound simultaneous operations independently of the shared database pool.
var quickSearchSlots = make(chan struct{}, 2)

type quickSearchOperationKey struct{}

type quickSearchOperation struct {
	slots  chan struct{}
	active atomic.Bool
}

// WithQuickSearch bounds a complete Quick operation, including optional author
// checks, queueing and SQL. Nested QuickSearch calls share the operation's slot
// and deadline. The callback must finish its work before returning.
func (c *Cache) WithQuickSearch(ctx context.Context, fn func(context.Context) error) error {
	return withQuickSearchOperation(ctx, quickSearchSlots, fn)
}

func withQuickSearchOperation(ctx context.Context, slots chan struct{}, fn func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if operation, ok := ctx.Value(quickSearchOperationKey{}).(*quickSearchOperation); ok &&
		operation.slots == slots && operation.active.Load() {
		if err := fn(ctx); err != nil {
			return err
		}
		return ctx.Err()
	}

	// Start the budget before waiting for capacity; shorter caller deadlines
	// and client disconnects always take precedence.
	ctx, cancel := context.WithTimeout(ctx, quickSearchTimeout)
	defer cancel()
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	operation := &quickSearchOperation{slots: slots}
	operation.active.Store(true)
	defer operation.active.Store(false)
	ctx = context.WithValue(ctx, quickSearchOperationKey{}, operation)
	if err := fn(ctx); err != nil {
		return err
	}
	return ctx.Err()
}

func quickSearchStatementTimeout(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	remaining := quickSearchTimeout
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < remaining {
		remaining = time.Until(deadline)
	}
	if remaining <= 0 {
		return "", context.DeadlineExceeded
	}
	// Round upward: PostgreSQL treats zero as disabling the timeout. The Go
	// context continues to enforce the exact (possibly sub-millisecond) deadline.
	milliseconds := (remaining + time.Millisecond - 1) / time.Millisecond
	return fmt.Sprintf("%dms", milliseconds), nil
}
