package repository

import (
	"context"
	"errors"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

// HomeAssetCleanup is a leased durable cleanup job. The public ID is provider
// metadata, never a credential-bearing Cloudinary configuration URL.
type HomeAssetCleanup struct {
	ID           uuid.UUID
	PublicID     string
	AttemptCount int32
}

var (
	ErrHomeSectionNotFound      = errors.New("home section not found")
	ErrHomeSectionMediaNotFound = errors.New("home section media not found")
	ErrHomeSectionMediaLimit    = errors.New("home section media limit reached")
)

type HomeSectionRepository interface {
	ListHomeSections(context.Context, bool) ([]model.HomeSection, error)
	GetHomeSection(context.Context, int16) (model.HomeSection, error)
	UpsertHomeSection(context.Context, model.HomeSectionWrite) (model.HomeSection, error)
	CreateHomeSectionMedia(context.Context, model.HomeSectionMediaWrite) (model.HomeSectionMedia, error)
	GetHomeSectionMedia(context.Context, uuid.UUID) (model.HomeSectionMedia, error)
	UpdateHomeSectionMedia(context.Context, uuid.UUID, model.HomeSectionMediaUpdate) (model.HomeSectionMedia, error)
	DeleteHomeSectionMedia(context.Context, uuid.UUID) (model.HomeSectionMedia, error)
	EnqueueHomeAssetCleanup(context.Context, string) error
	ClaimHomeAssetCleanup(context.Context, int, time.Duration) ([]HomeAssetCleanup, error)
	CompleteHomeAssetCleanup(context.Context, uuid.UUID) error
}
