package repository

import (
	"context"
	"strings"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

const countAdminJobsQuery = `
SELECT count(*)
FROM public.job_cache AS jobs
JOIN public.job_sources AS sources ON sources.id = jobs.source_id
LEFT JOIN public.locations AS locations ON locations.id = jobs.location_id
WHERE ($1::uuid IS NULL OR jobs.location_id = $1)
  AND (NOT $2::boolean OR jobs.location_id IS NULL)
  AND ($3::text = '' OR jobs.title ILIKE '%' || $3::text || '%'
       OR jobs.company ILIKE '%' || $3::text || '%'
       OR jobs.role ILIKE '%' || $3::text || '%'
       OR jobs.location_text ILIKE '%' || $3::text || '%'
       OR COALESCE(locations.display_name, '') ILIKE '%' || $3::text || '%'
       OR sources.display_name ILIKE '%' || $3::text || '%'
       OR sources.source_key ILIKE '%' || $3::text || '%'
       OR jobs.original_url ILIKE '%' || $3::text || '%')`

const listAdminJobsQuery = `
SELECT
    jobs.id,
    sources.source_key,
    sources.display_name,
    sources.approval_status,
    jobs.title,
    jobs.company,
    COALESCE(locations.display_name, jobs.location_text),
    jobs.location_id,
    jobs.location_assignment_source,
    jobs.role,
    jobs.required_skills,
    jobs.preferred_skills,
    jobs.seniority,
    jobs.minimum_experience_years,
    jobs.domains,
    jobs.employment_type,
    jobs.work_mode,
    jobs.status,
    jobs.original_url,
    jobs.content_hash,
    jobs.last_seen_at,
    jobs.updated_at
FROM public.job_cache AS jobs
JOIN public.job_sources AS sources ON sources.id = jobs.source_id
LEFT JOIN public.locations AS locations ON locations.id = jobs.location_id
WHERE ($1::uuid IS NULL OR jobs.location_id = $1)
  AND (NOT $2::boolean OR jobs.location_id IS NULL)
  AND ($3::text = '' OR jobs.title ILIKE '%' || $3::text || '%'
       OR jobs.company ILIKE '%' || $3::text || '%'
       OR jobs.role ILIKE '%' || $3::text || '%'
       OR jobs.location_text ILIKE '%' || $3::text || '%'
       OR COALESCE(locations.display_name, '') ILIKE '%' || $3::text || '%'
       OR sources.display_name ILIKE '%' || $3::text || '%'
       OR sources.source_key ILIKE '%' || $3::text || '%'
       OR jobs.original_url ILIKE '%' || $3::text || '%')
ORDER BY jobs.updated_at DESC, jobs.id DESC
LIMIT $4 OFFSET $5`

type PostgresJobRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresJobRepository uses the same pool as scan/promotion repositories so
// admin reads see the crawler's committed cache without a second connection.
func NewPostgresJobRepository(pool *pgxpool.Pool) *PostgresJobRepository {
	return &PostgresJobRepository{pool: pool}
}

func (r *PostgresJobRepository) ListAdminJobs(ctx context.Context, page, pageSize int, filter AdminJobFilter) (model.AdminJobPage, error) {
	locationID, unresolved, search := adminJobFilterArgs(filter)
	var total int
	if err := r.pool.QueryRow(ctx, countAdminJobsQuery, locationID, unresolved, search).Scan(&total); err != nil {
		return model.AdminJobPage{}, err
	}
	pageResult := model.AdminJobPage{Page: page, PageSize: pageSize, Total: total, Items: make([]model.AdminJob, 0)}
	offset := (page - 1) * pageSize
	rows, err := r.pool.Query(ctx, listAdminJobsQuery, locationID, unresolved, search, pageSize, offset)
	if err != nil {
		return model.AdminJobPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var job model.AdminJob
		if err := rows.Scan(
			&job.ID,
			&job.SourceKey,
			&job.SourceName,
			&job.SourceApprovalStatus,
			&job.Title,
			&job.Company,
			&job.Location,
			&job.LocationID,
			&job.LocationAssignmentSource,
			&job.Role,
			&job.RequiredSkills,
			&job.PreferredSkills,
			&job.Seniority,
			&job.MinimumExperience,
			&job.Domains,
			&job.EmploymentType,
			&job.WorkMode,
			&job.Status,
			&job.OriginalURL,
			&job.ContentHash,
			&job.LastSeenAt,
			&job.UpdatedAt,
		); err != nil {
			return model.AdminJobPage{}, err
		}
		pageResult.Items = append(pageResult.Items, job)
	}
	if err := rows.Err(); err != nil {
		return model.AdminJobPage{}, err
	}
	return pageResult, nil
}

func adminJobFilterArgs(filter AdminJobFilter) (any, bool, string) {
	search := strings.TrimSpace(filter.Search)
	if filter.LocationID == nil {
		return nil, filter.UnresolvedLocation, search
	}
	return *filter.LocationID, filter.UnresolvedLocation, search
}
