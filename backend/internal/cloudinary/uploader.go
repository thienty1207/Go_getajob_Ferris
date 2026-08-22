package cloudinary

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	cld "github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/gogetsomefoodferris/backend/internal/service"
	"github.com/google/uuid"
)

// UploadClient is the small Cloudinary surface the application needs. Keeping
// it narrow makes promotion tests deterministic without making network calls.
type UploadClient interface {
	Upload(context.Context, interface{}, uploader.UploadParams) (*uploader.UploadResult, error)
	Destroy(context.Context, uploader.DestroyParams) (*uploader.DestroyResult, error)
}

// Store adapts the official Cloudinary Go SDK to the promotion service. The
// API secret lives inside the SDK configuration and never crosses this package
// boundary into an HTTP response.
type Store struct {
	client UploadClient
}

// NewStore reads the server-only CLOUDINARY_URL through the official SDK
// parser. An empty value fails early so a production API cannot silently fall
// back to database image bytes.
func NewStore(rawURL string) (*Store, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, errors.New("CLOUDINARY_URL is required for promotion uploads")
	}
	instance, err := cld.NewFromURL(rawURL)
	if err != nil {
		// net/url includes the original credential-bearing value in parse errors.
		return nil, errors.New("invalid Cloudinary configuration")
	}
	return &Store{client: &instance.Upload}, nil
}

// NewStoreWithClient is used by unit tests and keeps the production adapter's
// validation logic identical to the real SDK path.
func NewStoreWithClient(client UploadClient) *Store {
	return &Store{client: client}
}

// Upload stores a content-addressed image so replacing a slot never overwrites
// a URL that a browser may still have cached. The database later decides which
// version is public.
func (s *Store) Upload(ctx context.Context, data []byte, _ string, slot int16, contentHash string) (service.PromotionAsset, error) {
	if s == nil || s.client == nil || len(data) == 0 || slot < 1 || slot > 3 || !isSHA256(contentHash) {
		return service.PromotionAsset{}, errors.New("invalid Cloudinary upload input")
	}
	return s.uploadImage(ctx, data, "ferris/promotions", fmt.Sprintf("slot-%d-%s", slot, contentHash))
}

func (s *Store) UploadHomeSection(ctx context.Context, data []byte, _ string, slot int16, contentHash string) (service.PromotionAsset, error) {
	if s == nil || s.client == nil || len(data) == 0 || slot < 1 || slot > 4 || !isSHA256(contentHash) {
		return service.PromotionAsset{}, errors.New("invalid Cloudinary home section upload input")
	}
	// The content hash remains useful for diagnostics, but it cannot be the
	// ownership key: two media rows may intentionally upload identical bytes.
	// A per-upload suffix ensures deleting one row can never delete another
	// row's provider asset.
	return s.uploadImage(ctx, data, "ferris/home-sections", fmt.Sprintf("slot-%d-%s-%s", slot, contentHash, uuid.NewString()))
}

func (s *Store) uploadImage(ctx context.Context, data []byte, folder, publicID string) (service.PromotionAsset, error) {
	overwrite := false
	uniqueFilename := false
	discardOriginalFilename := true
	result, err := s.client.Upload(ctx, bytes.NewReader(data), uploader.UploadParams{
		Folder:                  folder,
		PublicID:                publicID,
		ResourceType:            "image",
		Overwrite:               &overwrite,
		UniqueFilename:          &uniqueFilename,
		DiscardOriginalFilename: &discardOriginalFilename,
	})
	if err != nil {
		// SDK errors may include the credential-bearing CLOUDINARY_URL. Keep the
		// provider detail inside this boundary and return only a stable category.
		return service.PromotionAsset{}, errors.New("Cloudinary upload failed")
	}
	// cloudinary-go currently returns a nil Go error for some rejected HTTP
	// responses and puts the provider failure in UploadResult.Error instead.
	// Check that field before metadata validation so operators see the real
	// configuration failure rather than a misleading "incomplete metadata".
	if result != nil && strings.TrimSpace(result.Error.Message) != "" {
		return service.PromotionAsset{}, errors.New("Cloudinary upload failed")
	}
	if result == nil || strings.TrimSpace(result.PublicID) == "" || strings.TrimSpace(result.AssetID) == "" || !isSecureURL(result.SecureURL) {
		return service.PromotionAsset{}, errors.New("Cloudinary returned incomplete promotion metadata")
	}
	return service.PromotionAsset{
		PublicID:  result.PublicID,
		AssetID:   result.AssetID,
		SecureURL: result.SecureURL,
	}, nil
}

// Destroy removes a superseded image after the database has committed the new
// slot. Invalidation asks Cloudinary to clear cached delivery copies.
func (s *Store) Destroy(ctx context.Context, publicID string) error {
	if s == nil || s.client == nil || strings.TrimSpace(publicID) == "" {
		return nil
	}
	invalidate := true
	result, err := s.client.Destroy(ctx, uploader.DestroyParams{
		PublicID:     publicID,
		ResourceType: "image",
		Type:         "upload",
		Invalidate:   &invalidate,
	})
	if err != nil {
		return errors.New("Cloudinary destroy failed")
	}
	// Destroy has the same SDK response behavior as Upload: provider errors may
	// arrive inside the result with no Go error from the HTTP call.
	if result != nil && strings.TrimSpace(result.Error.Message) != "" {
		return errors.New("Cloudinary destroy failed")
	}
	if result == nil || (result.Result != "ok" && result.Result != "not found") {
		return errors.New("Cloudinary destroy failed")
	}
	return nil
}

func isSecureURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}
