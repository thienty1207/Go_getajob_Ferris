package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"strings"
	"unicode/utf8"
)

var (
	// These errors are mapped to stable HTTP error codes by the handler. Their
	// internal text is deliberately not returned to clients.
	ErrInvalidPromotion       = errors.New("invalid promotion")
	ErrPromotionImageTooLarge = errors.New("promotion image too large")
)

const (
	maxPromotionAltText = 180
	maxPromotionEyebrow = 80
	maxPromotionTitle   = 160
	maxPromotionBody    = 320
)

// validatePromotionSlot keeps the product contract bounded to three known
// positions. It is repeated at the service boundary even if the database also
// has a CHECK constraint, because invalid requests should fail before SQL.
func validatePromotionSlot(slot int16) error {
	if slot < 1 || slot > 3 {
		return fmt.Errorf("%w: slot must be between 1 and 3", ErrInvalidPromotion)
	}
	return nil
}

// readPromotionImage reads at most maxBytes+1 bytes so an oversized upload is
// detected without unbounded memory allocation. MIME detection uses file
// signatures, not the user-controlled filename or Content-Type header.
func readPromotionImage(header *multipart.FileHeader, maxBytes int64) ([]byte, string, string, error) {
	if header == nil || maxBytes <= 0 {
		return nil, "", "", ErrInvalidPromotion
	}

	input, err := header.Open()
	if err != nil {
		return nil, "", "", ErrInvalidPromotion
	}
	defer input.Close()

	data, err := io.ReadAll(io.LimitReader(input, maxBytes+1))
	if err != nil {
		return nil, "", "", ErrInvalidPromotion
	}
	if int64(len(data)) > maxBytes {
		return nil, "", "", ErrPromotionImageTooLarge
	}

	mimeType, ok := promotionImageMIME(data)
	if !ok {
		return nil, "", "", ErrInvalidPromotion
	}
	// The hash is used for replacement detection and HTTP cache validation; it
	// is not a security claim about who uploaded the content.
	digest := sha256.Sum256(data)
	return data, mimeType, hex.EncodeToString(digest[:]), nil
}

// promotionImageMIME accepts only the three formats the product is prepared
// to serve. In particular, SVG is excluded because it can carry active markup
// when rendered by a browser.
func promotionImageMIME(data []byte) (string, bool) {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png", true
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg", true
	}
	if len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp", true
	}
	return "", false
}

// validatePromotionAltText requires accessible text for every public image.
func validatePromotionAltText(value string) error {
	return validatePromotionText(value, maxPromotionAltText, true, "alt")
}

// validatePromotionText counts Unicode characters rather than bytes so a
// Vietnamese caption is not rejected earlier than an ASCII caption of the
// same visible length.
func validatePromotionText(value string, maxLength int, required bool, field string) error {
	trimmed := strings.TrimSpace(value)
	if required && trimmed == "" {
		return fmt.Errorf("%w: %s is required", ErrInvalidPromotion, field)
	}
	if utf8.RuneCountInString(trimmed) > maxLength {
		return fmt.Errorf("%w: %s is too long", ErrInvalidPromotion, field)
	}
	return nil
}

// validatePromotionCopy bounds optional presentation fields before they reach
// the database and the client DOM.
func validatePromotionCopy(eyebrow, title, body string) error {
	if err := validatePromotionText(eyebrow, maxPromotionEyebrow, false, "eyebrow"); err != nil {
		return err
	}
	if err := validatePromotionText(title, maxPromotionTitle, false, "title"); err != nil {
		return err
	}
	return validatePromotionText(body, maxPromotionBody, false, "body")
}

// validatePromotionTargetURL permits only explicit HTTP(S) destinations and
// rejects embedded credentials, which avoids turning admin content into an
// unsafe redirect or credential-bearing link.
func validatePromotionTargetURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("%w: target URL is invalid", ErrInvalidPromotion)
	}
	return nil
}
