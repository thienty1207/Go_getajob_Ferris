package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/service"
)

const adminSessionContextKey = "ferris.admin.session"

type adminSessionAPI interface {
	Authenticate(context.Context, string) (model.AdminSession, error)
	ValidateCSRF(model.AdminSession, string) bool
}

// RequireAdminSession turns the HttpOnly cookie into a server-side identity.
// A hidden admin route or frontend state flag is never treated as authorization.
func RequireAdminSession(auth adminSessionAPI, cfg config.Config) gin.HandlerFunc {
	cookieName := cfg.AdminCookieName
	if cookieName == "" {
		cookieName = "ferris_admin_session"
	}
	return func(c *gin.Context) {
		rawToken, err := c.Cookie(cookieName)
		if err != nil || rawToken == "" {
			writeError(c, http.StatusUnauthorized, "admin_auth_required", "Vui lòng đăng nhập tài khoản quản trị.")
			return
		}
		session, err := auth.Authenticate(c.Request.Context(), rawToken)
		if err != nil {
			writeAdminAuthError(c, err)
			return
		}
		c.Set(adminSessionContextKey, session)
		c.Next()
	}
}

// RequireAdminCSRF protects state-changing cookie-authenticated operations.
// A cross-site form can carry a cookie but cannot set this custom header.
func RequireAdminCSRF(auth adminSessionAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, ok := AdminSessionFromContext(c)
		if !ok || !auth.ValidateCSRF(session, c.GetHeader("X-CSRF-Token")) {
			writeError(c, http.StatusForbidden, "admin_csrf_invalid", "Phiên quản trị không hợp lệ cho thao tác này.")
			return
		}
		c.Next()
	}
}

// AdminSessionFromContext is shared by auth logout, promotion, and job
// handlers. Keeping one typed accessor prevents accidental context-key drift.
func AdminSessionFromContext(c *gin.Context) (model.AdminSession, bool) {
	value, exists := c.Get(adminSessionContextKey)
	if !exists {
		return model.AdminSession{}, false
	}
	session, ok := value.(model.AdminSession)
	return session, ok
}

func writeAdminAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrAdminSessionMissing), errors.Is(err, service.ErrAdminSessionExpired), errors.Is(err, service.ErrAdminSessionRevoked), errors.Is(err, service.ErrAdminInactive):
		writeError(c, http.StatusUnauthorized, "admin_auth_required", "Phiên quản trị không còn hợp lệ. Vui lòng đăng nhập lại.")
	case errors.Is(err, service.ErrAdminAuthStorage):
		writeError(c, http.StatusServiceUnavailable, "database_unavailable", "Database chưa sẵn sàng. Vui lòng thử lại sau.")
	default:
		writeError(c, http.StatusInternalServerError, "internal_error", "Service tạm thời không khả dụng.")
	}
}
