package cloudinary

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// TestCloudinaryLiveHomeSectionRoundTrip verifies the same provider boundary
// used by the admin Home editor. It is opt-in because it makes a real network
// request, consumes Cloudinary quota, and therefore must never run silently in
// the ordinary unit-test suite.
func TestCloudinaryLiveHomeSectionRoundTrip(t *testing.T) {
	if os.Getenv("FERRIS_CLOUDINARY_INTEGRATION") != "1" {
		t.Skip("set FERRIS_CLOUDINARY_INTEGRATION=1 to run the live Cloudinary round trip")
	}

	rawURL := os.Getenv("CLOUDINARY_URL")
	if rawURL == "" {
		t.Fatal("CLOUDINARY_URL is required for the live Cloudinary test")
	}
	store, err := NewStore(rawURL)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Start with a valid one-pixel PNG and append random bytes after the PNG
	// trailer. Cloudinary accepts the image while every test run receives a new
	// content hash, so a stale asset from an interrupted run cannot mask the
	// provider behavior we are trying to verify.
	image, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode PNG fixture: %v", err)
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("generate upload nonce: %v", err)
	}
	image = append(image, nonce...)
	digest := sha256.Sum256(image)
	contentHash := hex.EncodeToString(digest[:])
	expectedPublicIDPrefix := "ferris/home-sections/slot-4-" + contentHash + "-"

	uploadCtx, cancelUpload := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelUpload()
	capture := &capturingUploadClient{delegate: store.client}
	store.client = capture
	asset, err := store.UploadHomeSection(uploadCtx, image, "image/png", 4, contentHash)
	cleanupPublicID := asset.PublicID
	if cleanupPublicID == "" && capture.result != nil {
		cleanupPublicID = capture.result.PublicID
	}
	if cleanupPublicID != "" {
		t.Cleanup(func() {
			// Start the timeout when cleanup actually runs; a slow upload must not
			// consume the provider-deletion budget before the callback starts.
			cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancelCleanup()
			if err := store.Destroy(cleanupCtx, cleanupPublicID); err != nil {
				t.Errorf("cleanup Cloudinary asset failed")
			}
		})
	}
	if err != nil {
		if capture.result != nil {
			t.Logf("Cloudinary metadata presence: public_id=%t asset_id=%t secure_url=%t url=%t bytes=%d",
				capture.result.PublicID != "", capture.result.AssetID != "", capture.result.SecureURL != "",
				capture.result.URL != "", capture.result.Bytes)
			t.Logf("Cloudinary response: error=%q raw_type=%T",
				redactCloudinaryCredentials(capture.result.Error.Message, rawURL), capture.result.Response)
		}
		t.Fatalf("UploadHomeSection() live provider request failed")
	}
	if !strings.HasPrefix(asset.PublicID, expectedPublicIDPrefix) || asset.AssetID == "" || asset.SecureURL == "" {
		t.Fatalf("UploadHomeSection() metadata presence: expected_public_id_prefix=%t asset_id=%t secure_url=%t",
			strings.HasPrefix(asset.PublicID, expectedPublicIDPrefix), asset.AssetID != "", asset.SecureURL != "")
	}
}

// redactCloudinaryCredentials keeps live-provider diagnostics useful without
// ever copying the API key, API secret, or full CLOUDINARY_URL into test logs.
func redactCloudinaryCredentials(message, rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.User == nil {
		return message
	}

	redacted := strings.ReplaceAll(message, parsed.User.Username(), "[redacted-api-key]")
	if secret, ok := parsed.User.Password(); ok {
		redacted = strings.ReplaceAll(redacted, secret, "[redacted-api-secret]")
	}
	return strings.ReplaceAll(redacted, rawURL, "[redacted-cloudinary-url]")
}

type capturingUploadClient struct {
	delegate UploadClient
	result   *uploader.UploadResult
}

func (c *capturingUploadClient) Upload(ctx context.Context, input interface{}, params uploader.UploadParams) (*uploader.UploadResult, error) {
	result, err := c.delegate.Upload(ctx, input, params)
	c.result = result
	return result, err
}

func (c *capturingUploadClient) Destroy(ctx context.Context, params uploader.DestroyParams) (*uploader.DestroyResult, error) {
	return c.delegate.Destroy(ctx, params)
}
