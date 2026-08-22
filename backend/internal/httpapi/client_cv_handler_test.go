package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

type testClientCVAPI struct {
	items        []model.ClientCVHistoryItem
	total        int
	listUserID   uuid.UUID
	deleteUserID uuid.UUID
	deleteScanID uuid.UUID
}

func (api *testClientCVAPI) List(_ context.Context, userID uuid.UUID, _ int, _ int) ([]model.ClientCVHistoryItem, int, error) {
	api.listUserID = userID
	return api.items, api.total, nil
}

func (api *testClientCVAPI) Delete(_ context.Context, userID, scanID uuid.UUID) error {
	api.deleteUserID = userID
	api.deleteScanID = scanID
	return nil
}

func TestClientCVHistoryRequiresClientSession(t *testing.T) {
	t.Parallel()
	cfg := config.Config{ClientCookieName: "ferris_client_session", AllowedOrigins: []string{"http://127.0.0.1:5173"}}
	auth := newTestClientAuth()
	cvAPI := &testClientCVAPI{}
	router := newRouterForClientCVTest(cfg, auth, cvAPI)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/client/cv-history", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", response.Code)
	}
}

func TestClientCVHistoryListsStructuredDataAndDeletesWithCSRF(t *testing.T) {
	t.Parallel()
	cfg := config.Config{ClientCookieName: "ferris_client_session", AllowedOrigins: []string{"http://127.0.0.1:5173"}}
	auth := newTestClientAuth()
	scanID := uuid.New()
	cvAPI := &testClientCVAPI{
		total: 1,
		items: []model.ClientCVHistoryItem{{
			ScanID:     scanID,
			Status:     model.StatusCompleted,
			Location:   "Hồ Chí Minh",
			CreatedAt:  time.Date(2026, 8, 20, 7, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Date(2026, 8, 20, 7, 1, 0, 0, time.UTC),
			MatchCount: 3,
			Profile:    &model.StructuredProfile{Roles: []string{"Backend Developer"}, Skills: []string{"Go"}, Seniority: "mid"},
		}},
	}
	router := newRouterForClientCVTest(cfg, auth, cvAPI)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/client/cv-history", nil)
	request.AddCookie(&http.Cookie{Name: cfg.ClientCookieName, Value: "client-session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	if cvAPI.listUserID != auth.session.User.ID || !strings.Contains(response.Body.String(), "Backend Developer") || strings.Contains(response.Body.String(), "raw_cv") {
		t.Fatalf("list response=%s user=%s, want structured profile for session owner", response.Body.String(), cvAPI.listUserID)
	}

	request = httptest.NewRequest(http.MethodDelete, "/api/v1/client/cv-history/"+scanID.String(), nil)
	request.AddCookie(&http.Cookie{Name: cfg.ClientCookieName, Value: "client-session-token"})
	request.Header.Set("X-CSRF-Token", "client-csrf-token")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || cvAPI.deleteUserID != auth.session.User.ID || cvAPI.deleteScanID != scanID {
		t.Fatalf("delete status=%d user=%s scan=%s", response.Code, cvAPI.deleteUserID, cvAPI.deleteScanID)
	}
}

func newRouterForClientCVTest(cfg config.Config, auth *testClientAuth, cvAPI *testClientCVAPI) *gin.Engine {
	return NewAuthenticatedRouterWithClientAuth(
		cfg,
		NewHandler(&testScanAPI{}, healthyChecker{}, cfg),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		NewClientAuthHandler(auth, cfg),
		NewClientCVHandler(cvAPI),
		nil,
		nil,
	)
}
