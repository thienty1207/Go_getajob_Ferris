package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrInvalidHomeSection = errors.New("invalid home section")
	ErrHomeSectionStorage = errors.New("home section storage failure")
	ErrInvalidHomeMedia   = errors.New("invalid home section media")
)

type HomeSectionAssetStore interface {
	UploadHomeSection(context.Context, []byte, string, int16, string) (PromotionAsset, error)
	Destroy(context.Context, string) error
}

const (
	maxHomeEyebrow = 80
	maxHomeTitle   = 180
	maxHomeBody    = 1200
	maxHomeAlt     = 180
	maxHomeActor   = 320
	maxHomeMedia   = 10

	// Cloudinary cleanup runs outside request contexts. A short operation
	// deadline prevents a provider outage from blocking API shutdown forever,
	// while the PostgreSQL lease keeps failed work retryable.
	homeCleanupBatchSize        = 10
	homeCleanupPollInterval     = 30 * time.Second
	homeCleanupRetryAfter       = time.Minute
	homeCleanupOperationTimeout = 10 * time.Second
)

type HomeSectionInput struct {
	Slot         int16
	IsActive     bool
	Eyebrow      string
	Title        string
	Body         string
	ImageAltText string
	TargetURL    string
	File         *multipart.FileHeader
	UpdatedBy    string
}

type HomeSectionMediaInput struct {
	SortOrder    int16
	IsActive     bool
	ImageAltText string
	TargetURL    string
	File         *multipart.FileHeader
}

type HomeSectionService struct {
	repository repository.HomeSectionRepository
	assets     HomeSectionAssetStore
	maxBytes   int64

	cleanupCtx    context.Context
	cleanupCancel context.CancelFunc
	cleanupWake   chan struct{}
	cleanupStart  sync.Once
	cleanupClose  sync.Once
	cleanupWG     sync.WaitGroup
}

func NewHomeSectionService(homeRepository repository.HomeSectionRepository, cfg config.Config, assets HomeSectionAssetStore) *HomeSectionService {
	maxBytes := cfg.MaxPromotionImageBytes
	if maxBytes <= 0 {
		maxBytes = 5 * 1024 * 1024
	}
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	return &HomeSectionService{
		repository:    homeRepository,
		assets:        assets,
		maxBytes:      maxBytes,
		cleanupCtx:    cleanupCtx,
		cleanupCancel: cleanupCancel,
		cleanupWake:   make(chan struct{}, 1),
	}
}

// StartCleanupWorker begins the durable Cloudinary cleanup consumer. It is
// explicit so unit tests can exercise service methods without background
// timing, while the API process starts it exactly once during wiring.
func (s *HomeSectionService) StartCleanupWorker() {
	if s == nil || s.repository == nil || s.assets == nil {
		return
	}
	s.cleanupStart.Do(func() {
		s.cleanupWG.Add(1)
		go s.cleanupLoop()
	})
	s.signalCleanup()
}

// Close stops the cleanup consumer and waits for an in-flight bounded
// provider operation to finish or observe cancellation.
func (s *HomeSectionService) Close() {
	if s == nil {
		return
	}
	s.cleanupClose.Do(func() {
		if s.cleanupCancel != nil {
			s.cleanupCancel()
		}
	})
	s.cleanupWG.Wait()
}

func (s *HomeSectionService) List(ctx context.Context, publicOnly bool) ([]model.HomeSection, error) {
	return s.repository.ListHomeSections(ctx, publicOnly)
}

func (s *HomeSectionService) Upsert(ctx context.Context, input HomeSectionInput) (model.HomeSection, error) {
	layout, err := homeSectionLayout(input.Slot)
	if err != nil {
		return model.HomeSection{}, err
	}
	eyebrow := strings.TrimSpace(input.Eyebrow)
	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	altText := strings.TrimSpace(input.ImageAltText)
	updatedBy := strings.TrimSpace(input.UpdatedBy)
	if err := validateHomeText(eyebrow, maxHomeEyebrow, false, "eyebrow"); err != nil {
		return model.HomeSection{}, err
	}
	if err := validateHomeText(title, maxHomeTitle, input.IsActive, "title"); err != nil {
		return model.HomeSection{}, err
	}
	if err := validateHomeText(body, maxHomeBody, input.IsActive, "body"); err != nil {
		return model.HomeSection{}, err
	}
	if err := validatePromotionTargetURL(input.TargetURL); err != nil {
		return model.HomeSection{}, fmt.Errorf("%w: target URL", ErrInvalidHomeSection)
	}
	if err := validateHomeText(updatedBy, maxHomeActor, false, "updated by"); err != nil {
		return model.HomeSection{}, err
	}

	current, currentErr := s.repository.GetHomeSection(ctx, input.Slot)
	if currentErr != nil && !errors.Is(currentErr, repository.ErrHomeSectionNotFound) {
		return model.HomeSection{}, currentErr
	}
	hasImage := input.File != nil || (currentErr == nil && current.ImageContentHash != "")
	if hasImage {
		altText = deriveHomeImageAltText(title, altText, current.ImageAltText)
	}
	if err := validateHomeText(altText, maxHomeAlt, false, "alt"); err != nil {
		return model.HomeSection{}, err
	}
	if input.IsActive && !hasImage {
		return model.HomeSection{}, fmt.Errorf("%w: image is required", ErrInvalidHomeSection)
	}
	write := model.HomeSectionWrite{
		Slot: input.Slot, Layout: layout, IsActive: input.IsActive,
		Eyebrow: optionalHomeText(eyebrow), Title: optionalHomeText(title), Body: optionalHomeText(body),
		TargetURL: optionalHomeText(input.TargetURL),
		UpdatedBy: optionalHomeText(updatedBy),
	}
	var replacementPublicID string
	if input.File != nil {
		if s.assets == nil {
			return model.HomeSection{}, ErrHomeSectionStorage
		}
		imageBytes, mimeType, hash, err := readHomeSectionImage(input.File, s.maxBytes)
		if err != nil {
			return model.HomeSection{}, err
		}
		asset, err := s.assets.UploadHomeSection(ctx, imageBytes, mimeType, input.Slot, hash)
		if err != nil {
			return model.HomeSection{}, fmt.Errorf("%w: upload", ErrHomeSectionStorage)
		}
		write.ImageContentHash = stringPtr(hash)
		write.StorageProvider = stringPtr("CLOUDINARY")
		write.CloudinaryPublicID = stringPtr(asset.PublicID)
		write.CloudinarySecureURL = stringPtr(asset.SecureURL)
		write.CloudinaryAssetID = stringPtr(asset.AssetID)
		replacementPublicID = asset.PublicID
	}
	write.ImageAltText = optionalHomeText(altText)

	section, err := s.repository.UpsertHomeSection(ctx, write)
	if err != nil {
		if replacementPublicID != "" {
			// The database may have returned an ambiguous commit error. Queueing
			// is safe because the claimant rechecks that no Home row references
			// this public ID before deleting the provider asset.
			_ = s.enqueueCleanup(replacementPublicID)
		}
		return model.HomeSection{}, err
	}
	// Replacements enqueue their previous public ID in the same transaction as
	// the metadata update. Wake the worker only after that commit succeeds.
	s.signalCleanup()
	return section, nil
}

func (s *HomeSectionService) CreateMedia(ctx context.Context, input HomeSectionMediaInput) (model.HomeSectionMedia, error) {
	if input.File == nil || input.SortOrder < 0 || input.SortOrder >= maxHomeMedia {
		return model.HomeSectionMedia{}, ErrInvalidHomeMedia
	}
	section, sectionErr := s.repository.GetHomeSection(ctx, 4)
	if sectionErr != nil && !errors.Is(sectionErr, repository.ErrHomeSectionNotFound) {
		return model.HomeSectionMedia{}, sectionErr
	}
	if sectionErr == nil && len(section.Media) >= maxHomeMedia {
		return model.HomeSectionMedia{}, repository.ErrHomeSectionMediaLimit
	}
	altText := deriveHomeMediaAltText(input.SortOrder, input.ImageAltText)
	if err := validateHomeText(altText, maxHomeAlt, true, "alt"); err != nil {
		return model.HomeSectionMedia{}, fmt.Errorf("%w: %v", ErrInvalidHomeMedia, err)
	}
	if err := validatePromotionTargetURL(input.TargetURL); err != nil {
		return model.HomeSectionMedia{}, fmt.Errorf("%w: target URL", ErrInvalidHomeMedia)
	}
	if s.assets == nil {
		return model.HomeSectionMedia{}, ErrHomeSectionStorage
	}
	imageBytes, mimeType, hash, err := readHomeSectionImage(input.File, s.maxBytes)
	if err != nil {
		if errors.Is(err, ErrPromotionImageTooLarge) {
			return model.HomeSectionMedia{}, err
		}
		return model.HomeSectionMedia{}, fmt.Errorf("%w: %v", ErrInvalidHomeMedia, err)
	}
	asset, err := s.assets.UploadHomeSection(ctx, imageBytes, mimeType, 4, hash)
	if err != nil {
		return model.HomeSectionMedia{}, fmt.Errorf("%w: upload", ErrHomeSectionStorage)
	}
	if errors.Is(sectionErr, repository.ErrHomeSectionNotFound) {
		if _, err := s.repository.UpsertHomeSection(ctx, model.HomeSectionWrite{Slot: 4, Layout: model.HomeSectionMediaStrip, IsActive: true, UpdatedBy: stringPtr("system")}); err != nil {
			_ = s.enqueueCleanup(asset.PublicID)
			return model.HomeSectionMedia{}, err
		}
	}
	media, err := s.repository.CreateHomeSectionMedia(ctx, model.HomeSectionMediaWrite{
		SortOrder: input.SortOrder, IsActive: input.IsActive, ImageAltText: altText,
		ImageContentHash: hash, StorageProvider: "CLOUDINARY", CloudinaryPublicID: asset.PublicID,
		CloudinarySecureURL: asset.SecureURL, CloudinaryAssetID: asset.AssetID,
		TargetURL: optionalHomeText(input.TargetURL),
	})
	if err != nil {
		_ = s.enqueueCleanup(asset.PublicID)
		return model.HomeSectionMedia{}, err
	}
	return media, nil
}

func (s *HomeSectionService) UpdateMedia(ctx context.Context, id uuid.UUID, sortOrder *int16, isActive *bool, altText *string, targetURL *string) (model.HomeSectionMedia, error) {
	if id == uuid.Nil {
		return model.HomeSectionMedia{}, ErrInvalidHomeMedia
	}
	if sortOrder != nil && (*sortOrder < 0 || *sortOrder >= maxHomeMedia) {
		return model.HomeSectionMedia{}, ErrInvalidHomeMedia
	}
	if altText != nil {
		trimmed := strings.TrimSpace(*altText)
		if err := validateHomeText(trimmed, maxHomeAlt, true, "alt"); err != nil {
			return model.HomeSectionMedia{}, fmt.Errorf("%w: %v", ErrInvalidHomeMedia, err)
		}
		altText = stringPtr(trimmed)
	}
	if targetURL != nil {
		trimmed := strings.TrimSpace(*targetURL)
		if trimmed != "" {
			if err := validatePromotionTargetURL(trimmed); err != nil {
				return model.HomeSectionMedia{}, fmt.Errorf("%w: target URL", ErrInvalidHomeMedia)
			}
			targetURL = stringPtr(trimmed)
		} else {
			targetURL = nil
		}
	}
	return s.repository.UpdateHomeSectionMedia(ctx, id, model.HomeSectionMediaUpdate{SortOrder: sortOrder, IsActive: isActive, ImageAltText: altText, TargetURL: targetURL})
}

func (s *HomeSectionService) DeleteMedia(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return ErrInvalidHomeMedia
	}
	// PostgreSQL deletes metadata and enqueues its public ID atomically. The
	// provider asset is never deleted first, so a Cloudinary outage cannot make
	// a still-visible row point at a missing image.
	if _, err := s.repository.DeleteHomeSectionMedia(ctx, id); err != nil {
		return err
	}
	s.signalCleanup()
	return nil
}

func (s *HomeSectionService) cleanupLoop() {
	defer s.cleanupWG.Done()
	ticker := time.NewTicker(homeCleanupPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.cleanupCtx.Done():
			return
		case <-s.cleanupWake:
		case <-ticker.C:
		}
		_ = s.processCleanupBatch(s.cleanupCtx)
	}
}

func (s *HomeSectionService) signalCleanup() {
	if s == nil || s.cleanupWake == nil {
		return
	}
	select {
	case s.cleanupWake <- struct{}{}:
	default:
	}
}

// enqueueCleanup deliberately ignores the request context. A browser timeout
// or disconnect must not prevent recording an uploaded-but-unowned asset.
func (s *HomeSectionService) enqueueCleanup(publicID string) error {
	if s == nil || s.repository == nil || strings.TrimSpace(publicID) == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), homeCleanupOperationTimeout)
	defer cancel()
	if err := s.repository.EnqueueHomeAssetCleanup(ctx, publicID); err != nil {
		return err
	}
	s.signalCleanup()
	return nil
}

// processCleanupBatch claims leased jobs and acknowledges only provider
// deletions that succeeded. Failed destroys remain durable and become due
// again after the repository lease expires.
func (s *HomeSectionService) processCleanupBatch(parent context.Context) error {
	if s == nil || s.repository == nil || s.assets == nil {
		return nil
	}
	if parent == nil {
		parent = context.Background()
	}
	claimCtx, cancelClaim := context.WithTimeout(parent, homeCleanupOperationTimeout)
	jobs, err := s.repository.ClaimHomeAssetCleanup(claimCtx, homeCleanupBatchSize, homeCleanupRetryAfter)
	cancelClaim()
	if err != nil {
		return err
	}

	var firstErr error
	for _, job := range jobs {
		destroyCtx, cancelDestroy := context.WithTimeout(parent, homeCleanupOperationTimeout)
		destroyErr := s.assets.Destroy(destroyCtx, job.PublicID)
		cancelDestroy()
		if destroyErr != nil {
			if firstErr == nil {
				firstErr = ErrHomeSectionStorage
			}
			continue
		}

		completeCtx, cancelComplete := context.WithTimeout(parent, homeCleanupOperationTimeout)
		completeErr := s.repository.CompleteHomeAssetCleanup(completeCtx, job.ID)
		cancelComplete()
		if completeErr != nil && firstErr == nil {
			firstErr = completeErr
		}
	}
	return firstErr
}

func homeSectionLayout(slot int16) (model.HomeSectionLayout, error) {
	switch slot {
	case 1, 3:
		return model.HomeSectionContentLeft, nil
	case 2:
		return model.HomeSectionImageLeft, nil
	default:
		return "", ErrInvalidHomeSection
	}
}

func validateHomeText(value string, maxLength int, required bool, field string) error {
	if required && value == "" {
		return fmt.Errorf("%w: %s is required", ErrInvalidHomeSection, field)
	}
	if utf8.RuneCountInString(value) > maxLength {
		return fmt.Errorf("%w: %s is too long", ErrInvalidHomeSection, field)
	}
	return nil
}

func readHomeSectionImage(header *multipart.FileHeader, maxBytes int64) ([]byte, string, string, error) {
	if header == nil || maxBytes <= 0 {
		return nil, "", "", ErrInvalidHomeSection
	}
	input, err := header.Open()
	if err != nil {
		return nil, "", "", ErrInvalidHomeSection
	}
	defer input.Close()
	data, err := io.ReadAll(io.LimitReader(input, maxBytes+1))
	if err != nil {
		return nil, "", "", ErrInvalidHomeSection
	}
	if int64(len(data)) > maxBytes {
		return nil, "", "", ErrPromotionImageTooLarge
	}
	mimeType, ok := promotionImageMIME(data)
	if !ok {
		return nil, "", "", ErrInvalidHomeSection
	}
	digest := sha256.Sum256(data)
	return data, mimeType, hex.EncodeToString(digest[:]), nil
}

func optionalHomeText(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func stringPtr(value string) *string { return &value }

func deriveHomeImageAltText(title, requested, current string) string {
	if trimmed := strings.TrimSpace(requested); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(title); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(current); trimmed != "" {
		return trimmed
	}
	return "Ảnh section Home"
}

func deriveHomeMediaAltText(sortOrder int16, requested string) string {
	if trimmed := strings.TrimSpace(requested); trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf("Ảnh dải Home %d", sortOrder+1)
}
