package repository

import (
	"context"
	"errors"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

var (
	// These sentinel errors are translated at the HTTP boundary; SQL details
	// stay inside the repository package.
	ErrScanNotFound     = errors.New("scan not found")
	ErrInvalidScanState = errors.New("invalid scan state")
)

// ScanRepository owns durable scan lifecycle and result reads. It keeps SQL
// implementation details out of the service and handler layers.
type ScanRepository interface {
	CreateScan(context.Context, uuid.UUID, float64) (uuid.UUID, error)
	SetStatus(context.Context, uuid.UUID, model.ScanStatus, *string) error
	GetScan(context.Context, uuid.UUID) (model.Scan, error)
}
