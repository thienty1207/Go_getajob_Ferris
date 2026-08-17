package processor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// ErrUnavailable identifies the deliberate safe state where no real parser or
// matcher has been wired yet.
var ErrUnavailable = errors.New("scan processor unavailable")

// ScanProcessor is the future parser/DeepSeek/geocoder/matcher seam. It gets a
// temporary file path and must not assume raw CV bytes are retained by the API.
type ScanProcessor interface {
	Process(context.Context, uuid.UUID, string, float64) error
}

// ProcessingError carries a bounded machine-readable failure code while
// retaining the wrapped cause for internal diagnostics.
type ProcessingError struct {
	Code string
	Err  error
}

func (e *ProcessingError) Error() string {
	if e == nil {
		return "scan processing failed"
	}
	if e.Err == nil {
		return e.Code
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Err)
}

func (e *ProcessingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ErrorCode sanitizes processor-provided codes before they enter the database
// error_code column or an internal log field.
func ErrorCode(err error) string {
	var processingErr *ProcessingError
	if !errors.As(err, &processingErr) {
		return "processing_failed"
	}
	code := strings.ToLower(strings.TrimSpace(processingErr.Code))
	if code == "" || len(code) > 64 {
		return "processing_failed"
	}
	for _, character := range code {
		if !(unicode.IsLower(character) || unicode.IsDigit(character) || character == '_' || character == '-') {
			return "processing_failed"
		}
	}
	return code
}
