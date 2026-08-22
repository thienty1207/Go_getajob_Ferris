package cloudinary

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudinary/cloudinary-go/v2/api"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

func TestStoreUploadUsesContentAddressedImageMetadata(t *testing.T) {
	t.Parallel()
	fake := &recordingUploadClient{uploadResult: &uploader.UploadResult{
		PublicID:  "ferris/promotions/slot-1-hash",
		AssetID:   "asset-1",
		SecureURL: "https://res.cloudinary.com/demo/image/upload/v1/ferris/promotions/slot-1-hash.png",
	}}
	store := NewStoreWithClient(fake)
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	asset, err := store.Upload(context.Background(), []byte("png"), "image/png", 1, hash)
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if asset.PublicID != fake.uploadResult.PublicID || asset.AssetID != fake.uploadResult.AssetID || asset.SecureURL != fake.uploadResult.SecureURL {
		t.Fatalf("asset = %#v, want Cloudinary metadata", asset)
	}
	if fake.uploadParams.PublicID != "slot-1-"+hash || fake.uploadParams.Folder != "ferris/promotions" || fake.uploadParams.ResourceType != "image" {
		t.Fatalf("upload params = %#v, want content-addressed image upload", fake.uploadParams)
	}
}

func TestStoreUploadHomeSectionUsesDedicatedFolder(t *testing.T) {
	t.Parallel()
	fake := &recordingUploadClient{uploadResult: &uploader.UploadResult{
		PublicID:  "ferris/home-sections/slot-4-hash",
		AssetID:   "asset-home-1",
		SecureURL: "https://res.cloudinary.com/demo/image/upload/v1/ferris/home-sections/slot-4-hash.png",
	}}
	store := NewStoreWithClient(fake)
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	asset, err := store.UploadHomeSection(context.Background(), []byte("png"), "image/png", 4, hash)
	if err != nil {
		t.Fatalf("UploadHomeSection() error = %v", err)
	}
	if asset.PublicID != fake.uploadResult.PublicID || fake.uploadParams.Folder != "ferris/home-sections" || !strings.HasPrefix(fake.uploadParams.PublicID, "slot-4-"+hash+"-") {
		t.Fatalf("asset=%#v uploadParams=%#v, want dedicated home section folder", asset, fake.uploadParams)
	}
}

func TestStoreHomeSectionUploadsUseDistinctPublicIDsForIdenticalContent(t *testing.T) {
	t.Parallel()
	fake := &recordingUploadClient{
		uploadResultFor: func(params uploader.UploadParams) *uploader.UploadResult {
			return &uploader.UploadResult{
				PublicID:  params.Folder + "/" + params.PublicID,
				AssetID:   "asset-" + params.PublicID,
				SecureURL: "https://res.cloudinary.com/demo/image/upload/" + params.PublicID + ".png",
			}
		},
	}
	store := NewStoreWithClient(fake)
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	first, err := store.UploadHomeSection(context.Background(), []byte("identical"), "image/png", 4, hash)
	if err != nil {
		t.Fatalf("first UploadHomeSection() error = %v", err)
	}
	second, err := store.UploadHomeSection(context.Background(), []byte("identical"), "image/png", 4, hash)
	if err != nil {
		t.Fatalf("second UploadHomeSection() error = %v", err)
	}

	if first.PublicID == second.PublicID {
		t.Fatalf("identical uploads share public ID %q; each database owner needs its own provider asset", first.PublicID)
	}
	if len(fake.uploadParamsHistory) != 2 || fake.uploadParamsHistory[0].PublicID == fake.uploadParamsHistory[1].PublicID {
		t.Fatalf("upload params = %#v, want two distinct public IDs", fake.uploadParamsHistory)
	}
	for _, params := range fake.uploadParamsHistory {
		if !strings.HasPrefix(params.PublicID, "slot-4-"+hash+"-") {
			t.Fatalf("public ID %q does not retain slot/hash diagnostics", params.PublicID)
		}
	}
}

func TestStoreRejectsUntrustedCloudinaryResponse(t *testing.T) {
	t.Parallel()
	fake := &recordingUploadClient{uploadResult: &uploader.UploadResult{
		PublicID:  "asset",
		AssetID:   "asset-1",
		SecureURL: "http://evil.example/image.png",
	}}
	store := NewStoreWithClient(fake)
	_, err := store.Upload(context.Background(), []byte("png"), "image/png", 1, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err == nil {
		t.Fatal("Upload() error = nil, want rejected insecure provider URL")
	}
}

func TestStoreReportsCloudinaryUploadRejectionBeforeMetadataValidation(t *testing.T) {
	t.Parallel()
	const sensitive = "provider-secret"
	fake := &recordingUploadClient{uploadResult: &uploader.UploadResult{
		Error: api.ErrorResp{Message: "Invalid Signature: " + sensitive},
	}}
	store := NewStoreWithClient(fake)
	_, err := store.UploadHomeSection(context.Background(), []byte("png"), "image/png", 1, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err == nil {
		t.Fatal("UploadHomeSection() error = nil, want provider rejection")
	}
	if !strings.Contains(err.Error(), "Cloudinary upload failed") || strings.Contains(err.Error(), sensitive) {
		t.Fatalf("UploadHomeSection() error = %q, want sanitized provider rejection classification", err)
	}
}

func TestStoreUploadErrorDoesNotExposeProviderDetails(t *testing.T) {
	t.Parallel()
	const sensitive = "cloudinary://api-key:provider-secret@example-cloud"
	fake := &recordingUploadClient{uploadErr: errors.New("request failed for " + sensitive)}
	store := NewStoreWithClient(fake)

	_, err := store.UploadHomeSection(context.Background(), []byte("png"), "image/png", 1, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err == nil {
		t.Fatal("UploadHomeSection() error = nil, want provider failure")
	}
	if strings.Contains(err.Error(), sensitive) || strings.Contains(err.Error(), "provider-secret") {
		t.Fatalf("UploadHomeSection() exposed provider details: %q", err)
	}
}

func TestNewStoreErrorDoesNotExposeCredentialURL(t *testing.T) {
	t.Parallel()
	const rawURL = "cloudinary://api-key:provider-secret@%zz"

	_, err := NewStore(rawURL)
	if err == nil {
		t.Fatal("NewStore() error = nil, want invalid configuration")
	}
	if strings.Contains(err.Error(), rawURL) || strings.Contains(err.Error(), "provider-secret") {
		t.Fatalf("NewStore() exposed credential URL: %q", err)
	}
}

func TestStoreDestroyAcceptsNotFoundForIdempotentCleanup(t *testing.T) {
	t.Parallel()
	fake := &recordingUploadClient{destroyResult: &uploader.DestroyResult{Result: "not found"}}
	store := NewStoreWithClient(fake)
	if err := store.Destroy(context.Background(), "ferris/promotions/old"); err != nil {
		t.Fatalf("Destroy() error = %v, want idempotent not-found cleanup", err)
	}
	if fake.destroyParams.PublicID != "ferris/promotions/old" || fake.destroyParams.ResourceType != "image" {
		t.Fatalf("destroy params = %#v", fake.destroyParams)
	}
}

func TestStoreReportsCloudinaryDestroyRejection(t *testing.T) {
	t.Parallel()
	const sensitive = "provider-secret"
	fake := &recordingUploadClient{destroyResult: &uploader.DestroyResult{
		Error: api.ErrorResp{Message: "Invalid Signature: " + sensitive},
	}}
	store := NewStoreWithClient(fake)
	if err := store.Destroy(context.Background(), "ferris/home-sections/old"); err == nil || !strings.Contains(err.Error(), "Cloudinary destroy failed") || strings.Contains(err.Error(), sensitive) {
		t.Fatalf("Destroy() error = %v, want sanitized provider rejection classification", err)
	}
}

func TestStoreDestroyErrorDoesNotExposeProviderDetails(t *testing.T) {
	t.Parallel()
	const sensitive = "cloudinary://api-key:provider-secret@example-cloud"
	fake := &recordingUploadClient{destroyErr: errors.New("request failed for " + sensitive)}
	store := NewStoreWithClient(fake)

	err := store.Destroy(context.Background(), "ferris/home-sections/old")
	if err == nil {
		t.Fatal("Destroy() error = nil, want provider failure")
	}
	if strings.Contains(err.Error(), sensitive) || strings.Contains(err.Error(), "provider-secret") {
		t.Fatalf("Destroy() exposed provider details: %q", err)
	}
}

type recordingUploadClient struct {
	uploadResult        *uploader.UploadResult
	uploadResultFor     func(uploader.UploadParams) *uploader.UploadResult
	uploadErr           error
	destroyResult       *uploader.DestroyResult
	destroyErr          error
	uploadParams        uploader.UploadParams
	uploadParamsHistory []uploader.UploadParams
	destroyParams       uploader.DestroyParams
}

func (f *recordingUploadClient) Upload(_ context.Context, _ interface{}, params uploader.UploadParams) (*uploader.UploadResult, error) {
	f.uploadParams = params
	f.uploadParamsHistory = append(f.uploadParamsHistory, params)
	if f.uploadResultFor != nil {
		return f.uploadResultFor(params), f.uploadErr
	}
	return f.uploadResult, f.uploadErr
}

func (f *recordingUploadClient) Destroy(_ context.Context, params uploader.DestroyParams) (*uploader.DestroyResult, error) {
	f.destroyParams = params
	return f.destroyResult, f.destroyErr
}
