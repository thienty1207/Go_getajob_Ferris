package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitForDatabaseRetriesRecoveryUntilReady(t *testing.T) {
	attempts := 0
	err := waitForDatabase(context.Background(), time.Millisecond, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("database system is in recovery mode")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("waitForDatabase() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("ping attempts = %d, want 3", attempts)
	}
}

func TestWaitForDatabaseStopsAtContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()

	err := waitForDatabase(ctx, time.Millisecond, func(context.Context) error {
		return errors.New("database unavailable")
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waitForDatabase() error = %v, want context deadline", err)
	}
}
