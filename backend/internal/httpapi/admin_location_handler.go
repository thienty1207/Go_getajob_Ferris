package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/gogetsomefoodferris/backend/internal/service"
	"github.com/google/uuid"
)

type adminLocationAPI interface {
	List(context.Context, int, int) (model.AdminLocationPage, error)
	Options(context.Context) ([]model.AdminLocationOption, error)
	ListActive(context.Context) ([]model.ClientLocation, error)
	Create(context.Context, service.LocationInput) (model.AdminLocation, error)
	Update(context.Context, uuid.UUID, service.LocationInput) (model.AdminLocation, error)
	AssignJobLocation(context.Context, uuid.UUID, *uuid.UUID) error
}

type AdminLocationHandler struct {
	locations adminLocationAPI
}

func NewAdminLocationHandler(locations adminLocationAPI) *AdminLocationHandler {
	return &AdminLocationHandler{locations: locations}
}

type adminLocationResponse struct {
	ID           string   `json:"id"`
	DisplayName  string   `json:"display_name"`
	Province     string   `json:"province"`
	Country      string   `json:"country"`
	CanonicalKey string   `json:"canonical_key"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
	IsActive     bool     `json:"is_active"`
	JobCount     int      `json:"job_count"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type adminLocationListResponse struct {
	Items    []adminLocationResponse `json:"items"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
	Total    int                     `json:"total"`
}

type adminLocationOptionsResponse struct {
	Items []adminLocationOptionResponse `json:"items"`
}

type adminLocationOptionResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	IsActive    bool   `json:"is_active"`
}

type clientLocationResponse struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	Province    string   `json:"province"`
	Country     string   `json:"country"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
}

type clientLocationListResponse struct {
	Items []clientLocationResponse `json:"items"`
}

type adminLocationRequest struct {
	DisplayName string `json:"display_name"`
	Province    string `json:"province"`
	Country     string `json:"country"`
	IsActive    *bool  `json:"is_active"`
}

type adminJobLocationRequest struct {
	LocationID *string `json:"location_id"`
}

func (h *AdminLocationHandler) List(c *gin.Context) {
	page, pageSize, err := parseAdminLocationPaging(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_location_paging", "Thông số phân trang location không hợp lệ.")
		return
	}
	result, err := h.locations.List(c.Request.Context(), page, pageSize)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "Không thể đọc danh sách location lúc này.")
		return
	}
	items := make([]adminLocationResponse, 0, len(result.Items))
	for _, location := range result.Items {
		items = append(items, mapAdminLocation(location))
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, adminLocationListResponse{Items: items, Page: result.Page, PageSize: result.PageSize, Total: result.Total})
}

func (h *AdminLocationHandler) Options(c *gin.Context) {
	locations, err := h.locations.Options(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "Không thể đọc location options lúc này.")
		return
	}
	items := make([]adminLocationOptionResponse, 0, len(locations))
	for _, location := range locations {
		items = append(items, adminLocationOptionResponse{ID: location.ID.String(), DisplayName: location.DisplayName, IsActive: location.IsActive})
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, adminLocationOptionsResponse{Items: items})
}

func parseAdminLocationPaging(c *gin.Context) (int, int, error) {
	page, pageSize := 1, repository.AdminPageSize
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100000 {
			return 0, 0, errors.New("invalid page")
		}
		page = parsed
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > repository.AdminPageSize {
			return 0, 0, errors.New("invalid page size")
		}
		pageSize = parsed
	}
	return page, pageSize, nil
}

func (h *AdminLocationHandler) ListPublic(c *gin.Context) {
	locations, err := h.locations.ListActive(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "Không thể đọc danh sách location lúc này.")
		return
	}
	items := make([]clientLocationResponse, 0, len(locations))
	for _, location := range locations {
		items = append(items, clientLocationResponse{
			ID:          location.ID.String(),
			DisplayName: location.DisplayName,
			Province:    location.Province,
			Country:     location.Country,
			Latitude:    location.Latitude,
			Longitude:   location.Longitude,
		})
	}
	c.Header("Cache-Control", "public, max-age=60")
	c.JSON(http.StatusOK, clientLocationListResponse{Items: items})
}

func (h *AdminLocationHandler) Create(c *gin.Context) {
	request, ok := decodeAdminLocationRequest(c)
	if !ok {
		return
	}
	location, err := h.locations.Create(c.Request.Context(), toLocationInput(request))
	if err != nil {
		writeMappedError(c, err)
		return
	}
	c.JSON(http.StatusCreated, mapAdminLocation(location))
}

func (h *AdminLocationHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_location_id", "Location không hợp lệ.")
		return
	}
	request, ok := decodeAdminLocationRequest(c)
	if !ok {
		return
	}
	location, err := h.locations.Update(c.Request.Context(), id, toLocationInput(request))
	if err != nil {
		writeMappedError(c, err)
		return
	}
	c.JSON(http.StatusOK, mapAdminLocation(location))
}

func (h *AdminLocationHandler) AssignJobLocation(c *gin.Context) {
	jobID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_job_id", "Job không hợp lệ.")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4*1024)
	var request adminJobLocationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_location_id", "Vui lòng chọn location.")
		return
	}
	var locationID *uuid.UUID
	if request.LocationID != nil {
		parsed, err := uuid.Parse(strings.TrimSpace(*request.LocationID))
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid_location_id", "Location không hợp lệ.")
			return
		}
		locationID = &parsed
	}
	if err := h.locations.AssignJobLocation(c.Request.Context(), jobID, locationID); err != nil {
		writeMappedError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func decodeAdminLocationRequest(c *gin.Context) (adminLocationRequest, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	var request adminLocationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_location", "Vui lòng gửi đúng thông tin location.")
		return adminLocationRequest{}, false
	}
	return request, true
}

func toLocationInput(request adminLocationRequest) service.LocationInput {
	isActive := true
	if request.IsActive != nil {
		isActive = *request.IsActive
	}
	return service.LocationInput{DisplayName: request.DisplayName, Province: request.Province, Country: request.Country, IsActive: isActive}
}

func mapAdminLocation(location model.AdminLocation) adminLocationResponse {
	return adminLocationResponse{
		ID:           location.ID.String(),
		DisplayName:  location.DisplayName,
		Province:     location.Province,
		Country:      location.Country,
		CanonicalKey: location.CanonicalKey,
		Latitude:     location.Latitude,
		Longitude:    location.Longitude,
		IsActive:     location.IsActive,
		JobCount:     location.JobCount,
		CreatedAt:    location.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    location.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

var _ repository.LocationRepository = (*repository.PostgresLocationRepository)(nil)
