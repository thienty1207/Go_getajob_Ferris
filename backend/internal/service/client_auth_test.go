package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

type recordingClientRepository struct {
	createdUser     model.ClientUser
	createdIdentity model.ClientGoogleIdentity
	session         model.ClientSession
	rotatedCSRF     []byte
	loginTouched    bool
}

func (r *recordingClientRepository) CreateClientUserAndGoogleIdentity(_ context.Context, user model.ClientUser, identity model.ClientGoogleIdentity) (model.ClientUser, error) {
	r.createdUser = user
	r.createdIdentity = identity
	r.createdUser.ID = uuid.New()
	r.createdUser.Provider = "google"
	return r.createdUser, nil
}

func (r *recordingClientRepository) FindClientUserByGoogleSub(context.Context, string) (model.ClientUser, bool, error) {
	return r.createdUser, r.createdUser.ID != uuid.Nil, nil
}

func (r *recordingClientRepository) UpdateClientGoogleIdentity(context.Context, uuid.UUID, model.ClientGoogleIdentity) error {
	return nil
}

func (r *recordingClientRepository) UpdateClientUserProfile(context.Context, model.ClientUser) error {
	return nil
}

func (r *recordingClientRepository) TouchClientLastLogin(context.Context, uuid.UUID) error {
	r.loginTouched = true
	return nil
}

func (r *recordingClientRepository) CreateClientSession(_ context.Context, session model.ClientSession) error {
	r.session = session
	return nil
}

func (r *recordingClientRepository) FindClientSessionByTokenHash(context.Context, []byte) (model.ClientSession, error) {
	if r.session.ID == uuid.Nil {
		return model.ClientSession{}, repository.ErrClientSessionNotFound
	}
	return r.session, nil
}

func (r *recordingClientRepository) TouchClientSession(context.Context, uuid.UUID) error {
	return nil
}

func (r *recordingClientRepository) RotateClientCSRF(_ context.Context, _ uuid.UUID, hash []byte) error {
	r.rotatedCSRF = append([]byte(nil), hash...)
	r.session.CSRFTokenHash = append([]byte(nil), hash...)
	return nil
}

func (r *recordingClientRepository) RevokeClientSession(context.Context, uuid.UUID) error {
	return nil
}

func newClientAuthHarness(t *testing.T, verified *VerifiedGoogleToken) (*ClientAuthService, *recordingClientRepository) {
	t.Helper()
	repo := &recordingClientRepository{}
	auth := NewClientAuthService(repo, config.Config{
		GoogleClientID:     "client-id",
		GoogleClientSecret: "client-secret",
		GoogleRedirectURL:  "http://127.0.0.1:8080/api/v1/client/auth/google/callback",
		ClientSessionTTL:   time.Hour,
	})
	if verified != nil {
		auth.idTokenParse = func(_ context.Context, _ string) (VerifiedGoogleToken, error) {
			return *verified, nil
		}
	}
	auth.exchangeCode = func(_ context.Context, code string) (string, error) {
		if code == "" {
			return "", errors.New("empty code")
		}
		return "fake-id-token", nil
	}
	return auth, repo
}

func TestClientAuthStartLoginBuildsGoogleAuthURL(t *testing.T) {
	t.Parallel()
	auth, _ := newClientAuthHarness(t, nil)
	authURL, state, err := auth.StartLogin()
	if err != nil {
		t.Fatalf("StartLogin() error = %v", err)
	}
	if authURL == "" || state == "" {
		t.Fatal("StartLogin() returned empty auth URL or state")
	}
	if !errors.Is(err, nil) {
		t.Fatalf("StartLogin() error = %v", err)
	}
}

func TestClientAuthStartLoginFailsClosedWhenGoogleNotConfigured(t *testing.T) {
	t.Parallel()
	auth := NewClientAuthService(&recordingClientRepository{}, config.Config{})
	if _, _, err := auth.StartLogin(); !errors.Is(err, ErrGoogleNotConfigured) {
		t.Fatalf("StartLogin() error = %v, want ErrGoogleNotConfigured", err)
	}
	if _, err := auth.HandleCallback(context.Background(), "code", "state", "state"); !errors.Is(err, ErrGoogleNotConfigured) {
		t.Fatalf("HandleCallback() error = %v, want ErrGoogleNotConfigured", err)
	}
}

func TestClientAuthCallbackCreatesSessionAndUserFromVerifiedGoogleIdentity(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	verified := VerifiedGoogleToken{Sub: "google-sub-123", Email: "User@Example.com", EmailVerified: true, Name: "Example User", Picture: "https://example.com/avatar.png"}
	auth, repo := newClientAuthHarness(t, &verified)
	auth.now = func() time.Time { return now }

	result, err := auth.HandleCallback(context.Background(), "auth-code", "state", "state")
	if err != nil {
		t.Fatalf("HandleCallback() error = %v", err)
	}
	if result.User.Email != "user@example.com" {
		t.Fatalf("email = %q, want normalized lowercase", result.User.Email)
	}
	if result.SessionToken == "" || result.CSRFToken == "" || len(repo.session.TokenHash) != 32 || len(repo.session.CSRFTokenHash) != 32 {
		t.Fatalf("HandleCallback() produced unsafe session material: %#v", result)
	}
	if !auth.ValidateCSRF(repo.session, result.CSRFToken) || auth.ValidateCSRF(repo.session, "wrong") {
		t.Fatal("ValidateCSRF() did not enforce the session-bound token")
	}
	if !repo.loginTouched {
		t.Fatal("TouchClientLastLogin was not called")
	}
}

func TestClientAuthCallbackRejectsUnverifiedEmailAndBadState(t *testing.T) {
	t.Parallel()
	// email_verified=false must be rejected: an unverified address cannot be
	// used as a client identity.
	unverified := VerifiedGoogleToken{Sub: "sub", Email: "user@example.com", EmailVerified: false, Name: "User"}
	auth, _ := newClientAuthHarness(t, &unverified)
	if _, err := auth.HandleCallback(context.Background(), "code", "state", "state"); !errors.Is(err, ErrClientGoogleNotVerified) {
		t.Fatalf("unverified email error = %v, want ErrClientGoogleNotVerified", err)
	}

	// Mismatched one-time state must also be rejected.
	authValid, _ := newClientAuthHarness(t, &VerifiedGoogleToken{Sub: "sub", Email: "u@e.com", EmailVerified: true, Name: "U"})
	if _, err := authValid.HandleCallback(context.Background(), "code", "wrong", "state"); !errors.Is(err, ErrClientOAuthState) {
		t.Fatalf("mismatched state error = %v, want ErrClientOAuthState", err)
	}
}

func TestClientAuthAuthenticateFailsClosed(t *testing.T) {
	t.Parallel()
	auth, repo := newClientAuthHarness(t, nil)
	repo.session = model.ClientSession{ID: uuid.New(), ClientUserID: uuid.New(), ExpiresAt: time.Now().Add(-time.Hour)}
	if _, err := auth.Authenticate(context.Background(), "token"); !errors.Is(err, ErrClientSessionExpired) {
		t.Fatalf("expired session error = %v, want ErrClientSessionExpired", err)
	}
}

func TestClientAuthLegacyCSRFRefreshIsStableAndProtectedSessionCompatible(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	user := model.ClientUser{ID: uuid.New(), Email: "user@example.com", DisplayName: "User", Provider: "google"}
	repo := &recordingClientRepository{session: model.ClientSession{
		ID:            uuid.New(),
		ClientUserID:  user.ID,
		TokenHash:     hashToken("legacy-client-session-token"),
		CSRFTokenHash: hashToken("pre-cookie-client-csrf"),
		ExpiresAt:     now.Add(time.Hour),
		User:          user,
	}}
	auth := NewClientAuthService(repo, config.Config{})
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

	protectedSession, err := auth.Authenticate(context.Background(), "legacy-client-session-token")
	if err != nil {
		t.Fatalf("Authenticate() after legacy refresh error = %v", err)
	}
	if !auth.ValidateCSRF(protectedSession, first) || auth.ValidateCSRF(protectedSession, "pre-cookie-client-csrf") {
		t.Fatal("legacy refresh token did not remain compatible with protected-session validation")
	}
}
