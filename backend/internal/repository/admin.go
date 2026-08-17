package repository

import (
	"context"
	"errors"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

var (
	ErrAdminUserNotFound    = errors.New("admin user not found")
	ErrAdminEmailExists     = errors.New("admin email already exists")
	ErrAdminSessionNotFound = errors.New("admin session not found")
)

// AdminRepository owns the persistence of identities, sessions, and audit
// events. It never exposes raw passwords or session tokens to callers.
type AdminRepository interface {
	CreateAdminUser(context.Context, string, string) (model.AdminUser, error)
	FindAdminUserByEmail(context.Context, string) (model.AdminUser, string, error)
	UpdateAdminLastLogin(context.Context, uuid.UUID, time.Time) error
	CreateAdminSession(context.Context, model.AdminSession) error
	FindAdminSessionByTokenHash(context.Context, []byte) (model.AdminSession, error)
	TouchAdminSession(context.Context, uuid.UUID, time.Time) error
	RotateAdminCSRF(context.Context, uuid.UUID, []byte) error
	RevokeAdminSession(context.Context, uuid.UUID, time.Time) error
	CreateAdminAuditEvent(context.Context, model.AdminAuditEvent) error
}
