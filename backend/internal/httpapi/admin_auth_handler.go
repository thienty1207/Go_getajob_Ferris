package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/service"
)

const adminCSRFCookieName = "ferris_admin_csrf"

type adminAuthHandlerAPI interface {
	adminSessionAPI
	Login(context.Context, string, string) (service.LoginResult, error)
	RefreshCSRF(context.Context, model.AdminSession) (string, error)
	Logout(context.Context, model.AdminSession) error
	RecordAudit(context.Context, model.AdminAuditEvent) error
}

// AdminAuthHandler owns JSON/cookie translation for the admin session. The
// auth service remains responsible for hashing, expiry, and database state.
type AdminAuthHandler struct {
	auth         adminAuthHandlerAPI
	cookieName   string
	cookieTTL    time.Duration
	cookieSecure bool
	logger       *slog.Logger
}

func NewAdminAuthHandler(auth adminAuthHandlerAPI, cfg config.Config) *AdminAuthHandler {
	ttl := cfg.AdminSessionTTL
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	cookieName := strings.TrimSpace(cfg.AdminCookieName)
	if cookieName == "" {
		cookieName = "ferris_admin_session"
	}
	return &AdminAuthHandler{
		auth:         auth,
		cookieName:   cookieName,
		cookieTTL:    ttl,
		cookieSecure: cfg.AdminCookieSecure,
		logger:       slog.Default(),
	}
}

type adminLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type adminAuthResponse struct {
	Admin     adminResponse `json:"admin"`
	CSRFToken string        `json:"csrf_token"`
}

type adminResponse struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	IsActive    bool       `json:"is_active"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// Login validates an intentionally small JSON body and creates an HttpOnly
// session cookie. The CSRF token is returned in JSON because it is meant to be
// held by the same-origin admin JavaScript and sent as a custom header.
func (h *AdminAuthHandler) Login(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	var request adminLoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_admin_login", "Vui lòng nhập đúng email và mật khẩu.")
		return
	}
	result, err := h.auth.Login(c.Request.Context(), request.Email, request.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidAdminCredentials) {
			writeError(c, http.StatusUnauthorized, "admin_invalid_credentials", "Email hoặc mật khẩu không đúng.")
			return
		}
		h.logger.Error("admin login failed", "action", "admin_login", "result", "error", "error_code", "internal_error")
		writeAdminAuthError(c, err)
		return
	}
	h.setSessionCookie(c, result.SessionToken, result.Session.ExpiresAt)
	h.setCSRFCookie(c, result.CSRFToken, result.Session.ExpiresAt)
	h.recordAudit(c, model.AdminAuditEvent{AdminUserID: &result.User.ID, Action: "admin_login", Result: "SUCCESS"})
	c.JSON(http.StatusOK, adminAuthResponse{Admin: mapAdminUser(result.User), CSRFToken: result.CSRFToken})
}

// Me recovers the session-bound CSRF token from a separate HttpOnly cookie.
// The cookie is shared by tabs, while validation against the server-side hash
// prevents it from becoming a second authentication credential.
func (h *AdminAuthHandler) Me(c *gin.Context) {
	session, ok := AdminSessionFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "admin_auth_required", "Vui lòng đăng nhập tài khoản quản trị.")
		return
	}
	csrfToken, err := h.csrfToken(c, session)
	if err != nil {
		writeAdminAuthError(c, err)
		return
	}
	c.JSON(http.StatusOK, adminAuthResponse{Admin: mapAdminUser(session.User), CSRFToken: csrfToken})
}

// Logout requires CSRF because it is a state-changing cookie-authenticated
// endpoint. Revocation is idempotent, then the browser cookie is cleared.
func (h *AdminAuthHandler) Logout(c *gin.Context) {
	session, ok := AdminSessionFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "admin_auth_required", "Vui lòng đăng nhập tài khoản quản trị.")
		return
	}
	if err := h.auth.Logout(c.Request.Context(), session); err != nil {
		h.logger.Error("admin logout failed", "action", "admin_logout", "result", "error", "error_code", "internal_error")
		writeAdminAuthError(c, err)
		return
	}
	h.clearSessionCookie(c)
	h.clearCSRFCookie(c)
	h.recordAudit(c, model.AdminAuditEvent{AdminUserID: &session.User.ID, Action: "admin_logout", Result: "SUCCESS"})
	c.Status(http.StatusNoContent)
}

func (h *AdminAuthHandler) csrfToken(c *gin.Context, session model.AdminSession) (string, error) {
	if cookieToken, err := c.Cookie(adminCSRFCookieName); err == nil {
		cookieToken = strings.TrimSpace(cookieToken)
		if cookieToken != "" && h.auth.ValidateCSRF(session, cookieToken) {
			return cookieToken, nil
		}
	}

	// Sessions created before the CSRF cookie existed need one final rotation.
	// Once set, later tabs reuse this cookie instead of replacing the sole hash.
	csrfToken, err := h.auth.RefreshCSRF(c.Request.Context(), session)
	if err != nil {
		return "", err
	}
	h.setCSRFCookie(c, csrfToken, session.ExpiresAt)
	return csrfToken, nil
}

func (h *AdminAuthHandler) setSessionCookie(c *gin.Context, value string, expiresAt time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cookieName,
		Value:    value,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   maxAgeSeconds(time.Until(expiresAt)),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AdminAuthHandler) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AdminAuthHandler) setCSRFCookie(c *gin.Context, value string, expiresAt time.Time) {
	h.setBoundedAuthCookie(c, adminCSRFCookieName, value, expiresAt)
}

func (h *AdminAuthHandler) clearCSRFCookie(c *gin.Context) {
	h.clearAuthCookie(c, adminCSRFCookieName)
}

func (h *AdminAuthHandler) setBoundedAuthCookie(c *gin.Context, name, value string, expiresAt time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   maxAgeSeconds(time.Until(expiresAt)),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AdminAuthHandler) clearAuthCookie(c *gin.Context, name string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AdminAuthHandler) recordAudit(c *gin.Context, event model.AdminAuditEvent) {
	if requestID := strings.TrimSpace(c.GetHeader("X-Request-ID")); requestID != "" && len(requestID) <= 120 {
		event.RequestID = &requestID
	}
	if err := h.auth.RecordAudit(c.Request.Context(), event); err != nil {
		h.logger.Warn("admin audit event failed", "action", event.Action, "result", "error", "error_code", "audit_write_failed")
	}
}

func mapAdminUser(user model.AdminUser) adminResponse {
	return adminResponse{ID: user.ID.String(), Email: user.Email, IsActive: user.IsActive, LastLoginAt: user.LastLoginAt}
}

func maxAgeSeconds(duration time.Duration) int {
	seconds := int(duration / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}
