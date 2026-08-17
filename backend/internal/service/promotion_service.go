package service

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
)

// PromotionAsset is the provider metadata persisted after a successful
// Cloudinary upload. It intentionally contains no image bytes.
type PromotionAsset struct {
	PublicID  string
	AssetID   string
	SecureURL string
}

// PromotionAssetStore isolates external media storage from PostgreSQL. The
// production implementation uses Cloudinary; tests can use a deterministic
// fake without making network calls.
type PromotionAssetStore interface {
	Upload(context.Context, []byte, string, int16, string) (PromotionAsset, error)
	Destroy(context.Context, string) error
}

var ErrPromotionStorage = errors.New("promotion storage failure")

// PromotionInput is the untrusted admin form after the HTTP layer has parsed
// multipart fields. The current admin UI sends only File and Slot; the
// optional presentation fields remain accepted so old rows/clients can be
// migrated without a destructive schema change. Validation still happens here
// because this service owns promotion invariants for every caller.
type PromotionInput struct {
	Slot      int16
	File      *multipart.FileHeader
	AltText   string
	Eyebrow   string
	Title     string
	Body      string
	TargetURL string
}

// PromotionService validates promotion rules before delegating persistence.
// It never exposes raw image bytes through the metadata list operation.
type PromotionService struct {
	repository repository.PromotionRepository
	maxBytes   int64
	assets     PromotionAssetStore
}

// NewPromotionService creates the promotion use-case boundary with the same
// upload limit used by the HTTP request-size guard.
func NewPromotionService(promotionRepository repository.PromotionRepository, cfg config.Config, assetStores ...PromotionAssetStore) *PromotionService {
	maxBytes := cfg.MaxPromotionImageBytes
	if maxBytes <= 0 {
		maxBytes = 5 * 1024 * 1024
	}
	var assets PromotionAssetStore
	if len(assetStores) > 0 {
		assets = assetStores[0]
	}
	return &PromotionService{repository: promotionRepository, maxBytes: maxBytes, assets: assets}
}

// List returns active metadata for the client carousel; inactive slots remain
// invisible to public callers.
func (s *PromotionService) List(ctx context.Context) ([]model.Promotion, error) {
	return s.repository.ListActive(ctx)
}

// GetImage validates the slot before touching the database so malformed public
// URLs cannot become arbitrary database lookups.
func (s *PromotionService) GetImage(ctx context.Context, slot int16) (repository.PromotionImage, error) {
	if err := validatePromotionSlot(slot); err != nil {
		return repository.PromotionImage{}, err
	}
	return s.repository.GetActiveImage(ctx, slot)
}

// Upsert applies all presentation, URL, size, and binary-signature checks
// before storing a replacement for the requested slot.
func (s *PromotionService) Upsert(ctx context.Context, input PromotionInput) (model.Promotion, error) {
	if err := validatePromotionSlot(input.Slot); err != nil {
		return model.Promotion{}, err
	}
	altText := strings.TrimSpace(input.AltText)
	if altText == "" {
		// The public image still needs meaningful accessibility metadata, but the
		// admin upload flow is image-only because the artwork already contains
		// its designed copy. Keep the fallback deterministic by slot.
		altText = defaultPromotionAltText(input.Slot)
	}
	if err := validatePromotionAltText(altText); err != nil {
		return model.Promotion{}, err
	}
	if err := validatePromotionCopy(input.Eyebrow, input.Title, input.Body); err != nil {
		return model.Promotion{}, err
	}
	if err := validatePromotionTargetURL(input.TargetURL); err != nil {
		return model.Promotion{}, err
	}

	imageBytes, mimeType, contentHash, err := readPromotionImage(input.File, s.maxBytes)
	if err != nil {
		return model.Promotion{}, err
	}

	write := repository.PromotionWrite{
		Slot:        input.Slot,
		MIMEType:    mimeType,
		ContentHash: contentHash,
		AltText:     altText,
		Eyebrow:     optionalPromotionText(input.Eyebrow),
		Title:       optionalPromotionText(input.Title),
		Body:        optionalPromotionText(input.Body),
		TargetURL:   optionalPromotionText(input.TargetURL),
	}
	if s.assets == nil {
		// The compatibility path keeps existing unit callers and legacy rows
		// readable. The API runtime supplies a Cloudinary store, so new production
		// writes never enter this branch.
		write.StorageProvider = "DATABASE"
		write.ImageBytes = imageBytes
		return s.repository.Upsert(ctx, write)
	}

	previous, err := s.repository.GetActiveImage(ctx, input.Slot)
	if err != nil && !errors.Is(err, repository.ErrPromotionNotFound) {
		return model.Promotion{}, fmt.Errorf("read previous promotion asset: %w", ErrPromotionStorage)
	}
	asset, err := s.assets.Upload(ctx, imageBytes, mimeType, input.Slot, contentHash)
	if err != nil {
		return model.Promotion{}, fmt.Errorf("upload promotion asset: %w", ErrPromotionStorage)
	}
	write.StorageProvider = "CLOUDINARY"
	write.CloudinaryPublicID = asset.PublicID
	write.CloudinaryAssetID = asset.AssetID
	write.CloudinarySecureURL = asset.SecureURL
	promotion, err := s.repository.Upsert(ctx, write)
	if err != nil {
		_ = s.assets.Destroy(ctx, asset.PublicID)
		return model.Promotion{}, fmt.Errorf("persist promotion asset: %w", ErrPromotionStorage)
	}
	if previous.CloudinaryPublicID != "" && previous.CloudinaryPublicID != asset.PublicID {
		// The new row is already committed, so cleanup is best-effort. A provider
		// outage must not make the user retry a successful database update.
		_ = s.assets.Destroy(ctx, previous.CloudinaryPublicID)
	}
	return promotion, nil
}

func defaultPromotionAltText(slot int16) string {
	return fmt.Sprintf("Ảnh quảng bá Ferris - Slot %02d", slot)
}

// Delete removes one slot after validating its bounded slot number. The
// repository makes the operation idempotent for safe admin retries.
func (s *PromotionService) Delete(ctx context.Context, slot int16) error {
	if err := validatePromotionSlot(slot); err != nil {
		return err
	}
	return s.repository.Delete(ctx, slot)
}

func optionalPromotionText(value string) *string {
	// Empty optional copy is stored as NULL rather than an empty string so the
	// database and JSON response have one consistent representation.
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
