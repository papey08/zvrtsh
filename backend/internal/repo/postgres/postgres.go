// Package postgres implements all repositories on top of one connection pool.
package postgres

import (
	"context"
	"fmt"
	"time"
	"zvrtsh/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const uniqueViolation = "23505"

// Connect creates a connection pool and waits until the database accepts connections.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, dbError(err)
	}

	var pingErr error
	for i := 0; i < 30; i++ {
		if pingErr = pool.Ping(ctx); pingErr == nil {
			return pool, nil
		}
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	pool.Close()
	return nil, dbError(pingErr)
}

// querier is either the pool or a transaction.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func dbError(err error) error {
	return fmt.Errorf("%w: %w", model.ErrDatabase, err)
}
