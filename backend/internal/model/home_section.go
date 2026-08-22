package model

import (
	"time"

	"github.com/google/uuid"
)

type HomeSectionLayout string

const (
	HomeSectionContentLeft HomeSectionLayout = "CONTENT_LEFT"
	HomeSectionImageLeft   HomeSectionLayout = "IMAGE_LEFT"
	HomeSectionMediaStrip  HomeSectionLayout = "MEDIA_STRIP"
)

type HomeSection struct {
	ID                 uuid.UUID
	Slot               int16
	Layout             HomeSectionLayout
	IsActive           bool
	Eyebrow            string
	Title              string
	Body               string
	ImageAltText       string
	ImageURL           string
	ImageContentHash   string
	StorageProvider    string
	CloudinaryPublicID string
	CloudinaryAssetID  string
	TargetURL          string
	UpdatedBy          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Media              []HomeSectionMedia
}

type HomeSectionMedia struct {
	ID                 uuid.UUID
	SortOrder          int16
	IsActive           bool
	ImageAltText       string
	ImageURL           string
	ImageContentHash   string
	StorageProvider    string
	CloudinaryPublicID string
	CloudinaryAssetID  string
	TargetURL          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type HomeSectionWrite struct {
	Slot                int16
	Layout              HomeSectionLayout
	IsActive            bool
	Eyebrow             *string
	Title               *string
	Body                *string
	ImageAltText        *string
	ImageContentHash    *string
	StorageProvider     *string
	CloudinaryPublicID  *string
	CloudinarySecureURL *string
	CloudinaryAssetID   *string
	TargetURL           *string
	UpdatedBy           *string
}

type HomeSectionMediaWrite struct {
	HomeSectionID       uuid.UUID
	SortOrder           int16
	IsActive            bool
	ImageAltText        string
	ImageContentHash    string
	StorageProvider     string
	CloudinaryPublicID  string
	CloudinarySecureURL string
	CloudinaryAssetID   string
	TargetURL           *string
}

type HomeSectionMediaUpdate struct {
	SortOrder    *int16
	IsActive     *bool
	ImageAltText *string
	TargetURL    *string
}
