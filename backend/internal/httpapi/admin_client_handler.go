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
)

type adminClientUserAPI interface {
	ListAdminClientUsers(context.Context, int, int, repository.AdminClientUserFilter) (model.AdminClientUserPage, error)
}

type AdminClientUserHandler struct {
	users adminClientUserAPI
}

func NewAdminClientUserHandler(users adminClientUserAPI) *AdminClientUserHandler {
	return &AdminClientUserHandler{users: users}
}

type adminClientUserPageResponse struct {
	Items    []adminClientUserResponse `json:"items"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
	Total    int                       `json:"total"`
}

type adminClientUserResponse struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	Provider    string  `json:"provider"`
	CreatedAt   string  `json:"created_at"`
	LastLoginAt string  `json:"last_login_at"`
}

func (h *AdminClientUserHandler) List(c *gin.Context) {
	page, pageSize, err := parseAdminClientPaging(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_user_paging", "Thông số phân trang user không hợp lệ.")
		return
	}
	search := strings.TrimSpace(c.Query("q"))
	if len(search) > 200 {
		writeError(c, http.StatusBadRequest, "invalid_user_filter", "Từ khóa user quá dài.")
		return
	}
	result, err := h.users.ListAdminClientUsers(c.Request.Context(), page, pageSize, repository.AdminClientUserFilter{Search: search})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "Không thể đọc danh sách user lúc này.")
		return
	}
	items := make([]adminClientUserResponse, 0, len(result.Items))
	for _, user := range result.Items {
		items = append(items, mapAdminClientUser(user))
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, adminClientUserPageResponse{Items: items, Page: result.Page, PageSize: result.PageSize, Total: result.Total})
}

func parseAdminClientPaging(c *gin.Context) (int, int, error) {
	return parseBoundedPaging(c, "user")
}

func mapAdminClientUser(user model.AdminClientUser) adminClientUserResponse {
	return adminClientUserResponse{
		ID:          user.ID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Provider:    user.Provider,
		CreatedAt:   user.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		LastLoginAt: user.LastLoginAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func parseBoundedPaging(c *gin.Context, _ string) (int, int, error) {
	page, pageSize := 1, 10
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100000 {
			return 0, 0, errors.New("invalid page")
		}
		page = value
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 10 {
			return 0, 0, errors.New("invalid page size")
		}
		pageSize = value
	}
	return page, pageSize, nil
}
