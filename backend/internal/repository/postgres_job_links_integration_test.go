//go:build integration

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresJobLinksListHandlesNullableApprovalEvidence(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for the PostgreSQL integration gate")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	defer pool.Close()

	page, err := NewPostgresJobLinkRepository(pool).ListJobLinks(context.Background(), 1, 50)
	if err != nil {
		t.Fatalf("list Job Links: %v", err)
	}
	for _, link := range page.Items {
		if link.ApprovalStatus == "REVIEW" && (link.ApprovedAt != nil || link.ApprovedBy != nil) {
			t.Fatalf("review source unexpectedly contains approval evidence: %#v", link)
		}
	}
}

func TestPostgresJobLinksUpdateClosesJobsWhenBoundaryChanges(t *testing.T) {
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
	sourceID := uuid.New()
	sourceKey := "integration-update-boundary-" + sourceID.String()
	jobID := uuid.New()
	oldBaseURL := "https://old-" + sourceID.String() + ".example.com/careers/"
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM public.job_cache WHERE id = $1", jobID)
		_, _ = pool.Exec(ctx, "DELETE FROM public.job_sources WHERE id = $1", sourceID)
	})

	approvedAt := time.Now().UTC()
	approvedBy := "integration@example.com"
	_, err = pool.Exec(ctx, `
INSERT INTO public.job_sources (id, source_key, display_name, base_url, source_type, approval_status, approved_at, approved_by)
VALUES ($1, $2, 'Integration source', $3, 'EXPLICIT_PERMISSION', 'ACTIVE', $4, $5)`,
		sourceID, sourceKey, oldBaseURL, approvedAt, approvedBy)
	if err != nil {
		t.Fatalf("insert source: %v", err)
	}
	_, err = pool.Exec(ctx, `
INSERT INTO public.job_cache (
    id, source_id, source_job_key, content_hash, title, company, location_text,
    role, seniority, employment_type, work_mode, original_url, status
)
VALUES ($1, $2, 'integration-job', repeat('a', 64), 'Integration job', 'Integration company', 'Hà Nội',
		'Engineer', 'MID', 'FULL_TIME', 'ONSITE', $3, 'ACTIVE')`,
		jobID, sourceID, oldBaseURL+"integration-job")
	if err != nil {
		t.Fatalf("insert cached job: %v", err)
	}

	updatedAt := time.Now().UTC()
	updatedBy := "integration-update@example.com"
	newBaseURL := "https://new-" + sourceID.String() + ".example.com/careers/"
	_, err = NewPostgresJobLinkRepository(pool).UpdateJobLink(ctx, JobLinkWrite{
		ID:          sourceID,
		DisplayName: "new-" + sourceID.String() + ".example.com",
		BaseURL:     newBaseURL,
		ApprovedAt:  &updatedAt,
		ApprovedBy:  &updatedBy,
	})
	if err != nil {
		t.Fatalf("update source boundary: %v", err)
	}

	var status string
	var missingCycles int16
	if err := pool.QueryRow(ctx, "SELECT status, missing_healthy_cycles FROM public.job_cache WHERE id = $1", jobID).Scan(&status, &missingCycles); err != nil {
		t.Fatalf("read stale cached job: %v", err)
	}
	if status != "CLOSED" || missingCycles != 2 {
		t.Fatalf("stale cached job state = %s/%d, want CLOSED/2", status, missingCycles)
	}
}
