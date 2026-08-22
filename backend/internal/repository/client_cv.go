package repository

import (
	"context"
	"errors"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

var ErrClientCVNotFound = errors.New("client cv not found")

// ClientCVRepository owns the authenticated user's structured CV history.
// The owner ID always comes from the verified server session.
type ClientCVRepository interface {
	ListClientCVHistory(context.Context, uuid.UUID, int, int) ([]model.ClientCVHistoryItem, int, error)
	DeleteClientCV(context.Context, uuid.UUID, uuid.UUID) error
}
