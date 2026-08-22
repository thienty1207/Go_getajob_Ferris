package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const listHomeSectionsQuery = `
SELECT
    COALESCE(hs.id, '00000000-0000-0000-0000-000000000000'::uuid),
    slots.slot,
    COALESCE(hs.layout, CASE slots.slot WHEN 1 THEN 'CONTENT_LEFT' WHEN 2 THEN 'IMAGE_LEFT' WHEN 3 THEN 'CONTENT_LEFT' ELSE 'MEDIA_STRIP' END),
    COALESCE(hs.is_active, false),
    COALESCE(hs.eyebrow, ''),
    COALESCE(hs.title, ''),
    COALESCE(hs.body, ''),
    COALESCE(hs.image_alt_text, ''),
    COALESCE(hs.image_content_hash, ''),
    COALESCE(hs.storage_provider, ''),
    COALESCE(hs.cloudinary_public_id, ''),
    COALESCE(hs.cloudinary_secure_url, ''),
    COALESCE(hs.cloudinary_asset_id, ''),
    COALESCE(hs.target_url, ''),
    COALESCE(hs.updated_by, ''),
    COALESCE(hs.created_at, TIMESTAMPTZ 'epoch'),
    COALESCE(hs.updated_at, TIMESTAMPTZ 'epoch')
FROM generate_series(1, 4) AS slots(slot)
LEFT JOIN public.home_sections hs ON hs.slot = slots.slot
WHERE NOT $1::boolean
   OR (hs.is_active = true AND (hs.slot <> 4 OR EXISTS (
       SELECT 1 FROM public.home_section_media media
       WHERE media.home_section_id = hs.id AND media.is_active = true
   )))
ORDER BY slots.slot`

const getHomeSectionQuery = `
SELECT id, slot, layout, is_active, eyebrow, title, body, image_alt_text,
       image_content_hash, storage_provider, cloudinary_public_id,
       cloudinary_secure_url, cloudinary_asset_id, target_url, updated_by,
       created_at, updated_at
FROM public.home_sections
WHERE slot = $1`

const upsertHomeSectionQuery = `
INSERT INTO public.home_sections
    (slot, layout, is_active, eyebrow, title, body, image_alt_text,
     image_content_hash, storage_provider, cloudinary_public_id,
     cloudinary_secure_url, cloudinary_asset_id, target_url, updated_by,
     created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, now(), now())
ON CONFLICT (slot) DO UPDATE SET
    layout = EXCLUDED.layout,
    is_active = EXCLUDED.is_active,
    eyebrow = EXCLUDED.eyebrow,
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    image_alt_text = EXCLUDED.image_alt_text,
	image_content_hash = COALESCE(EXCLUDED.image_content_hash, home_sections.image_content_hash),
	storage_provider = COALESCE(EXCLUDED.storage_provider, home_sections.storage_provider),
	cloudinary_public_id = COALESCE(EXCLUDED.cloudinary_public_id, home_sections.cloudinary_public_id),
	cloudinary_secure_url = COALESCE(EXCLUDED.cloudinary_secure_url, home_sections.cloudinary_secure_url),
	cloudinary_asset_id = COALESCE(EXCLUDED.cloudinary_asset_id, home_sections.cloudinary_asset_id),
    target_url = EXCLUDED.target_url,
    updated_by = EXCLUDED.updated_by,
    updated_at = now()
RETURNING id, slot, layout, is_active, eyebrow, title, body, image_alt_text,
          image_content_hash, storage_provider, cloudinary_public_id,
          cloudinary_secure_url, cloudinary_asset_id, target_url, updated_by,
          created_at, updated_at`

const listHomeSectionMediaQuery = `
SELECT id, sort_order, is_active, image_alt_text, image_content_hash,
       storage_provider, cloudinary_public_id, cloudinary_secure_url,
       cloudinary_asset_id, target_url, created_at, updated_at
FROM public.home_section_media
WHERE home_section_id = $1 AND ($2::boolean OR is_active = true)
ORDER BY sort_order ASC, created_at ASC`

const createHomeSectionMediaQuery = `
INSERT INTO public.home_section_media
    (home_section_id, section_slot, sort_order, is_active, image_alt_text,
     image_content_hash, storage_provider, cloudinary_public_id,
     cloudinary_secure_url, cloudinary_asset_id, target_url, created_at, updated_at)
VALUES ($1, 4, $2, $3, $4, $5, $6, $7, $8, $9, $10, now(), now())
RETURNING id, sort_order, is_active, image_alt_text, image_content_hash,
          storage_provider, cloudinary_public_id, cloudinary_secure_url,
          cloudinary_asset_id, target_url, created_at, updated_at`

const updateHomeSectionMediaQuery = `
UPDATE public.home_section_media
SET sort_order = COALESCE($2, sort_order),
    is_active = COALESCE($3, is_active),
    image_alt_text = COALESCE($4, image_alt_text),
    target_url = COALESCE($5, target_url),
    updated_at = now()
WHERE id = $1
RETURNING id, sort_order, is_active, image_alt_text, image_content_hash,
          storage_provider, cloudinary_public_id, cloudinary_secure_url,
          cloudinary_asset_id, target_url, created_at, updated_at`

const getHomeSectionMediaQuery = `
SELECT id, sort_order, is_active, image_alt_text, image_content_hash,
       storage_provider, cloudinary_public_id, cloudinary_secure_url,
       cloudinary_asset_id, target_url, created_at, updated_at
FROM public.home_section_media
WHERE id = $1`

const deleteHomeSectionMediaQuery = `
DELETE FROM public.home_section_media
WHERE id = $1
RETURNING id, sort_order, is_active, image_alt_text, image_content_hash,
          storage_provider, cloudinary_public_id, cloudinary_secure_url,
          cloudinary_asset_id, target_url, created_at, updated_at`

const lockHomeSectionAssetQuery = `
SELECT cloudinary_public_id
FROM public.home_sections
WHERE slot = $1
FOR UPDATE`

// The reference guards protect legacy rows created before Home public IDs
// became unique. A shared historical asset is not eligible until its final
// Home section/media owner has been changed or deleted.
const enqueueHomeAssetCleanupQuery = `
INSERT INTO public.home_asset_cleanup_queue (cloudinary_public_id, created_at, updated_at)
SELECT $1, now(), now()
WHERE NOT EXISTS (
    SELECT 1 FROM public.home_sections WHERE cloudinary_public_id = $1
)
AND NOT EXISTS (
    SELECT 1 FROM public.home_section_media WHERE cloudinary_public_id = $1
)
ON CONFLICT (cloudinary_public_id) DO NOTHING`

const claimHomeAssetCleanupQuery = `
WITH due AS (
    SELECT cleanup.id
    FROM public.home_asset_cleanup_queue cleanup
    WHERE cleanup.next_attempt_at <= now()
      AND NOT EXISTS (
          SELECT 1 FROM public.home_sections WHERE cloudinary_public_id = cleanup.cloudinary_public_id
      )
      AND NOT EXISTS (
          SELECT 1 FROM public.home_section_media WHERE cloudinary_public_id = cleanup.cloudinary_public_id
      )
    ORDER BY cleanup.next_attempt_at ASC, cleanup.created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT $1
)
UPDATE public.home_asset_cleanup_queue cleanup
SET attempt_count = cleanup.attempt_count + 1,
    next_attempt_at = now() + ($2::bigint * interval '1 second'),
    updated_at = now()
FROM due
WHERE cleanup.id = due.id
RETURNING cleanup.id, cleanup.cloudinary_public_id, cleanup.attempt_count`

const completeHomeAssetCleanupQuery = `
DELETE FROM public.home_asset_cleanup_queue
WHERE id = $1`

type PostgresHomeSectionRepository struct {
	db homeSectionDatabase
}

func NewPostgresHomeSectionRepository(pool *pgxpool.Pool) *PostgresHomeSectionRepository {
	return &PostgresHomeSectionRepository{db: &pgxHomeSectionDatabase{pool: pool}}
}

// These narrow adapters keep transaction ordering directly testable without
// replacing pgx or opening a database connection in unit tests.
type homeSectionRows interface {
	Close()
	Next() bool
	Scan(...any) error
	Err() error
}

type homeSectionTransaction interface {
	QueryRow(context.Context, string, ...any) homeRowScanner
	Exec(context.Context, string, ...any) error
	Commit(context.Context) error
	Rollback(context.Context) error
}

type homeSectionDatabase interface {
	Begin(context.Context) (homeSectionTransaction, error)
	Query(context.Context, string, ...any) (homeSectionRows, error)
	QueryRow(context.Context, string, ...any) homeRowScanner
	Exec(context.Context, string, ...any) error
}

type pgxHomeSectionDatabase struct {
	pool *pgxpool.Pool
}

func (db *pgxHomeSectionDatabase) Begin(ctx context.Context) (homeSectionTransaction, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &pgxHomeSectionTransaction{tx: tx}, nil
}

func (db *pgxHomeSectionDatabase) Query(ctx context.Context, query string, arguments ...any) (homeSectionRows, error) {
	return db.pool.Query(ctx, query, arguments...)
}

func (db *pgxHomeSectionDatabase) QueryRow(ctx context.Context, query string, arguments ...any) homeRowScanner {
	return db.pool.QueryRow(ctx, query, arguments...)
}

func (db *pgxHomeSectionDatabase) Exec(ctx context.Context, query string, arguments ...any) error {
	_, err := db.pool.Exec(ctx, query, arguments...)
	return err
}

type pgxHomeSectionTransaction struct {
	tx pgx.Tx
}

func (tx *pgxHomeSectionTransaction) QueryRow(ctx context.Context, query string, arguments ...any) homeRowScanner {
	return tx.tx.QueryRow(ctx, query, arguments...)
}

func (tx *pgxHomeSectionTransaction) Exec(ctx context.Context, query string, arguments ...any) error {
	_, err := tx.tx.Exec(ctx, query, arguments...)
	return err
}

func (tx *pgxHomeSectionTransaction) Commit(ctx context.Context) error {
	return tx.tx.Commit(ctx)
}

func (tx *pgxHomeSectionTransaction) Rollback(ctx context.Context) error {
	return tx.tx.Rollback(ctx)
}

func (r *PostgresHomeSectionRepository) ListHomeSections(ctx context.Context, publicOnly bool) ([]model.HomeSection, error) {
	rows, err := r.db.Query(ctx, listHomeSectionsQuery, publicOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sections := make([]model.HomeSection, 0, 4)
	for rows.Next() {
		section, err := scanHomeSection(rows)
		if err != nil {
			return nil, err
		}
		if section.ID != uuid.Nil {
			if err := r.loadMedia(ctx, &section, includeInactiveForHomeRead(publicOnly)); err != nil {
				return nil, err
			}
		}
		sections = append(sections, section)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sections, nil
}

func includeInactiveForHomeRead(publicOnly bool) bool {
	return !publicOnly
}

func (r *PostgresHomeSectionRepository) GetHomeSection(ctx context.Context, slot int16) (model.HomeSection, error) {
	section, err := scanHomeSection(r.db.QueryRow(ctx, getHomeSectionQuery, slot))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.HomeSection{}, ErrHomeSectionNotFound
	}
	if err != nil {
		return model.HomeSection{}, err
	}
	if err := r.loadMedia(ctx, &section, true); err != nil {
		return model.HomeSection{}, err
	}
	return section, nil
}

func (r *PostgresHomeSectionRepository) UpsertHomeSection(ctx context.Context, write model.HomeSectionWrite) (model.HomeSection, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.HomeSection{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var previousPublicID *string
	if err := tx.QueryRow(ctx, lockHomeSectionAssetQuery, write.Slot).Scan(&previousPublicID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return model.HomeSection{}, err
	}
	section, err := scanHomeSection(tx.QueryRow(ctx, upsertHomeSectionQuery,
		write.Slot, write.Layout, write.IsActive, write.Eyebrow, write.Title, write.Body,
		write.ImageAltText, write.ImageContentHash, write.StorageProvider,
		write.CloudinaryPublicID, write.CloudinarySecureURL, write.CloudinaryAssetID,
		write.TargetURL, write.UpdatedBy,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.HomeSection{}, ErrHomeSectionNotFound
	}
	if err != nil {
		return model.HomeSection{}, err
	}
	oldPublicID := stringValue(previousPublicID)
	if oldPublicID != "" && oldPublicID != section.CloudinaryPublicID {
		if err := tx.Exec(ctx, enqueueHomeAssetCleanupQuery, oldPublicID); err != nil {
			return model.HomeSection{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return model.HomeSection{}, err
	}
	return section, nil
}

func (r *PostgresHomeSectionRepository) CreateHomeSectionMedia(ctx context.Context, write model.HomeSectionMediaWrite) (model.HomeSectionMedia, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.HomeSectionMedia{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var sectionID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM public.home_sections WHERE slot = 4 FOR UPDATE`).Scan(&sectionID); errors.Is(err, pgx.ErrNoRows) {
		return model.HomeSectionMedia{}, ErrHomeSectionNotFound
	} else if err != nil {
		return model.HomeSectionMedia{}, err
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.home_section_media WHERE home_section_id = $1`, sectionID).Scan(&count); err != nil {
		return model.HomeSectionMedia{}, err
	}
	if count >= 10 {
		return model.HomeSectionMedia{}, ErrHomeSectionMediaLimit
	}
	media, err := scanHomeSectionMedia(tx.QueryRow(ctx, createHomeSectionMediaQuery,
		sectionID, write.SortOrder, write.IsActive, write.ImageAltText, write.ImageContentHash,
		write.StorageProvider, write.CloudinaryPublicID, write.CloudinarySecureURL,
		write.CloudinaryAssetID, write.TargetURL,
	))
	if err != nil {
		return model.HomeSectionMedia{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.HomeSectionMedia{}, err
	}
	return media, nil
}

func (r *PostgresHomeSectionRepository) UpdateHomeSectionMedia(ctx context.Context, id uuid.UUID, update model.HomeSectionMediaUpdate) (model.HomeSectionMedia, error) {
	media, err := scanHomeSectionMedia(r.db.QueryRow(ctx, updateHomeSectionMediaQuery,
		id, update.SortOrder, update.IsActive, update.ImageAltText, update.TargetURL,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.HomeSectionMedia{}, ErrHomeSectionMediaNotFound
	}
	return media, err
}

func (r *PostgresHomeSectionRepository) GetHomeSectionMedia(ctx context.Context, id uuid.UUID) (model.HomeSectionMedia, error) {
	media, err := scanHomeSectionMedia(r.db.QueryRow(ctx, getHomeSectionMediaQuery, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.HomeSectionMedia{}, ErrHomeSectionMediaNotFound
	}
	return media, err
}

func (r *PostgresHomeSectionRepository) DeleteHomeSectionMedia(ctx context.Context, id uuid.UUID) (model.HomeSectionMedia, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.HomeSectionMedia{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	media, err := scanHomeSectionMedia(tx.QueryRow(ctx, deleteHomeSectionMediaQuery, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.HomeSectionMedia{}, ErrHomeSectionMediaNotFound
	}
	if err != nil {
		return model.HomeSectionMedia{}, err
	}
	if media.CloudinaryPublicID != "" {
		if err := tx.Exec(ctx, enqueueHomeAssetCleanupQuery, media.CloudinaryPublicID); err != nil {
			return model.HomeSectionMedia{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return model.HomeSectionMedia{}, err
	}
	return media, nil
}

func (r *PostgresHomeSectionRepository) EnqueueHomeAssetCleanup(ctx context.Context, publicID string) error {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return nil
	}
	return r.db.Exec(ctx, enqueueHomeAssetCleanupQuery, publicID)
}

func (r *PostgresHomeSectionRepository) ClaimHomeAssetCleanup(ctx context.Context, limit int, retryAfter time.Duration) ([]HomeAssetCleanup, error) {
	if limit <= 0 {
		limit = 10
	} else if limit > 100 {
		limit = 100
	}
	retrySeconds := int64(retryAfter / time.Second)
	if retrySeconds < 1 {
		retrySeconds = 60
	}
	rows, err := r.db.Query(ctx, claimHomeAssetCleanupQuery, limit, retrySeconds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]HomeAssetCleanup, 0, limit)
	for rows.Next() {
		var job HomeAssetCleanup
		if err := rows.Scan(&job.ID, &job.PublicID, &job.AttemptCount); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *PostgresHomeSectionRepository) CompleteHomeAssetCleanup(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return nil
	}
	return r.db.Exec(ctx, completeHomeAssetCleanupQuery, id)
}

func (r *PostgresHomeSectionRepository) loadMedia(ctx context.Context, section *model.HomeSection, includeInactive bool) error {
	if section == nil || section.ID == uuid.Nil || section.Slot != 4 {
		return nil
	}
	rows, err := r.db.Query(ctx, listHomeSectionMediaQuery, section.ID, includeInactive)
	if err != nil {
		return err
	}
	defer rows.Close()
	section.Media = make([]model.HomeSectionMedia, 0, 10)
	for rows.Next() {
		media, err := scanHomeSectionMedia(rows)
		if err != nil {
			return err
		}
		section.Media = append(section.Media, media)
	}
	return rows.Err()
}

type homeRowScanner interface {
	Scan(...any) error
}

func scanHomeSection(row homeRowScanner) (model.HomeSection, error) {
	var section model.HomeSection
	var eyebrow, title, body, imageAltText, imageHash, storageProvider, publicID, imageURL, assetID, targetURL, updatedBy *string
	if err := row.Scan(
		&section.ID, &section.Slot, &section.Layout, &section.IsActive,
		&eyebrow, &title, &body, &imageAltText,
		&imageHash, &storageProvider, &publicID,
		&imageURL, &assetID, &targetURL, &updatedBy,
		&section.CreatedAt, &section.UpdatedAt,
	); err != nil {
		return model.HomeSection{}, err
	}
	section.Eyebrow = stringValue(eyebrow)
	section.Title = stringValue(title)
	section.Body = stringValue(body)
	section.ImageAltText = stringValue(imageAltText)
	section.ImageContentHash = stringValue(imageHash)
	section.StorageProvider = stringValue(storageProvider)
	section.CloudinaryPublicID = stringValue(publicID)
	section.ImageURL = stringValue(imageURL)
	section.CloudinaryAssetID = stringValue(assetID)
	section.TargetURL = stringValue(targetURL)
	section.UpdatedBy = stringValue(updatedBy)
	section.Layout = model.HomeSectionLayout(strings.ToUpper(string(section.Layout)))
	section.Media = []model.HomeSectionMedia{}
	return section, nil
}

func scanHomeSectionMedia(row homeRowScanner) (model.HomeSectionMedia, error) {
	var media model.HomeSectionMedia
	var targetURL *string
	if err := row.Scan(
		&media.ID, &media.SortOrder, &media.IsActive, &media.ImageAltText,
		&media.ImageContentHash, &media.StorageProvider, &media.CloudinaryPublicID,
		&media.ImageURL, &media.CloudinaryAssetID, &targetURL,
		&media.CreatedAt, &media.UpdatedAt,
	); err != nil {
		return model.HomeSectionMedia{}, err
	}
	media.TargetURL = stringValue(targetURL)
	return media, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var _ HomeSectionRepository = (*PostgresHomeSectionRepository)(nil)
