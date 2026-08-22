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

// ClientOwnedScanRepository is the ownership-aware extension used once the
// client has authenticated. The legacy methods remain for migration-safe
// tests and old anonymous rows, but production client requests must use these
// methods so the user identity comes from the server session.
type ClientOwnedScanRepository interface {
	CreateScanForClient(context.Context, uuid.UUID, *uuid.UUID) (uuid.UUID, error)
	GetScanForClient(context.Context, uuid.UUID, uuid.UUID) (model.Scan, error)
}
