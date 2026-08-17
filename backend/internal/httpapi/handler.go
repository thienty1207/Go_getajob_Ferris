package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/service"
	"github.com/google/uuid"
)

type scanAPI interface {
	Start(context.Context, service.ScanInput) (uuid.UUID, error)
	Get(context.Context, uuid.UUID) (model.Scan, error)
}

type healthChecker interface {
	Ping(context.Context) error
}

// Handler owns the client-facing scan HTTP contract. It does not parse CV
// contents or write scan rows directly; those responsibilities stay behind
// the service and repository interfaces.
type Handler struct {
	scans           scanAPI
	health          healthChecker
	maxRequestBytes int64
}

// NewHandler configures the multipart request ceiling with room for form
// metadata while keeping the actual CV limit enforced by the service.
func NewHandler(scans scanAPI, health healthChecker, cfg config.Config) *Handler {
	maxRequestBytes := cfg.MaxCVBytes + 1024*1024
	if maxRequestBytes <= cfg.MaxCVBytes {
		maxRequestBytes = cfg.MaxCVBytes
	}
	return &Handler{scans: scans, health: health, maxRequestBytes: maxRequestBytes}
}

// CreateScan accepts a CV upload and returns immediately with a processing
// scan ID. The processor may later transition that row to failed or completed.
func (h *Handler) CreateScan(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxRequestBytes)
	multipartFile, file, err := c.Request.FormFile("cv")
	if err != nil {
		if isRequestTooLarge(err) {
			writeError(c, http.StatusRequestEntityTooLarge, "upload_too_large", "Tệp CV vượt quá dung lượng cho phép.")
			return
		}
		writeError(c, http.StatusBadRequest, "invalid_scan_request", "Vui lòng gửi đúng multipart field cv.")
		return
	}
	_ = multipartFile.Close()
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}

	radius, err := parseRadius(c.PostForm("radius_km"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_scan_request", "Bán kính tìm kiếm không hợp lệ.")
		return
	}
	locationID, err := uuid.Parse(strings.TrimSpace(c.PostForm("location_id")))
	if err != nil || locationID == uuid.Nil {
		writeError(c, http.StatusBadRequest, "invalid_scan_request", "Vui lòng chọn location hợp lệ.")
		return
	}

	scanID, err := h.scans.Start(c.Request.Context(), service.ScanInput{
		File:       file,
		LocationID: locationID,
		RadiusKm:   radius,
	})
	if err != nil {
		writeMappedError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, acceptedResponse{ScanID: scanID.String(), Status: "processing"})
}

// GetScan maps the persisted lifecycle state into the stable client response
// shapes expected by the frontend polling flow.
func (h *Handler) GetScan(c *gin.Context) {
	scanID, err := uuid.Parse(c.Param("scan_id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_scan_id", "Scan ID không hợp lệ.")
		return
	}

	scan, err := h.scans.Get(c.Request.Context(), scanID)
	if err != nil {
		writeMappedError(c, err)
		return
	}
	switch {
	case scan.Status.IsProcessing():
		c.JSON(http.StatusOK, processingResponse{ScanID: scan.ID.String(), Status: "processing"})
	case scan.Status == model.StatusFailed:
		c.JSON(http.StatusOK, failedResponse{
			ScanID:  scan.ID.String(),
			Status:  "failed",
			Message: "Matching service không thể hoàn tất việc quét CV.",
		})
	case scan.Status == model.StatusCompleted:
		matches := make([]matchResponse, 0, len(scan.Matches))
		for _, match := range scan.Matches {
			matches = append(matches, mapMatch(match))
		}
		c.JSON(http.StatusOK, completedResponse{ScanID: scan.ID.String(), Status: "completed", Matches: matches})
	default:
		writeError(c, http.StatusInternalServerError, "internal_error", "Service tạm thời không khả dụng.")
	}
}

// Health reports only whether the database dependency answers a ping; it does
// not claim that the parser, crawler, or matcher are configured.
func (h *Handler) Health(c *gin.Context) {
	if h.health == nil {
		writeError(c, http.StatusServiceUnavailable, "database_unavailable", "Service chưa sẵn sàng.")
		return
	}
	if err := h.health.Ping(c.Request.Context()); err != nil {
		writeError(c, http.StatusServiceUnavailable, "database_unavailable", "Service chưa sẵn sàng.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type acceptedResponse struct {
	ScanID string `json:"scan_id"`
	Status string `json:"status"`
}

type processingResponse struct {
	ScanID string `json:"scan_id"`
	Status string `json:"status"`
}

type failedResponse struct {
	ScanID  string `json:"scan_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type completedResponse struct {
	ScanID  string          `json:"scan_id"`
	Status  string          `json:"status"`
	Matches []matchResponse `json:"matches"`
}

type matchResponse struct {
	ID             string          `json:"id"`
	MatchPercent   float64         `json:"match_percent"`
	Title          string          `json:"title"`
	Company        string          `json:"company"`
	Location       string          `json:"location"`
	DistanceKm     *float64        `json:"distance_km,omitempty"`
	EmploymentType string          `json:"employment_type"`
	WorkMode       string          `json:"work_mode"`
	Salary         *salaryResponse `json:"salary,omitempty"`
	SkillTags      []string        `json:"skill_tags"`
	OriginalURL    string          `json:"original_url"`
}

type salaryResponse struct {
	Display  string `json:"display"`
	Currency string `json:"currency,omitempty"`
}

// mapMatch keeps the public job card intentionally small and caps skill tags
// at the three labels the client design displays.
func mapMatch(match model.JobMatch) matchResponse {
	tags := match.SkillTags
	if len(tags) > 3 {
		tags = tags[:3]
	}
	response := matchResponse{
		ID:             match.ID.String(),
		MatchPercent:   match.MatchPercent,
		Title:          match.Title,
		Company:        match.Company,
		Location:       match.Location,
		DistanceKm:     match.DistanceKm,
		EmploymentType: match.EmploymentType,
		WorkMode:       strings.ToLower(match.WorkMode),
		SkillTags:      tags,
		OriginalURL:    match.OriginalURL,
	}
	if match.Salary != nil && strings.TrimSpace(match.Salary.Display) != "" {
		response.Salary = &salaryResponse{Display: match.Salary.Display, Currency: match.Salary.Currency}
	}
	return response
}

// parseRadius rejects non-finite and non-positive values before they enter the
// scan service or database.
func parseRadius(raw string) (float64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, errors.New("missing radius")
	}
	radius, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(radius) || math.IsInf(radius, 0) || radius <= 0 {
		return 0, errors.New("invalid radius")
	}
	return radius, nil
}

func isRequestTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}
