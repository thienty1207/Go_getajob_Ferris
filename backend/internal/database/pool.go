package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open creates a PostgreSQL pool and pings it before returning. The ping is
// deliberate: callers can treat a returned pool as a ready dependency instead
// of discovering a bad URL only on the first user request.
func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL")
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}
	if err := waitForDatabase(ctx, 500*time.Millisecond, pool.Ping); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// waitForDatabase absorbs the short recovery window commonly seen after an
// abrupt workstation/VPS restart. It remains bounded by the caller's startup
// context, so a genuinely unavailable database still stops the API cleanly.
func waitForDatabase(ctx context.Context, retryInterval time.Duration, ping func(context.Context) error) error {
	if retryInterval <= 0 {
		retryInterval = 500 * time.Millisecond
	}
	var lastErr error
	for {
		if err := ping(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}

		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(ctx.Err(), lastErr)
		case <-timer.C:
		}
	}
}
