package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
)

type rotatingClientCSRFAuth struct {
	*testClientAuth
	csrfToken    string
	refreshCalls int
}

func newRotatingClientCSRFAuth() *rotatingClientCSRFAuth {
	base := newTestClientAuth()
	return &rotatingClientCSRFAuth{testClientAuth: base, csrfToken: base.callbackLogin.CSRFToken}
}

func (auth *rotatingClientCSRFAuth) ValidateCSRF(_ model.ClientSession, token string) bool {
	return token == auth.csrfToken
}

func (auth *rotatingClientCSRFAuth) RefreshCSRF(context.Context, model.ClientSession) (string, error) {
	auth.refreshCalls++
	auth.csrfToken = fmt.Sprintf("legacy-client-csrf-%d", auth.refreshCalls)
	return auth.csrfToken, nil
}

// newRouterForClientTest mounts only the client auth group so focused tests do
// not need admin or promotion dependencies.
func newRouterForClientTest(cfg config.Config, scanAPI *testScanAPI, clientAuth *ClientAuthHandler) *gin.Engine {
	return NewAuthenticatedRouterWithClientAuth(
		cfg,
		NewHandler(scanAPI, healthyChecker{}, cfg),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		clientAuth,
		nil,
		nil,
		nil,
	)
}

func TestClientGoogleStartRedirectsAndStoresStateCookie(t *testing.T) {
	t.Parallel()
	cfg := config.Config{ClientCookieName: "ferris_client_session", AllowedOrigins: []string{"http://localhost:5173"}}
	auth := newTestClientAuth()
	router := newRouterForClientTest(cfg, &testScanAPI{}, NewClientAuthHandler(auth, cfg))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/client/auth/google", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", response.Code)
	}
	if location := response.Header().Get("Location"); location != auth.startAuthURL {
		t.Fatalf("Location = %q, want %q", location, auth.startAuthURL)
	}
	var foundState bool
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == clientOAuthStateCookie {
			foundState = true
			if cookie.Value != auth.startState || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
				t.Fatalf("state cookie = %#v, want HttpOnly SameSite=Lax and matching state", cookie)
			}
		}
	}
	if !foundState {
		t.Fatalf("missing client oauth state cookie")
	}
}

func TestClientGoogleCallbackCreatesSessionAndRedirects(t *testing.T) {
	t.Parallel()
	cfg := config.Config{ClientCookieName: "ferris_client_session", ClientCookieSecure: true, AllowedOrigins: []string{"http://localhost:5173"}}
	auth := newTestClientAuth()
	router := newRouterForClientTest(cfg, &testScanAPI{}, NewClientAuthHandler(auth, cfg))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/client/auth/google/callback?code=code&state="+auth.startState, nil)
	request.AddCookie(&http.Cookie{Name: clientOAuthStateCookie, Value: auth.startState})
	router.ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body=%s", response.Code, response.Body.String())
	}
	sessionCookie := responseCookie(t, response.Result(), "ferris_client_session")
	if sessionCookie.Value != "client-session-token" || !sessionCookie.HttpOnly || !sessionCookie.Secure || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("client session cookie = %#v, want role-scoped HttpOnly Secure SameSite=Lax cookie", sessionCookie)
	}
	csrfCookie := responseCookie(t, response.Result(), "ferris_client_csrf")
	assertAuthCookie(t, csrfCookie, auth.callbackLogin.CSRFToken, true)
	if cookieByName(response.Result(), "ferris_admin_csrf") != nil {
		t.Fatal("client callback set the admin CSRF cookie")
	}
}

func TestClientGoogleCallbackRejectsMissingState(t *testing.T) {
	t.Parallel()
	cfg := config.Config{ClientCookieName: "ferris_client_session", AllowedOrigins: []string{"http://localhost:5173"}}
	auth := newTestClientAuth()
	router := newRouterForClientTest(cfg, &testScanAPI{}, NewClientAuthHandler(auth, cfg))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/client/auth/google/callback?code=code&state=wrong", nil)
	request.AddCookie(&http.Cookie{Name: clientOAuthStateCookie, Value: auth.startState})
	router.ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302 redirect to login", response.Code)
	}
	if location := response.Header().Get("Location"); !strings.Contains(location, "/client/login?error=state_error") {
		t.Fatalf("redirect Location = %q, want frontend login error redirect", location)
	}
}

func TestClientGoogleCallbackMissingStateCookieReturnsStateErrorAndClearsCookie(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		ClientCookieName:     "ferris_client_session",
		ClientRedirectOrigin: "http://127.0.0.1:5173",
		AllowedOrigins:       []string{"http://127.0.0.1:5173"},
	}
	auth := newTestClientAuth()
	router := newRouterForClientTest(cfg, &testScanAPI{}, NewClientAuthHandler(auth, cfg))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/client/auth/google/callback?code=code&state="+auth.startState, nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", response.Code)
	}
	if location := response.Header().Get("Location"); location != "http://127.0.0.1:5173/client/login?error=state_error" {
		t.Fatalf("redirect Location = %q, want state_error on configured client origin", location)
	}
	stateCookie := response.Result().Cookies()
	if len(stateCookie) == 0 {
		t.Fatalf("missing state-cookie clearing response")
	}
	var cleared bool
	for _, cookie := range stateCookie {
		if cookie.Name == clientOAuthStateCookie && cookie.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatalf("state cookie was not cleared after missing callback state")
	}
}

func TestClientMeRequiresSessionAndReusesCSRFCookieAcrossConsecutiveCalls(t *testing.T) {
	t.Parallel()
	cfg := config.Config{ClientCookieName: "ferris_client_session", AllowedOrigins: []string{"http://localhost:5173"}}
	auth := newRotatingClientCSRFAuth()
	router := newRouterForClientTest(cfg, &testScanAPI{}, NewClientAuthHandler(auth, cfg))

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/client/auth/me", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("without cookie status = %d, want 401", response.Code)
	}

	var tokens []string
	for range 2 {
		response = httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/client/auth/me", nil)
		request.AddCookie(&http.Cookie{Name: "ferris_client_session", Value: "client-session-token"})
		request.AddCookie(&http.Cookie{Name: "ferris_client_csrf", Value: "client-csrf-token"})
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("with cookies status = %d, want 200", response.Code)
		}
		var payload struct {
			User struct {
				Email string `json:"email"`
			} `json:"user"`
			CSRFToken string `json:"csrf_token"`
		}
		decodeJSON(t, response, &payload)
		if payload.User.Email != "user@example.com" {
			t.Fatalf("me user email = %q, want user@example.com", payload.User.Email)
		}
		tokens = append(tokens, payload.CSRFToken)
	}
	if tokens[0] != "client-csrf-token" || tokens[1] != tokens[0] {
		t.Fatalf("consecutive /me tokens = %q, want the same cookie-backed token", tokens)
	}
	if auth.refreshCalls != 0 {
		t.Fatalf("RefreshCSRF calls = %d, want 0 for a valid CSRF cookie", auth.refreshCalls)
	}
}

func TestClientMeRefreshesLegacySessionOnceAndKeepsTokenValidForProtectedRequest(t *testing.T) {
	t.Parallel()
	cfg := config.Config{ClientCookieName: "ferris_client_session", ClientCookieSecure: true, AllowedOrigins: []string{"http://localhost:5173"}}
	auth := newRotatingClientCSRFAuth()
	router := newRouterForClientTest(cfg, &testScanAPI{}, NewClientAuthHandler(auth, cfg))

	firstRequest := httptest.NewRequest(http.MethodGet, "/api/v1/client/auth/me", nil)
	firstRequest.AddCookie(&http.Cookie{Name: "ferris_client_session", Value: "client-session-token"})
	firstResponse := httptest.NewRecorder()
	router.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("legacy /me status = %d body=%s, want 200", firstResponse.Code, firstResponse.Body.String())
	}
	var firstPayload struct {
		CSRFToken string `json:"csrf_token"`
	}
	decodeJSON(t, firstResponse, &firstPayload)
	csrfCookie := responseCookie(t, firstResponse.Result(), "ferris_client_csrf")
	assertAuthCookie(t, csrfCookie, firstPayload.CSRFToken, true)

	secondRequest := httptest.NewRequest(http.MethodGet, "/api/v1/client/auth/me", nil)
	secondRequest.AddCookie(&http.Cookie{Name: "ferris_client_session", Value: "client-session-token"})
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

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/v1/client/auth/logout", nil)
	logoutRequest.AddCookie(&http.Cookie{Name: "ferris_client_session", Value: "client-session-token"})
	logoutRequest.Header.Set("X-CSRF-Token", firstPayload.CSRFToken)
	logoutResponse := httptest.NewRecorder()
	router.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("protected logout status = %d body=%s, want 204 with token from first /me", logoutResponse.Code, logoutResponse.Body.String())
	}
}

func TestClientLogoutRequiresCSRFAndRevokesSession(t *testing.T) {
	t.Parallel()
	cfg := config.Config{ClientCookieName: "ferris_client_session", AllowedOrigins: []string{"http://localhost:5173"}}
	auth := newTestClientAuth()
	router := newRouterForClientTest(cfg, &testScanAPI{}, NewClientAuthHandler(auth, cfg))

	request := httptest.NewRequest(http.MethodPost, "/api/v1/client/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: "ferris_client_session", Value: "client-session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("without csrf status = %d, want 403", response.Code)
	}
	if auth.logoutCalls != 0 {
		t.Fatalf("logout called without csrf: %d", auth.logoutCalls)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/client/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: "ferris_client_session", Value: "client-session-token"})
	request.Header.Set("X-CSRF-Token", "client-csrf-token")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || auth.logoutCalls != 1 {
		t.Fatalf("logout status=%d calls=%d, want 204/1", response.Code, auth.logoutCalls)
	}
	if cookie := responseCookie(t, response.Result(), "ferris_client_session"); cookie.MaxAge >= 0 || cookie.Value != "" {
		t.Fatalf("session clearing cookie = %#v, want empty MaxAge=-1", cookie)
	}
	if cookie := responseCookie(t, response.Result(), "ferris_client_csrf"); cookie.MaxAge >= 0 || cookie.Value != "" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("csrf clearing cookie = %#v, want empty HttpOnly SameSite=Lax MaxAge=-1", cookie)
	}
}

func TestClientSessionNeverAuthorizesAdminRoutes(t *testing.T) {
	t.Parallel()
	cfg := config.Config{ClientCookieName: "ferris_client_session", AdminCookieName: "ferris_admin_session", AllowedOrigins: []string{"http://localhost:5173"}}
	auth := newTestClientAuth()
	router := newRouterForClientTest(cfg, &testScanAPI{}, NewClientAuthHandler(auth, cfg))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/jobs", nil)
	request.AddCookie(&http.Cookie{Name: "ferris_client_session", Value: "client-session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code == http.StatusOK {
		t.Fatalf("client session reached an admin route (status %d)", response.Code)
	}
}
