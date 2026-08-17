package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/service"
	"github.com/google/uuid"
)

type testLocationAPI struct {
	items       []model.AdminLocation
	options     []model.AdminLocationOption
	activeItems []model.ClientLocation
	createCalls int
	assignCalls int
}

func (api *testLocationAPI) List(_ context.Context, page, pageSize int) (model.AdminLocationPage, error) {
	return model.AdminLocationPage{Items: api.items, Page: page, PageSize: pageSize, Total: len(api.items)}, nil
}

func (api *testLocationAPI) Options(context.Context) ([]model.AdminLocationOption, error) {
	return api.options, nil
}

func (api *testLocationAPI) ListActive(context.Context) ([]model.ClientLocation, error) {
	return api.activeItems, nil
}

func (api *testLocationAPI) Create(context.Context, service.LocationInput) (model.AdminLocation, error) {
	api.createCalls++
	return model.AdminLocation{ID: uuid.New(), DisplayName: "Hà Nội", Province: "Hà Nội", Country: "Vietnam", IsActive: true}, nil
}

func TestClientLocationRouteReturnsOnlyCanonicalSelectionFields(t *testing.T) {
	cfg := config.Config{AdminCookieName: "ferris_admin_session"}
	auth := newTestAdminAuth()
	locationID := uuid.New()
	api := &testLocationAPI{activeItems: []model.ClientLocation{{
		ID:          locationID,
		DisplayName: "Hồ Chí Minh",
		Province:    "Hồ Chí Minh",
		Country:     "Vietnam",
		IsActive:    true,
	}}}
	router := NewAuthenticatedRouterWithLocations(
		cfg,
		NewHandler(&testScanAPI{}, healthyChecker{}, cfg),
		NewPromotionHandler(&testPromotionAPI{}, cfg),
		NewAdminAuthHandler(auth, cfg),
		nil,
		nil,
		NewAdminLocationHandler(api),
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/client/locations", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if !containsText(response.Body.String(), locationID.String()) || !containsText(response.Body.String(), "Hồ Chí Minh") {
		t.Fatalf("client location payload = %s", response.Body.String())
	}
	if containsText(response.Body.String(), "job_count") || containsText(response.Body.String(), "created_at") {
		t.Fatalf("client location payload exposed admin-only fields: %s", response.Body.String())
	}
}

func (api *testLocationAPI) Update(context.Context, uuid.UUID, service.LocationInput) (model.AdminLocation, error) {
	return model.AdminLocation{ID: uuid.New(), DisplayName: "Hà Nội", Province: "Hà Nội", Country: "Vietnam", IsActive: true}, nil
}

func (api *testLocationAPI) AssignJobLocation(context.Context, uuid.UUID, *uuid.UUID) error {
	api.assignCalls++
	return nil
}

func TestAdminLocationRoutesExposeDatabaseContractAndProtectWrites(t *testing.T) {
	cfg := config.Config{AdminCookieName: "ferris_admin_session"}
	auth := newTestAdminAuth()
	api := &testLocationAPI{items: []model.AdminLocation{{ID: uuid.New(), DisplayName: "Hồ Chí Minh", Province: "Hồ Chí Minh", Country: "Vietnam", IsActive: true}}}
	router := NewAuthenticatedRouterWithLocations(
		cfg,
		NewHandler(&testScanAPI{}, healthyChecker{}, cfg),
		NewPromotionHandler(&testPromotionAPI{}, cfg),
		NewAdminAuthHandler(auth, cfg),
		nil,
		nil,
		NewAdminLocationHandler(api),
	)

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/locations", nil)
	listRequest.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d body = %s", listResponse.Code, listResponse.Body.String())
	}
	var payload struct {
		Items []struct {
			DisplayName string `json:"display_name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("list JSON = %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].DisplayName != "Hồ Chí Minh" {
		t.Fatalf("location payload = %#v", payload.Items)
	}

	optionsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/locations/options", nil)
	optionsRequest.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	optionsResponse := httptest.NewRecorder()
	router.ServeHTTP(optionsResponse, optionsRequest)
	if optionsResponse.Code != http.StatusOK || !strings.Contains(optionsResponse.Body.String(), `"items"`) {
		t.Fatalf("options status = %d body = %s", optionsResponse.Code, optionsResponse.Body.String())
	}

	missingCSRF := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/locations", strings.NewReader(`{"display_name":"Hà Nội","province":"Hà Nội","country":"Vietnam"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	router.ServeHTTP(missingCSRF, createRequest)
	if missingCSRF.Code != http.StatusForbidden || api.createCalls != 0 {
		t.Fatalf("missing CSRF status = %d create calls = %d", missingCSRF.Code, api.createCalls)
	}

	assignResponse := httptest.NewRecorder()
	assignRequest := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/jobs/"+uuid.NewString()+"/location", strings.NewReader(`{"location_id":null}`))
	assignRequest.Header.Set("Content-Type", "application/json")
	assignRequest.Header.Set("X-CSRF-Token", "csrf-token")
	assignRequest.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	router.ServeHTTP(assignResponse, assignRequest)
	if assignResponse.Code != http.StatusNoContent || api.assignCalls != 1 {
		t.Fatalf("assign status = %d calls = %d", assignResponse.Code, api.assignCalls)
	}
}

func TestAdminLocationListRejectsPageSizeOverTen(t *testing.T) {
	cfg := config.Config{AdminCookieName: "ferris_admin_session"}
	auth := newTestAdminAuth()
	api := &testLocationAPI{}
	router := NewAuthenticatedRouterWithLocations(
		cfg,
		NewHandler(&testScanAPI{}, healthyChecker{}, cfg),
		NewPromotionHandler(&testPromotionAPI{}, cfg),
		NewAdminAuthHandler(auth, cfg),
		nil,
		nil,
		NewAdminLocationHandler(api),
	)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/locations?page_size=11", nil)
	request.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), `"items"`) {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}
