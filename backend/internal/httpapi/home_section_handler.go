package httpapi

import (
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/service"
	"github.com/google/uuid"
)

type homeSectionAPI interface {
	List(context.Context, bool) ([]model.HomeSection, error)
	Upsert(context.Context, service.HomeSectionInput) (model.HomeSection, error)
	CreateMedia(context.Context, service.HomeSectionMediaInput) (model.HomeSectionMedia, error)
	UpdateMedia(context.Context, uuid.UUID, *int16, *bool, *string, *string) (model.HomeSectionMedia, error)
	DeleteMedia(context.Context, uuid.UUID) error
}

type HomeSectionHandler struct {
	sections        homeSectionAPI
	maxRequestBytes int64
}

func NewHomeSectionHandler(sections homeSectionAPI, maxImageBytes int64) *HomeSectionHandler {
	if maxImageBytes <= 0 {
		maxImageBytes = 5 * 1024 * 1024
	}
	return &HomeSectionHandler{sections: sections, maxRequestBytes: maxImageBytes + 1024*1024}
}

func (h *HomeSectionHandler) ListPublic(c *gin.Context) {
	h.list(c, true)
}

func (h *HomeSectionHandler) ListAdmin(c *gin.Context) {
	h.list(c, false)
}

func (h *HomeSectionHandler) list(c *gin.Context, publicOnly bool) {
	sections, err := h.sections.List(c.Request.Context(), publicOnly)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "Không thể đọc nội dung Home lúc này.")
		return
	}
	items := make([]homeSectionResponse, 0, len(sections))
	for _, section := range sections {
		items = append(items, mapHomeSection(section))
	}
	if publicOnly {
		c.Header("Cache-Control", "public, max-age=60")
	} else {
		c.Header("Cache-Control", "no-store")
	}
	c.JSON(http.StatusOK, gin.H{"sections": items})
}

func (h *HomeSectionHandler) Upsert(c *gin.Context) {
	slot, err := parseHomeSectionSlot(c.Param("slot"))
	if err != nil || slot == 4 {
		writeError(c, http.StatusBadRequest, "invalid_home_section", "Section Home không hợp lệ.")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxRequestBytes)
	file, fileHeader, err := h.optionalImage(c)
	if err != nil {
		writeMappedError(c, err)
		return
	}
	if file != nil {
		_ = file.Close()
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	isActive, err := parseOptionalBool(c.PostForm("is_active"), true)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_home_section", "Trạng thái section không hợp lệ.")
		return
	}
	actor, _ := adminActorEmail(c)
	section, err := h.sections.Upsert(c.Request.Context(), service.HomeSectionInput{
		Slot: slot, IsActive: isActive, Eyebrow: c.PostForm("eyebrow"), Title: c.PostForm("title"),
		Body: c.PostForm("body"), ImageAltText: c.PostForm("image_alt_text"), TargetURL: c.PostForm("target_url"),
		File: fileHeader, UpdatedBy: strings.TrimSpace(actor),
	})
	if err != nil {
		writeMappedError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, mapHomeSection(section))
}

func (h *HomeSectionHandler) CreateMedia(c *gin.Context) {
	if c.Param("slot") != "4" {
		writeError(c, http.StatusBadRequest, "invalid_home_section", "Media strip chỉ thuộc section 4.")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxRequestBytes)
	file, fileHeader, err := h.requiredImage(c)
	if err != nil {
		writeMappedError(c, err)
		return
	}
	_ = file.Close()
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	sortOrder, err := strconv.ParseInt(strings.TrimSpace(c.PostForm("sort_order")), 10, 16)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_home_media", "Thứ tự ảnh không hợp lệ.")
		return
	}
	isActive, err := parseOptionalBool(c.PostForm("is_active"), true)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_home_media", "Trạng thái ảnh không hợp lệ.")
		return
	}
	media, err := h.sections.CreateMedia(c.Request.Context(), service.HomeSectionMediaInput{
		SortOrder: int16(sortOrder), IsActive: isActive, ImageAltText: c.PostForm("image_alt_text"),
		TargetURL: c.PostForm("target_url"), File: fileHeader,
	})
	if err != nil {
		writeMappedError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, mapHomeSectionMedia(media))
}

func (h *HomeSectionHandler) UpdateMedia(c *gin.Context) {
	mediaID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || mediaID == uuid.Nil {
		writeError(c, http.StatusBadRequest, "invalid_home_media", "Ảnh section không hợp lệ.")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8*1024)
	var input homeSectionMediaUpdateRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_home_media", "Thông tin ảnh section chưa hợp lệ.")
		return
	}
	media, err := h.sections.UpdateMedia(c.Request.Context(), mediaID, input.SortOrder, input.IsActive, input.ImageAltText, input.TargetURL)
	if err != nil {
		writeMappedError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, mapHomeSectionMedia(media))
}

func (h *HomeSectionHandler) DeleteMedia(c *gin.Context) {
	mediaID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || mediaID == uuid.Nil {
		writeError(c, http.StatusBadRequest, "invalid_home_media", "Ảnh section không hợp lệ.")
		return
	}
	if err := h.sections.DeleteMedia(c.Request.Context(), mediaID); err != nil {
		writeMappedError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type homeSectionMediaUpdateRequest struct {
	SortOrder    *int16  `json:"sort_order"`
	IsActive     *bool   `json:"is_active"`
	ImageAltText *string `json:"image_alt_text"`
	TargetURL    *string `json:"target_url"`
}

type homeSectionResponse struct {
	ID               string                     `json:"id,omitempty"`
	Slot             int16                      `json:"slot"`
	Layout           string                     `json:"layout"`
	IsActive         bool                       `json:"is_active"`
	Eyebrow          string                     `json:"eyebrow,omitempty"`
	Title            string                     `json:"title,omitempty"`
	Body             string                     `json:"body,omitempty"`
	ImageAltText     string                     `json:"image_alt_text,omitempty"`
	ImageURL         string                     `json:"image_url,omitempty"`
	ImageContentHash string                     `json:"image_content_hash,omitempty"`
	TargetURL        string                     `json:"target_url,omitempty"`
	Media            []homeSectionMediaResponse `json:"media"`
}

type homeSectionMediaResponse struct {
	ID               string `json:"id"`
	SortOrder        int16  `json:"sort_order"`
	IsActive         bool   `json:"is_active"`
	ImageAltText     string `json:"image_alt_text"`
	ImageURL         string `json:"image_url"`
	ImageContentHash string `json:"image_content_hash"`
	TargetURL        string `json:"target_url,omitempty"`
}

func mapHomeSection(section model.HomeSection) homeSectionResponse {
	response := homeSectionResponse{Slot: section.Slot, Layout: string(section.Layout), IsActive: section.IsActive, Eyebrow: section.Eyebrow, Title: section.Title, Body: section.Body, ImageAltText: section.ImageAltText, ImageURL: section.ImageURL, ImageContentHash: section.ImageContentHash, TargetURL: section.TargetURL, Media: make([]homeSectionMediaResponse, 0, len(section.Media))}
	if section.ID != uuid.Nil {
		response.ID = section.ID.String()
	}
	for _, media := range section.Media {
		response.Media = append(response.Media, mapHomeSectionMedia(media))
	}
	return response
}

func mapHomeSectionMedia(media model.HomeSectionMedia) homeSectionMediaResponse {
	return homeSectionMediaResponse{ID: media.ID.String(), SortOrder: media.SortOrder, IsActive: media.IsActive, ImageAltText: media.ImageAltText, ImageURL: media.ImageURL, ImageContentHash: media.ImageContentHash, TargetURL: media.TargetURL}
}

func parseHomeSectionSlot(raw string) (int16, error) {
	slot, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 16)
	if err != nil || slot < 1 || slot > 4 {
		return 0, service.ErrInvalidHomeSection
	}
	return int16(slot), nil
}

func parseOptionalBool(raw string, fallback bool) (bool, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	return value, err
}

func (h *HomeSectionHandler) optionalImage(c *gin.Context) (interface{ Close() error }, *multipart.FileHeader, error) {
	file, header, err := c.Request.FormFile("image")
	if errors.Is(err, http.ErrMissingFile) {
		return nil, nil, nil
	}
	if err != nil {
		if isRequestTooLarge(err) {
			return nil, nil, service.ErrPromotionImageTooLarge
		}
		return nil, nil, service.ErrInvalidHomeSection
	}
	return file, header, nil
}

func (h *HomeSectionHandler) requiredImage(c *gin.Context) (interface{ Close() error }, *multipart.FileHeader, error) {
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		if isRequestTooLarge(err) {
			return nil, nil, service.ErrPromotionImageTooLarge
		}
		return nil, nil, service.ErrInvalidHomeMedia
	}
	return file, header, nil
}
