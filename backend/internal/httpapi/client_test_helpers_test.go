package httpapi

import (
	"context"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/service"
	"github.com/google/uuid"
)

type testClientAuth struct {
	session       model.ClientSession
	logoutCalls   int
	startAuthURL  string
	startState    string
	callbackLogin service.ClientLoginResult
}

func newTestClientAuth() *testClientAuth {
	user := model.ClientUser{ID: uuid.New(), Email: "user@example.com", DisplayName: "Example User", Provider: "google"}
	return &testClientAuth{
		session: model.ClientSession{
			ID:            uuid.New(),
			ClientUserID:  user.ID,
			ExpiresAt:     time.Now().Add(time.Hour),
			User:          user,
		},
		startAuthURL: "https://accounts.google.com/o/oauth2/v2/auth?example=redirect",
		startState:   "oauth-state-token",
		callbackLogin: service.ClientLoginResult{
			User:         model.ClientUser{ID: uuid.New(), Email: "user@example.com", DisplayName: "Example User", Provider: "google"},
			SessionToken: "client-session-token",
			CSRFToken:    "client-csrf-token",
			ExpiresAt:    time.Now().Add(time.Hour),
		},
	}
}

func (auth *testClientAuth) StartLogin() (string, string, error) {
	return auth.startAuthURL, auth.startState, nil
}

func (auth *testClientAuth) HandleCallback(_ context.Context, code, rawState, cookieState string) (service.ClientLoginResult, error) {
	if code == "" || rawState == "" || rawState != cookieState || cookieState != auth.startState {
		return service.ClientLoginResult{}, service.ErrClientOAuthState
	}
	return auth.callbackLogin, nil
}

func (auth *testClientAuth) Authenticate(_ context.Context, token string) (model.ClientSession, error) {
	if token != "client-session-token" {
		return model.ClientSession{}, service.ErrClientSessionMissing
	}
	return auth.session, nil
}

func (auth *testClientAuth) ValidateCSRF(_ model.ClientSession, token string) bool {
	return token == "client-csrf-token"
}

func (auth *testClientAuth) RefreshCSRF(context.Context, model.ClientSession) (string, error) {
	return "client-csrf-token", nil
}

func (auth *testClientAuth) Logout(context.Context, model.ClientSession) error {
	auth.logoutCalls++
	return nil
}
