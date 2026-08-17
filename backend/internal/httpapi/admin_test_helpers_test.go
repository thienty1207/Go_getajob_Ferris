package httpapi

import (
	"context"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/service"
	"github.com/google/uuid"
)

type testAdminAuth struct {
	session     model.AdminSession
	logoutCalls int
	auditCalls  int
}

func newTestAdminAuth() *testAdminAuth {
	user := model.AdminUser{ID: uuid.New(), Email: "admin@example.com", IsActive: true}
	return &testAdminAuth{session: model.AdminSession{
		ID:          uuid.New(),
		AdminUserID: user.ID,
		ExpiresAt:   time.Now().Add(time.Hour),
		User:        user,
	}}
}

func (auth *testAdminAuth) Login(context.Context, string, string) (service.LoginResult, error) {
	return service.LoginResult{User: auth.session.User, Session: auth.session, SessionToken: "session-token", CSRFToken: "csrf-token"}, nil
}

func (auth *testAdminAuth) Authenticate(_ context.Context, token string) (model.AdminSession, error) {
	if token != "session-token" {
		return model.AdminSession{}, service.ErrAdminSessionMissing
	}
	return auth.session, nil
}

func (auth *testAdminAuth) ValidateCSRF(_ model.AdminSession, token string) bool {
	return token == "csrf-token"
}

func (auth *testAdminAuth) RefreshCSRF(context.Context, model.AdminSession) (string, error) {
	return "csrf-token", nil
}

func (auth *testAdminAuth) Logout(context.Context, model.AdminSession) error {
	auth.logoutCalls++
	return nil
}

func (auth *testAdminAuth) RecordAudit(context.Context, model.AdminAuditEvent) error {
	auth.auditCalls++
	return nil
}
