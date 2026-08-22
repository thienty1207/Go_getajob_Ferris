package model

import (
	"time"

	"github.com/google/uuid"
)

type AdminClientUser struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	AvatarURL   *string
	Provider    string
	CreatedAt   time.Time
	LastLoginAt time.Time
}

type AdminClientUserPage struct {
	Items    []AdminClientUser
	Page     int
	PageSize int
	Total    int
}

type AdminCVProfile struct {
	ClientCVHistoryItem
	UserID      uuid.UUID
	Email       string
	DisplayName string
}

type AdminCVProfilePage struct {
	Items    []AdminCVProfile
	Page     int
	PageSize int
	Total    int
}
