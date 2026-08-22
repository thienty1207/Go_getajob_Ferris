//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresClientCVDeleteTransaction(t *testing.T) {
	pool := newCVDeleteIntegrationPool(t)
	ctx := context.Background()
	repo := NewPostgresClientCVRepository(pool)

	t.Run("deletes a sole structured profile", func(t *testing.T) {
		fixture := newCVDeleteFixture(t, pool)
		ownerID := fixture.insertClientUser()
		profileID := fixture.insertProfile()
		scanID := fixture.insertScan(ownerID, profileID)

		if err := repo.DeleteClientCV(ctx, ownerID, scanID); err != nil {
			t.Fatalf("DeleteClientCV() error = %v", err)
		}

		assertCVDeleteRowCount(t, pool, "SELECT count(*) FROM public.scans WHERE id = $1", scanID, 0)
		assertCVDeleteRowCount(t, pool, "SELECT count(*) FROM public.structured_profiles WHERE id = $1", profileID, 0)
	})

	t.Run("preserves a shared profile until its final scan is deleted", func(t *testing.T) {
		fixture := newCVDeleteFixture(t, pool)
		firstOwnerID := fixture.insertClientUser()
		secondOwnerID := fixture.insertClientUser()
		profileID := fixture.insertProfile()
		firstScanID := fixture.insertScan(firstOwnerID, profileID)
		secondScanID := fixture.insertScan(secondOwnerID, profileID)

		if err := repo.DeleteClientCV(ctx, firstOwnerID, firstScanID); err != nil {
			t.Fatalf("DeleteClientCV(first scan) error = %v", err)
		}
		assertCVDeleteRowCount(t, pool, "SELECT count(*) FROM public.structured_profiles WHERE id = $1", profileID, 1)
		assertCVDeleteRowCount(t, pool, "SELECT count(*) FROM public.scans WHERE id = $1", secondScanID, 1)

		if err := repo.DeleteClientCV(ctx, secondOwnerID, secondScanID); err != nil {
			t.Fatalf("DeleteClientCV(final scan) error = %v", err)
		}
		assertCVDeleteRowCount(t, pool, "SELECT count(*) FROM public.structured_profiles WHERE id = $1", profileID, 0)
	})

	t.Run("serializes concurrent deletions of a shared profile", func(t *testing.T) {
		barrier := newCVScanDeleteBarrier(2)
		concurrentPool := newCVDeleteIntegrationPoolWithTracer(t, barrier)
		fixture := newCVDeleteFixture(t, concurrentPool)
		firstOwnerID := fixture.insertClientUser()
		secondOwnerID := fixture.insertClientUser()
		profileID := fixture.insertProfile()
		firstScanID := fixture.insertScan(firstOwnerID, profileID)
		secondScanID := fixture.insertScan(secondOwnerID, profileID)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		errorsByDelete := make(chan error, 2)
		go func() {
			errorsByDelete <- NewPostgresClientCVRepository(concurrentPool).DeleteClientCV(ctx, firstOwnerID, firstScanID)
		}()
		go func() {
			errorsByDelete <- NewPostgresClientCVRepository(concurrentPool).DeleteClientCV(ctx, secondOwnerID, secondScanID)
		}()
		for range 2 {
			if err := <-errorsByDelete; err != nil {
				t.Fatalf("concurrent DeleteClientCV() error = %v", err)
			}
		}

		assertCVDeleteRowCount(t, concurrentPool, "SELECT count(*) FROM public.scans WHERE id = $1", firstScanID, 0)
		assertCVDeleteRowCount(t, concurrentPool, "SELECT count(*) FROM public.scans WHERE id = $1", secondScanID, 0)
		assertCVDeleteRowCount(t, concurrentPool, "SELECT count(*) FROM public.structured_profiles WHERE id = $1", profileID, 0)
	})

	t.Run("rejects a foreign owner without deleting anything", func(t *testing.T) {
		fixture := newCVDeleteFixture(t, pool)
		ownerID := fixture.insertClientUser()
		foreignOwnerID := fixture.insertClientUser()
		profileID := fixture.insertProfile()
		scanID := fixture.insertScan(ownerID, profileID)

		err := repo.DeleteClientCV(ctx, foreignOwnerID, scanID)
		if !errors.Is(err, ErrClientCVNotFound) {
			t.Fatalf("DeleteClientCV() error = %v, want ErrClientCVNotFound", err)
		}

		assertCVDeleteRowCount(t, pool, "SELECT count(*) FROM public.scans WHERE id = $1", scanID, 1)
		assertCVDeleteRowCount(t, pool, "SELECT count(*) FROM public.structured_profiles WHERE id = $1", profileID, 1)
	})

	t.Run("cascades scan matches", func(t *testing.T) {
		fixture := newCVDeleteFixture(t, pool)
		ownerID := fixture.insertClientUser()
		profileID := fixture.insertProfile()
		scanID := fixture.insertScan(ownerID, profileID)
		matchID := fixture.insertMatch(scanID)

		if err := repo.DeleteClientCV(ctx, ownerID, scanID); err != nil {
			t.Fatalf("DeleteClientCV() error = %v", err)
		}

		assertCVDeleteRowCount(t, pool, "SELECT count(*) FROM public.scan_matches WHERE id = $1", matchID, 0)
	})
}

func TestPostgresAdminCVDeleteTransaction(t *testing.T) {
	pool := newCVDeleteIntegrationPool(t)
	ctx := context.Background()
	repo := NewPostgresAdminCVRepository(pool)

	t.Run("deletes a sole structured profile", func(t *testing.T) {
		fixture := newCVDeleteFixture(t, pool)
		ownerID := fixture.insertClientUser()
		profileID := fixture.insertProfile()
		scanID := fixture.insertScan(ownerID, profileID)

		if err := repo.DeleteAdminCV(ctx, scanID); err != nil {
			t.Fatalf("DeleteAdminCV() error = %v", err)
		}

		assertCVDeleteRowCount(t, pool, "SELECT count(*) FROM public.scans WHERE id = $1", scanID, 0)
		assertCVDeleteRowCount(t, pool, "SELECT count(*) FROM public.structured_profiles WHERE id = $1", profileID, 0)
	})

	t.Run("preserves a profile still referenced by another scan", func(t *testing.T) {
		fixture := newCVDeleteFixture(t, pool)
		ownerID := fixture.insertClientUser()
		profileID := fixture.insertProfile()
		firstScanID := fixture.insertScan(ownerID, profileID)
		secondScanID := fixture.insertScan(ownerID, profileID)

		if err := repo.DeleteAdminCV(ctx, firstScanID); err != nil {
			t.Fatalf("DeleteAdminCV(first scan) error = %v", err)
		}
		assertCVDeleteRowCount(t, pool, "SELECT count(*) FROM public.structured_profiles WHERE id = $1", profileID, 1)
		assertCVDeleteRowCount(t, pool, "SELECT count(*) FROM public.scans WHERE id = $1", secondScanID, 1)

		if err := repo.DeleteAdminCV(ctx, secondScanID); err != nil {
			t.Fatalf("DeleteAdminCV(final scan) error = %v", err)
		}
		assertCVDeleteRowCount(t, pool, "SELECT count(*) FROM public.structured_profiles WHERE id = $1", profileID, 0)
	})
}

func newCVDeleteIntegrationPool(t *testing.T) *pgxpool.Pool {
	return newCVDeleteIntegrationPoolWithTracer(t, nil)
}

func newCVDeleteIntegrationPoolWithTracer(t *testing.T, tracer pgx.QueryTracer) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for the PostgreSQL integration gate")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse PostgreSQL pool config: %v", err)
	}
	config.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}
	return pool
}

type cvDeleteTraceKey struct{}

type cvDeleteTraceStage uint8

const (
	cvDeleteTraceScan cvDeleteTraceStage = iota + 1
	cvDeleteTraceOrphanCleanup
)

type cvScanDeleteBarrier struct {
	mu                   sync.Mutex
	want                 int
	scanArrived          int
	scanRelease          chan struct{}
	cleanupArrived       int
	cleanupRelease       chan struct{}
	profileLockStatement bool
}

func newCVScanDeleteBarrier(want int) *cvScanDeleteBarrier {
	return &cvScanDeleteBarrier{
		want:           want,
		scanRelease:    make(chan struct{}),
		cleanupRelease: make(chan struct{}),
	}
}

func (b *cvScanDeleteBarrier) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	normalizedSQL := strings.ToUpper(strings.Join(strings.Fields(data.SQL), " "))
	switch {
	case strings.TrimSpace(data.SQL) == strings.TrimSpace(deleteClientCVQuery):
		return context.WithValue(ctx, cvDeleteTraceKey{}, cvDeleteTraceScan)
	case strings.TrimSpace(data.SQL) == strings.TrimSpace(deleteOrphanCVProfileQuery):
		return context.WithValue(ctx, cvDeleteTraceKey{}, cvDeleteTraceOrphanCleanup)
	case strings.Contains(normalizedSQL, "FROM PUBLIC.STRUCTURED_PROFILES") && strings.Contains(normalizedSQL, "FOR UPDATE"):
		b.mu.Lock()
		b.profileLockStatement = true
		b.mu.Unlock()
	}
	return ctx
}

func (b *cvScanDeleteBarrier) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	stage, _ := ctx.Value(cvDeleteTraceKey{}).(cvDeleteTraceStage)
	b.mu.Lock()
	var release chan struct{}
	switch stage {
	case cvDeleteTraceScan:
		b.scanArrived++
		if b.scanArrived == b.want {
			close(b.scanRelease)
		}
		release = b.scanRelease
	case cvDeleteTraceOrphanCleanup:
		if b.profileLockStatement {
			b.mu.Unlock()
			return
		}
		b.cleanupArrived++
		if b.cleanupArrived == b.want {
			close(b.cleanupRelease)
		}
		release = b.cleanupRelease
	default:
		b.mu.Unlock()
		return
	}
	b.mu.Unlock()

	select {
	case <-release:
	case <-ctx.Done():
	}
}

type cvDeleteFixture struct {
	t          *testing.T
	pool       *pgxpool.Pool
	clientIDs  []uuid.UUID
	profileIDs []uuid.UUID
	scanIDs    []uuid.UUID
	sourceIDs  []uuid.UUID
	jobIDs     []uuid.UUID
}

func newCVDeleteFixture(t *testing.T, pool *pgxpool.Pool) *cvDeleteFixture {
	t.Helper()
	fixture := &cvDeleteFixture{t: t, pool: pool}
	t.Cleanup(fixture.cleanup)
	return fixture
}

func (f *cvDeleteFixture) insertClientUser() uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	email := "cv-delete-" + id.String() + "@example.test"
	if _, err := f.pool.Exec(context.Background(), `
INSERT INTO public.client_users (id, email, display_name, provider)
VALUES ($1, $2, 'CV delete integration', 'google')`, id, email); err != nil {
		f.t.Fatalf("insert client user: %v", err)
	}
	f.clientIDs = append(f.clientIDs, id)
	return id
}

func (f *cvDeleteFixture) insertProfile() uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	if _, err := f.pool.Exec(context.Background(), `
INSERT INTO public.structured_profiles (id, seniority, schema_version, parser_model)
VALUES ($1, 'MID', 'v1', 'cv-delete-integration')`, id); err != nil {
		f.t.Fatalf("insert structured profile: %v", err)
	}
	f.profileIDs = append(f.profileIDs, id)
	return id
}

func (f *cvDeleteFixture) insertScan(ownerID, profileID uuid.UUID) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	if _, err := f.pool.Exec(context.Background(), `
INSERT INTO public.scans (id, status, profile_id, client_user_id, location_text)
VALUES ($1, 'COMPLETED', $2, $3, 'CV delete integration')`, id, profileID, ownerID); err != nil {
		f.t.Fatalf("insert scan: %v", err)
	}
	f.scanIDs = append(f.scanIDs, id)
	return id
}

func (f *cvDeleteFixture) insertMatch(scanID uuid.UUID) uuid.UUID {
	f.t.Helper()
	sourceID := uuid.New()
	jobID := uuid.New()
	matchID := uuid.New()
	suffix := sourceID.String()
	if _, err := f.pool.Exec(context.Background(), `
INSERT INTO public.job_sources (id, source_key, display_name, base_url, source_type, approval_status, approved_at, approved_by)
VALUES ($1, $2, 'CV delete integration', $3, 'EXPLICIT_PERMISSION', 'ACTIVE', now(), 'integration')`,
		sourceID, "cv-delete-"+suffix, "https://cv-delete-"+suffix+".example.test/careers/"); err != nil {
		f.t.Fatalf("insert job source: %v", err)
	}
	f.sourceIDs = append(f.sourceIDs, sourceID)
	if _, err := f.pool.Exec(context.Background(), `
INSERT INTO public.job_cache (
    id, source_id, source_job_key, content_hash, title, company, location_text,
    role, seniority, employment_type, work_mode, original_url, status
)
VALUES ($1, $2, 'cv-delete-job', $3, 'CV delete job', 'Integration', 'Integration',
        'Engineer', 'MID', 'FULL_TIME', 'ONSITE', $4, 'ACTIVE')`,
		jobID, sourceID, strings.Repeat("a", 64), "https://cv-delete-"+suffix+".example.test/careers/job"); err != nil {
		f.t.Fatalf("insert job: %v", err)
	}
	f.jobIDs = append(f.jobIDs, jobID)
	if _, err := f.pool.Exec(context.Background(), `
INSERT INTO public.scan_matches (
    id, scan_id, job_id, required_skills_points, role_relevance_points,
    experience_points, seniority_points, preferred_skills_domain_points, match_percent
)
VALUES ($1, $2, $3, 35, 25, 15, 15, 10, 100)`, matchID, scanID, jobID); err != nil {
		f.t.Fatalf("insert scan match: %v", err)
	}
	return matchID
}

func (f *cvDeleteFixture) cleanup() {
	ctx := context.Background()
	f.cleanupRows(ctx, "scan", "DELETE FROM public.scans WHERE id = $1", f.scanIDs)
	f.cleanupRows(ctx, "profile", "DELETE FROM public.structured_profiles WHERE id = $1", f.profileIDs)
	f.cleanupRows(ctx, "job", "DELETE FROM public.job_cache WHERE id = $1", f.jobIDs)
	f.cleanupRows(ctx, "source", "DELETE FROM public.job_sources WHERE id = $1", f.sourceIDs)
	f.cleanupRows(ctx, "client", "DELETE FROM public.client_users WHERE id = $1", f.clientIDs)
}

func (f *cvDeleteFixture) cleanupRows(ctx context.Context, kind, query string, ids []uuid.UUID) {
	for _, id := range ids {
		if _, err := f.pool.Exec(ctx, query, id); err != nil {
			// Continue through every fixture so one cleanup failure does not hide
			// additional leaked rows, but make the integration test report it.
			f.t.Errorf("cleanup %s %s: %v", kind, id, err)
		}
	}
}

func assertCVDeleteRowCount(t *testing.T, pool *pgxpool.Pool, query string, id uuid.UUID, want int) {
	t.Helper()
	var got int
	if err := pool.QueryRow(context.Background(), query, id).Scan(&got); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if got != want {
		t.Fatalf("row count = %d, want %d", got, want)
	}
}
