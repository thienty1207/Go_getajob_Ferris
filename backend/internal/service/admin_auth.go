package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidAdminCredentials = errors.New("invalid admin credentials")
	ErrAdminSessionMissing     = errors.New("admin session missing")
	ErrAdminSessionExpired     = errors.New("admin session expired")
	ErrAdminSessionRevoked     = errors.New("admin session revoked")
	ErrAdminInactive           = errors.New("admin account inactive")
	ErrAdminCSRFInvalid        = errors.New("admin csrf token invalid")
	ErrInvalidAdminEmail       = errors.New("invalid admin email")
	ErrInvalidAdminPassword    = errors.New("invalid admin password: use 12-200 characters and no line breaks")
	ErrAdminAuthStorage        = errors.New("admin authentication storage failure")
)

const (
	adminTokenBytes        = 32
	adminMinPassword       = 12
	adminMaxPassword       = 200
	adminLegacyCSRFPurpose = "ferris/admin/csrf-cookie/v1"
	bcryptCost             = bcrypt.DefaultCost
)

// AdminAuthService owns credential verification and session token lifecycle.
// HTTP handlers never receive a password hash and repositories never receive a
// raw session token, which keeps the secret boundaries explicit.
type AdminAuthService struct {
	repository repository.AdminRepository
	sessionTTL time.Duration
	now        func() time.Time
}

// NewAdminAuthService creates the auth use-case with a bounded session TTL.
// The fallback protects tests or an incomplete local Config from accidentally
// creating sessions that never expire.
func NewAdminAuthService(adminRepository repository.AdminRepository, cfg config.Config) *AdminAuthService {
	ttl := cfg.AdminSessionTTL
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return &AdminAuthService{repository: adminRepository, sessionTTL: ttl, now: time.Now}
}

// NormalizeAdminEmail makes lookup and provisioning case-insensitive while
// preserving a single canonical value in the database.
func NormalizeAdminEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > 320 {
		return "", ErrInvalidAdminEmail
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || strings.ContainsAny(email, "\r\n") {
		return "", ErrInvalidAdminEmail
	}
	return email, nil
}

// ValidateAdminPassword keeps weak bootstrap credentials out of the database
// and avoids accepting arbitrarily large request bodies as password input.
func ValidateAdminPassword(password string) error {
	if len(password) < adminMinPassword || len(password) > adminMaxPassword || strings.ContainsAny(password, "\r\n") {
		return ErrInvalidAdminPassword
	}
	return nil
}

// HashAdminPassword is shared by the provisioning CLI and login tests. The
// bcrypt cost is intentionally centralized so future upgrades are auditable.
func HashAdminPassword(password string) (string, error) {
	if err := ValidateAdminPassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash admin password: %w", err)
	}
	return string(hash), nil
}

// LoginResult contains the only point where raw browser tokens exist. The
// handler immediately places SessionToken in an HttpOnly cookie and returns
// CSRFToken in JSON for a custom-header request check.
type LoginResult struct {
	User         model.AdminUser
	Session      model.AdminSession
	SessionToken string
	CSRFToken    string
}

// Login verifies credentials, creates a revocable session, and updates the
// last-login timestamp. Invalid credentials are deliberately collapsed to one
// public error so callers cannot distinguish an unknown email from a bad one.
func (s *AdminAuthService) Login(ctx context.Context, rawEmail, password string) (LoginResult, error) {
	email, err := NormalizeAdminEmail(rawEmail)
	if err != nil || ValidateAdminPassword(password) != nil {
		return LoginResult{}, ErrInvalidAdminCredentials
	}

	user, passwordHash, err := s.repository.FindAdminUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrAdminUserNotFound) {
			return LoginResult{}, ErrInvalidAdminCredentials
		}
		return LoginResult{}, fmt.Errorf("find admin user: %w", ErrAdminAuthStorage)
	}
	if !user.IsActive || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return LoginResult{}, ErrInvalidAdminCredentials
	}

	now := s.now()
	sessionToken, err := randomToken()
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate admin session token: %w", err)
	}
	csrfToken, err := randomToken()
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate admin csrf token: %w", err)
	}
	session := model.AdminSession{
		ID:            uuid.New(),
		AdminUserID:   user.ID,
		TokenHash:     hashToken(sessionToken),
		CSRFTokenHash: hashToken(csrfToken),
		ExpiresAt:     now.Add(s.sessionTTL),
		LastSeenAt:    now,
		CreatedAt:     now,
		User:          user,
	}
	if err := s.repository.CreateAdminSession(ctx, session); err != nil {
		return LoginResult{}, fmt.Errorf("create admin session: %w", ErrAdminAuthStorage)
	}
	// A timestamp is useful for operational review but is not part of the auth
	// decision. If this best-effort update fails, the already-created session is
	// still valid and the caller can continue without leaking a secret.
	_ = s.repository.UpdateAdminLastLogin(ctx, user.ID, now)
	user.LastLoginAt = &now
	return LoginResult{User: user, Session: session, SessionToken: sessionToken, CSRFToken: csrfToken}, nil
}

// Authenticate resolves a raw cookie token to a live session and user. The
// token is hashed before the repository sees it, and stale/revoked sessions
// fail closed even if a browser still sends their cookie.
func (s *AdminAuthService) Authenticate(ctx context.Context, rawToken string) (model.AdminSession, error) {
	if strings.TrimSpace(rawToken) == "" {
		return model.AdminSession{}, ErrAdminSessionMissing
	}
	session, err := s.repository.FindAdminSessionByTokenHash(ctx, hashToken(rawToken))
	if err != nil {
		if errors.Is(err, repository.ErrAdminSessionNotFound) {
			return model.AdminSession{}, ErrAdminSessionMissing
		}
		return model.AdminSession{}, fmt.Errorf("find admin session: %w", ErrAdminAuthStorage)
	}
	now := s.now()
	if session.RevokedAt != nil {
		return model.AdminSession{}, ErrAdminSessionRevoked
	}
	if !session.ExpiresAt.After(now) {
		return model.AdminSession{}, ErrAdminSessionExpired
	}
	if !session.User.IsActive {
		return model.AdminSession{}, ErrAdminInactive
	}
	_ = s.repository.TouchAdminSession(ctx, session.ID, now)
	return session, nil
}

// ValidateCSRF compares a presented header token against the digest bound to
// the current session. Constant-time comparison avoids an unnecessary timing
// signal for a state-changing request.
func (s *AdminAuthService) ValidateCSRF(session model.AdminSession, rawToken string) bool {
	presented := hashToken(strings.TrimSpace(rawToken))
	return subtle.ConstantTimeCompare(presented, session.CSRFTokenHash) == 1
}

// RefreshCSRF is the compatibility path for sessions created before the
// HttpOnly CSRF cookie existed. Its session-bound derivation is deterministic,
// so concurrent legacy tabs cannot race to install different sole hashes.
func (s *AdminAuthService) RefreshCSRF(ctx context.Context, session model.AdminSession) (string, error) {
	if err := s.ensureLiveSession(session); err != nil {
		return "", err
	}
	rawToken := deriveSessionCSRFToken(adminLegacyCSRFPurpose, session.ID, session.TokenHash)
	if err := s.repository.RotateAdminCSRF(ctx, session.ID, hashToken(rawToken)); err != nil {
		return "", fmt.Errorf("rotate admin csrf token: %w", ErrAdminAuthStorage)
	}
	return rawToken, nil
}

// Logout revokes a session at the server, making browser-cookie deletion only
// a convenience rather than the security boundary. Repeating logout is safe.
func (s *AdminAuthService) Logout(ctx context.Context, session model.AdminSession) error {
	if err := s.repository.RevokeAdminSession(ctx, session.ID, s.now()); err != nil {
		return fmt.Errorf("revoke admin session: %w", ErrAdminAuthStorage)
	}
	return nil
}

func (s *AdminAuthService) ensureLiveSession(session model.AdminSession) error {
	now := s.now()
	if session.RevokedAt != nil {
		return ErrAdminSessionRevoked
	}
	if !session.ExpiresAt.After(now) {
		return ErrAdminSessionExpired
	}
	if !session.User.IsActive {
		return ErrAdminInactive
	}
	return nil
}

// RecordAudit is intentionally best-effort for successful UI operations: an
// audit outage must be visible in logs but must not turn an already committed
// promotion update into a confusing client failure.
func (s *AdminAuthService) RecordAudit(ctx context.Context, event model.AdminAuditEvent) error {
	return s.repository.CreateAdminAuditEvent(ctx, event)
}

func randomToken() (string, error) {
	bytes := make([]byte, adminTokenBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func hashToken(raw string) []byte {
	digest := sha256.Sum256([]byte(raw))
	return digest[:]
}

func deriveSessionCSRFToken(purpose string, sessionID uuid.UUID, sessionTokenHash []byte) string {
	mac := hmac.New(sha256.New, sessionTokenHash)
	_, _ = mac.Write([]byte(purpose))
	_, _ = mac.Write(sessionID[:])
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
