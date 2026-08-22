package repository

import (
	"context"

	"github.com/gogetsomefoodferris/backend/internal/model"
)

type AdminClientUserFilter struct {
	Search string
}

type AdminClientUserRepository interface {
	ListAdminClientUsers(context.Context, int, int, AdminClientUserFilter) (model.AdminClientUserPage, error)
}
