package repository

import (
	"context"
	"errors"
	"strconv"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The metadata query intentionally excludes image_bytes. Public list traffic
// should not pull potentially large binaries into memory for every request.
const listActivePromotionsQuery = `
SELECT id, slot, content_hash, alt_text, eyebrow, title, body, target_url
FROM public.promotion_slides
WHERE is_active = true
ORDER BY slot ASC
LIMIT 3`

// Images are read separately so the handler can attach MIME, ETag, and
// immutable-cache headers without bloating the JSON contract.
const activePromotionImageQuery = `
SELECT slot, image_bytes, mime_type, content_hash, storage_provider,
       cloudinary_public_id, cloudinary_secure_url, cloudinary_asset_id
FROM public.promotion_slides
WHERE slot = $1 AND is_active = true`

// ON CONFLICT(slot) makes replacing a slide idempotent: retrying an admin
// request updates the same logical slot instead of creating duplicates.
const upsertPromotionQuery = `
INSERT INTO public.promotion_slides
    (slot, image_bytes, mime_type, content_hash, alt_text, eyebrow, title, body, target_url,
     is_active, storage_provider, cloudinary_public_id, cloudinary_secure_url, cloudinary_asset_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, $10, $11, $12, $13, now(), now())
ON CONFLICT (slot) DO UPDATE SET
    image_bytes = EXCLUDED.image_bytes,
    mime_type = EXCLUDED.mime_type,
    content_hash = EXCLUDED.content_hash,
    alt_text = EXCLUDED.alt_text,
    eyebrow = EXCLUDED.eyebrow,
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    target_url = EXCLUDED.target_url,
    is_active = true,
    storage_provider = EXCLUDED.storage_provider,
    cloudinary_public_id = EXCLUDED.cloudinary_public_id,
    cloudinary_secure_url = EXCLUDED.cloudinary_secure_url,
    cloudinary_asset_id = EXCLUDED.cloudinary_asset_id,
    updated_at = now()
RETURNING id, slot, content_hash, alt_text, eyebrow, title, body, target_url`

const deletePromotionQuery = `
DELETE FROM public.promotion_slides
WHERE slot = $1`

type PostgresPromotionRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresPromotionRepository binds promotion persistence to the shared
// application pool; it does not open a second database connection.
func NewPostgresPromotionRepository(pool *pgxpool.Pool) *PostgresPromotionRepository {
	return &PostgresPromotionRepository{pool: pool}
}

// ListActive returns stable slot order and lets SQL enforce the public three
// item maximum before data reaches the HTTP layer.
func (r *PostgresPromotionRepository) ListActive(ctx context.Context) ([]model.Promotion, error) {
	rows, err := r.pool.Query(ctx, listActivePromotionsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	promotions := make([]model.Promotion, 0, 3)
	for rows.Next() {
		promotion, err := scanPromotion(rows)
		if err != nil {
			return nil, err
		}
		promotions = append(promotions, promotion)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return promotions, nil
}

// GetActiveImage reads one active slot's binary payload for the image route.
func (r *PostgresPromotionRepository) GetActiveImage(ctx context.Context, slot int16) (PromotionImage, error) {
	var (
		image               PromotionImage
		cloudinaryPublicID  *string
		cloudinarySecureURL *string
		cloudinaryAssetID   *string
	)
	err := r.pool.QueryRow(ctx, activePromotionImageQuery, slot).Scan(
		&image.Slot,
		&image.ImageBytes,
		&image.MIMEType,
		&image.ContentHash,
		&image.StorageProvider,
		&cloudinaryPublicID,
		&cloudinarySecureURL,
		&cloudinaryAssetID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PromotionImage{}, ErrPromotionNotFound
	}
	if err != nil {
		return PromotionImage{}, err
	}
	image.CloudinaryPublicID = stringOrEmpty(cloudinaryPublicID)
	image.CloudinarySecureURL = stringOrEmpty(cloudinarySecureURL)
	image.CloudinaryAssetID = stringOrEmpty(cloudinaryAssetID)
	return image, nil
}

// Upsert replaces all mutable content for one slot atomically and returns the
// metadata needed by the admin caller after the write succeeds.
func (r *PostgresPromotionRepository) Upsert(ctx context.Context, write PromotionWrite) (model.Promotion, error) {
	var (
		promotion model.Promotion
		id        uuid.UUID
	)
	storageProvider := write.StorageProvider
	if storageProvider == "" {
		storageProvider = "DATABASE"
	}
	err := r.pool.QueryRow(ctx, upsertPromotionQuery,
		write.Slot,
		write.ImageBytes,
		write.MIMEType,
		write.ContentHash,
		write.AltText,
		write.Eyebrow,
		write.Title,
		write.Body,
		write.TargetURL,
		storageProvider,
		nullableString(write.CloudinaryPublicID),
		nullableString(write.CloudinarySecureURL),
		nullableString(write.CloudinaryAssetID),
	).Scan(
		&id,
		&promotion.Slot,
		&promotion.ContentHash,
		&promotion.AltText,
		&promotion.Eyebrow,
		&promotion.Title,
		&promotion.Body,
		&promotion.TargetURL,
	)
	if err != nil {
		return model.Promotion{}, err
	}
	promotion.ID = id
	promotion.ImageURL = promotionImageURL(promotion.Slot, promotion.ContentHash)
	return promotion, nil
}

// Delete is intentionally idempotent: deleting an already empty slot still
// produces a successful operation for admin retry safety.
func (r *PostgresPromotionRepository) Delete(ctx context.Context, slot int16) error {
	_, err := r.pool.Exec(ctx, deletePromotionQuery, slot)
	return err
}

func scanPromotion(row pgx.Row) (model.Promotion, error) {
	var promotion model.Promotion
	if err := row.Scan(&promotion.ID, &promotion.Slot, &promotion.ContentHash, &promotion.AltText, &promotion.Eyebrow, &promotion.Title, &promotion.Body, &promotion.TargetURL); err != nil {
		return model.Promotion{}, err
	}
	promotion.ImageURL = promotionImageURL(promotion.Slot, promotion.ContentHash)
	return promotion, nil
}

func promotionImageURL(slot int16, contentHash string) string {
	// The hash query parameter invalidates the browser's old metadata URL when
	// an admin replaces the image while the binary route remains stable.
	return "/api/v1/client/promotions/" + strconv.FormatInt(int64(slot), 10) + "/image?v=" + contentHash
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
