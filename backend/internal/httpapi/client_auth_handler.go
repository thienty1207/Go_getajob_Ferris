package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/service"
)

const (
	clientOAuthStateCookie = "ferris_client_oauth_state"
	clientCSRFCookieName   = "ferris_client_csrf"
)

type clientAuthHandlerAPI interface {
	clientSessionAPI
	StartLogin() (string, string, error)
	HandleCallback(context.Context, string, string, string) (service.ClientLoginResult, error)
	Authenticate(context.Context, string) (model.ClientSession, error)
	RefreshCSRF(context.Context, model.ClientSession) (string, error)
	Logout(context.Context, model.ClientSession) error
}

// ClientAuthHandler owns JSON/cookie translation for the client Google session,
// kept fully separate from the admin auth handler and cookie.
type ClientAuthHandler struct {
	auth         clientAuthHandlerAPI
	cookieName   string
	cookieTTL    time.Duration
	cookieSecure bool
	clientOrigin string
	loginPath    string
	logger       *slog.Logger
}

func NewClientAuthHandler(auth clientAuthHandlerAPI, cfg config.Config) *ClientAuthHandler {
	ttl := cfg.ClientSessionTTL
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	cookieName := strings.TrimSpace(cfg.ClientCookieName)
	if cookieName == "" {
		cookieName = "ferris_client_session"
	}
	origin := frontendHost(cfg)
	return &ClientAuthHandler{
		auth:         auth,
		cookieName:   cookieName,
		cookieTTL:    ttl,
		cookieSecure: cfg.ClientCookieSecure,
		clientOrigin: origin + "/client",
		loginPath:    origin + "/client/login",
		logger:       slog.Default(),
	}
}

type clientAuthResponse struct {
	User      clientUserResponse `json:"user"`
	CSRFToken string             `json:"csrf_token"`
}

type clientUserResponse struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	Provider    string    `json:"provider"`
	CreatedAt   time.Time `json:"created_at"`
	LastLoginAt time.Time `json:"last_login_at"`
}

// Start routes the browser to Google. The one-time state is mirrored in an
// HttpOnly SameSite=Lax cookie; nothing sensitive is logged.
func (h *ClientAuthHandler) Start(c *gin.Context) {
	authURL, state, err := h.auth.StartLogin()
	if err != nil {
		if errors.Is(err, service.ErrGoogleNotConfigured) {
			h.logger.Error("client google login not configured", "action", "client_google_start", "error_code", "google_not_configured")
			writeError(c, http.StatusServiceUnavailable, "google_not_configured", "Đăng nhập Google chưa được cấu hình.")
			return
		}
		h.logger.Error("client google start failed", "action", "client_google_start", "error_code", "internal_error")
		writeError(c, http.StatusInternalServerError, "internal_error", "Service tạm thời không khả dụng.")
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     clientOAuthStateCookie,
		Value:    state,
		Path:     "/",
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure,
	})
	c.Redirect(http.StatusFound, authURL)
}

// Callback verifies state, exchanges the code, and creates a client session.
// On failure the browser is redirected to /client/login?error=<code>.
func (h *ClientAuthHandler) Callback(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	cookieState, stateErr := c.Cookie(clientOAuthStateCookie)
	rawState := strings.TrimSpace(c.Query("state"))
	if code == "" || stateErr != nil || rawState == "" {
		// OAuth state is one-time data. Clear it even when Google returns an
		// incomplete callback so a stale browser cookie cannot be reused.
		h.clearOAuthState(c)
		h.redirectLogin(c, "state_error")
		return
	}
	result, err := h.auth.HandleCallback(c.Request.Context(), code, rawState, cookieState)
	if err != nil {
		errorCode := oauthErrorCode(err)
		h.clearOAuthState(c)
		h.logger.Error("client google callback failed", "action", "client_google_callback", "error_code", errorCode)
		h.redirectLogin(c, errorCode)
		return
	}
	h.clearOAuthState(c)
	h.setClientCookie(c, result.SessionToken, result.ExpiresAt)
	h.setCSRFCookie(c, result.CSRFToken, result.ExpiresAt)
	// No client-supplied redirect is trusted: always land on the client entry
	// path to avoid any open-redirect surface.
	c.Redirect(http.StatusFound, h.clientOrigin)
}

// Me returns the real user and the CSRF token shared by this browser's tabs.
// The token is useful only with the independently authenticated client session.
func (h *ClientAuthHandler) Me(c *gin.Context) {
	session, ok := ClientSessionFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "client_unauthorized", "Vui lòng đăng nhập.")
		return
	}
	csrfToken, err := h.csrfToken(c, session)
	if err != nil {
		writeClientAuthError(c, err)
		return
	}
	c.JSON(http.StatusOK, clientAuthResponse{User: mapClientUser(session.User), CSRFToken: csrfToken})
}

// Logout requires CSRF and revokes the client session server-side.
func (h *ClientAuthHandler) Logout(c *gin.Context) {
	session, ok := ClientSessionFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "client_unauthorized", "Vui lòng đăng nhập.")
		return
	}
	if err := h.auth.Logout(c.Request.Context(), session); err != nil {
		h.logger.Error("client logout failed", "action", "client_logout", "error_code", "internal_error")
		writeError(c, http.StatusInternalServerError, "internal_error", "Service tạm thời không khả dụng.")
		return
	}
	h.clearClientCookie(c)
	h.clearCSRFCookie(c)
	c.Status(http.StatusNoContent)
}

func (h *ClientAuthHandler) csrfToken(c *gin.Context, session model.ClientSession) (string, error) {
	if cookieToken, err := c.Cookie(clientCSRFCookieName); err == nil {
		cookieToken = strings.TrimSpace(cookieToken)
		if cookieToken != "" && h.auth.ValidateCSRF(session, cookieToken) {
			return cookieToken, nil
		}
	}

	// A missing or stale cookie identifies a pre-migration session. Rotate once,
	// then persist the raw token only in this HttpOnly browser cookie.
	csrfToken, err := h.auth.RefreshCSRF(c.Request.Context(), session)
	if err != nil {
		return "", err
	}
	h.setCSRFCookie(c, csrfToken, session.ExpiresAt)
	return csrfToken, nil
}

func mapClientUser(user model.ClientUser) clientUserResponse {
	return clientUserResponse{
		ID:          user.ID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Provider:    user.Provider,
		CreatedAt:   user.CreatedAt,
		LastLoginAt: user.LastLoginAt,
	}
}

func (h *ClientAuthHandler) setClientCookie(c *gin.Context, value string, expiresAt time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cookieName,
		Value:    value,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure,
	})
}

func (h *ClientAuthHandler) clearClientCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure,
	})
}

func (h *ClientAuthHandler) setCSRFCookie(c *gin.Context, value string, expiresAt time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     clientCSRFCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   maxAgeSeconds(time.Until(expiresAt)),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure,
	})
}

func (h *ClientAuthHandler) clearCSRFCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     clientCSRFCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure,
	})
}

func (h *ClientAuthHandler) clearOAuthState(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     clientOAuthStateCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure,
	})
}

func (h *ClientAuthHandler) redirectLogin(c *gin.Context, code string) {
	c.Redirect(http.StatusFound, h.loginPath+"?error="+code)
}

// oauthErrorCode maps a callback failure to a stable, user-safe error code that
// is safe to place in the redirect URL (no raw Google error or token is ever
// included).
func oauthErrorCode(err error) string {
	switch {
	case errors.Is(err, service.ErrClientOAuthState):
		return "state_error"
	case errors.Is(err, service.ErrClientGoogleExchange):
		return "token_exchange_error"
	case errors.Is(err, service.ErrClientGoogleIDToken):
		return "id_token_error"
	case errors.Is(err, service.ErrClientGoogleNotVerified):
		return "email_not_verified"
	default:
		return "oauth_error"
	}
}

// frontendHost returns the frontend origin (scheme + host, no path) the OAuth
// callback redirects the browser to (e.g. http://127.0.0.1:5173). It prefers the
// explicit CLIENT_REDIRECT_ORIGIN when set, otherwise derives one from the CORS
// allowlist, and only then falls back to the loopback default. It is never
// sourced from user/browser input, so it cannot be an open-redirect vector, and
// it always returns an absolute origin so the browser is never redirected back
// to the API's own host (which would 404 the frontend routes).
func frontendHost(cfg config.Config) string {
	if origin := strings.TrimRight(strings.TrimSpace(cfg.ClientRedirectOrigin), "/"); origin != "" && validFrontendOrigin(origin, cfg.Environment) {
		return origin
	}
	for _, origin := range cfg.AllowedOrigins {
		o := strings.TrimRight(origin, "/")
		if strings.Contains(o, ":5173") && strings.Contains(o, "127.0.0.1") && validFrontendOrigin(o, cfg.Environment) {
			return o
		}
	}
	for _, origin := range cfg.AllowedOrigins {
		o := strings.TrimRight(origin, "/")
		if strings.Contains(o, ":5173") && validFrontendOrigin(o, cfg.Environment) {
			return o
		}
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Environment), "production") {
		return "https://127.0.0.1:5173"
	}
	return "http://127.0.0.1:5173"
}

// validFrontendOrigin accepts an absolute http/https origin with a non-empty
// host that contains at least one dot or is a literal loopback host. In
// production, only https is allowed so the post-login redirect never transits
// (or points to) plaintext.
func validFrontendOrigin(origin, environment string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return false
	}
	// Reject host-only-with-port edge cases such as "http://:5173".
	hostname := parsed.Hostname()
	if hostname == "" {
		return false
	}
	if !strings.Contains(hostname, ".") && hostname != "localhost" && !isLoopbackHost(hostname) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(environment), "production") {
		return parsed.Scheme == "https"
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func isLoopbackHost(host string) bool {
	lower := strings.ToLower(host)
	return lower == "localhost" || strings.HasPrefix(lower, "127.") || lower == "::1"
}

func writeClientAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrClientSessionMissing), errors.Is(err, service.ErrClientSessionExpired), errors.Is(err, service.ErrClientSessionRevoked):
		writeError(c, http.StatusUnauthorized, "client_unauthorized", "Phiên đăng nhập không còn hợp lệ. Vui lòng đăng nhập lại.")
	default:
		writeError(c, http.StatusInternalServerError, "internal_error", "Service tạm thời không khả dụng.")
	}
}
