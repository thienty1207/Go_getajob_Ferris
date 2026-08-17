package repository

import (
	"context"
	"errors"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const createAdminUserQuery = `
INSERT INTO public.admin_users (email, password_hash, is_active, created_at, updated_at)
VALUES ($1, $2, true, now(), now())
RETURNING id, email, is_active, last_login_at, created_at, updated_at`

const findAdminUserByEmailQuery = `
SELECT id, email, password_hash, is_active, last_login_at, created_at, updated_at
FROM public.admin_users
WHERE lower(email) = lower($1)`

const updateAdminLastLoginQuery = `
UPDATE public.admin_users
SET last_login_at = $2, updated_at = now()
WHERE id = $1`

const createAdminSessionQuery = `
INSERT INTO public.admin_sessions
    (id, admin_user_id, token_hash, csrf_token_hash, expires_at, last_seen_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)`

const findAdminSessionQuery = `
SELECT
    s.id, s.admin_user_id, s.token_hash, s.csrf_token_hash, s.expires_at,
    s.last_seen_at, s.revoked_at, s.created_at,
    u.id, u.email, u.is_active, u.last_login_at, u.created_at, u.updated_at
FROM public.admin_sessions AS s
JOIN public.admin_users AS u ON u.id = s.admin_user_id
WHERE s.token_hash = $1`

const touchAdminSessionQuery = `
UPDATE public.admin_sessions
SET last_seen_at = $2
WHERE id = $1 AND revoked_at IS NULL AND expires_at > now()`

const rotateAdminCSRFQuery = `
UPDATE public.admin_sessions
SET csrf_token_hash = $2
WHERE id = $1 AND revoked_at IS NULL AND expires_at > now()`

const revokeAdminSessionQuery = `
UPDATE public.admin_sessions
SET revoked_at = COALESCE(revoked_at, $2)
WHERE id = $1`

const createAdminAuditEventQuery = `
INSERT INTO public.admin_audit_events
    (admin_user_id, action, result, resource_type, resource_key, request_id)
VALUES ($1, $2, $3, $4, $5, $6)`

type PostgresAdminRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAdminRepository binds auth persistence to the shared pool. The
// CLI and API use the same repository so provisioning and runtime auth follow
// one database contract.
func NewPostgresAdminRepository(pool *pgxpool.Pool) *PostgresAdminRepository {
	return &PostgresAdminRepository{pool: pool}
}

func (r *PostgresAdminRepository) CreateAdminUser(ctx context.Context, email, passwordHash string) (model.AdminUser, error) {
	var user model.AdminUser
	err := r.pool.QueryRow(ctx, createAdminUserQuery, email, passwordHash).Scan(
		&user.ID,
		&user.Email,
		&user.IsActive,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" {
			return model.AdminUser{}, ErrAdminEmailExists
		}
		return model.AdminUser{}, err
	}
	return user, nil
}

func (r *PostgresAdminRepository) FindAdminUserByEmail(ctx context.Context, email string) (model.AdminUser, string, error) {
	var (
		user         model.AdminUser
		passwordHash string
	)
	err := r.pool.QueryRow(ctx, findAdminUserByEmailQuery, email).Scan(
		&user.ID,
		&user.Email,
		&passwordHash,
		&user.IsActive,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AdminUser{}, "", ErrAdminUserNotFound
	}
	if err != nil {
		return model.AdminUser{}, "", err
	}
	return user, passwordHash, nil
}

func (r *PostgresAdminRepository) UpdateAdminLastLogin(ctx context.Context, userID uuid.UUID, loggedInAt time.Time) error {
	_, err := r.pool.Exec(ctx, updateAdminLastLoginQuery, userID, loggedInAt)
	return err
}

func (r *PostgresAdminRepository) CreateAdminSession(ctx context.Context, session model.AdminSession) error {
	_, err := r.pool.Exec(
		ctx,
		createAdminSessionQuery,
		session.ID,
		session.AdminUserID,
		session.TokenHash,
		session.CSRFTokenHash,
		session.ExpiresAt,
		session.LastSeenAt,
		session.CreatedAt,
	)
	return err
}

func (r *PostgresAdminRepository) FindAdminSessionByTokenHash(ctx context.Context, tokenHash []byte) (model.AdminSession, error) {
	var session model.AdminSession
	err := r.pool.QueryRow(ctx, findAdminSessionQuery, tokenHash).Scan(
		&session.ID,
		&session.AdminUserID,
		&session.TokenHash,
		&session.CSRFTokenHash,
		&session.ExpiresAt,
		&session.LastSeenAt,
		&session.RevokedAt,
		&session.CreatedAt,
		&session.User.ID,
		&session.User.Email,
		&session.User.IsActive,
		&session.User.LastLoginAt,
		&session.User.CreatedAt,
		&session.User.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AdminSession{}, ErrAdminSessionNotFound
	}
	if err != nil {
		return model.AdminSession{}, err
	}
	return session, nil
}

func (r *PostgresAdminRepository) TouchAdminSession(ctx context.Context, sessionID uuid.UUID, seenAt time.Time) error {
	_, err := r.pool.Exec(ctx, touchAdminSessionQuery, sessionID, seenAt)
	return err
}

func (r *PostgresAdminRepository) RotateAdminCSRF(ctx context.Context, sessionID uuid.UUID, csrfHash []byte) error {
	_, err := r.pool.Exec(ctx, rotateAdminCSRFQuery, sessionID, csrfHash)
	return err
}

func (r *PostgresAdminRepository) RevokeAdminSession(ctx context.Context, sessionID uuid.UUID, revokedAt time.Time) error {
	_, err := r.pool.Exec(ctx, revokeAdminSessionQuery, sessionID, revokedAt)
	return err
}

func (r *PostgresAdminRepository) CreateAdminAuditEvent(ctx context.Context, event model.AdminAuditEvent) error {
	_, err := r.pool.Exec(
		ctx,
		createAdminAuditEventQuery,
		event.AdminUserID,
		event.Action,
		event.Result,
		event.ResourceType,
		event.ResourceKey,
		event.RequestID,
	)
	return err
}
