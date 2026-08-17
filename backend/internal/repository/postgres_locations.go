package repository

import (
	"context"
	"errors"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const listLocationsQuery = `
SELECT locations.id,
       locations.display_name,
       locations.province,
       locations.country,
       locations.canonical_key,
       locations.latitude::double precision,
       locations.longitude::double precision,
       locations.is_active,
       count(jobs.id)::int,
       locations.created_at,
       locations.updated_at
FROM public.locations AS locations
LEFT JOIN public.job_cache AS jobs ON jobs.location_id = locations.id
GROUP BY locations.id
ORDER BY locations.is_active DESC, lower(locations.display_name), locations.id`

const countLocationsQuery = `
SELECT count(*)
FROM public.locations`

const listLocationOptionsQuery = `
SELECT id, display_name, is_active
FROM public.locations
ORDER BY is_active DESC, lower(display_name), id`

const listActiveLocationsQuery = `
SELECT id,
       display_name,
       province,
       country,
       latitude::double precision,
       longitude::double precision,
       is_active
FROM public.locations
WHERE is_active = true
ORDER BY lower(display_name), id`

const createLocationQuery = `
INSERT INTO public.locations (display_name, province, country, canonical_key, is_active)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, display_name, province, country, canonical_key, latitude::double precision, longitude::double precision, is_active, created_at, updated_at`

const updateLocationQuery = `
UPDATE public.locations
SET display_name = $2,
    province = $3,
    country = $4,
    canonical_key = $5,
    is_active = $6,
    updated_at = now()
WHERE id = $1
RETURNING id, display_name, province, country, canonical_key, latitude::double precision, longitude::double precision, is_active, created_at, updated_at`

const assignJobLocationQuery = `
UPDATE public.job_cache
SET location_id = $2, updated_at = now()
WHERE id = $1`

type PostgresLocationRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresLocationRepository(pool *pgxpool.Pool) *PostgresLocationRepository {
	return &PostgresLocationRepository{pool: pool}
}

func (r *PostgresLocationRepository) ListLocations(ctx context.Context, page, pageSize int) (model.AdminLocationPage, error) {
	var total int
	if err := r.pool.QueryRow(ctx, countLocationsQuery).Scan(&total); err != nil {
		return model.AdminLocationPage{}, err
	}
	rows, err := r.pool.Query(ctx, listLocationsQuery+" LIMIT $1 OFFSET $2", pageSize, (page-1)*pageSize)
	if err != nil {
		return model.AdminLocationPage{}, err
	}
	defer rows.Close()
	locations := make([]model.AdminLocation, 0, pageSize)
	for rows.Next() {
		var location model.AdminLocation
		var latitude, longitude pgtype.Float8
		if err := rows.Scan(&location.ID, &location.DisplayName, &location.Province, &location.Country, &location.CanonicalKey, &latitude, &longitude, &location.IsActive, &location.JobCount, &location.CreatedAt, &location.UpdatedAt); err != nil {
			return model.AdminLocationPage{}, err
		}
		location.Latitude = nullableFloat(latitude)
		location.Longitude = nullableFloat(longitude)
		locations = append(locations, location)
	}
	if err := rows.Err(); err != nil {
		return model.AdminLocationPage{}, err
	}
	return model.AdminLocationPage{Items: locations, Page: page, PageSize: pageSize, Total: total}, nil
}

func (r *PostgresLocationRepository) ListLocationOptions(ctx context.Context) ([]model.AdminLocationOption, error) {
	rows, err := r.pool.Query(ctx, listLocationOptionsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	options := make([]model.AdminLocationOption, 0)
	for rows.Next() {
		var option model.AdminLocationOption
		if err := rows.Scan(&option.ID, &option.DisplayName, &option.IsActive); err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return options, nil
}

func (r *PostgresLocationRepository) ListActiveLocations(ctx context.Context) ([]model.ClientLocation, error) {
	rows, err := r.pool.Query(ctx, listActiveLocationsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	locations := make([]model.ClientLocation, 0)
	for rows.Next() {
		var location model.ClientLocation
		var latitude, longitude pgtype.Float8
		if err := rows.Scan(&location.ID, &location.DisplayName, &location.Province, &location.Country, &latitude, &longitude, &location.IsActive); err != nil {
			return nil, err
		}
		location.Latitude = nullableFloat(latitude)
		location.Longitude = nullableFloat(longitude)
		locations = append(locations, location)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return locations, nil
}

func (r *PostgresLocationRepository) CreateLocation(ctx context.Context, write LocationWrite) (model.AdminLocation, error) {
	location, err := scanLocation(r.pool.QueryRow(ctx, createLocationQuery, write.DisplayName, write.Province, write.Country, write.CanonicalKey, write.IsActive))
	if isUniqueViolation(err) {
		return model.AdminLocation{}, ErrLocationConflict
	}
	return location, err
}

func (r *PostgresLocationRepository) UpdateLocation(ctx context.Context, id uuid.UUID, write LocationWrite) (model.AdminLocation, error) {
	location, err := scanLocation(r.pool.QueryRow(ctx, updateLocationQuery, id, write.DisplayName, write.Province, write.Country, write.CanonicalKey, write.IsActive))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AdminLocation{}, ErrLocationNotFound
	}
	if isUniqueViolation(err) {
		return model.AdminLocation{}, ErrLocationConflict
	}
	return location, err
}

func (r *PostgresLocationRepository) AssignJobLocation(ctx context.Context, jobID uuid.UUID, locationID *uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if locationID != nil {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.locations WHERE id = $1)`, *locationID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrLocationNotFound
		}
	}
	result, err := tx.Exec(ctx, assignJobLocationQuery, jobID, locationID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrJobNotFound
	}
	return tx.Commit(ctx)
}

type rowScanner interface {
	Scan(...any) error
}

func scanLocation(row rowScanner) (model.AdminLocation, error) {
	var location model.AdminLocation
	var latitude, longitude pgtype.Float8
	err := row.Scan(
		&location.ID,
		&location.DisplayName,
		&location.Province,
		&location.Country,
		&location.CanonicalKey,
		&latitude,
		&longitude,
		&location.IsActive,
		&location.CreatedAt,
		&location.UpdatedAt,
	)
	location.Latitude = nullableFloat(latitude)
	location.Longitude = nullableFloat(longitude)
	return location, err
}

func nullableFloat(value pgtype.Float8) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}

func isUniqueViolation(err error) bool {
	var pgError *pgconn.PgError
	return errors.As(err, &pgError) && pgError.Code == "23505"
}

var _ LocationRepository = (*PostgresLocationRepository)(nil)
