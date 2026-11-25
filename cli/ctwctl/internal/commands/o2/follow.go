package o2

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/clevertechware/observabilite-agents-demo/cli/ctwctl/internal/o2"
)

const (
	followOverlap  = 10 * time.Second
	followPageSize = 1000
)

// follower polls the logs. Each round queries a window that overlaps the previous one, because OpenObserve indexes
// logs with a delay: a line older than the last one printed can arrive later.
// Deduplication relies on the full content of the line, kept as long as it stays in the window.
// The window never goes below windowFloor, the oldest line of the initial batch, so that the backlog dropped by
// --limit is not printed out of order. A full page is followed by the next pages: duplicates do not hide a new line.
type follower struct {
	searcher        LogSearcher
	filter          logFilter
	limit           int
	interval        time.Duration
	now             func() time.Time
	writeHit        func(o2.Hit) error
	seen            map[string]int64
	latestTimestamp int64
	windowFloor     int64
	pageSize        int
}

func newFollower(
	searcher LogSearcher,
	filter logFilter,
	limit int,
	interval time.Duration,
	now func() time.Time,
	writeHit func(o2.Hit) error,
) *follower {
	return &follower{
		searcher: searcher,
		filter:   filter,
		limit:    limit,
		interval: interval,
		now:      now,
		writeHit: writeHit,
		seen:     map[string]int64{},
		pageSize: followPageSize,
	}
}

func (f *follower) run(ctx context.Context, since time.Duration) error {
	end := f.now()
	start := end.Add(-since)
	f.latestTimestamp = start.UnixMicro()
	f.windowFloor = f.latestTimestamp

	if err := f.initialBatch(ctx, start, end); err != nil {
		return err
	}

	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := f.poll(ctx); err != nil {
				return err
			}
		}
	}
}

func (f *follower) initialBatch(ctx context.Context, start, end time.Time) error {
	result, err := f.searcher.Search(ctx, o2.SearchRequest{
		SQL:   f.filter.sql(orderNewestFirst),
		Start: start,
		End:   end,
		Size:  f.limit,
	})
	if err != nil {
		return fmt.Errorf("follow logs: %w", err)
	}

	hits := slices.Clone(result.Hits)
	slices.Reverse(hits)

	if len(hits) >= f.limit {
		if oldest, ok := timestampOf(hits[0]); ok {
			f.windowFloor = oldest
		}
	}

	return f.emit(hits)
}

func (f *follower) poll(ctx context.Context) error {
	end := f.now()
	start := max(f.latestTimestamp-followOverlap.Microseconds(), f.windowFloor)
	f.forgetBefore(start)

	for from := 0; ; {
		result, err := f.searcher.Search(ctx, o2.SearchRequest{
			SQL:   f.filter.sql(orderOldestFirst),
			Start: time.UnixMicro(start),
			End:   end,
			From:  from,
			Size:  f.pageSize,
		})
		if err != nil {
			return fmt.Errorf("follow logs: %w", err)
		}

		if err = f.emit(result.Hits); err != nil {
			return err
		}

		if len(result.Hits) < f.pageSize {
			return nil
		}
		from += len(result.Hits)
	}
}

func (f *follower) emit(hits []o2.Hit) error {
	for _, hit := range hits {
		key, err := dedupKey(hit)
		if err != nil {
			return err
		}
		if _, dup := f.seen[key]; dup {
			continue
		}

		ts, _ := timestampOf(hit)
		f.seen[key] = ts
		f.latestTimestamp = max(f.latestTimestamp, ts)

		if err = f.writeHit(hit); err != nil {
			return err
		}
	}
	return nil
}

func (f *follower) forgetBefore(micros int64) {
	for key, ts := range f.seen {
		if ts < micros {
			delete(f.seen, key)
		}
	}
}

func dedupKey(hit o2.Hit) (string, error) {
	data, err := json.Marshal(hit)
	if err != nil {
		return "", fmt.Errorf("encode log: %w", err)
	}
	return string(data), nil
}
