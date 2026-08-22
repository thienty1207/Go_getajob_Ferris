package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

type clientCVAPI interface {
	List(context.Context, uuid.UUID, int, int) ([]model.ClientCVHistoryItem, int, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}

type ClientCVHandler struct {
	cvs clientCVAPI
}

func NewClientCVHandler(cvs clientCVAPI) *ClientCVHandler {
	return &ClientCVHandler{cvs: cvs}
}

type clientCVHistoryResponse struct {
	Items    []clientCVItemResponse `json:"items"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Total    int                    `json:"total"`
}

type clientCVItemResponse struct {
	ScanID     string                     `json:"scan_id"`
	Status     string                     `json:"status"`
	Location   string                     `json:"location"`
	CreatedAt  string                     `json:"created_at"`
	UpdatedAt  string                     `json:"updated_at"`
	MatchCount int                        `json:"match_count"`
	Profile    *structuredProfileResponse `json:"profile,omitempty"`
}

type structuredProfileResponse struct {
	Roles             []string                    `json:"roles"`
	Skills            []string                    `json:"skills"`
	YearsOfExperience float64                     `json:"years_of_experience"`
	Seniority         string                      `json:"seniority"`
	Domains           []string                    `json:"domains"`
	Education         []model.EducationRecord     `json:"education"`
	Certifications    []model.CertificationRecord `json:"certifications"`
}

func (h *ClientCVHandler) List(c *gin.Context) {
	session, ok := ClientSessionFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "client_unauthorized", "Vui lòng đăng nhập.")
		return
	}
	page, pageSize, err := parseClientCVPaging(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_cv_paging", "Thông số lịch sử CV không hợp lệ.")
		return
	}
	items, total, err := h.cvs.List(c.Request.Context(), session.User.ID, pageSize, (page-1)*pageSize)
	if err != nil {
		writeMappedError(c, err)
		return
	}
	responseItems := make([]clientCVItemResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, mapClientCVItem(item))
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, clientCVHistoryResponse{Items: responseItems, Page: page, PageSize: pageSize, Total: total})
}

func (h *ClientCVHandler) Delete(c *gin.Context) {
	session, ok := ClientSessionFromContext(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "client_unauthorized", "Vui lòng đăng nhập.")
		return
	}
	scanID, err := uuid.Parse(strings.TrimSpace(c.Param("scan_id")))
	if err != nil || scanID == uuid.Nil {
		writeError(c, http.StatusBadRequest, "invalid_scan_id", "Mã CV không hợp lệ.")
		return
	}
	if err := h.cvs.Delete(c.Request.Context(), session.User.ID, scanID); err != nil {
		writeMappedError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parseClientCVPaging(c *gin.Context) (int, int, error) {
	page, pageSize := 1, 10
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100000 {
			return 0, 0, strconv.ErrSyntax
		}
		page = parsed
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 10 {
			return 0, 0, strconv.ErrSyntax
		}
		pageSize = parsed
	}
	return page, pageSize, nil
}

func mapClientCVItem(item model.ClientCVHistoryItem) clientCVItemResponse {
	response := clientCVItemResponse{
		ScanID:     item.ScanID.String(),
		Status:     strings.ToLower(string(item.Status)),
		Location:   item.Location,
		CreatedAt:  item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:  item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		MatchCount: item.MatchCount,
	}
	if item.Profile != nil {
		response.Profile = &structuredProfileResponse{
			Roles:             item.Profile.Roles,
			Skills:            item.Profile.Skills,
			YearsOfExperience: item.Profile.YearsOfExperience,
			Seniority:         item.Profile.Seniority,
			Domains:           item.Profile.Domains,
			Education:         item.Profile.Education,
			Certifications:    item.Profile.Certifications,
		}
	}
	return response
}

var _ repository.ClientCVRepository = (*repository.PostgresClientCVRepository)(nil)
