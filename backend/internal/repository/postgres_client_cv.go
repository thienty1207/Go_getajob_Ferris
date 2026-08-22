package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const countClientCVHistoryQuery = `
SELECT count(*)
FROM public.scans
WHERE client_user_id = $1`

const listClientCVHistoryQuery = `
SELECT
    scans.id,
    scans.status,
    scans.location_text,
    scans.created_at,
    scans.updated_at,
    profiles.id,
    COALESCE(profiles.roles, ARRAY[]::text[]),
    COALESCE(profiles.skills, ARRAY[]::text[]),
    COALESCE(profiles.years_of_experience, 0)::double precision,
    COALESCE(profiles.seniority, ''),
    COALESCE(profiles.domains, ARRAY[]::text[]),
    COALESCE(profiles.education, '[]'::jsonb),
    COALESCE(profiles.certifications, '[]'::jsonb),
    COALESCE(profiles.schema_version, ''),
    COALESCE(profiles.parser_model, ''),
    COALESCE(match_counts.match_count, 0)::int
FROM public.scans AS scans
LEFT JOIN public.structured_profiles AS profiles ON profiles.id = scans.profile_id
LEFT JOIN (
    SELECT scan_id, count(*)::int AS match_count
    FROM public.scan_matches
    GROUP BY scan_id
) AS match_counts ON match_counts.scan_id = scans.id
WHERE scans.client_user_id = $1
ORDER BY scans.created_at DESC, scans.id DESC
LIMIT $2 OFFSET $3`

const deleteClientCVQuery = `
DELETE FROM public.scans
WHERE id = $1 AND client_user_id = $2
RETURNING profile_id`

const lockCVProfileQuery = `
SELECT id
FROM public.structured_profiles
WHERE id = $1
FOR UPDATE`

const deleteOrphanCVProfileQuery = `
DELETE FROM public.structured_profiles AS profiles
WHERE profiles.id = $1
  AND NOT EXISTS (
      SELECT 1
      FROM public.scans AS remaining_scans
      WHERE remaining_scans.profile_id = profiles.id
  )`

type PostgresClientCVRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresClientCVRepository(pool *pgxpool.Pool) *PostgresClientCVRepository {
	return &PostgresClientCVRepository{pool: pool}
}

func (r *PostgresClientCVRepository) ListClientCVHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.ClientCVHistoryItem, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, countClientCVHistoryQuery, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, listClientCVHistoryQuery, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]model.ClientCVHistoryItem, 0)
	for rows.Next() {
		item, scanErr := scanClientCVHistoryRow(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func scanClientCVHistoryRow(rows pgx.Rows) (model.ClientCVHistoryItem, error) {
	var (
		item           model.ClientCVHistoryItem
		status         string
		profileID      pgtype.UUID
		roles          []string
		skills         []string
		seniority      string
		domains        []string
		years          float64
		education      []byte
		certifications []byte
		schemaVersion  string
		parserModel    string
	)
	if err := rows.Scan(
		&item.ScanID,
		&status,
		&item.Location,
		&item.CreatedAt,
		&item.UpdatedAt,
		&profileID,
		&roles,
		&skills,
		&years,
		&seniority,
		&domains,
		&education,
		&certifications,
		&schemaVersion,
		&parserModel,
		&item.MatchCount,
	); err != nil {
		return model.ClientCVHistoryItem{}, err
	}
	item.Status = model.ScanStatus(status)
	if !item.Status.IsValid() {
		return model.ClientCVHistoryItem{}, errors.New("invalid client cv scan status")
	}
	if profileID.Valid {
		var profile model.StructuredProfile
		if err := json.Unmarshal(education, &profile.Education); err != nil {
			return model.ClientCVHistoryItem{}, err
		}
		if err := json.Unmarshal(certifications, &profile.Certifications); err != nil {
			return model.ClientCVHistoryItem{}, err
		}
		profile.Roles = roles
		profile.Skills = skills
		profile.YearsOfExperience = years
		profile.Seniority = seniority
		profile.Domains = domains
		_ = schemaVersion
		_ = parserModel
		item.Profile = &profile
	}
	return item, nil
}

func (r *PostgresClientCVRepository) DeleteClientCV(ctx context.Context, userID, scanID uuid.UUID) error {
	return deleteCVScan(ctx, r.pool, deleteClientCVQuery, ErrClientCVNotFound, scanID, userID)
}

func deleteCVScan(ctx context.Context, pool *pgxpool.Pool, deleteScanQuery string, notFoundError error, args ...any) error {
	// PostgreSQL gives sibling data-modifying CTEs one snapshot, so an orphan
	// check in the same statement still sees the scan being deleted. Separate
	// commands in this Read Committed transaction give cleanup a fresh snapshot
	// while still rolling the scan deletion back if cleanup cannot complete.
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var profileID pgtype.UUID
	if err := tx.QueryRow(ctx, deleteScanQuery, args...).Scan(&profileID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notFoundError
		}
		return err
	}
	if profileID.Valid {
		// Serialize reference cleanup on the profile row. If another transaction
		// is deleting a scan for this profile, the waiter acquires this lock after
		// that transaction commits; its following Read Committed command then sees
		// the committed deletion and can reliably remove the final orphan.
		var lockedProfileID pgtype.UUID
		if err := tx.QueryRow(ctx, lockCVProfileQuery, profileID).Scan(&lockedProfileID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, deleteOrphanCVProfileQuery, profileID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

var _ ClientCVRepository = (*PostgresClientCVRepository)(nil)
