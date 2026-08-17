package processor

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestErrorCodeSanitizesTypedProcessingErrors(t *testing.T) {
	if got := ErrorCode(&ProcessingError{Code: " Parser_FAILED "}); got != "parser_failed" {
		t.Fatalf("ErrorCode() = %q, want parser_failed", got)
	}
	if got := ErrorCode(&ProcessingError{Code: "bad code"}); got != "processing_failed" {
		t.Fatalf("ErrorCode() = %q, want processing_failed", got)
	}
	if got := ErrorCode(errors.New("provider failed")); got != "processing_failed" {
		t.Fatalf("ErrorCode() = %q, want processing_failed", got)
	}
}

func TestUnavailableProcessorReturnsExplicitTypedError(t *testing.T) {
	err := (UnavailableProcessor{}).Process(context.Background(), uuid.New(), "temporary-path", 25)
	var processingErr *ProcessingError
	if !errors.As(err, &processingErr) {
		t.Fatalf("Process() error = %T, want *ProcessingError", err)
	}
	if got := ErrorCode(err); got != "parser_not_configured" {
		t.Fatalf("ErrorCode() = %q, want parser_not_configured", got)
	}
}
