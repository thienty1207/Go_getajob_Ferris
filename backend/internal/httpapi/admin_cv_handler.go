package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

type adminCVAPI interface {
	ListAdminCVProfiles(context.Context, int, int, repository.AdminCVFilter) (model.AdminCVProfilePage, error)
	DeleteAdminCV(context.Context, uuid.UUID) error
}

type AdminCVHandler struct {
	cvs adminCVAPI
}

func NewAdminCVHandler(cvs adminCVAPI) *AdminCVHandler {
	return &AdminCVHandler{cvs: cvs}
}

type adminCVPageResponse struct {
	Items    []adminCVResponse `json:"items"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Total    int               `json:"total"`
}

type adminCVResponse struct {
	ScanID      string                     `json:"scan_id"`
	UserID      string                     `json:"user_id"`
	Email       string                     `json:"email"`
	DisplayName string                     `json:"display_name"`
	Status      string                     `json:"status"`
	Location    string                     `json:"location"`
	CreatedAt   string                     `json:"created_at"`
	UpdatedAt   string                     `json:"updated_at"`
	MatchCount  int                        `json:"match_count"`
	Profile     *structuredProfileResponse `json:"profile,omitempty"`
}

func (h *AdminCVHandler) List(c *gin.Context) {
	page, pageSize, err := parseBoundedPaging(c, "cv")
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_cv_paging", "Thông số phân trang CV không hợp lệ.")
		return
	}
	userSearch := strings.TrimSpace(c.Query("user"))
	roleSearch := strings.TrimSpace(c.Query("role"))
	if len(userSearch) > 200 || len(roleSearch) > 160 {
		writeError(c, http.StatusBadRequest, "invalid_cv_filter", "Bộ lọc CV quá dài.")
		return
	}
	result, err := h.cvs.ListAdminCVProfiles(c.Request.Context(), page, pageSize, repository.AdminCVFilter{User: userSearch, Role: roleSearch})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "Không thể đọc danh sách CV lúc này.")
		return
	}
	items := make([]adminCVResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, mapAdminCV(item))
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, adminCVPageResponse{Items: items, Page: result.Page, PageSize: result.PageSize, Total: result.Total})
}

func (h *AdminCVHandler) Delete(c *gin.Context) {
	scanID, err := uuid.Parse(strings.TrimSpace(c.Param("scan_id")))
	if err != nil || scanID == uuid.Nil {
		writeError(c, http.StatusBadRequest, "invalid_scan_id", "Mã CV không hợp lệ.")
		return
	}
	if err := h.cvs.DeleteAdminCV(c.Request.Context(), scanID); err != nil {
		writeMappedError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func mapAdminCV(item model.AdminCVProfile) adminCVResponse {
	response := adminCVResponse{
		ScanID: item.ScanID.String(), UserID: item.UserID.String(), Email: item.Email, DisplayName: item.DisplayName,
		Status: strings.ToLower(string(item.Status)), Location: item.Location,
		CreatedAt: item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"), MatchCount: item.MatchCount,
	}
	if item.Profile != nil {
		response.Profile = &structuredProfileResponse{Roles: item.Profile.Roles, Skills: item.Profile.Skills, YearsOfExperience: item.Profile.YearsOfExperience, Seniority: item.Profile.Seniority, Domains: item.Profile.Domains, Education: item.Profile.Education, Certifications: item.Profile.Certifications}
	}
	return response
}
