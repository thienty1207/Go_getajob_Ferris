package repository

import (
	"context"
	"errors"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

var ErrAdminCVNotFound = errors.New("admin cv not found")

type AdminCVFilter struct {
	User string
	Role string
}

type AdminCVRepository interface {
	ListAdminCVProfiles(context.Context, int, int, AdminCVFilter) (model.AdminCVProfilePage, error)
	DeleteAdminCV(context.Context, uuid.UUID) error
}
