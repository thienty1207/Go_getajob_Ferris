package httpapi

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/gogetsomefoodferris/backend/internal/service"
	"github.com/google/uuid"
)

func TestClientListPromotionsReturnsPublicStructuredContract(t *testing.T) {
	t.Parallel()
	api := &testPromotionAPI{promotions: []model.Promotion{{
		ID:          uuid.New(),
		Slot:        1,
		ImageURL:    "/api/v1/client/promotions/1/image?v=abc",
		AltText:     "Ferris campaign",
		Title:       stringPointer("Tìm việc phù hợp"),
		ContentHash: "abc",
	}}}
	router := NewRouter(testConfig(), NewHandler(&testScanAPI{}, healthyChecker{}, testConfig()), NewPromotionHandler(api, testConfig()))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/client/promotions", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Promotions []struct {
			Slot        int    `json:"slot"`
			ImageURL    string `json:"image_url"`
			AltText     string `json:"alt_text"`
			Title       string `json:"title"`
			ContentHash string `json:"content_hash"`
		} `json:"promotions"`
	}
	decodeJSON(t, response, &payload)
	if len(payload.Promotions) != 1 || payload.Promotions[0].Slot != 1 || payload.Promotions[0].ImageURL == "" || payload.Promotions[0].Title != "Tìm việc phù hợp" || payload.Promotions[0].ContentHash != "abc" {
		t.Fatalf("payload = %#v, want structured promotion", payload)
	}
	if response.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Fatalf("Cache-Control = %q, want short public cache", response.Header().Get("Cache-Control"))
	}
}

func TestClientListPromotionsCapsUnexpectedRepositoryOutput(t *testing.T) {
	t.Parallel()
	api := &testPromotionAPI{promotions: []model.Promotion{{Slot: 1}, {Slot: 2}, {Slot: 3}, {Slot: 4}}}
	router := NewRouter(testConfig(), NewHandler(&testScanAPI{}, healthyChecker{}, testConfig()), NewPromotionHandler(api, testConfig()))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/client/promotions", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var payload promotionListResponse
	decodeJSON(t, response, &payload)
	if len(payload.Promotions) != 3 {
		t.Fatalf("promotions = %d, want max 3", len(payload.Promotions))
	}
}

func TestClientPromotionImageUsesETagAndReturnsNotModified(t *testing.T) {
	t.Parallel()
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	api := &testPromotionAPI{image: repository.PromotionImage{Slot: 1, ImageBytes: []byte("image-bytes"), MIMEType: "image/png", ContentHash: hash}}
	router := NewRouter(testConfig(), NewHandler(&testScanAPI{}, healthyChecker{}, testConfig()), NewPromotionHandler(api, testConfig()))

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/v1/client/promotions/1/image?v="+hash, nil))
	if first.Code != http.StatusOK || first.Body.String() != "image-bytes" || first.Header().Get("ETag") != `"`+hash+`"` {
		t.Fatalf("first image response = status %d, body %q, etag %q", first.Code, first.Body.String(), first.Header().Get("ETag"))
	}
	if first.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("image Cache-Control = %q", first.Header().Get("Cache-Control"))
	}

	secondRequest := httptest.NewRequest(http.MethodGet, "/api/v1/client/promotions/1/image?v="+hash, nil)
	secondRequest.Header.Set("If-None-Match", `"`+hash+`"`)
	second := httptest.NewRecorder()
	router.ServeHTTP(second, secondRequest)
	if second.Code != http.StatusNotModified || second.Body.Len() != 0 {
		t.Fatalf("conditional response = status %d, body %q, want 304 without body", second.Code, second.Body.String())
	}
}

func TestClientPromotionImageRejectsInvalidStoredIntegrityMetadata(t *testing.T) {
	t.Parallel()
	api := &testPromotionAPI{image: repository.PromotionImage{Slot: 1, ImageBytes: []byte("image-bytes"), MIMEType: "image/png", ContentHash: "not-a-sha"}}
	router := NewRouter(testConfig(), NewHandler(&testScanAPI{}, healthyChecker{}, testConfig()), NewPromotionHandler(api, testConfig()))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/client/promotions/1/image", nil))

	if response.Code != http.StatusInternalServerError || response.Body.String() == "image-bytes" {
		t.Fatalf("status = %d, body=%q, want generic integrity failure", response.Code, response.Body.String())
	}
}

func TestAdminPromotionUploadRequiresAuthenticatedSessionAndCSRF(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	api := &testPromotionAPI{promotion: model.Promotion{Slot: 2, AltText: "Campaign", ImageURL: "/image"}}
	auth := newTestAdminAuth()
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(api, cfg), NewAdminAuthHandler(auth, cfg), nil)
	body, contentType := promotionMultipartBody(t, map[string]string{
		"alt_text": "Campaign",
		"title":    "Title",
	}, "campaign.png", []byte("\x89PNG\r\n\x1a\n"))
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/promotions/2", body)
	request.Header.Set("Content-Type", contentType)
	request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	request.Header.Set("X-CSRF-Token", "csrf-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	var payload map[string]any
	decodeJSON(t, response, &payload)
	if payload["slot"] != float64(2) || payload["alt_text"] != "Campaign" {
		t.Fatalf("payload = %#v", payload)
	}
	if api.upsertInput.Slot != 2 || api.upsertInput.File == nil || api.upsertInput.AltText != "Campaign" || api.upsertCalls != 1 {
		t.Fatalf("upsert input = %#v, calls=%d", api.upsertInput, api.upsertCalls)
	}
}

func TestAdminPromotionUploadRejectsMissingSessionOrWrongCSRF(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	api := &testPromotionAPI{}
	auth := newTestAdminAuth()
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(api, cfg), NewAdminAuthHandler(auth, cfg), nil)

	for _, csrf := range []string{"", "wrong"} {
		request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/promotions/1", nil)
		request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
		request.Header.Set("X-CSRF-Token", csrf)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("csrf %q status = %d, want 403", csrf, response.Code)
		}
	}
	if api.upsertCalls != 0 {
		t.Fatalf("upsert calls = %d, want 0", api.upsertCalls)
	}
}

func TestAdminPromotionUploadRequiresSession(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	api := &testPromotionAPI{}
	auth := newTestAdminAuth()
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(api, cfg), NewAdminAuthHandler(auth, cfg), nil)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/promotions/1", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	assertErrorResponse(t, response, "admin_auth_required", "đăng nhập")
	if api.upsertCalls != 0 {
		t.Fatalf("upsert calls = %d, want 0", api.upsertCalls)
	}
}

func TestAdminPromotionDeleteIsIdempotentAndSeparatedFromClient(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	api := &testPromotionAPI{}
	auth := newTestAdminAuth()
	router := NewAuthenticatedRouter(cfg, NewHandler(&testScanAPI{}, healthyChecker{}, cfg), NewPromotionHandler(api, cfg), NewAdminAuthHandler(auth, cfg), nil)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/promotions/3", nil)
	request.AddCookie(&http.Cookie{Name: "ferris_admin_session", Value: "session-token"})
	request.Header.Set("X-CSRF-Token", "csrf-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent || api.deletedSlot != 3 {
		t.Fatalf("status = %d, deleted slot = %d, want 204/3", response.Code, api.deletedSlot)
	}
	clientResponse := httptest.NewRecorder()
	router.ServeHTTP(clientResponse, httptest.NewRequest(http.MethodDelete, "/api/v1/client/promotions/3", nil))
	if clientResponse.Code != http.StatusNotFound {
		t.Fatalf("client DELETE status = %d, want route separation 404", clientResponse.Code)
	}
}

type testPromotionAPI struct {
	promotions  []model.Promotion
	promotion   model.Promotion
	image       repository.PromotionImage
	listErr     error
	imageErr    error
	upsertErr   error
	deleteErr   error
	upsertInput service.PromotionInput
	upsertCalls int
	deletedSlot int16
}

func (api *testPromotionAPI) List(context.Context) ([]model.Promotion, error) {
	return api.promotions, api.listErr
}

func (api *testPromotionAPI) GetImage(_ context.Context, _ int16) (repository.PromotionImage, error) {
	if api.imageErr != nil {
		return repository.PromotionImage{}, api.imageErr
	}
	return api.image, nil
}

func (api *testPromotionAPI) Upsert(_ context.Context, input service.PromotionInput) (model.Promotion, error) {
	api.upsertCalls++
	api.upsertInput = input
	return api.promotion, api.upsertErr
}

func (api *testPromotionAPI) Delete(_ context.Context, slot int16) error {
	api.deletedSlot = slot
	return api.deleteErr
}

func stringPointer(value string) *string {
	return &value
}

func promotionMultipartBody(t *testing.T, fields map[string]string, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}
