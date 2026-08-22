package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const countAdminCVProfilesQuery = `
SELECT count(*)
FROM public.scans AS scans
JOIN public.client_users AS users ON users.id = scans.client_user_id
LEFT JOIN public.structured_profiles AS profiles ON profiles.id = scans.profile_id
WHERE ($1::text = '' OR users.email ILIKE '%' || $1::text || '%' OR users.display_name ILIKE '%' || $1::text || '%')
  AND ($2::text = '' OR EXISTS (
      SELECT 1 FROM unnest(COALESCE(profiles.roles, ARRAY[]::text[])) AS role_name
      WHERE role_name ILIKE '%' || $2::text || '%'
  ))`

const listAdminCVProfilesQuery = `
SELECT
    scans.id,
    scans.status,
    scans.location_text,
    scans.created_at,
    scans.updated_at,
    users.id,
    users.email,
    users.display_name,
    profiles.id,
    COALESCE(profiles.roles, ARRAY[]::text[]),
    COALESCE(profiles.skills, ARRAY[]::text[]),
    COALESCE(profiles.years_of_experience, 0)::double precision,
    COALESCE(profiles.seniority, ''),
    COALESCE(profiles.domains, ARRAY[]::text[]),
    COALESCE(profiles.education, '[]'::jsonb),
    COALESCE(profiles.certifications, '[]'::jsonb),
    COALESCE(match_counts.match_count, 0)::int
FROM public.scans AS scans
JOIN public.client_users AS users ON users.id = scans.client_user_id
LEFT JOIN public.structured_profiles AS profiles ON profiles.id = scans.profile_id
LEFT JOIN (
    SELECT scan_id, count(*)::int AS match_count
    FROM public.scan_matches
    GROUP BY scan_id
) AS match_counts ON match_counts.scan_id = scans.id
WHERE ($1::text = '' OR users.email ILIKE '%' || $1::text || '%' OR users.display_name ILIKE '%' || $1::text || '%')
  AND ($2::text = '' OR EXISTS (
      SELECT 1 FROM unnest(COALESCE(profiles.roles, ARRAY[]::text[])) AS role_name
      WHERE role_name ILIKE '%' || $2::text || '%'
  ))
ORDER BY scans.created_at DESC, scans.id DESC
LIMIT $3 OFFSET $4`

const deleteAdminCVQuery = `
DELETE FROM public.scans
WHERE id = $1
RETURNING profile_id`

type PostgresAdminCVRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresAdminCVRepository(pool *pgxpool.Pool) *PostgresAdminCVRepository {
	return &PostgresAdminCVRepository{pool: pool}
}

func (r *PostgresAdminCVRepository) ListAdminCVProfiles(ctx context.Context, page, pageSize int, filter AdminCVFilter) (model.AdminCVProfilePage, error) {
	userSearch := strings.TrimSpace(filter.User)
	roleSearch := strings.TrimSpace(filter.Role)
	var total int
	if err := r.pool.QueryRow(ctx, countAdminCVProfilesQuery, userSearch, roleSearch).Scan(&total); err != nil {
		return model.AdminCVProfilePage{}, err
	}
	rows, err := r.pool.Query(ctx, listAdminCVProfilesQuery, userSearch, roleSearch, pageSize, (page-1)*pageSize)
	if err != nil {
		return model.AdminCVProfilePage{}, err
	}
	defer rows.Close()
	pageResult := model.AdminCVProfilePage{Page: page, PageSize: pageSize, Total: total, Items: make([]model.AdminCVProfile, 0)}
	for rows.Next() {
		item, scanErr := scanAdminCVRow(rows)
		if scanErr != nil {
			return model.AdminCVProfilePage{}, scanErr
		}
		pageResult.Items = append(pageResult.Items, item)
	}
	if err := rows.Err(); err != nil {
		return model.AdminCVProfilePage{}, err
	}
	return pageResult, nil
}

func scanAdminCVRow(rows pgx.Rows) (model.AdminCVProfile, error) {
	var (
		item           model.AdminCVProfile
		status         string
		profileID      pgtype.UUID
		roles          []string
		skills         []string
		seniority      string
		domains        []string
		years          float64
		education      []byte
		certifications []byte
	)
	if err := rows.Scan(
		&item.ScanID,
		&status,
		&item.Location,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.UserID,
		&item.Email,
		&item.DisplayName,
		&profileID,
		&roles,
		&skills,
		&years,
		&seniority,
		&domains,
		&education,
		&certifications,
		&item.MatchCount,
	); err != nil {
		return model.AdminCVProfile{}, err
	}
	item.Status = model.ScanStatus(status)
	if !item.Status.IsValid() {
		return model.AdminCVProfile{}, errors.New("invalid admin cv scan status")
	}
	if profileID.Valid {
		profile := &model.StructuredProfile{Roles: roles, Skills: skills, YearsOfExperience: years, Seniority: seniority, Domains: domains}
		if err := json.Unmarshal(education, &profile.Education); err != nil {
			return model.AdminCVProfile{}, err
		}
		if err := json.Unmarshal(certifications, &profile.Certifications); err != nil {
			return model.AdminCVProfile{}, err
		}
		item.Profile = profile
	}
	return item, nil
}

func (r *PostgresAdminCVRepository) DeleteAdminCV(ctx context.Context, scanID uuid.UUID) error {
	return deleteCVScan(ctx, r.pool, deleteAdminCVQuery, ErrAdminCVNotFound, scanID)
}

var _ AdminCVRepository = (*PostgresAdminCVRepository)(nil)
