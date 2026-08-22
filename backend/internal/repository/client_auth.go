package repository

import (
	"context"
	"errors"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

var (
	ErrClientUserNotFound    = errors.New("client user not found")
	ErrClientSessionNotFound = errors.New("client session not found")
)

// ClientAuthRepository owns persistence of client identity and sessions. It
// never exposes raw session tokens or Google secrets to callers. It is
// deliberately separated from the admin repository: a client session never
// becomes an admin session.
type ClientAuthRepository interface {
	CreateClientUserAndGoogleIdentity(context.Context, model.ClientUser, model.ClientGoogleIdentity) (model.ClientUser, error)
	FindClientUserByGoogleSub(context.Context, string) (model.ClientUser, bool, error)
	UpdateClientGoogleIdentity(context.Context, uuid.UUID, model.ClientGoogleIdentity) error
	UpdateClientUserProfile(context.Context, model.ClientUser) error
	TouchClientLastLogin(context.Context, uuid.UUID) error
	CreateClientSession(context.Context, model.ClientSession) error
	FindClientSessionByTokenHash(context.Context, []byte) (model.ClientSession, error)
	TouchClientSession(context.Context, uuid.UUID) error
	RotateClientCSRF(context.Context, uuid.UUID, []byte) error
	RevokeClientSession(context.Context, uuid.UUID) error
}
