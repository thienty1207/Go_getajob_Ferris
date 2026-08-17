package service

import (
	"context"
	"errors"
	"testing"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
)

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

func TestPromotionServiceUpsertNormalizesMetadataAndStoresValidatedImage(t *testing.T) {
	t.Parallel()
	repo := &recordingPromotionRepository{}
	service := NewPromotionService(repo, config.Config{MaxPromotionImageBytes: 1024})
	header := promotionFileHeader(t, "campaign.png", pngSignature)

	promotion, err := service.Upsert(context.Background(), PromotionInput{
		Slot:      2,
		File:      header,
		AltText:   "  Tìm việc phù hợp  ",
		Eyebrow:   "  START HERE ",
		Title:     "  Đăng CV hôm nay ",
		Body:      "  Mô tả ngắn ",
		TargetURL: " https://example.com/careers ",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if repo.lastWrite.Slot != 2 || string(repo.lastWrite.ImageBytes) != string(pngSignature) || repo.lastWrite.MIMEType != "image/png" {
		t.Fatalf("write = %#v, want validated slot/image/mime", repo.lastWrite)
	}
	if repo.lastWrite.AltText != "Tìm việc phù hợp" || valueOf(repo.lastWrite.Eyebrow) != "START HERE" || valueOf(repo.lastWrite.Title) != "Đăng CV hôm nay" || valueOf(repo.lastWrite.Body) != "Mô tả ngắn" || valueOf(repo.lastWrite.TargetURL) != "https://example.com/careers" {
		t.Fatalf("normalized write = %#v", repo.lastWrite)
	}
	if promotion.Slot != 2 {
		t.Fatalf("promotion = %#v, want repository result", promotion)
	}
}

func TestPromotionServiceUpsertGeneratesAltTextWhenAdminOnlySendsImage(t *testing.T) {
	t.Parallel()
	repo := &recordingPromotionRepository{}
	service := NewPromotionService(repo, config.Config{MaxPromotionImageBytes: 1024})

	_, err := service.Upsert(context.Background(), PromotionInput{
		Slot: 1,
		File: promotionFileHeader(t, "campaign.png", pngSignature),
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if repo.lastWrite.AltText != "Ảnh quảng bá Ferris - Slot 01" {
		t.Fatalf("generated alt text = %q, want slot-specific default", repo.lastWrite.AltText)
	}
}

func TestPromotionServiceUpsertRejectsInvalidInputBeforeRepository(t *testing.T) {
	t.Parallel()
	repo := &recordingPromotionRepository{}
	service := NewPromotionService(repo, config.Config{MaxPromotionImageBytes: 1024})

	_, err := service.Upsert(context.Background(), PromotionInput{
		Slot:    4,
		File:    promotionFileHeader(t, "campaign.png", pngSignature),
		AltText: "Campaign",
	})
	if !errors.Is(err, ErrInvalidPromotion) {
		t.Fatalf("Upsert() error = %v, want ErrInvalidPromotion", err)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("upsert calls = %d, want 0", repo.upsertCalls)
	}
}

func TestPromotionServiceGetImageValidatesSlotAndDelegates(t *testing.T) {
	t.Parallel()
	repo := &recordingPromotionRepository{image: repository.PromotionImage{Slot: 1, MIMEType: "image/png"}}
	service := NewPromotionService(repo, config.Config{MaxPromotionImageBytes: 1024})

	image, err := service.GetImage(context.Background(), 1)
	if err != nil || image.MIMEType != "image/png" || repo.imageSlot != 1 {
		t.Fatalf("GetImage() = %#v, err=%v, slot=%d", image, err, repo.imageSlot)
	}
	if _, err := service.GetImage(context.Background(), 0); !errors.Is(err, ErrInvalidPromotion) {
		t.Fatalf("invalid GetImage() error = %v, want ErrInvalidPromotion", err)
	}
}

func TestPromotionServiceDeleteValidatesSlotAndDelegates(t *testing.T) {
	t.Parallel()
	repo := &recordingPromotionRepository{}
	service := NewPromotionService(repo, config.Config{MaxPromotionImageBytes: 1024})

	if err := service.Delete(context.Background(), 3); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if repo.deletedSlot != 3 {
		t.Fatalf("deleted slot = %d, want 3", repo.deletedSlot)
	}
	if err := service.Delete(context.Background(), 4); !errors.Is(err, ErrInvalidPromotion) {
		t.Fatalf("invalid Delete() error = %v, want ErrInvalidPromotion", err)
	}
}

type recordingPromotionRepository struct {
	lastWrite   repository.PromotionWrite
	upsertCalls int
	image       repository.PromotionImage
	imageSlot   int16
	deletedSlot int16
}

func (r *recordingPromotionRepository) ListActive(context.Context) ([]model.Promotion, error) {
	return []model.Promotion{{Slot: 1}}, nil
}

func (r *recordingPromotionRepository) GetActiveImage(_ context.Context, slot int16) (repository.PromotionImage, error) {
	r.imageSlot = slot
	return r.image, nil
}

func (r *recordingPromotionRepository) Upsert(_ context.Context, write repository.PromotionWrite) (model.Promotion, error) {
	r.upsertCalls++
	r.lastWrite = write
	return model.Promotion{Slot: write.Slot}, nil
}

func (r *recordingPromotionRepository) Delete(_ context.Context, slot int16) error {
	r.deletedSlot = slot
	return nil
}

func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
