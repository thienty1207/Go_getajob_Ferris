package httpapi

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
)

const clientSessionContextKey = "ferris.client.session"

type clientSessionAPI interface {
	Authenticate(context.Context, string) (model.ClientSession, error)
	ValidateCSRF(model.ClientSession, string) bool
}

// RequireClientSession turns the HttpOnly client cookie into a server-side
// identity. A client session is never accepted as an admin session.
func RequireClientSession(auth clientSessionAPI, cfg config.Config) gin.HandlerFunc {
	cookieName := cfg.ClientCookieName
	if cookieName == "" {
		cookieName = "ferris_client_session"
	}
	return func(c *gin.Context) {
		rawToken, err := c.Cookie(cookieName)
		if err != nil || rawToken == "" {
			writeError(c, http.StatusUnauthorized, "client_unauthorized", "Vui lòng đăng nhập.")
			return
		}
		session, err := auth.Authenticate(c.Request.Context(), rawToken)
		if err != nil {
			writeClientAuthError(c, err)
			return
		}
		c.Set(clientSessionContextKey, session)
		c.Next()
	}
}

// RequireClientCSRF protects state-changing cookie-authenticated client
// operations. A cross-site form can carry the cookie but cannot set the
// custom header, mirroring the admin mutation pattern.
func RequireClientCSRF(auth clientSessionAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, ok := ClientSessionFromContext(c)
		if !ok || !auth.ValidateCSRF(session, c.GetHeader("X-CSRF-Token")) {
			writeError(c, http.StatusForbidden, "client_csrf_invalid", "Phiên đăng nhập không hợp lệ cho thao tác này.")
			return
		}
		c.Next()
	}
}

// ClientSessionFromContext is the single typed accessor for the client session
// set by RequireClientSession.
func ClientSessionFromContext(c *gin.Context) (model.ClientSession, bool) {
	value, exists := c.Get(clientSessionContextKey)
	if !exists {
		return model.ClientSession{}, false
	}
	session, ok := value.(model.ClientSession)
	return session, ok
}
