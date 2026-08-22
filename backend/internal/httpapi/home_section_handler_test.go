package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/service"
	"github.com/google/uuid"
)

func TestPublicHomeSectionsReturnsCloudinaryMetadataWithoutStorageIdentifiers(t *testing.T) {
	sectionAPI := &testHomeSectionAPI{sections: []model.HomeSection{{
		ID: uuid.New(), Slot: 1, Layout: model.HomeSectionContentLeft, IsActive: true,
		Title: "Nội dung thật", Body: "Được đọc từ API", ImageAltText: "Ảnh Home",
		ImageURL:           "https://res.cloudinary.com/example/image/upload/home.png",
		ImageContentHash:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CloudinaryPublicID: "ferris/home-sections/slot-1-hash", CloudinaryAssetID: "asset-secret-like-id",
	}}}
	handler := NewHandler(&testScanAPI{}, healthyChecker{}, testConfig())
	handler.SetHomeSectionHandler(NewHomeSectionHandler(sectionAPI, 1024))
	router := NewRouter(testConfig(), handler)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/client/home-sections", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Cache-Control"); got != "public, max-age=60" {
		t.Fatalf("Cache-Control = %q, want public cache policy", got)
	}
	if body := response.Body.String(); body == "" || strings.Contains(body, "ferris/home-sections/slot-1-hash") || strings.Contains(body, "asset-secret-like-id") {
		t.Fatalf("public response unexpectedly exposes storage identifiers: %s", body)
	}
	if sectionAPI.listCalls != 1 || sectionAPI.lastPublicOnly != true {
		t.Fatalf("List calls=%d publicOnly=%v, want one public read", sectionAPI.listCalls, sectionAPI.lastPublicOnly)
	}
}

type testHomeSectionAPI struct {
	sections       []model.HomeSection
	listCalls      int
	lastPublicOnly bool
}

func (api *testHomeSectionAPI) List(_ context.Context, publicOnly bool) ([]model.HomeSection, error) {
	api.listCalls++
	api.lastPublicOnly = publicOnly
	return api.sections, nil
}

func (*testHomeSectionAPI) Upsert(context.Context, service.HomeSectionInput) (model.HomeSection, error) {
	return model.HomeSection{}, nil
}

func (*testHomeSectionAPI) CreateMedia(context.Context, service.HomeSectionMediaInput) (model.HomeSectionMedia, error) {
	return model.HomeSectionMedia{}, nil
}

func (*testHomeSectionAPI) UpdateMedia(context.Context, uuid.UUID, *int16, *bool, *string, *string) (model.HomeSectionMedia, error) {
	return model.HomeSectionMedia{}, nil
}

func (*testHomeSectionAPI) DeleteMedia(context.Context, uuid.UUID) error { return nil }
