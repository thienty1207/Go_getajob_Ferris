package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/gogetsomefoodferris/backend/internal/service"
)

// errorResponse is the single machine-readable error envelope shared by scan
// and promotion routes. Internal errors are intentionally mapped to generic
// public messages.
type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeError terminates the current Gin chain so a handler cannot accidentally
// append a second response after an error.
func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errorResponse{Code: code, Message: message})
}

// writeMappedError converts domain/repository errors into stable HTTP status
// codes without forwarding raw error text to the browser.
func writeMappedError(c *gin.Context, err error) {
	status, code, message := mapError(err)
	writeError(c, status, code, message)
}

func mapError(err error) (int, string, string) {
	switch {
	case errors.Is(err, service.ErrInvalidPromotion):
		return http.StatusBadRequest, "invalid_promotion", "Thông tin quảng bá chưa hợp lệ."
	case errors.Is(err, service.ErrPromotionImageTooLarge):
		return http.StatusRequestEntityTooLarge, "promotion_image_too_large", "Ảnh quảng bá vượt quá dung lượng cho phép."
	case errors.Is(err, service.ErrPromotionStorage):
		return http.StatusBadGateway, "promotion_storage_unavailable", "Không thể lưu ảnh quảng bá lúc này. Vui lòng thử lại sau."
	case errors.Is(err, service.ErrInvalidHomeSection):
		return http.StatusBadRequest, "invalid_home_section", "Nội dung section Home chưa hợp lệ."
	case errors.Is(err, service.ErrInvalidHomeMedia):
		return http.StatusBadRequest, "invalid_home_media", "Ảnh section Home chưa hợp lệ."
	case errors.Is(err, service.ErrHomeSectionStorage):
		return http.StatusBadGateway, "home_section_storage_unavailable", "Không thể lưu ảnh section Home lúc này. Vui lòng thử lại sau."
	case errors.Is(err, repository.ErrHomeSectionNotFound), errors.Is(err, repository.ErrHomeSectionMediaNotFound):
		return http.StatusNotFound, "home_section_not_found", "Section Home không tồn tại."
	case errors.Is(err, repository.ErrHomeSectionMediaLimit):
		return http.StatusConflict, "home_section_media_limit", "Section ảnh đã đạt tối đa 10 ảnh."
	case errors.Is(err, repository.ErrPromotionNotFound):
		return http.StatusNotFound, "promotion_not_found", "Quảng bá không tồn tại."
	case errors.Is(err, service.ErrInvalidScanInput):
		return http.StatusBadRequest, "invalid_scan_request", "Thông tin scan chưa hợp lệ."
	case errors.Is(err, service.ErrInvalidUpload):
		return http.StatusBadRequest, "invalid_upload", "Tệp CV không hợp lệ. Chỉ nhận PDF, DOCX hoặc TXT đúng định dạng."
	case errors.Is(err, service.ErrUploadTooLarge):
		return http.StatusRequestEntityTooLarge, "upload_too_large", "Tệp CV vượt quá dung lượng cho phép."
	case errors.Is(err, repository.ErrScanNotFound):
		return http.StatusNotFound, "scan_not_found", "Scan không tồn tại."
	case errors.Is(err, repository.ErrClientCVNotFound):
		return http.StatusNotFound, "cv_not_found", "CV không tồn tại hoặc không thuộc tài khoản này."
	case errors.Is(err, repository.ErrAdminCVNotFound):
		return http.StatusNotFound, "cv_not_found", "CV không tồn tại."
	case errors.Is(err, service.ErrInvalidClientCVUser):
		return http.StatusBadRequest, "invalid_cv_request", "Yêu cầu CV chưa hợp lệ."
	case errors.Is(err, service.ErrScanProcessing):
		return http.StatusInternalServerError, "scan_unavailable", "Không thể cập nhật trạng thái scan."
	case errors.Is(err, service.ErrAdminSessionMissing), errors.Is(err, service.ErrAdminSessionExpired), errors.Is(err, service.ErrAdminSessionRevoked), errors.Is(err, service.ErrAdminInactive):
		return http.StatusUnauthorized, "admin_auth_required", "Phiên quản trị không còn hợp lệ. Vui lòng đăng nhập lại."
	case errors.Is(err, service.ErrAdminAuthStorage):
		return http.StatusServiceUnavailable, "database_unavailable", "Database chưa sẵn sàng. Vui lòng thử lại sau."
	case errors.Is(err, service.ErrAdminCSRFInvalid):
		return http.StatusForbidden, "admin_csrf_invalid", "Phiên quản trị không hợp lệ cho thao tác này."
	case errors.Is(err, service.ErrInvalidAdminEmail), errors.Is(err, service.ErrInvalidAdminPassword):
		return http.StatusBadRequest, "invalid_admin_input", "Thông tin tài khoản quản trị chưa hợp lệ."
	case errors.Is(err, service.ErrInvalidJobLinkURL), errors.Is(err, service.ErrInvalidJobLinkApproval):
		return http.StatusBadRequest, "invalid_job_link", "URL Job Link chưa hợp lệ hoặc thiếu thông tin duyệt."
	case errors.Is(err, service.ErrInvalidJobLinkID):
		return http.StatusBadRequest, "invalid_job_link_id", "Job Link không hợp lệ."
	case errors.Is(err, service.ErrInvalidJobLinkStatus):
		return http.StatusBadRequest, "invalid_job_link_status", "Trạng thái Job Link không hợp lệ."
	case errors.Is(err, service.ErrInvalidCrawlerInterval), errors.Is(err, service.ErrInvalidSettingsActor):
		return http.StatusBadRequest, "invalid_crawler_settings", "Khoảng thời gian crawl chưa hợp lệ."
	case errors.Is(err, service.ErrInvalidCrawlRequestSource):
		return http.StatusBadRequest, "invalid_crawl_request", "Job Link không hợp lệ."
	case errors.Is(err, service.ErrInvalidCrawlRequestActor):
		return http.StatusBadRequest, "invalid_crawl_request", "Không thể tạo yêu cầu crawl."
	case errors.Is(err, repository.ErrJobLinkNotFound):
		return http.StatusNotFound, "job_link_not_found", "Job Link không tồn tại."
	case errors.Is(err, repository.ErrJobLinkConflict):
		return http.StatusConflict, "job_link_exists", "Job Link này đã được thêm trước đó."
	case errors.Is(err, repository.ErrCrawlSourceNotFound):
		return http.StatusNotFound, "job_link_not_found", "Job Link không tồn tại."
	case errors.Is(err, repository.ErrCrawlSourceInactive):
		return http.StatusConflict, "job_link_inactive", "Chỉ Job Link đang ACTIVE mới có thể crawl."
	case errors.Is(err, service.ErrInvalidLocation):
		return http.StatusBadRequest, "invalid_location", "Thông tin location chưa hợp lệ."
	case errors.Is(err, service.ErrInvalidLocationID):
		return http.StatusBadRequest, "invalid_location_id", "Location hoặc job không hợp lệ."
	case errors.Is(err, repository.ErrLocationNotFound):
		return http.StatusNotFound, "location_not_found", "Location không tồn tại."
	case errors.Is(err, repository.ErrLocationConflict):
		return http.StatusConflict, "location_exists", "Location này đã tồn tại."
	case errors.Is(err, repository.ErrJobNotFound):
		return http.StatusNotFound, "job_not_found", "Job không tồn tại."
	default:
		return http.StatusInternalServerError, "internal_error", "Service tạm thời không khả dụng."
	}
}
