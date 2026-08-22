package model

import (
	"time"

	"github.com/google/uuid"
)

// ClientUser is the client-facing identity backed by Google login. Only the
// minimal UI/identity fields are carried; no Google subject is exposed to the
// frontend, and no tokens or secrets are ever persisted here.
type ClientUser struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	AvatarURL   *string
	Provider    string
	CreatedAt   time.Time
	LastLoginAt time.Time
}

// ClientSession is a server-side client browser session. The raw token/cookie
// is never stored; only its SHA-256 hash and the CSRF token hash are kept.
type ClientSession struct {
	ID             uuid.UUID
	ClientUserID   uuid.UUID
	TokenHash      []byte
	CSRFTokenHash  []byte
	ExpiresAt      time.Time
	LastSeenAt     time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
	User           ClientUser
}

// ClientGoogleIdentity is the stable Google `sub` linkage for a client user.
type ClientGoogleIdentity struct {
	ID          uuid.UUID
	ClientUserID uuid.UUID
	GoogleSub   string
	Email       string
	DisplayName string
	AvatarURL   *string
}
