package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

func TestValidateAdminPasswordExplainsProvisioningPolicy(t *testing.T) {
	err := ValidateAdminPassword("short")
	if err == nil {
		t.Fatal("ValidateAdminPassword() error = nil, want policy error")
	}
	if !strings.Contains(err.Error(), "12") || !strings.Contains(err.Error(), "200") || !strings.Contains(err.Error(), "line breaks") {
		t.Fatalf("ValidateAdminPassword() error = %q, want actionable length and line-break guidance", err)
	}
}

func TestAdminAuthLoginCreatesHashedSessionAndRefreshesCSRF(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	hash, err := HashAdminPassword("correct horse battery")
	if err != nil {
		t.Fatalf("HashAdminPassword() error = %v", err)
	}
	repo := &recordingAdminRepository{user: model.AdminUser{ID: uuid.New(), Email: "admin@example.com", IsActive: true}, passwordHash: hash}
	auth := NewAdminAuthService(repo, config.Config{AdminSessionTTL: time.Hour})
	auth.now = func() time.Time { return now }

	result, err := auth.Login(context.Background(), " Admin@Example.com ", "correct horse battery")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.SessionToken == "" || result.CSRFToken == "" || len(result.Session.TokenHash) != 32 || len(result.Session.CSRFTokenHash) != 32 {
		t.Fatalf("Login() returned unsafe session material: %#v", result)
	}
	if string(result.Session.TokenHash) == result.SessionToken || string(result.Session.CSRFTokenHash) == result.CSRFToken {
		t.Fatal("session persistence must contain digests, not raw tokens")
	}
	if !auth.ValidateCSRF(result.Session, result.CSRFToken) || auth.ValidateCSRF(result.Session, "wrong") {
		t.Fatal("ValidateCSRF() did not enforce the session-bound token")
	}

	repo.session = result.Session
	repo.session.User = result.User
	got, err := auth.Authenticate(context.Background(), result.SessionToken)
	if err != nil || got.ID != result.Session.ID {
		t.Fatalf("Authenticate() = %#v, err=%v", got, err)
	}
	refreshed, err := auth.RefreshCSRF(context.Background(), result.Session)
	if err != nil || refreshed == result.CSRFToken || repo.rotatedCSRF == nil {
		t.Fatalf("RefreshCSRF() = %q, err=%v, hash=%x", refreshed, err, repo.rotatedCSRF)
	}
}

func TestAdminAuthRejectsInvalidCredentialsWithoutCreatingSession(t *testing.T) {
	t.Parallel()
	hash, err := HashAdminPassword("correct horse battery")
	if err != nil {
		t.Fatalf("HashAdminPassword() error = %v", err)
	}
	repo := &recordingAdminRepository{user: model.AdminUser{ID: uuid.New(), Email: "admin@example.com", IsActive: true}, passwordHash: hash}
	auth := NewAdminAuthService(repo, config.Config{})

	if _, err := auth.Login(context.Background(), "admin@example.com", "wrong password"); !errors.Is(err, ErrInvalidAdminCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidAdminCredentials", err)
	}
	if repo.session.ID != uuid.Nil {
		t.Fatal("invalid credentials created a session")
	}
	if _, err := auth.Login(context.Background(), "missing@example.com", "correct horse battery"); !errors.Is(err, ErrInvalidAdminCredentials) {
		t.Fatalf("missing-user Login() error = %v, want generic invalid credentials", err)
	}
}

func TestAdminAuthRejectsExpiredAndRevokedSessions(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	user := model.AdminUser{ID: uuid.New(), Email: "admin@example.com", IsActive: true}
	repo := &recordingAdminRepository{session: model.AdminSession{ID: uuid.New(), ExpiresAt: now.Add(-time.Minute), User: user}}
	auth := NewAdminAuthService(repo, config.Config{})
	auth.now = func() time.Time { return now }
	if _, err := auth.Authenticate(context.Background(), "session"); !errors.Is(err, ErrAdminSessionExpired) {
		t.Fatalf("expired Authenticate() error = %v, want expired", err)
	}
	repo.session.ExpiresAt = now.Add(time.Hour)
	revokedAt := now.Add(-time.Minute)
	repo.session.RevokedAt = &revokedAt
	if _, err := auth.Authenticate(context.Background(), "session"); !errors.Is(err, ErrAdminSessionRevoked) {
		t.Fatalf("revoked Authenticate() error = %v, want revoked", err)
	}
}

func TestAdminAuthLegacyCSRFRefreshIsStableAndProtectedSessionCompatible(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	user := model.AdminUser{ID: uuid.New(), Email: "admin@example.com", IsActive: true}
	repo := &recordingAdminRepository{session: model.AdminSession{
		ID:            uuid.New(),
		AdminUserID:   user.ID,
		TokenHash:     hashToken("legacy-session-token"),
		CSRFTokenHash: hashToken("pre-cookie-csrf-token"),
		ExpiresAt:     now.Add(time.Hour),
		User:          user,
	}}
	auth := NewAdminAuthService(repo, config.Config{})
	auth.now = func() time.Time { return now }

	first, err := auth.RefreshCSRF(context.Background(), repo.session)
	if err != nil {
		t.Fatalf("first RefreshCSRF() error = %v", err)
	}
	second, err := auth.RefreshCSRF(context.Background(), repo.session)
	if err != nil {
		t.Fatalf("second RefreshCSRF() error = %v", err)
	}
	if first == "" || second != first {
		t.Fatalf("legacy refresh tokens = %q and %q, want one stable session-bound token", first, second)
	}

	protectedSession, err := auth.Authenticate(context.Background(), "legacy-session-token")
	if err != nil {
		t.Fatalf("Authenticate() after legacy refresh error = %v", err)
	}
	if !auth.ValidateCSRF(protectedSession, first) || auth.ValidateCSRF(protectedSession, "pre-cookie-csrf-token") {
		t.Fatal("legacy refresh token did not remain compatible with protected-session validation")
	}
}

type recordingAdminRepository struct {
	user         model.AdminUser
	passwordHash string
	session      model.AdminSession
	rotatedCSRF  []byte
	lastAudit    model.AdminAuditEvent
}

func (r *recordingAdminRepository) CreateAdminUser(context.Context, string, string) (model.AdminUser, error) {
	return r.user, nil
}

func (r *recordingAdminRepository) FindAdminUserByEmail(_ context.Context, email string) (model.AdminUser, string, error) {
	if r.user.ID == uuid.Nil || r.user.Email != email {
		return model.AdminUser{}, "", repository.ErrAdminUserNotFound
	}
	return r.user, r.passwordHash, nil
}

func (r *recordingAdminRepository) UpdateAdminLastLogin(context.Context, uuid.UUID, time.Time) error {
	return nil
}

func (r *recordingAdminRepository) CreateAdminSession(_ context.Context, session model.AdminSession) error {
	r.session = session
	return nil
}

func (r *recordingAdminRepository) FindAdminSessionByTokenHash(context.Context, []byte) (model.AdminSession, error) {
	if r.session.ID == uuid.Nil {
		return model.AdminSession{}, repository.ErrAdminSessionNotFound
	}
	return r.session, nil
}

func (r *recordingAdminRepository) TouchAdminSession(context.Context, uuid.UUID, time.Time) error {
	return nil
}

func (r *recordingAdminRepository) RotateAdminCSRF(_ context.Context, _ uuid.UUID, hash []byte) error {
	r.rotatedCSRF = append([]byte(nil), hash...)
	r.session.CSRFTokenHash = append([]byte(nil), hash...)
	return nil
}

func (r *recordingAdminRepository) RevokeAdminSession(context.Context, uuid.UUID, time.Time) error {
	return nil
}

func (r *recordingAdminRepository) CreateAdminAuditEvent(_ context.Context, event model.AdminAuditEvent) error {
	r.lastAudit = event
	return nil
}
