package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPromotionImageValidationAcceptsSupportedSignatures(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  []byte
		mimeType string
	}{
		{name: "png", filename: "hero.png", content: append([]byte("\x89PNG\r\n\x1a\n"), []byte("data")...), mimeType: "image/png"},
		{name: "jpeg", filename: "hero.jpg", content: append([]byte{0xff, 0xd8, 0xff, 0xe0}, []byte("data")...), mimeType: "image/jpeg"},
		{name: "webp", filename: "hero.webp", content: append([]byte("RIFF\x04\x00\x00\x00WEBP"), []byte("data")...), mimeType: "image/webp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := promotionFileHeader(t, tt.filename, tt.content)
			got, mimeType, hash, err := readPromotionImage(file, 1024)
			if err != nil {
				t.Fatalf("readPromotionImage() error = %v", err)
			}
			if !bytes.Equal(got, tt.content) || mimeType != tt.mimeType {
				t.Fatalf("readPromotionImage() = (%q, %q), want original bytes and %q", got, mimeType, tt.mimeType)
			}
			digest := sha256.Sum256(tt.content)
			if hash != hex.EncodeToString(digest[:]) {
				t.Fatalf("hash = %q, want lowercase SHA-256", hash)
			}
		})
	}
}

func TestPromotionImageValidationRejectsSVGAndUnknownBytes(t *testing.T) {
	for _, filename := range []string{"hero.svg", "hero.bin"} {
		t.Run(filename, func(t *testing.T) {
			_, _, _, err := readPromotionImage(promotionFileHeader(t, filename, []byte("<svg>")), 1024)
			if err == nil {
				t.Fatalf("readPromotionImage(%q) error = nil, want invalid image", filename)
			}
		})
	}
}

func TestPromotionImageValidationRejectsOverflow(t *testing.T) {
	_, _, _, err := readPromotionImage(promotionFileHeader(t, "hero.png", append([]byte("\x89PNG\r\n\x1a\n"), []byte("0123456789")...)), 8)
	if err != ErrPromotionImageTooLarge {
		t.Fatalf("readPromotionImage() error = %v, want ErrPromotionImageTooLarge", err)
	}
}

func TestPromotionMetadataValidation(t *testing.T) {
	if err := validatePromotionSlot(0); err == nil {
		t.Fatal("validatePromotionSlot(0) error = nil")
	}
	if err := validatePromotionSlot(3); err != nil {
		t.Fatalf("validatePromotionSlot(3) error = %v", err)
	}
	if err := validatePromotionTargetURL("javascript:alert(1)"); err == nil {
		t.Fatal("validatePromotionTargetURL() accepted javascript URL")
	}
	if err := validatePromotionTargetURL("https://user:pass@example.com/campaign"); err == nil {
		t.Fatal("validatePromotionTargetURL() accepted URL credentials")
	}
	if err := validatePromotionTargetURL("https://example.com/campaign"); err != nil {
		t.Fatalf("validatePromotionTargetURL() error = %v", err)
	}
	if err := validatePromotionAltText(strings.Repeat("a", 181)); err == nil {
		t.Fatal("validatePromotionAltText() accepted text longer than 180 characters")
	}
}

func promotionFileHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("PUT", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(1024 * 1024); err != nil {
		t.Fatal(err)
	}
	return request.MultipartForm.File["image"][0]
}
