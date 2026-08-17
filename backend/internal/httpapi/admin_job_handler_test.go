package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

func TestAdminJobListReturnsStructuredRowsAndFixtureMarker(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	auth := newTestAdminAuth()
	jobs := &testAdminJobs{page: model.AdminJobPage{
		Items: []model.AdminJob{{
			ID:                   uuid.New(),
			SourceKey:            "development-fixture",
			SourceName:           "Development Fixture",
			SourceApprovalStatus: "REVIEW",
			Title:                "Software Engineer - Development Fixture",
			Company:              "Development Fixture (not a live job)",
			Location:             "Ho Chi Minh City, Vietnam",
			Role:                 "Software Engineer",
			RequiredSkills:       []string{"Go", "PostgreSQL"},
			Status:               "DISABLED",
			WorkMode:             "HYBRID",
			OriginalURL:          "https://example.invalid/job/development-fixture",
			ContentHash:          "hash",
			LastSeenAt:           time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
			UpdatedAt:            time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
		}},
		Page:     1,
		PageSize: 10,
		Total:    1,
	}}
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(&testPromotionAPI{}, cfg), NewAdminAuthHandler(auth, cfg), NewAdminJobHandler(jobs))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/jobs?page=1&page_size=10", nil)
	request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || jobs.pageNumber != 1 || jobs.pageSize != 10 {
		t.Fatalf("status=%d page=%d size=%d body=%s", response.Code, jobs.pageNumber, jobs.pageSize, response.Body.String())
	}
	if !containsText(response.Body.String(), `"is_development_fixture":true`) || containsText(response.Body.String(), "raw") {
		t.Fatalf("job response = %s, want fixture marker and no raw description", response.Body.String())
	}
}

func TestAdminJobListPassesSearchQuery(t *testing.T) {
	cfg := testConfig()
	auth := newTestAdminAuth()
	jobs := &testAdminJobs{}
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(&testPromotionAPI{}, cfg), NewAdminAuthHandler(auth, cfg), NewAdminJobHandler(jobs))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/jobs?q=backend%20developer", nil)
	request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || jobs.filter.Search != "backend developer" {
		t.Fatalf("status=%d search=%q body=%s", response.Code, jobs.filter.Search, response.Body.String())
	}
}

func TestAdminJobListPassesCanonicalLocationFilter(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	auth := newTestAdminAuth()
	locationID := uuid.New()
	jobs := &testAdminJobs{}
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(&testPromotionAPI{}, cfg), NewAdminAuthHandler(auth, cfg), NewAdminJobHandler(jobs))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/jobs?location_id="+locationID.String(), nil)
	request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || jobs.filter.LocationID == nil || *jobs.filter.LocationID != locationID {
		t.Fatalf("status=%d filter=%#v body=%s", response.Code, jobs.filter, response.Body.String())
	}
}

func TestAdminJobListPassesUnresolvedLocationFilter(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	auth := newTestAdminAuth()
	jobs := &testAdminJobs{}
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(&testPromotionAPI{}, cfg), NewAdminAuthHandler(auth, cfg), NewAdminJobHandler(jobs))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/jobs?unresolved=true", nil)
	request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !jobs.filter.UnresolvedLocation {
		t.Fatalf("status=%d filter=%#v body=%s", response.Code, jobs.filter, response.Body.String())
	}
}

func TestAdminJobListRejectsConflictingLocationFilters(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	auth := newTestAdminAuth()
	jobs := &testAdminJobs{}
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(&testPromotionAPI{}, cfg), NewAdminAuthHandler(auth, cfg), NewAdminJobHandler(jobs))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/jobs?location_id="+uuid.NewString()+"&unresolved=true", nil)
	request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || jobs.pageNumber != 0 || !strings.Contains(response.Body.String(), "invalid_job_filter") {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, jobs.pageNumber, response.Body.String())
	}
}

func TestAdminJobListRejectsOversizedPage(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	auth := newTestAdminAuth()
	jobs := &testAdminJobs{}
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(&testPromotionAPI{}, cfg), NewAdminAuthHandler(auth, cfg), NewAdminJobHandler(jobs))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/jobs?page_size=51", nil)
	request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || jobs.pageNumber != 0 {
		t.Fatalf("status=%d calls page=%d, want 400 without repository call", response.Code, jobs.pageNumber)
	}
}

type testAdminJobs struct {
	page       model.AdminJobPage
	pageNumber int
	pageSize   int
	filter     repository.AdminJobFilter
}

func (jobs *testAdminJobs) ListAdminJobs(_ context.Context, page, pageSize int, filter repository.AdminJobFilter) (model.AdminJobPage, error) {
	jobs.pageNumber = page
	jobs.pageSize = pageSize
	jobs.filter = filter
	return jobs.page, nil
}

func containsText(value, target string) bool {
	for i := 0; i+len(target) <= len(value); i++ {
		if value[i:i+len(target)] == target {
			return true
		}
	}
	return false
}
