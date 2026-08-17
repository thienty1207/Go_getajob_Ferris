package repository

import (
	"context"
	"errors"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const countJobLinksQuery = `
SELECT count(*)
FROM public.job_sources`

const listJobLinksQuery = `
SELECT
    sources.id,
    sources.source_key,
    sources.display_name,
    sources.base_url,
    sources.approval_status,
    sources.approved_at,
    sources.approved_by,
    sources.created_at,
    sources.updated_at,
    active_request.id,
    active_request.status,
    latest.run_status,
    latest.finished_at,
    latest.pages_seen,
    latest.jobs_seen,
    latest.jobs_created,
    latest.jobs_updated,
    latest.jobs_missing,
    latest.error_code
FROM public.job_sources AS sources
LEFT JOIN LATERAL (
    SELECT id, status
    FROM public.crawl_requests
    WHERE source_id = sources.id
      AND status IN ('PENDING', 'RUNNING')
    ORDER BY requested_at, id
    LIMIT 1
) AS active_request ON true
LEFT JOIN LATERAL (
    SELECT run_status, finished_at, pages_seen, jobs_seen, jobs_created, jobs_updated, jobs_missing, error_code
    FROM public.source_crawl_runs
    WHERE source_id = sources.id
    ORDER BY started_at DESC, id DESC
    LIMIT 1
) AS latest ON true
ORDER BY sources.updated_at DESC, sources.id DESC
LIMIT $1 OFFSET $2`

const createJobLinkQuery = `
INSERT INTO public.job_sources (
    id,
    source_key,
    display_name,
    base_url,
    source_type,
    approval_status,
    approved_at,
    approved_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, source_key, display_name, base_url, approval_status, approved_at, approved_by, created_at, updated_at`

const updateJobLinkQuery = `
UPDATE public.job_sources
SET base_url = $2,
    display_name = $3,
    approval_status = 'ACTIVE',
    approved_at = $4,
    approved_by = $5,
    updated_at = now()
WHERE id = $1
RETURNING id, source_key, display_name, base_url, approval_status, approved_at, approved_by, created_at, updated_at`

const lockJobLinkBaseURLQuery = `
SELECT base_url
FROM public.job_sources
WHERE id = $1
FOR UPDATE`

const closeJobLinkJobsQuery = `
UPDATE public.job_cache
SET status = 'CLOSED',
    missing_healthy_cycles = 2,
    updated_at = now()
WHERE source_id = $1
  AND status IN ('ACTIVE', 'VERIFYING')`

const disableJobLinkQuery = `
UPDATE public.job_sources
SET approval_status = 'DISABLED', updated_at = now()
WHERE id = $1`

const setJobLinkStatusQuery = `
UPDATE public.job_sources
SET approval_status = $2, updated_at = now()
WHERE id = $1`

const deleteJobLinkQuery = `
DELETE FROM public.job_sources
WHERE id = $1`

const deleteJobLinkMatchesQuery = `
DELETE FROM public.scan_matches
WHERE job_id IN (SELECT id FROM public.job_cache WHERE source_id = $1)`

const deleteJobLinkJobsQuery = `
DELETE FROM public.job_cache
WHERE source_id = $1`

const deleteJobLinkRunsQuery = `
DELETE FROM public.source_crawl_runs
WHERE source_id = $1`

type PostgresJobLinkRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresJobLinkRepository(pool *pgxpool.Pool) *PostgresJobLinkRepository {
	return &PostgresJobLinkRepository{pool: pool}
}

func (r *PostgresJobLinkRepository) ListJobLinks(ctx context.Context, page, pageSize int) (model.JobLinkPage, error) {
	var total int
	if err := r.pool.QueryRow(ctx, countJobLinksQuery).Scan(&total); err != nil {
		return model.JobLinkPage{}, err
	}
	result := model.JobLinkPage{Page: page, PageSize: pageSize, Total: total, Items: make([]model.JobLink, 0)}
	rows, err := r.pool.Query(ctx, listJobLinksQuery, pageSize, (page-1)*pageSize)
	if err != nil {
		return model.JobLinkPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var link model.JobLink
		var approvedAt *time.Time
		var approvedBy *string
		var lastStatus *string
		var lastFinishedAt *time.Time
		var activeRequestID *uuid.UUID
		var activeRequestStatus *string
		var lastPages, lastJobs, lastCreated, lastUpdated, lastMissing *int
		var lastErrorCode *string
		if err := rows.Scan(
			&link.ID,
			&link.SourceKey,
			&link.DisplayName,
			&link.URL,
			&link.ApprovalStatus,
			&approvedAt,
			&approvedBy,
			&link.CreatedAt,
			&link.UpdatedAt,
			&activeRequestID,
			&activeRequestStatus,
			&lastStatus,
			&lastFinishedAt,
			&lastPages,
			&lastJobs,
			&lastCreated,
			&lastUpdated,
			&lastMissing,
			&lastErrorCode,
		); err != nil {
			return model.JobLinkPage{}, err
		}
		link.ApprovedAt = approvedAt
		link.ApprovedBy = approvedBy
		link.LastCrawlStatus = lastStatus
		link.LastCrawlAt = lastFinishedAt
		link.ActiveCrawlRequestID = activeRequestID
		link.ActiveCrawlRequestStatus = activeRequestStatus
		if lastPages != nil {
			link.LastCrawlPages = *lastPages
		}
		if lastJobs != nil {
			link.LastCrawlJobs = *lastJobs
		}
		if lastCreated != nil {
			link.LastCrawlCreated = *lastCreated
		}
		if lastUpdated != nil {
			link.LastCrawlUpdated = *lastUpdated
		}
		if lastMissing != nil {
			link.LastCrawlMissing = *lastMissing
		}
		link.LastCrawlErrorCode = lastErrorCode
		result.Items = append(result.Items, link)
	}
	if err := rows.Err(); err != nil {
		return model.JobLinkPage{}, err
	}
	return result, nil
}

func (r *PostgresJobLinkRepository) CreateJobLink(ctx context.Context, write JobLinkWrite) (model.JobLink, error) {
	var link model.JobLink
	var approvedAt *time.Time
	var approvedBy *string
	err := r.pool.QueryRow(ctx, createJobLinkQuery,
		write.ID,
		write.SourceKey,
		write.DisplayName,
		write.BaseURL,
		write.SourceType,
		write.ApprovalStatus,
		write.ApprovedAt,
		write.ApprovedBy,
	).Scan(
		&link.ID,
		&link.SourceKey,
		&link.DisplayName,
		&link.URL,
		&link.ApprovalStatus,
		&approvedAt,
		&approvedBy,
		&link.CreatedAt,
		&link.UpdatedAt,
	)
	link.ApprovedAt = approvedAt
	link.ApprovedBy = approvedBy
	if pgError := new(pgconn.PgError); errors.As(err, &pgError) && pgError.Code == "23505" {
		return model.JobLink{}, ErrJobLinkConflict
	}
	return link, err
}

func (r *PostgresJobLinkRepository) UpdateJobLink(ctx context.Context, write JobLinkWrite) (model.JobLink, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return model.JobLink{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentBaseURL string
	if err := tx.QueryRow(ctx, lockJobLinkBaseURLQuery, write.ID).Scan(&currentBaseURL); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.JobLink{}, ErrJobLinkNotFound
		}
		return model.JobLink{}, err
	}
	if currentBaseURL != write.BaseURL {
		if _, err := tx.Exec(ctx, closeJobLinkJobsQuery, write.ID); err != nil {
			return model.JobLink{}, err
		}
	}

	var link model.JobLink
	var approvedAt *time.Time
	var approvedBy *string
	err = tx.QueryRow(ctx, updateJobLinkQuery,
		write.ID,
		write.BaseURL,
		write.DisplayName,
		write.ApprovedAt,
		write.ApprovedBy,
	).Scan(
		&link.ID,
		&link.SourceKey,
		&link.DisplayName,
		&link.URL,
		&link.ApprovalStatus,
		&approvedAt,
		&approvedBy,
		&link.CreatedAt,
		&link.UpdatedAt,
	)
	link.ApprovedAt = approvedAt
	link.ApprovedBy = approvedBy
	if errors.Is(err, pgx.ErrNoRows) {
		return model.JobLink{}, ErrJobLinkNotFound
	}
	if pgError := new(pgconn.PgError); errors.As(err, &pgError) && pgError.Code == "23505" {
		return model.JobLink{}, ErrJobLinkConflict
	}
	if err != nil {
		return model.JobLink{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.JobLink{}, err
	}
	return link, nil
}

func (r *PostgresJobLinkRepository) DisableJobLink(ctx context.Context, id uuid.UUID) error {
	return r.SetJobLinkStatus(ctx, id, "DISABLED")
}

func (r *PostgresJobLinkRepository) SetJobLinkStatus(ctx context.Context, id uuid.UUID, status string) error {
	result, err := r.pool.Exec(ctx, setJobLinkStatusQuery, id, status)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrJobLinkNotFound
	}
	return nil
}

func (r *PostgresJobLinkRepository) DeleteJobLink(ctx context.Context, id uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, deleteJobLinkMatchesQuery, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, deleteJobLinkJobsQuery, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, deleteJobLinkRunsQuery, id); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, deleteJobLinkQuery, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrJobLinkNotFound
	}
	return tx.Commit(ctx)
}

var _ JobLinkRepository = (*PostgresJobLinkRepository)(nil)
