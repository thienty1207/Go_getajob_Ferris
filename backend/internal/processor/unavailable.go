package processor

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// UnavailableProcessor is the explicit no-mock fallback. It marks a scan as
// failed instead of inventing a parsed profile or match result.
type UnavailableProcessor struct{}

func (UnavailableProcessor) Process(context.Context, uuid.UUID, string, float64) error {
	return &ProcessingError{Code: "parser_not_configured", Err: errors.New("no scan parser is configured")}
}
