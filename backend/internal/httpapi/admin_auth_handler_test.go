package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogetsomefoodferris/backend/internal/config"
)

func TestAdminLoginSetsHttpOnlySessionCookieAndReturnsCSRF(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
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
	if cookie := response.Result().Cookies()[0]; cookie.Name != "ferris_admin_session" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie = %#v, want HttpOnly SameSite=Lax", cookie)
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
}

func TestAdminMeRequiresSessionAndRotatesCSRF(t *testing.T) {
	t.Parallel()
	cfg := config.Config{AdminCookieName: "ferris_admin_session"}
	auth := newTestAdminAuth()
	promotionAPI := &testPromotionAPI{}
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(promotionAPI, cfg), NewAdminAuthHandler(auth, cfg), nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth/me", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("without cookie status = %d, want 401", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "csrf-token") {
		t.Fatalf("with cookie status = %d body=%s, want refreshed csrf", response.Code, response.Body.String())
	}
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
}
