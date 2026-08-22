package service

import (
	"context"
	"errors"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

var ErrInvalidClientCVUser = errors.New("invalid client cv user")

// ClientCVService keeps the history use-case owner-scoped before it reaches
// the HTTP response mapper. It returns structured profile data only.
type ClientCVService struct {
	repository repository.ClientCVRepository
}

func NewClientCVService(clientCVRepository repository.ClientCVRepository) *ClientCVService {
	return &ClientCVService{repository: clientCVRepository}
}

func (s *ClientCVService) List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.ClientCVHistoryItem, int, error) {
	if userID == uuid.Nil {
		return nil, 0, ErrInvalidClientCVUser
	}
	return s.repository.ListClientCVHistory(ctx, userID, limit, offset)
}

func (s *ClientCVService) Delete(ctx context.Context, userID, scanID uuid.UUID) error {
	if userID == uuid.Nil || scanID == uuid.Nil {
		return ErrInvalidClientCVUser
	}
	return s.repository.DeleteClientCV(ctx, userID, scanID)
}
