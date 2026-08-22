//go:build integration

package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Asserts the location-ownership SQL behind the admin PATCH endpoint. The
// crawler-side preservation is covered by the Rust location_ownership_contract
// test; here we prove the backend write sets the ownership marker correctly for
// both assignment (ADMIN) and clear (AUTO) on a migrated schema.
//
// Everything runs inside a transaction that is rolled back, so no test data
// leaks regardless of cleanup ordering.
func TestPostgresAssignJobLocationWritesOwnershipSource(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for the PostgreSQL integration gate")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	sourceName := "integration-location-own-" + uuid.NewString()
	sourceID := uuid.New()
	jobID := uuid.New()
	locationID := uuid.New()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
INSERT INTO public.job_sources (id, source_key, display_name, base_url, source_type, approval_status, approved_at, approved_by)
VALUES ($1, $2, 'Integration location ownership', 'https://loc-own.invalid/careers/', 'EXPLICIT_PERMISSION', 'ACTIVE', now(), 'integration')`,
		sourceID, sourceName); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO public.locations (id, display_name, province, country, canonical_key)
VALUES ($1, 'Integ Loc', 'Integ', 'Vietnam', $2)`,
		locationID, "integ-loc-"+sourceName); err != nil {
		t.Fatalf("insert location: %v", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO public.job_cache (id, source_id, source_job_key, content_hash, title, company, location_text, role, seniority, employment_type, work_mode, original_url, status, location_assignment_source)
VALUES ($1, $2, 'loc-own-job', $3, 'Location own', 'Integ', 'Integ', 'Engineer', 'MID', 'FULL_TIME', 'ONSITE', 'https://loc-own.invalid/careers/1', 'ACTIVE', 'AUTO')`,
		jobID, sourceID, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("insert cached job: %v", err)
	}

	// Assign a location: must record ADMIN ownership and set location_id.
	if _, err := tx.Exec(ctx, assignJobLocationQuery, jobID, &locationID); err != nil {
		t.Fatalf("assign location: %v", err)
	}
	var loc *uuid.UUID
	var source string
	if err := tx.QueryRow(ctx,
		"SELECT location_id, location_assignment_source FROM public.job_cache WHERE id = $1", jobID).
		Scan(&loc, &source); err != nil {
		t.Fatalf("read assigned job: %v", err)
	}
	if loc == nil || *loc != locationID {
		t.Fatalf("location_id = %v, want %v", loc, locationID)
	}
	if source != "ADMIN" {
		t.Fatalf("location_assignment_source = %q, want ADMIN on assignment", source)
	}

	// Clear (NULL) must revert to AUTO so the crawler may re-resolve.
	if _, err := tx.Exec(ctx, assignJobLocationQuery, jobID, (*uuid.UUID)(nil)); err != nil {
		t.Fatalf("clear location: %v", err)
	}
	if err := tx.QueryRow(ctx,
		"SELECT location_id, location_assignment_source FROM public.job_cache WHERE id = $1", jobID).
		Scan(&loc, &source); err != nil {
		t.Fatalf("read cleared job: %v", err)
	}
	if loc != nil {
		t.Fatalf("cleared location_id = %v, want NULL", loc)
	}
	if source != "AUTO" {
		t.Fatalf("location_assignment_source = %q, want AUTO after clear", source)
	}
}
