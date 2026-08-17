package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/service"
	"github.com/google/uuid"
)

type testJobLinkAPI struct {
	createCalls  int
	updateCalls  int
	disableCalls int
	statusCalls  int
	deleteCalls  int
	crawlCalls   int
	page         model.JobLinkPage
	item         model.JobLink
	request      model.CrawlRequest
}

func (api *testJobLinkAPI) ListJobLinks(context.Context, int, int) (model.JobLinkPage, error) {
	return api.page, nil
}

func (api *testJobLinkAPI) Create(context.Context, service.JobLinkInput) (model.JobLink, error) {
	api.createCalls++
	return api.item, nil
}

func (api *testJobLinkAPI) Update(context.Context, uuid.UUID, service.JobLinkInput) (model.JobLink, error) {
	api.updateCalls++
	return api.item, nil
}

func (api *testJobLinkAPI) Disable(context.Context, uuid.UUID) error {
	api.disableCalls++
	return nil
}

func (api *testJobLinkAPI) SetStatus(context.Context, uuid.UUID, string) error {
	api.statusCalls++
	return nil
}

func (api *testJobLinkAPI) Delete(context.Context, uuid.UUID) error {
	api.deleteCalls++
	return nil
}

func (api *testJobLinkAPI) Request(context.Context, uuid.UUID, string) (model.CrawlRequest, error) {
	api.crawlCalls++
	return api.request, nil
}

func TestAdminJobLinkRoutesRequireSessionAndCSRF(t *testing.T) {
	cfg := config.Config{AdminCookieName: "ferris_admin_session"}
	auth := newTestAdminAuth()
	api := &testJobLinkAPI{}
	router := NewAuthenticatedRouter(
		cfg,
		NewHandler(&testScanAPI{}, healthyChecker{}, cfg),
		NewPromotionHandler(&testPromotionAPI{}, cfg),
		NewAdminAuthHandler(auth, cfg),
		nil,
		NewAdminJobLinkHandler(api),
	)

	unauthenticated := httptest.NewRecorder()
	router.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/api/v1/admin/job-links", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", unauthenticated.Code)
	}

	missingCSRF := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/job-links", strings.NewReader(`{"url":"https://jobs.example.com/careers"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	router.ServeHTTP(missingCSRF, request)
	if missingCSRF.Code != http.StatusForbidden || api.createCalls != 0 {
		t.Fatalf("missing CSRF status = %d create calls = %d", missingCSRF.Code, api.createCalls)
	}
}

func TestAdminJobLinkCrawlNowRequiresCSRFAndQueuesRequest(t *testing.T) {
	cfg := config.Config{AdminCookieName: "ferris_admin_session"}
	auth := newTestAdminAuth()
	id := uuid.New()
	requestID := uuid.New()
	api := &testJobLinkAPI{request: model.CrawlRequest{ID: requestID, SourceID: id, Status: "PENDING", RequestedBy: "admin@example.com"}}
	router := NewAuthenticatedRouter(
		cfg,
		NewHandler(&testScanAPI{}, healthyChecker{}, cfg),
		NewPromotionHandler(&testPromotionAPI{}, cfg),
		NewAdminAuthHandler(auth, cfg),
		nil,
		NewAdminJobLinkHandler(api, api),
	)

	withoutCSRF := httptest.NewRequest(http.MethodPost, "/api/v1/admin/job-links/"+id.String()+"/crawl", nil)
	withoutCSRF.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	withoutCSRFResponse := httptest.NewRecorder()
	router.ServeHTTP(withoutCSRFResponse, withoutCSRF)
	if withoutCSRFResponse.Code != http.StatusForbidden || api.crawlCalls != 0 {
		t.Fatalf("without CSRF status = %d calls = %d", withoutCSRFResponse.Code, api.crawlCalls)
	}

	withCSRF := httptest.NewRequest(http.MethodPost, "/api/v1/admin/job-links/"+id.String()+"/crawl", nil)
	withCSRF.Header.Set("X-CSRF-Token", "csrf-token")
	withCSRF.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	withCSRFResponse := httptest.NewRecorder()
	router.ServeHTTP(withCSRFResponse, withCSRF)
	if withCSRFResponse.Code != http.StatusAccepted || api.crawlCalls != 1 || !strings.Contains(withCSRFResponse.Body.String(), requestID.String()) {
		t.Fatalf("with CSRF status = %d calls = %d body = %s", withCSRFResponse.Code, api.crawlCalls, withCSRFResponse.Body.String())
	}
}

func TestAdminJobLinkRoutesExposeMetadataAndStateChanges(t *testing.T) {
	cfg := config.Config{AdminCookieName: "ferris_admin_session"}
	auth := newTestAdminAuth()
	id := uuid.New()
	api := &testJobLinkAPI{item: model.JobLink{ID: id, URL: "https://jobs.example.com/careers/", DisplayName: "jobs.example.com", ApprovalStatus: "ACTIVE"}}
	router := NewAuthenticatedRouter(
		cfg,
		NewHandler(&testScanAPI{}, healthyChecker{}, cfg),
		NewPromotionHandler(&testPromotionAPI{}, cfg),
		NewAdminAuthHandler(auth, cfg),
		nil,
		NewAdminJobLinkHandler(api),
	)

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/job-links", strings.NewReader(`{"url":"https://jobs.example.com/careers"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set("X-CSRF-Token", "csrf-token")
	createRequest.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	createResponse := httptest.NewRecorder()
	router.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated || api.createCalls != 1 {
		t.Fatalf("create status = %d calls = %d body = %s", createResponse.Code, api.createCalls, createResponse.Body.String())
	}
	if strings.Contains(strings.ToLower(createResponse.Body.String()), "raw") || strings.Contains(strings.ToLower(createResponse.Body.String()), "html") {
		t.Fatalf("response exposed fetched content: %s", createResponse.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/job-links/"+id.String(), nil)
	deleteRequest.Header.Set("X-CSRF-Token", "csrf-token")
	deleteRequest.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	deleteResponse := httptest.NewRecorder()
	router.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || api.deleteCalls != 1 || api.disableCalls != 0 {
		t.Fatalf("delete status = %d hard-delete calls = %d soft-disable calls = %d", deleteResponse.Code, api.deleteCalls, api.disableCalls)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/job-links?page=1&page_size=10", nil)
	listRequest.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d body = %s", listResponse.Code, listResponse.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(listResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("list response JSON: %v", err)
	}
	if _, ok := payload["items"]; !ok {
		t.Fatalf("list response missing items: %s", listResponse.Body.String())
	}
}

func TestAdminJobLinkStatusRouteIsSeparateFromDelete(t *testing.T) {
	cfg := config.Config{AdminCookieName: "ferris_admin_session"}
	auth := newTestAdminAuth()
	id := uuid.New()
	api := &testJobLinkAPI{item: model.JobLink{ID: id, URL: "https://jobs.example.com/careers/", ApprovalStatus: "ACTIVE"}}
	router := NewAuthenticatedRouter(
		cfg,
		NewHandler(&testScanAPI{}, healthyChecker{}, cfg),
		NewPromotionHandler(&testPromotionAPI{}, cfg),
		NewAdminAuthHandler(auth, cfg),
		nil,
		NewAdminJobLinkHandler(api),
	)

	request := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/job-links/"+id.String()+"/status", strings.NewReader(`{"approval_status":"DISABLED"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "csrf-token")
	request.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || api.statusCalls != 1 || api.deleteCalls != 0 {
		t.Fatalf("status response = %d status calls = %d delete calls = %d", response.Code, api.statusCalls, api.deleteCalls)
	}
}

func TestAdminJobLinkUpdateRouteReapprovesAndPreservesSessionBoundary(t *testing.T) {
	cfg := config.Config{AdminCookieName: "ferris_admin_session"}
	auth := newTestAdminAuth()
	id := uuid.New()
	api := &testJobLinkAPI{item: model.JobLink{ID: id, URL: "https://new.example.com/jobs/", ApprovalStatus: "ACTIVE"}}
	router := NewAuthenticatedRouter(
		cfg,
		NewHandler(&testScanAPI{}, healthyChecker{}, cfg),
		NewPromotionHandler(&testPromotionAPI{}, cfg),
		NewAdminAuthHandler(auth, cfg),
		nil,
		NewAdminJobLinkHandler(api),
	)

	request := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/job-links/"+id.String(), strings.NewReader(`{"url":"https://new.example.com/jobs"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "csrf-token")
	request.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || api.updateCalls != 1 {
		t.Fatalf("update status = %d calls = %d body = %s", response.Code, api.updateCalls, response.Body.String())
	}
}

func TestAdminJobLinkListSerializesReviewApprovalEvidenceAsNull(t *testing.T) {
	cfg := config.Config{AdminCookieName: "ferris_admin_session"}
	auth := newTestAdminAuth()
	api := &testJobLinkAPI{page: model.JobLinkPage{
		Page:     1,
		PageSize: 10,
		Total:    1,
		Items: []model.JobLink{{
			ID:             uuid.New(),
			URL:            "https://jobs.example.com/careers/",
			SourceKey:      "source-review",
			DisplayName:    "jobs.example.com",
			ApprovalStatus: "REVIEW",
		}},
	}}
	router := NewAuthenticatedRouter(
		cfg,
		NewHandler(&testScanAPI{}, healthyChecker{}, cfg),
		NewPromotionHandler(&testPromotionAPI{}, cfg),
		NewAdminAuthHandler(auth, cfg),
		nil,
		NewAdminJobLinkHandler(api),
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/job-links?page=1&page_size=10", nil)
	request.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Items []struct {
			ApprovalStatus string  `json:"approval_status"`
			ApprovedAt     *string `json:"approved_at"`
			ApprovedBy     *string `json:"approved_by"`
		} `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("list response JSON: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ApprovalStatus != "REVIEW" {
		t.Fatalf("unexpected review item: %#v", payload.Items)
	}
	if payload.Items[0].ApprovedAt != nil || payload.Items[0].ApprovedBy != nil {
		t.Fatalf("review approval evidence should be null: %#v", payload.Items[0])
	}
}

func TestAdminJobLinkListIncludesZeroCrawlCountersForLatestRun(t *testing.T) {
	cfg := config.Config{AdminCookieName: "ferris_admin_session"}
	auth := newTestAdminAuth()
	status := "ANOMALY"
	finishedAt := time.Now().UTC()
	api := &testJobLinkAPI{page: model.JobLinkPage{
		Page:     1,
		PageSize: 10,
		Total:    1,
		Items: []model.JobLink{{
			ID:               uuid.New(),
			URL:              "https://jobs.example.com/careers/",
			SourceKey:        "source-real",
			DisplayName:      "jobs.example.com",
			ApprovalStatus:   "ACTIVE",
			LastCrawlStatus:  &status,
			LastCrawlAt:      &finishedAt,
			LastCrawlPages:   1,
			LastCrawlJobs:    0,
			LastCrawlCreated: 0,
			LastCrawlUpdated: 0,
			LastCrawlMissing: 0,
		}},
	}}
	router := NewAuthenticatedRouter(
		cfg,
		NewHandler(&testScanAPI{}, healthyChecker{}, cfg),
		NewPromotionHandler(&testPromotionAPI{}, cfg),
		NewAdminAuthHandler(auth, cfg),
		nil,
		NewAdminJobLinkHandler(api),
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/job-links?page=1&page_size=10", nil)
	request.AddCookie(&http.Cookie{Name: cfg.AdminCookieName, Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("list response JSON: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("unexpected item count: %#v", payload.Items)
	}
	if jobs, ok := payload.Items[0]["last_crawl_jobs"]; !ok || jobs != float64(0) {
		t.Fatalf("last_crawl_jobs = %#v, want explicit zero", jobs)
	}
}
