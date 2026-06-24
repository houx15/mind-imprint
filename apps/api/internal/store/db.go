// Package store is the data-access layer: the pgx connection pool, goose
// migrations, and sqlc-generated type-safe queries.
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a pgx connection pool against dsn and verifies it with a Ping.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	// Small service: a modest pool ceiling is plenty.
	cfg.MaxConns = 20

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
