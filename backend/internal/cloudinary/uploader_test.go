package cloudinary

import (
	"context"
	"testing"

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

type recordingUploadClient struct {
	uploadResult  *uploader.UploadResult
	destroyResult *uploader.DestroyResult
	uploadParams  uploader.UploadParams
	destroyParams uploader.DestroyParams
}

func (f *recordingUploadClient) Upload(_ context.Context, _ interface{}, params uploader.UploadParams) (*uploader.UploadResult, error) {
	f.uploadParams = params
	return f.uploadResult, nil
}

func (f *recordingUploadClient) Destroy(_ context.Context, params uploader.DestroyParams) (*uploader.DestroyResult, error) {
	f.destroyParams = params
	return f.destroyResult, nil
}
