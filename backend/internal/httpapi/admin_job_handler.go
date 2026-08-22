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
	"github.com/google/uuid"
)

type adminJobAPI interface {
	ListAdminJobs(context.Context, int, int, repository.AdminJobFilter) (model.AdminJobPage, error)
}

// AdminJobHandler exposes the operational job cache without exposing raw
// descriptions. It is intentionally read-only in this first admin slice.
type AdminJobHandler struct {
	jobs adminJobAPI
}

func NewAdminJobHandler(jobs adminJobAPI) *AdminJobHandler {
	return &AdminJobHandler{jobs: jobs}
}

type adminJobPageResponse struct {
	Items    []adminJobResponse `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Total    int                `json:"total"`
}

type adminJobResponse struct {
	ID                   string   `json:"id"`
	SourceKey            string   `json:"source_key"`
	SourceName           string   `json:"source_name"`
	SourceApprovalStatus string   `json:"source_approval_status"`
	IsDevelopmentFixture bool     `json:"is_development_fixture"`
	Title                string   `json:"title"`
	Company              string   `json:"company"`
	Location             string   `json:"location"`
	LocationID           *string  `json:"location_id,omitempty"`
	LocationAssignmentSource string `json:"location_assignment_source"`
	Role                 string   `json:"role"`
	RequiredSkills       []string `json:"required_skills"`
	PreferredSkills      []string `json:"preferred_skills"`
	Seniority            string   `json:"seniority"`
	MinimumExperience    *float64 `json:"minimum_experience_years,omitempty"`
	Domains              []string `json:"domains"`
	EmploymentType       string   `json:"employment_type"`
	WorkMode             string   `json:"work_mode"`
	Status               string   `json:"status"`
	OriginalURL          string   `json:"original_url"`
	ContentHash          string   `json:"content_hash"`
	LastSeenAt           string   `json:"last_seen_at"`
	UpdatedAt            string   `json:"updated_at"`
}

// List validates bounded page parameters before asking PostgreSQL for rows.
// Returning an explicit page object lets the frontend distinguish empty data
// from an API failure and gives operators stable pagination semantics.
func (h *AdminJobHandler) List(c *gin.Context) {
	page, pageSize, err := parseAdminJobPaging(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_job_paging", "Thông số phân trang job không hợp lệ.")
		return
	}
	filter, err := parseAdminJobFilter(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_job_filter", "Bộ lọc job không hợp lệ.")
		return
	}
	result, err := h.jobs.ListAdminJobs(c.Request.Context(), page, pageSize, filter)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "Không thể đọc danh sách job lúc này.")
		return
	}
	items := make([]adminJobResponse, 0, len(result.Items))
	for _, job := range result.Items {
		items = append(items, mapAdminJob(job))
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, adminJobPageResponse{Items: items, Page: result.Page, PageSize: result.PageSize, Total: result.Total})
}

func parseAdminJobFilter(c *gin.Context) (repository.AdminJobFilter, error) {
	var filter repository.AdminJobFilter
	filter.Search = strings.TrimSpace(c.Query("q"))
	if len(filter.Search) > 200 {
		return repository.AdminJobFilter{}, errors.New("search is too long")
	}
	if raw := strings.TrimSpace(c.Query("location_id")); raw != "" {
		locationID, err := uuid.Parse(raw)
		if err != nil {
			return repository.AdminJobFilter{}, errors.New("invalid location id")
		}
		filter.LocationID = &locationID
	}
	if raw := strings.TrimSpace(c.Query("unresolved")); raw != "" {
		unresolved, err := strconv.ParseBool(raw)
		if err != nil {
			return repository.AdminJobFilter{}, errors.New("invalid unresolved flag")
		}
		filter.UnresolvedLocation = unresolved
	}
	if filter.LocationID != nil && filter.UnresolvedLocation {
		return repository.AdminJobFilter{}, errors.New("conflicting location filters")
	}
	return filter, nil
}

func parseAdminJobPaging(c *gin.Context) (int, int, error) {
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

func mapAdminJob(job model.AdminJob) adminJobResponse {
	var locationID *string
	if job.LocationID != nil {
		value := job.LocationID.String()
		locationID = &value
	}
	return adminJobResponse{
		ID:                   job.ID.String(),
		SourceKey:            job.SourceKey,
		SourceName:           job.SourceName,
		SourceApprovalStatus: job.SourceApprovalStatus,
		IsDevelopmentFixture: job.SourceKey == "development-fixture",
		Title:                job.Title,
		Company:              job.Company,
		Location:             job.Location,
		LocationID:           locationID,
		LocationAssignmentSource: job.LocationAssignmentSource,
		Role:                 job.Role,
		RequiredSkills:       job.RequiredSkills,
		PreferredSkills:      job.PreferredSkills,
		Seniority:            job.Seniority,
		MinimumExperience:    job.MinimumExperience,
		Domains:              job.Domains,
		EmploymentType:       job.EmploymentType,
		WorkMode:             strings.ToLower(job.WorkMode),
		Status:               strings.ToLower(job.Status),
		OriginalURL:          job.OriginalURL,
		ContentHash:          job.ContentHash,
		LastSeenAt:           job.LastSeenAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:            job.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

var _ repository.JobRepository = (*repository.PostgresJobRepository)(nil)
