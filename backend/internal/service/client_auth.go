package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

const clientLegacyCSRFPurpose = "ferris/client/csrf-cookie/v1"

var (
	ErrGoogleNotConfigured     = errors.New("google client login is not configured")
	ErrClientAuthStorage       = errors.New("client authentication storage failure")
	ErrClientSessionMissing    = errors.New("client session missing")
	ErrClientSessionExpired    = errors.New("client session expired")
	ErrClientSessionRevoked    = errors.New("client session revoked")
	ErrClientCSRFInvalid       = errors.New("client csrf token invalid")
	ErrClientOAuthState        = errors.New("client oauth state invalid")
	ErrClientGoogleFailure     = errors.New("client google identity could not be verified")
	ErrClientGoogleExchange    = errors.New("client google code exchange failed")
	ErrClientGoogleIDToken     = errors.New("client google id token could not be verified")
	ErrClientGoogleNotVerified = errors.New("client google email not verified")
)

// ClientAuthService owns the Google login lifecycle and client session tokens.
// It never exposes the GOOGLE_CLIENT_SECRET, raw OAuth responses, or Google
// access/refresh tokens to handlers or callers.
type ClientAuthService struct {
	repository   repository.ClientAuthRepository
	oauth        *oauth2.Config
	sessionTTL   time.Duration
	stateTTL     time.Duration
	now          func() time.Time
	idTokenParse func(ctx context.Context, raw string) (VerifiedGoogleToken, error)
	exchangeCode func(ctx context.Context, code string) (string, error)
}

// VerifiedGoogleToken is the minimal verified OIDC identity used to anchor a
// client user. Only fields actually consumed are surfaced.
type VerifiedGoogleToken struct {
	Sub           string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
	Issuer        string
	Audience      string
	ExpiresAt     time.Time
}

// NewClientAuthService builds the Google login use-case. When GOOGLE_CLIENT_ID
// is unset, logins fail closed with ErrGoogleNotConfigured.
func NewClientAuthService(authRepository repository.ClientAuthRepository, cfg config.Config) *ClientAuthService {
	ttl := cfg.ClientSessionTTL
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	service := &ClientAuthService{
		repository: authRepository,
		sessionTTL: ttl,
		stateTTL:   10 * time.Minute,
		now:        time.Now,
	}
	if strings.TrimSpace(cfg.GoogleClientID) != "" {
		service.oauth = &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		}
		service.idTokenParse = func(ctx context.Context, raw string) (VerifiedGoogleToken, error) {
			return parseGoogleIDToken(ctx, raw, cfg.GoogleClientID)
		}
		service.exchangeCode = func(ctx context.Context, code string) (string, error) {
			token, err := service.oauth.Exchange(ctx, code)
			if err != nil {
				return "", err
			}
			raw, _ := token.Extra("id_token").(string)
			return raw, nil
		}
	}
	return service
}

// StartLogin returns the Google authorization URL for the client browser and a
// one-time short-lived `state` value the handler stores in an HttpOnly cookie.
func (s *ClientAuthService) StartLogin() (authURL, state string, err error) {
	if s.oauth == nil {
		return "", "", ErrGoogleNotConfigured
	}
	state, err = randomOAuthState()
	if err != nil {
		return "", "", fmt.Errorf("generate client oauth state: %w", err)
	}
	authURL = s.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline, oauth2.SetAuthURLParam("prompt", "select_account"))
	return authURL, state, nil
}

// HandleCallback exchanges a Google code, verifies the OIDC identity, upserts
// the client user, and creates a fresh browser session. rawState is the state
// the browser returned; cookieState is the one-time value the handler stored.
func (s *ClientAuthService) HandleCallback(ctx context.Context, code, rawState, cookieState string) (ClientLoginResult, error) {
	if s.oauth == nil {
		return ClientLoginResult{}, ErrGoogleNotConfigured
	}
	if strings.TrimSpace(rawState) == "" || cookieState == "" || subtle.ConstantTimeCompare([]byte(rawState), []byte(cookieState)) != 1 {
		return ClientLoginResult{}, ErrClientOAuthState
	}
	if strings.TrimSpace(code) == "" {
		return ClientLoginResult{}, ErrClientOAuthState
	}
	if s.exchangeCode == nil {
		return ClientLoginResult{}, ErrGoogleNotConfigured
	}
	rawIDToken, err := s.exchangeCode(ctx, strings.TrimSpace(code))
	if err != nil {
		return ClientLoginResult{}, ErrClientGoogleExchange
	}
	if rawIDToken == "" {
		return ClientLoginResult{}, ErrClientGoogleExchange
	}
	if s.idTokenParse == nil {
		return ClientLoginResult{}, ErrGoogleNotConfigured
	}
	verified, err := s.idTokenParse(ctx, rawIDToken)
	if err != nil {
		return ClientLoginResult{}, ErrClientGoogleIDToken
	}
	if !verified.EmailVerified {
		return ClientLoginResult{}, ErrClientGoogleNotVerified
	}

	user := model.ClientUser{
		Email:       strings.ToLower(strings.TrimSpace(verified.Email)),
		DisplayName: strings.TrimSpace(verified.Name),
	}
	if verified.Email == "" || user.DisplayName == "" {
		return ClientLoginResult{}, ErrClientGoogleIDToken
	}
	picture := strings.TrimSpace(verified.Picture)
	if picture != "" {
		user.AvatarURL = &picture
	}
	identity := model.ClientGoogleIdentity{
		GoogleSub:   verified.Sub,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
	}

	created, err := s.repository.CreateClientUserAndGoogleIdentity(ctx, user, identity)
	if err != nil {
		return ClientLoginResult{}, fmt.Errorf("upsert client user: %w", ErrClientAuthStorage)
	}

	now := s.now()
	sessionToken, err := randomToken()
	if err != nil {
		return ClientLoginResult{}, fmt.Errorf("generate client session token: %w", err)
	}
	csrfToken, err := randomToken()
	if err != nil {
		return ClientLoginResult{}, fmt.Errorf("generate client csrf token: %w", err)
	}
	session := model.ClientSession{
		ID:            uuid.New(),
		ClientUserID:  created.ID,
		TokenHash:     hashToken(sessionToken),
		CSRFTokenHash: hashToken(csrfToken),
		ExpiresAt:     now.Add(s.sessionTTL),
		LastSeenAt:    now,
		CreatedAt:     now,
	}
	if err := s.repository.CreateClientSession(ctx, session); err != nil {
		return ClientLoginResult{}, fmt.Errorf("create client session: %w", ErrClientAuthStorage)
	}
	_ = s.repository.TouchClientLastLogin(ctx, created.ID)
	created.LastLoginAt = now

	return ClientLoginResult{User: created, SessionToken: sessionToken, CSRFToken: csrfToken, ExpiresAt: session.ExpiresAt}, nil
}

// Authenticate resolves a raw client cookie token to a live session and user.
// The token is hashed before the repository sees it and fails closed on
// missing/revoked/expired sessions.
func (s *ClientAuthService) Authenticate(ctx context.Context, rawToken string) (model.ClientSession, error) {
	if strings.TrimSpace(rawToken) == "" {
		return model.ClientSession{}, ErrClientSessionMissing
	}
	session, err := s.repository.FindClientSessionByTokenHash(ctx, hashToken(rawToken))
	if err != nil {
		if errors.Is(err, repository.ErrClientSessionNotFound) {
			return model.ClientSession{}, ErrClientSessionMissing
		}
		return model.ClientSession{}, fmt.Errorf("find client session: %w", ErrClientAuthStorage)
	}
	now := s.now()
	if session.RevokedAt != nil {
		return model.ClientSession{}, ErrClientSessionRevoked
	}
	if !session.ExpiresAt.After(now) {
		return model.ClientSession{}, ErrClientSessionExpired
	}
	_ = s.repository.TouchClientSession(ctx, session.ID)
	return session, nil
}

// ValidateCSRF compares a presented header token against the digest bound to
// the client session in constant time.
func (s *ClientAuthService) ValidateCSRF(session model.ClientSession, rawToken string) bool {
	presented := hashToken(strings.TrimSpace(rawToken))
	return subtle.ConstantTimeCompare(presented, session.CSRFTokenHash) == 1
}

// RefreshCSRF migrates a session that has no usable client CSRF cookie. The
// deterministic, role-separated derivation makes concurrent legacy refreshes
// converge on one hash instead of invalidating a neighboring browser tab.
func (s *ClientAuthService) RefreshCSRF(ctx context.Context, session model.ClientSession) (string, error) {
	if err := s.ensureLiveSession(session); err != nil {
		return "", err
	}
	rawToken := deriveSessionCSRFToken(clientLegacyCSRFPurpose, session.ID, session.TokenHash)
	if err := s.repository.RotateClientCSRF(ctx, session.ID, hashToken(rawToken)); err != nil {
		return "", fmt.Errorf("rotate client csrf token: %w", ErrClientAuthStorage)
	}
	return rawToken, nil
}

// Logout revokes a client session server-side.
func (s *ClientAuthService) Logout(ctx context.Context, session model.ClientSession) error {
	if err := s.repository.RevokeClientSession(ctx, session.ID); err != nil {
		return fmt.Errorf("revoke client session: %w", ErrClientAuthStorage)
	}
	return nil
}

func (s *ClientAuthService) ensureLiveSession(session model.ClientSession) error {
	now := s.now()
	if session.RevokedAt != nil {
		return ErrClientSessionRevoked
	}
	if !session.ExpiresAt.After(now) {
		return ErrClientSessionExpired
	}
	return nil
}

// ClientLoginResult is the bridging point where a browser token exists in
// memory; the handler places SessionToken into the HttpOnly cookie.
type ClientLoginResult struct {
	User         model.ClientUser
	SessionToken string
	CSRFToken    string
	ExpiresAt    time.Time
}

func parseGoogleIDToken(ctx context.Context, raw string, audience string) (VerifiedGoogleToken, error) {
	payload, err := idtoken.Validate(ctx, raw, audience)
	if err != nil {
		return VerifiedGoogleToken{}, err
	}
	email, _ := payload.Claims["email"].(string)
	emailVerified, _ := payload.Claims["email_verified"].(bool)
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)
	return VerifiedGoogleToken{
		Sub:           payload.Subject,
		Email:         email,
		EmailVerified: emailVerified,
		Name:          name,
		Picture:       picture,
		Issuer:        payload.Issuer,
		Audience:      payload.Audience,
		ExpiresAt:     time.Unix(payload.Expires, 0),
	}, nil
}

func randomOAuthState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
