package repository

import (
	"context"
	"errors"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const insertClientUserQuery = `
INSERT INTO public.client_users (email, display_name, avatar_url, provider, last_login_at, updated_at)
VALUES (lower($1), $2, $3, 'google', now(), now())
RETURNING id, email, display_name, avatar_url, provider, created_at, last_login_at`

const insertClientGoogleIdentityQuery = `
INSERT INTO public.client_google_identities (client_user_id, google_sub, email, display_name, avatar_url, updated_at)
VALUES ($1, $2, lower($3), $4, $5, now())`

const findClientUserByGoogleSubQuery = `
SELECT
    u.id, u.email, u.display_name, u.avatar_url, u.provider, u.created_at, u.last_login_at
FROM public.client_users AS u
JOIN public.client_google_identities AS gi ON gi.client_user_id = u.id
WHERE gi.google_sub = $1
LIMIT 1`

const updateClientGoogleIdentityQuery = `
UPDATE public.client_google_identities
SET email = lower($3), display_name = $4, avatar_url = $5, updated_at = now()
WHERE client_user_id = $1 AND google_sub = $2`

const updateClientUserProfileQuery = `
UPDATE public.client_users
SET email = lower($2), display_name = $3, avatar_url = $4, updated_at = now()
WHERE id = $1`

const touchClientLastLoginQuery = `
UPDATE public.client_users
SET last_login_at = now(), updated_at = now()
WHERE id = $1`

const createClientSessionQuery = `
INSERT INTO public.client_sessions
    (id, client_user_id, token_hash, csrf_token_hash, expires_at, last_seen_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)`

const findClientSessionQuery = `
SELECT
    s.id, s.client_user_id, s.token_hash, s.csrf_token_hash, s.expires_at,
    s.last_seen_at, s.revoked_at, s.created_at,
    u.id, u.email, u.display_name, u.avatar_url, u.provider, u.created_at, u.last_login_at
FROM public.client_sessions AS s
JOIN public.client_users AS u ON u.id = s.client_user_id
WHERE s.token_hash = $1`

const touchClientSessionQuery = `
UPDATE public.client_sessions
SET last_seen_at = $2
WHERE id = $1 AND revoked_at IS NULL AND expires_at > now()`

const rotateClientCSRFQuery = `
UPDATE public.client_sessions
SET csrf_token_hash = $2
WHERE id = $1 AND revoked_at IS NULL AND expires_at > now()`

const revokeClientSessionQuery = `
UPDATE public.client_sessions
SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL`

type PostgresClientAuthRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresClientAuthRepository(pool *pgxpool.Pool) *PostgresClientAuthRepository {
	return &PostgresClientAuthRepository{pool: pool}
}

// CreateClientUserAndGoogleIdentity creates or updates a client user anchored by
// its Google identity. The Google subject (sub) is the identity anchor, never
// the email, so two Google accounts that share one email never converge onto
// the same user/identity. The whole operation is transactional so a partial
// write never leaves a session bound to a half-created user.
func (r *PostgresClientAuthRepository) CreateClientUserAndGoogleIdentity(ctx context.Context, user model.ClientUser, identity model.ClientGoogleIdentity) (model.ClientUser, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return model.ClientUser{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Resolve any existing user already bound to this Google subject first.
	var created model.ClientUser
	var userID uuid.UUID
	err = tx.QueryRow(
		ctx,
		findClientUserByGoogleSubQuery,
		identity.GoogleSub,
	).Scan(&userID, &created.Email, &created.DisplayName, &created.AvatarURL, &created.Provider, &created.CreatedAt, &created.LastLoginAt)

	switch {
	case err == nil:
		// Existing identity: refresh the user profile to the latest verified data.
		if _, err := tx.Exec(ctx, updateClientUserProfileQuery, userID, user.Email, user.DisplayName, user.AvatarURL); err != nil {
			return model.ClientUser{}, err
		}
		if _, err := tx.Exec(ctx, updateClientGoogleIdentityQuery, userID, identity.GoogleSub, identity.Email, identity.DisplayName, identity.AvatarURL); err != nil {
			return model.ClientUser{}, err
		}
		created.ID = userID
	case errors.Is(err, pgx.ErrNoRows):
		// New Google identity: create a fresh client user row for it.
		if err := tx.QueryRow(
			ctx,
			insertClientUserQuery,
			user.Email,
			user.DisplayName,
			user.AvatarURL,
		).Scan(
			&created.ID,
			&created.Email,
			&created.DisplayName,
			&created.AvatarURL,
			&created.Provider,
			&created.CreatedAt,
			&created.LastLoginAt,
		); err != nil {
			return model.ClientUser{}, err
		}
		if _, err := tx.Exec(
			ctx,
			insertClientGoogleIdentityQuery,
			created.ID,
			identity.GoogleSub,
			identity.Email,
			identity.DisplayName,
			identity.AvatarURL,
		); err != nil {
			return model.ClientUser{}, err
		}
	default:
		return model.ClientUser{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return model.ClientUser{}, err
	}
	return created, nil
}

// FindClientUserByGoogleSub returns the client user bound to a Google subject.
func (r *PostgresClientAuthRepository) FindClientUserByGoogleSub(ctx context.Context, googleSub string) (model.ClientUser, bool, error) {
	var user model.ClientUser
	err := r.pool.QueryRow(ctx, findClientUserByGoogleSubQuery, googleSub).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.AvatarURL,
		&user.Provider,
		&user.CreatedAt,
		&user.LastLoginAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ClientUser{}, false, nil
	}
	if err != nil {
		return model.ClientUser{}, false, err
	}
	return user, true, nil
}

func (r *PostgresClientAuthRepository) UpdateClientGoogleIdentity(ctx context.Context, userID uuid.UUID, identity model.ClientGoogleIdentity) error {
	_, err := r.pool.Exec(ctx, updateClientGoogleIdentityQuery, userID, identity.GoogleSub, identity.Email, identity.DisplayName, identity.AvatarURL)
	return err
}

func (r *PostgresClientAuthRepository) UpdateClientUserProfile(ctx context.Context, user model.ClientUser) error {
	_, err := r.pool.Exec(ctx, updateClientUserProfileQuery, user.ID, user.Email, user.DisplayName, user.AvatarURL)
	return err
}

func (r *PostgresClientAuthRepository) TouchClientLastLogin(ctx context.Context, clientUserID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, touchClientLastLoginQuery, clientUserID)
	return err
}

func (r *PostgresClientAuthRepository) CreateClientSession(ctx context.Context, session model.ClientSession) error {
	_, err := r.pool.Exec(
		ctx,
		createClientSessionQuery,
		session.ID,
		session.ClientUserID,
		session.TokenHash,
		session.CSRFTokenHash,
		session.ExpiresAt,
		session.LastSeenAt,
		session.CreatedAt,
	)
	return err
}

func (r *PostgresClientAuthRepository) FindClientSessionByTokenHash(ctx context.Context, tokenHash []byte) (model.ClientSession, error) {
	var session model.ClientSession
	err := r.pool.QueryRow(ctx, findClientSessionQuery, tokenHash).Scan(
		&session.ID,
		&session.ClientUserID,
		&session.TokenHash,
		&session.CSRFTokenHash,
		&session.ExpiresAt,
		&session.LastSeenAt,
		&session.RevokedAt,
		&session.CreatedAt,
		&session.User.ID,
		&session.User.Email,
		&session.User.DisplayName,
		&session.User.AvatarURL,
		&session.User.Provider,
		&session.User.CreatedAt,
		&session.User.LastLoginAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ClientSession{}, ErrClientSessionNotFound
	}
	if err != nil {
		return model.ClientSession{}, err
	}
	return session, nil
}

func (r *PostgresClientAuthRepository) TouchClientSession(ctx context.Context, sessionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, touchClientSessionQuery, sessionID, time.Now().UTC())
	return err
}

func (r *PostgresClientAuthRepository) RotateClientCSRF(ctx context.Context, sessionID uuid.UUID, csrfHash []byte) error {
	_, err := r.pool.Exec(ctx, rotateClientCSRFQuery, sessionID, csrfHash)
	return err
}

func (r *PostgresClientAuthRepository) RevokeClientSession(ctx context.Context, sessionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, revokeClientSessionQuery, sessionID)
	return err
}

var _ ClientAuthRepository = (*PostgresClientAuthRepository)(nil)
