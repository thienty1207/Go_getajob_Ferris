package repository

import (
	"context"
	"errors"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresCrawlRequestRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresCrawlRequestRepository(pool *pgxpool.Pool) *PostgresCrawlRequestRepository {
	return &PostgresCrawlRequestRepository{pool: pool}
}

const enqueueCrawlRequestQuery = `
INSERT INTO public.crawl_requests (source_id, requested_by)
SELECT id, $2
FROM public.job_sources
WHERE id = $1 AND approval_status = 'ACTIVE'
ON CONFLICT DO NOTHING
RETURNING id, source_id, status, requested_by, requested_at, started_at, finished_at, source_run_id, error_code`

const activeCrawlRequestQuery = `
SELECT id, source_id, status, requested_by, requested_at, started_at, finished_at, source_run_id, error_code
FROM public.crawl_requests
WHERE source_id = $1 AND status IN ('PENDING', 'RUNNING')
ORDER BY requested_at, id
LIMIT 1`

func (r *PostgresCrawlRequestRepository) EnqueueCrawlRequest(ctx context.Context, sourceID uuid.UUID, actor string) (model.CrawlRequest, error) {
	request, err := scanCrawlRequest(r.pool.QueryRow(ctx, enqueueCrawlRequestQuery, sourceID, actor))
	if err == nil {
		return request, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.CrawlRequest{}, err
	}

	// A duplicate pending/running request is idempotent. The partial unique
	// index prevents two workers from claiming the same source at once.
	request, existingErr := scanCrawlRequest(r.pool.QueryRow(ctx, activeCrawlRequestQuery, sourceID))
	if existingErr == nil {
		return request, nil
	}
	if !errors.Is(existingErr, pgx.ErrNoRows) {
		return model.CrawlRequest{}, existingErr
	}

	var approvalStatus string
	statusErr := r.pool.QueryRow(ctx, `SELECT approval_status FROM public.job_sources WHERE id = $1`, sourceID).Scan(&approvalStatus)
	if errors.Is(statusErr, pgx.ErrNoRows) {
		return model.CrawlRequest{}, ErrCrawlSourceNotFound
	}
	if statusErr != nil {
		return model.CrawlRequest{}, statusErr
	}
	if approvalStatus != "ACTIVE" {
		return model.CrawlRequest{}, ErrCrawlSourceInactive
	}

	// The request may have completed between the first insert and the active
	// request lookup. Retry once so a new click can enqueue the next crawl.
	request, retryErr := scanCrawlRequest(r.pool.QueryRow(ctx, enqueueCrawlRequestQuery, sourceID, actor))
	if retryErr == nil {
		return request, nil
	}
	if errors.Is(retryErr, pgx.ErrNoRows) {
		request, existingErr = scanCrawlRequest(r.pool.QueryRow(ctx, activeCrawlRequestQuery, sourceID))
		if existingErr == nil {
			return request, nil
		}
	}
	return model.CrawlRequest{}, retryErr
}

type crawlRequestScanner interface {
	Scan(...any) error
}

func scanCrawlRequest(row crawlRequestScanner) (model.CrawlRequest, error) {
	var request model.CrawlRequest
	err := row.Scan(
		&request.ID,
		&request.SourceID,
		&request.Status,
		&request.RequestedBy,
		&request.RequestedAt,
		&request.StartedAt,
		&request.FinishedAt,
		&request.SourceRunID,
		&request.ErrorCode,
	)
	return request, err
}

var _ CrawlRequestRepository = (*PostgresCrawlRequestRepository)(nil)
