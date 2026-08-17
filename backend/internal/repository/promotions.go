package repository

import (
	"context"
	"errors"

	"github.com/gogetsomefoodferris/backend/internal/model"
)

var ErrPromotionNotFound = errors.New("promotion not found")

// PromotionWrite is the already validated persistence payload for one slot.
// The service layer owns validation; the repository only maps this shape to
// parameterized SQL.
type PromotionWrite struct {
	Slot                int16
	ImageBytes          []byte
	MIMEType            string
	ContentHash         string
	AltText             string
	Eyebrow             *string
	Title               *string
	Body                *string
	TargetURL           *string
	StorageProvider     string
	CloudinaryPublicID  string
	CloudinarySecureURL string
	CloudinaryAssetID   string
}

// PromotionImage is returned only by the dedicated binary image read path.
// It must not be embedded in the public metadata response.
type PromotionImage struct {
	Slot                int16
	ImageBytes          []byte
	MIMEType            string
	ContentHash         string
	StorageProvider     string
	CloudinaryPublicID  string
	CloudinarySecureURL string
	CloudinaryAssetID   string
}

// PromotionRepository is the service boundary for active promotion metadata,
// image bytes, idempotent slot replacement, and deletion.
type PromotionRepository interface {
	ListActive(context.Context) ([]model.Promotion, error)
	GetActiveImage(context.Context, int16) (PromotionImage, error)
	Upsert(context.Context, PromotionWrite) (model.Promotion, error)
	Delete(context.Context, int16) error
}
