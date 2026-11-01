package repositories

import (
	"context"
	"errors"

	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/inventory"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	stockQuery   = `SELECT s.quantity FROM stock s, pg_sleep($2::float8) WHERE s.sku = $1`
	lockQuery    = `SELECT quantity FROM stock WHERE sku = $1 FOR UPDATE`
	decrementSQL = `UPDATE stock SET quantity = quantity - $2 WHERE sku = $1`
)

type PGStock struct {
	pool             *pgxpool.Pool
	slowQuerySeconds float64
}

func NewPGStock(pool *pgxpool.Pool, slowQuerySeconds float64) *PGStock {
	return &PGStock{pool: pool, slowQuerySeconds: slowQuerySeconds}
}

// Quantity returns the quantity of the given SKU.
func (r *PGStock) Quantity(ctx context.Context, sku string) (int, error) {
	var quantity int
	err := r.pool.QueryRow(ctx, stockQuery, sku, r.slowQuerySeconds).Scan(&quantity)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, inventory.ErrSkuNotFound
	}
	return quantity, err
}

// Reserve locks the stock row for the duration of the transaction so that concurrent
// reservations cannot drive the quantity below zero.
func (r *PGStock) Reserve(ctx context.Context, sku string, quantity int) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var available int
		err := tx.QueryRow(ctx, lockQuery, sku).Scan(&available)
		if errors.Is(err, pgx.ErrNoRows) {
			return inventory.ErrSkuNotFound
		}
		if err != nil {
			return err
		}
		if available < quantity {
			return inventory.ErrInsufficientStock
		}
		_, err = tx.Exec(ctx, decrementSQL, sku, quantity)
		return err
	})
}
