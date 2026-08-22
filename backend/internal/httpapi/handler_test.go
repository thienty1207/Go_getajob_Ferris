package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/gogetsomefoodferris/backend/internal/service"
	"github.com/google/uuid"
)

func TestCreateScanReturnsFrontendAcceptedContract(t *testing.T) {
	scanID := uuid.New()
	locationID := uuid.New()
	api := &testScanAPI{startID: scanID}
	router := NewRouter(testConfig(), NewHandler(api, healthyChecker{}, testConfig()))

	body, contentType := multipartBody(t, "resume.pdf", "%PDF-1.7", map[string]string{
		"location_id": locationID.String(),
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/client/scans", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", response.Code, response.Body.String())
	}
	var payload map[string]any
	decodeJSON(t, response, &payload)
	if payload["scan_id"] != scanID.String() || payload["status"] != "processing" {
		t.Fatalf("payload = %#v, want snake_case processing response", payload)
	}
	if api.lastInput.LocationID != locationID || api.lastInput.RadiusKm != 0 || api.lastInput.File == nil {
		t.Fatalf("service input = %#v, want parsed multipart fields", api.lastInput)
	}
}

func TestAuthenticatedCreateScanRequiresCSRFAndBindsSessionOwner(t *testing.T) {
	cfg := testConfig()
	cfg.ClientCookieName = "ferris_client_session"
	auth := newTestClientAuth()
	api := &testScanAPI{startID: uuid.New()}
	handler := NewHandler(api, healthyChecker{}, cfg)
	router := NewAuthenticatedRouterWithClientAuth(
		cfg, handler, nil, nil, nil, nil, nil, nil,
		NewClientAuthHandler(auth, cfg), nil, nil, nil,
	)
	locationID := uuid.New()

	body, contentType := multipartBody(t, "resume.txt", "Backend Engineer with Go experience", map[string]string{"location_id": locationID.String()})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/client/scans", body)
	request.Header.Set("Content-Type", contentType)
	request.AddCookie(&http.Cookie{Name: cfg.ClientCookieName, Value: "client-session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || api.startCalls != 0 {
		t.Fatalf("request without CSRF status=%d starts=%d body=%s, want 403 and no scan", response.Code, api.startCalls, response.Body.String())
	}

	body, contentType = multipartBody(t, "resume.txt", "Backend Engineer with Go experience", map[string]string{"location_id": locationID.String()})
	request = httptest.NewRequest(http.MethodPost, "/api/v1/client/scans", body)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("X-CSRF-Token", "client-csrf-token")
	request.AddCookie(&http.Cookie{Name: cfg.ClientCookieName, Value: "client-session-token"})
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || api.startCalls != 1 || api.lastInput.ClientUserID == nil || *api.lastInput.ClientUserID != auth.session.User.ID {
		t.Fatalf("authenticated upload status=%d starts=%d owner=%v body=%s, want session-owned scan", response.Code, api.startCalls, api.lastInput.ClientUserID, response.Body.String())
	}
}

func TestCreateScanAcceptsCanonicalLocationWithoutRadius(t *testing.T) {
	api := &testScanAPI{startID: uuid.New()}
	router := NewRouter(testConfig(), NewHandler(api, healthyChecker{}, testConfig()))
	body, contentType := multipartBody(t, "resume.pdf", "%PDF-1.7", map[string]string{
		"location_id": uuid.NewString(),
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/client/scans", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted || api.lastInput.RadiusKm != 0 {
		t.Fatalf("status=%d radius=%v body=%s, want accepted location-only request", response.Code, api.lastInput.RadiusKm, response.Body.String())
	}
}

func TestCreateScanRejectsMissingCanonicalLocationID(t *testing.T) {
	api := &testScanAPI{startID: uuid.New()}
	router := NewRouter(testConfig(), NewHandler(api, healthyChecker{}, testConfig()))
	body, contentType := multipartBody(t, "resume.pdf", "%PDF-1.7", map[string]string{"radius_km": "25"})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/client/scans", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || api.startCalls != 0 {
		t.Fatalf("status=%d start calls=%d body=%s", response.Code, api.startCalls, response.Body.String())
	}
	assertErrorResponse(t, response, "invalid_scan_request", "location")
}

func TestCreateScanRejectsMissingMultipartFields(t *testing.T) {
	api := &testScanAPI{startID: uuid.New()}
	router := NewRouter(testConfig(), NewHandler(api, healthyChecker{}, testConfig()))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/client/scans", strings.NewReader("location=Hanoi&radius_km=25"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	assertErrorResponse(t, response, "invalid_scan_request", "cv")
	if api.startCalls != 0 {
		t.Fatalf("Start calls = %d, want 0", api.startCalls)
	}
}

func TestCreateScanMapsInternalErrorWithoutLeakingDetails(t *testing.T) {
	api := &testScanAPI{startErr: errors.New("password=super-secret SQLSTATE 23505")}
	router := NewRouter(testConfig(), NewHandler(api, healthyChecker{}, testConfig()))
	body, contentType := multipartBody(t, "resume.txt", "plain text cv", map[string]string{
		"location_id": uuid.NewString(),
		"radius_km":   "25",
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/client/scans", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	assertErrorResponse(t, response, "internal_error", "Service")
	if strings.Contains(response.Body.String(), "super-secret") || strings.Contains(response.Body.String(), "SQLSTATE") {
		t.Fatalf("response leaked internal error: %s", response.Body.String())
	}
}

func TestGetScanMapsProcessingStatus(t *testing.T) {
	scanID := uuid.New()
	api := &testScanAPI{scan: model.Scan{ID: scanID, Status: model.StatusParsing}}
	router := NewRouter(testConfig(), NewHandler(api, healthyChecker{}, testConfig()))
	response := performGet(router, scanID.String())

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store for private scan data", got)
	}
	var payload map[string]any
	decodeJSON(t, response, &payload)
	if payload["scan_id"] != scanID.String() || payload["status"] != "processing" {
		t.Fatalf("payload = %#v, want processing", payload)
	}
}

func TestGetScanMapsFailedStatusWithoutInternalCode(t *testing.T) {
	scanID := uuid.New()
	api := &testScanAPI{scan: model.Scan{ID: scanID, Status: model.StatusFailed, ErrorCode: "database_password_leak"}}
	router := NewRouter(testConfig(), NewHandler(api, healthyChecker{}, testConfig()))
	response := performGet(router, scanID.String())

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var payload map[string]any
	decodeJSON(t, response, &payload)
	if payload["status"] != "failed" || payload["message"] != "Matching service không thể hoàn tất việc quét CV." || strings.Contains(response.Body.String(), "database_password") {
		t.Fatalf("payload = %#v, must expose only failed status", payload)
	}
}

func TestGetScanCapsSkillTagsAndReturnsCompletedContract(t *testing.T) {
	scanID := uuid.New()
	distance := 4.2
	api := &testScanAPI{scan: model.Scan{
		ID:     scanID,
		Status: model.StatusCompleted,
		Matches: []model.JobMatch{{
			ID:             uuid.New(),
			MatchPercent:   87.5,
			Title:          "Backend Engineer",
			Company:        "Real Company",
			Location:       "Hà Nội",
			DistanceKm:     &distance,
			EmploymentType: "Full-time",
			WorkMode:       "onsite",
			Salary:         &model.JobSalary{Display: "30.000.000 VND", Currency: "VND"},
			SkillTags:      []string{"Go", "PostgreSQL", "Docker", "Kubernetes"},
			OriginalURL:    "https://jobs.example/backend",
		}},
	}}
	router := NewRouter(testConfig(), NewHandler(api, healthyChecker{}, testConfig()))
	response := performGet(router, scanID.String())

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Status  string `json:"status"`
		Matches []struct {
			MatchPercent float64  `json:"match_percent"`
			SkillTags    []string `json:"skill_tags"`
			DistanceKm   float64  `json:"distance_km"`
			Salary       struct {
				Display  string `json:"display"`
				Currency string `json:"currency"`
			} `json:"salary"`
		} `json:"matches"`
	}
	decodeJSON(t, response, &payload)
	if payload.Status != "completed" || len(payload.Matches) != 1 {
		t.Fatalf("payload = %#v, want one completed match", payload)
	}
	if payload.Matches[0].MatchPercent != 87.5 || payload.Matches[0].DistanceKm != distance || len(payload.Matches[0].SkillTags) != 3 || payload.Matches[0].Salary.Currency != "VND" {
		t.Fatalf("completed match = %#v, want mapped public fields", payload.Matches[0])
	}
}

func TestGetScanRejectsInvalidUUIDBeforeRepository(t *testing.T) {
	api := &testScanAPI{}
	router := NewRouter(testConfig(), NewHandler(api, healthyChecker{}, testConfig()))
	response := performGet(router, "not-a-uuid")

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	assertErrorResponse(t, response, "invalid_scan_id", "Scan")
	if api.getCalls != 0 {
		t.Fatalf("Get calls = %d, want 0", api.getCalls)
	}
}

func TestGetScanMapsNotFound(t *testing.T) {
	api := &testScanAPI{getErr: repository.ErrScanNotFound}
	router := NewRouter(testConfig(), NewHandler(api, healthyChecker{}, testConfig()))
	response := performGet(router, uuid.New().String())

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	assertErrorResponse(t, response, "scan_not_found", "không tồn tại")
}

func TestGetScanIsRateLimitedSeparatelyFromUpload(t *testing.T) {
	scanID := uuid.New()
	api := &testScanAPI{scan: model.Scan{ID: scanID, Status: model.StatusParsing}}
	cfg := testConfig()
	cfg.ReadRateLimitPerMinute = 1
	router := NewRouter(cfg, NewHandler(api, healthyChecker{}, cfg))

	first := performGet(router, scanID.String())
	second := performGet(router, scanID.String())
	if first.Code != http.StatusOK {
		t.Fatalf("first GET status = %d, want 200", first.Code)
	}
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second GET status = %d, want 429", second.Code)
	}
}

func TestOptionsUsesExplicitCORSOrigin(t *testing.T) {
	router := NewRouter(testConfig(), NewHandler(&testScanAPI{}, healthyChecker{}, testConfig()))
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/client/scans", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	request.Header.Set("Access-Control-Request-Headers", "content-type,x-csrf-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want explicit origin", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPatch) {
		t.Fatalf("Access-Control-Allow-Methods = %q, want PATCH for admin status updates", got)
	}
}

func TestHealthMapsDatabaseFailureWithoutLeakingDetails(t *testing.T) {
	router := NewRouter(testConfig(), NewHandler(&testScanAPI{}, failingChecker{}, testConfig()))
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	assertErrorResponse(t, response, "database_unavailable", "chưa sẵn sàng")
	if strings.Contains(response.Body.String(), "password") {
		t.Fatalf("health response leaked database error: %s", response.Body.String())
	}
}

func testConfig() config.Config {
	return config.Config{
		Address:                "127.0.0.1:8080",
		MaxCVBytes:             10 * 1024 * 1024,
		MaxRadiusKm:            500,
		AllowedOrigins:         []string{"http://localhost:5173"},
		RateLimitPerMinute:     10,
		ReadRateLimitPerMinute: 60,
	}
}

type healthyChecker struct{}

func (healthyChecker) Ping(context.Context) error { return nil }

type failingChecker struct{}

func (failingChecker) Ping(context.Context) error { return errors.New("password=secret") }

type testScanAPI struct {
	startID    uuid.UUID
	startErr   error
	scan       model.Scan
	getErr     error
	startCalls int
	getCalls   int
	lastInput  service.ScanInput
}

func (api *testScanAPI) Start(_ context.Context, input service.ScanInput) (uuid.UUID, error) {
	api.startCalls++
	api.lastInput = input
	return api.startID, api.startErr
}

func (api *testScanAPI) Get(_ context.Context, _ uuid.UUID) (model.Scan, error) {
	api.getCalls++
	return api.scan, api.getErr
}

func multipartBody(t *testing.T, filename, content string, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("cv", filename)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("part.Write() error = %v", err)
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("WriteField(%q) error = %v", key, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}
	return &body, writer.FormDataContentType()
}

func performGet(router http.Handler, scanID string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/client/scans/"+scanID, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func decodeJSON(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("response JSON error = %v; body=%s", err, response.Body.String())
	}
}

func assertErrorResponse(t *testing.T, response *httptest.ResponseRecorder, code, messagePart string) {
	t.Helper()
	var payload map[string]string
	decodeJSON(t, response, &payload)
	if payload["code"] != code || !strings.Contains(payload["message"], messagePart) {
		t.Fatalf("error payload = %#v, want code %q and message containing %q", payload, code, messagePart)
	}
}
