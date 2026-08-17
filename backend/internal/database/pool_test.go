package database

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestOpenRejectsBlankDatabaseURL(t *testing.T) {
	_, err := Open(context.Background(), " ")
	if err == nil {
		t.Fatal("Open() error = nil, want blank URL error")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("Open() error = %q, want DATABASE_URL in local configuration error", err)
	}
}

func TestOpenHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Open(ctx, "postgres://app:secret@127.0.0.1:5432/jobs?connect_timeout=1")
	if err == nil {
		t.Fatal("Open() error = nil for canceled context")
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cancel") {
		t.Fatalf("Open() error = %v, want canceled-context error", err)
	}
}
