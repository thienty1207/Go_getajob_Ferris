package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/model"
)

type adminSettingsAPI interface {
	GetCrawlerSettings(context.Context) (model.CrawlerSettings, error)
	GetCrawlerRuntime(context.Context) (model.CrawlerRuntime, error)
	UpdateCrawlerSettings(context.Context, int, int, string) (model.CrawlerSettings, error)
}

type AdminSettingsHandler struct {
	settings adminSettingsAPI
}

func NewAdminSettingsHandler(settings adminSettingsAPI) *AdminSettingsHandler {
	return &AdminSettingsHandler{settings: settings}
}

type adminSettingsResponse struct {
	Crawler model.CrawlerSettings `json:"crawler"`
	Runtime model.CrawlerRuntime  `json:"runtime"`
}

type adminCrawlerSettingsRequest struct {
	IntervalHours   int `json:"interval_hours"`
	IntervalMinutes int `json:"interval_minutes"`
}

func (h *AdminSettingsHandler) Get(c *gin.Context) {
	settings, err := h.settings.GetCrawlerSettings(c.Request.Context())
	if err != nil {
		writeMappedError(c, err)
		return
	}
	runtime, err := h.settings.GetCrawlerRuntime(c.Request.Context())
	if err != nil {
		writeMappedError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, adminSettingsResponse{Crawler: settings, Runtime: runtime})
}

func (h *AdminSettingsHandler) UpdateCrawler(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4*1024)
	var request adminCrawlerSettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_crawler_settings", "Khoảng thời gian crawl chưa hợp lệ.")
		return
	}
	actor, ok := adminActorEmail(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "admin_auth_required", "Phiên quản trị không còn hợp lệ.")
		return
	}
	settings, err := h.settings.UpdateCrawlerSettings(c.Request.Context(), request.IntervalHours, request.IntervalMinutes, strings.TrimSpace(actor))
	if err != nil {
		writeMappedError(c, err)
		return
	}
	runtime, err := h.settings.GetCrawlerRuntime(c.Request.Context())
	if err != nil {
		writeMappedError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, adminSettingsResponse{Crawler: settings, Runtime: runtime})
}
