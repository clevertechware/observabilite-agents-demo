package commands

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

// follower relit les logs par polling. Chaque tour interroge une fenêtre qui recouvre le tour précédent, car
// OpenObserve indexe les logs avec un délai : une ligne plus ancienne que la dernière affichée peut arriver ensuite.
// La déduplication s'appuie sur le contenu complet de la ligne, conservé tant qu'elle reste dans la fenêtre.
// La fenêtre ne descend jamais sous windowFloor, la plus ancienne ligne du lot initial, pour ne pas réafficher hors ordre
// l'arriéré que --limit a écarté. Une page pleine est suivie des pages suivantes : des doublons ne masquent pas une ligne
// nouvelle.
type follower struct {
	searcher        logSearcher
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

func newFollower(searcher logSearcher, filter logFilter, limit int, interval time.Duration,
	now func() time.Time, writeHit func(o2.Hit) error) *follower {
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
		return err
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
			return err
		}
		if err := f.emit(result.Hits); err != nil {
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
		if err := f.writeHit(hit); err != nil {
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
		return "", fmt.Errorf("encodage d'un log : %w", err)
	}
	return string(data), nil
}
