package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/gogetsomefoodferris/backend/internal/service"
)

type promotionAPI interface {
	List(context.Context) ([]model.Promotion, error)
	GetImage(context.Context, int16) (repository.PromotionImage, error)
	Upsert(context.Context, service.PromotionInput) (model.Promotion, error)
	Delete(context.Context, int16) error
}

// PromotionHandler translates the promotion service boundary into two
// deliberately separate HTTP surfaces: public client reads and session/CSRF-
// gated admin writes.
type PromotionHandler struct {
	promotions      promotionAPI
	maxRequestBytes int64
	logger          *slog.Logger
}

// NewPromotionHandler configures request-size protection in front of the
// service's image-size validation. The extra margin allows multipart metadata
// while keeping the body bounded.
func NewPromotionHandler(promotions promotionAPI, cfg config.Config) *PromotionHandler {
	maxImageBytes := cfg.MaxPromotionImageBytes
	if maxImageBytes <= 0 {
		maxImageBytes = 5 * 1024 * 1024
	}
	maxRequestBytes := maxImageBytes + 1024*1024
	if maxRequestBytes < maxImageBytes {
		maxRequestBytes = maxImageBytes
	}
	return &PromotionHandler{
		promotions:      promotions,
		maxRequestBytes: maxRequestBytes,
		logger:          slog.Default(),
	}
}

// List returns metadata only. Keeping image bytes out of this response makes
// the carousel cheap to refresh and preserves the dedicated image cache path.
func (h *PromotionHandler) List(c *gin.Context) {
	h.list(c, true)
}

// ListAdmin returns the same bounded metadata but disables browser caching so
// an operator sees the current database state after an upload/delete action.
func (h *PromotionHandler) ListAdmin(c *gin.Context) {
	h.list(c, false)
}

func (h *PromotionHandler) list(c *gin.Context, publicCache bool) {
	promotions, err := h.promotions.List(c.Request.Context())
	if err != nil {
		h.logger.Error("promotion list failed", "action", "list", "result", "error", "error_code", "internal_error")
		writeError(c, http.StatusInternalServerError, "internal_error", "Service tạm thời không khả dụng.")
		return
	}
	if len(promotions) > 3 {
		promotions = promotions[:3]
	}
	items := make([]promotionResponse, 0, len(promotions))
	for _, promotion := range promotions {
		items = append(items, mapPromotion(promotion))
	}
	if publicCache {
		c.Header("Cache-Control", "public, max-age=60")
	} else {
		c.Header("Cache-Control", "no-store")
	}
	c.JSON(http.StatusOK, promotionListResponse{Promotions: items})
}

// Image serves one validated binary and attaches a content-derived ETag. A
// client that already has the same hash receives 304 without another payload.
func (h *PromotionHandler) Image(c *gin.Context) {
	slot, err := parsePromotionSlot(c.Param("slot"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_promotion_slot", "Vị trí quảng bá không hợp lệ.")
		return
	}
	image, err := h.promotions.GetImage(c.Request.Context(), slot)
	if err != nil {
		writeMappedError(c, err)
		return
	}
	if !isValidPromotionHash(image.ContentHash) {
		h.logger.Error("promotion image failed integrity checks", "action", "image", "result", "error", "slot", slot)
		writeError(c, http.StatusInternalServerError, "internal_error", "Service tạm thời không khả dụng.")
		return
	}

	etag := `"` + image.ContentHash + `"`
	c.Header("ETag", etag)
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	if etagMatches(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return
	}
	if image.StorageProvider == "CLOUDINARY" {
		if !isValidPromotionCloudinaryURL(image.CloudinarySecureURL) {
			h.logger.Error("promotion Cloudinary URL failed integrity checks", "action", "image", "result", "error", "slot", slot)
			writeError(c, http.StatusInternalServerError, "internal_error", "Service tạm thời không khả dụng.")
			return
		}
		// Redirecting preserves the stable same-origin route while letting the
		// CDN deliver the image directly instead of consuming API bandwidth.
		c.Redirect(http.StatusFound, image.CloudinarySecureURL)
		return
	}
	if image.StorageProvider != "" && image.StorageProvider != "DATABASE" {
		h.logger.Error("promotion image has unknown storage provider", "action", "image", "result", "error", "slot", slot)
		writeError(c, http.StatusInternalServerError, "internal_error", "Service tạm thời không khả dụng.")
		return
	}
	if !isSupportedPromotionMIME(image.MIMEType) || len(image.ImageBytes) == 0 {
		h.logger.Error("legacy promotion image failed integrity checks", "action", "image", "result", "error", "slot", slot)
		writeError(c, http.StatusInternalServerError, "internal_error", "Service tạm thời không khả dụng.")
		return
	}
	c.Data(http.StatusOK, image.MIMEType, image.ImageBytes)
}

// Upsert parses one multipart slot replacement. Authorization is enforced by
// middleware before this function runs; content validation is delegated to the
// service so non-HTTP callers cannot bypass it.
func (h *PromotionHandler) Upsert(c *gin.Context) {
	slot, err := parsePromotionSlot(c.Param("slot"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_promotion_slot", "Vị trí quảng bá không hợp lệ.")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxRequestBytes)
	multipartFile, file, err := c.Request.FormFile("image")
	if err != nil {
		if isRequestTooLarge(err) {
			writeError(c, http.StatusRequestEntityTooLarge, "promotion_image_too_large", "Ảnh quảng bá vượt quá dung lượng cho phép.")
			return
		}
		writeError(c, http.StatusBadRequest, "invalid_promotion", "Vui lòng gửi đúng multipart field image.")
		return
	}
	_ = multipartFile.Close()
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}

	promotion, err := h.promotions.Upsert(c.Request.Context(), service.PromotionInput{
		Slot:      slot,
		File:      file,
		AltText:   c.PostForm("alt_text"),
		Eyebrow:   c.PostForm("eyebrow"),
		Title:     c.PostForm("title"),
		Body:      c.PostForm("body"),
		TargetURL: c.PostForm("target_url"),
	})
	if err != nil {
		writeMappedError(c, err)
		return
	}
	h.logger.Info("promotion updated", "action", "upsert", "result", "success", "slot", slot)
	c.JSON(http.StatusOK, mapPromotion(promotion))
}

// Delete removes one promotion slot and intentionally returns 204 even when
// the slot was already empty, making admin retries safe.
func (h *PromotionHandler) Delete(c *gin.Context) {
	slot, err := parsePromotionSlot(c.Param("slot"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_promotion_slot", "Vị trí quảng bá không hợp lệ.")
		return
	}
	if err := h.promotions.Delete(c.Request.Context(), slot); err != nil {
		writeMappedError(c, err)
		return
	}
	h.logger.Info("promotion deleted", "action", "delete", "result", "success", "slot", slot)
	c.Status(http.StatusNoContent)
}

type promotionListResponse struct {
	Promotions []promotionResponse `json:"promotions"`
}

type promotionResponse struct {
	Slot        int16  `json:"slot"`
	ImageURL    string `json:"image_url"`
	AltText     string `json:"alt_text"`
	Eyebrow     string `json:"eyebrow,omitempty"`
	Title       string `json:"title,omitempty"`
	Body        string `json:"body,omitempty"`
	TargetURL   string `json:"target_url,omitempty"`
	ContentHash string `json:"content_hash"`
}

func mapPromotion(promotion model.Promotion) promotionResponse {
	return promotionResponse{
		Slot:        promotion.Slot,
		ImageURL:    promotion.ImageURL,
		AltText:     promotion.AltText,
		Eyebrow:     valueOrEmpty(promotion.Eyebrow),
		Title:       valueOrEmpty(promotion.Title),
		Body:        valueOrEmpty(promotion.Body),
		TargetURL:   valueOrEmpty(promotion.TargetURL),
		ContentHash: promotion.ContentHash,
	}
}

// parsePromotionSlot converts an untrusted path parameter into the only
// numeric range the promotion contract permits.
func parsePromotionSlot(raw string) (int16, error) {
	slot, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 16)
	if err != nil || slot < 1 || slot > 3 {
		return 0, service.ErrInvalidPromotion
	}
	return int16(slot), nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// isSupportedPromotionMIME is a defense-in-depth check before serving bytes
// loaded from persistence; the upload service and database enforce the same
// allowlist on the write path.
func isSupportedPromotionMIME(mimeType string) bool {
	switch mimeType {
	case "image/png", "image/jpeg", "image/webp":
		return true
	default:
		return false
	}
}

func isValidPromotionHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func isValidPromotionCloudinaryURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

// etagMatches supports the simple exact/multi-value forms browsers send for
// this immutable content path, including the wildcard revalidation token.
func etagMatches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		if strings.TrimSpace(candidate) == etag || strings.TrimSpace(candidate) == "*" {
			return true
		}
	}
	return false
}
