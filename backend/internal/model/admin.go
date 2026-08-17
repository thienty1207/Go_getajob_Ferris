package model

import (
	"time"

	"github.com/google/uuid"
)

// AdminUser is the safe identity summary used by the HTTP layer. Password
// hashes stay inside the authentication repository/service boundary and are
// never part of this model's JSON mapping.
type AdminUser struct {
	ID          uuid.UUID
	Email       string
	IsActive    bool
	LastLoginAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AdminSession is the server-side session record returned by the repository.
// TokenHash and CSRFTokenHash are digests, never the raw browser tokens.
type AdminSession struct {
	ID            uuid.UUID
	AdminUserID   uuid.UUID
	TokenHash     []byte
	CSRFTokenHash []byte
	ExpiresAt     time.Time
	LastSeenAt    time.Time
	RevokedAt     *time.Time
	CreatedAt     time.Time
	User          AdminUser
}

// AdminAuditEvent contains only operational metadata. It deliberately has no
// fields for passwords, cookies, CV contents, job descriptions, or provider
// secrets so future audit callers cannot accidentally persist them.
type AdminAuditEvent struct {
	AdminUserID  *uuid.UUID
	Action       string
	Result       string
	ResourceType *string
	ResourceKey  *string
	RequestID    *string
}
