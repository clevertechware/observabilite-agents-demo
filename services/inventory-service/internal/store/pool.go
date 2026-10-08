package store

import (
	"context"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string, tracerOptions ...otelpgx.Option) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	poolConfig.ConnConfig.Tracer = otelpgx.NewTracer(tracerOptions...)

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}
	if err := otelpgx.RecordStats(pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
