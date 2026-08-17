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

type adminJobLinkAPI interface {
	ListJobLinks(context.Context, int, int) (model.JobLinkPage, error)
	Create(context.Context, service.JobLinkInput) (model.JobLink, error)
	Update(context.Context, uuid.UUID, service.JobLinkInput) (model.JobLink, error)
	SetStatus(context.Context, uuid.UUID, string) error
	Delete(context.Context, uuid.UUID) error
}

type adminJobLinkCrawlAPI interface {
	Request(context.Context, uuid.UUID, string) (model.CrawlRequest, error)
}

type AdminJobLinkHandler struct {
	links adminJobLinkAPI
	crawl adminJobLinkCrawlAPI
}

func NewAdminJobLinkHandler(links adminJobLinkAPI, crawl ...adminJobLinkCrawlAPI) *AdminJobLinkHandler {
	var crawlAPI adminJobLinkCrawlAPI
	if len(crawl) > 0 {
		crawlAPI = crawl[0]
	}
	return &AdminJobLinkHandler{links: links, crawl: crawlAPI}
}

type adminJobLinkPageResponse struct {
	Items    []adminJobLinkResponse `json:"items"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Total    int                    `json:"total"`
}

type adminJobLinkResponse struct {
	ID                       string  `json:"id"`
	URL                      string  `json:"url"`
	SourceKey                string  `json:"source_key"`
	DisplayName              string  `json:"display_name"`
	ApprovalStatus           string  `json:"approval_status"`
	ApprovedAt               *string `json:"approved_at"`
	ApprovedBy               *string `json:"approved_by"`
	CreatedAt                string  `json:"created_at"`
	UpdatedAt                string  `json:"updated_at"`
	LastCrawlStatus          string  `json:"last_crawl_status,omitempty"`
	LastCrawlAt              *string `json:"last_crawl_at,omitempty"`
	ActiveCrawlRequestID     string  `json:"active_crawl_request_id,omitempty"`
	ActiveCrawlRequestStatus string  `json:"active_crawl_request_status,omitempty"`
	LastCrawlPages           *int    `json:"last_crawl_pages,omitempty"`
	LastCrawlJobs            *int    `json:"last_crawl_jobs,omitempty"`
	LastCrawlCreated         *int    `json:"last_crawl_created,omitempty"`
	LastCrawlUpdated         *int    `json:"last_crawl_updated,omitempty"`
	LastCrawlMissing         *int    `json:"last_crawl_missing,omitempty"`
	LastCrawlErrorCode       string  `json:"last_crawl_error_code,omitempty"`
}

type adminCrawlRequestResponse struct {
	ID          string  `json:"id"`
	SourceID    string  `json:"source_id"`
	Status      string  `json:"status"`
	RequestedBy string  `json:"requested_by"`
	RequestedAt string  `json:"requested_at"`
	StartedAt   *string `json:"started_at,omitempty"`
	FinishedAt  *string `json:"finished_at,omitempty"`
	SourceRunID *string `json:"source_run_id,omitempty"`
	ErrorCode   *string `json:"error_code,omitempty"`
}

type adminJobLinkRequest struct {
	URL string `json:"url"`
}

type adminJobLinkStatusRequest struct {
	ApprovalStatus string `json:"approval_status"`
}

func (h *AdminJobLinkHandler) List(c *gin.Context) {
	page, pageSize, err := parseAdminJobLinkPaging(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_job_link_paging", "Thông số phân trang link không hợp lệ.")
		return
	}
	result, err := h.links.ListJobLinks(c.Request.Context(), page, pageSize)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "Không thể đọc danh sách Job Link lúc này.")
		return
	}
	items := make([]adminJobLinkResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, mapAdminJobLink(item))
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, adminJobLinkPageResponse{Items: items, Page: result.Page, PageSize: result.PageSize, Total: result.Total})
}

func (h *AdminJobLinkHandler) Create(c *gin.Context) {
	request, ok := decodeAdminJobLinkRequest(c)
	if !ok {
		return
	}
	actor, ok := adminActorEmail(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "admin_auth_required", "Phiên quản trị không còn hợp lệ.")
		return
	}
	link, err := h.links.Create(c.Request.Context(), service.JobLinkInput{URL: request.URL, ApprovedBy: actor})
	if err != nil {
		writeMappedError(c, err)
		return
	}
	c.JSON(http.StatusCreated, mapAdminJobLink(link))
}

func (h *AdminJobLinkHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_job_link_id", "Job Link không hợp lệ.")
		return
	}
	request, ok := decodeAdminJobLinkRequest(c)
	if !ok {
		return
	}
	actor, ok := adminActorEmail(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "admin_auth_required", "Phiên quản trị không còn hợp lệ.")
		return
	}
	link, err := h.links.Update(c.Request.Context(), id, service.JobLinkInput{URL: request.URL, ApprovedBy: actor})
	if err != nil {
		writeMappedError(c, err)
		return
	}
	c.JSON(http.StatusOK, mapAdminJobLink(link))
}

func (h *AdminJobLinkHandler) SetStatus(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_job_link_id", "Job Link không hợp lệ.")
		return
	}
	request, ok := decodeAdminJobLinkStatusRequest(c)
	if !ok {
		return
	}
	if err := h.links.SetStatus(c.Request.Context(), id, request.ApprovalStatus); err != nil {
		writeMappedError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AdminJobLinkHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_job_link_id", "Job Link không hợp lệ.")
		return
	}
	if err := h.links.Delete(c.Request.Context(), id); err != nil {
		writeMappedError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AdminJobLinkHandler) Crawl(c *gin.Context) {
	if h.crawl == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "Crawl service tạm thời không khả dụng.")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_job_link_id", "Job Link không hợp lệ.")
		return
	}
	actor, ok := adminActorEmail(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "admin_auth_required", "Phiên quản trị không còn hợp lệ.")
		return
	}
	request, err := h.crawl.Request(c.Request.Context(), id, actor)
	if err != nil {
		writeMappedError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusAccepted, mapAdminCrawlRequest(request))
}

func decodeAdminJobLinkRequest(c *gin.Context) (adminJobLinkRequest, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	var request adminJobLinkRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_job_link", "Vui lòng gửi đúng URL Job Link.")
		return adminJobLinkRequest{}, false
	}
	return request, true
}

func decodeAdminJobLinkStatusRequest(c *gin.Context) (adminJobLinkStatusRequest, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4*1024)
	var request adminJobLinkStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_job_link_status", "Vui lòng chọn trạng thái Job Link.")
		return adminJobLinkStatusRequest{}, false
	}
	return request, true
}

func adminActorEmail(c *gin.Context) (string, bool) {
	session, ok := AdminSessionFromContext(c)
	if !ok || strings.TrimSpace(session.User.Email) == "" {
		return "", false
	}
	return session.User.Email, true
}

func parseAdminJobLinkPaging(c *gin.Context) (int, int, error) {
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

func mapAdminJobLink(link model.JobLink) adminJobLinkResponse {
	response := adminJobLinkResponse{
		ID:             link.ID.String(),
		URL:            link.URL,
		SourceKey:      link.SourceKey,
		DisplayName:    link.DisplayName,
		ApprovalStatus: link.ApprovalStatus,
		ApprovedBy:     link.ApprovedBy,
		CreatedAt:      link.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      link.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if link.ApprovedAt != nil {
		formatted := link.ApprovedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		response.ApprovedAt = &formatted
	}
	if link.LastCrawlStatus != nil {
		response.LastCrawlStatus = *link.LastCrawlStatus
	}
	if link.LastCrawlAt != nil {
		formatted := link.LastCrawlAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		response.LastCrawlAt = &formatted
	}
	if link.ActiveCrawlRequestID != nil {
		response.ActiveCrawlRequestID = link.ActiveCrawlRequestID.String()
	}
	if link.ActiveCrawlRequestStatus != nil {
		response.ActiveCrawlRequestStatus = *link.ActiveCrawlRequestStatus
	}
	if link.LastCrawlErrorCode != nil {
		response.LastCrawlErrorCode = *link.LastCrawlErrorCode
	}
	if link.LastCrawlStatus != nil {
		pages := link.LastCrawlPages
		jobs := link.LastCrawlJobs
		created := link.LastCrawlCreated
		updated := link.LastCrawlUpdated
		missing := link.LastCrawlMissing
		response.LastCrawlPages = &pages
		response.LastCrawlJobs = &jobs
		response.LastCrawlCreated = &created
		response.LastCrawlUpdated = &updated
		response.LastCrawlMissing = &missing
	}
	return response
}

func mapAdminCrawlRequest(request model.CrawlRequest) adminCrawlRequestResponse {
	response := adminCrawlRequestResponse{
		ID:          request.ID.String(),
		SourceID:    request.SourceID.String(),
		Status:      request.Status,
		RequestedBy: request.RequestedBy,
		RequestedAt: request.RequestedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		ErrorCode:   request.ErrorCode,
	}
	if request.StartedAt != nil {
		value := request.StartedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		response.StartedAt = &value
	}
	if request.FinishedAt != nil {
		value := request.FinishedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		response.FinishedAt = &value
	}
	if request.SourceRunID != nil {
		value := request.SourceRunID.String()
		response.SourceRunID = &value
	}
	return response
}

var _ repository.JobLinkRepository = (*repository.PostgresJobLinkRepository)(nil)
