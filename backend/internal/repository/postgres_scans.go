package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The scan transaction creates the row and immediately moves it to PARSING so
// a caller never observes a durable RECEIVED row that has no processor work.
const createScanQuery = `
INSERT INTO public.scans (status, location_id, location_text, latitude, longitude, radius_km)
SELECT 'RECEIVED', id, display_name, latitude, longitude, $2
FROM public.locations
WHERE id = $1 AND is_active = true
RETURNING id`

const markParsingStatusQuery = `
UPDATE public.scans
SET status = 'PARSING', updated_at = now()
WHERE id = $1 AND status = 'RECEIVED'`

const verifyCommittedScanQuery = `
SELECT status
FROM public.scans
WHERE id = $1`

// State transitions are guarded in SQL as a second line of defense. A service
// bug cannot arbitrarily move a scan backward or skip the allowed lifecycle.
const setScanStatusQuery = `
UPDATE public.scans
SET status = $2, error_code = $3, updated_at = now()
WHERE id = $1
  AND (
      (status = 'RECEIVED' AND $2 = 'PARSING')
      OR (status = 'PARSING' AND $2 IN ('MATCHING', 'FAILED'))
      OR (status = 'MATCHING' AND $2 IN ('COMPLETED', 'FAILED'))
      OR (status = 'FAILED' AND $2 = 'FAILED' AND error_code = $3)
  )`

const getScanQuery = `
SELECT id, status, error_code, location_id, location_text, latitude::double precision, longitude::double precision, radius_km::double precision
FROM public.scans
WHERE id = $1`

// Public matches come from active_job_cache, which hides closed jobs and
// unapproved sources from client responses.
const completedMatchesQuery = `
SELECT
    matches.id,
    matches.match_percent::double precision,
    jobs.title,
    jobs.company,
    jobs.location_text,
    matches.distance_km::double precision,
    jobs.employment_type,
    jobs.work_mode,
    jobs.salary_source_text,
    jobs.salary_currency,
    COALESCE(jobs.required_skills[1:3], ARRAY[]::text[]) AS skill_tags,
    jobs.original_url
FROM public.scan_matches AS matches
JOIN public.active_job_cache AS jobs ON jobs.id = matches.job_id
WHERE matches.scan_id = $1
ORDER BY matches.match_percent DESC, matches.distance_km NULLS LAST, matches.id
LIMIT 100`

const listMatchCandidatesQuery = `
WITH scan_context AS (
    SELECT id,
           location_id,
           latitude::double precision AS scan_latitude,
           longitude::double precision AS scan_longitude,
           radius_km::double precision AS radius_km
    FROM public.scans
    WHERE id = $1
)
SELECT
    jobs.id,
    jobs.title,
    jobs.company,
    jobs.location_text,
    jobs.role,
    jobs.required_skills,
    jobs.preferred_skills,
    jobs.seniority,
    jobs.minimum_experience_years::double precision,
    jobs.domains,
    jobs.employment_type,
    jobs.work_mode,
    jobs.salary_source_text,
    jobs.salary_currency,
    jobs.original_url,
    CASE
        WHEN jobs.work_mode = 'REMOTE' THEN NULL::double precision
        WHEN jobs.location_id = scan_context.location_id THEN 0::double precision
        WHEN scan_context.scan_latitude IS NOT NULL
             AND scan_context.scan_longitude IS NOT NULL
             AND jobs.latitude IS NOT NULL
             AND jobs.longitude IS NOT NULL
        THEN 6371.0 * acos(LEAST(1.0, GREATEST(-1.0,
            cos(radians(scan_context.scan_latitude)) * cos(radians(jobs.latitude))
            * cos(radians(jobs.longitude) - radians(scan_context.scan_longitude))
            + sin(radians(scan_context.scan_latitude)) * sin(radians(jobs.latitude))
        )))
        ELSE NULL::double precision
    END AS distance_km
FROM public.active_job_cache AS jobs
CROSS JOIN scan_context
WHERE jobs.work_mode = 'REMOTE'
   OR jobs.location_id = scan_context.location_id
   OR (
       scan_context.scan_latitude IS NOT NULL
       AND scan_context.scan_longitude IS NOT NULL
       AND jobs.latitude IS NOT NULL
       AND jobs.longitude IS NOT NULL
       AND 6371.0 * acos(LEAST(1.0, GREATEST(-1.0,
           cos(radians(scan_context.scan_latitude)) * cos(radians(jobs.latitude))
           * cos(radians(jobs.longitude) - radians(scan_context.scan_longitude))
           + sin(radians(scan_context.scan_latitude)) * sin(radians(jobs.latitude))
       ))) <= scan_context.radius_km
   )
ORDER BY jobs.id`

const completeScanProfileQuery = `
INSERT INTO public.structured_profiles (
    roles, skills, years_of_experience, seniority, domains, education, certifications, schema_version, parser_model
)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, 'v1', $8)
RETURNING id`

const completeScanStatusQuery = `
UPDATE public.scans
SET profile_id = $2, status = 'MATCHING', updated_at = now()
WHERE id = $1 AND status = 'PARSING'`

const insertScanMatchQuery = `
INSERT INTO public.scan_matches (
    scan_id, job_id, required_skills_points, role_relevance_points, experience_points,
    seniority_points, preferred_skills_domain_points, match_percent, distance_km
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

const finishScanQuery = `
UPDATE public.scans
SET status = 'COMPLETED', updated_at = now()
WHERE id = $1 AND status = 'MATCHING'`

type PostgresScanRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresScanRepository binds scan persistence to the shared pool.
func NewPostgresScanRepository(pool *pgxpool.Pool) *PostgresScanRepository {
	return &PostgresScanRepository{pool: pool}
}

// CreateScan atomically creates and claims a scan for processing. If commit
// acknowledgement is interrupted, it verifies the row before retrying so a
// client does not accidentally receive a duplicate scan.
func (r *PostgresScanRepository) CreateScan(ctx context.Context, locationID uuid.UUID, radiusKm float64) (uuid.UUID, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	var id uuid.UUID
	if err := tx.QueryRow(ctx, createScanQuery, locationID, radiusKm).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrLocationNotFound
		}
		return uuid.Nil, err
	}
	commandTag, err := tx.Exec(ctx, markParsingStatusQuery, id)
	if err != nil {
		return uuid.Nil, err
	}
	if commandTag.RowsAffected() != 1 {
		return uuid.Nil, ErrInvalidScanState
	}
	if err := tx.Commit(ctx); err != nil {
		if recovered, _ := r.recoverCommittedParsingScan(id); recovered {
			return id, nil
		}
		return uuid.Nil, err
	}
	return id, nil
}

func (r *PostgresScanRepository) recoverCommittedParsingScan(id uuid.UUID) (bool, error) {
	// This bounded read distinguishes an unknown commit result from a failed
	// transaction without re-running the insert.
	recoveryCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var status model.ScanStatus
	err := r.pool.QueryRow(recoveryCtx, verifyCommittedScanQuery, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return status == model.StatusParsing, nil
}

// SetStatus validates the state/error-code pairing in Go and lets the guarded
// SQL predicate enforce legal transitions against concurrent writers.
func (r *PostgresScanRepository) SetStatus(ctx context.Context, id uuid.UUID, status model.ScanStatus, errorCode *string) error {
	if !status.IsValid() {
		return fmt.Errorf("%w: %q", ErrInvalidScanState, status)
	}
	if status == model.StatusFailed && (errorCode == nil || strings.TrimSpace(*errorCode) == "") {
		return fmt.Errorf("%w: failed status requires error code", ErrInvalidScanState)
	}

	commandTag, err := r.pool.Exec(ctx, setScanStatusQuery, id, status, errorCode)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrScanNotFound
	}
	return nil
}

// GetScan reads the scan and, for completed scans, their matches in one
// repeatable-read transaction so the status and result set are consistent.
func (r *PostgresScanRepository) GetScan(ctx context.Context, id uuid.UUID) (model.Scan, error) {
	var (
		scan       model.Scan
		status     string
		errorCode  pgtype.Text
		locationID pgtype.UUID
		latitude   pgtype.Float8
		longitude  pgtype.Float8
	)

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return model.Scan{}, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, getScanQuery, id).Scan(
		&scan.ID,
		&status,
		&errorCode,
		&locationID,
		&scan.Location,
		&latitude,
		&longitude,
		&scan.RadiusKm,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Scan{}, ErrScanNotFound
	}
	if err != nil {
		return model.Scan{}, err
	}
	scan.Status = model.ScanStatus(status)
	if locationID.Valid {
		value := uuid.UUID(locationID.Bytes)
		scan.LocationID = &value
	}
	if latitude.Valid {
		value := latitude.Float64
		scan.Latitude = &value
	}
	if longitude.Valid {
		value := longitude.Float64
		scan.Longitude = &value
	}
	if !scan.Status.IsValid() {
		return model.Scan{}, fmt.Errorf("%w: %q", ErrInvalidScanState, status)
	}
	if errorCode.Valid {
		scan.ErrorCode = errorCode.String
	}

	if scan.Status != model.StatusCompleted {
		return scan, nil
	}

	rows, err := tx.Query(ctx, completedMatchesQuery, id)
	if err != nil {
		return model.Scan{}, err
	}
	defer rows.Close()

	scan.Matches = make([]model.JobMatch, 0)
	for rows.Next() {
		match, scanErr := scanMatchRow(rows)
		if scanErr != nil {
			return model.Scan{}, scanErr
		}
		scan.Matches = append(scan.Matches, match)
	}
	if err := rows.Err(); err != nil {
		return model.Scan{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Scan{}, err
	}
	return scan, nil
}

func (r *PostgresScanRepository) LoadScanContext(ctx context.Context, id uuid.UUID) (model.Scan, error) {
	return r.GetScan(ctx, id)
}

func (r *PostgresScanRepository) ListMatchCandidates(ctx context.Context, scan model.Scan) ([]model.JobCandidate, error) {
	rows, err := r.pool.Query(ctx, listMatchCandidatesQuery, scan.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	candidates := make([]model.JobCandidate, 0)
	for rows.Next() {
		var candidate model.JobCandidate
		var minimumExperience, distanceKm pgtype.Float8
		var salaryDisplay, salaryCurrency pgtype.Text
		if err := rows.Scan(
			&candidate.ID,
			&candidate.Title,
			&candidate.Company,
			&candidate.Location,
			&candidate.Role,
			&candidate.RequiredSkills,
			&candidate.PreferredSkills,
			&candidate.Seniority,
			&minimumExperience,
			&candidate.Domains,
			&candidate.EmploymentType,
			&candidate.WorkMode,
			&salaryDisplay,
			&salaryCurrency,
			&candidate.OriginalURL,
			&distanceKm,
		); err != nil {
			return nil, err
		}
		if minimumExperience.Valid {
			value := minimumExperience.Float64
			candidate.MinimumExperience = &value
		}
		if salaryDisplay.Valid {
			value := salaryDisplay.String
			candidate.SalaryDisplay = &value
		}
		if salaryCurrency.Valid {
			value := salaryCurrency.String
			candidate.SalaryCurrency = &value
		}
		if distanceKm.Valid {
			value := distanceKm.Float64
			candidate.DistanceKm = &value
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candidates, nil
}

func (r *PostgresScanRepository) CompleteScan(ctx context.Context, scanID uuid.UUID, profile model.StructuredProfile, parserModel string, matches []model.ScoredJobMatch) error {
	education, err := json.Marshal(profile.Education)
	if err != nil {
		return err
	}
	certifications, err := json.Marshal(profile.Certifications)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var profileID uuid.UUID
	if err := tx.QueryRow(ctx, completeScanProfileQuery, profile.Roles, profile.Skills, profile.YearsOfExperience, profile.Seniority, profile.Domains, string(education), string(certifications), parserModel).Scan(&profileID); err != nil {
		return err
	}
	status, err := tx.Exec(ctx, completeScanStatusQuery, scanID, profileID)
	if err != nil {
		return err
	}
	if status.RowsAffected() != 1 {
		return ErrInvalidScanState
	}
	for _, match := range matches {
		if _, err := tx.Exec(ctx, insertScanMatchQuery, scanID, match.JobID, match.RequiredSkillsPoints, match.RoleRelevancePoints, match.ExperiencePoints, match.SeniorityPoints, match.PreferredSkillsDomainPoints, match.MatchPercent, match.DistanceKm); err != nil {
			return err
		}
	}
	finished, err := tx.Exec(ctx, finishScanQuery, scanID)
	if err != nil {
		return err
	}
	if finished.RowsAffected() != 1 {
		return ErrInvalidScanState
	}
	return tx.Commit(ctx)
}

// scanMatchRow converts nullable PostgreSQL fields into the pointer-shaped
// public model used by the client response mapper.
func scanMatchRow(rows pgx.Rows) (model.JobMatch, error) {
	var (
		match          model.JobMatch
		workMode       string
		distanceKm     pgtype.Float8
		salaryDisplay  pgtype.Text
		salaryCurrency pgtype.Text
	)

	err := rows.Scan(
		&match.ID,
		&match.MatchPercent,
		&match.Title,
		&match.Company,
		&match.Location,
		&distanceKm,
		&match.EmploymentType,
		&workMode,
		&salaryDisplay,
		&salaryCurrency,
		&match.SkillTags,
		&match.OriginalURL,
	)
	if err != nil {
		return model.JobMatch{}, err
	}
	match.WorkMode = strings.ToLower(workMode)
	if distanceKm.Valid {
		value := distanceKm.Float64
		match.DistanceKm = &value
	}
	if salaryDisplay.Valid && strings.TrimSpace(salaryDisplay.String) != "" {
		match.Salary = &model.JobSalary{Display: salaryDisplay.String}
		if salaryCurrency.Valid {
			match.Salary.Currency = salaryCurrency.String
		}
	}
	if len(match.SkillTags) > 3 {
		match.SkillTags = match.SkillTags[:3]
	}
	return match, nil
}
