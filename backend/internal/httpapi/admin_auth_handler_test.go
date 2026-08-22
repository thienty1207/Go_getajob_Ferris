package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/service"
)

type rotatingAdminCSRFAuth struct {
	*testAdminAuth
	csrfToken    string
	refreshCalls int
}

func newRotatingAdminCSRFAuth() *rotatingAdminCSRFAuth {
	return &rotatingAdminCSRFAuth{testAdminAuth: newTestAdminAuth(), csrfToken: "csrf-token"}
}

func (auth *rotatingAdminCSRFAuth) ValidateCSRF(_ model.AdminSession, token string) bool {
	return token == auth.csrfToken
}

func (auth *rotatingAdminCSRFAuth) RefreshCSRF(context.Context, model.AdminSession) (string, error) {
	auth.refreshCalls++
	auth.csrfToken = fmt.Sprintf("legacy-admin-csrf-%d", auth.refreshCalls)
	return auth.csrfToken, nil
}

func TestAdminLoginSetsHttpOnlySessionCookieAndReturnsCSRF(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	cfg.AdminCookieSecure = true
	auth := newTestAdminAuth()
	promotionAPI := &testPromotionAPI{}
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(promotionAPI, cfg), NewAdminAuthHandler(auth, cfg), nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"correct horse battery"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	sessionCookie := responseCookie(t, response.Result(), "ferris_admin_session")
	assertAuthCookie(t, sessionCookie, "session-token", true)
	csrfCookie := responseCookie(t, response.Result(), "ferris_admin_csrf")
	assertAuthCookie(t, csrfCookie, "csrf-token", true)
	if cookieByName(response.Result(), "ferris_client_csrf") != nil {
		t.Fatal("admin login set the client CSRF cookie")
	}
	if strings.Contains(response.Body.String(), "session-token") {
		t.Fatal("admin login exposed the HttpOnly session token in JSON")
	}
	var payload struct {
		Admin struct {
			Email string `json:"email"`
		} `json:"admin"`
		CSRFToken string `json:"csrf_token"`
	}
	decodeJSON(t, response, &payload)
	if payload.Admin.Email != "admin@example.com" || payload.CSRFToken == "" {
		t.Fatalf("payload = %#v, want admin and csrf token", payload)
	}
	if payload.CSRFToken != csrfCookie.Value {
		t.Fatalf("csrf response token = %q, cookie = %q, want the same session-bound token", payload.CSRFToken, csrfCookie.Value)
	}
}

func TestAdminMeRequiresSessionAndReusesCSRFCookieAcrossConsecutiveCalls(t *testing.T) {
	t.Parallel()
	cfg := config.Config{AdminCookieName: "ferris_admin_session"}
	auth := newRotatingAdminCSRFAuth()
	promotionAPI := &testPromotionAPI{}
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(promotionAPI, cfg), NewAdminAuthHandler(auth, cfg), nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth/me", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("without cookie status = %d, want 401", response.Code)
	}

	var tokens []string
	for range 2 {
		request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth/me", nil)
		request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
		request.AddCookie(&http.Cookie{Name: "ferris_admin_csrf", Value: "csrf-token"})
		response = httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("with cookies status = %d body=%s, want 200", response.Code, response.Body.String())
		}
		var payload struct {
			CSRFToken string `json:"csrf_token"`
		}
		decodeJSON(t, response, &payload)
		tokens = append(tokens, payload.CSRFToken)
	}
	if tokens[0] != "csrf-token" || tokens[1] != tokens[0] {
		t.Fatalf("consecutive /me tokens = %q, want the same cookie-backed token", tokens)
	}
	if auth.refreshCalls != 0 {
		t.Fatalf("RefreshCSRF calls = %d, want 0 for a valid CSRF cookie", auth.refreshCalls)
	}
}

func TestAdminMeRefreshesLegacySessionOnceAndKeepsTokenValidForProtectedRequest(t *testing.T) {
	t.Parallel()
	cfg := config.Config{AdminCookieName: "ferris_admin_session", AdminCookieSecure: true}
	auth := newRotatingAdminCSRFAuth()
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(&testPromotionAPI{}, cfg), NewAdminAuthHandler(auth, cfg), nil)

	firstRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth/me", nil)
	firstRequest.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	firstResponse := httptest.NewRecorder()
	router.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("legacy /me status = %d body=%s, want 200", firstResponse.Code, firstResponse.Body.String())
	}
	var firstPayload struct {
		CSRFToken string `json:"csrf_token"`
	}
	decodeJSON(t, firstResponse, &firstPayload)
	csrfCookie := responseCookie(t, firstResponse.Result(), "ferris_admin_csrf")
	assertAuthCookie(t, csrfCookie, firstPayload.CSRFToken, true)

	secondRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth/me", nil)
	secondRequest.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	secondRequest.AddCookie(csrfCookie)
	secondResponse := httptest.NewRecorder()
	router.ServeHTTP(secondResponse, secondRequest)
	var secondPayload struct {
		CSRFToken string `json:"csrf_token"`
	}
	decodeJSON(t, secondResponse, &secondPayload)
	if secondResponse.Code != http.StatusOK || secondPayload.CSRFToken != firstPayload.CSRFToken || auth.refreshCalls != 1 {
		t.Fatalf("second /me status=%d token=%q refreshes=%d, want same token %q and one legacy refresh", secondResponse.Code, secondPayload.CSRFToken, auth.refreshCalls, firstPayload.CSRFToken)
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/logout", nil)
	logoutRequest.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	logoutRequest.Header.Set("X-CSRF-Token", firstPayload.CSRFToken)
	logoutResponse := httptest.NewRecorder()
	router.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("protected logout status = %d body=%s, want 204 with token from first /me", logoutResponse.Code, logoutResponse.Body.String())
	}
}

func TestAdminMeReturnsServiceUnavailableWhenSessionStorageIsDown(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	auth := newTestAdminAuth()
	auth.authenticateErr = service.ErrAdminAuthStorage
	promotionAPI := &testPromotionAPI{}
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(promotionAPI, cfg), NewAdminAuthHandler(auth, cfg), nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s, want 503", response.Code, response.Body.String())
	}
	assertErrorResponse(t, response, "database_unavailable", "chưa sẵn sàng")
}

func TestAdminLoginReturnsServiceUnavailableWhenSessionStorageIsDown(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	auth := newTestAdminAuth()
	auth.loginErr = service.ErrAdminAuthStorage
	promotionAPI := &testPromotionAPI{}
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(promotionAPI, cfg), NewAdminAuthHandler(auth, cfg), nil)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"correct horse battery"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s, want 503", response.Code, response.Body.String())
	}
	assertErrorResponse(t, response, "database_unavailable", "chưa sẵn sàng")
}

func TestAdminLogoutRequiresCSRFAndRevokesSession(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	auth := newTestAdminAuth()
	promotionAPI := &testPromotionAPI{}
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(promotionAPI, cfg), NewAdminAuthHandler(auth, cfg), nil)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("without csrf status = %d, want 403", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	request.Header.Set("X-CSRF-Token", "csrf-token")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || auth.logoutCalls != 1 {
		t.Fatalf("logout status = %d calls=%d, want 204/1", response.Code, auth.logoutCalls)
	}
	if cookie := responseCookie(t, response.Result(), "ferris_admin_session"); cookie.MaxAge >= 0 || cookie.Value != "" {
		t.Fatalf("session clearing cookie = %#v, want empty MaxAge=-1", cookie)
	}
	if cookie := responseCookie(t, response.Result(), "ferris_admin_csrf"); cookie.MaxAge >= 0 || cookie.Value != "" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("csrf clearing cookie = %#v, want empty HttpOnly SameSite=Lax MaxAge=-1", cookie)
	}
}

func TestAdminLogoutReturnsServiceUnavailableWhenSessionStorageIsDown(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	auth := newTestAdminAuth()
	auth.logoutErr = service.ErrAdminAuthStorage
	promotionAPI := &testPromotionAPI{}
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(promotionAPI, cfg), NewAdminAuthHandler(auth, cfg), nil)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	request.Header.Set("X-CSRF-Token", "csrf-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s, want 503", response.Code, response.Body.String())
	}
	assertErrorResponse(t, response, "database_unavailable", "chưa sẵn sàng")
}

func cookieByName(response *http.Response, name string) *http.Cookie {
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func responseCookie(t *testing.T, response *http.Response, name string) *http.Cookie {
	t.Helper()
	cookie := cookieByName(response, name)
	if cookie == nil {
		t.Fatalf("response did not set cookie %q; cookies=%#v", name, response.Cookies())
	}
	return cookie
}

func assertAuthCookie(t *testing.T, cookie *http.Cookie, value string, secure bool) {
	t.Helper()
	if cookie.Value != value || cookie.Path != "/" || !cookie.HttpOnly || cookie.Secure != secure || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge <= 0 || cookie.Expires.IsZero() {
		t.Fatalf("auth cookie = %#v, want value match, Path=/, HttpOnly, Secure=%v, SameSite=Lax, and bounded lifetime", cookie, secure)
	}
}
