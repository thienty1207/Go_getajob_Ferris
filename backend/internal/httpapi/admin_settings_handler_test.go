package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/service"
)

type testSettingsAPI struct {
	settings       model.CrawlerSettings
	updateCalls    int
	updatedHours   int
	updatedMinutes int
	updatedActor   string
}

func (api *testSettingsAPI) GetCrawlerSettings(context.Context) (model.CrawlerSettings, error) {
	return api.settings, nil
}

func (api *testSettingsAPI) GetCrawlerRuntime(context.Context) (model.CrawlerRuntime, error) {
	return model.CrawlerRuntime{Status: "OFFLINE"}, nil
}

func (api *testSettingsAPI) UpdateCrawlerSettings(_ context.Context, hours, minutes int, actor string) (model.CrawlerSettings, error) {
	api.updateCalls++
	api.updatedHours = hours
	api.updatedMinutes = minutes
	api.updatedActor = actor
	return api.settings, nil
}

func TestAdminSettingsRoutesReadAndUpdateCrawlerInterval(t *testing.T) {
	cfg := config.Config{AdminCookieName: "ferris_admin_session"}
	auth := newTestAdminAuth()
	api := &testSettingsAPI{settings: service.CrawlerSettingsFromSeconds(23_400)}
	router := NewAuthenticatedRouterWithLocations(
		cfg,
		NewHandler(&testScanAPI{}, healthyChecker{}, cfg),
		NewPromotionHandler(&testPromotionAPI{}, cfg),
		NewAdminAuthHandler(auth, cfg),
		nil,
		nil,
		NewAdminLocationHandler(&testLocationAPI{}),
		NewAdminSettingsHandler(api),
	)

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	getRequest.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	getResponse := httptest.NewRecorder()
	router.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK || !strings.Contains(getResponse.Body.String(), `"interval_hours":6`) || !strings.Contains(getResponse.Body.String(), `"status":"OFFLINE"`) {
		t.Fatalf("GET settings status = %d body = %s", getResponse.Code, getResponse.Body.String())
	}

	patchRequest := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/settings/crawler", strings.NewReader(`{"interval_hours":2,"interval_minutes":30}`))
	patchRequest.Header.Set("Content-Type", "application/json")
	patchRequest.Header.Set("X-CSRF-Token", "csrf-token")
	patchRequest.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	patchResponse := httptest.NewRecorder()
	router.ServeHTTP(patchResponse, patchRequest)
	if patchResponse.Code != http.StatusOK || api.updateCalls != 1 || api.updatedHours != 2 || api.updatedMinutes != 30 || api.updatedActor != "admin@example.com" {
		t.Fatalf("PATCH settings status = %d calls = %d values = %d:%d actor=%q body=%s", patchResponse.Code, api.updateCalls, api.updatedHours, api.updatedMinutes, api.updatedActor, patchResponse.Body.String())
	}
}

func TestAdminSettingsUpdateRequiresCSRF(t *testing.T) {
	cfg := config.Config{AdminCookieName: "ferris_admin_session"}
	auth := newTestAdminAuth()
	api := &testSettingsAPI{}
	router := NewAuthenticatedRouterWithLocations(
		cfg,
		NewHandler(&testScanAPI{}, healthyChecker{}, cfg),
		NewPromotionHandler(&testPromotionAPI{}, cfg),
		NewAdminAuthHandler(auth, cfg),
		nil,
		nil,
		NewAdminLocationHandler(&testLocationAPI{}),
		NewAdminSettingsHandler(api),
	)
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/settings/crawler", strings.NewReader(`{"interval_hours":6,"interval_minutes":0}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || api.updateCalls != 0 {
		t.Fatalf("status = %d update calls = %d", response.Code, api.updateCalls)
	}
}
